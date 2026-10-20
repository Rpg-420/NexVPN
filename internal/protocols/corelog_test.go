package protocols

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The cores talk to a terminal, and the app is the only place their output is
// kept. These tests pin what the user actually sees in the log view and in the
// report file: no escape codes, no thousand-line storms, and no duplicate
// "stopped" lines for a process that stopped once.

func TestCoreLogMergerCapsARateOfDistinctCoreLines(t *testing.T) {
	// Merging only helps repeats. The WireGuard storm in the 2026-10-11 log was
	// different: one line per failing connection, each naming another domain, so
	// not one of them repeated the line before it. A rate cap is what keeps that
	// from pushing the useful lines out of the engine's 1000-entry buffer.
	var got []string
	m := newCoreLogMerger(time.Hour, func(level, text string) { got = append(got, level+": "+text) })

	for i := 0; i < 200; i++ {
		m.add("error", fmt.Sprintf("dns: lookup failed for host%d.example.com: context deadline exceeded", i))
	}
	m.Flush()

	if len(got) > coreLogBudgetLines+2 {
		t.Fatalf("200 distinct lines produced %d entries, want the budget of %d plus a notice", len(got), coreLogBudgetLines)
	}
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "suppressed") {
		t.Errorf("the suppressed total was never reported, so the log looks like the core went quiet:\n%s", joined)
	}
	if !strings.Contains(joined, "host0.example.com") {
		t.Errorf("the first lines of the storm were dropped instead of published: %q", got)
	}
	// Distinct domains must not be merged into one another — that would erase the
	// evidence of which names were failing. The budget is spent on the first few,
	// so several of them have to be visible.
	for _, want := range []string{"host0.", "host3.", "host12."} {
		if !strings.Contains(joined, want) {
			t.Errorf("the line for %q was never published; distinct failures should each be visible until the budget runs out:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "host150.") {
		t.Errorf("a line past the budget was published anyway; the storm is supposed to be capped")
	}
}

func TestStripCoreANSI(t *testing.T) {
	// A real sing-box line, colours and all.
	in := "\x1b[31mERROR\x1b[0m[0021] [\x1b[38;5;132m516280692\x1b[0m 10.0s] dns: lookup failed for www.facebook.com"
	want := "ERROR[0021] [516280692 10.0s] dns: lookup failed for www.facebook.com"

	got := stripCoreANSI(in)
	if got != want {
		t.Errorf("stripCoreANSI = %q, want %q", got, want)
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("escape sequences survived the strip: %q", got)
	}
	if plain := stripCoreANSI("plain line"); plain != "plain line" {
		t.Errorf("a line without escapes was altered: %q", plain)
	}
}

func TestLogDedupeKeyCollapsesPerConnectionNoise(t *testing.T) {
	a := logDedupeKey("error", "ERROR[0021] [516280692 10.0s] connection: open connection to www.google.com:443 using outbound/proxy: context deadline exceeded")
	b := logDedupeKey("error", "ERROR[0033] [99887766 3.5s] connection: open connection to www.google.com:443 using outbound/proxy: context deadline exceeded")
	if a != b {
		t.Errorf("two instances of the same failure produced different keys:\n  %s\n  %s", a, b)
	}

	// Different domains are different problems and must never be merged.
	c := logDedupeKey("error", "ERROR[0021] [516280692 10.0s] dns: lookup failed for www.facebook.com")
	d := logDedupeKey("error", "ERROR[0022] [12345678 10.0s] dns: lookup failed for mtalk.google.com")
	if c == d {
		t.Errorf("lookups for two different domains collapsed into one key")
	}
}

func TestCoreLogMergerMergesAStormIntoOneLinePlusCount(t *testing.T) {
	var got []string
	m := newCoreLogMerger(time.Hour, func(level, text string) {
		got = append(got, level+": "+text)
	})

	line := "dns: lookup failed for www.google.com: context deadline exceeded"
	for i := 0; i < 60; i++ {
		m.add("error", line)
	}
	if len(got) != 1 {
		t.Fatalf("60 identical lines produced %d entries, want 1 until the run is closed", len(got))
	}

	m.Flush()
	if len(got) != 2 {
		t.Fatalf("after Flush there are %d entries, want the original line plus one summary", len(got))
	}
	if !strings.Contains(got[1], "×60") {
		t.Errorf("summary does not report the count: %q", got[1])
	}
	if !strings.HasPrefix(got[1], "error: ") {
		t.Errorf("a merged error run must keep its severity, got %q", got[1])
	}
}

func TestCoreLogMergerFlushIsIdempotentAndSilentForSingletons(t *testing.T) {
	var got []string
	m := newCoreLogMerger(time.Hour, func(level, text string) { got = append(got, text) })

	m.add("info", "one lonely line")
	m.Flush()
	m.Flush()

	if len(got) != 1 {
		t.Fatalf("got %d entries, want exactly 1: a single line has nothing to merge and Flush must not repeat it", len(got))
	}
	if got[0] != "one lonely line" {
		t.Errorf("first line of a run was not published immediately: %q", got[0])
	}
}

func TestCoreLogMergerClosesARunWhenTheLineChanges(t *testing.T) {
	var got []string
	m := newCoreLogMerger(time.Hour, func(level, text string) { got = append(got, text) })

	for i := 0; i < 5; i++ {
		m.add("error", "dns: lookup failed for a.example.com")
	}
	m.add("error", "connection closed by peer")

	if len(got) != 3 {
		t.Fatalf("got %d entries, want the first storm, its summary, then the new line: %v", len(got), got)
	}
	if !strings.Contains(got[1], "×5") {
		t.Errorf("the count of the closed run was not published before the new line: %q", got[1])
	}
	if got[2] != "connection closed by peer" {
		t.Errorf("the new line was not emitted: %q", got[2])
	}
}

func TestCoreLogMergerExpiresWithoutNewLines(t *testing.T) {
	// A storm that never stops delivering the same line must still report that
	// it is ongoing, otherwise the log reads as "fixed itself".
	var got []string
	m := newCoreLogMerger(50*time.Millisecond, func(level, text string) { got = append(got, text) })

	m.add("error", "connection refused")
	for i := 0; i < 3; i++ {
		m.add("error", "connection refused")
		time.Sleep(30 * time.Millisecond)
	}
	time.Sleep(150 * time.Millisecond)

	if len(got) < 2 {
		t.Fatalf("the merger stayed silent after the window expired (got %d entries); a continuing problem must keep reporting", len(got))
	}
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "×") {
		t.Errorf("no merged count was ever published: %q", joined)
	}
}

