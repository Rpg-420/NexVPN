package protocols

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/diary"
	"nexvpn/internal/sysproc"
	"nexvpn/internal/sysproxy"
)

type XrayAdapter struct {
	mu          sync.RWMutex
	cmd         *exec.Cmd
	running     bool
	bytesRx     int64
	bytesTx     int64
	tmpConfig   string
	logger      LoggerFunc
	onExit      OnExitFunc
	statsCancel context.CancelFunc
	lastError   string
	// exitCh is closed once the process exits, letting Start() detect a core
	// that dies immediately (bad config, missing TUN rights) instead of
	// blocking on a port probe that can never succeed.
	exitCh   chan struct{}
	exitOnce *sync.Once

	// logReaders tracks the stdout/stderr scanners so the exit monitor can wait
	// for the core's final error line before reporting why it died. Without it a
	// crash is reported as a bare "exit status 1", which the engine reads as a
	// dead server and answers with a pointless failover.
	logReaders sync.WaitGroup

	// logMerger folds the core's repeated lines into one entry plus a count.
	// Fresh per process, created in Start.
	logMerger *coreLogMerger
}

func NewXrayAdapter() *XrayAdapter {
	return &XrayAdapter{exitCh: make(chan struct{}), exitOnce: &sync.Once{}}
}

func (a *XrayAdapter) Name() config.Protocol {
	return config.ProtocolXray
}

func (a *XrayAdapter) SetLogger(fn LoggerFunc) {
	a.mu.Lock()
	a.logger = fn
	a.mu.Unlock()
}

// SetOnExit registers a callback invoked when the xray process exits unexpectedly.
func (a *XrayAdapter) SetOnExit(fn OnExitFunc) {
	a.mu.Lock()
	a.onExit = fn
	a.mu.Unlock()
}

func (a *XrayAdapter) log(level, msg string) {
	a.mu.RLock()
	logger := a.logger
	a.mu.RUnlock()
	if logger != nil {
		logger(level, msg, "xray")
	}
}

