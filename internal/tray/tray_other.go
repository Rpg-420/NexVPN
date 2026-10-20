//go:build !windows

package tray

// SetToggleConnectHandler registers a callback for toggling connection
func SetToggleConnectHandler(fn func()) {}

// SetFastConnectHandler registers a callback for connecting to the fastest profile
func SetFastConnectHandler(fn func()) {}

// SetPanicWipeHandler registers a callback for panic disconnecting
func SetPanicWipeHandler(fn func()) {}

// SetDisconnectHandler backward-compatibility helper
func SetDisconnectHandler(fn func()) {}

// SetShowHandler registers a callback invoked when Show or left click is triggered
func SetShowHandler(fn func()) {}

// SetExitHandler registers a callback invoked when Exit is clicked from tray menu
func SetExitHandler(fn func()) {}

// SetServerListProvider is a no-op off Windows: there is no tray menu to fill.
func SetServerListProvider(fn func() []ServerItem) {}

// SetServerSelectHandler is a no-op off Windows.
func SetServerSelectHandler(fn func(string)) {}

// SetDiagnosticsHandlers is a no-op off Windows: there is no tray menu to add
// the diagnostics rows to. The startup report itself is still written, and
// tools/NexVPN-Report.cmd still reads it.
func SetDiagnosticsHandlers(onOpenReport, onToggleGPU func(), gpuDisabled func() bool) {}

// UpdateTrayState syncs the current connection state and updates tooltip
func UpdateTrayState(connected bool, profile string, latency int) {}

// SetTrayLanguage changes the active display language in tray menu
func SetTrayLanguage(lang string) {}

// ApplyMainWindowIcon is a no-op on non-Windows
func ApplyMainWindowIcon() {}

// InitSystemTray is a no-op on non-Windows platforms
func InitSystemTray(tooltip string) {}

// ShowMainWindow is a no-op on non-Windows
func ShowMainWindow() {}

// RemoveSystemTray removes the tray icon
func RemoveSystemTray() {}

// UpdateTooltip updates the tooltip text on the tray icon
func UpdateTooltip(tooltip string) {}
