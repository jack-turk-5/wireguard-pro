/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2017-2025 WireGuard LLC. All Rights Reserved.
 */

// Forked from golang.zx2c4.com/wireguard@v0.0.0-20260522210424-ecfc5a8d5446/conn/bind_std.go, see README.md.
//
// Modified from upstream: adopts already-bound sockets (Options/New/adopt)
// instead of binding its own (deleted listenNet), exposes Port/Describe for
// startup diagnostics, and returns upstream's own conn.ErrUDPGSODisabled
// instead of a locally-defined duplicate. See README.md for the exact
// regions changed. Everything else -- Close, Send, receiveIP, coalesce/
// split -- is unchanged from upstream.
//
// The conn package import is aliased wgconn: upstream's own bind_std.go
// uses the identifier "conn" pervasively as a local variable name (e.g.
// `conn := s.ipv4`), which would shadow an unaliased import of the very
// package these forked functions came from.

package stdbind

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"runtime"
	"sync"
	"syscall"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	wgconn "golang.zx2c4.com/wireguard/conn"
)

var (
	_ wgconn.Bind = (*StdNetBind)(nil)
)

// StdNetBind implements Bind for all platforms. While Windows has its own Bind
// (see bind_windows.go), it may fall back to StdNetBind.
// TODO: Remove usage of ipv{4,6}.PacketConn when net.UDPConn has comparable
// methods for sending and receiving multiple datagrams per-syscall. See the
// proposal in https://github.com/golang/go/issues/45886#issuecomment-1218301564.
type StdNetBind struct {
	mu            sync.Mutex // protects all fields except as specified
	ipv4          *net.UDPConn
	ipv6          *net.UDPConn
	ipv4PC        *ipv4.PacketConn // will be nil on non-Linux
	ipv6PC        *ipv6.PacketConn // will be nil on non-Linux
	ipv4TxOffload bool
	ipv4RxOffload bool
	ipv6TxOffload bool
	ipv6RxOffload bool

	// these two fields are not guarded by mu
	udpAddrPool sync.Pool
	msgsPool    sync.Pool

	blackhole4 bool
	blackhole6 bool

	// file4/file6 are the adopted, already-bound sockets this Bind was
	// built from (see New) -- pristine, held for the process lifetime, and
	// never closed themselves; see adopt and the Close doc comment for why.
	file4, file6 *os.File
	// port is fixed at New and never changes afterward: it's read back from
	// the adopted socket(s), not chosen by this Bind.
	port uint16
	logf func(string, ...any)
	// info4/info6 hold what New's applySockopts configured, plus (once Open
	// has run) the offload flags supportsUDPOffload found -- see Describe.
	info4, info6 FamilyInfo
	warnedPort   bool
}

// Options configures New. UDP4/UDP6 must be already-bound, already-
// listening AF_INET/AF_INET6 SOCK_DGRAM sockets (typically from
// sockact.Sockets) -- New adopts them rather than binding its own; either
// may be nil, but not both.
type Options struct {
	UDP4, UDP6 *os.File
	// Logf receives diagnostic/warning lines (e.g. a mismatched requested
	// port, see Open); defaults to log.Printf.
	Logf func(string, ...any)
}

