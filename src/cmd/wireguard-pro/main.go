// Command wireguard-pro is the entire application in a single binary: it
// brings up wg0 via wireguard-go (using a systemd-activated socket for the
// VPN UDP listener when available), reconciles peers from SQLite onto the
// running device via wgctrl, applies the nftables ruleset, runs the
// expired-peer sweep, and serves the dashboard API and frontend.
package main

import (
	"context"
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
	"wireguard-pro/internal/wgmgr"
)

const (
	ifaceName = "wg0"
	mtu       = 1420

	// Peer address allocation (internal/wgmgr.NextAvailableIP) assumes a /24
	// and /64, so the interface itself is brought up with matching prefixes.
	ipv4Prefix = "/24"
	ipv6Prefix = "/64"

	// Indexes into the systemd-activated fd list, matching the declaration
	// order in quadlet/wireguard-pro.socket (ListenStream before
	// ListenDatagram): 0 = dashboard TCP, 1 = VPN UDP.
	dashSocketIndex = 0
	vpnSocketIndex  = 1

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

	files := sockact.Load()

	bind, vpnPort, err := buildBind(files)
	if err != nil {
		return fmt.Errorf("build VPN bind: %w", err)
	}
	log.Printf("wireguard-pro: VPN UDP bind ready on port %d", vpnPort)

	eng, err := wgengine.Up(wgengine.Config{
		InterfaceName: ifaceName,
		MTU:           mtu,
		IPv4Addr:      cfg.WGIPv4BaseAddr + ipv4Prefix,
		IPv6Addr:      cfg.WGIPv6BaseAddr + ipv6Prefix,
		Bind:          bind,
	})
	if err != nil {
		return fmt.Errorf("bring up %s: %w", ifaceName, err)
	}
	defer eng.Close()

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
		Tokens:      auth.New(cfg.SecretKey, tokenMaxAge),
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

	listener, err := dashListener(files)
	if err != nil {
		return fmt.Errorf("build dashboard listener: %w", err)
	}
	log.Printf("wireguard-pro: dashboard listening on %s", listener.Addr())

	serveErrCh := make(chan error, 1)
	go func() { serveErrCh <- httpServer.Serve(listener) }()

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

// buildBind prefers the systemd-activated UDP socket for the VPN listener,
// falling back to a self-bound conn.StdNetBind for non-systemd local dev.
func buildBind(files *sockact.Files) (conn.Bind, uint16, error) {
	if files.Len() > vpnSocketIndex {
		pc, err := files.PacketConn(vpnSocketIndex)
		if err != nil {
			return nil, 0, fmt.Errorf("resolve activated VPN socket: %w", err)
		}
		actualPort := uint16(0)
		if udpAddr, ok := pc.LocalAddr().(*net.UDPAddr); ok {
			actualPort = uint16(udpAddr.Port)
		}
		log.Printf("wireguard-pro: using systemd-activated socket for VPN UDP listener")
		return wgengine.NewFdBind(pc), actualPort, nil
	}

	log.Printf("wireguard-pro: no systemd-activated VPN socket found, falling back to self-bound (local dev only)")
	return conn.NewStdNetBind(), vpnDevFallbackPort, nil
}

// dashListener prefers the systemd-activated TCP socket for the dashboard,
// falling back to a self-bound listener for non-systemd local dev.
func dashListener(files *sockact.Files) (net.Listener, error) {
	if files.Len() > dashSocketIndex {
		log.Printf("wireguard-pro: using systemd-activated socket for dashboard listener")
		return files.Listener(dashSocketIndex)
	}

	log.Printf("wireguard-pro: no systemd-activated dashboard socket found, falling back to self-bound (local dev only)")
	return net.Listen("tcp", dashDevFallbackAddr)
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
		if err := writeKeyFile(path, key); err != nil {
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
	if err := writeKeyFile(path, key); err != nil {
		return wgtypes.Key{}, err
	}
	return key, nil
}

func writeKeyFile(path string, key wgtypes.Key) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", path, err)
		}
	}
	if err := os.WriteFile(path, []byte(key.String()+"\n"), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
