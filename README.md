# WireGuard Pro 🛡️

[![Build](https://img.shields.io/badge/build-passing-brightgreen)](https://github.com/yourusername/wireguard-pro/actions)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Rootless-Podman](https://img.shields.io/badge/podman-rootless-blueviolet)](https://podman.io/)
[![API Docs](https://img.shields.io/badge/docs-Swagger-informational)](http://your-server-ip:10086/apidocs/)

🚀 **Rootless, Dynamic, API-Driven WireGuard VPN Dashboard**  
🌐 **Socket Activated via Systemd Quadlet**  
⚡ **Live Traffic Stats, QR Codes, Dark Mode**
🎯 **Zero Downtime Upgrades**

---

## 🌟 Features

- 100% rootless Podman deployment
- Dynamic peer creation & deletion via API
- Swagger UI API documentation
- Auto-expiring peers support
- Live VPN traffic graphs (RX/TX)
- QR Code generator for mobile VPN setup
- Dark Mode toggle
- Server uptime/load metrics display
- Fully socket-activated (super fast startup)

---

## 📦 Quickstart

```bash
git clone https://github.com/yourusername/wireguard-pro.git
cd wireguard-pro
make deploy
```

Visit `http(s)://ip:51819/` to open your new dashboard!

For setup instructions: see [quickstart.md](docs/quickstart.md)

---

## 📜 API Documentation

- Swagger UI: [http://your-server-ip:10086/apidocs/](http://your-server-ip:10086/apidocs/)

Example Endpoints:
- `POST /api/peers/new` → Create peer
- `POST /api/peers/delete` → Remove peer
- `GET /api/peers/list` → List peers
- `GET /api/peers/stats` → Live stats

---

## 📈 Live Stats + Charts

- RX and TX traffic updated every 10 seconds
- Last handshake time health-colored
- Server uptime and load averages shown

---

## 🔒 Secrets

Secrets managed via Podman Secrets:

```bash
podman secret create wg-privatekey ./secrets/wg_privatekey
```
See [quickstart.md](docs/quickstart.md) for more info on secret generation

---

## 🔥 Pro Tips

```bash
# Upgrade system safely -- minimal (ms) downtime
make upgrade

# Watch logs
make logs

# Clean rebuild
make clean
make deploy
```

---

## 📜 License

3-Clause BSD License. See [LICENSE](LICENSE) for details.

---

## 🎯 Credits

- Inspired by [donaldzou/WGDashboard](https://github.com/donaldzou/WGDashboard) and [wg-easy/wg-easy](https://github.com/wg-easy/wg-easy)

---

# 🎁 Future Additions

✅ Kubernetes Helm Chart  
✅ OAuth2.0 Integration
✅ 2-Factor Authentication

---

**Let's go Pro! 🚀**

---

**For Madelyn❤️**