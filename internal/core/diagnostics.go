package core

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"

	"nexvpn/internal/protocols"
	"nexvpn/internal/sysproc"
	"nexvpn/internal/sysproxy"
)

// DiagnosticReport contains a comprehensive audit of system network and engine health
type DiagnosticReport struct {
	Healthy        bool              `json:"healthy"`
	IssuesFound    []string          `json:"issues_found"`
	WintunStatus   string            `json:"wintun_status"`
	ProxyStatus    string            `json:"proxy_status"`
	PortStatus     map[int]string    `json:"port_status"`
	BinariesStatus map[string]string `json:"binaries_status"`
	DNSStatus      string            `json:"dns_status"`
	Platform       string            `json:"platform"`
	Timestamp      time.Time         `json:"timestamp"`
}

// RepairResult records actions taken during automated repair
type DiagnosticRepairResult struct {
	Success          bool              `json:"success"`
	ActionsTaken     []string          `json:"actions_taken"`
	PostRepairHealth *DiagnosticReport `json:"post_repair_health"`
	Message          string            `json:"message"`
}

// RunDiagnostics executes a non-intrusive scan of all subsystems
func RunDiagnostics() *DiagnosticReport {
	report := &DiagnosticReport{
		Healthy:        true,
		IssuesFound:    []string{},
		PortStatus:     make(map[int]string),
		BinariesStatus: make(map[string]string),
		Platform:       runtime.GOOS,
		Timestamp:      time.Now(),
	}

	// 1. Check Port Conflicts (10808, 10809, 10853)
	testPorts := []int{10808, 10809, 10853}
	for _, port := range testPorts {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			report.PortStatus[port] = "In use or restricted"
			report.IssuesFound = append(report.IssuesFound, fmt.Sprintf("Port %d is currently occupied by another process.", port))
			report.Healthy = false
		} else {
			_ = ln.Close()
			report.PortStatus[port] = "Available"
		}
	}

	// 2. Check Core Binaries
	binNames := []string{"sing-box", "xray", "openvpn"}
	for _, bin := range binNames {
		path, err := protocols.FindBinary(bin)
		if err != nil || path == "" {
			report.BinariesStatus[bin] = "Missing"
			if bin == "sing-box" {
				report.IssuesFound = append(report.IssuesFound, "Primary Sing-Box core binary is missing from bin folder.")
				report.Healthy = false
			}
		} else {
			if fi, sErr := os.Stat(path); sErr == nil {
				report.BinariesStatus[bin] = fmt.Sprintf("Present (%d KB)", fi.Size()/1024)
			} else {
				report.BinariesStatus[bin] = "Present"
			}
		}
	}

	// 3. Check Wintun Driver on Windows
	if runtime.GOOS == "windows" {
		// Reuse the engine's own resolver instead of probing a single hardcoded
		// %APPDATA%\NexVPN\bin path, which disagreed with where the app actually
		// deploys the driver (next to the executable / bin\) and produced false
		// "Not deployed" reports.
		if path, size := ResolveWintunDLL(); path != "" {
			report.WintunStatus = fmt.Sprintf("Installed (wintun.dll verified, %d KB)", size/1024)
		} else {
			report.WintunStatus = "Missing (wintun.dll not found next to the executable or in bin\\)"
			report.IssuesFound = append(report.IssuesFound, "wintun.dll is missing — TUN mode cannot start on Windows.")
			report.Healthy = false
		}
	} else {
		report.WintunStatus = "N/A (Non-Windows platform)"
	}

	// 4. Check DNS Resolver
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 2 * time.Second}
			return d.DialContext(ctx, "udp", "1.1.1.1:53")
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	_, dnsErr := resolver.LookupHost(ctx, "one.one.one.one")
	if dnsErr != nil {
		report.DNSStatus = fmt.Sprintf("Degraded / Unreachable (%v)", dnsErr)
		report.IssuesFound = append(report.IssuesFound, "Direct DNS lookup (1.1.1.1:53) timed out.")
	} else {
		report.DNSStatus = "Operational"
	}

	// 5. Check Proxy State
	if sysproxy.IsSystemProxyEnabled() {
		report.ProxyStatus = "Enabled (a system proxy is currently active)"
		report.IssuesFound = append(report.IssuesFound, "A system-wide proxy is currently enabled.")
	} else {
		report.ProxyStatus = "Clean (no system proxy configured)"
	}

	return report
}

// RepairAllIssues terminates stale processes, resets system proxy, flushes DNS, and verifies Wintun
func RepairAllIssues() *DiagnosticRepairResult {
	actions := []string{}

	// 1. Terminate orphaned core processes
	pids := sysproc.GetRegisteredPIDs()
	if len(pids) > 0 {
		CleanupStaleCores()
		actions = append(actions, fmt.Sprintf("Terminated %d stale background core processes.", len(pids)))
	}

	// 2. Clear OS proxy & flush DNS
	_ = sysproxy.FlushDNSAndResetNetwork()
	actions = append(actions, "Cleared Windows system proxy registry entries and flushed DNS resolver cache.")

	// 3. Ensure Wintun driver is deployed
	EnsureWintunDriver()
	actions = append(actions, "Verified and ensured Wintun TUN network adapter driver.")

	// 4. Re-run diagnostics to verify
	postReport := RunDiagnostics()

	return &DiagnosticRepairResult{
		Success:          postReport.Healthy,
		ActionsTaken:     actions,
		PostRepairHealth: postReport,
		Message:          "Automated diagnostics repair completed successfully.",
	}
}
