// Package wgengine wires up a wireguard-go device: TUN creation, interface
// setup, and a conn.Bind that can take its UDP socket from an inherited fd
// (systemd socket activation) instead of binding its own.
package wgengine

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/conn"
)

// FdBind implements conn.Bind over a net.PacketConn that's already open,
// typically handed in via systemd socket activation. Unlike conn.StdNetBind,
// Open() never calls net.ListenUDP -- there's nowhere upstream to plug an
// externally-provided socket into that call, since StdNetBind.Open() binds
// unconditionally.
//
// This trades StdNetBind's batched/GSO send-receive path for a simple
// one-packet-at-a-time implementation. That's an intentional, accepted cost:
// the point of this type is fd inheritance for socket activation, not
// matching StdNetBind's throughput optimizations.
//
// Close()/Open() are not symmetric with StdNetBind's: Device.BindUpdate()
// closes and reopens the Bind on every Up()/Down() transition (and whenever
// wgctrl changes ListenPort), which StdNetBind handles by just binding a
// fresh UDP socket again. A systemd-activated fd can't be "rebound" -- it's
// handed to this process exactly once at startup, so if Close() actually
// closed it, the next Open() would fail forever. Instead Close() only forces
// the in-flight read to unblock (via a past read deadline, satisfying the
// Bind contract that receive funcs must return net.ErrClosed after Close)
// and Open() clears that deadline -- the underlying socket stays alive for
// the process lifetime.
type FdBind struct {
	mu     sync.Mutex
	pc     net.PacketConn
	closed bool
}

var _ conn.Bind = (*FdBind)(nil)

// NewFdBind wraps an already-bound net.PacketConn (e.g. from
// sockact.PacketConn) as a conn.Bind.
func NewFdBind(pc net.PacketConn) *FdBind {
	return &FdBind{pc: pc}
}

type fdEndpoint struct {
	addr netip.AddrPort
}

var _ conn.Endpoint = (*fdEndpoint)(nil)

func (e *fdEndpoint) ClearSrc()           {}
func (e *fdEndpoint) SrcToString() string { return "" }
func (e *fdEndpoint) DstToString() string { return e.addr.String() }
func (e *fdEndpoint) DstToBytes() []byte {
	b, _ := e.addr.MarshalBinary()
	return b
}
func (e *fdEndpoint) DstIP() netip.Addr { return e.addr.Addr() }
func (e *fdEndpoint) SrcIP() netip.Addr { return netip.Addr{} }

// ParseEndpoint parses s (an "ip:port" address) into a conn.Endpoint.
func (b *FdBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	addr, err := netip.ParseAddrPort(s)
	if err != nil {
		return nil, err
	}
	return &fdEndpoint{addr: addr}, nil
}

// Open returns a single receive function reading from the wrapped
// net.PacketConn, and the port it's actually bound to. It does not bind a
// new socket -- see the FdBind doc comment for why.
func (b *FdBind) Open(port uint16) ([]conn.ReceiveFunc, uint16, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.pc == nil {
		return nil, 0, fmt.Errorf("wgengine: FdBind has no socket (never provided)")
	}
	if err := b.pc.SetReadDeadline(time.Time{}); err != nil {
		return nil, 0, fmt.Errorf("wgengine: clear read deadline on reopen: %w", err)
	}
	b.closed = false

	actualPort := port
	if udpAddr, ok := b.pc.LocalAddr().(*net.UDPAddr); ok {
		actualPort = uint16(udpAddr.Port)
	}

	pc := b.pc
	fn := func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		n, addr, err := pc.ReadFrom(bufs[0])
		if err != nil {
			b.mu.Lock()
			closed := b.closed
			b.mu.Unlock()
			if closed {
				return 0, net.ErrClosed
			}
			return 0, err
		}
		sizes[0] = n
		udpAddr, ok := addr.(*net.UDPAddr)
		if !ok {
			return 0, fmt.Errorf("wgengine: unexpected addr type %T from ReadFrom", addr)
		}
		eps[0] = &fdEndpoint{addr: udpAddr.AddrPort()}
		return 1, nil
	}

	return []conn.ReceiveFunc{fn}, actualPort, nil
}

// Close unblocks the in-flight receive (forcing it to return net.ErrClosed,
// per the Bind contract) without closing the underlying socket -- see the
// FdBind doc comment for why. Safe to call multiple times and safe to Open()
// again afterward.
func (b *FdBind) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.pc == nil || b.closed {
		return nil
	}
	b.closed = true
	return b.pc.SetReadDeadline(time.Now())
}

// SetMark is a no-op: SO_MARK requires raw fd access, and this deployment has
// no policy-routing rule keyed on a fwmark to justify adding it. Logged
// rather than silently swallowed, since a future caller relying on marking
// working would otherwise fail silently.
func (b *FdBind) SetMark(mark uint32) error {
	log.Printf("wgengine: FdBind.SetMark(%d) is a no-op (no fwmark/policy-routing support in this Bind)", mark)
	return nil
}

// Send writes each of bufs to ep as a separate UDP datagram.
func (b *FdBind) Send(bufs [][]byte, ep conn.Endpoint) error {
	b.mu.Lock()
	pc := b.pc
	closed := b.closed
	b.mu.Unlock()

	if pc == nil || closed {
		return net.ErrClosed
	}
	e, ok := ep.(*fdEndpoint)
	if !ok {
		return conn.ErrWrongEndpointType
	}

	addr := net.UDPAddrFromAddrPort(e.addr)
	for _, buf := range bufs {
		if _, err := pc.WriteTo(buf, addr); err != nil {
			return err
		}
	}
	return nil
}

// BatchSize is 1: FdBind's Open()/Send() handle one packet per call, unlike
// StdNetBind's batched recvmmsg/sendmmsg path.
func (b *FdBind) BatchSize() int {
	return 1
}
