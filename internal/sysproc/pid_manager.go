package sysproc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

var (
	pidMu   sync.Mutex
	pidFile string
)

func init() {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.TempDir()
	}
	dir := filepath.Join(appData, "NexVPN", "tmp")
	_ = os.MkdirAll(dir, 0700)
	pidFile = filepath.Join(dir, "run.pid")
}

func readPIDs() []int {
	var pids []int
	data, err := os.ReadFile(pidFile)
	if err == nil {
		_ = json.Unmarshal(data, &pids)
	}
	return pids
}

func writePIDs(pids []int) {
	data, err := json.Marshal(pids)
	if err == nil {
		_ = os.WriteFile(pidFile, data, 0600)
	}
}

// RegisterPID adds a PID to the tracking file
func RegisterPID(pid int) {
	if pid <= 0 {
		return
	}
	pidMu.Lock()
	defer pidMu.Unlock()
	pids := readPIDs()
	for _, p := range pids {
		if p == pid {
			return
		}
	}
	pids = append(pids, pid)
	writePIDs(pids)
}

// UnregisterPID removes a PID from the tracking file
func UnregisterPID(pid int) {
	if pid <= 0 {
		return
	}
	pidMu.Lock()
	defer pidMu.Unlock()
	pids := readPIDs()
	var kept []int
	for _, p := range pids {
		if p != pid {
			kept = append(kept, p)
		}
	}
	writePIDs(kept)
}

// GetRegisteredPIDs returns all currently tracked PIDs
func GetRegisteredPIDs() []int {
	pidMu.Lock()
	defer pidMu.Unlock()
	return readPIDs()
}

// ClearPIDs empties the PID tracking file
func ClearPIDs() {
	pidMu.Lock()
	defer pidMu.Unlock()
	_ = os.Remove(pidFile)
}
