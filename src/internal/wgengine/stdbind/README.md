# stdbind

A fork of upstream wireguard-go's `conn.StdNetBind`, modified to *adopt*
already-bound sockets (systemd socket activation) instead of binding its
own -- see `docs/design-doc.md` §4.1 for the design rationale. This gets
production the same `recvmmsg`/`sendmmsg`, UDP GSO/GRO, and PKTINFO fast
path that `conn.NewStdNetBind()` already gets in local dev, replacing the
one-packet-at-a-time `wgengine.FdBind`.

Pinned to `golang.zx2c4.com/wireguard@v0.0.0-20260522210424-ecfc5a8d5446`,
package `conn`.

## Verbatim files

Byte-identical to upstream except the package clause (`conn` → `stdbind`)
and a `// Forked from ...` provenance line:

| File | Upstream source |
|---|---|
| `features_linux.go` / `features_default.go` | same name |
| `sticky_linux.go` / `sticky_default.go` | same name |
| `gso_linux.go` / `gso_default.go` | same name |
| `errors_linux.go` / `errors_default.go` | same name |
| `mark_unix.go` / `mark_default.go` | same name |

## Modified files

| File | Upstream source | What changed |
|---|---|---|
| `bind.go` | `bind_std.go` | See "Regions changed in bind.go" below. |
| `bind_test.go` | `bind_std_test.go` | `TestStdNetBindReceiveFuncAfterClose` replaced with an adoption-based suite (`newInherited`, `TestOpenCloseCycles`, `TestReceiveFuncReturnsErrClosed`, `TestSendReceiveBatch`, `TestOpenIgnoresPort`, `TestNoGoroutineLeak`). `Test_coalesceMessages`, `Test_splitCoalescedMessages`, and the `mockSetGSOSize`/`mockGetGSOSize` helpers are verbatim. |
| `sticky_linux_test.go` | same name | `Test_listenConfig` dropped (tested upstream's `net.ListenConfig`-based `listenConfig()`, which this fork doesn't have). `Test_setSrcControl`/`Test_getSrcFromControl` are verbatim. |
| `sockopts_linux.go` | `controlfns.go` + `controlfns_linux.go` | Rewritten from a `net.ListenConfig.Control` callback chain into a post-hoc `applySockopts(f *os.File, network string)` that runs directly against an already-adopted socket via `f.SyscallConn().Control`. `socketBufferSize` and `kernelVersion()` are otherwise unchanged. Adds `FamilyInfo` (feeds `Describe`) and `addrFromSockaddr`. |
| `sockopts_default.go` | `controlfns.go` + `controlfns_unix.go` | Same rewrite, minus the Linux-only options (`SO_*BUFFORCE`, PKTINFO, `UDP_GRO` have no portable equivalent). Exists so `stdbind`'s tests, which exercise `New`/`Open` for real via loopback sockets, also run correctly on a macOS dev machine -- not just `go vet`. |

Not copied: `conn.go` (defines `Bind`, `Endpoint`, `ReceiveFunc`,
`IdealBatchSize`, `ErrBindAlreadyOpen`, `ErrWrongEndpointType`,
`ErrUDPGSODisabled`) -- these are imported from the real
`golang.zx2c4.com/wireguard/conn` package instead, aliased `wgconn`. The
alias is required, not cosmetic: upstream's own `bind_std.go` uses the
identifier `conn` pervasively as a local variable name (e.g. `conn :=
s.ipv4`), which would shadow an unaliased import of the very package these
functions came from.

Also not copied: `bind_windows.go`, `boundif_android.go`, `bindtest/`,
`winrio/`, `controlfns_unix.go` (superseded by `sockopts_default.go`).

## Regions changed in `bind.go`

- Added `Options{UDP4, UDP6 *os.File; Logf}`, `New(Options) (*StdNetBind,
  error)`, `Port() uint16`, `Description`/`Describe()`.
- Struct gained `file4, file6 *os.File`, `port uint16`, `logf`, `info4,
  info6 FamilyInfo`, `warnedPort bool`.