func (a *XrayAdapter) Start(ctx context.Context, configContent string) error {
	xrayBin, err := findBinary("xray")
	if err != nil {
		return fmt.Errorf("Xray core binary not found. Please ensure 'xray' binary is in bin folder")
	}

	// Reset the exit signal so a previous run's termination cannot be mistaken
	// for this run failing to start.
	a.mu.Lock()
	a.exitCh = make(chan struct{})
	a.exitOnce = &sync.Once{}
	a.mu.Unlock()

	// Fix #15: Use dedicated secure temp dir
	tmpDir := ensureNexVPNTempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "xray-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(configContent); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	tmpFile.Close()

	binDir := filepath.Dir(xrayBin)
	cmd := exec.CommandContext(ctx, xrayBin, "run", "-config", tmpPath)
	cmd.Dir = binDir
	PrepareHiddenCmd(cmd)

	cmd.Env = append(os.Environ(),
		"xray.location.asset="+binDir,
		"XRAY_LOCATION_ASSET="+binDir,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)

	// A fresh log merger per process: repeat counts from a previous core must
	// never be attributed to this one.
	a.mu.Lock()
	a.logMerger = newCoreLogMerger(coreLogMergeWindow, a.log)
	a.mu.Unlock()

	stdout, err := cmd.StdoutPipe()
	if err == nil {
		a.logReaders.Add(1)
		go func() {
			defer a.logReaders.Done()
			a.streamLogs(stdout, "info")
		}()
	}
	stderr, err := cmd.StderrPipe()
	if err == nil {
		a.logReaders.Add(1)
		go func() {
			defer a.logReaders.Done()
			a.streamLogs(stderr, "warning")
		}()
	}

	if err := cmd.Start(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to start xray process: %w", err)
	}

	sysproc.AssignCmdToJobObject(cmd)

	a.mu.Lock()
	a.cmd = cmd
	a.tmpConfig = tmpPath
	a.running = true
	a.bytesRx = 0
	a.bytesTx = 0
	a.lastError = ""
	a.mu.Unlock()

	a.log("info", fmt.Sprintf("Xray core started (PID: %d)", cmd.Process.Pid))

	statsCtx, statsCancel := context.WithCancel(ctx)
	a.mu.Lock()
	a.statsCancel = statsCancel
	a.mu.Unlock()
	go a.sampleStats(statsCtx)

	// Fix #2: Monitor process exit — fire onExit callback on unexpected crash
	go func() {
		waitErr := cmd.Wait()

		// Give the log scanners a beat to store the fatal line before reading it:
		// cmd.Wait() closes the pipes, but the scanner that saw the reason may not
		// have run yet, and without it the caller only learns "exit status 1".
		waitForLogReaders(&a.logReaders, 2*time.Second)

		// Publish the repeat count of the last merged error run before announcing
		// that the process is gone, so the log reads in the order it happened.
		a.flushCoreLog()

		a.mu.Lock()
		wasRunning := a.running
		a.running = false
		onExit := a.onExit
		exitCh := a.exitCh
		exitOnce := a.exitOnce
		lastXrayError := a.lastError
		a.mu.Unlock()

		if exitCh != nil && exitOnce != nil {
			exitOnce.Do(func() { close(exitCh) })
		}

		a.log("info", "Xray process exited")

		if wasRunning && onExit != nil {
			if waitErr != nil {
				onExit(fmt.Errorf("xray process exited unexpectedly: %w%s", waitErr, coreErrorSuffix(lastXrayError)))
			} else {
				onExit(fmt.Errorf("xray process exited unexpectedly%s", coreErrorSuffix(lastXrayError)))
			}
		}
	}()

	// Startup liveness probe: if the core dies within the first moments (invalid
	// config, TUN adapter creation without admin rights, port conflict) surface
	// it as a Start() error so the engine can fall back to System Proxy mode
	// immediately instead of waiting for the 60s port timeout.
	a.mu.RLock()
	exitCh := a.exitCh
	a.mu.RUnlock()
	startupFailure := func() error {
		a.mu.RLock()
		lastErr := a.lastError
		a.mu.RUnlock()
		a.mu.Lock()
		a.onExit = nil // engine fallback takes over; suppress the crash callback
		a.mu.Unlock()
		if lastErr == "" {
			lastErr = "xray exited immediately after launch"
		}
		diary.Log("connect", "core-exited-early", "core", "xray", "err", lastErr)
		return fmt.Errorf("xray exited during startup: %s", lastErr)
	}

	// A custom JSON profile can carry a TUN inbound, which is created only after
	// launch and can take ~10s — long past the flat 1.5s window. Wait for the
	// adapter instead, so a failure surfaces here (and reaches the engine's TUN
	// -> System Proxy fallback) rather than as an anonymous crash later on.
	if tunName := tunInterfaceName(configContent); tunName != "" && WintunDLLPresent(binDir) {
		// See the matching block in singbox.go: an adapter that already exists
		// satisfies this probe instantly, so its presence is recorded and a short
		// survival window is required before it is trusted as "the core is up".
		preexisting := WintunAdapterReady(binDir, tunName)
		if preexisting {
			diary.Log("connect", "tun-adapter-preexisting", "core", "xray", "adapter", tunName)
			a.log("warning", fmt.Sprintf(
				"A TUN adapter named %s already existed before this connection; readiness will not be trusted until the core has survived its first seconds", tunName))
		}
		spawnedAt := time.Now()

		deadline := time.Now().Add(tunStartupWindow)
		for {
			select {
			case <-exitCh:
				return startupFailure()
			case <-time.After(250 * time.Millisecond):
			}
			if WintunAdapterReady(binDir, tunName) {
				if preexisting && time.Since(spawnedAt) < tunReadinessFloor {
					continue
				}
				diary.Log("connect", "tun-ready", "core", "xray", "adapter", tunName)
				break
			}
			if time.Now().After(deadline) {
				diary.Log("connect", "tun-timeout",
					"core", "xray", "adapter", tunName, "window", tunStartupWindow.String())
				a.mu.Lock()
				a.onExit = nil
				a.mu.Unlock()
				// Release 10808/10809 before the engine starts its fallback core.
				_ = a.Stop()
				return fmt.Errorf(
					"xray is running but its TUN adapter (%s) never came up within %v",
					tunName, tunStartupWindow)
			}
		}
	} else {
		select {
		case <-exitCh:
			return startupFailure()
		case <-time.After(1500 * time.Millisecond):
		}
		diary.Log("connect", "proxy-core-ready", "core", "xray")
	}

	// Probe local port (non-blocking best-effort warm-up)
	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(300 * time.Millisecond)
			conn, err := net.DialTimeout("tcp", "127.0.0.1:10808", 500*time.Millisecond)
			if err == nil {
				conn.Close()
				break
			}
		}
	}()

	return nil
}

