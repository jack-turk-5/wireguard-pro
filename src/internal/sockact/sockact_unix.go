//go:build unix

package sockact

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// classify sorts files by SO_TYPE and, for SOCK_DGRAM sockets, by the
// address family unix.Getsockname's returned sockaddr reports -- which
// sidesteps the net.IP.To4() ambiguity between "::" and "0.0.0.0" that an
// address-string-based classifier would hit. Unit-testable without any
// LISTEN_* environment variables: callers pass real socket files directly.
func classify(files []*os.File) (*Sockets, error) {
	s := &Sockets{Names: make([]string, len(files))}

	for i, f := range files {
		s.Names[i] = f.Name()
		fd := int(f.Fd())

		typ, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_TYPE)
		if err != nil {
			return nil, fmt.Errorf("sockact: SO_TYPE for fd %d (%s): %w", fd, f.Name(), err)
		}

		switch typ {
		case unix.SOCK_STREAM:
			ln, err := net.FileListener(f)
			if err != nil {
				return nil, fmt.Errorf("sockact: FileListener for fd %d (%s): %w", fd, f.Name(), err)
			}
			f.Close() // FileListener dups the fd; the original is no longer needed.
			s.Listeners = append(s.Listeners, ln)

		case unix.SOCK_DGRAM:
			if err := classifyDgram(s, f, fd); err != nil {
				return nil, err
			}

		default:
			return nil, fmt.Errorf("sockact: fd %d (%s) is neither SOCK_STREAM nor SOCK_DGRAM (SO_TYPE=%d)", fd, f.Name(), typ)
		}
	}

	return s, nil
}

// classifyDgram assigns f to Sockets.UDP4 or UDP6 by the address family
// reported by getsockname, and -- for UDP6 -- records whether it's
// dual-stack. The file is kept as-is (not dup'd/closed): callers adopt it
// for the process lifetime (see wgengine/stdbind.New).
func classifyDgram(s *Sockets, f *os.File, fd int) error {
	sa, err := unix.Getsockname(fd)
	if err != nil {
		return fmt.Errorf("sockact: getsockname for fd %d (%s): %w", fd, f.Name(), err)
	}

	switch sa.(type) {
	case *unix.SockaddrInet4:
		if s.UDP4 != nil {
			return fmt.Errorf("sockact: duplicate UDP4 socket (fd %d %s already have %s)", fd, f.Name(), s.UDP4.Name())
		}
		s.UDP4 = f

	case *unix.SockaddrInet6:
		if s.UDP6 != nil {
			return fmt.Errorf("sockact: duplicate UDP6 socket (fd %d %s already have %s)", fd, f.Name(), s.UDP6.Name())
		}
		v6only, err := unix.GetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_V6ONLY)
		if err != nil {
			return fmt.Errorf("sockact: IPV6_V6ONLY for fd %d (%s): %w", fd, f.Name(), err)
		}
		s.UDP6 = f
		s.UDP6DualStack = v6only == 0

	default:
		return fmt.Errorf("sockact: fd %d (%s) is a datagram socket of unsupported address family %T", fd, f.Name(), sa)
	}

	return nil
}
