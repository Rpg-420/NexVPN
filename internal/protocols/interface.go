package protocols

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"nexvpn/internal/config"
)

// OnExitFunc is called when the underlying process exits unexpectedly.
// err is nil if the exit was requested (via Stop/cancel), non-nil on unexpected crash.
type OnExitFunc func(err error)

type ProtocolAdapter interface {
	Name() config.Protocol
	Start(ctx context.Context, configContent string) error
	Stop() error
	GetStats() (bytesRx int64, bytesTx int64, err error)
	IsRunning() bool
	SetLogger(fn LoggerFunc)
	// SetOnExit registers a callback invoked when the core process exits unexpectedly.
	SetOnExit(fn OnExitFunc)
}

func NewAdapter(protocol config.Protocol) (ProtocolAdapter, error) {
	switch protocol {
	case config.ProtocolXray:
		return NewXrayAdapter(), nil
	case config.ProtocolSingBox, config.ProtocolHysteria2, config.ProtocolWARP, config.ProtocolWireGuard:
		// Sing-box has full built-in support for Sing-box, Hysteria2, Shadowsocks, TUIC, WireGuard and WARP with TUN and Clash stats
		return NewSingBoxAdapter(), nil
	case config.ProtocolOpenVPN:
		return NewOpenVPNAdapter(), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

// ensureNexVPNTempDir creates and returns %APPDATA%\NexVPN\tmp, falling back to os.TempDir().
func ensureNexVPNTempDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	dir := filepath.Join(appData, "NexVPN", "tmp")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return ""
	}
	return dir
}
