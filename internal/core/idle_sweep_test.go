package core

import (
	"testing"
)

// SweepIdleCores exists to reap a core that outlived its session, and its one
// dangerous property is that it calls CleanupStaleCores — which kills every
// registered PID. A timer around that function is only safe because of the state
// guard, so the guard is what these tests pin.
func TestSweepIdleCoresNeverRunsWhileASessionIsLive(t *testing.T) {
	for _, state := range []ConnectionState{StateConnected, StateConnecting, StateDisconnecting} {
		e := NewEngine()
		e.mu.Lock()
		e.status.State = state
		e.mu.Unlock()

		if swept := e.SweepIdleCores(); swept != 0 {
			t.Errorf("SweepIdleCores while %s = %d, want 0: killing the registered PID would drop the user's tunnel", state, swept)
		}
	}
}

func TestSweepIdleCoresIsQuietWithNothingRegistered(t *testing.T) {
	e := NewEngine()
	e.mu.Lock()
	e.status.State = StateDisconnected
	e.mu.Unlock()

	// The interesting property is that this path is a no-op rather than an error:
	// it runs on a timer, so any noise it makes is noise the user did not ask for.
	if swept := e.SweepIdleCores(); swept != 0 {
		t.Errorf("SweepIdleCores with no registered PID = %d, want 0", swept)
	}
}

// There is deliberately no test that drives handleCoreExit end to end: it runs
// setErrorStatus, which clears the Windows system proxy, and a unit test has no
// business changing machine-wide network state on the machine it runs on. The
// duplicated crash line it fixes (two hand-copied onExit bodies, one of which
// logged the same message twice) is verified by reading the single shared
// implementation instead.
