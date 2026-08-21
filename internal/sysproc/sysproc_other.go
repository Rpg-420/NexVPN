//go:build !windows

package sysproc

import (
	"os"
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

// KillProcessTree terminates process group on non-Windows platforms
func KillProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	if proc, err := os.FindProcess(pid); err == nil {
		_ = proc.Kill()
	}
}

