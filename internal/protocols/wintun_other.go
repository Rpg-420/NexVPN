//go:build !windows

package protocols

// EnsureWintunAdapter is a no-op on non-Windows operating systems.
func EnsureWintunAdapter(binDir string, adapterName string) error {
	return nil
}

// WintunDLLPresent is always false off Windows: there is no wintun.dll, and no
// TUN readiness gate is needed either.
func WintunDLLPresent(binDir string) bool {
	return false
}

// WintunAdapterReady always reports ready off Windows: there is no Wintun
// adapter to wait for, so the TUN readiness gate in Start() must not block.
func WintunAdapterReady(binDir, adapterName string) bool {
	return true
}
