package protocols

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/diary"
	"nexvpn/internal/sysproc"
	"nexvpn/internal/sysproxy"
)

type SingBoxAdapter struct {
	mu          sync.RWMutex
	cmd         *exec.Cmd
	running     bool
	bytesRx     int64
	bytesTx     int64
	tmpConfig   string
	clashPort   int
	logger      LoggerFunc
	onExit      OnExitFunc
	statsCancel context.CancelFunc
	lastError   string
	lockFile    string

	// exitCh is closed by waitForExit once the core process terminates; used to
	// detect an immediate startup failure (bad config, TUN permission denied, ...)
	// so the engine can fall back to System Proxy mode instead of waiting forever.
	exitCh   chan struct{}
	exitOnce sync.Once

	// logReaders tracks the stdout/stderr scanners. waitForExit waits on it
	// before reading lastError: cmd.Wait() closes the pipes, but the scanner that
	// is about to store the fatal line may not have run yet, and that line is the
	// only thing that explains an otherwise anonymous "exit status 1".
	logReaders sync.WaitGroup

	// logMerger folds the cores' error storms into one line plus a count. Fresh
	// per process, created in Start.
	logMerger *coreLogMerger
}

func NewSingBoxAdapter() *SingBoxAdapter {
	return &SingBoxAdapter{}
}

func (a *SingBoxAdapter) Name() config.Protocol {
	return config.ProtocolSingBox
}

func (a *SingBoxAdapter) SetLogger(fn LoggerFunc) {
	a.mu.Lock()
	a.logger = fn
	a.mu.Unlock()
}

// SetOnExit registers a callback invoked when the sing-box process exits unexpectedly.
func (a *SingBoxAdapter) SetOnExit(fn OnExitFunc) {
	a.mu.Lock()
	a.onExit = fn
	a.mu.Unlock()
}

func (a *SingBoxAdapter) log(level, msg string) {
	a.mu.RLock()
	logger := a.logger
	a.mu.RUnlock()
	if logger != nil {
		logger(level, msg, "sing-box")
	}
}

// getFreeLocalPort finds an available TCP port for Clash API to prevent 9090 port conflicts
func getFreeLocalPort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 19090
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

