/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2017-2025 WireGuard LLC. All Rights Reserved.
 */

// Forked from golang.zx2c4.com/wireguard@v0.0.0-20260522210424-ecfc5a8d5446/conn/bind_std_test.go, see README.md.
//
// TestStdNetBindReceiveFuncAfterClose is replaced by the adoption-based
// suite below (newInherited, TestOpenCloseCycles, etc.), since it relied on
// upstream's NewStdNetBind/self-binding Open, which this fork doesn't have.
// Test_coalesceMessages, Test_splitCoalescedMessages, and the mock helpers
// are verbatim.

package stdbind

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/ipv6"

	wgconn "golang.zx2c4.com/wireguard/conn"
)

// newInherited builds an already-bound udp4 127.0.0.1:0 socket, plus a
// best-effort udp6 [::1]:<same port> socket (skipped, not failed, if IPv6
// loopback is unavailable), as the *os.File pair New expects -- letting
// tests exercise adoption without any real systemd activation.
func newInherited(t *testing.T) Options {
	t.Helper()

	c4, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen udp4: %v", err)
	}
	t.Cleanup(func() { c4.Close() })
	f4, err := c4.File()
	if err != nil {
		t.Fatalf("dup udp4 to file: %v", err)
	}
	t.Cleanup(func() { f4.Close() })

	port := c4.LocalAddr().(*net.UDPAddr).Port
	o := Options{UDP4: f4}

	c6, err := net.ListenUDP("udp6", &net.UDPAddr{IP: net.IPv6loopback, Port: port})
	if err != nil {
		t.Logf("best-effort udp6 [::1]:%d unavailable, testing UDP4-only: %v", port, err)
		return o
	}
	t.Cleanup(func() { c6.Close() })
	f6, err := c6.File()
	if err != nil {
		t.Fatalf("dup udp6 to file: %v", err)
	}
	t.Cleanup(func() { f6.Close() })

	o.UDP6 = f6
	return o
}