// New adopts already-bound sockets as a StdNetBind, applying the same
// socket options and offload configuration upstream's Open would apply to a
// freshly-bound one -- see applySockopts in sockopts_linux.go/
// sockopts_default.go -- and validating that a dual-stack (IPV6_V6ONLY=0)
// UDP6 socket wasn't handed in: this fork always expects UDP4 and UDP6 as
// separate sockets, one family each, so a client's own IPv4 traffic isn't
// silently routed to a family this Bind isn't tracking offload state for.
func New(o Options) (*StdNetBind, error) {
	if o.UDP4 == nil && o.UDP6 == nil {
		return nil, fmt.Errorf("stdbind: New requires at least one of Options.UDP4/UDP6")
	}
	logf := o.Logf
	if logf == nil {
		logf = log.Printf
	}

	s := &StdNetBind{
		file4: o.UDP4,
		file6: o.UDP6,
		logf:  logf,
		udpAddrPool: sync.Pool{
			New: func() any {
				return &net.UDPAddr{
					IP: make([]byte, 16),
				}
			},
		},
		msgsPool: sync.Pool{
			New: func() any {
				// ipv6.Message and ipv4.Message are interchangeable as they are
				// both aliases for x/net/internal/socket.Message.
				msgs := make([]ipv6.Message, wgconn.IdealBatchSize)
				for i := range msgs {
					msgs[i].Buffers = make(net.Buffers, 1)
					msgs[i].OOB = make([]byte, 0, stickyControlSize+gsoControlSize)
				}
				return &msgs
			},
		},
	}

	var port4, port6 uint16
	if o.UDP4 != nil {
		info, p, err := applySockopts(o.UDP4, "udp4")
		if err != nil {
			return nil, fmt.Errorf("stdbind: configure adopted UDP4 socket: %w", err)
		}
		s.info4 = *info
		port4 = p
	}
	if o.UDP6 != nil {
		info, p, err := applySockopts(o.UDP6, "udp6")
		if err != nil {
			return nil, fmt.Errorf("stdbind: configure adopted UDP6 socket: %w", err)
		}
		if info.DualStack {
			return nil, fmt.Errorf("stdbind: adopted UDP6 socket has IPV6_V6ONLY=0 (dual-stack) -- reinstall wireguard-pro.socket with BindIPv6Only=ipv6-only and a separate ListenDatagram= for IPv4, then `make reload` (see docs/quickstart.md's upgrade note)")
		}
		s.info6 = *info
		port6 = p
	}
	if o.UDP4 != nil && o.UDP6 != nil && port4 != port6 {
		return nil, fmt.Errorf("stdbind: adopted UDP4 (port %d) and UDP6 (port %d) sockets are on different ports", port4, port6)
	}
	if o.UDP4 != nil {
		s.port = port4
	} else {
		s.port = port6
	}

	return s, nil
}

// Port returns the port this Bind's adopted socket(s) are bound to. Fixed
// at New; Open never changes it (see Open's doc comment).
func (s *StdNetBind) Port() uint16 {
	return s.port
}

// Description summarizes an adopted StdNetBind for the startup diagnostics
// line (docs/design-doc.md §4.4). String renders it as
// "port=51820 batch=128 kernel=6.8 v4[...] v6[...]".
type Description struct {
	Port      uint16
	BatchSize int
	Kernel    string      // kernel release, e.g. "6.8"; "n/a" on non-Linux
	IPv4      *FamilyInfo // nil if UDP4 wasn't adopted
	IPv6      *FamilyInfo // nil if UDP6 wasn't adopted
}

func (d Description) String() string {
	s := fmt.Sprintf("port=%d batch=%d kernel=%s", d.Port, d.BatchSize, d.Kernel)
	if d.IPv4 != nil {
		s += " v4[" + d.IPv4.describe() + "]"
	}
	if d.IPv6 != nil {
		s += " v6[" + d.IPv6.describe() + "]"
	}
	return s
}

