//go:build !windows

package core

// ApplyGamingOptimizations is a no-op on non-Windows platforms
func (g *GamingOptimizer) ApplyGamingOptimizations() (*GamingOptimizationStatus, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	return &GamingOptimizationStatus{
		Active:              false,
		Supported:           false,
		OptimizedInterfaces: []string{},
		Message:             "Gaming TCP/IP tweaks are only supported on Windows.",
	}, nil
}

// RevertGamingOptimizations is a no-op on non-Windows platforms
func (g *GamingOptimizer) RevertGamingOptimizations() (*GamingOptimizationStatus, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	return &GamingOptimizationStatus{
		Active:    false,
		Supported: false,
		Message:   "Restored default settings (non-Windows platform).",
	}, nil
}
