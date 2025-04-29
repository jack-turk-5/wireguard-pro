# Secrets Directory

Place your generated `wg-privatekey` here and create a Podman secret:

```bash
podman secret create wg-privatekey ./secrets/wg-privatekey
```

This key will be injected securely into the container at runtime.

## Hint
Run `make secrets` to generate WireGuard keypair if it doesn't exist yet