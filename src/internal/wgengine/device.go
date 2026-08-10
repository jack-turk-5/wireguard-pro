package wgengine

import (
	"fmt"
	"net"
	"strings"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
)

// Engine owns the TUN device, the wireguard-go Device, and the UAPI socket
// used for external control (wgctrl, wg(8) for manual debugging).
type Engine struct {
	Tun    tun.Device
	Device *device.Device
	uapi   net.Listener
}

// Config describes how to bring up the wg0 interface.
type Config struct {
	InterfaceName string
	MTU           int
	IPv4Addr      string // CIDR, e.g. "10.8.0.1/24"
	IPv6Addr      string // CIDR, e.g. "fd86:ea04:1111::1/64"
	// Bind is the conn.Bind to use for the device's UDP transport. Pass an
	// *FdBind wrapping a systemd-activated socket in production; tests/local
	// dev without socket activation can pass conn.NewStdNetBind() instead.
	Bind conn.Bind
	// LogLevel is "silent", "error", or "verbose" (case-insensitive);
	// anything else falls back to "error".
	LogLevel string
}

// Up creates the TUN device, assigns addresses/brings the link up via
// netlink, and constructs the wireguard-go Device bound to cfg.Bind. It does
// not configure a private key or peers -- callers do that via wgctrl against
// the UAPI socket this opens (see internal/wgmgr).
func Up(cfg Config) (*Engine, error) {
	tunDev, err := tun.CreateTUN(cfg.InterfaceName, cfg.MTU)
	if err != nil {
		return nil, fmt.Errorf("wgengine: create TUN %s: %w", cfg.InterfaceName, err)
	}

	realName, err := tunDev.Name()
	if err != nil {
		tunDev.Close()
		return nil, fmt.Errorf("wgengine: get TUN name: %w", err)
	}

	if err := configureLink(realName, cfg.IPv4Addr, cfg.IPv6Addr); err != nil {
		tunDev.Close()
		return nil, err
	}

	logger := device.NewLogger(parseLogLevel(cfg.LogLevel), fmt.Sprintf("(%s) ", realName))
	dev := device.NewDevice(tunDev, cfg.Bind, logger)

	uapiFile, err := ipc.UAPIOpen(realName)
	if err != nil {
		dev.Close()
		return nil, fmt.Errorf("wgengine: open UAPI socket for %s: %w", realName, err)
	}
	uapiListener, err := ipc.UAPIListen(realName, uapiFile)
	if err != nil {
		dev.Close()
		return nil, fmt.Errorf("wgengine: listen on UAPI socket for %s: %w", realName, err)
	}

	go func() {
		for {
			c, err := uapiListener.Accept()
			if err != nil {
				return
			}
			go dev.IpcHandle(c)
		}
	}()

	if err := dev.Up(); err != nil {
		uapiListener.Close()
		dev.Close()
		return nil, fmt.Errorf("wgengine: bring up device: %w", err)
	}

	return &Engine{Tun: tunDev, Device: dev, uapi: uapiListener}, nil
}

func parseLogLevel(s string) int {
	switch strings.ToLower(s) {
	case "silent":
		return device.LogLevelSilent
	case "verbose":
		return device.LogLevelVerbose
	default:
		return device.LogLevelError
	}
}

// Close tears down the UAPI listener and the wireguard-go device (which in
// turn closes the TUN device and the Bind).
func (e *Engine) Close() error {
	if e.uapi != nil {
		e.uapi.Close()
	}
	e.Device.Close()
	return nil
}

// configureLink assigns the given addresses to the named interface and sets
// it up, via netlink -- this replaces wg-quick's non-peer plumbing (`ip addr
// add`, `ip link set up`). It does NOT create a netlink.Wireguard link: that
// type drives the Linux kernel's in-tree WireGuard implementation, which is
// not what's in play here -- tun.CreateTUN already created a plain TUN
// device; netlink is only used here to configure it by name.
func configureLink(name, ipv4CIDR, ipv6CIDR string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("wgengine: find link %s: %w", name, err)
	}

	for _, cidr := range []string{ipv4CIDR, ipv6CIDR} {
		if cidr == "" {
			continue
		}
		addr, err := netlink.ParseAddr(cidr)
		if err != nil {
			return fmt.Errorf("wgengine: parse address %q: %w", cidr, err)
		}
		if err := netlink.AddrAdd(link, addr); err != nil {
			return fmt.Errorf("wgengine: add address %q to %s: %w", cidr, name, err)
		}
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("wgengine: set %s up: %w", name, err)
	}

	return nil
}
