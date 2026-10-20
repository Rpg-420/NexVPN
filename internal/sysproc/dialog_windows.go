//go:build windows

package sysproc

import (
	"syscall"
	"unsafe"
)

// A native dialog is the last diagnostic surface that still works when the
// application window does not.
//
// Wails renders into WebView2, and WebView2 can fail to present for reasons the
// Go side cannot see: a driver that composites nothing, or an endpoint-security
// product that injects an unsigned DLL and makes the renderer get killed. In both
// cases the process is healthy, the window is blank or never appears, and every
// error message the app wants to show lives inside that window. user32 does not.
var (
	user32Dialog   = syscall.NewLazyDLL("user32.dll")
	procMessageBox = user32Dialog.NewProc("MessageBoxW")
)

const (
	mbIconError = 0x00000010
	mbIconInfo  = 0x00000040
	mbOK        = 0x00000000
)

func showMessageBox(title, message string, flags uintptr) {
	// A modal Win32 dialog blocks until the user dismisses it, so callers that
	// still have work to do must run this on their own goroutine. Nothing here
	// can fail in a way worth reporting: if the desktop cannot show a dialog,
	// there is nowhere to report to.
	pTitle, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	pMessage, err := syscall.UTF16PtrFromString(message)
	if err != nil {
		return
	}
	procMessageBox.Call(0, uintptr(unsafe.Pointer(pMessage)), uintptr(unsafe.Pointer(pTitle)), flags)
}

// ShowFatalDialog reports a failure that stopped the application, outside the
// application window.
func ShowFatalDialog(title, message string) {
	showMessageBox(title, message, mbIconError|mbOK)
}

// ShowNoticeDialog reports an informational result the user has to acknowledge,
// such as a rendering change that only applies after a restart.
func ShowNoticeDialog(title, message string) {
	showMessageBox(title, message, mbIconInfo|mbOK)
}
