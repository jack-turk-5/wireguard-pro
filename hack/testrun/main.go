// Command testrun runs `go test` inside a fresh, unprivileged user+network
// namespace mapped to root within itself -- the same namespace-based
// privilege model rootless Podman relies on, so tests that need it get a
// meaningful signal without needing real root or CI-level privileges.
//
// This is what makes the privileged tests in internal/nft (see
// internal/nft/system_test.go's newSystemConn) run for real instead of
// skipping: they create a real network namespace and apply real nftables
// rulesets via netlink, which needs CAP_NET_ADMIN. Run `go test` directly
// without this wrapper and those tests just skip themselves cleanly.
//
// Unlike a shell script wrapping the unshare(1) binary, this sets up the
// namespace via exec.Cmd's SysProcAttr directly: Go's os/exec exposes
// Cloneflags/UidMappings/GidMappings, which apply as part of the clone()
// that creates the child process. That sidesteps the restriction that would
// otherwise apply -- unshare(CLONE_NEWUSER) fails on a process with more
// than one thread, which any running Go program almost always already is;
// doing it at clone()-time for a brand new child process doesn't hit that
// restriction, and needs no external unshare(1) or shell.
//
// This is its own module (see go.mod in this directory) since it's a
// standalone tool invoked via `go run`, not a package imported by
// wireguard-pro itself.
//
// Usage: go -C hack/testrun run . [go test args...]  (defaults to ./...)
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "testrun:", err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"./..."}
	}

	srcDir, err := moduleRoot()
	if err != nil {
		return err
	}

	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("find go toolchain: %w", err)
	}

	cmd := exec.Command(goBin, append([]string{"test"}, args...)...)
	cmd.Dir = srcDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	uid, gid := os.Getuid(), os.Getgid()
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET | syscall.CLONE_NEWNS,
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: uid, Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: gid, Size: 1},
		},
		// Mapping a gid as a non-root user requires /proc/self/setgroups to
		// say "deny" first; false here is what makes exec.Cmd write that.
		GidMappingsEnableSetgroups: false,
	}

	return cmd.Run()
}

// moduleRoot locates wireguard-pro's own module directory (src/, a sibling
// of this file's hack/testrun/ directory), from this source file's own
// build-time path -- robust regardless of the caller's current directory or
// whether this was invoked via `go run`.
func moduleRoot() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	return filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", "..", "src"))
}
