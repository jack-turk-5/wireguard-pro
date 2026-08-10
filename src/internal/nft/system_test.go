package nft

import (
	"runtime"
	"testing"

	"github.com/google/nftables"
	"github.com/vishvananda/netns"
)

// newSystemConn returns a netlink connection to a fresh, disposable network
// namespace, for tests that need to apply real rulesets against a real
// kernel. Mirrors google/nftables' own internal/nftest.OpenSystemConn
// (unexported, so not importable directly).
//
// Creating that namespace needs CAP_NET_ADMIN in the current user
// namespace; running `go test` unprivileged just skips these tests, and
// running it via `go -C hack/testrun run .` (which grants exactly that
// capability inside an unprivileged user+net namespace, the same way
// rootless Podman itself does) executes them for real -- see that command's
// doc comment.
func newSystemConn(t *testing.T) *nftables.Conn {
	t.Helper()

	// Namespace operations are thread-local; lock this goroutine to its
	// current OS thread so the namespace switch below actually takes
	// effect for every syscall this test makes, and restore/unlock on
	// cleanup.
	runtime.LockOSThread()

	ns, err := netns.New()
	if err != nil {
		runtime.UnlockOSThread()
		t.Skipf("skipping: creating a test network namespace requires CAP_NET_ADMIN (run via `go -C hack/testrun run .`): %v", err)
	}
	t.Cleanup(func() {
		defer runtime.UnlockOSThread()
		if err := ns.Close(); err != nil {
			t.Errorf("close test namespace: %v", err)
		}
	})

	conn, err := nftables.New(nftables.WithNetNSFd(int(ns)))
	if err != nil {
		t.Fatalf("nftables.New: %v", err)
	}
	return conn
}
