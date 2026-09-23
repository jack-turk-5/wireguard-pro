//go:build !linux && !openbsd && !freebsd

/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2017-2025 WireGuard LLC. All Rights Reserved.
 */

// Forked from golang.zx2c4.com/wireguard@v0.0.0-20260522210424-ecfc5a8d5446/conn/mark_default.go, see README.md.

package stdbind

func (s *StdNetBind) SetMark(mark uint32) error {
	return nil
}
