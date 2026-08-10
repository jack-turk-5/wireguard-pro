package wgmgr

import (
	"fmt"
	"net"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// SetPrivateKey sets the device's private key. Called once at startup, before
// any peers are added.
func (c *Client) SetPrivateKey(priv wgtypes.Key) error {
	err := c.wg.ConfigureDevice(c.iface, wgtypes.Config{PrivateKey: &priv})
	if err != nil {
		return fmt.Errorf("wgmgr: set private key on %s: %w", c.iface, err)
	}
	return nil
}

// AddPeer adds (or replaces) a peer, restricting its allowed-ips to exactly
// its own /32 and /128 addresses -- equivalent to the original app's
// `wg set wg0 peer <pub> allowed-ips <ipv4>/32,<ipv6>/128`.
func (c *Client) AddPeer(pub wgtypes.Key, ipv4, ipv6 string) error {
	allowedIPs, err := peerAllowedIPs(ipv4, ipv6)
	if err != nil {
		return err
	}

	err = c.wg.ConfigureDevice(c.iface, wgtypes.Config{
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:         pub,
				ReplaceAllowedIPs: true,
				AllowedIPs:        allowedIPs,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("wgmgr: add peer %s to %s: %w", pub, c.iface, err)
	}
	return nil
}

// RemovePeer removes a peer from the running device.
func (c *Client) RemovePeer(pub wgtypes.Key) error {
	err := c.wg.ConfigureDevice(c.iface, wgtypes.Config{
		Peers: []wgtypes.PeerConfig{
			{PublicKey: pub, Remove: true},
		},
	})
	if err != nil {
		return fmt.Errorf("wgmgr: remove peer %s from %s: %w", pub, c.iface, err)
	}
	return nil
}

// PeerStat is the live-stats view of a single peer, mirroring what the
// original app's `wg show wg0 dump` parsing exposed to the frontend.
type PeerStat struct {
	PublicKey           wgtypes.Key
	LastHandshakeTime   time.Time
	ReceiveBytes        int64
	TransmitBytes       int64
	PersistentKeepalive time.Duration
}

// Stats returns live stats for every peer currently configured on the device.
func (c *Client) Stats() ([]PeerStat, error) {
	dev, err := c.wg.Device(c.iface)
	if err != nil {
		return nil, fmt.Errorf("wgmgr: get device %s: %w", c.iface, err)
	}

	stats := make([]PeerStat, 0, len(dev.Peers))
	for _, p := range dev.Peers {
		stats = append(stats, PeerStat{
			PublicKey:           p.PublicKey,
			LastHandshakeTime:   p.LastHandshakeTime,
			ReceiveBytes:        p.ReceiveBytes,
			TransmitBytes:       p.TransmitBytes,
			PersistentKeepalive: p.PersistentKeepaliveInterval,
		})
	}
	return stats, nil
}

func peerAllowedIPs(ipv4, ipv6 string) ([]net.IPNet, error) {
	var nets []net.IPNet
	for _, cidr := range []string{ipv4 + "/32", ipv6 + "/128"} {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("wgmgr: parse allowed-ip %q: %w", cidr, err)
		}
		nets = append(nets, *ipNet)
	}
	return nets, nil
}
