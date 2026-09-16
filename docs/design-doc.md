# Design: maximum-throughput rootless WireGuard

Status: proposed (2026-09-16)

## 1. Goal and constraints

wireguard-pro runs a WireGuard **server** as a rootless Podman Quadlet container
(`Network=pasta:--mtu,1500`, `CAP_NET_ADMIN`, `/dev/net/tun`) on wireguard-go. The
WireGuard UDP socket is opened by `systemd --user` socket activation in the **host**
network namespace and handed into the container, so encrypted UDP already bypasses pasta.

Target: consistent 1 GbE line rate (~940 Mbit/s) through the tunnel, for mixed traffic
(internet egress and host/LAN services), with **no root at runtime**. A one-shot root
host step is acceptable only if it is measurably significant.

Today the tunnel is "fastish" with small periodic drops, and no throughput number has
ever been recorded for this project.

## 2. What the research settled

### 2.1 Kernel WireGuard is not an option rootless

A kernel `wg` device *can* be created inside the container netns with `CAP_NET_ADMIN`
(WireGuard's generic-netlink ops use `GENL_UNS_ADMIN_PERM`; rtnetlink checks
`CAP_NET_ADMIN` against the netns owner). But a kernel WireGuard device's UDP socket is
pinned to the netns it was **created** in ([wireguard.com/netns](https://www.wireguard.com/netns/):
"This socket always lives in namespace A – the original birthplace namespace"), and the
device netlink attribute set (`ifindex, ifname, private-key, public-key, flags,
listen-port, fwmark, peers`) has no way to hand it an fd or a different netns.

- Created inside the container → its encrypted UDP goes through pasta (per-packet
  userspace forwarding, the very thing to avoid).
- Created on the host and moved into the container (the
  [schmensch/homelab `wgfh.sh`](https://github.com/schmensch/homelab/tree/main/high-speed-wireguard-in-rootless-podman)
  approach) → requires real root.

Socket activation is therefore the only rootless way to get the UDP socket into the host
netns, and it forces a userspace WireGuard implementation.

### 2.2 wireguard-go with its offload path is the right engine

With TUN TSO/GRO via `virtio_net_hdr`, UDP GSO/GRO (`UDP_SEGMENT`/`UDP_GRO`) and
`sendmmsg`/`recvmmsg`, wireguard-go outperforms in-kernel WireGuard on the same hardware
(Tailscale, i5-12400: 13.0 vs 2.66 Gbit/s —
[Surpassing 10Gb/s over Tailscale](https://tailscale.com/blog/more-throughput),
[wireguard-go PR #75](https://github.com/WireGuard/wireguard-go/pull/75)). boringtun has
no GSO/GRO/mmsg support and no published benchmark against wireguard-go — do not go back.
The repo is on `golang.zx2c4.com/wireguard v0.0.0-20260522210424-ecfc5a8d5446`, which
contains all of the above.

### 2.3 The bottleneck is our own `FdBind`

`src/internal/wgengine/fdbind.go` exists to accept the inherited fd and *deliberately*
discards the fast path:

- `BatchSize()==1`, one `ReadFrom` per packet, `WriteTo` loop for send
- no `recvmmsg`/`sendmmsg`, no `UDP_GRO`/`UDP_SEGMENT`, no `IP_PKTINFO`
- no socket-buffer sizing, one receive goroutine, one endpoint allocation per packet

Production (socket-activated) gets this; local dev (`conn.NewStdNetBind()`) gets the full
fast path. The TUN side is already fine: `tun.CreateTUN` enables `IFF_VNET_HDR` with
`TUN_F_CSUM|TSO4|TSO6` (+ `USO4|USO6` on kernel ≥ 6.2), batch size 128, TCP+UDP GRO
coalescing on write, GSO split on read.

### 2.4 Likely cause of the drops: the socket buffer

A `systemd --user` socket cannot use `SO_RCVBUFFORCE`, so `SO_RCVBUF` is clamped at
`net.core.rmem_max` = 212992 bytes (208 KiB) ≈ 1.7 ms of 1 GbE. With one syscall per
packet and one receive goroutine, any Go scheduling/GC hiccup overflows it and the kernel
drops (`UdpRcvbufErrors`). Batched GRO reads drain far faster; if drops persist after the
bind fix, raising `rmem_max`/`wmem_max` is the one-shot root sysctl.

### 2.5 pasta

pasta is single-threaded, its tap has no `IFF_VNET_HDR` (no GSO/TSO/GRO), and its own CI
measures roughly 2.5–3.7 Gbit/s ns→host / 2.3–2.9 Gbit/s host→ns at 1500-byte frames on
a 3.6 GHz core ([passt perf data](https://passt.top/builds/latest/web/perf.js)). That is
above 1 GbE, so at this target pasta is a CPU/jitter cost, not the ceiling. Tap MTU above
1500 mainly helps socket-originated traffic; forwarded GSO skbs are re-segmented at
`gso_size` regardless of tap MTU — verify empirically. Run a recent passt (≥ 2026);
Ubuntu-24.04-era pasta breaks forwarded return traffic
([podman#27541](https://github.com/containers/podman/issues/27541)).

TCP MSS clamping is not needed through pasta: pasta proxies TCP, so the peer's own MSS
(1380 from its 1420 MTU) sizes the segments toward the peer.

## 3. Strategy

1. **Phase 1 (zero root — this document):** replace `FdBind` with an in-tree fork of
   upstream `StdNetBind` that *adopts* the systemd fds → `recvmmsg`/`sendmmsg`, UDP
   GSO/GRO, PKTINFO, two receive goroutines, batch 128. Add startup diagnostics, a
   `WG_MTU` knob, and a benchmark harness so every later decision is measured.
2. **Phase 2 (decision gate, measured):** if `UdpRcvbufErrors` still increments under
   load, apply the one-shot host sysctl drop-in (`net.core.rmem_max`/`wmem_max`) plus
   `ReceiveBuffer=`/`SendBuffer=` in the socket unit.
3. **Phase 3 (only if 1+2 do not reach consistent line rate; separate design):** a
   "turbo" deployment mode that removes pasta from the data path: `--network=host`
   rootless + a root one-shot boot unit that creates a persistent user-owned TUN
   (`ip tuntap add mode tun user <uid> name wg0`) with addresses/routes/forwarding/nft on
   the host. wireguard-go can attach to a pre-made, user-owned TUN without
   `CAP_NET_ADMIN`. Zero root at runtime, but root at boot and nft management moves to the
   host. Out of scope here.

## 4. Phase 1 design

### 4.1 Fork upstream `StdNetBind` → `src/internal/wgengine/stdbind/`

Copy from `$GOMODCACHE/golang.zx2c4.com/wireguard@v0.0.0-20260522210424-ecfc5a8d5446/conn/`,
keeping the MIT SPDX headers plus a `// Forked from …` line, and upstream identifier
names so `diff -u` against upstream stays trivial. A `README.md` in the package pins the
commit, lists verbatim files, the exact regions changed in `bind.go`, and the refresh
recipe.

| Upstream | Destination | Notes |
|---|---|---|
| `bind_std.go` | `bind.go` | **modified** (below) |
| `features_linux.go` / `features_default.go` | same | verbatim |
| `sticky_linux.go` / `sticky_default.go` (+ `sticky_linux_test.go`) | same | verbatim |
| `gso_linux.go` / `gso_default.go` | same | verbatim |
| `errors_linux.go` / `errors_default.go` | same | verbatim |
| `mark_unix.go` / `mark_default.go` | same | verbatim |
| `controlfns.go` + `controlfns_linux.go` | `sockopts_linux.go` + `sockopts_default.go` | rewritten as a *post-hoc* applier (`listenConfig` is dead for us); keep `socketBufferSize = 7<<20` and `kernelVersion()` |
| `bind_std_test.go` | `bind_test.go` | keep `Test_coalesceMessages`/`Test_splitCoalescedMessages` verbatim |
| `conn.go` | — | don't copy: import `golang.zx2c4.com/wireguard/conn` for `Bind`, `Endpoint`, `ReceiveFunc`, `IdealBatchSize`, `ErrBindAlreadyOpen`, `ErrWrongEndpointType`, `ErrUDPGSODisabled` |

The `_default` counterparts keep `go vet ./...` working on a macOS dev machine; verify
both `go vet ./...` and `GOOS=linux go vet ./...` from `src/`. `golang.org/x/net` becomes
a direct dependency (`go mod tidy`).

**Changes to `bind.go` (only these):**

```go
type Options struct {
    UDP4, UDP6 *os.File       // already-bound AF_INET / AF_INET6 SOCK_DGRAM; either may be nil
    Logf func(string, ...any) // default log.Printf
}
func New(o Options) (*StdNetBind, error)    // applies sockopts (§4.1b), validates same port on both
func (s *StdNetBind) Port() uint16
func (s *StdNetBind) Describe() Description // §4.4
```

- Struct gains `file4, file6 *os.File` (pristine, held for the process lifetime, never
  closed), `port uint16`, `logf`, `info4, info6 FamilyInfo`, `warnedPort bool`.
- Delete `listenNet`. Add `adopt(f *os.File) (*net.UDPConn, error)` =
  `net.FilePacketConn(f)` (dups the fd, sets `O_NONBLOCK`, fresh netpoller registration)
  and assert `*net.UDPConn`.
- `Open(uport)`: keep the `ErrBindAlreadyOpen` guard; drop the `again:`/`EADDRINUSE`
  retry; `adopt` each present family; then the upstream tail verbatim
  (`supportsUDPOffload`, `ipv4.NewPacketConn`, `makeReceiveIPv4/6` — keep these names,
  `ReceiveFunc.PrettyName()` depends on them). Return `fns, s.port`. If
  `uport != 0 && uport != s.port`, warn once ("listen_port %d requested but
  socket-activated bind is fixed at %d"); nothing in-tree sets `ListenPort`, and an error
  would surface as `IpcErrorPortInUse`.
- `Close()`: **verbatim upstream** — really closes the dup'd conns. This is the
  **dup-per-Open** lifecycle: `device.BindUpdate` waits for old receive goroutines
  (`netc.stopping.Wait()`) before re-`Open`, and a closed dup makes `ReadBatch` return an
  error satisfying `errors.Is(err, net.ErrClosed)` so `RoutineReceiveIncoming` exits
  immediately. (The current FdBind deadline trick yields a Temporary error instead, which
  costs ~3 s of retry-sleep per Down.) Socket options live on the shared `struct sock`,
  so they persist across dups. Go's `poll.FD.destroy` performs `EPOLL_CTL_DEL` before
  `close(2)`, so no stale registration remains — this is not the dup2-onto-live-fd
  pattern documented in `e2e_test.go`.
- GSO-disable path: delete the local `ErrUDPGSODisabled` type; log the local address
  ourselves and return upstream `conn.ErrUDPGSODisabled{RetryErr: err}` so
  `device/send.go` demotes it to Verbosef; flip `info{4,6}.TxOffload = false`.
- `New` fails with a clear message if only a *dual-stack* UDP6 socket is present
  (`IPV6_V6ONLY == 0`, i.e. the old socket unit), pointing at the upgrade note. No
  compatibility mode.

Known and accepted: `device/sticky_linux.go` type-asserts `*conn.StdNetBind`, so the
route-change listener that clears sticky `src` is inactive for the fork. Fine for a
server (every inbound packet refreshes endpoint and src; the pasta netns has one
interface). `SetMark` (`SO_MARK`) would return EPERM here — it is only called when
`fwmark != 0`, which nothing sets.

#### 4.1b Post-hoc socket options (`sockopts_linux.go`, via `f.SyscallConn().Control`)

Per family: `SO_RCVBUF`/`SO_SNDBUF` = 7 MiB, then `SO_RCVBUFFORCE`/`SO_SNDBUFFORCE`
(expected EPERM — the socket lives in the init netns/userns; kept so a root deployment
benefits), read back the effective sizes; v4 `IP_PKTINFO=1` / v6 `IPV6_RECVPKTINFO=1`;
v6 **read** `IPV6_V6ONLY` (it cannot be set after bind → EINVAL); `UDP_GRO=1` gated on
kernel ≥ 5.12 (mandatory — otherwise `supportsUDPOffload` reports rx=false). Everything is
recorded in `FamilyInfo`.

### 4.2 `sockact` classifies by socket type and family, not by index

Rewrite `src/internal/sockact/sockact.go` (+ `sockact_unix.go`, `//go:build unix`):

```go
type Sockets struct {
    Listeners     []net.Listener // every SOCK_STREAM fd, activation order
    UDP4, UDP6    *os.File
    UDP6DualStack bool           // IPV6_V6ONLY == 0
    Names         []string       // for logging only
}
func Load() (*Sockets, error)                      // classify(activation.Files(true))
func (s *Sockets) Activated() bool
func classify(files []*os.File) (*Sockets, error)  // unit-testable without LISTEN_* env
```

Classify with `SO_TYPE` plus `unix.Getsockname` (the sockaddr type gives the family and
sidesteps the `IP.To4()` ambiguity of `::` vs `0.0.0.0`). Stream → `net.FileListener`,
then close the original. A duplicate UDP family is an error. Delete
`Files/Len/Listener(i)/PacketConn(i)`.

### 4.3 Wire-up: `main.go`, `config`, `wgengine`

- `main.go`: drop the `dashSocketIndex`/`vpnSocketIndex`/`mtu` constants. `buildBind(s)`:
  if no UDP fd and `s.Activated()` → **error** ("socket-activated but no UDP socket
  passed; check ListenDatagram=") — a self-bound port inside pasta's netns is unreachable
  from outside, so a silent fallback is a trap; not activated → keep
  `conn.NewStdNetBind()` for local dev. Otherwise `stdbind.New(...)` and return
  `b, b.Port()`. `dashListeners(s)` returns `s.Listeners` (v4+v6) or the dev fallback;
  serve one `http.Server` on all listeners (`serveErrCh` buffered to `len(listeners)`;
  `Shutdown` closes all).
- Delete `src/internal/wgengine/fdbind.go`.
- `config.go`: `WGMTU` from `WG_MTU` (default 1420, valid 1280–65535); `main.go` passes it.
- `wgengine`: add `TunOffloads(dev tun.Device) (vnetHdr, udpGSO bool)` in a `_linux.go`
  (vnetHdr = `BatchSize() > 1`; udpGSO = re-issue the identical `TUNSETOFFLOAD` ioctl that
  `tun_linux.go` already issued — idempotent; failure ⇒ kernel < 6.2) with a no-op default.

### 4.4 Startup diagnostics (after `wgengine.Up`, once offload flags are populated)

```
wireguard-pro: sockets: 4 activated (names=…) tcp=2 udp4=yes udp6=yes(v6only)
wireguard-pro: bind: port=51820 batch=128 kernel=6.8 v4[0.0.0.0:51820 gro=on gso=on rcvbuf=425984 sndbuf=425984 sticky=on] v6[[::]:51820 …]
wireguard-pro: tun: wg0 mtu=1420 batch=128 vnet_hdr=on udp_gso=on
```

If `rcvbuf == 425984` (the kernel-doubled default clamp), append a hint pointing at the
sysctl note. The local-dev path logs `bind: upstream StdNetBind (self-bound)`.

### 4.5 Socket unit, docs, Makefile

`quadlet/wireguard-pro.socket`:

```ini
[Socket]
ListenStream=0.0.0.0:51819
ListenStream=[::]:51819
ListenDatagram=0.0.0.0:51820
ListenDatagram=[::]:51820
BindIPv6Only=ipv6-only
ReceiveBuffer=7M
SendBuffer=7M
Service=wireguard-pro.service
```

`BindIPv6Only` is per-unit, hence the TCP pair as well. `ReceiveBuffer=` clamps to
`rmem_max` under `systemd --user` exactly like our setsockopt — belt and braces.

Docs: `docs/quickstart.md` gains an **Upgrading** note (the unit now binds four sockets →
reinstall the unit and `make reload`; a bare socket restart is not enough because fds are
only passed at service start) and the **optional one-shot host sysctl**
(`/etc/sysctl.d/90-wireguard-pro.conf`: `net.core.rmem_max=7340032`,
`net.core.wmem_max=7340032`) with when-to-apply guidance (only if `UdpRcvbufErrors` grows
under load — the Phase 2 gate). `docs/env.md`: `WG_MTU`, and that `GOGC`/`GOMAXPROCS` can be
set from the env file (Go ≥ 1.25 is already cgroup-aware for `GOMAXPROCS`). `README.md`:
architecture bullet update. Makefile: `bench-sidecar` target and a `vet` target
(darwin + `GOOS=linux`).

### 4.6 Tests

- `stdbind/bind_test.go` (runs on darwin via the single-message path and on linux via the
  batch path): `newInherited(t)` builds `udp4 127.0.0.1:0` plus a best-effort
  `udp6 [::1]:<same port>` file pair. `TestOpenCloseCycles` (5× Open → second Open =
  `ErrBindAlreadyOpen` → Close → Close nil; port stable), `TestReceiveFuncReturnsErrClosed`
  (blocked reader, `Close()`, `errors.Is(net.ErrClosed)` within 2 s), `TestSendReceiveBatch`
  (64 datagrams in; payload/endpoint/`SrcIP` on linux; `Send` 32 bufs out through
  `coalesceMessages`), `TestOpenIgnoresPort` (warns once), `TestNoGoroutineLeak`, verbatim
  coalesce/split tests. `bind_linux_test.go`: `Describe()` reports gso/gro/sticky/v6only as
  expected for the kernel. `TestMain` brings `lo` up when euid == 0 (hack/testrun netns).
- `sockact/sockact_test.go`: real `tcp4`/`udp4`/`udp6`/dual-stack `udp` files fed to
  `classify` in shuffled order; duplicate families error.
- `e2e_test.go`: pass `{vpn6, dash, vpn4}` in scrambled order (v6 best-effort), assert
  `dev.ListenPort == vpn4 port`, optionally fire one garbage datagram and confirm the
  dashboard still answers. The fork+exec shim is unchanged.

### 4.7 Benchmark harness `hack/bench/`

- `Containerfile.sidecar`: alpine + `iperf3 iproute2 ethtool tcpdump`.
- `sidecar.sh`: `podman run --rm -it --network=container:wireguard-pro …` — run
  `iperf3 -s -B 10.8.0.1` (measures the **WG hop only**, no pasta), `ip -s link show wg0`,
  `ethtool -k wg0`, `ip route`.
- `host-collect.sh <label> <secs>`: `uname -r`, `sysctl net.core.rmem_max`, the
  `bind:`/`tun:` journal lines, `nstat -az UdpInDatagrams UdpInErrors UdpRcvbufErrors
  UdpOutDatagrams` deltas, mid-run `ss -uapmn 'sport = :51820'`, `pidstat`/`top` CPU for
  `wireguard-pro` and the `pasta` process → `hack/bench/results/<label>-<ts>.txt`.
- `README.md`: peer-side commands (`iperf3 -c 10.8.0.1 -t 20 [-P 4] [-R]`,
  `iperf3 -c <LAN host>` for the through-pasta rows, `iperf3 -u -b 900M -l 1400` for
  loss), the pasta quirk (from inside the netns the host's own IP is the netns itself —
  use the pasta gateway address or a separate LAN machine), the per-case procedure, and
  the results table:

| case | bind | pasta mtu | WG_MTU | GOGC | TCP P1 fwd/rev | TCP P4 fwd/rev | retrans | UDP 900M loss | UdpRcvbufErrors Δ | CPU wgpro | CPU pasta |
|---|---|---|---|---|---|---|---|---|---|---|---|
| A0 | FdBind (`:fdbind` image) | 1500 | 1420 | 100 | | | | | | | |
| B0 | stdbind | 1500 | 1420 | 100 | | | | | | | |
| B1 | stdbind | 65520 (`Network=pasta`) | 1420 | 100 | | | | | | | |
| B2/B3 | stdbind | best | 1380 / 1500 | 100 | | | | | | | |
| B4/B5 | stdbind | best | best | 200 / 400 | | | | | | | |

Each row is run twice: peer→10.8.0.1 (WG only) and peer→LAN host (through pasta).

## 5. Order of work

1. `hack/bench/` first; tag the current image `:fdbind`; run the **A0 baseline**.
2. `sockact` rewrite + tests → `go test ./internal/sockact` (Mac and `hack/testrun`).
3. `stdbind` verbatim files + README + pure tests → `go vet` darwin & linux, `go mod tidy`.
4. `stdbind` adoption (`New/adopt/Port/Describe`, sockopts, GSO-disable change) → full
   bind tests.
5. `main.go`/`config`/`wgengine` wiring, delete `fdbind.go`, diagnostics, `e2e_test.go` →
   `make test` and `go -C hack/testrun run . ./... -v -count=1`.
6. Socket unit + docs + Makefile → deploy with `make reload`; check the journal
   `bind:`/`tun:` lines show both families `gro=on gso=on`, a real peer handshakes over v4
   and v6, `nstat` shows no `UdpInErrors` growth.
7. Benchmark matrix B0–B5; pick defaults from the data; commit the results file; make
   the Phase 2 sysctl decision from the `UdpRcvbufErrors` column.

## 6. Verification

- `cd src && go vet ./... && GOOS=linux go vet ./... && go test ./...` on the dev machine.
- `make test` (container) and `go -C hack/testrun run . ./... -v -count=1` (namespaced;
  exercises the batch path, PKTINFO, GRO flags, e2e with scrambled fds).
- On the host after `make reload`: the journal shows 4 activated sockets, a `bind:` line
  with `batch=128 gro=on gso=on` per family, `tun: … vnet_hdr=on`; the dashboard is
  reachable on v4 and v6; `wg show`/dashboard report port 51820.
- Benchmark: A0 vs B0 through both paths; expect higher and *flat* TCP throughput,
  `UdpRcvbufErrors Δ = 0`, lower `wireguard-pro` CPU per Gbit. If `UdpRcvbufErrors` still
  increments → apply the Phase 2 sysctl and re-run B0.

## 7. Risks

- Podman passes all `LISTEN_FDS` fds (conmon `--preserve-fds`), but `LISTEN_FDNAMES`
  propagation varies by version → classification never depends on names or index; the
  sockets log line makes mismatches obvious.
- `IPV6_V6ONLY` is unit-controlled; a stale unit yields a clear startup error plus the
  upgrade note.
- Kernel floors: `UDP_SEGMENT` ≥ 4.18, `UDP_GRO` ≥ 5.12, TUN USO ≥ 6.2 (all degrade
  gracefully; diagnostics show which are off).
- GSO EIO auto-disable on NICs without tx checksum offload — handled by upstream retry
  logic; `Describe()` reflects the flip.
- The CI container may lack IPv6 → v6 sockets are best-effort in tests.
- Local Go toolchain vs `go 1.26.5` in `go.mod` → rely on `GOTOOLCHAIN=auto` or run in the
  test container.

## 8. Sources

- WireGuard netns semantics: https://www.wireguard.com/netns/
- WireGuard netlink attributes: https://docs.kernel.org/next/netlink/specs/wireguard.html
- Rootless kernel WireGuard in Podman: https://www.procustodibus.com/blog/2022/10/wireguard-in-podman/, https://github.com/containers/podman/discussions/25044
- Reference host-side approach: https://github.com/schmensch/homelab/tree/main/high-speed-wireguard-in-rootless-podman
- wireguard-go offloads: https://tailscale.com/blog/throughput-improvements, https://tailscale.com/blog/more-throughput, https://tailscale.com/blog/quic-udp-throughput, https://github.com/WireGuard/wireguard-go/pull/75
- passt/pasta: https://passt.top/passt/about/, https://passt.top/builds/latest/web/perf.js, https://passt.top/passt/tree/tap.c, https://github.com/containers/podman/issues/27541, https://github.com/containers/podman/issues/23883
- UDP socket buffer clamping: `systemd.socket(5)` `ReceiveBuffer=`, `socket(7)` `SO_RCVBUFFORCE`, https://peira.dev/blog/tailscale-offsite-backup-wireguard-buffer/
- bypass4netns (Phase 3 candidate research): https://arxiv.org/abs/2402.00365
