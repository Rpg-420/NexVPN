package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/diary"
	"nexvpn/internal/sysproc"
	"nexvpn/internal/sysproxy"
)

// connectJournalFile is the on-disk marker that records "a connection attempt
// or session is in flight".
//
// It exists because the damage a VPN client does when it dies is asymmetric:
// a clean exit runs shutdown, which stops the core, clears the system proxy and
// removes the kill-switch firewall rules. A killed process runs none of that,
// and what is left behind — a core still holding 127.0.0.1:10808, a Wintun
// adapter Windows can no longer enumerate, a system proxy pointing at a dead
// port, a firewall policy that blocks everything — makes the *next* launch fail
// in ways that look like unrelated bugs.
//
// The file is written when a connection attempt starts and removed only when
// the state is genuinely resolved: a clean disconnect, a failure the app is
// still alive to handle, or a clean shutdown. Anything still on disk at startup
// therefore means "the previous process ended without cleaning up", which is
// exactly the condition worth repairing.
const connectJournalFile = "connect.lock"

// ConnectJournal is the contents of the marker file. It is written for a human
// reading a bug report as much as for the repair routine: it answers "what was
// the app doing when it died".
type ConnectJournal struct {
	// Stage is "connecting" or "connected".
	Stage     string    `json:"stage"`
	ProfileID string    `json:"profile_id,omitempty"`
	Profile   string    `json:"profile"`
	Protocol  string    `json:"protocol"`
	Server    string    `json:"server"`
	Mode      string    `json:"mode"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConnectJournalPath returns the absolute path of the marker file.
func ConnectJournalPath() string {
	return filepath.Join(config.DefaultDataDir(), connectJournalFile)
}

// WriteConnectJournal records the current connection attempt.
//
// A failure to write is not fatal and is deliberately not surfaced: the marker
// is a recovery aid, and refusing to connect because a bookkeeping file could
// not be created would trade a rare inconvenience for a total outage.
func WriteConnectJournal(journal ConnectJournal) {
	journal.UpdatedAt = time.Now()
	if journal.StartedAt.IsZero() {
		journal.StartedAt = journal.UpdatedAt
	}

	path := ConnectJournalPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return
	}

	payload, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return
	}

	// Write-then-rename, matching the config store: a crash midway through the
	// write must not leave a truncated marker that ReadConnectJournal would
	// then refuse to parse, because an unparseable marker is indistinguishable
	// from no marker and would silently disable the repair.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0600); err != nil {
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
	}
}

// ClearConnectJournal removes the marker. Removing a file that is not there is
// success, not an error, so callers can call it unconditionally.
func ClearConnectJournal() {
	_ = os.Remove(ConnectJournalPath())
	_ = os.Remove(ConnectJournalPath() + ".tmp")
}

// ReadConnectJournal returns the marker when one is present and parseable.
func ReadConnectJournal() (ConnectJournal, bool) {
	payload, err := os.ReadFile(ConnectJournalPath())
	if err != nil {
		return ConnectJournal{}, false
	}
	var journal ConnectJournal
	if err := json.Unmarshal(payload, &journal); err != nil {
		// A corrupt marker still proves the previous run did not exit cleanly,
		// so report its existence with an empty body rather than pretending
		// nothing happened.
		return ConnectJournal{Stage: "unknown"}, true
	}
	return journal, true
}

// AutoRepairFromJournal repairs whatever the previous run left behind, and
// reports what it did so the caller can log it.
//
// The second return value reports whether a marker was found at all. A false
// return means the previous run exited cleanly and nothing was touched — this
// routine never acts speculatively, because every action it takes (killing a
// process, deleting a network adapter, rewriting firewall policy) is
// destructive if the premise is wrong.
func AutoRepairFromJournal() (ConnectJournal, []string, bool) {
	journal, found := ReadConnectJournal()
	if !found {
		return ConnectJournal{}, nil, false
	}

	actions := []string{}

	// 1. Terminate any core this app started that is still running. This is
	//    scoped to NexVPN's own core images, never "whatever holds the port".
	CleanupStaleCores()
	actions = append(actions, "terminated orphaned core processes")

	// 2. Free the two local proxy ports. 10808 and 10809 are hard-coded
	//    throughout the app, so a leftover listener is a guaranteed failure of
	//    the next connection attempt, not a theoretical one.
	for _, port := range []int{10808, 10809} {
		if releaseProxyPort(port, 2*time.Second) {
			actions = append(actions, fmt.Sprintf("freed port %d", port))
			continue
		}
		actions = append(actions, fmt.Sprintf("port %d still held by %s",
			port, sysproc.DescribeProcessHoldingPort(port)))
	}

	// 3. Remove a leftover NexVPN TUN adapter. A ghost adapter is the worst of
	//    the leftovers: it satisfies a core's readiness probe instantly, so a
	//    dying core looks connected.
	RemoveStaleTUNAdapter()
	actions = append(actions, "removed stale TUN adapters")

	// 4. Point the system proxy back at nothing. A proxy still aimed at
	//    127.0.0.1:10808 after the core died is a total loss of connectivity
	//    that looks like "the internet is down" rather than "the VPN is off".
	if err := sysproxy.ClearSystemProxy(); err != nil {
		actions = append(actions, fmt.Sprintf("system proxy reset failed: %v", err))
	} else {
		actions = append(actions, "system proxy reset")
	}

	// 5. Flush DNS and the route cache. The cores install their own resolvers,
	//    and a stale entry outlives the process that created it.
	_ = sysproxy.FlushDNSAndResetNetwork()
	actions = append(actions, "DNS and route cache flushed")

	// 6. Remove a partially applied kill switch. This runs only inside the
	//    repair path, i.e. only when the previous run died mid-connection, so
	//    it cannot tear down a kill switch the user legitimately has on.
	if err := GetKillSwitch().ForceCleanup(); err != nil {
		actions = append(actions, fmt.Sprintf("kill switch cleanup failed: %v", err))
	} else {
		actions = append(actions, "kill switch rules cleared")
	}

	ClearConnectJournal()

	diary.Log("startup", "auto-repair",
		"stage", journal.Stage,
		"profile", journal.Profile,
		"server", journal.Server,
		"died_at", journal.UpdatedAt.Format(time.RFC3339),
		"actions", fmt.Sprintf("%d", len(actions)))

	return journal, actions, true
}
