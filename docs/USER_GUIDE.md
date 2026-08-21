# NexVPN Pro - User Guide

Welcome to **NexVPN Pro**, the ultimate multi-protocol, multi-core desktop client for internet freedom.

## Table of Contents
1. [Core Engines](#core-engines)
2. [Adding Profiles](#adding-profiles)
3. [Smart Network Doctor](#smart-network-doctor)
4. [App Split Tunneling](#app-split-tunneling)
5. [Cross-Platform Support](#cross-platform-support)

---

## Core Engines
NexVPN intelligently routes your traffic using three dedicated networking cores:

### 1. Aether Core (MASQUE & gool)
Aether is our most advanced anti-censorship engine. It specializes in penetrating deep packet inspection (DPI) by wrapping traffic in HTTP/3 (MASQUE) or using nested Cloudflare WARP-in-WARP tunnels (gool).
- **Scan Modes**: Select between Turbo, Balanced, Thorough, Stealth, and Ironclad depending on your network conditions.
- **Protocols**: Supports `masque` and `gool`.
- **Usage**: Go to the **Aether** tab, click **Add Profile**, select your protocol and scanner, and connect.

### 2. Sing-box Core (V2Ray & WireGuard)
The industry standard for VLESS, Trojan, Hysteria2, and direct WireGuard/Cloudflare WARP connections.
- **Smart DNS**: Automatically bypasses DNS poisoning.
- **Multi-Queue TUN**: Provides high-performance full-system routing.

### 3. OpenVPN Core
Native Layer-3 routing for classic VPN configurations (`.ovpn` files).

---

## Adding Profiles
- **Clipboard/URI**: Copy any `vless://`, `vmess://`, or `trojan://` link and click the **+** (Import from Clipboard) button in the V2Ray tab.
- **Subscription**: Paste a subscription URL in the Discovery Hub to automatically fetch and update node lists.
- **Cloudflare WARP**: Go to the WARP tab to generate a custom WARP profile with Clean IPs.
- **Aether Profile**: Go to the Aether tab and create a custom MASQUE or gool profile.

---

## Smart Network Doctor
If you experience "No Internet" or proxy errors after a disconnection, the **Smart Network Doctor** can fix it instantly:
- Click the **Network Doctor** button in the sidebar.
- It will automatically:
  - Flush DNS Caches (`ipconfig`, `resolvectl`, `dscacheutil`).
  - Clear abandoned system proxies.
  - Reset frozen Wintun/TUN adapters.
  - Kill orphaned core processes (`aether`, `sing-box`, `openvpn`).

## Linux Polkit Setup (Important)
If you are using NexVPN on Linux, you must run the provided initialization script to configure background capabilities and passwordless execution. If you skip this step, NetworkManager or Polkit will repeatedly ask for your root password when connecting.

1. Open your terminal in the NexVPN installation folder.
2. Run the script with sudo:
   ```bash
   sudo ./install_linux.sh
   ```
3. It will automatically assign `cap_net_admin` capabilities and install Polkit policies.

---

## App Split Tunneling
Bypass the VPN for local/domestic apps (like local banking or gaming apps):
1. Open **App Split Tunneling**.
2. Add the applications you want to bypass (e.g., `chrome.exe` on Windows, or `spotify` on macOS/Linux).
3. The routing engine will automatically route these apps outside the VPN tunnel.

---

## Cross-Platform Support
- **Windows**: Full Wintun integration and registry proxy management.
- **Linux**: Passwordless core execution (via Polkit). Run `sudo ./install_linux.sh` once after installation to enable seamless TUN connectivity. KDE/GNOME proxies are updated instantly via D-Bus.
- **macOS**: Native pfctl Kill Switch, System Settings proxy configuration, and LaunchAgent auto-start integration.
