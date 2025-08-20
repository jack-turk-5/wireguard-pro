# Secrets Directory

Place your generated `wg-privatekey` here and create a Podman secret:

```bash
podman secret create wg-privatekey ./secrets/wg-privatekey
```

Secrets can be injected into a container at runtime by using the `--secret id=wg-privatekey` flag or with `Secret=wg-privatekey` in quadlet flavor. 

## UI User Setup
The UI will automatically seed one user whose credentials are stored im the `admin-user` and `admin-pass` secrets. If you want to change them, delete the secrets and rerun `make credentials` to rerun the script to re-collect values from stdin.