func (f *FamilyInfo) describe() string {
	return fmt.Sprintf("%s gro=%s gso=%s rcvbuf=%d sndbuf=%d sticky=%s",
		f.LocalAddr, onOff(f.RxOffload), onOff(f.TxOffload), f.RcvBuf, f.SndBuf,
		onOff(f.PktInfo && StdNetSupportsStickySockets))
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// Describe reports this Bind's current configuration and, once Open has
// run, its negotiated offload state -- see the "bind:" startup diagnostics
// line in cmd/wireguard-pro/main.go.
func (s *StdNetBind) Describe() Description {
	s.mu.Lock()
	defer s.mu.Unlock()

	d := Description{
		Port:      s.port,
		BatchSize: s.BatchSize(),
		Kernel:    kernelVersionString(),
	}
	if s.file4 != nil {
		info := s.info4
		d.IPv4 = &info
	}
	if s.file6 != nil {
		info := s.info6
		d.IPv6 = &info
	}
	return d
}

type StdNetEndpoint struct {
	// AddrPort is the endpoint destination.
	netip.AddrPort
	// src is the current sticky source address and interface index, if
	// supported. Typically this is a PKTINFO structure from/for control
	// messages, see unix.PKTINFO for an example.
	src []byte
}

var (
	_ wgconn.Bind     = (*StdNetBind)(nil)
	_ wgconn.Endpoint = &StdNetEndpoint{}
)

func (*StdNetBind) ParseEndpoint(s string) (wgconn.Endpoint, error) {
	e, err := netip.ParseAddrPort(s)
	if err != nil {
		return nil, err
	}
	return &StdNetEndpoint{
		AddrPort: e,
	}, nil
}

func (e *StdNetEndpoint) ClearSrc() {
	if e.src != nil {
		// Truncate src, no need to reallocate.
		e.src = e.src[:0]
	}
}

func (e *StdNetEndpoint) DstIP() netip.Addr {
	return e.AddrPort.Addr()
}

// See control_default,linux, etc for implementations of SrcIP and SrcIfidx.

func (e *StdNetEndpoint) DstToBytes() []byte {
	b, _ := e.AddrPort.MarshalBinary()
	return b
}

func (e *StdNetEndpoint) DstToString() string {
	return e.AddrPort.String()
}

// adopt wraps an already-bound socket file as a *net.UDPConn.
// net.FilePacketConn dups the fd -- a fresh netpoller registration, with
// O_NONBLOCK set on the dup -- leaving f itself untouched for the next
// adopt call after a future Close/Open cycle; see the Close doc comment for
// why f is never closed itself.
func adopt(f *os.File) (*net.UDPConn, error) {
	pc, err := net.FilePacketConn(f)
	if err != nil {
		return nil, err
	}
	conn, ok := pc.(*net.UDPConn)
	if !ok {
		pc.Close()
		return nil, fmt.Errorf("stdbind: adopted file is a %T, not a UDP socket", pc)
	}
	return conn, nil
}

// Open adopts fresh dups of this Bind's UDP4/UDP6 sockets (see adopt) and
// returns their receive funcs -- unlike upstream, it never binds a new
// socket, so uport is only ever compared against the fixed, adopted port
// (see Port), not used to choose one: nothing in this codebase sets
// wgctrl's ListenPort, and doing so would surface as IpcErrorPortInUse
// rather than silently doing nothing.
func (s *StdNetBind) Open(uport uint16) ([]wgconn.ReceiveFunc, uint16, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ipv4 != nil || s.ipv6 != nil {
		return nil, 0, wgconn.ErrBindAlreadyOpen
	}

	if uport != 0 && uport != s.port && !s.warnedPort {
		s.warnedPort = true
		s.logf("stdbind: listen_port %d requested but socket-activated bind is fixed at %d", uport, s.port)
	}

	var v4conn, v6conn *net.UDPConn
	var v4pc *ipv4.PacketConn
	var v6pc *ipv6.PacketConn
	var err error

	if s.file4 != nil {
		v4conn, err = adopt(s.file4)
		if err != nil {
			return nil, 0, fmt.Errorf("stdbind: adopt UDP4 socket: %w", err)
		}
	}
	if s.file6 != nil {
		v6conn, err = adopt(s.file6)
		if err != nil {
			if v4conn != nil {
				v4conn.Close()
			}
			return nil, 0, fmt.Errorf("stdbind: adopt UDP6 socket: %w", err)
		}
	}

	var fns []wgconn.ReceiveFunc
	if v4conn != nil {
		s.ipv4TxOffload, s.ipv4RxOffload = supportsUDPOffload(v4conn)
		s.info4.TxOffload, s.info4.RxOffload = s.ipv4TxOffload, s.ipv4RxOffload
		if runtime.GOOS == "linux" || runtime.GOOS == "android" {
			v4pc = ipv4.NewPacketConn(v4conn)
			s.ipv4PC = v4pc
		}
		fns = append(fns, s.makeReceiveIPv4(v4pc, v4conn, s.ipv4RxOffload))
		s.ipv4 = v4conn
	}
	if v6conn != nil {
		s.ipv6TxOffload, s.ipv6RxOffload = supportsUDPOffload(v6conn)
		s.info6.TxOffload, s.info6.RxOffload = s.ipv6TxOffload, s.ipv6RxOffload
		if runtime.GOOS == "linux" || runtime.GOOS == "android" {
			v6pc = ipv6.NewPacketConn(v6conn)
			s.ipv6PC = v6pc
		}
		fns = append(fns, s.makeReceiveIPv6(v6pc, v6conn, s.ipv6RxOffload))
		s.ipv6 = v6conn
	}
	if len(fns) == 0 {
		return nil, 0, syscall.EAFNOSUPPORT
	}

	return fns, s.port, nil
}

func (s *StdNetBind) putMessages(msgs *[]ipv6.Message) {
	for i := range *msgs {
		(*msgs)[i].OOB = (*msgs)[i].OOB[:0]
		(*msgs)[i] = ipv6.Message{Buffers: (*msgs)[i].Buffers, OOB: (*msgs)[i].OOB}
	}
	s.msgsPool.Put(msgs)
}

func (s *StdNetBind) getMessages() *[]ipv6.Message {
	return s.msgsPool.Get().(*[]ipv6.Message)
}

var (
	// If compilation fails here these are no longer the same underlying type.
	_ ipv6.Message = ipv4.Message{}
)

type batchReader interface {
	ReadBatch([]ipv6.Message, int) (int, error)
}

type batchWriter interface {
	WriteBatch([]ipv6.Message, int) (int, error)
}

func (s *StdNetBind) receiveIP(
	br batchReader,
	conn *net.UDPConn,
	rxOffload bool,
	bufs [][]byte,
	sizes []int,
	eps []wgconn.Endpoint,
) (n int, err error) {
	msgs := s.getMessages()
	for i := range bufs {
		(*msgs)[i].Buffers[0] = bufs[i]
		(*msgs)[i].OOB = (*msgs)[i].OOB[:cap((*msgs)[i].OOB)]
	}
	defer s.putMessages(msgs)
	var numMsgs int
	if runtime.GOOS == "linux" || runtime.GOOS == "android" {
		if rxOffload {
			readAt := len(*msgs) - (wgconn.IdealBatchSize / udpSegmentMaxDatagrams)
			numMsgs, err = br.ReadBatch((*msgs)[readAt:], 0)
			if err != nil {
				return 0, err
			}
			numMsgs, err = splitCoalescedMessages(*msgs, readAt, getGSOSize)
			if err != nil {
				return 0, err
			}
		} else {
			numMsgs, err = br.ReadBatch(*msgs, 0)
			if err != nil {
				return 0, err
			}
		}
	} else {
		msg := &(*msgs)[0]
		msg.N, msg.NN, _, msg.Addr, err = conn.ReadMsgUDP(msg.Buffers[0], msg.OOB)
		if err != nil {
			return 0, err
		}
		numMsgs = 1
	}
	for i := 0; i < numMsgs; i++ {
		msg := &(*msgs)[i]
		sizes[i] = msg.N
		if sizes[i] == 0 {
			continue
		}
		addrPort := msg.Addr.(*net.UDPAddr).AddrPort()
		ep := &StdNetEndpoint{AddrPort: addrPort} // TODO: remove allocation
		getSrcFromControl(msg.OOB[:msg.NN], ep)
		eps[i] = ep
	}
	return numMsgs, nil
}

func (s *StdNetBind) makeReceiveIPv4(pc *ipv4.PacketConn, conn *net.UDPConn, rxOffload bool) wgconn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []wgconn.Endpoint) (n int, err error) {
		return s.receiveIP(pc, conn, rxOffload, bufs, sizes, eps)
	}
}

