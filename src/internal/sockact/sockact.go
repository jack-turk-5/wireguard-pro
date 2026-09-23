// Package sockact resolves and classifies systemd socket-activated file
// descriptors for this process.
//
// Sockets are classified by socket type (SOCK_STREAM/SOCK_DGRAM) and, for
// datagram sockets, by address family -- not by declaration index or
// LISTEN_FDNAMES. Podman passes all LISTEN_FDS fds through, but
// LISTEN_FDNAMES propagation has varied by version, and index-based
// classification breaks the moment the .socket unit's declaration order
// changes. See classify in sockact_unix.go.
package sockact

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/coreos/go-systemd/v22/activation"
)

// Sockets is the resolved, classified set of systemd-activated file
// descriptors for this process: every SOCK_STREAM fd as a net.Listener (in
// activation order), and at most one SOCK_DGRAM fd per address family, held
// as a raw *os.File for the caller to adopt (see wgengine/stdbind).
type Sockets struct {
	Listeners []net.Listener // every SOCK_STREAM fd, activation order
	UDP4      *os.File
	UDP6      *os.File
	// UDP6DualStack reports whether UDP6 has IPV6_V6ONLY=0, i.e. it also
	// serves IPv4-mapped traffic instead of leaving that to UDP4.
	UDP6DualStack bool
	// Names holds each activated fd's systemd name (LISTEN_FDNAMES, or a
	// synthesized "LISTEN_FD_<n>"), in the same order it was resolved --
	// for logging only, never for classification.
	Names []string
}

// Load resolves and classifies every systemd-activated fd for this process
// exactly once. Safe to call even when not socket-activated (e.g. local
// dev): returns an empty, non-activated Sockets.
func Load() (*Sockets, error) {
	return classify(activation.Files(true))
}

// Activated reports whether this process was started via systemd socket
// activation at all.
func (s *Sockets) Activated() bool {
	return len(s.Listeners) > 0 || s.UDP4 != nil || s.UDP6 != nil
}

// Describe renders a one-line summary for startup diagnostics
// (docs/design-doc.md §4.4), e.g.
// "4 activated (names=dashboard-v4:dashboard-v6:vpn-v4:vpn-v6) tcp=2 udp4=yes udp6=yes(v6only)".
func (s *Sockets) Describe() string {
	udp4 := "no"
	if s.UDP4 != nil {
		udp4 = "yes"
	}
	udp6 := "no"
	if s.UDP6 != nil {
		if s.UDP6DualStack {
			udp6 = "yes(dual-stack)"
		} else {
			udp6 = "yes(v6only)"
		}
	}
	return fmt.Sprintf("%d activated (names=%s) tcp=%d udp4=%s udp6=%s",
		len(s.Names), strings.Join(s.Names, ":"), len(s.Listeners), udp4, udp6)
}
