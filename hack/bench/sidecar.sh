#!/bin/sh
# sidecar.sh builds and runs the benchmark sidecar in the running
# wireguard-pro container's network namespace, then starts an iperf3 server
# bound to the wg0 address -- this measures the WG hop only (wireguard-go +
# TUN), never crossing pasta, since both processes share one netns.
#
# Usage: hack/bench/sidecar.sh [wg-addr]
#   wg-addr defaults to 10.8.0.1 (WG_IPV4_BASE_ADDR's default).
set -eu

DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
IMAGE=wireguard-pro-bench-sidecar
WG_ADDR="${1:-10.8.0.1}"

podman build -t "$IMAGE" -f "$DIR/Containerfile.sidecar" "$DIR"

exec podman run --rm -it \
	--network=container:wireguard-pro \
	"$IMAGE" "
		echo '--- ip -s link show wg0 ---'
		ip -s link show wg0
		echo '--- ethtool -k wg0 ---'
		ethtool -k wg0 2>&1 || true
		echo '--- ip route ---'
		ip route
		echo '--- iperf3 server on ${WG_ADDR} (Ctrl-C to stop) ---'
		exec iperf3 -s -B ${WG_ADDR}
	"
