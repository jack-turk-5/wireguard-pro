package nft

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/google/nftables"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// TestForwardChain_RejectsPrivateDestinationBeforeAccept is a real-traffic
// regression test for a rule-ordering bug found while reviewing the
// production ruleset: netfilter's FORWARD hook fires *after* the routing
// decision, so a packet's oifname is already fixed by the time forward-chain
// rules evaluate. The original hand-written nftables.conf placed the
// wg0->tap* accept rule *before* the private-range reject rule; since every
// forwarded packet from wg0 has oifname "tap0" (the only egress interface),
// the accept rule matched first for everything and the reject rule was
// unreachable dead code. container/nftables.json now orders the reject
// rules first -- verified here for real, not just by re-reading the JSON:
// a UDP packet sent from a client namespace, through a veth literally named
// "wg0", into a gateway namespace running the actual production ruleset
// toward an address inside a blocked private range, must come back with an
// ICMP port-unreachable (what the reject verdict generates). An accept
// would instead silently forward the packet into the void beyond the test's
// fake "tap0" with no response at all.
func TestForwardChain_RejectsPrivateDestinationBeforeAccept(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	orig, err := netns.Get()
	if err != nil {
		t.Skipf("skipping: cannot get current network namespace: %v", err)
	}
	defer orig.Close()

	clientNS, err := netns.New()
	if err != nil {
		t.Skipf("skipping: creating a test network namespace requires CAP_NET_ADMIN (run via `go -C hack/testrun run .`): %v", err)
	}
	defer clientNS.Close()
	defer netns.Set(orig)

	if err := netns.Set(orig); err != nil {
		t.Fatalf("netns.Set(orig): %v", err)
	}
	gwNS, err := netns.New()
	if err != nil {
		t.Fatalf("create gateway namespace: %v", err)
	}
	defer gwNS.Close()

	if err := netns.Set(orig); err != nil {
		t.Fatalf("netns.Set(orig): %v", err)
	}

	// wg0 (client-facing) <-> veth-client, matching production's inbound leg.
	wg0Veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: "wg0"},
		PeerName:  "veth-client",
	}
	if err := netlink.LinkAdd(wg0Veth); err != nil {
		t.Fatalf("add wg0/veth-client pair: %v", err)
	}
	wg0Link, err := netlink.LinkByName("wg0")
	if err != nil {
		t.Fatalf("find wg0: %v", err)
	}
	clientLink, err := netlink.LinkByName("veth-client")
	if err != nil {
		t.Fatalf("find veth-client: %v", err)
	}
	if err := netlink.LinkSetNsFd(wg0Link, int(gwNS)); err != nil {
		t.Fatalf("move wg0 into gateway namespace: %v", err)
	}
	if err := netlink.LinkSetNsFd(clientLink, int(clientNS)); err != nil {
		t.Fatalf("move veth-client into client namespace: %v", err)
	}

	// tap0 (upstream-facing), matching production's egress leg. The peer
	// just needs to exist as a real link for routing/oifname purposes --
	// nothing needs to actually be reachable through it for this test.
	tap0Veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: "tap0"},
		PeerName:  "veth-upstream",
	}
	if err := netlink.LinkAdd(tap0Veth); err != nil {
		t.Fatalf("add tap0/veth-upstream pair: %v", err)
	}
	tap0Link, err := netlink.LinkByName("tap0")
	if err != nil {
		t.Fatalf("find tap0: %v", err)
	}
	if err := netlink.LinkSetNsFd(tap0Link, int(gwNS)); err != nil {
		t.Fatalf("move tap0 into gateway namespace: %v", err)
	}

	gwHandle, err := netlink.NewHandleAt(gwNS)
	if err != nil {
		t.Fatalf("netlink handle for gateway namespace: %v", err)
	}
	defer gwHandle.Close()
	clientHandle, err := netlink.NewHandleAt(clientNS)
	if err != nil {
		t.Fatalf("netlink handle for client namespace: %v", err)
	}
	defer clientHandle.Close()

	// 192.0.2.0/30 (TEST-NET-1, RFC 5737) for the client<->wg0 link -- kept
	// well outside every range the reject rule blocks, so it can't be
	// confused with the traffic under test.
	setupLink(t, gwHandle, "wg0", "192.0.2.1/30")
	setupLink(t, clientHandle, "veth-client", "192.0.2.2/30")
	setupLink(t, gwHandle, "tap0", "198.51.100.1/24") // TEST-NET-2, RFC 5737
	if err := gwHandle.LinkSetUp(mustLink(t, gwHandle, "lo")); err != nil {
		t.Fatalf("set gateway lo up: %v", err)
	}

	// Route the blocked test destination out tap0, exactly like production
	// routes all non-wg0-local traffic out the single egress interface.
	blockedDestNet := mustParseCIDR(t, "10.0.0.0/8")
	if err := gwHandle.RouteAdd(&netlink.Route{
		LinkIndex: mustLink(t, gwHandle, "tap0").Attrs().Index,
		Dst:       blockedDestNet,
	}); err != nil {
		t.Fatalf("add route for blocked destination via tap0: %v", err)
	}
	if err := clientHandle.RouteAdd(&netlink.Route{
		LinkIndex: mustLink(t, clientHandle, "veth-client").Attrs().Index,
		Dst:       blockedDestNet,
		Gw:        net.ParseIP("192.0.2.1"),
	}); err != nil {
		t.Fatalf("add client route to gateway: %v", err)
	}

	setNS(t, gwNS)
	err = os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0o644)
	setNS(t, orig)
	if err != nil {
		// /proc/sys's mount policy inside the outer container varies by
		// container runtime/version -- some mount it read-only even within
		// a nested namespace that otherwise has CAP_NET_ADMIN. Not something
		// this test can control, so skip cleanly rather than fail.
		t.Skipf("skipping: cannot enable ip_forward in the gateway namespace (read-only /proc/sys under this container runtime): %v", err)
	}

	conn, err := nftables.New(nftables.WithNetNSFd(int(gwNS)))
	if err != nil {
		t.Fatalf("nftables.New for gateway namespace: %v", err)
	}
	rulesetPath := filepath.Join(repoRoot(t), "container", "nftables.json")
	if err := applyFile(conn, rulesetPath); err != nil {
		t.Fatalf("apply %s: %v", rulesetPath, err)
	}

	setNS(t, clientNS)
	udpConn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP("10.0.0.5"), Port: 53})
	if err != nil {
		t.Fatalf("dial blocked destination: %v", err)
	}
	defer udpConn.Close()
	if _, err := udpConn.Write([]byte("test")); err != nil {
		t.Fatalf("write test packet: %v", err)
	}

	// The ICMP port-unreachable the reject verdict generates arrives
	// asynchronously; Linux surfaces it as an error on the connected UDP
	// socket's next syscall, which may be this Write retry or the Read.
	udpConn.SetDeadline(time.Now().Add(3 * time.Second))
	_, writeErr := udpConn.Write([]byte("test"))
	buf := make([]byte, 16)
	_, readErr := udpConn.Read(buf)
	setNS(t, orig)

	if !isConnRefused(writeErr) && !isConnRefused(readErr) {
		t.Fatalf("expected ICMP port-unreachable (connection refused) for a private-range destination through wg0, got write err=%v read err=%v -- the reject rule did not win, meaning the accept rule matched first (the exact bug this test guards against)", writeErr, readErr)
	}
}

func isConnRefused(err error) bool {
	return err != nil && errors.Is(err, syscall.ECONNREFUSED)
}

func setupLink(t *testing.T, h *netlink.Handle, name, cidr string) {
	t.Helper()
	link := mustLink(t, h, name)
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		t.Fatalf("parse address %q: %v", cidr, err)
	}
	if err := h.AddrAdd(link, addr); err != nil {
		t.Fatalf("add address %q to %s: %v", cidr, name, err)
	}
	if err := h.LinkSetUp(link); err != nil {
		t.Fatalf("set %s up: %v", name, err)
	}
}

func mustLink(t *testing.T, h *netlink.Handle, name string) netlink.Link {
	t.Helper()
	link, err := h.LinkByName(name)
	if err != nil {
		t.Fatalf("find %s: %v", name, err)
	}
	return link
}

func mustParseCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("parse CIDR %q: %v", cidr, err)
	}
	return n
}

func setNS(t *testing.T, ns netns.NsHandle) {
	t.Helper()
	if err := netns.Set(ns); err != nil {
		t.Fatalf("netns.Set: %v", err)
	}
}

// repoRoot locates this repo's root directory (parent of src/), from this
// source file's own build-time path.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}
