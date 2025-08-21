# WireGuard Pro 🛡️

[![Release and Publish Image](https://github.com/jack-turk-5/wireguard-pro/actions/workflows/release.yml/badge.svg)](https://github.com/jack-turk-5/wireguard-pro/actions/workflows/release.yml)
[![License](https://img.shields.io/badge/license-BSD-blue)](LICENSE)
[![Rootless-Podman](https://img.shields.io/badge/podman-rootless-blueviolet)](https://podman.io/)

🚀 **Rootless, Dynamic, API-Driven WireGuard VPN Dashboard**  
🌐 **Socket Activated via Systemd Quadlet**  
⚡ **Live Traffic Stats, QR Codes, Dark Mode**
🎯 **Zero Downtime Upgrades**

---

## 🌟 Features

- 100% Rootless Podman deployment
- Dynamic peer creation & deletion via API
- Swagger UI API documentation
- Auto-expiring peers
- Live VPN traffic graphs (RX/TX)
- QR Code generator for mobile VPN setup
- Server uptime/load metrics display

---

## 📦 Quickstart

```bash
git clone https://github.com/jack-turk-5/wireguard-pro.git
cd wireguard-pro
make deploy
```

Visit `http(s)://ip:51819/` to open the dashboard!

For setup instructions: see [quickstart.md](docs/quickstart.md)

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

- Kubernetes Helm Chart  
- OAuth2.0 Integration
- 2-Factor Authentication
- Swagger API Docs

---

**Let's go Pro! 🚀**

---

**For Madelyn❤️**