<div align="center">
  <img src="frontend/src/assets/images/logo.png" alt="NexVPN Logo" width="120" height="120">
  <h1>NexVPN Pro</h1>
  <p><b>Next-Generation Multi-Core Anti-Censorship Client</b></p>
  <p>
    <a href="https://github.com/Rpg-420/NexVPN/releases/latest"><img src="https://img.shields.io/github/v/release/Rpg-420/NexVPN?style=for-the-badge&color=00F0FF" alt="Latest Release"></a>
    <a href="https://github.com/Rpg-420/NexVPN/blob/main/LICENSE"><img src="https://img.shields.io/github/license/Rpg-420/NexVPN?style=for-the-badge&color=A855F7" alt="License"></a>
    <img src="https://img.shields.io/badge/Platform-Win%20%7C%20Mac%20%7C%20Linux-10B981?style=for-the-badge" alt="Platforms">
  </p>
  <p>
    <a href="#features">Features</a> •
    <a href="#the-three-engines">Engines</a> •
    <a href="#installation">Installation</a> •
    <a href="#documentation">Documentation</a> •
    <a href="README_FA.md">🇮🇷 توضیحات فارسی</a>
  </p>
</div>

---

**NexVPN Pro** is not just another proxy client. It is an intelligent, cross-platform circumvention toolkit engineered from the ground up using **Go, Wails, and React**. By dynamically unifying the world's most powerful routing cores into a single desktop application, it guarantees absolute internet freedom against severe Deep Packet Inspection (DPI) firewalls.

## ✨ Features

- **🧠 Multi-Core Dispatcher:** Seamlessly switches between Aether (MASQUE), Sing-box, and OpenVPN.
- **🛡️ Smart Network Doctor:** Detects connection drops and automatically repairs frozen TUN adapters, flushes DNS caches, and cleans abandoned system proxies.
- **🎯 Process-Based Split Tunneling:** Bypass the VPN for local apps (like banking or gaming) by simply selecting their executable files. Traffic is intercepted at the process level.
- **⚡ Zero-Leak Kill Switch:** Deep OS integration ensures that if the tunnel drops, your real IP is never exposed.
- **🖥️ Deep OS Integration:**
  - **Linux:** Passwordless execution via Polkit and `cap_net_admin` netlink routing.
  - **Windows:** Deep Registry management and Wintun driver optimization.
  - **macOS:** LaunchAgent autostart and `pfctl` firewall manipulation.

## 🚀 The Three Engines

NexVPN doesn't lock you into a single protocol. It dynamically routes your traffic through the most effective engine available:

### 1. Aether Core (MASQUE / HTTP3)
The flagship anti-censorship engine. It utilizes UDP over HTTP/3 with nested **WARP-in-WARP** tunnels. Features a 5-tier adaptive IP scanner (from *Turbo* to *Ironclad*) that aggressively hunts for clean Cloudflare endpoints when the network is under attack.

### 2. Sing-box Core
The industry standard for modern proxy protocols. It handles **VLESS Reality, Trojan, Hysteria2, and WireGuard**. Features a high-performance Full-System TUN and a built-in Smart DNS router to prevent DNS pollution.

### 3. OpenVPN Core
Native Layer-3 routing for classic configurations. Integrated perfectly with the app's cross-platform firewall chains for zero data leaks.

---

## 🛠️ Installation

Download the latest pre-compiled binaries for Windows, macOS, or Linux from the [Releases](https://github.com/Rpg-420/NexVPN/releases) page.

> [!IMPORTANT]
> **Linux Users:** After downloading and extracting, you **must** run the initialization script to grant the app background execution rights without prompting for root passwords on every connection:
> ```bash
> sudo ./install_linux.sh
> ```

## 🏗️ Build from Source

Requirements:
- [Go](https://golang.org/doc/install) 1.23+
- [Node.js](https://nodejs.org/) 20+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)

```bash
# Clone the repository
git clone https://github.com/Rpg-420/NexVPN.git
cd NexVPN

# Install Wails CLI if you haven't already
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Build for your operating system
wails build -clean
```
The compiled binary will be located in the `build/bin` directory.

## 📚 Documentation

We have prepared beautiful, comprehensive documentation for both users and developers. Visit our Official GitHub Pages:

- 🌐 **[NexVPN Official Website & Docs](https://Rpg-420.github.io/NexVPN/)**

Direct Links:
- 📖 [User Guide (English)](https://Rpg-420.github.io/NexVPN/user-guide-en.html)
- 💻 [Developer Guide (English)](https://Rpg-420.github.io/NexVPN/developer-guide-en.html)

## 🤝 Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

## 📄 License
This project is open-source and available under the [MIT License](LICENSE).
