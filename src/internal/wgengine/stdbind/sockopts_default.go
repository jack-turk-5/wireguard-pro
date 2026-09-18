//go:build !linux

/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2017-2025 WireGuard LLC. All Rights Reserved.
 */

// Forked from golang.zx2c4.com/wireguard@v0.0.0-20260522210424-ecfc5a8d5446/conn/controlfns.go
// and controlfns_unix.go, see README.md.
//
// This fork's socket adoption is Linux-only in production (systemd socket
// activation is a Linux concept); this file exists only so `stdbind` (and
// its tests, which exercise New/Open for real via loopback sockets, not
// just `go vet`) build and run correctly on a macOS dev machine.

package stdbind

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// UDP socket read/write buffer size (7MB), matching sockopts_linux.go.
const socketBufferSize = 7 << 20

// FamilyInfo mirrors sockopts_linux.go's -- see its doc comment. TxOffload/
// RxOffload/PktInfo are always false here: this platform's stdbind is
// exercised only by local tests, never adopted for a real deployment.
type FamilyInfo struct {
	LocalAddr string
	RcvBuf    int
	SndBuf    int
	PktInfo   bool
	DualStack bool
	TxOffload bool
	RxOffload bool
}

// applySockopts is sockopts_linux.go's applySockopts without the
// Linux-only options (SO_*BUFFORCE, PKTINFO, UDP_GRO have no portable
// equivalent) -- see its doc comment for the shared parts.
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
		if v, err := unix.GetsockoptInt(ifd, unix.SOL_SOCKET, unix.SO_RCVBUF); err == nil {
			info.RcvBuf = v
		}
		if v, err := unix.GetsockoptInt(ifd, unix.SOL_SOCKET, unix.SO_SNDBUF); err == nil {
			info.SndBuf = v
		}

		if network == "udp6" {
			v6only, err := unix.GetsockoptInt(ifd, unix.IPPROTO_IPV6, unix.IPV6_V6ONLY)
			if err != nil {
				opErr = fmt.Errorf("read IPV6_V6ONLY: %w", err)
				return
			}
			info.DualStack = v6only == 0
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

func kernelVersionString() string {
	return "n/a"
}
