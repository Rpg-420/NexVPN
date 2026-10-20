//go:build !windows

package protocols

// WindowsVisibleAdapterNames is Windows-only; elsewhere there is no Wintun
// driver to leave a ghost behind, so callers get nil ("unknown") and the default
// adapter name is always kept.
func WindowsVisibleAdapterNames() map[string]bool {
	return nil
}

// WintunGhostScan is Windows-only in every meaningful respect.
type WintunGhostScan struct {
	Visible map[string]bool
	Bound   map[string]bool
}

// ScanWintunGhosts reports unknown on both halves off Windows, so nothing is ever
// treated as ghosted.
func ScanWintunGhosts() WintunGhostScan {
	return WintunGhostScan{}
}

// Ghosted is always false off Windows: there is no Wintun driver to leave a
// ghost adapter behind.
func (s WintunGhostScan) Ghosted(name string) bool {
	return false
}

// NexVPNWintunAdapterNames is empty off Windows: no Wintun adapter can exist, so
// there is nothing to look for and nothing to remove.
func NexVPNWintunAdapterNames() []string {
	return nil
}

// NexVPNWintunAdapterPresent is always false off Windows.
func NexVPNWintunAdapterPresent(scan WintunGhostScan) bool {
	return false
}

// NexVPNWintunAdapterLive is always false off Windows: there is no Wintun pool
// for an adapter to hold.
func NexVPNWintunAdapterLive(scan WintunGhostScan) bool {
	return false
}

// ClearStaleNexVPNWintunAdapters is a no-op off Windows.
func ClearStaleNexVPNWintunAdapters() {}
