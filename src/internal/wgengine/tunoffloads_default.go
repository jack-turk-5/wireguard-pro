//go:build !linux

package wgengine

import "golang.zx2c4.com/wireguard/tun"

// TunOffloads is a no-op on non-Linux platforms -- vnet_hdr/UDP GSO/USO are
// Linux TUN ioctls with no equivalent elsewhere. See tunoffloads_linux.go.
func TunOffloads(dev tun.Device) (vnetHdr, udpGSO bool) {
	return false, false
}
