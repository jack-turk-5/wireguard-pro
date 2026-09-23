//go:build !linux

/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2017-2025 WireGuard LLC. All Rights Reserved.
 */

// Forked from golang.zx2c4.com/wireguard@v0.0.0-20260522210424-ecfc5a8d5446/conn/gso_default.go, see README.md.

package stdbind

// getGSOSize parses control for UDP_GRO and if found returns its GSO size data.
func getGSOSize(control []byte) (int, error) {
	return 0, nil
}

// setGSOSize sets a UDP_SEGMENT in control based on gsoSize.
func setGSOSize(control *[]byte, gsoSize uint16) {
}

// gsoControlSize returns the recommended buffer size for pooling sticky and UDP
// offloading control data.
const gsoControlSize = 0
