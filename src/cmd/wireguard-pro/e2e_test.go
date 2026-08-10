package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// activationHelperEnv, when set in this test binary's own environment,
// turns it into a one-shot activation shim instead of a test runner: see
// TestMain.
const activationHelperEnv = "WGPRO_E2E_ACTIVATE_TARGET"

// TestMain lets this test binary double as its own activation shim. systemd
// requires LISTEN_PID to equal the *receiving* process's own pid, but a
// parent can never know a not-yet-started child's pid in advance to put in
// its environment. The fix (the same one systemd itself uses): fork first,
// then let the child -- which always knows its own pid -- set LISTEN_PID
// and exec into the real target, preserving the pid across the exec.
//
// TestEndToEnd re-execs this same compiled test binary (os.Args[0]) with
// activationHelperEnv set and the activated fds passed via
// exec.Cmd.ExtraFiles. On re-exec, TestMain intercepts here, before any
// test ever runs, sets LISTEN_PID to its own (correct) pid, and execs
// straight into the real wireguard-pro binary -- no shell, no second
// language, no self-built subprocess helper.
func TestMain(m *testing.M) {
	if target := os.Getenv(activationHelperEnv); target != "" {
		if err := os.Setenv("LISTEN_PID", strconv.Itoa(os.Getpid())); err != nil {
			fmt.Fprintln(os.Stderr, "e2e activation helper: setenv LISTEN_PID:", err)
			os.Exit(1)
		}
		if err := syscall.Exec(target, []string{target}, os.Environ()); err != nil {
			fmt.Fprintln(os.Stderr, "e2e activation helper: exec:", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

// TestEndToEnd smoke-tests the real, built wireguard-pro binary's startup
// sequence: config/db bring-up, TUN creation, netlink address/link-up, the
// wireguard-go device + UAPI socket, FdBind over a *real* systemd-activated
// fd, and wgctrl's wguser backend round-tripping the persisted server key
// against that same UAPI socket.
//
// It runs the binary as a genuinely separate process (fork+exec via
// TestMain's activation shim above) rather than calling run() in-process:
// an early attempt at the latter crashed the Go runtime's netpoller
// (`netpoll failed`, EINVAL) by dup2-ing a live socket onto fd 3/4 out from
// under epoll registrations the runtime already held for it -- systemd-style
// fd handoff is only safe into a *fresh* process whose runtime hasn't
// touched those fds yet, which is exactly what exec.Cmd.ExtraFiles gives us
// for free.
//
// Needs CAP_NET_ADMIN (to create wg0) and CAP_SYS_ADMIN (to isolate
// /var/run and /etc/wireguard writes in a private mount namespace so this
// never touches a real deployment's files) -- run via `go -C hack/testrun
// run .` (see repo root), which supplies both through an unprivileged
// user+net+mount namespace. Run bare, it skips cleanly. Also skips if
// /dev/net/tun has no bound kernel driver on this host.
func TestEndToEnd(t *testing.T) {
	probe, err := tun.CreateTUN("wgprotest-probe", mtu)
	if err != nil {
		t.Skipf("skipping: cannot create a TUN device (no CAP_NET_ADMIN, or no tun kernel module on this host) -- run via `go -C hack/testrun run .`: %v", err)
	}
	probe.Close()

	if err := syscall.Mount("tmpfs", "/var/run", "tmpfs", 0, ""); err != nil {
		t.Skipf("skipping: cannot mount a private tmpfs at /var/run (needs CAP_SYS_ADMIN in a private mount namespace) -- run via `go -C hack/testrun run .`: %v", err)
	}
	if err := os.MkdirAll("/etc/wireguard", 0o755); err != nil {
		t.Fatalf("mkdir /etc/wireguard: %v", err)
	}
	if err := syscall.Mount("tmpfs", "/etc/wireguard", "tmpfs", 0, ""); err != nil {
		t.Fatalf("mount tmpfs over /etc/wireguard: %v", err)
	}

	// lo starts down in a fresh network namespace (this one courtesy of
	// hack/testrun's CLONE_NEWNET), so 127.0.0.1 is unreachable until it's
	// brought up -- affects the whole process tree uniformly since network
	// namespace membership was established at process creation, before the
	// parent test process or its child even existed.
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("find lo: %v", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatalf("set lo up: %v", err)
	}

	// go test always runs the test binary with its cwd set to this
	// package's own source directory (src/cmd/wireguard-pro), so "." and
	// the relative repo-root walk below are both stable regardless of
	// where `go test`/hack/testrun was invoked from.
	bin := filepath.Join(t.TempDir(), "wireguard-pro")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build wireguard-pro: %v\n%s", err, out)
	}

	dashLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen dashboard socket: %v", err)
	}
	dashAddr := dashLn.Addr().String()
	dashFile, err := dashLn.(*net.TCPListener).File()
	if err != nil {
		t.Fatalf("dup dashboard socket: %v", err)
	}

	vpnConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen VPN socket: %v", err)
	}
	vpnFile, err := vpnConn.File()
	if err != nil {
		t.Fatalf("dup VPN socket: %v", err)
	}

	child := exec.Command(os.Args[0])
	child.ExtraFiles = []*os.File{dashFile, vpnFile} // become fd 3, 4 in the child
	child.Env = append(os.Environ(),
		activationHelperEnv+"="+bin,
		"LISTEN_FDS=2",
		"LISTEN_FDNAMES=dashboard:vpn",
		"SECRET_KEY=test-secret-key",
		"WG_HOST=test.example.com",
		"WG_PORT=51820",
		"DB_FILE="+filepath.Join(t.TempDir(), "peers.db"),
		"NFT_CONF_FILE="+filepath.Join("..", "..", "..", "container", "nftables.json"),
	)
	child.Stdout = os.Stderr
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		t.Fatalf("start activated wireguard-pro: %v", err)
	}
	// The listen backlog on these sockets has been live since net.Listen/
	// ListenUDP above, well before the child's httpServer.Serve ever runs --
	// so a bare TCP dial against dashLn would "succeed" immediately
	// regardless of the child's actual progress. Closing the parent's
	// copies now (the child has its own, independent dup via ExtraFiles)
	// just leaves cleanup unambiguous; the real readiness signal is the
	// HTTP-level probe in waitForDashboard below.
	dashFile.Close()
	vpnFile.Close()
	dashLn.Close()
	vpnConn.Close()
	t.Cleanup(func() {
		child.Process.Signal(syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- child.Wait() }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			child.Process.Kill()
			<-done
		}
	})

	waitForDashboard(t, dashAddr)

	if _, err := netlink.LinkByName(ifaceName); err != nil {
		t.Errorf("%s link not found after startup: %v", ifaceName, err)
	}

	uapiSock := filepath.Join("/var/run/wireguard", ifaceName+".sock")
	if _, err := os.Stat(uapiSock); err != nil {
		t.Errorf("UAPI socket not found at %s: %v", uapiSock, err)
	}

	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		t.Fatalf("read persisted server key: %v", err)
	}
	priv, err := wgtypes.ParseKey(strings.TrimSpace(string(keyData)))
	if err != nil {
		t.Fatalf("parse persisted server key: %v", err)
	}

	wg, err := wgctrl.New()
	if err != nil {
		t.Fatalf("open wgctrl client: %v", err)
	}
	defer wg.Close()
	dev, err := wg.Device(ifaceName)
	if err != nil {
		t.Fatalf("wgctrl.Device(%s): %v", ifaceName, err)
	}
	if dev.PublicKey != priv.PublicKey() {
		t.Errorf("device public key %s does not match persisted key's public key %s", dev.PublicKey, priv.PublicKey())
	}
}

// waitForDashboard polls with a real HTTP request rather than a bare TCP
// dial: the listen backlog on the dashboard socket has been live since it
// was created in the parent, well before the child's httpServer.Serve ever
// runs, so a TCP-level connect alone would "succeed" immediately regardless
// of the child's actual startup progress. An HTTP round-trip only succeeds
// once something is actually calling Accept and speaking the protocol.
func waitForDashboard(t *testing.T, addr string) {
	t.Helper()
	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://" + addr + "/")
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("dashboard at %s did not become reachable in time", addr)
}
