//go:build windows

package sysproc

import "syscall"

// HideWindowAttr returns Windows-specific process attributes to hide console windows
func HideWindowAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