func (s *StdNetBind) makeReceiveIPv6(pc *ipv6.PacketConn, conn *net.UDPConn, rxOffload bool) wgconn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []wgconn.Endpoint) (n int, err error) {
		return s.receiveIP(pc, conn, rxOffload, bufs, sizes, eps)
	}
}

// TODO: When all Binds handle IdealBatchSize, remove this dynamic function and
// rename the IdealBatchSize constant to BatchSize.
func (s *StdNetBind) BatchSize() int {
	if runtime.GOOS == "linux" || runtime.GOOS == "android" {
		return wgconn.IdealBatchSize
	}
	return 1
}

// Close really closes the dup'd conns adopt created in Open -- unlike the
// FdBind this replaces, which never closed the fd it was handed at all
// (since a systemd-activated fd is handed to this process exactly once).
// That's safe here specifically because file4/file6 are never touched by
// Close: they're the pristine originals, so the next Open's adopt call
// dups a fresh, unclosed fd again. device.BindUpdate waits for old receive
// goroutines (netc.stopping.Wait()) before re-Open, and a closed dup makes
// ReadBatch return an error satisfying errors.Is(err, net.ErrClosed), so
// RoutineReceiveIncoming exits immediately -- no retry-sleep the way the
// old FdBind's read-deadline trick needed. Socket options live on the
// shared struct sock, so they persist across dups regardless.
func (s *StdNetBind) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err1, err2 error
	if s.ipv4 != nil {
		err1 = s.ipv4.Close()
		s.ipv4 = nil
		s.ipv4PC = nil
	}
	if s.ipv6 != nil {
		err2 = s.ipv6.Close()
		s.ipv6 = nil
		s.ipv6PC = nil
	}
	s.blackhole4 = false
	s.blackhole6 = false
	s.ipv4TxOffload = false
	s.ipv4RxOffload = false
	s.ipv6TxOffload = false
	s.ipv6RxOffload = false
	if err1 != nil {
		return err1
	}
	return err2
}

