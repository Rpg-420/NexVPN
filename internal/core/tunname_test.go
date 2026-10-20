package core

import (
	"strings"
	"testing"

	"nexvpn/internal/config"
)

// TestTunInterfaceCandidates covers the naming rules the ghost-avoidance relies
// on: the default first, then a bounded set of suffixed alternatives.
//
// The bound matters. Every extra candidate is a name the cleanup script has to
// know about, and an unbounded list would let a machine that keeps ghosting
// adapters accumulate one per connection. It also has to stay in step with the
// `$names` array in RemoveStaleTUNAdapter, or an adapter chosen here could never
// be deleted.
func TestTunInterfaceCandidates(t *testing.T) {
	names := tunInterfaceCandidates()

	if len(names) < 2 {
		t.Fatalf("a ghost on the default name must have somewhere to go, got %v", names)
	}
	if names[0] != config.DefaultTunInterfaceName {
		t.Errorf("the default must be tried first, got %q", names[0])
	}

	seen := make(map[string]bool, len(names))
	for _, n := range names {
		if n == "" {
			t.Error("a candidate must not be empty")
		}
		if seen[n] {
			t.Errorf("duplicate candidate %q", n)
		}
		seen[n] = true
		if !strings.HasPrefix(n, config.DefaultTunInterfaceName) {
			t.Errorf("candidate %q must stay inside the NexVPN namespace", n)
		}
	}

	// The suffixes RemoveStaleTUNAdapter deletes. If a name is chosen that the
	// cleanup cannot see, a dead adapter would linger and be reused forever.
	for _, n := range names[1:] {
		if !strings.HasPrefix(n, config.DefaultTunInterfaceName+"-") {
			t.Errorf("suffixed candidate %q must be %s-<n> so cleanup can match it", n, config.DefaultTunInterfaceName)
		}
	}
}

// TestChooseTunInterfaceNameAlwaysReturnsACandidate is the safety property: on
// every path — including "the OS adapter list could not be read" and "every
// candidate is ghosted" — the function returns a name from the candidate list,
// never an empty string. An empty name would make the core fall back to its own
// default and reintroduce the collision the function exists to avoid.
func TestChooseTunInterfaceNameAlwaysReturnsACandidate(t *testing.T) {
	got := chooseTunInterfaceName()
	for _, n := range tunInterfaceCandidates() {
		if got == n {
			return
		}
	}
	t.Errorf("chooseTunInterfaceName returned %q, which is not a known candidate", got)
}
