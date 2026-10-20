# NexVPN Pro - Comprehensive Developer & Architecture Guide

Welcome to the technical engineering guide for **NexVPN Pro**. This document details the software architecture, internal packages, IPC event bus, protocol adapter lifecycle, and CI/CD pipelines.

---

## 🏛️ System Architecture Overview

NexVPN Pro follows a hybrid reactive desktop architecture:

```
┌─────────────────────────────────────────────────────────────┐
│             React 18 + TypeScript + TailwindCSS             │
│        (Pages, Modals, Real-time Gauges, i18n Engine)       │
└──────────────────────────────┬──────────────────────────────┘
                               │  Wails v2 Bidirectional IPC
┌──────────────────────────────┴──────────────────────────────┐
│                    Go Core Backend Layer                    │
│                                                             │
│  ┌─────────────────┐ ┌─────────────────┐ ┌───────────────┐  │
│  │ Engine Dispatch │ │ Discovery Suite │ │ System Guard  │  │
│  │ • Sing-box      │ │ • Telegram      │ │ • KillSwitch  │  │
│  │ • Xray-core     │ │ • Clean IP      │ │ • DNS Shield  │  │
│  │ • Aether MASQUE │ │ • Fragment TLS  │ │ • Net Doctor  │  │
│  │ • OpenVPN       │ │ • Free Configs  │ │ • SplitTunnel │  │
│  │ • Psiphon       │ │ • ISP Optimizer │ │ • Systray/HUD │  │
│  └─────────────────┘ └─────────────────┘ └───────────────┘  │
└──────────────────────────────┬──────────────────────────────┘
                               │  OS-Level System Calls
┌──────────────────────────────┴──────────────────────────────┐
│                  Operating System Drivers                   │
│  • Windows: Wintun C-ABI, Netsh Firewall, WinINet Registry  │
│  • Linux: TUN / Netlink, Polkit cap_net_admin, Iptables/NFT │
│  • macOS: PF (pfctl) Firewall, SystemConfiguration SCNetwork│
└─────────────────────────────────────────────────────────────┘
```

---

## 📂 Codebase Breakdown

### Root Directory
- `app.go`: Wails lifecycle entrypoint (`startup`, `shutdown`), core event broadcasters, main connection dispatch handlers.
- `app_discovery.go`: Asynchronous concurrent runner for Telegram crawlers, Cloudflare clean IP scans, and free config testers.
- `app_extra.go`: Mobile Remote HTTP server controller, speed tests, configuration vault export/import, window management.
- `version.go`: Single source of truth constant `AppVersion = "1.4.0"`.
- `main.go`: Application window properties, assets embedding, and runtime bootstrap.

### `internal/` Subsystems
- **`internal/core/`**:
  - `engine.go`: Active connection state machine (`idle`, `connecting`, `connected`, `error`). Manages log pipes, traffic monitors, and auto-failovers.
  - `dispatcher.go`: Multi-core dispatcher selecting the optimal protocol adapter based on configuration format.
  - `killswitch.go`: Multi-platform firewall manager preventing data leaks.
  - `dns_leak.go`: Active probe executing DNS leak tests through the live proxy against multiple global DNS resolvers.
  - `health_monitor.go`: Periodic health checker validating connection vitality with background ping probes.
  - `remote_server.go`: Embedded lightweight HTTP/WebSocket server for Mobile Remote QR pairing.
- **`internal/protocols/`**:
  - `interface.go`: Defines the unified `ProtocolAdapter` interface.
  - `singbox.go`: Sing-box process management, TUN configuration generation, Clash API memory traffic reader.
  - `xray.go`: Xray-core process adapter with gRPC / WebSocket / Reality config builders.
  - `aether.go`: MASQUE HTTP/3 UDP and WARP-in-WARP encapsulation adapter.
  - `openvpn.go`: Layer-3 OpenVPN process adapter with management socket hooks.
  - `psiphon.go`: Obfuscated SSH emergency bypass adapter.