func (a *SingBoxAdapter) Start(ctx context.Context, configContent string) error {
	sbBin, err := findBinary("sing-box")
	if err != nil {
		return fmt.Errorf("sing-box core binary not found, please ensure the 'sing-box' binary is in the bin folder")
	}

	// Dynamically allocate a free local port for clash_api
	port := getFreeLocalPort()
	a.mu.Lock()
	a.clashPort = port
	a.mu.Unlock()

	configContent = injectClashConfig(configContent, port)
	configContent = injectCacheFile(configContent, singBoxCachePath())

	// Fix #15: Use dedicated secure temp dir
	tmpDir := ensureNexVPNTempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "singbox-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(configContent); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	_ = tmpFile.Close()

	binDir := filepath.Dir(sbBin)
	cmdArgs := []string{"run", "-c", tmpPath, "-D", binDir}
	var cmd *exec.Cmd
	var lockFilePath string

	u, errUser := user.Current()
	isRoot := errUser == nil && u.Uid == "0"

	if runtime.GOOS == "linux" && !isRoot && !hasNetCap(sbBin) && strings.Contains(configContent, `"type": "tun"`) {
		if _, err := exec.LookPath("pkexec"); err == nil {
			lockFile, err := os.CreateTemp(ensureNexVPNTempDir(), "singbox-lock-*.lock")
			if err == nil {
				lockFilePath = lockFile.Name()
				_ = lockFile.Close()
				script := `
					"$1" run -c "$2" -D "$3" &
					pid=$!
					while kill -0 $pid 2>/dev/null && [ -f "$4" ]; do
						sleep 0.5
					done
					kill -9 $pid 2>/dev/null
					exit 0
				`
				args := []string{"sh", "-c", script, "sh", sbBin, tmpPath, binDir, lockFilePath}
				cmd = exec.CommandContext(ctx, "pkexec", args...)
			}
		}
	}

	if cmd == nil {
		cmd = exec.CommandContext(ctx, sbBin, cmdArgs...)
	}

	cmd.Dir = binDir
	// A fresh log merger per process: repeat counts from a previous core must
	// never be attributed to this one.
	a.mu.Lock()
	a.logMerger = newCoreLogMerger(coreLogMergeWindow, a.log)
	a.mu.Unlock()
	// IMPORTANT: the generated sing-box config still uses several fields that
	// upstream has marked deprecated (legacy DNS servers, fakeip options, the
	// special/wireguard outbounds, legacy inbound/tun keys). These environment
	// variables keep them working, but they are a hard coupling to the installed
	// core version: a sing-box release that removes the compatibility shim will
	// make the app fail to start its tunnel. When upgrading the bundled core,
	// migrate the config generator (BuildSingBoxConfig in internal/parser) to the
	// current schema and drop the corresponding flag below.
	cmd.Env = append(os.Environ(),
		"ENABLE_DEPRECATED_MISSING_DOMAIN_RESOLVER=true",
		"ENABLE_DEPRECATED_SPECIAL_OUTBOUNDS=true",
		"ENABLE_DEPRECATED_WIREGUARD_OUTBOUND=true",
		"ENABLE_DEPRECATED_LEGACY_DNS_SERVERS=true",
		"ENABLE_DEPRECATED_LEGACY_DNS_FAKEIP_OPTIONS=true",
		"ENABLE_DEPRECATED_FAKEIP=true",
		"ENABLE_DEPRECATED_LEGACY_INBOUND=true",
		"ENABLE_DEPRECATED_INBOUND=true",
		"ENABLE_DEPRECATED_TUN=true",
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"DBUS_SYSTEM_BUS_ADDRESS=unix:path=/dev/null", // Prevent sing-box from triggering NetworkManager Polkit route prompt
	)
	PrepareHiddenCmd(cmd)

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
			a.streamLogs(stderr, "error")
		}()
	}

	if err := cmd.Start(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to start sing-box: %w", err)
	}

	sysproc.AssignCmdToJobObject(cmd)

	a.mu.Lock()
	a.cmd = cmd
	a.running = true
	a.tmpConfig = tmpPath
	a.bytesRx = 0
	a.bytesTx = 0
	a.lastError = ""
	a.lockFile = lockFilePath
	a.exitCh = make(chan struct{})
	a.mu.Unlock()

	a.log("info", fmt.Sprintf("Sing-box core started (PID: %d)", cmd.Process.Pid))

	statsCtx, statsCancel := context.WithCancel(ctx)
	a.mu.Lock()
	a.statsCancel = statsCancel
	a.mu.Unlock()
	go a.pollClashStats(statsCtx)

	go a.waitForExit(cmd, tmpPath)

	// Startup liveness probe: a core that dies within the first moments (TUN
	// adapter creation without admin rights, invalid config, port conflicts, ...)
	// must surface as a Start() error so the engine can fall back to System
	// Proxy mode instead of blocking on a port probe that can never succeed.
	startupFailure := func() error {
		a.mu.RLock()
		lastErr := a.lastError
		a.mu.RUnlock()
		a.mu.Lock()
		a.onExit = nil // engine fallback takes over; suppress the crash callback
		a.mu.Unlock()
		if lastErr == "" {
			lastErr = "sing-box exited immediately after launch"
		}
		diary.Log("connect", "core-exited-early", "core", "sing-box", "err", lastErr)
		if hint := singboxSchemaHint(lastErr); hint != "" {
			return fmt.Errorf("sing-box exited during startup: %s (%s)", lastErr, hint)
		}
		return fmt.Errorf("sing-box exited during startup: %s", lastErr)
	}

	if tunName := tunInterfaceName(configContent); tunName != "" && WintunDLLPresent(binDir) {
		// A TUN config needs a longer, *readiness-based* window than the flat
		// 1.5s a proxy-only core needs.
		//
		// The Wintun adapter is created after the process launches and can take
		// ~10s, during which sing-box logs "open interface take too much time to
		// finish!"; if the adapter cannot be created it then exits fatally ~15s
		// in. With the old flat window the core passed this probe, the engine
		// verified reachability through the SOCKS port (already up and working)
		// and announced "connected" — and the core died ~13s later. That crash
		// carried no reason, so the engine read it as a dead *server* and spent
		// every auto-failover attempt switching servers, which can never fix a
		// local adapter fault. Waiting for the adapter here routes the failure
		// into the engine's existing TUN -> System Proxy fallback instead.
		//
		// `preexisting` matters because the probe only asks whether an adapter
		// with this name exists, and a leftover one answers yes immediately. The
		// engine removes NexVPN's adapters before spawning a TUN core
		// (core.PrepareTUNForConnect), so this should be false — but if it is not,
		// an instant "ready" would report a core that is about to die as
		// connected, which is the exact failure this probe exists to prevent.
		preexisting := WintunAdapterReady(binDir, tunName)
		if preexisting {
			diary.Log("connect", "tun-adapter-preexisting", "core", "sing-box", "adapter", tunName)
			a.log("warning", fmt.Sprintf(
				"A TUN adapter named %s already existed before this connection; readiness will not be trusted until the core has survived its first seconds", tunName))
		}
		spawnedAt := time.Now()

		deadline := time.Now().Add(tunStartupWindow)
		for {
			select {
			case <-a.exitCh:
				return startupFailure()
			case <-time.After(250 * time.Millisecond):
			}
			if WintunAdapterReady(binDir, tunName) {
				// Give a core that inherited a stale adapter a chance to fail
				// first. Without this the probe can report "connected" for a
				// process that exits a moment later, and the engine answers that
				// by failing over to another server.
				if preexisting && time.Since(spawnedAt) < tunReadinessFloor {
					continue
				}
				diary.Log("connect", "tun-ready", "core", "sing-box", "adapter", tunName)
				break
			}
			if time.Now().After(deadline) {
				diary.Log("connect", "tun-timeout",
					"core", "sing-box", "adapter", tunName, "window", tunStartupWindow.String())
				a.mu.Lock()
				a.onExit = nil
				a.mu.Unlock()
				// The process is still alive and holding 10808/10809 — it has to
				// go before the engine can start its System Proxy fallback core,
				// otherwise that core fails to bind the ports.
				_ = a.Stop()
				return fmt.Errorf(
					"sing-box is running but its TUN adapter (%s) never came up within %v",
					tunName, tunStartupWindow)
			}
		}
	} else {
		select {
		case <-a.exitCh:
			return startupFailure()
		case <-time.After(1500 * time.Millisecond):
		}
		diary.Log("connect", "proxy-core-ready", "core", "sing-box")
	}

	return nil
}

