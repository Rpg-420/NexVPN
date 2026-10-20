package config

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// The engine proves a server unreachable; the store is what remembers it. These
// tests pin that memory, because the cooldown it creates is what stops failover,
// "fastest server" and the automatic retries from walking back onto a host that
// refused a connection seconds earlier.

func addFailureProfile(t *testing.T, s *Store, id string) Profile {
	t.Helper()

	ids, err := s.AddProfileBatch([]Profile{{ID: id, Name: id, Protocol: ProtocolSingBox, Server: "203.0.113.9"}})
	if err != nil || len(ids) != 1 {
		t.Fatalf("seeding the profile failed: %v (%v)", err, ids)
	}
	return s.GetProfiles()[0]
}

func TestRecordProfileFailureCountsAndCoolsDown(t *testing.T) {
	s := isolatedStore(t)

	p := addFailureProfile(t, s, "dead-node")
	if err := s.RecordProfileFailure(p.ID, "dial tcp 203.0.113.9:443: connectex refused"); err != nil {
		t.Fatalf("RecordProfileFailure failed: %v", err)
	}

	got := s.GetProfiles()[0]
	if got.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures = %d, want 1", got.ConsecutiveFailures)
	}
	if got.LastFailureAt.IsZero() {
		t.Errorf("LastFailureAt was not recorded")
	}
	if !strings.Contains(got.LastFailureReason, "connectex") {
		t.Errorf("LastFailureReason = %q, want the reason that was passed", got.LastFailureReason)
	}
	if !ProfileIsCoolingDown(got, time.Now(), 0) {
		t.Errorf("a profile that just failed must be inside its cooldown")
	}
	if ProfileIsCoolingDown(got, time.Now().Add(ServerFailureCooldown+time.Minute), 0) {
		t.Errorf("the cooldown must expire on its own; a server that comes back is usable again without the user doing anything")
	}
}

func TestRecordProfileFailureIgnoresRepeatStorms(t *testing.T) {
	s := isolatedStore(t)
	p := addFailureProfile(t, s, "flapping-node")

	// The same failure twice in a row is one event, not two: a single connect
	// attempt can report the same dead server many times, and each report used to
	// mean another rewrite of the whole store file.
	for i := 0; i < 5; i++ {
		if err := s.RecordProfileFailure(p.ID, "same reason"); err != nil {
			t.Fatalf("RecordProfileFailure #%d failed: %v", i, err)
		}
	}
	if got := s.GetProfiles()[0]; got.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures = %d, want 1 for a repeated identical reason inside the debounce window", got.ConsecutiveFailures)
	}

	// A different reason is a different event and must be counted.
	if err := s.RecordProfileFailure(p.ID, "tunnel verification failed: EOF"); err != nil {
		t.Fatalf("RecordProfileFailure with a new reason failed: %v", err)
	}
	if got := s.GetProfiles()[0]; got.ConsecutiveFailures != 2 {
		t.Errorf("ConsecutiveFailures = %d, want 2 after a distinct failure", got.ConsecutiveFailures)
	}
}

func TestRecordProfileFailureTruncatesReasonSafely(t *testing.T) {
	s := isolatedStore(t)
	p := addFailureProfile(t, s, "long-reason")

	if err := s.RecordProfileFailure(p.ID, strings.Repeat("x", profileFailureReasonMaxLen*3)); err != nil {
		t.Fatalf("RecordProfileFailure failed: %v", err)
	}
	got := s.GetProfiles()[0]
	if n := len([]rune(got.LastFailureReason)); n > profileFailureReasonMaxLen+1 {
		t.Errorf("reason is %d runes, want it bounded at %d so the store file stays readable", n, profileFailureReasonMaxLen)
	}

	// The reason is a core error string and can be Persian. Cutting at a byte
	// index would write a broken character into the store and into the report
	// file a user attaches to a bug report.
	persian := strings.Repeat("سرور در دسترس نیست ", 30)
	if err := s.RecordProfileFailure(p.ID, "متفاوت: "+persian); err != nil {
		t.Fatalf("recording a Persian reason failed: %v", err)
	}
	trimmed := s.GetProfiles()[0].LastFailureReason
	if !utf8.ValidString(trimmed) {
		t.Errorf("the truncated reason is not valid UTF-8: %q", trimmed)
	}
}

