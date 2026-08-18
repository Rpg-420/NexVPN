//go:build !windows

package sysproc

import (
	"os/exec"
	"syscall"
)

// HideWindowAttr returns POSIX process attributes for non-Windows platforms
func HideWindowAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// AssignCmdToJobObject is a no-op on non-Windows platforms
func AssignCmdToJobObject(cmd *exec.Cmd) {
}

