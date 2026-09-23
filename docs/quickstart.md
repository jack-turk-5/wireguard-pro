# Quickstart Guide

## Requirements

- Podman (rootless), with `pasta` for rootless networking
- Systemd user services enabled (`loginctl enable-linger <user>` if running unattended)
- A Linux host with a `tun` kernel module available
- `wireguard-tools` (optional; only needed if you want to inspect the running device with `wg show`)

## 1. Clone the repository

```bash
git clone https://github.com/jack-turk-5/wireguard-pro.git
cd wireguard-pro
```

## 2. Create the config directory

The Quadlet unit expects an environment file and the nftables ruleset at `%h/.config/wireguard-pro/` (`%h` is Quadlet shorthand for your home directory):

```bash
mkdir -p ~/.config/wireguard-pro
cp container/nftables.json ~/.config/wireguard-pro/nftables.json
```

Create `~/.config/wireguard-pro/env` with at least the required variables:

```bash
cat > ~/.config/wireguard-pro/env <<'EOF'
WG_HOST=vpn.example.com
WG_PORT=51820
EOF
chmod 600 ~/.config/wireguard-pro/env
```

See [env.md](env.md) for the full list of variables, including optional ones.

## 3. Deploy

```bash
make deploy
```

This creates the `admin-user`/`admin-pass` secrets (prompting for values on first run), builds the image, and starts the service and socket.

## 4. Access the dashboard

Visit `http(s)://<host>:51819/` and sign in with the credentials from step 3.

## Common commands

See the [Makefile](../Makefile) for the full list; the most common ones:

```bash
make upgrade   # rebuild and reload
make logs      # stream logs
make status    # view systemd unit status
make stop      # stop the service and socket
make clean     # remove the container, image, and volumes
```

## Testing changes

The test suite runs in an isolated container and only requires Podman on the host:

```bash
make test
```

## Upgrading

### The socket unit now binds four sockets

`wireguard-pro.socket` binds separate IPv4/IPv6 sockets for both the
dashboard and the VPN listener (four `Listen*=` lines total) instead of one
dual-stack pair, so the VPN UDP path can use the full `recvmmsg`/`sendmmsg`,
GSO/GRO batch path per address family. If you deployed before this change,
reinstall the unit and reload:

```bash
make build   # installs the updated quadlet/wireguard-pro.socket
make reload
```

A bare `systemctl --user restart wireguard-pro.socket` is **not** enough:
systemd only hands activated file descriptors to the service at service
*start*, not on a socket-only restart, so the service would keep running
against its old (now-stale) fd set until it's restarted too -- `make reload`
restarts both.

### Optional: raise the host's UDP socket buffer ceiling

Only apply this if `UdpRcvbufErrors` (see `hack/bench/host-collect.sh`) is
still climbing under load after the socket-unit upgrade above -- the
batched receive path already absorbs the vast majority of what used to
overflow the old one-packet-at-a-time bind. This is a one-shot, root-only
host step; nothing at runtime needs root.

```bash
sudo tee /etc/sysctl.d/90-wireguard-pro.conf <<'EOF'
net.core.rmem_max=7340032
net.core.wmem_max=7340032
EOF
sudo sysctl --system
```
