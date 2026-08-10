# Environment Variables

Every environment variable that affects WireGuard Pro's behavior, read by `internal/config` and `cmd/wireguard-pro/main.go`. Set these in `%h/.config/wireguard-pro/env` (referenced by `EnvironmentFile=` in the Quadlet unit).

## Required

| Variable | Description |
|:---|:---|
| `SECRET_KEY` | Secret key used to sign dashboard bearer tokens. Generate a strong random value and keep it stable across restarts, or existing sessions will be invalidated. |
| `WG_HOST` | The hostname or IP address clients use to reach this server, returned to the dashboard as part of the server config. |
| `WG_PORT` | The WireGuard UDP port clients connect to. Must match the `ListenDatagram=` port configured in `wireguard-pro.socket`. |

## Optional

| Variable | Default | Description |
|:---|:---|:---|
| `WG_ALLOWED_IPS` | `0.0.0.0/0, ::/0` | AllowedIPs value returned to clients in their config (controls what traffic clients route through the tunnel). |
| `WG_DNS_SERVER` | `1.1.1.1` | DNS server returned to clients in their config. |
| `WG_IPV4_BASE_ADDR` | `10.8.0.1` | The `wg0` interface's own IPv4 address; must be the first address in its /24. Peer addresses are allocated sequentially from this base. |
| `WG_IPV6_BASE_ADDR` | `fd86:ea04:1111::1` | The `wg0` interface's own IPv6 address; must be the first address in its /64. Peer addresses are allocated sequentially from this base. |
| `DB_FILE` | `/data/peers.db` | Path to the SQLite database file storing peers and dashboard users. |
| `NFT_CONF_FILE` | `/etc/nftables.json` | Path to the nftables ruleset, in libnftables JSON schema (see [nftables.json](../container/nftables.json)). Applied directly via netlink at startup. |
| `FRONTEND_DIR` | (unset) | If set, serves the dashboard frontend from this directory instead of the build embedded in the binary. Useful for dropping in a new frontend build without rebuilding the image. |

## Secrets (not environment variables)

These are read from files, not environment variables, so they can be provided as Podman secrets (mounted at `/run/secrets/<name>`):

| File | Purpose |
|:---|:---|
| `admin-user`, `admin-pass` | Seed or rotate the dashboard's admin account on startup. See `make credentials`. |

The WireGuard server key pair is generated automatically on first boot and persisted to `/etc/wireguard/privatekey` (backed by the `wg-keydata` Podman volume); no secret is needed for it.