// singboxSchemaHint inspects a startup failure and, when it looks like a
// config-schema incompatibility, explains the likely cause (an upgraded core
// that dropped the deprecated fields this app still emits).
func singboxSchemaHint(lastErr string) string {
	lower := strings.ToLower(lastErr)
	switch {
	case strings.Contains(lower, "unknown field"),
		strings.Contains(lower, "deprecated"),
		strings.Contains(lower, "json: cannot unmarshal"),
		strings.Contains(lower, "unmarshal"),
		strings.Contains(lower, "legacy"):
		return "the bundled sing-box core is likely newer than the generated config schema; update the app or reinstall a matching core"
	default:
		return ""
	}
}

// tunStartupWindow bounds how long Start() waits for a TUN adapter to appear.
//
// It sits just above sing-box's own ~10s interface-open timeout plus the retry
// that follows it, because that retry is exactly where a ghost adapter turns
// into a fatal exit and takes the whole core down.
const tunStartupWindow = 18 * time.Second

// tunReadinessFloor is how long a core must survive before an adapter that was
// already present counts as ready.
//
// The readiness probe cannot tell "the core created this adapter" from "a
// previous run left it behind" — both answer yes. Real adapter creation takes
// around ten seconds, so an adapter that is openable a fraction of a second
// after spawn is almost certainly the stale one, and a core in that situation is
// usually about to exit. Requiring a short survival window means such a core is
// reported through startupFailure() (which carries the core's own error) instead
// of being announced as connected.
const tunReadinessFloor = 2500 * time.Millisecond

