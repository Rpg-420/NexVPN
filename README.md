# NexVPN Pro

![NexVPN Pro Banner](https://raw.githubusercontent.com/NexVPN/NexVPN/main/build/appicon.png)

**NexVPN Pro** is a next-generation, cross-platform, multi-protocol circumvention client and VPN engine designed for maximum resilience against Deep Packet Inspection (DPI) and severe network restrictions. Powered by Wails, React, and Go, NexVPN unifies the world's most advanced routing and obfuscation cores into a single, seamless, and intelligent interface.

---

## 🌟 Key Features

### 🧠 Smart Multi-Core Engine
NexVPN does not rely on a single protocol. It intelligently dispatches connections to the most suitable underlying core:
- **Aether Core**: Our flagship user-space engine built for extreme censorship. Supports **MASQUE** (HTTP/3 over UDP/TCP) and **gool** (Cloudflare WARP-in-WARP nested tunneling). Features multi-staged adaptive scanners (Turbo, Balanced, Thorough, Stealth, Ironclad) for hunting clean IP endpoints.
- **Sing-box Core**: Universally acclaimed for Xray/V2Ray protocols (VLESS, Trojan, Hysteria2) and high-performance WireGuard tunneling. Supports advanced routing, Reality, and full-system TUN mode.
- **OpenVPN Core**: Native layer-3 routing with TAP/Wintun integration.

### 🛡️ Cross-Platform System Integration
NexVPN acts natively on your operating system:
- **Windows**: Deep Wintun integration, registry-level proxy management, and process-based split tunneling.
- **Linux**: Passwordless core execution via **Polkit** (`pkexec` and `CAP_NET_ADMIN` support), automated KDE/GNOME D-Bus proxy refreshment, and native `netlink` routing.
- **macOS**: System proxy bypass configurations, `pfctl` firewall anchors, and native LaunchAgent autostart integration.

### 🩺 Smart Network Doctor & Repair
Network acting up? NexVPN's cross-platform Network Doctor can diagnose and repair your system automatically. It resets frozen network adapters, clears DNS caches (via `ipconfig`, `resolvectl`, or `dscacheutil`), cleans abandoned proxy environments, and force-kills orphaned engine processes.

### 🎯 Process-Based Split Tunneling
Route only the apps you want through the VPN. NexVPN intercepts connections and dynamically routes traffic based on the originating process executable, allowing you to bypass domestic apps while tunneling international traffic.

### 🔒 Native Kill Switch
Prevents accidental data leaks by modifying system firewalls (`netsh` on Windows, `iptables` chains on Linux, `pfctl` on macOS) to drop outbound traffic if the core engine drops unexpectedly.

---

## 🚀 Getting Started

### Prerequisites
- [Go 1.21+](https://golang.org/doc/install)
- [Node.js 18+](https://nodejs.org/en/download/)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)

### Installation & Build

1. **Clone the repository:**
   ```bash
   git clone https://github.com/NexVPN/NexVPN.git
   cd NexVPN
   ```

2. **Linux Pre-requisites Setup (Linux Only):**
   To enable passwordless execution and full TUN mode without repeated root prompts, install the Polkit policies:
   ```bash
   sudo ./install_linux.sh
   ```

3. **Development Mode:**
   Run the app in live development mode with hot-reload:
   ```bash
   wails dev
   ```

4. **Production Build:**
   Compile the final executable for your platform:
   ```bash
   wails build
   ```

---

## 📖 Documentation

For detailed guides, please refer to our GitHub Pages documentation or the internal `docs/` folder:
- [User Guide (English)](docs/USER_GUIDE.md)
- [Developer Guide (English)](docs/DEVELOPER_GUIDE.md)

*Persian documentation is also available:*
- [راهنمای کاربر (فارسی)](docs/USER_GUIDE_FA.md)
- [راهنمای توسعه‌دهندگان (فارسی)](docs/DEVELOPER_GUIDE_FA.md)

---

## 🛠️ Project Architecture

NexVPN relies on a modular architecture:
- **Frontend (`/frontend`)**: A responsive UI built with React, TypeScript, and Tailwind CSS.
- **Backend (`/internal`)**: Go-based controllers handling system proxies (`sysproxy`), protocol adapter dispatching (`protocols`), automated binary downlaods (`core`), and system repairs (`parser/doctor`).
- **Cores (`/bin`)**: Target runtime binaries (e.g., `sing-box`, `aether`, `openvpn`) downloaded on-demand or bundled during the build.

---

## 🤝 Contributing
We welcome contributions! Whether it's adding new protocol adapters, improving the UI, or fixing system-specific bugs, feel free to submit a Pull Request.

## 📄 License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