func TestInjectCacheFileEnablesTheFakeIPStore(t *testing.T) {
	cfg := `{
  "log": {"level": "warn"},
  "experimental": {"clash_api": {"external_controller": "127.0.0.1:9090"}},
  "inbounds": [],
  "outbounds": [{"type": "direct", "tag": "direct"}]
}`

	out := injectCacheFile(cfg, filepath.Join(string(filepath.Separator), "tmp", "sing-box-cache.db"))
	if !strings.Contains(out, `"cache_file"`) {
		t.Fatalf("cache_file was not injected:\n%s", out)
	}
	if !strings.Contains(out, `"store_fakeip": true`) {
		t.Errorf("store_fakeip must be enabled, otherwise the fake-IP map dies with the process and the next session drops every cached 198.18.x.x connection")
	}
	if !strings.Contains(out, `"enabled": true`) {
		t.Errorf("cache_file.enabled must be true")
	}
	if !strings.Contains(out, "sing-box-cache.db") {
		t.Errorf("the cache path was not written into the config:\n%s", out)
	}
	// clash_api is what the byte counters poll; losing it would silently zero the
	// traffic graph.
	if !strings.Contains(out, "clash_api") {
		t.Errorf("existing experimental.clash_api was dropped")
	}
}

func TestInjectCacheFileRespectsAnExistingCacheAndBadInput(t *testing.T) {
	owned := `{"experimental": {"cache_file": {"enabled": false}}}`
	if out := injectCacheFile(owned, "/tmp/x.db"); !strings.Contains(out, `"enabled": false`) {
		t.Errorf("a config that already configures cache_file must not be overwritten:\n%s", out)
	}

	broken := `{"experimental": [`
	if out := injectCacheFile(broken, "/tmp/x.db"); out != broken {
		t.Errorf("unparseable config must be returned untouched, got %q", out)
	}

	if out := injectCacheFile(`{"dns":{}}`, ""); out != `{"dns":{}}` {
		t.Errorf("no usable cache path must leave the config untouched, got %q", out)
	}
}

