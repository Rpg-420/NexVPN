//go:build !windows

package sysproc

import "context"

// registerPowerNotifications is a no-op off Windows.
//
// Suspend/resume and mains-power transitions are read through powrprof and
// kernel32, which have no portable equivalent. The engine's health monitor
// covers the resulting gap, so the only cost is a slower reaction on a laptop
// waking from sleep.
func registerPowerNotifications(_ context.Context, _ func(string)) {}
