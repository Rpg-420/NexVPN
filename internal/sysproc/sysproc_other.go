//go:build !windows

package sysproc

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// HideWindowAttr returns POSIX process attributes for non-Windows platforms
func HideWindowAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// AssignCmdToJobObject registers the PID on non-Windows platforms
func AssignCmdToJobObject(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		RegisterPID(cmd.Process.Pid)
	}
}

// WaitForProcessExit reports whether the process is gone within the timeout.
//
// Signal 0 performs the permission and existence check without delivering
// anything: nil means alive, ESRCH means gone, and EPERM means alive but owned
// by someone else. Only ESRCH counts as gone.
func WaitForProcessExit(pid int, timeout time.Duration) bool {
	if pid <= 0 {
		return true
	}
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Kill(pid, 0)
		if err == syscall.ESRCH {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(40 * time.Millisecond)
	}
}

// KillProcessTree terminates process group on non-Windows platforms and returns
// an error when the process is still alive afterwards. The error is what lets a
// core adapter report a truthful "stopped" instead of logging success over a
// process that is still holding its listener.
func KillProcessTree(pid int) error {
	if pid <= 0 || pid == os.Getpid() {
		return nil
	}
	UnregisterPID(pid)
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	if proc, err := os.FindProcess(pid); err == nil {
		_ = proc.Kill()
	}
	if WaitForProcessExit(pid, 3*time.Second) {
		return nil
	}
	return fmt.Errorf("process %d is still running after SIGKILL", pid)
}

// IsAdmin checks whether the current process is running as root
func IsAdmin() bool {
	return os.Geteuid() == 0
}
