# Secrets Directory

`create_credentials.py` creates the `admin-user`/`admin-pass` Podman secrets, prompting for values on stdin. Run it directly, or via `make credentials`:

```bash
make credentials
```

If a secret already exists, it's left as-is (the script skips it rather than prompting again). To change the admin credentials, delete the existing secrets first, then rerun:

```bash
podman secret rm admin-user admin-pass
make credentials
```

The dashboard reads these secrets on startup to seed or rotate the admin account.

The WireGuard server key pair does not use a secret: it's generated automatically on first boot and persisted to the `wg-keydata` Podman volume (see [env.md](../docs/env.md)).
