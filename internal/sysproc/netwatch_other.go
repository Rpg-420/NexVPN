//go:build !windows

package sysproc

import "context"

// registerInterfaceNotifications is a no-op off Windows.
//
// The platform-neutral fingerprint poller in netwatch.go is the only source
// there, which is acceptable: the TUN/Wintun path this feature mainly protects
// is Windows-specific, and the poller still covers the general case.
func registerInterfaceNotifications(_ context.Context, _ func()) {}
