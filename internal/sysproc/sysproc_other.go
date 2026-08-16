//go:build !windows

package sysproc

import "syscall"

// HideWindowAttr returns POSIX process attributes for non-Windows platforms
func HideWindowAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}
