//go:build !windows

package sysproc

// ShowFatalDialog is a no-op off Windows: there is no Win32 message box, and the
// launch diary plus tools/NexVPN-Report are the diagnostics path there.
func ShowFatalDialog(title, message string) {}

// ShowNoticeDialog is a no-op off Windows.
func ShowNoticeDialog(title, message string) {}
