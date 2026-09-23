//go:build linux

/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2017-2025 WireGuard LLC. All Rights Reserved.
 */

// Forked from golang.zx2c4.com/wireguard@v0.0.0-20260522210424-ecfc5a8d5446/conn/controlfns.go
// and controlfns_linux.go, see README.md.
//
// Rewritten as a post-hoc applier: upstream's controlFns run inside
// net.ListenConfig.Control, called while a fresh socket is being bound.
// This fork never binds -- it adopts already-bound (typically
// systemd-activated) sockets -- so applySockopts runs the same option set
// directly against an adopted *os.File via SyscallConn().Control instead.
// socketBufferSize and kernelVersion are otherwise unchanged from upstream.

package stdbind

import (
	"fmt"
	"net"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

// UDP socket read/write buffer size (7MB). See upstream controlfns.go for
// why this specific value; Linux clamps it to net.core.{r,w}mem_max unless
// SO_*BUFFORCE succeeds (CAP_NET_ADMIN), handled below.
const socketBufferSize = 7 << 20

// FamilyInfo records what applySockopts found/applied for one adopted
// address family's socket, plus (once Open has run) its offload state.
// Everything here feeds StdNetBind.Describe (bind.go, §4.4 of
// docs/design-doc.md).
type FamilyInfo struct {
	LocalAddr string // e.g. "0.0.0.0:51820"
	RcvBuf    int    // effective SO_RCVBUF read back after setting
	SndBuf    int    // effective SO_SNDBUF read back after setting
	PktInfo   bool   // IP_PKTINFO / IPV6_RECVPKTINFO applied (sticky-socket source tracking)
	DualStack bool   // v6 only: IPV6_V6ONLY == 0
	TxOffload bool   // UDP_SEGMENT supported; set by Open via supportsUDPOffload
	RxOffload bool   // UDP_GRO active; set by Open via supportsUDPOffload
}

// applySockopts configures an adopted socket the same way upstream's
// listenConfig control chain would a freshly-bound one: SO_RCVBUF/SNDBUF,
// then the *BUFFORCE variants (expected to fail with EPERM without
// CAP_NET_ADMIN in this netns/userns -- kept so a root deployment, see
// docs/design-doc.md Phase 3, still benefits), IP_PKTINFO/IPV6_RECVPKTINFO,
// and UDP_GRO (kernel >= 5.12 only -- mandatory, not best-effort: without it
// supportsUDPOffload reports rxOffload=false and the batch receive path
// silently degrades to one packet per syscall). Returns the socket's bound
// port, read back via getsockname.
func applySockopts(f *os.File, network string) (*FamilyInfo, uint16, error) {
	info := &FamilyInfo{}
	var port uint16
	var opErr error

	sc, err := f.SyscallConn()
	if err != nil {
		return nil, 0, fmt.Errorf("SyscallConn: %w", err)
	}

	err = sc.Control(func(fd uintptr) {
		ifd := int(fd)

		_ = unix.SetsockoptInt(ifd, unix.SOL_SOCKET, unix.SO_RCVBUF, socketBufferSize)
		_ = unix.SetsockoptInt(ifd, unix.SOL_SOCKET, unix.SO_SNDBUF, socketBufferSize)
		_ = unix.SetsockoptInt(ifd, unix.SOL_SOCKET, unix.SO_RCVBUFFORCE, socketBufferSize)
		_ = unix.SetsockoptInt(ifd, unix.SOL_SOCKET, unix.SO_SNDBUFFORCE, socketBufferSize)
		if v, err := unix.GetsockoptInt(ifd, unix.SOL_SOCKET, unix.SO_RCVBUF); err == nil {
			info.RcvBuf = v
		}
		if v, err := unix.GetsockoptInt(ifd, unix.SOL_SOCKET, unix.SO_SNDBUF); err == nil {
			info.SndBuf = v
		}

		switch network {
		case "udp4":
			if runtime.GOOS != "android" {
				info.PktInfo = unix.SetsockoptInt(ifd, unix.IPPROTO_IP, unix.IP_PKTINFO, 1) == nil
			}
		case "udp6":
			v6only, err := unix.GetsockoptInt(ifd, unix.IPPROTO_IPV6, unix.IPV6_V6ONLY)
			if err != nil {
				opErr = fmt.Errorf("read IPV6_V6ONLY: %w", err)
				return
			}
			info.DualStack = v6only == 0
			if runtime.GOOS != "android" {
				info.PktInfo = unix.SetsockoptInt(ifd, unix.IPPROTO_IPV6, unix.IPV6_RECVPKTINFO, 1) == nil
			}
		default:
			opErr = fmt.Errorf("unhandled network: %s: %w", network, unix.EINVAL)
			return
		}

		if major, minor := kernelVersion(); major > 5 || (major == 5 && minor >= 12) {
			_ = unix.SetsockoptInt(ifd, unix.IPPROTO_UDP, unix.UDP_GRO, 1)
		}

		sa, err := unix.Getsockname(ifd)
		if err != nil {
			opErr = fmt.Errorf("getsockname: %w", err)
			return
		}
		port, info.LocalAddr = addrFromSockaddr(sa)
	})
	if err != nil {
		return nil, 0, err
	}
	if opErr != nil {
		return nil, 0, opErr
	}

	return info, port, nil
}

func addrFromSockaddr(sa unix.Sockaddr) (port uint16, addr string) {
	switch a := sa.(type) {
	case *unix.SockaddrInet4:
		return uint16(a.Port), (&net.UDPAddr{IP: net.IP(a.Addr[:]), Port: a.Port}).String()
	case *unix.SockaddrInet6:
		return uint16(a.Port), (&net.UDPAddr{IP: net.IP(a.Addr[:]), Port: a.Port}).String()
	default:
		return 0, ""
	}
}

// kernelVersion is taken from go/src/internal/syscall/unix/kernel_version_linux.go,
// via upstream controlfns_linux.go, unchanged.
func kernelVersion() (major, minor int) {
	var uname unix.Utsname
	if err := unix.Uname(&uname); err != nil {
		return
	}

	var (
		values    [2]int
		value, vi int
	)
	for _, c := range uname.Release {
		if '0' <= c && c <= '9' {
			value = (value * 10) + int(c-'0')
		} else {
			// Note that we're assuming N.N.N here.
			// If we see anything else, we are likely to mis-parse it.
			values[vi] = value
			vi++
			if vi >= len(values) {
				break
			}
			value = 0
		}
	}

	return values[0], values[1]
}

func kernelVersionString() string {
	major, minor := kernelVersion()
	if major == 0 && minor == 0 {
		return "unknown"
	}
	return fmt.Sprintf("%d.%d", major, minor)
}
