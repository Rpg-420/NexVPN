//go:build !windows

package sysproc

// DiscoverOptimalMTU returns the platform default MTU on non-Windows systems.
//
// This is deliberately not presented as a measurement: unlike the Windows
// implementation it does not probe the path with the Don't Fragment bit, so
// callers must not advertise the result as "auto-detected".
func DiscoverOptimalMTU(targetHost string) int {
	return 1380
}

// ApplyWintunMTU is a no-op on non-Windows platforms because there is no Wintun
// adapter to configure. It returns a descriptive error instead of silently
// succeeding so callers can report the real outcome.
func ApplyWintunMTU(adapterName string, mtu int) error {
	return ErrMTUUnsupported
}
