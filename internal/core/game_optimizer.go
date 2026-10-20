package core

import (
	"sync"
)

// GamingOptimizationStatus represents active TCP/IP gaming tweaks
type GamingOptimizationStatus struct {
	Active                 bool     `json:"active"`
	TcpAckFrequencyApplied bool     `json:"tcp_ack_frequency_applied"`
	TCPNoDelayApplied      bool     `json:"tcp_nodelay_applied"`
	NetworkThrottlingTuned bool     `json:"network_throttling_tuned"`
	SystemResponsiveness   bool     `json:"system_responsiveness"`
	OptimizedInterfaces    []string `json:"optimized_interfaces"`
	Supported              bool     `json:"supported"` // false on platforms without registry tweaks
	Message                string   `json:"message"`
}

type GamingOptimizer struct {
	mu     sync.Mutex
	active bool
}

var globalGamingOptimizer = &GamingOptimizer{}

// GetGamingOptimizer returns the global gaming optimizer singleton
func GetGamingOptimizer() *GamingOptimizer {
	return globalGamingOptimizer
}

// GetStatus reports the *actual* current state of the tweaks.
//
// It does not rely on the in-memory `active` flag alone: it re-reads the real
// registry state so the value stays correct after an application restart or
// after the user changes the settings outside NexVPN. On platforms without
// registry support it reports Supported=false rather than pretending to be off.
func (g *GamingOptimizer) GetStatus() GamingOptimizationStatus {
	g.mu.Lock()
	cachedActive := g.active
	g.mu.Unlock()

	status := readGamingOptimizationState()
	status.Active = status.NetworkThrottlingTuned || status.TcpAckFrequencyApplied || cachedActive

	// Keep the cached flag in sync with reality.
	g.mu.Lock()
	g.active = status.Active
	g.mu.Unlock()

	if !status.Supported {
		status.Active = false
		return status
	}

	if status.Active {
		status.Message = "Ultra-low latency gaming tweaks are active."
	} else {
		status.Message = "Gaming optimizations are not applied."
	}
	return status
}
