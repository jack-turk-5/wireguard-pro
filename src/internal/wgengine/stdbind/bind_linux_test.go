//go:build linux

package stdbind

import (
	"os"
	"testing"

	"github.com/vishvananda/netlink"
)

// TestMain brings lo up when running as root: hack/testrun's CLONE_NEWNET
// namespace starts with lo down, which would otherwise fail every test here
// that dials 127.0.0.1/[::1] (see e2e_test.go for the same pattern). A no-op
// outside that namespace (euid != 0, or lo already up).
func TestMain(m *testing.M) {
	if os.Geteuid() == 0 {
		if link, err := netlink.LinkByName("lo"); err == nil {
			_ = netlink.LinkSetUp(link)
		}
	}
	os.Exit(m.Run())
}

// TestDescribe checks that Describe reports offload/dual-stack flags
// consistent with what New/Open actually found on this kernel -- the flags
// themselves are allowed to be off (e.g. UDP_GRO needs kernel >= 5.12), but
// Describe must accurately reflect whatever applySockopts/supportsUDPOffload
// set, not a hardcoded expectation.
func TestDescribe(t *testing.T) {
	o := newInherited(t)
	s, err := New(o)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, _, err := s.Open(0); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	d := s.Describe()
	if d.Port != s.Port() {
		t.Errorf("Describe Port = %d, want %d", d.Port, s.Port())
	}
	if d.Kernel == "" {
		t.Error("Describe Kernel is empty")
	}

	if d.IPv4 == nil {
		t.Fatal("Describe IPv4 is nil, want non-nil (UDP4 was adopted)")
	}
	if d.IPv4.RxOffload != s.ipv4RxOffload || d.IPv4.TxOffload != s.ipv4TxOffload {
		t.Errorf("Describe IPv4 offload (rx=%v tx=%v) doesn't match what Open found (rx=%v tx=%v)",
			d.IPv4.RxOffload, d.IPv4.TxOffload, s.ipv4RxOffload, s.ipv4TxOffload)
	}
	if d.IPv4.DualStack {
		t.Error("Describe IPv4 DualStack = true, meaningless for an AF_INET socket")
	}

	if o.UDP6 != nil {
		if d.IPv6 == nil {
			t.Fatal("Describe IPv6 is nil, want non-nil (UDP6 was adopted)")
		}
		if d.IPv6.DualStack {
			t.Error("Describe IPv6 DualStack = true for an explicit udp6 (v6-only) socket")
		}
		if d.IPv6.RxOffload != s.ipv6RxOffload || d.IPv6.TxOffload != s.ipv6TxOffload {
			t.Errorf("Describe IPv6 offload (rx=%v tx=%v) doesn't match what Open found (rx=%v tx=%v)",
				d.IPv6.RxOffload, d.IPv6.TxOffload, s.ipv6RxOffload, s.ipv6TxOffload)
		}
	} else if d.IPv6 != nil {
		t.Error("Describe IPv6 is non-nil, but UDP6 was never adopted (best-effort v6 unavailable in this env)")
	}

	t.Logf("Describe(): %s", d)
}
