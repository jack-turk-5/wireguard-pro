#!/bin/sh
# host-collect.sh gathers one benchmark case's diagnostics on the deployment
# host -- run it *during* the peer's iperf3 window, for the same duration.
# Needs the real host: systemd --user journal, nstat, ss, pidstat all live
# there, not in the Mac dev environment or the bench sidecar.
#
# Usage: hack/bench/host-collect.sh <label> <secs>
#   label   short case id, e.g. "A0" or "B2-mtu1380" (used in the output filename)
#   secs    how long the peer's iperf3 run lasts
set -eu

LABEL="${1:?usage: host-collect.sh <label> <secs>}"
SECS="${2:?usage: host-collect.sh <label> <secs>}"

DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
OUT_DIR="$DIR/results"
mkdir -p "$OUT_DIR"
OUT="$OUT_DIR/${LABEL}-$(date +%Y%m%d-%H%M%S).txt"

WGPRO_PID="$(pgrep -f /usr/local/bin/wireguard-pro | head -1 || true)"
PASTA_PID="$(pgrep -x pasta | head -1 || true)"

{
	echo "=== ${LABEL} $(date -Iseconds) ==="
	uname -r
	sysctl net.core.rmem_max net.core.wmem_max 2>/dev/null || true

	echo "--- latest bind:/tun:/sockets: journal lines ---"
	journalctl --user-unit wireguard-pro.service --no-pager 2>/dev/null \
		| grep -E 'wireguard-pro: (sockets|bind|tun):' | tail -n 6

	echo "--- nstat baseline (zeroes the counters) ---"
	nstat -az UdpInDatagrams UdpInErrors UdpRcvbufErrors UdpOutDatagrams >/dev/null

	echo "--- sampling for ${SECS}s (wgpro pid=${WGPRO_PID:-?} pasta pid=${PASTA_PID:-?}) ---"
	(
		sleep "$((SECS / 2))"
		echo "--- ss mid-run ---"
		ss -uapmn 'sport = :51820' 2>/dev/null || true
	) &
	SS_PID=$!

	if [ -n "$WGPRO_PID" ]; then
		pidstat -p "$WGPRO_PID" 1 "$SECS" 2>/dev/null &
	fi
	WGPRO_PIDSTAT=$!

	if [ -n "$PASTA_PID" ]; then
		pidstat -p "$PASTA_PID" 1 "$SECS" 2>/dev/null &
	fi
	PASTA_PIDSTAT=$!

	wait "$SS_PID" "$WGPRO_PIDSTAT" "$PASTA_PIDSTAT" 2>/dev/null || true

	echo "--- nstat delta over the run ---"
	nstat -az UdpInDatagrams UdpInErrors UdpRcvbufErrors UdpOutDatagrams
} | tee "$OUT"

echo "results: $OUT"
