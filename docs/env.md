# Environment Variables

Every environment variable that affects WireGuard Pro's behavior, read by `internal/config` and `cmd/wireguard-pro/main.go`. Set these in `%h/.config/wireguard-pro/env` (referenced by `EnvironmentFile=` in the Quadlet unit).

## Required

| Variable | Description |
|:---|:---|
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
| `LOG_LEVEL` | `error` | Controls wireguard-go's own device logger: `silent`, `error`, or `verbose`. `verbose` logs every handshake attempt/keepalive -- useful when debugging, too noisy for normal operation. |
| `WG_MTU` | `1420` | The `wg0` interface's MTU (valid range 1280-65535). See [design-doc.md](design-doc.md) §2.5 on why this is independent of pasta's tap MTU -- forwarded GSO packets are re-segmented at `gso_size` regardless of tap MTU. |

## Go runtime tuning

`GOGC` and `GOMAXPROCS` aren't read by this application directly -- they're
the Go runtime's own standard environment variables, and since
`EnvironmentFile=` in the Quadlet unit passes the whole env file straight
through to the process, setting them there works without any code change.
`GOMAXPROCS` is usually best left unset: Go >= 1.25's runtime is
cgroup-aware and already sizes itself to the container's CPU quota. `GOGC`
is the one worth experimenting with under load (see the `hack/bench/`
results table) if GC pauses show up as jitter in the throughput numbers.

## Secrets (not environment variables)

These are read from files, not environment variables, so they can be provided as Podman secrets (mounted at `/run/secrets/<name>`):

| File | Purpose |
|:---|:---|
| `admin-user`, `admin-pass` | Seed or rotate the dashboard's admin account on startup. See `make credentials`. |

The WireGuard server key pair is generated automatically on first boot and persisted to `/etc/wireguard/privatekey` (backed by the `wg-keydata` Podman volume); no secret is needed for it.

The dashboard's JWT signing key works the same way: generated automatically on first boot and persisted next to the database (`secret_key` alongside `DB_FILE`, so it follows `DB_FILE` if you override it) rather than being set via the environment.
