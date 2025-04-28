--- FILE: QUICKSTART.md ---

# 🚀 WireGuard Pro — Quickstart Guide

### 🛠 Requirements:
- ✅ Podman (rootless)
- ✅ Systemd user services enabled
- ✅ WireGuard installed on host (for wg syncconf)
- ✅ Rootless networking (slirp4netns or pasta)
- ✅ Linux server (Debian/Ubuntu recommended)

---

# 📦 1. Clone the Project

```bash
git clone https://github.com/yourusername/wireguard-pro.git
cd wireguard-pro
```

---

# 🔒 2. Set up your WireGuard Private Key Secret

```bash
mkdir -p secrets
wg genkey | tee secrets/wg_privatekey | wg pubkey > secrets/wg_publickey
podman secret create wg-pro-privatekey ./secrets/wg_privatekey
```

---

# 🏗️ 3. Build and Deploy

```bash
make deploy
```

That's it! 🎯

---

# 🌐 4. Access the Dashboard

- Visit `http://your-server-ip:10086/`
- Manage peers, view live stats, toggle dark mode, and more!

---

# 📈 5. Extra Commands

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

---

# 📜 API Docs (Swagger UI)

- Visit: `http://your-server-ip:10086/apidocs/`
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

You now have one of the **best WireGuard dashboards** available — open source, rootless, live stats, QR-ready, dark-mode powered, and production-grade. 🚀

---

# 🎁 Bonus Ideas

✅ Deploy buttons for DigitalOcean/Vultr  
✅ Kubernetes Helm Chart  
✅ OAuth2 API Protection  
✅ SaaS Peer Subscription Model

---

# 📣 Let's Launch!

Push your repo live and **become a legend** 🌟.

(If you need help with final launch steps, just ask!) 🚀