func TestProfileFailureBookkeepingNeedsARealProfile(t *testing.T) {
	s := isolatedStore(t)

	// An unknown id must be refused rather than quietly dropped: the engine's
	// verdict would otherwise vanish with no trace of why the server kept being
	// offered again.
	if err := s.RecordProfileFailure("never-existed", "reason"); err == nil {
		t.Errorf("recording a failure for an unknown profile must report an error rather than silently succeed")
	}
	if err := s.ClearProfileFailure("never-existed"); err == nil {
		t.Errorf("clearing an unknown profile must report an error too")
	}
}

func TestClearProfileFailureEndsTheCooldown(t *testing.T) {
	s := isolatedStore(t)
	p := addFailureProfile(t, s, "recovered-node")

	if err := s.RecordProfileFailure(p.ID, "unreachable"); err != nil {
		t.Fatalf("RecordProfileFailure failed: %v", err)
	}
	if err := s.ClearProfileFailure(p.ID); err != nil {
		t.Fatalf("ClearProfileFailure failed: %v", err)
	}

	got := s.GetProfiles()[0]
	if got.ConsecutiveFailures != 0 || !got.LastFailureAt.IsZero() || got.LastFailureReason != "" {
		t.Errorf("failure history survived a clear: %+v", got)
	}
	if ProfileIsCoolingDown(got, time.Now(), 0) {
		t.Errorf("a server that carried traffic must not stay on the cooldown list")
	}

	// Clearing twice must not rewrite the store file for no reason.
	if err := s.ClearProfileFailure(p.ID); err != nil {
		t.Errorf("a repeat clear returned %v, want nil", err)
	}
}

func TestProfileIsCoolingDownNeedsAFailure(t *testing.T) {
	now := time.Now()

	if ProfileIsCoolingDown(Profile{}, now, 0) {
		t.Errorf("a profile that never failed is not cooling down — an unmeasured subscription must stay usable")
	}
	// A timestamp without a count is an inconsistent record; it must not quarantine
	// the server forever.
	if ProfileIsCoolingDown(Profile{LastFailureAt: now, ConsecutiveFailures: 0}, now, 0) {
		t.Errorf("LastFailureAt alone must not cool a profile down")
	}
	// A failure that predates the window has expired: a server that came back must
	// be eligible again without the user touching anything.
	if ProfileIsCoolingDown(Profile{LastFailureAt: now.Add(-time.Minute), ConsecutiveFailures: 1}, now, 30*time.Second) {
		t.Errorf("a profile whose last failure predates the window must be eligible again")
	}
	if !ProfileIsCoolingDown(Profile{LastFailureAt: now, ConsecutiveFailures: 1}, now, 30*time.Second) {
		t.Errorf("a failure inside the window must keep the profile out of the way")
	}
}

func TestIsNexVPNAdapterName(t *testing.T) {
	// The network watcher's self-filter depends on this: a TUN adapter of ours that
	// slips through shows up as a phantom "network change" on every connect.
	yes := []string{"NexVPN-TUN", "NexVPN-TUN-3", "NexVPN-OVPN", "  NexVPN-TUN  "}
	no := []string{"Wi-Fi", "Ethernet", "TAP-Windows Adapter V9", "WARP", "", "MyNexVPN"}

	for _, n := range yes {
		if !IsNexVPNAdapterName(n) {
			t.Errorf("IsNexVPNAdapterName(%q) = false, want true", n)
		}
	}
	for _, n := range no {
		if IsNexVPNAdapterName(n) {
			t.Errorf("IsNexVPNAdapterName(%q) = true, want false: a third-party adapter is never ours", n)
		}
	}
}

func TestGetSettingsNormalisesTunSystemProxy(t *testing.T) {
	s := isolatedStore(t)

	// Unset means the historical behaviour: TUN adapter plus system proxy.
	if got := s.GetSettings(); got.TunSystemProxy != "dual" {
		t.Errorf("default TunSystemProxy = %q, want dual (changing the default would silently alter routing for every existing install)", got.TunSystemProxy)
	}

	if err := s.SaveSettings(Settings{TunSystemProxy: "tun_only"}); err != nil {
		t.Fatalf("SaveSettings failed: %v", err)
	}
	if got := s.GetSettings(); got.TunSystemProxy != "tun_only" {
		t.Errorf("TunSystemProxy = %q, want tun_only", got.TunSystemProxy)
	}

	// A hand-edited value the app does not know must not reach the engine.
	if err := s.SaveSettings(Settings{TunSystemProxy: "whatever"}); err != nil {
		t.Fatalf("SaveSettings failed: %v", err)
	}
	if got := s.GetSettings(); got.TunSystemProxy != "dual" {
		t.Errorf("an unrecognised TunSystemProxy = %q, want it normalised to dual", got.TunSystemProxy)
	}
}
