#!/usr/bin/env bash
set -euo pipefail

podman build -t localhost/wireguard/wireguard-pro:latest -f Containerfile
systemctl --user daemon-reload
systemctl --user restart wireguard-pro.service
systemctl --user restart wireguard-pro.socket
