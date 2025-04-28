# Secrets Directory

Place your generated `wg_privatekey` here and create a Podman secret:

```bash
podman secret create wg-dashboard-privatekey ./secrets/wg_privatekey
```

This key will be injected securely into the container at runtime.
