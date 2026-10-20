//go:build !windows

package sysproc

import "fmt"

// ProcessHoldingPort returns the PID and executable base name of the process
// listening on the given local TCP port.
//
// On non-Windows platforms this deliberately reports "unknown" rather than
// shelling out to `lsof`/`ss`: the port-ownership recovery in internal/core
// exists because Windows keeps a dead core's listener alive for a beat, and on
// macOS/Linux a killed process releases the socket immediately. Reporting
// unknown keeps the caller from killing an unrelated process based on a
// best-effort parse.
func ProcessHoldingPort(port int) (int, string, bool) {
	return 0, "", false
}

// DescribeProcessHoldingPort is a small formatting helper for log lines.
func DescribeProcessHoldingPort(port int) string {
	return fmt.Sprintf("nothing identifiable is listening on 127.0.0.1:%d", port)
}

// ProcessImageName resolves a PID to its executable base name.
//
// Unsupported off Windows: an empty string means "unknown", and callers that use
// this to authorise a kill must treat unknown as a refusal, not a match.
func ProcessImageName(pid int) string {
	return ""
}
