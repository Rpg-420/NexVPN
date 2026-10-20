//go:build !windows

package core

// readGamingOptimizationState reports that registry-based TCP/IP tweaks are not
// available on this platform, rather than reporting a misleading "inactive".
func readGamingOptimizationState() GamingOptimizationStatus {
	return GamingOptimizationStatus{
		Supported:           false,
		OptimizedInterfaces: []string{},
		Message:             "Gaming TCP/IP registry tweaks are only available on Windows.",
	}
}
