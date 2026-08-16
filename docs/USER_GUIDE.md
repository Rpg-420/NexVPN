# NexVPN Pro v1.1.0 — Comprehensive User Guide

![NexVPN Banner](https://raw.githubusercontent.com/nexvpn/nexvpn-desktop/main/frontend/public/banner.png)

Welcome to **NexVPN Pro Client**, the next-generation multi-protocol anti-censorship desktop client engineered for Windows. Powered by triple core engines (**Sing-Box**, **Xray-Core**, and **Native OpenVPN**) with **Wintun** kernel-level virtual network adapter acceleration, NexVPN delivers unmatched stability, ultra-low latency, and robust evasion against deep packet inspection (DPI).

---

## 📑 Table of Contents
1. [Installation & First Launch](#1-installation--first-launch)
2. [Adding & Managing Server Profiles](#2-adding--managing-server-profiles)
   - [V2Ray / Xray Links & Subscription Feeds](#v2ray--xray-protocols)
   - [1-Click Cloudflare WARP & WireGuard](#cloudflare-warp--wireguard)
   - [Native OpenVPN (.ovpn) Profiles](#native-openvpn)
3. [Smart Dashboard & Traffic Telemetry](#3-smart-dashboard--traffic-telemetry)
4. [Advanced Anti-Censorship & Performance Tools](#4-advanced-anti-censorship--performance-tools)
   - [DPI Evasion & TLS Fragment Studio](#dpi-evasion--tls-fragment-studio)
   - [Ultra-Low Latency Gaming Mode](#ultra-low-latency-gaming-mode)
   - [Proxy Chaining & Multi-Hop Relay](#proxy-chaining--multi-hop-relay)
   - [DNS Leak Diagnostics & Anti-Poisoning](#dns-leak-diagnostics--anti-poisoning)
   - [Clean IP Scanner for WARP](#clean-ip-scanner-for-warp)
   - [Smart Network Doctor & 1-Click Repair](#smart-network-doctor--1-click-repair)
   - [LAN Wi-Fi Proxy Sharing](#lan-wi-fi-proxy-sharing)
   - [Mobile Remote Control Hub](#mobile-remote-control-hub)
   - [Monthly Data Quota Manager](#monthly-data-quota-manager)
   - [Full Backup, Restore & Cloud Sync](#full-backup-restore--cloud-sync)
5. [System Behavior & Security Configurations](#5-system-behavior--security-configurations)
6. [Desktop Global Hotkeys](#6-desktop-global-hotkeys)
7. [Troubleshooting & Frequently Asked Questions (FAQ)](#7-troubleshooting--faq)

---

## 1. Installation & First Launch

### System Requirements
* **Operating System**: Windows 10 / Windows 11 (64-bit recommended, 32-bit supported)
* **Privileges**: Administrator permissions (required for virtual TUN network adapter initialization and WinINet proxy control)
* **WebView2 Runtime**: Automatically included with modern Windows 10/11 updates

### Starting the Application
1. Launch `NexVPN.exe`.
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
4. Click **Ping All** to test the real TCP/HTTP round-trip latency to all servers simultaneously.
5. Click **Connect** on your preferred server.

### Cloudflare WARP & WireGuard
Get free, unlimited, and fast global internet access with zero configuration:
1. Navigate to the **WireGuard & WARP** tab.
2. Click **1-Click Register WARP Account**. NexVPN securely communicates with Cloudflare's registration API, generates a private WireGuard key pair, and sets up your profile instantly.
3. Enable **Double WARP (WARP-on-WARP)** to route traffic through two nested Cloudflare edge locations for total censorship evasion.
4. Run the **Clean IP Scanner** to find the fastest reachable Cloudflare endpoints on your ISP network.

### Native OpenVPN
1. Navigate to the **OpenVPN** tab.
2. Click **Add .ovpn Profile** and select your configuration file.
3. If the server requires credentials, enter your username and password when prompted.
4. Click **Connect**. NexVPN invokes the native OpenVPN daemon with TAP/Wintun routing.

---

## 3. Smart Dashboard & Traffic Telemetry

The Dashboard provides real-time visibility into your connection:
* **Interactive Connect/Disconnect Orb**: 1-click connection state toggle with glowing visual status.
* **Active Node Details**: Server name, country flag, protocol type, outbound IP, and ISP name.
* **Dual Routing Indicator**: Confirms active status of TUN virtual adapter and system proxy.
* **Live Traffic Monitor**: Real-time download/upload speed gauges, current session duration, and total bandwidth consumed.
* **Traffic Graph**: Smooth dynamic charts plotting real-time data throughput.

---

## 4. Advanced Anti-Censorship & Performance Tools

### DPI Evasion & TLS Fragment Studio
Slices TLS `ClientHello` packets across multiple TCP segments to fool Deep Packet Inspection (DPI) firewalls and unblock SNI-filtered servers.
* **1-Click ISP Presets**:
  * *MCI Ultra Bypass*: Micro-chunks (10-30 bytes) with 10ms jitter for mobile networks.
  * *Irancell Quantum*: Balanced chunking (50-100 bytes) with zero packet drop.
  * *Fixed Line Smooth*: 100-200 byte chunks for ADSL, VDSL, and FTTH fiber networks.
* **Custom Parameters**: Freely adjust fragment packet size and interval delay.

### Ultra-Low Latency Gaming Mode
* Switches Wintun memory stack to low-overhead mode.
* Sets MTU to 1400 and clamps MSS to 1360 to prevent packet fragmentation.
* Prioritizes UDP packets for zero voice delay on Discord, TeamSpeak, and low in-game jitter.
* Integrates anti-sanction gaming DNS resolvers: **Electro DNS**, **Shecan**, **Begzar**, and **Cloudflare Zero Trust**.

### Proxy Chaining & Multi-Hop Relay
Route your connection sequentially across up to 4 nodes (e.g. *Local PC -> Iran Server -> Germany Reality -> Cloudflare WARP -> Target Web*). This provides total anonymity and overcomes severe international firewall throttling.

### DNS Leak Diagnostics & Anti-Poisoning
Tests your active connection against DNS poisoning, DNS hijacking, and query interception. Confirms that all queries are encrypted via DNS-over-HTTPS (DoH) inside the tunnel.

### Clean IP Scanner for WARP
Probes hundreds of Cloudflare Anycast IP ranges concurrently to find clean, unthrottled endpoints for your specific internet service provider.

### Smart Network Doctor & 1-Click Repair
Diagnoses and repairs common Windows networking issues caused by unexpected shutdowns, residual proxy settings, or stuck network drivers.
* Flushes DNS resolver cache (`ipconfig /flushdns`).
* Resets WinINet proxy settings.
* Restores Winsock and TCP/IP stack (`netsh winsock reset`).
* Cleans orphaned adapter states.

### LAN Wi-Fi Proxy Sharing
Enables your PC to act as a local gateway proxy. Other devices on the same Wi-Fi network (smartphones, PlayStation, Xbox, Smart TVs) can route through NexVPN by setting their proxy to your PC's local IP and port.

### Mobile Remote Control Hub
Displays a local QR code and web interface. Scan the QR code with your phone camera to connect/disconnect and monitor NexVPN from your mobile browser without installing extra apps.

### Monthly Data Quota Manager
Track data usage against your monthly ISP plan and receive visual alerts when reaching 80% or 90% of your data threshold.

### Full Backup, Restore & Cloud Sync
Export all your server profiles, custom routing rules, and preferences to an encrypted `.json` file, or synchronize them seamlessly with private cloud storage.

---

## 5. System Behavior & Security Configurations

Access the **Settings** page (`Ctrl + S`) to customize:
* **Routing Rule Mode**:
  * *Bypass Iran & Domestic Sites*: Routes local traffic directly for max speed while tunneling international traffic.
  * *Global Tunnel*: Routes 100% of all traffic through the VPN.
  * *Block Ads & Trackers*: Automatically drops known ad and telemetry domains.
* **Split Tunneling (App Routing)**: Choose specific apps to route through or bypass the VPN tunnel.
* **Kill Switch**: Automatically blocks all internet traffic if the VPN connection drops unexpectedly, preventing IP leaks.
* **Start with Windows**: Launches NexVPN silently in the system tray when Windows boots.
* **Auto-Connect**: Connects to the last used or fastest server on application startup.
* **Color Themes**: Cyber Dark, Midnight Blue, Obsidian Slate, Emerald Matrix.

---

## 6. Desktop Global Hotkeys

Use these system-wide hotkeys from anywhere in Windows (even while playing games or browsing):
* `Ctrl + Shift + C`: Quick Connect / Disconnect VPN
* `Ctrl + Shift + S`: Run Real Speed Test
* `Ctrl + Shift + R`: Open ISP Status Radar
* `Ctrl + Shift + L`: Toggle Wi-Fi LAN Sharing
* `Ctrl + Shift + E`: Emergency Network Repair
* `Ctrl + Shift + Q`: Data Quota Manager

---

## 7. Troubleshooting & FAQ

#### Q: The VPN connects, but websites do not load.
1. Open the **Smart Network Doctor** from the sidebar and click **1-Click Repair**.
2. Go to **Settings** and ensure **DNS Server** is set to `https://1.1.1.1/dns-query` (DoH).
3. If using V2Ray on a mobile network, open **Fragment Studio** and select the preset for your operator.

#### Q: Can I run NexVPN alongside other VPN clients?
NexVPN automatically cleans residual proxy settings from other tools upon launch and exit. However, running two TUN-mode VPNs concurrently is not recommended as they compete for default gateway routing.

#### Q: How do I report bugs or submit feature suggestions?
Click **About** (`Ctrl + I`) to access our official **GitHub Repository** and **Telegram Community Channel**.
