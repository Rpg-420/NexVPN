//go:build windows

package sysproc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"
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
	RegisterPID(cmd.Process.Pid)
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
// and isolate the child process in a new process group so console signals do not propagate.
func HideWindowAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000 | 0x00000200, // CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP
	}
}

// WaitForProcessExit reports whether the process is gone within the timeout.
//
// OpenProcess with SYNCHRONIZE is enough to wait on the handle, and it fails
// outright once the PID no longer exists — that is the common "already dead"
// case and must not be reported as a timeout, or every caller would log a
// spurious failure for a process that was never running.
func WaitForProcessExit(pid int, timeout time.Duration) bool {
	if pid <= 0 {
		return true
	}
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return true // no such process: nothing left to wait for
	}
	defer func() { _ = windows.CloseHandle(h) }()

	ms := uint32(timeout / time.Millisecond)
	if ms == 0 {
		ms = 1
	}
	status, err := windows.WaitForSingleObject(h, ms)
	if err != nil {
		return false
	}
	return status == windows.WAIT_OBJECT_0
}

// terminateProcess is the direct kill, used when taskkill cannot do the job.
func terminateProcess(pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(h) }()
	return windows.TerminateProcess(h, 1)
}

// KillProcessTree terminates a process tree on Windows without showing a console
// window, and returns an error when the process is still alive afterwards.
//
// The error is the point. The previous version discarded taskkill's result and
// returned nothing, so a kill that silently failed was indistinguishable from
// success. That is how sing-box came to log "Sing-box stopped" while the core
// kept running and holding 127.0.0.1:10808, and the next core then died on
// `bind: Only one usage of each socket address (protocol/network address/port)`
// — a port conflict that reads like an unrelated bug. Nothing in the pipeline
// could see it, and neither could the user.
func KillProcessTree(pid int) error {
	if pid <= 0 || pid == os.Getpid() {
		return nil
	}
	UnregisterPID(pid)

	// 1. taskkill /T is what actually reaches grandchildren — the loopback relay
	//    OpenVPN spawns, wrapper scripts, and anything else the core started.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	tk := exec.CommandContext(ctx, "taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	tk.SysProcAttr = HideWindowAttr()
	taskkillErr := tk.Run()
	cancel()

	if WaitForProcessExit(pid, 1500*time.Millisecond) {
		return nil
	}

	// 2. taskkill can fail for reasons unrelated to the process being alive: a
	//    restricted token, a child whose PATH has no system32, an elevated
	//    process we cannot open. TerminateProcess on a handle we open ourselves
	//    does not depend on taskkill at all, so it is the fallback.
	termErr := terminateProcess(pid)
	if termErr != nil && taskkillErr != nil {
		return fmt.Errorf("could not terminate process %d: taskkill: %v; TerminateProcess: %w",
			pid, taskkillErr, termErr)
	}

	if WaitForProcessExit(pid, 3*time.Second) {
		return nil
	}
	return fmt.Errorf("process %d is still running after taskkill and TerminateProcess", pid)
}

// IsAdmin checks whether the current process has elevated Administrator privileges on Windows
func IsAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer func() { _ = windows.FreeSid(sid) }()
	token := windows.Token(0)
	member, err := token.IsMember(sid)
	return err == nil && member
}