func TestOpenCloseCycles(t *testing.T) {
	s, err := New(newInherited(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var firstPort uint16
	for i := 0; i < 5; i++ {
		fns, port, err := s.Open(0)
		if err != nil {
			t.Fatalf("Open #%d: %v", i, err)
		}
		if len(fns) == 0 {
			t.Fatalf("Open #%d: no receive funcs", i)
		}
		if i == 0 {
			firstPort = port
		} else if port != firstPort {
			t.Errorf("Open #%d: port %d, want %d (stable across cycles)", i, port, firstPort)
		}

		if _, _, err := s.Open(0); !errors.Is(err, wgconn.ErrBindAlreadyOpen) {
			t.Errorf("Open #%d while already open: err = %v, want ErrBindAlreadyOpen", i, err)
		}

		if err := s.Close(); err != nil {
			t.Fatalf("Close #%d: %v", i, err)
		}
		if err := s.Close(); err != nil {
			t.Errorf("second Close #%d: %v, want nil", i, err)
		}
	}
}

func TestReceiveFuncReturnsErrClosed(t *testing.T) {
	s, err := New(newInherited(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	fns, _, err := s.Open(0)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	fn := fns[0]

	batch := s.BatchSize()
	bufs := make([][]byte, batch)
	for i := range bufs {
		bufs[i] = make([]byte, 1500)
	}
	sizes := make([]int, batch)
	eps := make([]wgconn.Endpoint, batch)

	done := make(chan error, 1)
	go func() {
		_, err := fn(bufs, sizes, eps)
		done <- err
	}()

	// Give the receive goroutine a moment to actually block in the read
	// before closing, so this exercises the "unblock an in-flight read"
	// path rather than racing Close.
	time.Sleep(20 * time.Millisecond)
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, net.ErrClosed) {
			t.Errorf("receive func returned %v, want an error satisfying net.ErrClosed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("receive func did not return within 2s of Close")
	}
}

func TestSendReceiveBatch(t *testing.T) {
	s, err := New(newInherited(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	fns, port, err := s.Open(0)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	var recvV4 wgconn.ReceiveFunc
	for _, fn := range fns {
		if fn.PrettyName() == "v4" {
			recvV4 = fn
		}
	}
	if recvV4 == nil {
		t.Fatal("Open returned no v4 receive func")
	}

	// A plain, unrelated UDP socket plays "the peer": it sends datagrams
	// into the bind under test, and receives what Send writes back out.
	peer, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen peer udp4: %v", err)
	}
	defer peer.Close()
	bindAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: int(port)}

	const inN = 64
	for i := 0; i < inN; i++ {
		if _, err := peer.WriteToUDP([]byte(fmt.Sprintf("pkt-%d", i)), bindAddr); err != nil {
			t.Fatalf("peer write #%d: %v", i, err)
		}
	}

	batch := s.BatchSize()
	bufs := make([][]byte, batch)
	for i := range bufs {
		bufs[i] = make([]byte, 1500)
	}
	sizes := make([]int, batch)
	eps := make([]wgconn.Endpoint, batch)

	received, withSrcIP := 0, 0
	deadline := time.Now().Add(5 * time.Second)
	for received < inN && time.Now().Before(deadline) {
		cnt, err := recvV4(bufs, sizes, eps)
		if err != nil {
			t.Fatalf("receive func: %v", err)
		}
		for i := 0; i < cnt; i++ {
			if sizes[i] == 0 {
				continue
			}
			if got := string(bufs[i][:sizes[i]]); !strings.HasPrefix(got, "pkt-") {
				t.Errorf("unexpected payload %q", got)
			}
			if eps[i].DstIP() != netip.MustParseAddr("127.0.0.1") {
				t.Errorf("endpoint DstIP = %v, want 127.0.0.1", eps[i].DstIP())
			}
			if runtime.GOOS == "linux" && eps[i].SrcIP().IsValid() {
				withSrcIP++
			}
			received++
		}
	}
	if runtime.GOOS == "linux" {
		// Not asserted: whenever rxOffload/GRO is active (true here, and
		// true for any real deployment -- that's the whole point of this
		// fork), every physically-read message goes through
		// splitCoalescedMessages, which -- verbatim upstream, see its doc
		// comment -- never copies OOB/PKTINFO into the sub-messages it
		// splits out (their destination index never lines up with the
		// physical read's index once GRO batches multiple datagrams per
		// syscall). So withSrcIP is legitimately 0 here, structurally, not
		// just when traffic happens to coalesce. Accepted per
		// docs/design-doc.md §4.1's sticky-socket note: this deployment is
		// single-homed (one pasta netns interface), so SrcIP precision on
		// received packets doesn't affect where replies go out.
		t.Logf("%d/%d received datagrams had a populated SrcIP (rxOffload=%v)", withSrcIP, received, s.ipv4RxOffload)
	}
	if received != inN {
		t.Fatalf("received %d/%d datagrams before the 5s deadline", received, inN)
	}

	// Send 32 buffers in one call -- exercises coalesceMessages when GSO
	// offload is active, or the per-buffer fallback otherwise; either way
	// the peer must see 32 separate datagrams.
	sendDst, err := s.ParseEndpoint(peer.LocalAddr().String())
	if err != nil {
		t.Fatalf("ParseEndpoint(peer): %v", err)
	}

	const outN = 32
	sendBufs := make([][]byte, outN)
	for i := range sendBufs {
		sendBufs[i] = []byte(fmt.Sprintf("out-%02d", i))
	}
	if err := s.Send(sendBufs, sendDst); err != nil {
		t.Fatalf("Send: %v", err)
	}

	peer.SetReadDeadline(time.Now().Add(2 * time.Second))
	readBuf := make([]byte, 1500)
	for got := 0; got < outN; got++ {
		if _, _, err := peer.ReadFromUDP(readBuf); err != nil {
			t.Fatalf("peer read #%d/%d: %v", got, outN, err)
		}
	}
}

func TestOpenIgnoresPort(t *testing.T) {
	s, err := New(newInherited(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	adoptedPort := s.Port()

	var mu sync.Mutex
	var warnings int
	s.logf = func(format string, args ...any) {
		mu.Lock()
		warnings++
		mu.Unlock()
		t.Logf(format, args...)
	}

	for i := 0; i < 3; i++ {
		_, port, err := s.Open(adoptedPort + 1)
		if err != nil {
			t.Fatalf("Open #%d: %v", i, err)
		}
		if port != adoptedPort {
			t.Errorf("Open #%d returned port %d, want the adopted socket's port %d", i, port, adoptedPort)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("Close #%d: %v", i, err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if warnings != 1 {
		t.Errorf("warned %d times across 3 Opens with a mismatched requested port, want exactly 1", warnings)
	}
}

func TestNoGoroutineLeak(t *testing.T) {
	s, err := New(newInherited(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	before := runtime.NumGoroutine()
	for i := 0; i < 3; i++ {
		if _, _, err := s.Open(0); err != nil {
			t.Fatalf("Open #%d: %v", i, err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("Close #%d: %v", i, err)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		if n := runtime.NumGoroutine(); n <= before+2 { // small slack for GC/runtime bookkeeping
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("goroutine count grew from %d to %d after 3 Open/Close cycles", before, runtime.NumGoroutine())
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func mockSetGSOSize(control *[]byte, gsoSize uint16) {
	*control = (*control)[:cap(*control)]
	binary.LittleEndian.PutUint16(*control, gsoSize)
}

func Test_coalesceMessages(t *testing.T) {
	cases := []struct {
		name     string
		buffs    [][]byte
		wantLens []int
		wantGSO  []int
	}{
		{
			name: "one message no coalesce",
			buffs: [][]byte{
				make([]byte, 1, 1),
			},
			wantLens: []int{1},
			wantGSO:  []int{0},
		},
		{
			name: "two messages equal len coalesce",
			buffs: [][]byte{
				make([]byte, 1, 2),
				make([]byte, 1, 1),
			},
			wantLens: []int{2},
			wantGSO:  []int{1},
		},
		{
			name: "two messages unequal len coalesce",
			buffs: [][]byte{
				make([]byte, 2, 3),
				make([]byte, 1, 1),
			},
			wantLens: []int{3},
			wantGSO:  []int{2},
		},
		{
			name: "three messages second unequal len coalesce",
			buffs: [][]byte{
				make([]byte, 2, 3),
				make([]byte, 1, 1),
				make([]byte, 2, 2),
			},
			wantLens: []int{3, 2},
			wantGSO:  []int{2, 0},
		},
		{
			name: "three messages limited cap coalesce",
			buffs: [][]byte{
				make([]byte, 2, 4),
				make([]byte, 2, 2),
				make([]byte, 2, 2),
			},
			wantLens: []int{4, 2},
			wantGSO:  []int{2, 0},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			addr := &net.UDPAddr{
				IP:   net.ParseIP("127.0.0.1").To4(),
				Port: 1,
			}
			msgs := make([]ipv6.Message, len(tt.buffs))
			for i := range msgs {
				msgs[i].Buffers = make([][]byte, 1)
				msgs[i].OOB = make([]byte, 0, 2)
			}
			got := coalesceMessages(addr, &StdNetEndpoint{AddrPort: addr.AddrPort()}, tt.buffs, msgs, mockSetGSOSize)
			if got != len(tt.wantLens) {
				t.Fatalf("got len %d want: %d", got, len(tt.wantLens))
			}
			for i := 0; i < got; i++ {
				if msgs[i].Addr != addr {
					t.Errorf("msgs[%d].Addr != passed addr", i)
				}
				gotLen := len(msgs[i].Buffers[0])
				if gotLen != tt.wantLens[i] {
					t.Errorf("len(msgs[%d].Buffers[0]) %d != %d", i, gotLen, tt.wantLens[i])
				}
				gotGSO, err := mockGetGSOSize(msgs[i].OOB)
				if err != nil {
					t.Fatalf("msgs[%d] getGSOSize err: %v", i, err)
				}
				if gotGSO != tt.wantGSO[i] {
					t.Errorf("msgs[%d] gsoSize %d != %d", i, gotGSO, tt.wantGSO[i])
				}
			}
		})
	}
}

func mockGetGSOSize(control []byte) (int, error) {
	if len(control) < 2 {
		return 0, nil
	}
	return int(binary.LittleEndian.Uint16(control)), nil
}

func Test_splitCoalescedMessages(t *testing.T) {
	newMsg := func(n, gso int) ipv6.Message {
		msg := ipv6.Message{
			Buffers: [][]byte{make([]byte, 1<<16-1)},
			N:       n,
			OOB:     make([]byte, 2),
		}
		binary.LittleEndian.PutUint16(msg.OOB, uint16(gso))
		if gso > 0 {
			msg.NN = 2
		}
		return msg
	}

	cases := []struct {
		name        string
		msgs        []ipv6.Message
		firstMsgAt  int
		wantNumEval int
		wantMsgLens []int
		wantErr     bool
	}{
		{
			name: "second last split last empty",
			msgs: []ipv6.Message{
				newMsg(0, 0),
				newMsg(0, 0),
				newMsg(3, 1),
				newMsg(0, 0),
			},
			firstMsgAt:  2,
			wantNumEval: 3,
			wantMsgLens: []int{1, 1, 1, 0},
			wantErr:     false,
		},
		{
			name: "second last no split last empty",
			msgs: []ipv6.Message{
				newMsg(0, 0),
				newMsg(0, 0),
				newMsg(1, 0),
				newMsg(0, 0),
			},
			firstMsgAt:  2,
			wantNumEval: 1,
			wantMsgLens: []int{1, 0, 0, 0},
			wantErr:     false,
		},
		{
			name: "second last no split last no split",
			msgs: []ipv6.Message{
				newMsg(0, 0),
				newMsg(0, 0),
				newMsg(1, 0),
				newMsg(1, 0),
			},
			firstMsgAt:  2,
			wantNumEval: 2,
			wantMsgLens: []int{1, 1, 0, 0},
			wantErr:     false,
		},
		{
			name: "second last no split last split",
			msgs: []ipv6.Message{
				newMsg(0, 0),
				newMsg(0, 0),
				newMsg(1, 0),
				newMsg(3, 1),
			},
			firstMsgAt:  2,
			wantNumEval: 4,
			wantMsgLens: []int{1, 1, 1, 1},
			wantErr:     false,
		},
		{
			name: "second last split last split",
			msgs: []ipv6.Message{
				newMsg(0, 0),
				newMsg(0, 0),
				newMsg(2, 1),
				newMsg(2, 1),
			},
			firstMsgAt:  2,
			wantNumEval: 4,
			wantMsgLens: []int{1, 1, 1, 1},
			wantErr:     false,
		},
		{
			name: "second last no split last split overflow",
			msgs: []ipv6.Message{
				newMsg(0, 0),
				newMsg(0, 0),
				newMsg(1, 0),
				newMsg(4, 1),
			},
			firstMsgAt:  2,
			wantNumEval: 4,
			wantMsgLens: []int{1, 1, 1, 1},
			wantErr:     true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitCoalescedMessages(tt.msgs, 2, mockGetGSOSize)
			if err != nil && !tt.wantErr {
				t.Fatalf("err: %v", err)
			}
			if got != tt.wantNumEval {
				t.Fatalf("got to eval: %d want: %d", got, tt.wantNumEval)
			}
			for i, msg := range tt.msgs {
				if msg.N != tt.wantMsgLens[i] {
					t.Fatalf("msg[%d].N: %d want: %d", i, msg.N, tt.wantMsgLens[i])
				}
			}
		})
	}
}
