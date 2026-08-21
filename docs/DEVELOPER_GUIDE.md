# NexVPN Pro - Developer Guide

NexVPN is built with **Wails**, **React**, and **Go**. The architecture is heavily modularized to support multiple routing engines and cross-platform native integrations.

## Directory Structure
- `frontend/`: The React + TypeScript frontend.
- `internal/`: Go backend logic.
  - `config/`: State, store, and struct definitions.
  - `core/`: The main Dispatcher (`engine.go`, `dispatcher.go`), auto-start, kill-switch, and binary managers.
  - `parser/`: Split tunneling parser, configuration generators, and the Network Doctor (`doctor.go`).
  - `protocols/`: The Protocol Adapters (`interface.go`, `singbox.go`, `aether.go`, `openvpn.go`).
  - `sysproc/`: OS-level process manipulation and job object wrappers (Windows).
  - `sysproxy/`: OS-level system proxy modifiers (macOS `networksetup`, Windows Registry, Linux `gsettings`/D-Bus).
- `install_linux.sh`: Sets up Polkit policies and `setcap` for passwordless execution on Linux.

## The Multi-Core Engine Architecture
NexVPN doesn't hardcode a single core. The `internal/protocols/interface.go` defines the `ProtocolAdapter` interface:
```go
type ProtocolAdapter interface {
    Name() config.Protocol
    Start(ctx context.Context, configContent string) error
    Stop() error
    GetStats() (int64, int64, error)
    IsRunning() bool
    SetLogger(fn LoggerFunc)
    SetOnExit(fn OnExitFunc)
}
```
When a user connects, `engine.go` resolves the protocol type and dispatches the connection to the appropriate adapter (`AetherAdapter`, `SingBoxAdapter`, or `OpenVPNAdapter`).

## Adding a New Protocol Core
1. **Create the Adapter**: Create `internal/protocols/mycore.go` and implement the `ProtocolAdapter` interface.
2. **Register**: Add it to `internal/protocols/interface.go` and `internal/core/dispatcher.go`.
3. **Frontend**: Update `frontend/src/types/index.ts` to include your new protocol, and create a React UI page to configure it.
4. **Binary Management**: Add the download URL and checksum logic to `internal/core/binary_downloader.go`.

## Linux Polkit Integration
On Linux, TUN routing and iptables kill-switches typically require root privileges. 
Instead of prompting the user via `sudo` or triggering GNOME password prompts, NexVPN uses:
1. `setcap cap_net_admin,cap_net_bind_service=+ep` to grant capability-based access.
2. A dynamically generated Polkit Policy (`/usr/share/polkit-1/actions/dev.nexvpn.policy`) mapping the exact absolute paths of the binaries to `<allow_active>yes</allow_active>`.
3. Disabling `DBUS_SYSTEM_BUS_ADDRESS` inside the Go execution environment so cores (like sing-box) fallback to `netlink` instead of triggering NetworkManager D-Bus prompts.

## Building the App
```bash
# Development (Hot reload)
wails dev

# Production Build
wails build -clean -platform windows/amd64,linux/amd64,darwin/arm64
```
