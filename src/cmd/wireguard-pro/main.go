// Command wireguard-pro is the entire application in a single binary: it
// brings up wg0 via wireguard-go (using a systemd-activated socket for the
// VPN UDP listener when available), reconciles peers from SQLite onto the
// running device via wgctrl, applies the nftables ruleset, runs the
// expired-peer sweep, and serves the dashboard API and frontend.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"wireguard-pro/internal/api"
	"wireguard-pro/internal/auth"
	"wireguard-pro/internal/config"
	"wireguard-pro/internal/db"
	"wireguard-pro/internal/nft"
	"wireguard-pro/internal/scheduler"
	"wireguard-pro/internal/sockact"
	"wireguard-pro/internal/webui"
	"wireguard-pro/internal/wgengine"
	"wireguard-pro/internal/wgengine/stdbind"
	"wireguard-pro/internal/wgmgr"
)

const (
	ifaceName = "wg0"

	// Peer address allocation (internal/wgmgr.NextAvailableIP) assumes a /24
	// and /64, so the interface itself is brought up with matching prefixes.
	ipv4Prefix = "/24"
	ipv6Prefix = "/64"

	// rcvbufKernelClamp is net.core.rmem_max's kernel-default (212992),
	// doubled by the kernel's own bookkeeping overhead -- the effective
	// SO_RCVBUF read back when SO_RCVBUFFORCE fails (no CAP_NET_ADMIN in
	// this netns/userns) and the host hasn't applied the Phase 2 sysctl
	// bump yet. See docs/design-doc.md §2.4 and the startup diagnostics
	// below.
	rcvbufKernelClamp = 212992 * 2

	// Matches bootstrap.py's hardcoded paths exactly, so an existing
	// deployment's persisted key/secrets keep working unchanged.
	privateKeyPath       = "/etc/wireguard/privatekey"
	privateKeySecretPath = "/run/secrets/wg-privatekey"
	adminUserSecretPath  = "/run/secrets/admin-user"
	adminPassSecretPath  = "/run/secrets/admin-pass"

	tokenMaxAge         = 30 * time.Minute
	expirySweepInterval = time.Hour

	dashDevFallbackAddr = ":51819"
	vpnDevFallbackPort  = 51820
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("wireguard-pro: %v", err)
	}
}

// run drives the whole startup sequence -- config, db, VPN bind, wg0
// bring-up, nftables, server key, peer reconciliation, the expiry-sweep
// goroutine, and the dashboard HTTP server -- then blocks until SIGINT/
// SIGTERM or a fatal server error, shutting down gracefully on the way out.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	database, err := db.Open(cfg.DBFile)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	seedAdminUser(database)

	// Colocated with the DB rather than a fixed path, so it follows DB_FILE
	// overrides to wherever the app's persistent data actually lives.
	secretKeyPath := filepath.Join(filepath.Dir(cfg.DBFile), "secret_key")
	secretKey, err := loadOrCreateSecretKey(secretKeyPath)
	if err != nil {
		return fmt.Errorf("load/create secret key: %w", err)
	}

	sockets, err := sockact.Load()
	if err != nil {
		return fmt.Errorf("resolve systemd-activated sockets: %w", err)
	}
	log.Printf("wireguard-pro: sockets: %s", sockets.Describe())

	bind, vpnPort, err := buildBind(sockets)
	if err != nil {
		return fmt.Errorf("build VPN bind: %w", err)
	}
	log.Printf("wireguard-pro: VPN UDP bind ready on port %d", vpnPort)

	eng, err := wgengine.Up(wgengine.Config{
		InterfaceName: ifaceName,
		MTU:           cfg.WGMTU,
		IPv4Addr:      cfg.WGIPv4BaseAddr + ipv4Prefix,
		IPv6Addr:      cfg.WGIPv6BaseAddr + ipv6Prefix,
		Bind:          bind,
		LogLevel:      cfg.LogLevel,
	})
	if err != nil {
		return fmt.Errorf("bring up %s: %w", ifaceName, err)
	}
	defer eng.Close()

	logStartupDiagnostics(bind, eng)

	if err := nft.Apply(cfg.NFTConfFile); err != nil {
		return fmt.Errorf("apply nftables ruleset: %w", err)
	}
	log.Printf("wireguard-pro: applied nftables ruleset from %s", cfg.NFTConfFile)

	priv, err := loadOrCreateServerKey(privateKeyPath, privateKeySecretPath)
	if err != nil {
		return fmt.Errorf("load/create server key: %w", err)
	}
	pub := priv.PublicKey()
	log.Printf("wireguard-pro: %s up, public key %s, UAPI socket at /var/run/wireguard/%s.sock", ifaceName, pub, ifaceName)

	wgm, err := wgmgr.New(ifaceName)
	if err != nil {
		return fmt.Errorf("open wgctrl client: %w", err)
	}
	defer wgm.Close()

	if err := wgm.SetPrivateKey(priv); err != nil {
		return fmt.Errorf("configure private key via wgctrl: %w", err)
	}

	if err := reconcilePeers(database, wgm); err != nil {
		return fmt.Errorf("reconcile peers onto device: %w", err)
	}

	sweepCtx, stopSweep := context.WithCancel(context.Background())
	defer stopSweep()
	go scheduler.RunExpirySweep(sweepCtx, database, wgm, expirySweepInterval)

	srv := &api.Server{
		Config:      cfg,
		DB:          database,
		WGMgr:       wgm,
		Tokens:      auth.New(secretKey, tokenMaxAge),
		WGPublicKey: pub.String(),
	}
	frontendFS, err := webui.FS(os.Getenv("FRONTEND_DIR"))
	if err != nil {
		return fmt.Errorf("build frontend filesystem: %w", err)
	}

	mux := http.NewServeMux()
	srv.Routes(mux)
	mux.Handle("/", webui.Handler(frontendFS))
	httpServer := &http.Server{Handler: mux}

	listeners, err := dashListeners(sockets)
	if err != nil {
		return fmt.Errorf("build dashboard listener(s): %w", err)
	}
	for _, ln := range listeners {
		log.Printf("wireguard-pro: dashboard listening on %s", ln.Addr())
	}

	// Buffered to len(listeners): Shutdown below closes every listener
	// Serve is called on, and each of those goroutines then sends its own
	// (non-blocking, thanks to the buffer) error here.
	serveErrCh := make(chan error, len(listeners))
	for _, ln := range listeners {
		go func(ln net.Listener) { serveErrCh <- httpServer.Serve(ln) }(ln)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		log.Printf("wireguard-pro: shutting down")
	case err := <-serveErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("dashboard http server: %w", err)
		}
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	return httpServer.Shutdown(shutdownCtx)
}