// TestInjectedCacheFilePassesCoreCheck asks the bundled sing-box whether the
// schema is right. `experimental.cache_file` replaced several older spellings,
// and an unknown field is a FATAL at startup — which would take every TUN
// connection down rather than merely losing the cache.
func TestInjectedCacheFilePassesCoreCheck(t *testing.T) {
	sbBin := findTestSingBox(t)
	if sbBin == "" {
		t.Skip("sing-box binary not found, skipping core validation")
	}

	cfg := `{
  "log": {"level": "warn"},
  "dns": {
    "servers": [
      {"tag": "dns-remote", "type": "udp", "server": "1.1.1.1", "server_port": 53, "detour": "direct"},
      {"tag": "dns-fakeip", "type": "fakeip", "inet4_range": "198.18.0.0/15", "inet6_range": "fc00::/18"}
    ],
    "rules": [{"inbound": ["tun-in"], "query_type": ["A", "AAAA"], "server": "dns-fakeip"}],
    "final": "dns-remote",
    "strategy": "prefer_ipv4",
    "timeout": "3s"
  },
  "experimental": {"clash_api": {"external_controller": "127.0.0.1:9090"}},
  "inbounds": [{"type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": 10808}],
  "outbounds": [{"type": "direct", "tag": "direct"}],
  "route": {"rules": [{"action": "sniff"}], "final": "direct", "auto_detect_interface": true, "default_domain_resolver": "dns-remote"}
}`

	cacheDir := t.TempDir()
	injected := injectCacheFile(cfg, filepath.Join(cacheDir, "sing-box-cache.db"))

	tmp, err := os.CreateTemp("", "sb-cache-file-*.json")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(injected); err != nil {
		tmp.Close()
		t.Fatalf("writing temp config failed: %v", err)
	}
	tmp.Close()

	binDir, _ := filepath.Abs(filepath.Dir(sbBin))
	cmd := exec.Command(sbBin, "check", "-c", tmp.Name(), "-D", binDir)
	cmd.Env = append(os.Environ(),
		"ENABLE_DEPRECATED_MISSING_DOMAIN_RESOLVER=true",
		"ENABLE_DEPRECATED_LEGACY_DNS_SERVERS=true",
		"ENABLE_DEPRECATED_LEGACY_DNS_FAKEIP_OPTIONS=true",
		"ENABLE_DEPRECATED_FAKEIP=true",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sing-box rejected the config with the injected cache_file: %v\nOutput: %s\nConfig:\n%s",
			err, strings.TrimSpace(string(out)), injected)
	}
}

func findTestSingBox(t *testing.T) string {
	t.Helper()

	for _, c := range []string{
		filepath.Join("..", "..", "bin", "sing-box.exe"),
		filepath.Join("..", "..", "build", "bin", "bin", "sing-box.exe"),
		filepath.Join("..", "..", "bin", "sing-box"),
	} {
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if fi, statErr := os.Stat(abs); statErr == nil && !fi.IsDir() {
			return abs
		}
	}
	return ""
}

// There is deliberately no test that calls an adapter's Stop(): it clears the
// Windows system proxy registry keys, and a unit test has no business changing
// machine-wide state on the machine it runs on. The duplicated teardown lines are
// fixed by moving "Sing-box stopped" inside the `cmd != nil` guard in
// SingBoxAdapter.Stop and XrayAdapter.Stop, which keeps a repeat stop of an
// already-stopped core silent.

func TestSingBoxCachePathIsUsableOrEmpty(t *testing.T) {
	p := singBoxCachePath()
	if p == "" {
		// Legitimate on a machine with no writable per-user config dir; the
		// injection then leaves the config alone instead of pointing at a path the
		// core cannot open.
		t.Skip("no writable cache directory available")
	}
	if !filepath.IsAbs(p) {
		t.Errorf("cache path must be absolute, got %q", p)
	}
	if filepath.Base(p) != "sing-box-cache.db" {
		t.Errorf("unexpected cache file name: %q", p)
	}
	if dir := filepath.Dir(p); !strings.Contains(strings.ToLower(dir), "nexvpn") {
		t.Errorf("the cache must live under NexVPN's own directory, got %q", dir)
	}
}
