//go:build !windows

package tray

// SetDisconnectHandler registers a callback invoked when Disconnect is clicked from tray menu
func SetDisconnectHandler(fn func()) {}

// ApplyMainWindowIcon is a no-op on non-Windows platforms
func ApplyMainWindowIcon() {}

// InitSystemTray is a no-op on non-Windows platforms
func InitSystemTray(tooltip string) {}

// ShowMainWindow is a no-op on non-Windows platforms
func ShowMainWindow() {}

// RemoveSystemTray removes the tray icon
func RemoveSystemTray() {}

// UpdateTooltip updates the tooltip text
func UpdateTooltip(tooltip string) {}
