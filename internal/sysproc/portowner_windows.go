//go:build windows

package sysproc

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
)

// ProcessHoldingPort returns the PID and executable base name of the process
// listening on the given local TCP port.
//
// ok is false when nothing is listening, when the port is owned by a socket we
// cannot attribute (the table changes between the two lookups), or on any error.
// Callers must treat "not ok" as "leave it alone", never as "safe to proceed".
//
// The PID comes from `netstat -ano -p tcp`, which is the only dependency-free
// way to map a Windows port to an owner. This runs on the connection-recovery
// path only — never on the happy path — so the subprocess cost is irrelevant.
func ProcessHoldingPort(port int) (int, string, bool) {
	if port <= 0 {
		return 0, "", false
	}

	cmd := exec.Command("netstat", "-ano", "-p", "tcp")
	cmd.SysProcAttr = HideWindowAttr()
	out, err := cmd.Output()
	if err != nil {
		return 0, "", false
	}

	suffix := ":" + strconv.Itoa(port)
	pid := 0
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		// TCP  <local>  <remote>  LISTENING  <pid>
		if len(fields) < 5 || !strings.EqualFold(fields[3], "LISTENING") {
			continue
		}
		if !strings.HasSuffix(fields[1], suffix) {
			continue
		}
		// `:1080` must not match `:10808`, and `:10808` must not match
		// `:110808`. HasSuffix alone is not enough for the first case.
		if !isPortField(fields[1], port) {
			continue
		}
		n, convErr := strconv.Atoi(fields[4])
		if convErr != nil || n <= 0 {
			continue
		}
		pid = n
		break
	}
	if pid == 0 {
		return 0, "", false
	}

	return pid, processImageName(pid), true
}

// isPortField reports whether a netstat local-address field ends with exactly
// the given port. Handles both `127.0.0.1:10808` and `[::]:10808`.
func isPortField(field string, port int) bool {
	idx := strings.LastIndex(field, ":")
	if idx < 0 {
		return false
	}
	n, err := strconv.Atoi(field[idx+1:])
	return err == nil && n == port
}

// ProcessImageName resolves a PID to its executable base name, lower-cased.
//
// Returns "" when the process is gone or unreadable. Callers that use this to
// decide whether a process may be terminated MUST treat "" as "unknown", never
// as a match: this process runs elevated, so a wrong answer here kills an
// unrelated application.
func ProcessImageName(pid int) string {
	if pid <= 0 {
		return ""
	}
	return processImageName(pid)
}

// processImageName resolves a PID to its executable base name, lower-cased.
// Returns "" when the process is gone or unreadable — the caller only needs the
// name to decide whether killing it is safe, so an unknown name must not be
// treated as a match.
func processImageName(pid int) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer func() { _ = windows.CloseHandle(h) }()

	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return ""
	}
	full := windows.UTF16ToString(buf[:size])
	if i := strings.LastIndexAny(full, `\/`); i >= 0 {
		full = full[i+1:]
	}
	return strings.ToLower(full)
}

// DescribeProcessHoldingPort is a small formatting helper for log lines.
func DescribeProcessHoldingPort(port int) string {
	pid, name, ok := ProcessHoldingPort(port)
	if !ok {
		return fmt.Sprintf("nothing identifiable is listening on 127.0.0.1:%d", port)
	}
	if name == "" {
		return fmt.Sprintf("127.0.0.1:%d is held by PID %d", port, pid)
	}
	return fmt.Sprintf("127.0.0.1:%d is held by %s (PID %d)", port, name, pid)
}
