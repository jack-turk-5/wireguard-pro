package wgengine

import (
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/tun"
)

// Matches tun_linux.go's own tunTCPOffloads/tunUDPOffloads exactly -- see
// TunOffloads.
const (
	tunTCPOffloads = unix.TUN_F_CSUM | unix.TUN_F_TSO4 | unix.TUN_F_TSO6
	tunUDPOffloads = unix.TUN_F_USO4 | unix.TUN_F_USO6
)

// TunOffloads reports whether dev's TUN device has the virtio-net-header
// batch path (vnetHdr) and UDP GSO/USO offload (udpGSO) active, for startup
// diagnostics (docs/design-doc.md §4.4). tun_linux.go's NativeTun tracks
// both internally but doesn't export them, so this infers vnetHdr from
// BatchSize() (tun_linux.go sets it to conn.IdealBatchSize only when
// vnetHdr is on) and determines udpGSO by re-issuing the identical
// TUNSETOFFLOAD ioctl tun_linux.go's own initFromFlags already issued at
// CreateTUN time: idempotent when it already succeeded, so this doesn't
// change anything -- it just reads back the same yes/no answer (failure
// means the running kernel is below the v6.2 USO floor).
func TunOffloads(dev tun.Device) (vnetHdr, udpGSO bool) {
	vnetHdr = dev.BatchSize() > 1
	if !vnetHdr {
		return false, false
	}

	sc, err := dev.File().SyscallConn()
	if err != nil {
		return true, false
	}
	var ok bool
	if ctrlErr := sc.Control(func(fd uintptr) {
		ok = unix.IoctlSetInt(int(fd), unix.TUNSETOFFLOAD, tunTCPOffloads|tunUDPOffloads) == nil
	}); ctrlErr != nil {
		return true, false
	}
	return true, ok
}
