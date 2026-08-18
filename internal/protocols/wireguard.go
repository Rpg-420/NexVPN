package protocols

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"nexvpn/internal/config"
	"nexvpn/internal/parser"
)

type WireGuardAdapter struct {
	mu            sync.RWMutex
	singboxEngine *SingBoxAdapter
	logger        LoggerFunc
}

func NewWireGuardAdapter() *WireGuardAdapter {
	return &WireGuardAdapter{
		singboxEngine: NewSingBoxAdapter(),
	}
}

func (a *WireGuardAdapter) Name() config.Protocol {
	return config.ProtocolWireGuard
}

func (a *WireGuardAdapter) SetLogger(fn LoggerFunc) {
	a.mu.Lock()
	a.logger = fn
	engine := a.singboxEngine
	a.mu.Unlock()

	if engine != nil {
		engine.SetLogger(fn)
	}
}

func (a *WireGuardAdapter) SetOnExit(fn OnExitFunc) {
	a.mu.RLock()
	engine := a.singboxEngine
	a.mu.RUnlock()

	if engine != nil {
		engine.SetOnExit(fn)
	}
}

func (a *WireGuardAdapter) Start(ctx context.Context, configContent string) error {
	if configContent == "" {
		return fmt.Errorf("wireguard configuration content is empty")
	}

	// If configContent is INI .conf format, convert to Sing-Box JSON
	finalJSON := configContent
	if strings.Contains(configContent, "[Interface]") {
		outbound, host, err := parser.ParseWireGuardConf(configContent)
		if err != nil {
			return fmt.Errorf("failed to parse wireguard .conf: %w", err)
		}
		sbJSON, err := parser.BuildSingBoxConfig(outbound, "tun", "bypass_iran", host)
		if err != nil {
			return fmt.Errorf("failed to build wireguard singbox config: %w", err)
		}
		finalJSON = sbJSON
	}

	a.mu.RLock()
	engine := a.singboxEngine
	a.mu.RUnlock()

	if engine == nil {
		return fmt.Errorf("singbox engine not initialized")
	}

	return engine.Start(ctx, finalJSON)
}

func (a *WireGuardAdapter) Stop() error {
	a.mu.RLock()
	engine := a.singboxEngine
	a.mu.RUnlock()

	if engine != nil {
		return engine.Stop()
	}
	return nil
}

func (a *WireGuardAdapter) GetStats() (int64, int64, error) {
	a.mu.RLock()
	engine := a.singboxEngine
	a.mu.RUnlock()

	if engine != nil {
		return engine.GetStats()
	}
	return 0, 0, nil
}

func (a *WireGuardAdapter) IsRunning() bool {
	a.mu.RLock()
	engine := a.singboxEngine
	a.mu.RUnlock()

	if engine != nil {
		return engine.IsRunning()
	}
	return false
}
