package core

import (
	"testing"
	"time"

	"nexvpn/internal/config"
)

// A server the engine just proved unreachable sits in a short cooldown, and the
// candidate ordering has to respect that: the 2026-10-11 log re-selected a
// subscription host that refused TCP ~80 times in two minutes because nothing
// in the selection path remembered the failure.

func coolingProfile(id string, latency int) config.Profile {
	return config.Profile{
		ID:                  id,
		Name:                id,
		Server:              "10.0.0.9",
		LatencyMs:           latency,
		ConsecutiveFailures: 2,
		LastFailureAt:       time.Now(),
		LastFailureReason:   "tunnel verification failed: connection refused",
	}
}

func TestRankFailoverCandidatesDemotesCoolingDownServers(t *testing.T) {
	profiles := []config.Profile{
		{ID: "failed", Server: "10.0.0.1", LatencyMs: 10},
		// The fastest number in the list, and the server that just proved it is
		// dead. Latency alone must not be able to put it back in front.
		coolingProfile("dead-but-fast", 5),
		{ID: "healthy-slow", Server: "10.0.0.2", LatencyMs: 300},
		{ID: "healthy-unmeasured", Server: "10.0.0.3"},
	}

	got := ids(RankFailoverCandidates(profiles, "failed", 0))

	if len(got) != 3 {
		t.Fatalf("candidate order = %v, want the three remaining profiles (a cooldown demotes, it never deletes)", got)
	}
	if got[len(got)-1] != "dead-but-fast" {
		t.Errorf("candidate order = %v, want the cooled-down server last: %v", got, "dead-but-fast")
	}
	for i, id := range []string{"healthy-slow", "healthy-unmeasured"} {
		if got[i] != id {
			t.Errorf("candidate %d = %q, want %q — healthy peers keep their own ordering (%v)", i, got[i], id, got)
		}
	}
}

func TestRankFailoverCandidatesKeepsExpiredCooldownsInPlace(t *testing.T) {
	stale := coolingProfile("old-failure", 20)
	stale.LastFailureAt = time.Now().Add(-2 * config.ServerFailureCooldown)

	got := ids(RankFailoverCandidates([]config.Profile{
		stale,
		{ID: "other", Server: "10.0.0.2", LatencyMs: 200},
	}, "", 0))

	if len(got) != 2 || got[0] != "old-failure" {
		t.Errorf("candidate order = %v, want the expired failure ranked by latency again: a server that came back is usable without the user doing anything", got)
	}
}

func TestCountCoolingDown(t *testing.T) {
	profiles := []config.Profile{
		coolingProfile("a", 10),
		coolingProfile("b", 20),
		{ID: "c", Server: "10.0.0.3", LatencyMs: 30},
	}

	if n := CountCoolingDown(profiles); n != 2 {
		t.Errorf("CountCoolingDown = %d, want 2", n)
	}
	if n := CountCoolingDown(nil); n != 0 {
		t.Errorf("CountCoolingDown(nil) = %d, want 0", n)
	}
}

// TestEngineReportsServerHealth covers the contract between the engine, which
// knows whether a server carried traffic, and the app, which persists it.
func TestEngineReportsServerHealth(t *testing.T) {
	type report struct {
		id      string
		healthy bool
		reason  string
	}

	e := NewEngine()
	var got []report
	e.OnServerHealth(func(profileID string, healthy bool, reason string) {
		got = append(got, report{profileID, healthy, reason})
	})

	e.reportServerHealth("node-1", false, "tunnel verification failed: EOF")
	e.reportServerHealth("node-1", true, "")

	if len(got) != 2 {
		t.Fatalf("the engine published %d verdicts, want 2: %v", len(got), got)
	}
	if got[0] != (report{"node-1", false, "tunnel verification failed: EOF"}) {
		t.Errorf("failure verdict = %+v, want the profile id, the verdict and the reason", got[0])
	}
	if got[1] != (report{"node-1", true, ""}) {
		t.Errorf("success verdict = %+v, want {node-1 true }", got[1])
	}

	// A profile with no id cannot be recorded anywhere; reporting it would only
	// produce a store error for a name that does not exist.
	e.reportServerHealth("", false, "reason")
	if len(got) != 2 {
		t.Errorf("an empty profile id was published (%v), want it dropped before the callback", got)
	}
}

func TestEngineWithoutServerHealthListenerIsSafe(t *testing.T) {
	// Nothing is wired up in a bare engine (and in tests, and off the app path).
	// Reporting must not panic — a failure that panics inside the connect
	// goroutine would take the app down instead of logging.
	e := NewEngine()
	e.reportServerHealth("node-1", false, "reason")
	e.reportServerHealth("node-1", true, "")
}
