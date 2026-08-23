# Contributing to NexVPN Pro

Thank you for your interest in contributing to **NexVPN Pro**! NexVPN is an open-source, next-generation multi-core anti-censorship client engineered to provide resilient, secure, and unrestricted internet access.

We welcome contributions of all kinds: bug reports, feature requests, documentation improvements, code contributions, and testing across different operating systems and ISPs.

---

## 📋 Table of Contents
- [Code of Conduct](#code-of-conduct)
- [Architecture Overview](#architecture-overview)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Coding Guidelines](#coding-guidelines)
  - [Go Backend](#go-backend)
  - [TypeScript / React Frontend](#typescript--react-frontend)
- [Adding a New Protocol Engine](#adding-a-new-protocol-engine)
- [Pull Request Process](#pull-request-process)
- [Reporting Security Issues](#reporting-security-issues)

---

## 🤝 Code of Conduct
This project adheres to the Contributor Covenant [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code. Please report unacceptable behavior to project maintainers.

---

## 🏛️ Architecture Overview
NexVPN Pro uses a modern hybrid desktop architecture:
1. **Backend (Go 1.22+)**: Handles system networking, multi-core process lifecycle (Sing-box, Xray, Aether, OpenVPN, Psiphon), TUN interface management, system proxy registry configuration, firewall killswitch manipulation, and background discovery scanners.
2. **Frontend (React 18 + TypeScript + TailwindCSS + Vite)**: Renders a sleek, responsive, glassmorphic UI with real-time bidirectional event streaming via Wails v2 IPC.
3. **Wails v2 Framework**: Bridges Go and TypeScript with strongly typed bindings, asynchronous RPC, and event emission.

---

## 🛠️ Development Setup

### Prerequisites
- **Go**: Version 1.22 or higher ([Download](https://go.dev/dl/))
- **Node.js**: Version 20 LTS or higher ([Download](https://nodejs.org/))
- **Wails CLI v2**:
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```
- **Platform Specifics**:
  - **Windows**: WebView2 runtime (pre-installed on Windows 10/11).
  - **Linux**: `sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config`
  - **macOS**: Xcode Command Line Tools (`xcode-select --install`)

### Running in Development Mode
```bash
# 1. Clone the repository
git clone https://github.com/Rpg-420/NexVPN.git
cd NexVPN

# 2. Run Wails live development server with hot-reload
wails dev
```

### Building for Production
```bash
# Clean production build
wails build -clean

# Output will be generated in:
# build/bin/NexVPN.exe (Windows)
# build/bin/NexVPN (Linux / macOS)
```

---

## 📂 Project Structure

```
NexVPN/
├── .github/                  # CI/CD Workflows, issue/PR templates, security policies
├── app.go                    # Main Wails Go application entrypoint & API bindings
├── app_discovery.go          # Discovery Hub, Telegram scraper, & Free config APIs
├── app_extra.go              # Utility APIs, Mobile Remote, backup/restore, speed test
├── main.go                   # Go runtime initialization & window configuration
├── version.go                # Single source of truth for versioning (e.g. 1.3.5)
├── internal/
│   ├── config/               # Profile storage, encrypted vault, JSON settings
│   ├── core/                 # Engine dispatcher, Killswitch, DNS leak, Network Doctor
│   ├── discovery/            # Cloudflare scanner, Fragment tester, Telegram crawler
│   ├── parser/               # VLESS, VMess, Trojan, SS, Hysteria2, TUIC, Warp parsers
│   ├── protocols/            # Protocol adapters (Sing-box, Xray, Aether, OpenVPN, Psiphon)
│   ├── sysproc/              # Hidden process execution flags & background daemons
│   ├── sysproxy/             # Windows WinINet, Linux GNOME/KDE, macOS sysproxy handlers
│   └── tray/                 # Windows taskbar system tray integration & icon handlers
├── frontend/
│   ├── src/
│   │   ├── components/       # Modals (CleanIP, FragmentStudio, MobileRemote, NetworkDoctor...)
│   │   ├── pages/            # Page views (Dashboard, DiscoveryHub, Settings, Profiles...)
│   │   ├── lib/              # i18n translations (EN/FA), API client, utility functions
│   │   ├── hooks/            # React hooks for traffic, status, and theme
│   │   └── types/            # TypeScript interfaces matching Go backend models
└── docs/                     # Comprehensive documentation & Web Landing Showcase
```

---

## 📐 Coding Guidelines

### Go Backend
- Follow standard Go conventions (`gofmt`, `go vet`, `golint`).
- Keep concurrency thread-safe using `sync.Mutex` or `sync.RWMutex` where state is shared.
- Never spawn GUI console windows on Windows: always use `sysproc.HideWindowAttr()` for `exec.Cmd`.
- Clean up resources gracefully: ensure processes, temporary config files, and firewall rules are deleted on shutdown or crash recovery.
- Format errors with descriptive context.

### TypeScript / React Frontend
- Use functional components with React Hooks.
- Ensure all new UI text strings are added to `frontend/src/lib/i18n.ts` with both `en` and `fa` translations.
- Maintain responsive RTL (Right-to-Left) and LTR (Left-to-Right) support.
- Stick to the unified design language: Dark theme, glassmorphism (`glass-card`), vibrant cyan/purple/emerald accents, Lucide icons.

---

## 🔌 Adding a New Protocol Engine
To integrate a new proxy engine or protocol:
1. Implement the `protocols.ProtocolAdapter` interface in `internal/protocols/<protocol>.go`:
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
2. Register the protocol in `internal/protocols/interface.go` within `NewAdapter()`.
3. Add URI and config parsing in `internal/parser/parser.go`.
4. Add frontend configuration fields and icons in `frontend/src/pages/` and `frontend/src/lib/i18n.ts`.

---

## 🚀 Pull Request Process
1. Fork the repository and create a descriptive feature branch (e.g., `feature/wireguard-multihop` or `fix/dns-leak-windows`).
2. Implement your changes following the coding guidelines.
3. Test your build locally on at least one target platform:
   ```bash
   wails build -clean
   ```
4. Verify that existing tests pass:
   ```bash
   go test ./...
   ```
5. Commit your changes with clear, concise commit messages following [Conventional Commits](https://www.conventionalcommits.org/).
6. Submit a Pull Request targeting the `main` branch, filling out the PR template completely.

---

## 🔒 Reporting Security Issues
If you discover a security vulnerability, please do **NOT** open a public GitHub issue. Refer to [SECURITY.md](SECURITY.md) for our coordinated disclosure process.