// buildBind adopts the systemd-activated UDP socket(s) for the VPN
// listener via stdbind, falling back to a self-bound conn.StdNetBind for
// non-systemd local dev. A self-bound port inside pasta's netns would be
// unreachable from outside it, so an activated process with no UDP socket
// at all is a startup error, not a silent fallback -- that shape means
// ListenDatagram= is missing from the socket unit, not that activation
// wasn't used.
func buildBind(s *sockact.Sockets) (conn.Bind, uint16, error) {
	if !s.Activated() {
		log.Printf("wireguard-pro: no systemd-activated sockets found, falling back to self-bound (local dev only)")
		return conn.NewStdNetBind(), vpnDevFallbackPort, nil
	}
	if s.UDP4 == nil && s.UDP6 == nil {
		return nil, 0, fmt.Errorf("socket-activated but no UDP socket passed; check ListenDatagram= in wireguard-pro.socket")
	}

	b, err := stdbind.New(stdbind.Options{UDP4: s.UDP4, UDP6: s.UDP6})
	if err != nil {
		return nil, 0, err
	}
	log.Printf("wireguard-pro: using systemd-activated socket(s) for VPN UDP listener")
	return b, b.Port(), nil
}

// dashListeners returns every systemd-activated TCP listener for the
// dashboard, falling back to a single self-bound listener for non-systemd
// local dev.
func dashListeners(s *sockact.Sockets) ([]net.Listener, error) {
	if len(s.Listeners) > 0 {
		log.Printf("wireguard-pro: using %d systemd-activated socket(s) for the dashboard listener", len(s.Listeners))
		return s.Listeners, nil
	}

	log.Printf("wireguard-pro: no systemd-activated dashboard socket found, falling back to self-bound (local dev only)")
	ln, err := net.Listen("tcp", dashDevFallbackAddr)
	if err != nil {
		return nil, err
	}
	return []net.Listener{ln}, nil
}

// logStartupDiagnostics prints the "bind:"/"tun:" lines described in
// docs/design-doc.md §4.4, once eng.Up has populated offload flags.
func logStartupDiagnostics(bind conn.Bind, eng *wgengine.Engine) {
	if sb, ok := bind.(*stdbind.StdNetBind); ok {
		d := sb.Describe()
		log.Printf("wireguard-pro: bind: %s%s", d, rcvbufClampHint(d))
	} else {
		log.Printf("wireguard-pro: bind: upstream StdNetBind (self-bound)")
	}

	vnetHdr, udpGSO := wgengine.TunOffloads(eng.Tun)
	name, _ := eng.Tun.Name()
	mtu, _ := eng.Tun.MTU()
	log.Printf("wireguard-pro: tun: %s mtu=%d batch=%d vnet_hdr=%s udp_gso=%s",
		name, mtu, eng.Tun.BatchSize(), onOff(vnetHdr), onOff(udpGSO))
}

