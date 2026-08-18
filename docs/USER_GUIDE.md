# NexVPN Pro v1.3.3 — Comprehensive User Guide

![NexVPN Banner](https://raw.githubusercontent.com/Rpg-420/NexVPN/main/frontend/public/banner.png)

Welcome to **NexVPN Pro Client**, the next-generation multi-protocol anti-censorship desktop client engineered for Windows. Powered by triple core engines (**Sing-Box**, **Xray-Core**, and **Native OpenVPN**) with **Wintun** kernel-level virtual network adapter acceleration, NexVPN delivers unmatched stability, ultra-low latency, and robust evasion against deep packet inspection (DPI).

---

## 📑 Table of Contents
1. [Installation & First Launch](#1-installation--first-launch)
2. [Adding & Managing Server Profiles](#2-adding--managing-server-profiles)
   - [V2Ray / Xray Links & Subscription Feeds](#v2ray--xray-protocols)
   - [Cloudflare WARP & AmneziaWG Protocol](#cloudflare-warp--amneziawg-protocol)
   - [Native OpenVPN Profiles with Smart Diagnostics](#native-openvpn-profiles)
   - [Discovery Hub & Live Config Scanner](#discovery-hub--live-config-scanner)
3. [Smart Dashboard & Traffic Telemetry](#3-smart-dashboard--traffic-telemetry)
4. [Advanced Anti-Censorship & Performance Tools](#4-advanced-anti-censorship--performance-tools)
   - [AmneziaWG Obfuscation & Junk Packets](#amneziawg-obfuscation--junk-packets)
   - [DPI Evasion & TLS Fragment Studio](#dpi-evasion--tls-fragment-studio)
   - [Ultra-Low Latency Gaming Mode](#ultra-low-latency-gaming-mode)
   - [Proxy Chaining & Multi-Hop Relay](#proxy-chaining--multi-hop-relay)
   - [Clean IP Anycast Scanner for WARP](#clean-ip-anycast-scanner-for-warp)
   - [Smart Network Doctor & 1-Click Repair](#smart-network-doctor--1-click-repair)
   - [LAN Wi-Fi Proxy Sharing](#lan-wi-fi-proxy-sharing)
   - [Mobile Remote Control Hub](#mobile-remote-control-hub)
   - [Screen QR Code Scanner](#screen-qr-code-scanner)
   - [Universal Config Converter](#universal-config-converter)
   - [Nearby Network Sync](#nearby-network-sync)
5. [System Behavior & Security Configurations](#5-system-behavior--security-configurations)
6. [Desktop Global Hotkeys](#6-desktop-global-hotkeys)
7. [Troubleshooting & Frequently Asked Questions (FAQ)](#7-troubleshooting--faq)

---

## 1. Installation & First Launch

### System Requirements
* **Operating System**: Windows 10 / Windows 11 (64-bit)
* **Privileges**: Administrator permissions (required for virtual TUN network adapter initialization and WinINet proxy control)
* **WebView2 Runtime**: Automatically included with modern Windows 10/11 updates

### Starting the Application
1. Launch `NexVPN.exe` as Administrator.
2. When prompted by Windows UAC (User Account Control), grant administrator permissions.
3. NexVPN will automatically verify that the bundled **Wintun driver**, **Sing-Box**, **Xray-Core**, and **OpenVPN** binaries are present and healthy.

---

## 2. Adding & Managing Server Profiles

### V2Ray / Xray Protocols
NexVPN natively parses and routes all modern proxy protocols:
* **Protocols**: `VLESS`, `VMess`, `Trojan`, `Shadowsocks`, `Hysteria 2`, `TUIC`, `SOCKS5`, `HTTP`.
* **Security & Transports**: `Reality`, `Vision (XTLS)`, `gRPC`, `WebSocket (WS)`, `HTTP/2`, `QUIC`, `mKCP`.

#### Steps to Add:
1. Navigate to the **V2Ray / Xray Servers** tab from the left sidebar.
2. Click **Import Link / Subscription** or press `Ctrl + V` anywhere to paste from clipboard.
3. Enter either a single config URI (`vless://...`, `vmess://...`, `ss://...`, `trojan://...`) or a subscription URL (`https://example.com/sub`).
4. Click **Ping All (⚡)** to test the real round-trip latency to all servers simultaneously.
5. Click **Connect** on your preferred server.

### Cloudflare WARP & AmneziaWG Protocol
1. Open the **WireGuard & WARP** page.
2. Click **1-Click Generate WARP** to automatically register with Cloudflare edge APIs and generate AmneziaWG anti-DPI parameters (`Jc`, `Jmin`, `Jmax`, `H1-H4`).
3. For heavy DPI restrictions, enable **WARP-on-WARP (Double Tunnel)**.
4. Run the **Clean IP Scanner** to benchmark zero-loss Anycast endpoints with the lowest ping.

### Native OpenVPN Profiles
1. Open the **OpenVPN** page.
2. Click **Add .ovpn File** and select your configuration.
3. Enter username and password if required.
4. The Engine applies MSS clamping (1360) and 512-packet anti-jitter replay windows. If authentication fails or server is unreachable, an actionable diagnostic popup explains the error immediately.

### Discovery Hub & Live Config Scanner
1. Open the **Discovery Hub & Toolbox** page.
2. Click **Fetch Configs** to aggregate verified public nodes.
3. Filter by protocol (**VLESS**, **VMESS**, **TROJAN**, **SHADOWSOCKS**, **HYSTERIA2**).
4. Run **Test Latency & Health** to discard inactive nodes, and import working servers with 1 click.

---

## 3. Smart Dashboard & Traffic Telemetry

* **Connect Orb Button**: High-visibility glowing power orb with live status feedback.
* **Smart Connect (⚡)**: Auto-connect to the lowest latency node with a single click.
* **Server Overview Pill**: Visual flag badge, ISP name, outbound IP, and active protocol.
* **Error Diagnostic Card**: Displays specific failure details with instant access to the Smart Doctor.
* **Real-Time Traffic Graph**: Zero-lag bandwidth usage chart and live download/upload speedometer.

---

## 4. Advanced Anti-Censorship & Performance Tools

### AmneziaWG Obfuscation & Junk Packets
In restricted networks, vanilla WireGuard handshakes are often blocked by DPI systems. NexVPN incorporates full AmneziaWG packet obfuscation:
* Injects randomized junk packet bursts (`Jc=5`, `Jmin=40`, `Jmax=80`).
* Applies custom packet magic headers (`H1`-`H4`) to camouflage handshakes.

### DPI Evasion & TLS Fragment Studio
* Slices TLS `ClientHello` packets into micro-fragments to bypass middlebox SNI sniffers.
* Built-in 1-click presets for major mobile and broadband ISPs.

### Ultra-Low Latency Gaming Mode
* Optimizes MTU to 1400 and MSS to 1360 to eliminate packet fragmentation.
* Prioritizes Discord voice and game traffic.
* Anti-sanction DNS resolvers (**Electro**, **Shecan**, **Begzar**, **Cloudflare Zero Trust**).

### LAN Wi-Fi Proxy Sharing
* Press `Ctrl + Shift + L` to turn your PC into an encrypted local proxy gateway.
* Connect Smart TVs, game consoles, and mobile devices to your PC's local IP and port `10809`.

### Mobile Remote Control Hub
* Scan the displayed QR code with your smartphone to open the responsive remote web dashboard.
* Switch nodes, monitor speed, and toggle connections directly from your phone browser.

### Screen QR Code Scanner
* Click **Scan QR from Screen** to automatically detect and import QR config codes displayed on your desktop without needing a camera.

### Universal Config Converter
* Convert between JSON, URI links, Clash, Sing-box, and V2Ray configuration formats in real time.

---

## 5. System Behavior & Security Configurations

* **Firewall Kill Switch**: Prevents IP leaks by halting non-VPN outbound traffic on disconnection.
* **Smart Domestic Routing (Bypass Iran)**: Local Iranian websites and banking portals route directly at full local speed.
* **Privacy Shield**: Automatically masks credentials, tokens, and UUIDs in runtime application logs.

---

## 6. Desktop Global Hotkeys

| Shortcut | Action |
| :--- | :--- |
| `Ctrl + Shift + C` | Toggle VPN Connect / Disconnect |
| `Ctrl + Shift + S` | Run Real-Time Speed Benchmark |
| `Ctrl + Shift + R` | Open ISP Status Radar |
| `Ctrl + Shift + L` | Toggle Wi-Fi LAN Proxy Sharing |
| `Ctrl + Shift + E` | Trigger Emergency Network Repair (Doctor) |
| `Ctrl + Shift + D` | Open DNS Leak Test |
| `Ctrl + Shift + F` | Launch DPI Evasion & Fragment Studio |

---

## 7. Troubleshooting & FAQ

### 1. No internet after disconnecting from VPN?
Press `Ctrl + Shift + E` or click **Smart Doctor** to instantly clear WinINet proxy settings, flush DNS cache, and reset Winsock.

### 2. WARP connects but websites do not load?
Open **WireGuard & WARP**, click **Clean IP Scanner**, run a scan, and pick a clean Anycast endpoint with low latency.

---

<div align="center">

**NexVPN Pro — The Ultimate Secure, Fast, and Uncensored Experience**

[Official Telegram](https://t.me/nexvpnpro) • [GitHub Repository](https://github.com/Rpg-420/NexVPN)

</div>