// tunInterfaceName returns the `interface_name` of the first TUN inbound in the
// config, or "" when the config has no TUN inbound (proxy-only modes).
func tunInterfaceName(configContent string) string {
	var parsed struct {
		Inbounds []struct {
			Type          string `json:"type"`
			InterfaceName string `json:"interface_name"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal([]byte(configContent), &parsed); err != nil {
		return ""
	}
	for _, in := range parsed.Inbounds {
		if in.Type == "tun" && in.InterfaceName != "" {
			return in.InterfaceName
		}
	}
	return ""
}

func (a *SingBoxAdapter) waitForExit(cmd *exec.Cmd, tmpPath string) {
	err := cmd.Wait()

	// cmd.Wait() closes the stdout/stderr pipes, so the scanners are about to hit
	// EOF — but they may not have stored the fatal line yet, and that line is the
	// entire reason this function reports anything. Without the wait, every crash
	// surfaced as a bare "exit status 1" and the engine read it as a dead server.
	waitForLogReaders(&a.logReaders, 2*time.Second)

	// Publish the repeat count of the last merged error run before announcing
	// that the process is gone, so the log reads in the order it happened.
	a.flushCoreLog()

	a.mu.Lock()
	wasRunning := a.running
	a.running = false
	a.cmd = nil
	lastErr := a.lastError
	a.mu.Unlock()

	a.exitOnce.Do(func() { close(a.exitCh) })

	_ = os.Remove(tmpPath)

	if wasRunning {
		a.log("info", "Sing-box process exited")
		if err != nil {
			a.mu.RLock()
			onExit := a.onExit
			a.mu.RUnlock()
			if onExit != nil {
				onExit(fmt.Errorf("sing-box process exited unexpectedly: %w%s", err, coreErrorSuffix(lastErr)))
			}
		}
	}
}

// coreErrorSuffix appends the core's own last error line to a process-exit
// message, so a caller — and the diary — sees the reason instead of just the
// exit code. Returns "" when the core printed nothing useful.
func coreErrorSuffix(lastErr string) string {
	lastErr = strings.TrimSpace(lastErr)
	if lastErr == "" {
		return ""
	}
	return " — core said: " + lastErr
}

// waitForLogReaders blocks until the log scanners finish, or until d elapses.
//
// The bound matters: a scanner stuck on a pipe that never reaches EOF must not
// be able to wedge process teardown. A late log line is worth waiting for; a
// hung Stop() is not.
func waitForLogReaders(wg *sync.WaitGroup, d time.Duration) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(d):
	}
}

// singBoxCachePath returns a writable path for the core's own persistent store,
// or "" when there is nowhere safe to put it.
//
// The data directory is used rather than the core's base directory (`-D`, which
// points at the bin folder next to the executable) because that folder is
// read-only for an install under Program Files, and a core that cannot open its
// cache file is a core that fails at startup.
func singBoxCachePath() string {
	for _, dir := range []string{config.DefaultDataDir(), ensureNexVPNTempDir()} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			continue
		}
		return filepath.Join(dir, "sing-box-cache.db")
	}
	return ""
}

// injectCacheFile enables the core's persistent cache, including the fake-IP
// store.
//
// A fresh core process has no idea that 198.18.0.7 was www.facebook.com five
// seconds ago — and Windows keeps that synthetic address in its DNS resolver
// cache across reconnects, so the first connections of a new session arrive
// addressed to a fake IP nobody can map back to a name. Those connections are
// dropped, which is exactly the `router: missing fakeip record, try enable
// 'experimental.cache_file'` burst (fourteen lines in two seconds) in the
// 2026-10-11 log.
//
// The path is injected at spawn time instead of being written into the stored
// profile: profiles are portable, and one created on another machine must not
// point at that machine's data directory.
func injectCacheFile(configContent, cachePath string) string {
	if cachePath == "" {
		return configContent
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configContent), &cfg); err != nil {
		return configContent
	}

	exp, ok := cfg["experimental"].(map[string]interface{})
	if !ok {
		exp = make(map[string]interface{})
	}
	// An imported config that already configured its own cache is respected.
	if _, exists := exp["cache_file"]; !exists {
		exp["cache_file"] = map[string]interface{}{
			"enabled":      true,
			"store_fakeip": true,
			"path":         cachePath,
		}
	}
	cfg["experimental"] = exp

	if b, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		return string(b)
	}
	return configContent
}

// injectClashConfig injects the dynamic port and secret to clash_api
func injectClashConfig(configContent string, port int) string {
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configContent), &cfg); err != nil {
		return configContent
	}
	exp, ok := cfg["experimental"].(map[string]interface{})
	if !ok {
		exp = make(map[string]interface{})
	}
	clashAPI, ok := exp["clash_api"].(map[string]interface{})
	if !ok {
		clashAPI = make(map[string]interface{})
	}
	clashAPI["external_controller"] = fmt.Sprintf("127.0.0.1:%d", port)
	clashAPI["secret"] = ""
	exp["clash_api"] = clashAPI
	cfg["experimental"] = exp
	if b, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		return string(b)
	}
	return configContent
}

