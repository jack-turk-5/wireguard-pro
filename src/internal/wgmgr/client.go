// Package wgmgr manages peers on a running wireguard-go device via wgctrl,
// replacing the original app's shell-outs to wg(8)/wg-quick and its on-disk
// wg0.conf text file. The SQLite DB (internal/db) is the sole source of
// truth for peer state; every add/remove here is a direct UAPI call.
package wgmgr

import (
	"fmt"

	"golang.zx2c4.com/wireguard/wgctrl"
)

// Client manages peers on a single WireGuard interface.
type Client struct {
	wg    *wgctrl.Client
	iface string
}

// New opens a wgctrl client targeting the named interface. iface must already
// exist and be running (e.g. via wgengine.Up) -- New does not create it.
func New(iface string) (*Client, error) {
	wg, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("wgmgr: open wgctrl client: %w", err)
	}
	return &Client{wg: wg, iface: iface}, nil
}

// Close closes the underlying wgctrl client.
func (c *Client) Close() error {
	return c.wg.Close()
}