func (s *StdNetBind) Send(bufs [][]byte, endpoint wgconn.Endpoint) error {
	s.mu.Lock()
	blackhole := s.blackhole4
	conn := s.ipv4
	offload := s.ipv4TxOffload
	br := batchWriter(s.ipv4PC)
	is6 := false
	if endpoint.DstIP().Is6() {
		blackhole = s.blackhole6
		conn = s.ipv6
		br = s.ipv6PC
		is6 = true
		offload = s.ipv6TxOffload
	}
	s.mu.Unlock()

	if blackhole {
		return nil
	}
	if conn == nil {
		return syscall.EAFNOSUPPORT
	}

	msgs := s.getMessages()
	defer s.putMessages(msgs)
	ua := s.udpAddrPool.Get().(*net.UDPAddr)
	defer s.udpAddrPool.Put(ua)
	if is6 {
		as16 := endpoint.DstIP().As16()
		copy(ua.IP, as16[:])
		ua.IP = ua.IP[:16]
	} else {
		as4 := endpoint.DstIP().As4()
		copy(ua.IP, as4[:])
		ua.IP = ua.IP[:4]
	}
	ua.Port = int(endpoint.(*StdNetEndpoint).Port())
	var (
		retried bool
		err     error
	)
retry:
	if offload {
		n := coalesceMessages(ua, endpoint.(*StdNetEndpoint), bufs, *msgs, setGSOSize)
		err = s.send(conn, br, (*msgs)[:n])
		if err != nil && offload && errShouldDisableUDPGSO(err) {
			offload = false
			s.mu.Lock()
			if is6 {
				s.ipv6TxOffload = false
				s.info6.TxOffload = false
			} else {
				s.ipv4TxOffload = false
				s.info4.TxOffload = false
			}
			s.mu.Unlock()
			retried = true
			goto retry
		}
	} else {
		for i := range bufs {
			(*msgs)[i].Addr = ua
			(*msgs)[i].Buffers[0] = bufs[i]
			setSrcControl(&(*msgs)[i].OOB, endpoint.(*StdNetEndpoint))
		}
		err = s.send(conn, br, (*msgs)[:len(bufs)])
	}
	if retried {
		// Upstream's own conn.ErrUDPGSODisabled carries the local address
		// in an unexported field we can't set from here (see its Error()
		// method), so log it ourselves before returning the error.
		s.logf("stdbind: disabling UDP GSO on %s, NIC(s) may not support checksum offload: %v", conn.LocalAddr(), err)
		return wgconn.ErrUDPGSODisabled{RetryErr: err}
	}
	return err
}

