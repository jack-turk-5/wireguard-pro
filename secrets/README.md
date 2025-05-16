# Secrets Directory

Place your generated `wg-privatekey` here and create a Podman secret:

```bash
podman secret create wg-privatekey ./secrets/wg-privatekey
```

This key will be injected securely into the container at runtime.

## Efficiency Hint
Run `make secrets` to generate WireGuard keypair if it doesn't exist yet

## Initial User Setup
To generate admin credentials for the UI, set `ADMIN_USERNAME` and `ADMIN_PASSWORD` environment variables in `~/.config/wireguard-pro/env`