#!/usr/bin/env bash
# Build cmd/wireguard-pro and smoke-test it inside an unprivileged
# user+net+mount namespace -- the same namespace-based privilege model
# rootless Podman itself relies on, so a pass here is a meaningful signal
# without needing real root or the actual target host.
#
# Validates: TUN creation, netlink address/link-up, the wireguard-go device +
# UAPI socket, and (the riskiest piece) FdBind over a *real* systemd-activated
# fd, via systemd-socket-activate mirroring wireguard-pro.socket's fd order
# (ListenStream before ListenDatagram -> fd index 0 = dashboard TCP, index 1 =
# VPN UDP).
#
# Usage: hack/test-rootless.sh [dashboard_port] [vpn_port] [run_seconds]
set -euo pipefail

DASH_PORT="${1:-51819}"
VPN_PORT="${2:-51820}"
RUN_SECONDS="${3:-5}"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
src_dir="$(dirname "$script_dir")"
bin="$(mktemp -u /tmp/wireguard-pro-test.XXXXXX)"
trap 'rm -f "$bin"' EXIT

command -v unshare >/dev/null || { echo "unshare (util-linux) is required" >&2; exit 1; }
command -v systemd-socket-activate >/dev/null || { echo "systemd-socket-activate is required" >&2; exit 1; }
command -v python3 >/dev/null || { echo "python3 is required (used to send the activation probe packets)" >&2; exit 1; }

echo "==> building $bin"
( cd "$src_dir" && go build -o "$bin" ./cmd/wireguard-pro )

echo "==> running rootlessly (unshare --user --net --mount --map-root-user)"
timeout "$((RUN_SECONDS + 3))" unshare --user --net --mount --map-root-user -- bash -c '
  set -euo pipefail
  ip link set lo up
  mount -t tmpfs tmpfs /var/run

  systemd-socket-activate -l '"$DASH_PORT"' -l '"$VPN_PORT"' --datagram '"$bin"' &
  ssa_pid=$!

  sleep 1
  python3 -c "
import socket
for port in ('"$DASH_PORT"', '"$VPN_PORT"'):
    s = socket.socket(socket.AF_INET6, socket.SOCK_DGRAM)
    s.sendto(b\"probe\", (\"::1\", port))
"
  sleep '"$RUN_SECONDS"'
  kill "$ssa_pid" 2>/dev/null || true
  wait "$ssa_pid" 2>/dev/null || true
'
