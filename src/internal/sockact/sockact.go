// Package sockact resolves systemd socket-activated file descriptors.
//
// activation.Listeners() and activation.PacketConns() in go-systemd each call
// Files(true) internally, which unsets LISTEN_FDS/LISTEN_PID after the first
// read. Calling both independently starves whichever runs second of its fds.
// This package calls Files once and hands out typed views over the result.
package sockact

import (
	"fmt"
	"net"
	"os"

	"github.com/coreos/go-systemd/v22/activation"
)

type Files struct {
	files []*os.File
}

// Load resolves all systemd-activated fds exactly once. Safe to call even
// when not socket-activated (e.g. local dev): returns an empty set.
func Load() *Files {
	return &Files{files: activation.Files(true)}
}

func (f *Files) Len() int {
	return len(f.files)
}

// Listener converts the fd at index (declaration order in the .socket unit,
// 0-based) into a net.Listener, for ListenStream= sockets.
func (f *Files) Listener(index int) (net.Listener, error) {
	file, err := f.at(index)
	if err != nil {
		return nil, err
	}
	return net.FileListener(file)
}

// PacketConn converts the fd at index into a net.PacketConn, for
// ListenDatagram= sockets.
func (f *Files) PacketConn(index int) (net.PacketConn, error) {
	file, err := f.at(index)
	if err != nil {
		return nil, err
	}
	return net.FilePacketConn(file)
}

func (f *Files) at(index int) (*os.File, error) {
	if index < 0 || index >= len(f.files) {
		return nil, fmt.Errorf("sockact: no activated fd at index %d (have %d)", index, len(f.files))
	}
	return f.files[index], nil
}
