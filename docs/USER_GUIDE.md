# NexVPN Pro - Comprehensive User Guide

Welcome to the complete user manual for **NexVPN Pro**, the most versatile, high-performance anti-censorship desktop client for Windows, Linux, and macOS.

---

## 📑 Table of Contents
1. [Core Philosophy & Getting Started](#1-core-philosophy--getting-started)
2. [Multi-Engine Routing Cores](#2-multi-engine-routing-cores)
3. [Adding & Managing Profiles](#3-adding--managing-profiles)
4. [Discovery Hub & Live Scrapers](#4-discovery-hub--live-scrapers)
   - [Telegram Proxy Crawler](#telegram-proxy-crawler)
   - [Cloudflare Clean IP Scanner](#cloudflare-clean-ip-scanner)
   - [TLS Fragment Studio](#tls-fragment-studio)
   - [Free Configs Aggregator](#free-configs-aggregator)
5. [Process-Based App Split Tunneling](#5-process-based-app-split-tunneling)
6. [Zero-Leak Kill Switch & DNS Shield](#6-zero-leak-kill-switch--dns-shield)
7. [Smart Network Doctor & Emergency Repair](#7-smart-network-doctor--emergency-repair)
8. [Mobile Remote & LAN Proxy Sharing](#8-mobile-remote--lan-proxy-sharing)
9. [Backup, Cloud Sync & Nearby P2P](#9-backup-cloud-sync--nearby-p2p)
10. [Gaming Mode & Desktop Floating Widget](#10-gaming-mode--desktop-floating-widget)
11. [Troubleshooting & Operator Optimization](#11-troubleshooting--operator-optimization)

---

## 1. Core Philosophy & Getting Started

NexVPN Pro operates on a simple premise: **No single protocol can defeat modern censorship indefinitely.** When ISPs deploy SNI-blocking, UDP throttling, or Deep Packet Inspection (DPI), NexVPN allows you to instantly pivot between cutting-edge protocols (VLESS Reality, MASQUE HTTP/3, Hysteria 2, WireGuard, OpenVPN, and Psiphon) without switching applications.

### Quick Connection in 3 Steps:
1. **Choose or Import a Server**: Paste a link (`vless://`, `ss://`, `vmess://`, `warp://`), import a `.ovpn` file, or fetch free configs from the **Discovery Hub**.
2. **Select Proxy Mode**:
   - **TUN Mode (Full System)**: Creates a virtual network adapter (Wintun / Linux TUN) that routes all system traffic (browsers, games, terminal, apps) through the encrypted tunnel.
   - **System Proxy Mode**: Configures OS HTTP/SOCKS5 system settings, ideal for quick browsing without driver permissions.
3. **Click Connect**: Hit the large glowing Connect button or use the System Tray menu near your desktop clock.

---

## 2. Multi-Engine Routing Cores

NexVPN integrates five distinct routing engines:

### 🔹 Aether Core (MASQUE / HTTP3)
- **What it does**: Encapsulates UDP packets within HTTP/3 streams and sets up nested Cloudflare WARP-in-WARP tunnels.
- **When to use**: During severe regional throttling or when standard TLS protocols fail.
- **Scanner Modes**:
  - *Turbo*: Fast scan for immediate connection.
  - *Balanced*: Optimal blend of latency and reliability.
  - *Stealth*: Obfuscated packet timing to avoid DPI pattern recognition.
  - *Ironclad*: Deep scan through hundreds of IPv4 endpoints for maximum stability.

### 🔹 Sing-box Core
- **What it does**: High-efficiency multi-protocol core supporting **VLESS Reality**, **Trojan**, **Hysteria 2**, **TUIC v5**, and **WireGuard**.
- **When to use**: For ultra-fast streaming (4K YouTube, Twitch) and high-speed downloads with minimum CPU usage.

### 🔹 Xray Core
- **What it does**: Industry standard for V2Ray protocols with support for gRPC, WebSocket, TCP, and TLS Camouflage.

### 🔹 OpenVPN Core
- **What it does**: Layer-3 TUN VPN routing with support for username/password authentication, custom certificates, and `.ovpn` files.

### 🔹 Psiphon Emergency Core
- **What it does**: Tactical obfuscated SSH tunnel engine designed for extreme blackout conditions when all standard proxy ports are blocked.

---

## 3. Adding & Managing Profiles

### Methods to Add Profiles:
- **Clipboard Import**: Copy any URI (`vless://`, `vmess://`, `ss://`, `trojan://`, `hy2://`, `tuic://`, `warp://`) and click the `+` button in the navigation bar.
- **Subscription Links**: Add your subscription URL in the Discovery Hub to automatically fetch and keep your server list synchronized.
- **Screen QR Scanner**: Click the QR icon to capture a QR code directly from your monitor, webcam, or a saved screenshot.
- **WARP & WARP+ Generator**: Go to the WARP tab and generate a new Cloudflare WARP license key and clean endpoint configuration with one click.

---

## 4. Discovery Hub & Live Scrapers

The **Discovery Hub** is your built-in radar for fresh proxy endpoints:

### Telegram Proxy Crawler
- Scrapes verified public Telegram channels for MTProto and V2Ray configs.
- Performs concurrent TCP latency ping tests and allows 1-click launch directly into Telegram Desktop.

### Cloudflare Clean IP Scanner
- Tests hundreds of Cloudflare IPv4 ranges across major subnets (`162.159.192.x`, `188.114.96.x`, `104.16.x.x`).
- Measures latency (ms), jitter, and packet loss to assign the cleanest IP to your WARP or CDN profiles.

### TLS Fragment Studio
- Bypasses SNI-based DPI blocking by splitting the initial TLS `ClientHello` handshake into smaller fragments with microsecond delays.
- Preset configurations tailored for aggressive ISPs (*MCI Aggressive, Irancell Stealth, Fixed ADSL*).

### Free Configs Aggregator
- Continuously collects open-source proxy nodes, validates their ping, and filters them by protocol and geographic location.

---

## 5. Process-Based App Split Tunneling

Want your local banking apps or online games to bypass the VPN while browsers stay protected?
1. Open the **App Split Tunneling** modal.
2. Select or browse for your application executable (e.g., `chrome.exe`, `valorant.exe`, `spotify.exe`).
3. Choose the routing rule:
   - **Direct (Bypass VPN)**: App connects directly using your real local internet.
   - **Proxy (Force VPN)**: App is guaranteed to route through the encrypted tunnel.

---

## 6. Zero-Leak Kill Switch & DNS Shield

### Zero-Leak Kill Switch
When enabled, NexVPN configures OS-level firewall rules (*Windows Netsh, Linux iptables, macOS pfctl*). If the VPN disconnects unexpectedly, all non-VPN outbound internet traffic is blocked instantly, ensuring your real IP address is never leaked.

### Multi-Resolver DNS Leak Shield
- Automatically tests your DNS resolver against multiple trusted providers (Cloudflare 1.1.1.1, Quad9 9.9.9.9, Google 8.8.8.8).
- Detects DNS hijacking and enforces encrypted DNS-over-HTTPS (DoH) / DNS-over-TLS (DoT).

---

## 7. Smart Network Doctor & Emergency Repair

If your system loses internet connectivity after a forced reboot or abrupt disconnection:
1. Click **Network Doctor** on the sidebar.
2. The automatic diagnostic engine will:
   - 🧹 Flush OS DNS resolver cache.
   - 🔌 Reset Windows Winsock / Linux Netlink state.
   - 🚫 Remove dangling WinINet / GNOME system proxy settings.
   - 🔪 Terminate orphaned background core processes (`sing-box.exe`, `xray.exe`, `openvpn.exe`).
   - 🔄 Reinitialize Wintun virtual network adapters.

---

## 8. Mobile Remote & LAN Proxy Sharing

### Mobile Remote Companion
Control your desktop VPN without touching your keyboard:
1. Open the **Mobile Remote** modal in NexVPN.
2. Scan the displayed QR code with your smartphone camera (connected to the same Wi-Fi network).
3. Enter the 6-digit PIN to toggle connection, switch servers, and view live bandwidth on your phone.

### LAN Proxy Sharing
Share your VPN with Smart TVs, Apple TV, PlayStation, Xbox, or other laptops:
1. Enable **LAN Share** in Settings.
2. Configure the HTTP/SOCKS5 proxy on your other device using the local IP and port displayed (e.g., `192.168.1.50:10808`).

---

## 9. Backup, Cloud Sync & Nearby P2P

- **Encrypted Vault Backup**: Export your entire server list, custom fragmentation settings, and credentials into an AES-256 encrypted file.
- **GitHub Gist Sync**: Automatically synchronize your profiles across multiple computers via private GitHub Gists.
- **Nearby P2P Sync**: Discovers other NexVPN instances on your local network to share profiles securely over mDNS/UDP.

---

## 10. Gaming Mode & Desktop Floating Widget

- **Gaming Mode**: Optimizes MTU packet sizing and enables aggressive UDP packet routing for minimal ping and zero jitter in online competitive games.
- **Floating Mini Widget**: A lightweight, semi-transparent desktop HUD displaying live latency, download speed, and an instant Disconnect/Reconnect button.

---

## 11. Troubleshooting & Operator Optimization

### 📱 Tips for Mobile Operators (MCI / Irancell / Rightel)
- If VLESS Reality is blocked, open **Fragment Studio** and enable `10-30` byte fragment length with `15-30ms` sleep interval.
- If UDP is throttled, switch to **Aether MASQUE** or **Hysteria 2 with Port Hopping**.

### 🏢 Tips for Fixed Lines (ADSL / VDSL / FTTH / Shatel / Mokhaberat)
- Run the **Cloudflare Clean IP Scanner** to find subnets with `< 60ms` latency.
- Set routing mode to **Bypass Domestic / Iranian Sites** to ensure local services and online banking load at maximum native speed.
