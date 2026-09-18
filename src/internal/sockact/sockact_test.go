package sockact

import (
	"math/rand"
	"net"
	"os"
	"testing"
)

func fileFromListener(t *testing.T, ln net.Listener) *os.File {
	t.Helper()
	tcpLn, ok := ln.(*net.TCPListener)
	if !ok {
		t.Fatalf("listener is %T, want *net.TCPListener", ln)
	}
	f, err := tcpLn.File()
	if err != nil {
		t.Fatalf("dup listener to file: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func fileFromUDPConn(t *testing.T, c *net.UDPConn) *os.File {
	t.Helper()
	f, err := c.File()
	if err != nil {
		t.Fatalf("dup UDP conn to file: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// TestClassify feeds classify a real tcp4 listener, udp4 socket, and
// explicit (v6-only) udp6 socket in shuffled order, and checks each lands in
// the right Sockets field regardless of order.
func TestClassify(t *testing.T) {
	tcpLn, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp4: %v", err)
	}
	defer tcpLn.Close()

	udp4Conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen udp4: %v", err)
	}
	defer udp4Conn.Close()

	udp6Conn, err := net.ListenUDP("udp6", &net.UDPAddr{IP: net.IPv6loopback})
	if err != nil {
		t.Skipf("listen udp6 (v6-only) unavailable in this environment: %v", err)
	}
	defer udp6Conn.Close()

	files := []*os.File{
		fileFromListener(t, tcpLn),
		fileFromUDPConn(t, udp4Conn),
		fileFromUDPConn(t, udp6Conn),
	}
	rand.Shuffle(len(files), func(i, j int) { files[i], files[j] = files[j], files[i] })

	s, err := classify(files)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}

	if len(s.Listeners) != 1 {
		t.Errorf("Listeners = %d entries, want 1", len(s.Listeners))
	}
	if s.UDP4 == nil {
		t.Error("UDP4 not set")
	}
	if s.UDP6 == nil {
		t.Fatal("UDP6 not set")
	}
	if s.UDP6DualStack {
		t.Error("UDP6DualStack = true for an explicit udp6 (v6-only) socket")
	}
	if !s.Activated() {
		t.Error("Activated() = false, want true")
	}
	if len(s.Names) != len(files) {
		t.Errorf("Names has %d entries, want %d", len(s.Names), len(files))
	}
}

// TestClassifyDualStackUDP6 checks UDP6DualStack against a socket bound on
// the unversioned "udp" network, which Go leaves IPV6_V6ONLY=0 on.
func TestClassifyDualStackUDP6(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv6unspecified})
	if err != nil {
		t.Skipf("listen dual-stack udp unavailable in this environment: %v", err)
	}
	defer conn.Close()

	s, err := classify([]*os.File{fileFromUDPConn(t, conn)})
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if s.UDP6 == nil {
		t.Fatal("UDP6 not set")
	}
	if !s.UDP6DualStack {
		t.Error("UDP6DualStack = false, want true for an IPV6_V6ONLY=0 socket")
	}
}

// TestClassifyDuplicateFamily checks that two UDP4 sockets is an error
// rather than silently keeping one.
func TestClassifyDuplicateFamily(t *testing.T) {
	a, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen udp4 a: %v", err)
	}
	defer a.Close()
	b, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen udp4 b: %v", err)
	}
	defer b.Close()

	files := []*os.File{fileFromUDPConn(t, a), fileFromUDPConn(t, b)}
	if _, err := classify(files); err == nil {
		t.Fatal("classify with two UDP4 sockets: want error, got nil")
	}
}

// TestLoadNotActivated checks that Load, run outside of systemd activation
// (no LISTEN_PID/LISTEN_FDS in the environment), returns a valid,
// non-activated Sockets rather than an error -- the local-dev path.
func TestLoadNotActivated(t *testing.T) {
	t.Setenv("LISTEN_PID", "")
	t.Setenv("LISTEN_FDS", "")

	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Activated() {
		t.Error("Activated() = true without systemd activation env vars set")
	}
}
