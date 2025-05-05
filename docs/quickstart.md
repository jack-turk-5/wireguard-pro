# 🚀 WireGuard Pro — Quickstart Guide

### 🛠 Requirements:
- ✅ Podman (rootless)
- ✅ Systemd user services enabled
- ✅ WireGuard installed on host (for wg syncconf)
- ✅ Rootless networking (slirp4netns or pasta)
- ✅ Linux server (Debian/Ubuntu recommended)

---

# 1. Install WireGuard Tools from your Distro's Package Manager
```bash
sudo apt install wireguard-tools
sudo dnf install wireguard-tools
sudo pacman -Syu wireguard-tools
sudo apk add wireguard-tools
# etc...
```

---

# 📦 2. Clone the Project and Deploy

```bash
git clone https://github.com/yourusername/wireguard-pro.git
cd wireguard-pro
make deploy
```

That's it! 🎯

---

# 🌐 3. Access the Dashboard

- Visit `http://your-server-ip:10086/`
- Manage peers, view live stats, toggle dark mode, and more!

---

# 📈 3. Extra Commands

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

Check out the [Makefile](../Makefile) for more details 📖

---

# 📜 API Docs (Swagger UI)

- Visit: `http://your-server-ip:51819/apidocs/`
- Full auto-generated API explorer (add/delete/list peers live!)

---

# ⚙️ Systemd Quadlet

- `wireguard-pro.container`
- `wireguard-pro.socket`

Deployed automatically by `make start`.

---

# 📦 What's Inside

| Component | Purpose |
|:---|:---|
| Flask Backend | WireGuard management API |
| Swagger | API documentation |
| QR Code | VPN config display |
| Chart.js | RX/TX traffic graphs |
| Dark Mode | User toggle |
| Server Info | Uptime/load metrics |
| Socket Activation | Rootless startup |

---

# 🔥 Pro Tips

```bash
# Enable rootless Podman auto-start on boot
loginctl enable-linger $(whoami)

# Easy upgrade
git pull
make upgrade

# Clean reset
make stop
make clean
make deploy
```

---

# 🏆 Congratulations!

You now have one of the **best** WireGuard dashboards available — open source, rootless, live stats, QR-ready, dark-mode powered, and production-grade. 🚀
