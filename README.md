# WireGuard Pro

[![Release and Publish Image](https://github.com/jack-turk-5/wireguard-pro/actions/workflows/release.yml/badge.svg)](https://github.com/jack-turk-5/wireguard-pro/actions/workflows/release.yml)
[![License](https://img.shields.io/badge/license-BSD-blue)](LICENSE.md)
[![Rootless Podman](https://img.shields.io/badge/podman-rootless-blueviolet)](https://podman.io/)

A rootless, socket-activated WireGuard VPN dashboard: a single Go binary that brings up the WireGuard interface, manages peers, and serves an API and web dashboard, deployed as a rootless Podman Quadlet.

## Features

- Rootless Podman deployment, socket-activated for both the dashboard and the VPN listener
- Dynamic peer creation and deletion via the dashboard or API
- Auto-expiring peers, swept on a fixed interval
- Live VPN traffic graphs (RX/TX) and per-peer handshake status
- QR code generator for mobile client setup
- Server uptime/load metrics
- Dark mode

## Architecture

The application is a single statically-linked Go binary (`cmd/wireguard-pro`) that:

- Brings up the `wg0` interface via `wireguard-go`, using a systemd-activated socket for the VPN UDP listener when available
- Applies the nftables ruleset directly via netlink (no shelling out to `nft(8)`)
- Manages peers through `wgctrl` against a SQLite database, which is the sole source of truth for peer state
- Serves the dashboard API and the built Angular frontend (embedded in the binary) over HTTP
- Runs an expiry sweep for auto-expiring peers

See [docs/quickstart.md](docs/quickstart.md) for deployment instructions and [docs/env.md](docs/env.md) for the full list of configuration variables.

## Quickstart

```bash
git clone https://github.com/jack-turk-5/wireguard-pro.git
cd wireguard-pro
make deploy
```

Visit `http(s)://<host>:51819/` to open the dashboard.

For full setup instructions, see [docs/quickstart.md](docs/quickstart.md).

## Testing

The test suite runs in an isolated container; only Podman is required on the host:

```bash
make test
```

See [hack/Containerfile.test](hack/Containerfile.test) for details.

## Secrets

The admin dashboard user is seeded from Podman secrets:

```bash
make credentials
```

This prompts for and creates the `admin-user`/`admin-pass` secrets if they don't already exist. The WireGuard server key pair is generated automatically on first boot and persisted to a Podman volume; no secret is needed for it.

## Common commands

| Command | Purpose |
|:---|:---|
| `make build` | Build the container image and reload systemd |
| `make test` | Run the test suite in an isolated container |
| `make start` | Start the service and socket |
| `make stop` | Stop the service and socket |
| `make reload` | Reload the container/socket |
| `make upgrade` | Rebuild and reload |
| `make clean` | Remove the container, image, and volumes |
| `make status` | View systemd unit status |
| `make logs` | Stream logs |

See the [Makefile](Makefile) for the full list of targets.

## License

3-Clause BSD License. See [LICENSE.md](LICENSE.md) for details.

## Credits

Inspired by [donaldzou/WGDashboard](https://github.com/donaldzou/WGDashboard) and [wg-easy/wg-easy](https://github.com/wg-easy/wg-easy).

---

For Madelyn.