- **`internal/discovery/`**:
  - `cloudflare.go`: Multi-threaded IP scanner testing latency and packet loss across WARP and CDN CIDR subnets.
  - `fragment_scanner.go`: TLS `ClientHello` fragment tester benchmarking length and sleep intervals.
  - `telegram.go`: Web scraper extracting proxy links and testing TCP latency concurrently.
  - `free_configs.go`: Subscription and public config aggregator.
  - `isp.go`: Autonomous ISP detector mapping IP addresses to telecommunication providers.
- **`internal/parser/`**:
  - `parser.go`: Universal URI and configuration parser supporting `vless://`, `vmess://`, `trojan://`, `ss://`, `hy2://`, `tuic://`, `warp://`, and `.ovpn`.
  - `doctor.go`: Network Doctor diagnostics and system state repair engine.
- **`internal/sysproxy/`**:
  - Platform-specific implementations (`sysproxy_windows.go`, `sysproxy_linux.go`, `sysproxy_darwin.go`) for atomic system proxy configuration.
- **`internal/tray/`**:
  - Windows taskbar notification area icon, right-click context menu, fast connect, and panic wipe hooks.

---

## 🔌 Protocol Adapter Interface

All protocol engines adhere to the `ProtocolAdapter` contract:

```go
type ProtocolAdapter interface {
    Name() config.Protocol
    Start(ctx context.Context, configContent string) error
    Stop() error
    GetStats() (bytesRx int64, bytesTx int64, err error)
    IsRunning() bool
    SetLogger(fn LoggerFunc)
    SetOnExit(fn OnExitFunc)
}
```

### Implementing a New Protocol:
1. Create `internal/protocols/myprotocol.go`.
2. Implement `ProtocolAdapter` methods.
3. Register the protocol in `internal/protocols/interface.go` under `NewAdapter()`.
4. Add URI parser rules in `internal/parser/parser.go`.
5. Update frontend views and `frontend/src/lib/i18n.ts`.

---

## 📡 Wails v2 Event Streaming Bus

NexVPN uses bidirectional asynchronous event streams between Go and React:

| Event Name | Direction | Payload | Description |
| :--- | :--- | :--- | :--- |
| `status:changed` | Go ➔ TS | `ConnectionStatus` | Broadcasts state (`connected`, `disconnected`, `error`) |
| `stats:update` | Go ➔ TS | `TrafficStats` | Real-time bytes RX/TX, download & upload speeds |
| `health:update` | Go ➔ TS | `HealthStatus` | Live ping latency, jitter, packet loss status |
| `log:entry` | Go ➔ TS | `LogEntry` | Streaming terminal logs for the Logs tab |
| `session:recorded`| Go ➔ TS | `ConnectionSession`| Connection history record on disconnect |
| `telegram:test_progress` | Go ➔ TS | `TelegramTestProgress` | Real-time progress during Telegram proxy testing |
| `cf:scan_progress` | Go ➔ TS | `CloudflareScanProgress`| Clean IP scanner progress updates |

---

## 🏗️ Build & CI/CD Pipeline

The GitHub Actions workflow in `.github/workflows/release.yml` automatically compiles binaries for:
- **Windows**: `x86_64` (amd64), `aarch64` (arm64), `i386` (32-bit) with Inno Setup Installer (`.exe`) and portable `.zip`.
- **Linux**: `x86_64` (amd64), `aarch64` (arm64) with `install_linux.sh` Polkit bundle.
- **macOS**: Universal Binary (Intel + Apple Silicon M-series) with `.dmg` installer.

### Manual Compilation
```bash
# Windows
wails build -platform windows/amd64 -clean

# Linux (requires WebKit2GTK 4.1 development libraries)
wails build -platform linux/amd64 -tags webkit2_41 -clean

# macOS
wails build -platform darwin/universal -clean
```