- Deleted `listenNet` and `NewStdNetBind` (upstream's self-binding
  constructor; nothing in this codebase calls it -- local dev uses the real
  `conn.NewStdNetBind()` directly, not this fork). Added `adopt(f *os.File)
  (*net.UDPConn, error)`.
- `Open`: no longer binds anything. Adopts `file4`/`file6` via `adopt`, then
  the upstream tail (`supportsUDPOffload`, `ipv4.NewPacketConn`,
  `makeReceiveIPv4/6`) runs unchanged. Returns `s.port` always (the port is
  fixed at `New`); warns once via `logf` if the requested port differs.
- `Close`: byte-identical to upstream. It closes the dup'd conns `adopt`
  created -- `file4`/`file6` themselves are never touched, so the next
  `Open`'s `adopt` call dups a fresh fd again. This is why `Close` can be
  "really close" here where the old `FdBind` couldn't: a systemd-activated
  fd is handed to the process exactly once, but `adopt` never consumes
  `file4`/`file6`, only dups them.
- Deleted the locally-defined `ErrUDPGSODisabled` type. `Send`'s GSO-disable
  path now returns upstream's own `wgconn.ErrUDPGSODisabled{RetryErr: err}`
  (so `device/send.go` demotes it to `Verbosef` the same as it would for
  the real `conn.StdNetBind`) and flips `info4`/`info6.TxOffload = false`
  alongside the existing `ipv{4,6}TxOffload` fields. Since
  `wgconn.ErrUDPGSODisabled`'s `onLaddr` field is unexported (unsettable
  from here), the local address is logged separately via `logf` instead.
- `New` rejects a dual-stack (`IPV6_V6ONLY=0`) `UDP6` socket outright --
  the old socket unit's shape -- with a message pointing at the
  `docs/quickstart.md` upgrade note. No compatibility mode.
- Everything else (`Send`, `receiveIP`, `coalesceMessages`,
  `splitCoalescedMessages`, the batch reader/writer plumbing) is unchanged
  except identifier qualification (`Bind`→`wgconn.Bind`,
  `Endpoint`→`wgconn.Endpoint`, `ReceiveFunc`→`wgconn.ReceiveFunc`,
  `IdealBatchSize`→`wgconn.IdealBatchSize`,
  `ErrBindAlreadyOpen`→`wgconn.ErrBindAlreadyOpen`).

## Known and accepted limitations

- `device/sticky_linux.go` type-asserts `*conn.StdNetBind` to clear sticky
  `src` on a route-change notification; `*stdbind.StdNetBind` isn't that
  type, so this fork never gets that callback. Accepted for a server: every
  inbound packet refreshes endpoint and src anyway, and the pasta netns has
  exactly one interface.
- `SetMark` (`SO_MARK`, from the verbatim `mark_unix.go`) would return
  `EPERM` here -- it's only called when `fwmark != 0`, which nothing in this
  codebase sets.

## Refresh recipe

To pick up a newer upstream `golang.zx2c4.com/wireguard`:

1. `cd src && go get golang.zx2c4.com/wireguard@<new version> && go mod tidy`.
2. Diff each verbatim file above against
   `$(go env GOMODCACHE)/golang.zx2c4.com/wireguard@<new version>/conn/<upstream name>`
   and re-copy if it changed (re-apply the package-clause/provenance-line
   edit -- both are mechanical, see the commit that introduced this
   package for the exact transform).
3. Diff `bind_std.go` against `bind.go`'s upstream regions (everything
   *not* listed under "Regions changed in bind.go" above) and re-apply
   upstream's changes to those regions by hand; re-verify the changed
   regions still make sense against the new upstream code.
4. Diff `controlfns.go`/`controlfns_linux.go`/`controlfns_unix.go` against
   `sockopts_linux.go`/`sockopts_default.go` for new socket options worth
   adding to `applySockopts`.
5. Update the pinned commit in this file and in the `// Forked from ...`
   line of every file above.
6. `go vet ./... && GOOS=linux go vet ./...` from `src/`, then the full
   test suite (`go test ./internal/wgengine/stdbind/...` on darwin, and via
   `hack/testrun`/`make test` on Linux).
