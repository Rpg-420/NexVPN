//go:build windows

package sysproc

import (
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	globalJobHandle windows.Handle
	jobOnce         sync.Once
)

func initJobObject() {
	hJob, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}

	_, err = windows.SetInformationJobObject(
		hJob,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		_ = windows.CloseHandle(hJob)
		return
	}

	globalJobHandle = hJob
}

// AssignCmdToJobObject ensures that child process is bound to the parent's Job Object
// so that when parent is killed or terminates in any way, Windows terminates the child process.
func AssignCmdToJobObject(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	jobOnce.Do(initJobObject)
	if globalJobHandle != 0 {
		hProc, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
		if err == nil {
			_ = windows.AssignProcessToJobObject(globalJobHandle, hProc)
			_ = windows.CloseHandle(hProc)
		}
	}
}

// HideWindowAttr returns Windows-specific process attributes to hide console windows
func HideWindowAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

// KillProcessTree terminates a process tree on Windows without showing console
func KillProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
	cmd.SysProcAttr = HideWindowAttr()
	_ = cmd.Run()
}

