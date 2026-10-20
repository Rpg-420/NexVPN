package core

import "testing"

// TestContainsUnsafeShellChars is the regression guard for the Kill Switch
// injection-check bug: the original implementation tested against the Go
// literal ";&|$`\\n\\r\\t", in which `\\n` denotes a backslash followed by the
// letter 'n'. The effective reject-set was therefore {; & | $ ` \ n r t}, which
// wrongly rejected any argument containing the letters n, r or t — e.g.
// "--reject-with icmp-net-prohibited" and "-o tun+" — and failed to reject real
// newlines/tabs. These cases lock the corrected behaviour in place.
func TestContainsUnsafeShellChars(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"plain program", "iptables", false},
		{"letters n r t are safe", "nrt", false},
		{"reject-with icmp-net-prohibited", "--reject-with icmp-net-prohibited", false},
		{"tun device", "-o tun+", false},
		{"dash args", "-A", false},
		{"semicolon", "a;b", true},
		{"ampersand", "a&b", true},
		{"pipe", "a|b", true},
		{"dollar", "a$b", true},
		{"backtick", "a`b", true},
		{"backslash", "a\\b", true},
		{"double quote", "a\"b", true},
		{"single quote", "a'b", true},
		{"newline", "a\nb", true},
		{"carriage return", "a\rb", true},
		{"tab", "a\tb", true},
		{"del char", "a\x7fb", true},
	}
	for _, c := range cases {
		if got := containsUnsafeShellChars(c.in); got != c.want {
			t.Errorf("%s: containsUnsafeShellChars(%q) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
}

// TestResolveAllIPs covers the Kill Switch server-allowlist resolver. Only the
// deterministic, network-free branches are asserted here (empty input and
// literal IP passthrough); hostname resolution depends on DNS and is exercised
// indirectly in integration.
func TestResolveAllIPs(t *testing.T) {
	if got := resolveAllIPs(""); got != nil {
		t.Errorf("resolveAllIPs(\"\") = %v, want nil", got)
	}

	for _, ip := range []string{"1.2.3.4", "203.0.113.7", "::1", "2001:db8::1"} {
		got := resolveAllIPs(ip)
		if len(got) != 1 || got[0] != ip {
			t.Errorf("resolveAllIPs(%q) = %v, want [%q]", ip, got, ip)
		}
	}
}

// TestKillSwitchManagerInitialState documents the observable state contract of
// the manager WITHOUT touching the OS firewall: a fresh manager must report
// inactive.
//
// NOTE: Enable()/Disable() are deliberately NOT called here — they mutate the
// real Windows firewall policy and would be destructive on a developer or CI
// machine. Their command construction is covered indirectly by
// TestContainsUnsafeShellChars (the argument validator both paths rely on).
func TestKillSwitchManagerInitialState(t *testing.T) {
	k := &KillSwitchManager{}
	if k.IsActive() {
		t.Fatal("a fresh KillSwitchManager must report inactive")
	}
	// A fresh manager's guard is not enabled, so a second Enable() would not
	// early-return on a stale `active` flag.
	k.mu.Lock()
	active := k.active
	k.mu.Unlock()
	if active {
		t.Fatal("internal active flag must start false")
	}
}
