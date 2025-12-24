# 🚀 WireGuard Pro — Quickstart Guide

### 🛠 Requirements:
- ✅ Podman (rootless)
- ✅ Python
- ✅ Wireguard Tools
- ✅ Systemd user services enabled
- ✅ Linux server (Debian, Ubuntu, Arch tested) 

---

# 1. Install WireGuard Tools and Python from your Distro's Package Manager
```bash
sudo apt-get install wireguard-tools python
# etc...
```

---

# 2. Create files for Nftables and environment

Both files are expected to be at %h/.config/wireguard-pro/env (%h is Podman Quadlet shorthand for the home directory). Refer to [env.md](env.md) for a comprehensive list of all environmentally mutable properties and check out [nftables.conf](../container/nftables.conf) to see an example firewall configuration for deploying on a VPS.

---

# 3. Clone the Project and Deploy

```bash
git clone https://github.com/jack-turk-5/wireguard-pro.git
cd wireguard-pro
make deploy
```

That's it!

---

# 4. Access the Dashboard

- Visit `http(s)://ip:51819/`
- Manage peers, view live stats, toggle dark mode, and more!

---

# 5. Extra Commands

| Command | Purpose |
|:---|:---|
| `make build` | Build Podman image |
| `make start` | Start service and socket |
| `make stop` | Stop service and socket |
| `make reload` | Reload container/socket |
| `make upgrade` | Rebuild + reload |
| `make clean` | Full reset |
| `make status` | View systemd status |
| `make logs` | Stream logs live |

Check out the [Makefile](../Makefile) for more details

---

# API Docs (Swagger UI)

- Visit: `http(s)://ip:51819/apidocs/`
- Full auto-generated API explorer (add/delete/list peers live!)

---

# Systemd Quadlet

- `wireguard-pro.container`
- `wireguard-pro.socket`

Deployed automatically by `make start`.

---

# What's Inside

| Component | Purpose |
|:---|:---|
| Flask Backend | WireGuard management API |
| Swagger | API documentation |
| QR Code | VPN config display |
| Chart.js | RX/TX traffic graphs |
| Dark Mode | User toggle |
| Server Info | Uptime/load metrics |

---

# Pro Tips

```bash
# Enable rootless Podman auto-start on boot (may need to be root)
(sudo) loginctl enable-linger $(whoami)

# Easy upgrade
git pull
make upgrade

# Clean reset
make stop
make clean
make deploy
```

---

# Congratulations!

You now have one of the **best** WireGuard dashboards available — open source, rootless, and production-grade.