func (s *StdNetBind) send(conn *net.UDPConn, pc batchWriter, msgs []ipv6.Message) error {
	var (
		n     int
		err   error
		start int
	)
	if runtime.GOOS == "linux" || runtime.GOOS == "android" {
		for {
			n, err = pc.WriteBatch(msgs[start:], 0)
			if err != nil || n == len(msgs[start:]) {
				break
			}
			start += n
		}
	} else {
		for _, msg := range msgs {
			_, _, err = conn.WriteMsgUDP(msg.Buffers[0], msg.OOB, msg.Addr.(*net.UDPAddr))
			if err != nil {
				break
			}
		}
	}
	return err
}

const (
	// Exceeding these values results in EMSGSIZE. They account for layer3 and
	// layer4 headers. IPv6 does not need to account for itself as the payload
	// length field is self excluding.
	maxIPv4PayloadLen = 1<<16 - 1 - 20 - 8
	maxIPv6PayloadLen = 1<<16 - 1 - 8

	// This is a hard limit imposed by the kernel.
	udpSegmentMaxDatagrams = 64
)

type setGSOFunc func(control *[]byte, gsoSize uint16)

func coalesceMessages(addr *net.UDPAddr, ep *StdNetEndpoint, bufs [][]byte, msgs []ipv6.Message, setGSO setGSOFunc) int {
	var (
		base     = -1 // index of msg we are currently coalescing into
		gsoSize  int  // segmentation size of msgs[base]
		dgramCnt int  // number of dgrams coalesced into msgs[base]
		endBatch bool // tracking flag to start a new batch on next iteration of bufs
	)
	maxPayloadLen := maxIPv4PayloadLen
	if ep.DstIP().Is6() {
		maxPayloadLen = maxIPv6PayloadLen
	}
	for i, buf := range bufs {
		if i > 0 {
			msgLen := len(buf)
			baseLenBefore := len(msgs[base].Buffers[0])
			freeBaseCap := cap(msgs[base].Buffers[0]) - baseLenBefore
			if msgLen+baseLenBefore <= maxPayloadLen &&
				msgLen <= gsoSize &&
				msgLen <= freeBaseCap &&
				dgramCnt < udpSegmentMaxDatagrams &&
				!endBatch {
				msgs[base].Buffers[0] = append(msgs[base].Buffers[0], buf...)
				if i == len(bufs)-1 {
					setGSO(&msgs[base].OOB, uint16(gsoSize))
				}
				dgramCnt++
				if msgLen < gsoSize {
					// A smaller than gsoSize packet on the tail is legal, but
					// it must end the batch.
					endBatch = true
				}
				continue
			}
		}
		if dgramCnt > 1 {
			setGSO(&msgs[base].OOB, uint16(gsoSize))
		}
		// Reset prior to incrementing base since we are preparing to start a
		// new potential batch.
		endBatch = false
		base++
		gsoSize = len(buf)
		setSrcControl(&msgs[base].OOB, ep)
		msgs[base].Buffers[0] = buf
		msgs[base].Addr = addr
		dgramCnt = 1
	}
	return base + 1
}

type getGSOFunc func(control []byte) (int, error)

func splitCoalescedMessages(msgs []ipv6.Message, firstMsgAt int, getGSO getGSOFunc) (n int, err error) {
	for i := firstMsgAt; i < len(msgs); i++ {
		msg := &msgs[i]
		if msg.N == 0 {
			return n, err
		}
		var (
			gsoSize    int
			start      int
			end        = msg.N
			numToSplit = 1
		)
		gsoSize, err = getGSO(msg.OOB[:msg.NN])
		if err != nil {
			return n, err
		}
		if gsoSize > 0 {
			numToSplit = (msg.N + gsoSize - 1) / gsoSize
			end = gsoSize
		}
		for j := 0; j < numToSplit; j++ {
			if n > i {
				return n, errors.New("splitting coalesced packet resulted in overflow")
			}
			copied := copy(msgs[n].Buffers[0], msg.Buffers[0][start:end])
			msgs[n].N = copied
			msgs[n].Addr = msg.Addr
			start = end
			end += gsoSize
			if end > msg.N {
				end = msg.N
			}
			n++
		}
		if i != n-1 {
			// It is legal for bytes to move within msg.Buffers[0] as a result
			// of splitting, so we only zero the source msg len when it is not
			// the destination of the last split operation above.
			msg.N = 0
		}
	}
	return n, nil
}