func (a *SingBoxAdapter) pollClashStats(ctx context.Context) {
	client := &http.Client{Timeout: 5 * time.Second}

	a.mu.RLock()
	cPort := a.clashPort
	a.mu.RUnlock()
	if cPort <= 0 {
		cPort = 19090
	}
	trafficURL := fmt.Sprintf("http://127.0.0.1:%d/traffic", cPort)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		req, err := http.NewRequestWithContext(ctx, "GET", trafficURL, nil)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				_ = resp.Body.Close()
				return
			default:
			}

			line := scanner.Bytes()
			var traffic struct {
				Up   int64 `json:"up"`
				Down int64 `json:"down"`
			}
			if err := json.Unmarshal(line, &traffic); err == nil {
				a.mu.Lock()
				a.bytesRx += traffic.Down
				a.bytesTx += traffic.Up
				a.mu.Unlock()
			}
		}
		_ = resp.Body.Close()
		time.Sleep(1 * time.Second)
	}
}

func (a *SingBoxAdapter) streamLogs(r io.Reader, defaultLevel string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		// The cores format their output for a terminal. The escape sequences are
		// stripped before anything is stored, otherwise they reach the log view,
		// the diary and the report file a user attaches to a bug report.
		text := stripCoreANSI(strings.TrimSpace(scanner.Text()))
		if text == "" {
			continue
		}
		level := coreLogLevel(text, defaultLevel)
		if level == "error" {
			a.mu.Lock()
			a.lastError = text
			a.mu.Unlock()
		}
		a.mergeCoreLog(level, text)
	}
}

// mergeCoreLog hands one core line to the per-process merger, so a run of
// identical errors costs one log entry plus a count instead of a line per
// failed connection.
func (a *SingBoxAdapter) mergeCoreLog(level, text string) {
	a.mu.RLock()
	merger := a.logMerger
	a.mu.RUnlock()

	if merger == nil {
		a.log(level, text)
		return
	}
	merger.add(level, text)
}

// flushCoreLog publishes the repeats counted for the last run of identical
// lines. Safe to call more than once.
func (a *SingBoxAdapter) flushCoreLog() {
	a.mu.RLock()
	merger := a.logMerger
	a.mu.RUnlock()

	if merger != nil {
		merger.Flush()
	}
}

func (a *SingBoxAdapter) Stop() error {
	a.mu.Lock()
	if a.statsCancel != nil {
		a.statsCancel()
		a.statsCancel = nil
	}
	cmd := a.cmd
	tmpConfig := a.tmpConfig
	lockFile := a.lockFile
	a.cmd = nil
	a.tmpConfig = ""
	a.lockFile = ""
	a.running = false
	// Fix #2: Clear onExit before stopping — prevent false alarm on intentional stop
	a.onExit = nil
	a.mu.Unlock()

	_ = sysproxy.ClearSystemProxy()

	if cmd != nil && cmd.Process != nil {
		pid := cmd.Process.Pid
		a.log("info", fmt.Sprintf("Stopping Sing-box core (PID: %d)...", pid))
		if lockFile != "" {
			_ = os.Remove(lockFile)
			time.Sleep(200 * time.Millisecond) // Give the script time to catch the deleted lockfile and kill the process gracefully
		}
		if err := KillProcessTree(pid); err != nil {
			// The core is still alive and still holding its listener, so the
			// caller must not be told the stop succeeded. Returning the error is
			// what lets the engine free 10808 before spawning the next core;
			// logging "Sing-box stopped" over a live process is how
			// `bind: Only one usage of each socket address` came back.
			a.mu.Lock()
			a.lastError = err.Error()
			a.mu.Unlock()
			a.log("error", fmt.Sprintf("Sing-box core (PID: %d) did not stop: %v", pid, err))
			if tmpConfig != "" {
				_ = os.Remove(tmpConfig)
			}
			return fmt.Errorf("sing-box process %d did not stop: %w", pid, err)
		}
		a.flushCoreLog()
		if tmpConfig != "" {
			_ = os.Remove(tmpConfig)
		}
		// Only a core that was actually running gets a "stopped" line. Several
		// paths call Stop() for the same process — the TUN timeout, the engine's
		// fallback, the disconnect, the exit callback — and each one used to
		// announce a stop that had already happened, which turned a four-line
		// teardown into a wall of duplicates in every log.
		a.log("info", "Sing-box stopped")
	}
	return nil
}

func (a *SingBoxAdapter) GetStats() (int64, int64, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if !a.running {
		return 0, 0, nil
	}
	return a.bytesRx, a.bytesTx, nil
}

func (a *SingBoxAdapter) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}