func (a *XrayAdapter) sampleStats(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check if proxy port 10808 / 10809 has active connections
			a.mu.RLock()
			running := a.running
			a.mu.RUnlock()
			if !running {
				return
			}
			// Xray's stats API is gRPC-only and is not enabled in the generated
			// config, so this adapter has no byte counters of its own. Returning
			// 0/0 is intentional: the engine's stats loop detects the zero pair
			// and falls back to the OS-level TUN adapter counters
			// (QueryTunnelAdapterBytes). Reporting fabricated numbers here would
			// suppress that fallback and show wrong traffic.
		}
	}
}

func (a *XrayAdapter) streamLogs(r io.Reader, defaultLevel string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		// Xray colours its output for a terminal; the escapes would otherwise end
		// up in the log view, the diary and every bug report built from them.
		text := stripCoreANSI(strings.TrimSpace(scanner.Text()))
		if text == "" {
			continue
		}
		level := defaultLevel
		lower := strings.ToLower(text)

		if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "fatal") {
			level = "error"
			a.mu.Lock()
			a.lastError = text
			a.mu.Unlock()
		} else if strings.Contains(lower, "warn") {
			level = "warning"
		} else if strings.Contains(lower, "info") {
			level = "info"
		}
		a.mergeCoreLog(level, text)
	}
}

// mergeCoreLog routes one core line through the per-process merger, so a run of
// identical failures costs one entry plus a count rather than a line per failed
// connection.
func (a *XrayAdapter) mergeCoreLog(level, text string) {
	a.mu.RLock()
	merger := a.logMerger
	a.mu.RUnlock()

	if merger == nil {
		a.log(level, text)
		return
	}
	merger.add(level, text)
}

// flushCoreLog publishes the repeats counted for the last open run. Safe to
// call more than once.
func (a *XrayAdapter) flushCoreLog() {
	a.mu.RLock()
	merger := a.logMerger
	a.mu.RUnlock()

	if merger != nil {
		merger.Flush()
	}
}

func (a *XrayAdapter) Stop() error {
	a.mu.Lock()
	if a.statsCancel != nil {
		a.statsCancel()
		a.statsCancel = nil
	}
	cmd := a.cmd
	tmpConfig := a.tmpConfig
	a.cmd = nil
	a.tmpConfig = ""
	a.running = false
	// Fix #2: Clear onExit before stopping — prevent false alarm on intentional stop
	a.onExit = nil
	a.mu.Unlock()

	_ = sysproxy.ClearSystemProxy()

	if cmd != nil && cmd.Process != nil {
		pid := cmd.Process.Pid
		a.log("info", fmt.Sprintf("Stopping Xray core (PID: %d)...", pid))
		if err := KillProcessTree(pid); err != nil {
			// Same reasoning as sing-box: a core that survived the kill still
			// holds its listener and its config file, so the failure has to
			// reach the log rather than be swallowed here.
			a.log("error", fmt.Sprintf("Xray core (PID: %d) did not stop: %v", pid, err))
		}
		a.flushCoreLog()
		// Only a core that was actually running gets a "stopped" line: several
		// paths stop the same process (TUN timeout, engine fallback, disconnect,
		// exit callback), and each duplicate makes a teardown look like a crash
		// loop when reading the log afterwards.
		a.log("info", "Xray core stopped")
	}
	if tmpConfig != "" {
		_ = os.Remove(tmpConfig)
	}

	return nil
}

func (a *XrayAdapter) GetStats() (int64, int64, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if !a.running {
		return 0, 0, nil
	}
	return a.bytesRx, a.bytesTx, nil
}

func (a *XrayAdapter) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}
