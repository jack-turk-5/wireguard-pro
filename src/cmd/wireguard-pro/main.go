// Command wireguard-pro is a smoke-test harness for phase 1 of the rewrite:
// bring up wg0 via wireguard-go, using a systemd-activated socket for the
// Bind when available and falling back to a self-bound socket for local
// testing. This does not yet include the API/db/scheduler layers -- see the
// project plan for delivery sequencing.
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"wireguard-pro/internal/sockact"
	"wireguard-pro/internal/wgengine"
)

const (
	ifaceName = "wg0"
	mtu       = 1420
	// index of the VPN UDP listener within the .socket unit's declared
	// order -- see quadlet/wireguard-pro.socket (ListenDatagram after
	// ListenStream, so this is fd index 1 -> fd 4).
	vpnSocketIndex = 1
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("wireguard-pro: %v", err)
	}
}

func run() error {
	bind, port, err := buildBind()
	if err != nil {
		return fmt.Errorf("build bind: %w", err)
	}
	log.Printf("wireguard-pro: VPN UDP bind ready on port %d", port)

	eng, err := wgengine.Up(wgengine.Config{
		InterfaceName: ifaceName,
		MTU:           mtu,
		IPv4Addr:      envOr("WG_IPV4_ADDR", "10.8.0.1/24"),
		IPv6Addr:      envOr("WG_IPV6_ADDR", "fd86:ea04:1111::1/64"),
		Bind:          bind,
	})
	if err != nil {
		return fmt.Errorf("bring up %s: %w", ifaceName, err)
	}
	defer eng.Close()

	priv, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return fmt.Errorf("generate private key: %w", err)
	}
	log.Printf("wireguard-pro: %s up, public key %s, UAPI socket at /var/run/wireguard/%s.sock", ifaceName, priv.PublicKey(), ifaceName)

	// NOTE: setting the private key via wgctrl.ConfigureDevice belongs in
	// internal/wgmgr (next phase) -- this smoke test only proves the TUN +
	// netlink + Bind + UAPI wiring comes up cleanly.

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Printf("wireguard-pro: shutting down")
	return nil
}

// buildBind prefers the systemd-activated UDP socket for the VPN listener;
// falling back to a self-bound conn.StdNetBind lets this smoke test (and any
// future non-systemd local dev flow) run without socket activation.
func buildBind() (conn.Bind, uint16, error) {
	files := sockact.Load()
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
	return conn.NewStdNetBind(), 51820, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
