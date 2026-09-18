# Benchmark harness

Measures wireguard-pro throughput before/after the `stdbind` change (see
`docs/design-doc.md`). Two things are measured per case, on two different
paths:

- **WG hop only** (peer → `10.8.0.1`): exercises wireguard-go + TUN, never
  crosses pasta. Server side is `sidecar.sh`, run on the deployment host.
- **Through pasta** (peer → a LAN host reachable via the tunnel): exercises
  the full path a real client uses, pasta included.

Needs the real deployment host for anything beyond building the sidecar
image -- `systemd --user`, pasta, and a real NIC don't exist in a laptop's
podman VM. See the parent `docs/design-doc.md` §5 for how this fits into the
overall implementation order (this harness and the A0 baseline run first,
before any bind code changes).

## Setup

On the **deployment host** (where `wireguard-pro.socket`/`.service` run):

```sh
hack/bench/sidecar.sh          # builds + runs the iperf3 server at 10.8.0.1
```

Leave that running in one terminal. In another terminal on the same host,
run `host-collect.sh <label> <secs>` for the same window as each peer-side
run below.

## Peer-side commands

From a **separate peer machine** already connected to the tunnel:

```sh
# WG hop only, single stream, forward and reverse
iperf3 -c 10.8.0.1 -t 20
iperf3 -c 10.8.0.1 -t 20 -R

# WG hop only, 4 parallel streams
iperf3 -c 10.8.0.1 -t 20 -P 4
iperf3 -c 10.8.0.1 -t 20 -P 4 -R

# UDP loss at ~900 Mbit/s, 1400-byte payload
iperf3 -c 10.8.0.1 -u -b 900M -l 1400 -t 20

# Through pasta: target a LAN host, NOT the deployment host's own address --
# from inside the container's netns, the host's own IP resolves to the
# netns itself (pasta quirk), not a real host/LAN round trip. Use the pasta
# gateway address, or point at an actual separate LAN machine.
iperf3 -c <LAN host> -t 20
iperf3 -c <LAN host> -t 20 -P 4
```

## Per-case procedure

1. Deploy the case (image tag / socket unit / `WG_MTU` / `GOGC` per the row
   below) and confirm the `bind:`/`tun:` journal lines show the expected
   flags.
2. Start `sidecar.sh` on the host.
3. On the peer, run one of the commands above; simultaneously on the host,
   run `host-collect.sh <label> <secs>` with a matching `<secs>`.
4. Repeat for the through-pasta variant (target a LAN host instead of
   `10.8.0.1`).
5. Fill in the results table below from `results/<label>-*.txt` and the
   iperf3 output.

## Results

| case | bind | pasta mtu | WG_MTU | GOGC | TCP P1 fwd/rev | TCP P4 fwd/rev | retrans | UDP 900M loss | UdpRcvbufErrors Δ | CPU wgpro | CPU pasta |
|---|---|---|---|---|---|---|---|---|---|---|---|
| A0 | FdBind (`:fdbind` image) | 1500 | 1420 | 100 | | | | | | | |
| B0 | stdbind | 1500 | 1420 | 100 | | | | | | | |
| B1 | stdbind | 65520 (`Network=pasta`) | 1420 | 100 | | | | | | | |
| B2/B3 | stdbind | best | 1380 / 1500 | 100 | | | | | | | |
| B4/B5 | stdbind | best | best | 200 / 400 | | | | | | | |

Each row is run twice: peer→10.8.0.1 (WG only) and peer→LAN host (through
pasta). "best" means: carry forward whichever value won the prior row(s).