// rcvbufClampHint flags the specific effective SO_RCVBUF value that means
// "SO_RCVBUFFORCE failed and net.core.rmem_max is still at its kernel
// default" -- i.e. the Phase 2 sysctl bump (docs/quickstart.md) hasn't been
// applied yet. Empty once it has (or once the buffer isn't clamped at that
// exact value for some other reason).
func rcvbufClampHint(d stdbind.Description) string {
	clamped := (d.IPv4 != nil && d.IPv4.RcvBuf == rcvbufKernelClamp) ||
		(d.IPv6 != nil && d.IPv6.RcvBuf == rcvbufKernelClamp)
	if !clamped {
		return ""
	}
	return " (rcvbuf clamped to net.core.rmem_max default; see docs/quickstart.md's sysctl note if UdpRcvbufErrors grows under load)"
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// reconcilePeers pushes every peer stored in the DB onto the freshly-created
// device -- replaces wg-quick's config-file-driven peer load, since a new
// wireguard-go device process always starts with zero peers regardless of
// what was configured before restart.
func reconcilePeers(database *db.DB, wgm *wgmgr.Client) error {
	peers, err := database.ListPeers()
	if err != nil {
		return err
	}
	for _, p := range peers {
		pub, err := wgtypes.ParseKey(p.PublicKey)
		if err != nil {
			log.Printf("wireguard-pro: skipping peer with unparseable public key %q: %v", p.PublicKey, err)
			continue
		}
		if err := wgm.AddPeer(pub, p.IPv4Address, p.IPv6Address); err != nil {
			return fmt.Errorf("add peer %s: %w", p.PublicKey, err)
		}
	}
	log.Printf("wireguard-pro: reconciled %d peer(s) from db onto device", len(peers))
	return nil
}

// seedAdminUser mirrors main.py's lifespan hook: best-effort seed/rotate the
// admin user from mounted secrets, logging and continuing on any failure
// (including the secrets simply not being mounted) rather than treating it
// as fatal.
func seedAdminUser(database *db.DB) {
	user, err := readSecretFile(adminUserSecretPath)
	if err != nil {
		log.Printf("wireguard-pro: admin secrets not found, skipping user seeding: %v", err)
		return
	}
	pass, err := readSecretFile(adminPassSecretPath)
	if err != nil {
		log.Printf("wireguard-pro: admin secrets not found, skipping user seeding: %v", err)
		return
	}
	if err := database.UpsertUser(user, pass); err != nil {
		log.Printf("wireguard-pro: seed admin user: %v", err)
		return
	}
	log.Printf("wireguard-pro: seeded user %q", user)
}

func readSecretFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// loadOrCreateServerKey replicates bootstrap.py's setup_wireguard key
// handling: reuse an already-persisted key if present, else import from a
// mounted secret (and persist it), else generate a fresh one and persist it.
// This must not regenerate on every restart -- the server's public key is
// what every existing peer's config points at.
func loadOrCreateServerKey(path, secretPath string) (wgtypes.Key, error) {
	if data, err := os.ReadFile(path); err == nil {
		return wgtypes.ParseKey(strings.TrimSpace(string(data)))
	} else if !os.IsNotExist(err) {
		return wgtypes.Key{}, fmt.Errorf("read %s: %w", path, err)
	}

	if data, err := os.ReadFile(secretPath); err == nil {
		key, err := wgtypes.ParseKey(strings.TrimSpace(string(data)))
		if err != nil {
			return wgtypes.Key{}, fmt.Errorf("parse %s: %w", secretPath, err)
		}
		if err := writeKeyFile(path, key.String()); err != nil {
			return wgtypes.Key{}, err
		}
		return key, nil
	} else if !os.IsNotExist(err) {
		return wgtypes.Key{}, fmt.Errorf("read %s: %w", secretPath, err)
	}

	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("generate key: %w", err)
	}
	if err := writeKeyFile(path, key.String()); err != nil {
		return wgtypes.Key{}, err
	}
	return key, nil
}

// loadOrCreateSecretKey mirrors loadOrCreateServerKey's persist-once
// behavior for the dashboard's JWT signing key: reuse it if already
// persisted, else generate a fresh random one and persist it. Unlike the
// WireGuard key, there's no "import from a mounted secret" path -- an
// operator can't reasonably supply their own JWT secret ahead of time the
// way they might already have a WireGuard key from `wg genkey`, so this
// deliberately doesn't live in the environment at all (see internal/config's
// doc comment).
func loadOrCreateSecretKey(path string) (string, error) {
	if data, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(data)), nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate secret key: %w", err)
	}
	key := base64.RawURLEncoding.EncodeToString(buf)
	if err := writeKeyFile(path, key); err != nil {
		return "", err
	}
	return key, nil
}

func writeKeyFile(path, contents string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", path, err)
		}
	}
	if err := os.WriteFile(path, []byte(contents+"\n"), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
