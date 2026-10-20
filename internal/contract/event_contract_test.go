// Package contract holds tests that pin the agreements between the Go backend
// and the TypeScript frontend. Nothing here exercises behaviour; each test
// asserts that a convention two packages share by hand is still shared.
package contract

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// eventsEmittedButNotConsumed names backend events that the frontend is
// deliberately not subscribed to, with the reason.
//
// Every entry is a decision, not an oversight. The bug this test exists to
// prevent is the silent kind: `scanner:error`, `cf_scanner:error` and
// `frag_scanner:error` were emitted for months with no subscriber, so a scan
// that failed left the spinner running and the Stop button lit with nothing on
// screen to explain it. An event that nobody listens to must be either consumed
// or written down here.
var eventsEmittedButNotConsumed = map[string]string{
	"session:recorded":              "diagnostic stream; the session is already persisted to history.json by the same code path",
	"ping:batch:progress":           "superseded by the per-profile `profiles:ping_progress` stream, which the profile list renders",
	"batch_test:real_progress":      "superseded by `profiles:ping_progress`; kept for the external test harness",
	"profiles:ping_complete":        "terminal companion to `profiles:ping_progress`; the caller's promise already resolves when the sweep ends",
	"telegram:test_complete":        "terminal companion to `telegram:test_progress`; the page has no spinner that depends on it",
	"free_configs:test_complete":    "terminal companion to `free_configs:test_progress`; the page has no spinner that depends on it",
	"openvpn_sources:test_complete": "terminal companion to `openvpn_sources:test_progress`; the modal clears its spinner when testOpenVPNServers resolves",
}

// eventsEmitRe matches `runtime.EventsEmit(ctx, "name"` in Go source.
var eventsEmitRe = regexp.MustCompile(`EventsEmit\(\s*[^,]+,\s*"([^"]+)"`)

// eventsOnRe matches both `api.onEvent('name'` and the raw
// `runtime.EventsOn('name'` form.
//
// The raw form matters: grepping only for `onEvent` reports events as
// unconsumed when they are in fact consumed via `runtime.EventsOn`, which is a
// false positive this test must not repeat.
var eventsOnRe = regexp.MustCompile(`(?:onEvent|EventsOn)\(\s*['"]([^'"]+)['"]`)

// repoRoot walks up from this file until it finds go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test file")
	}
	dir := filepath.Dir(file)
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not find go.mod above the test file")
	return ""
}

// collectEmitters returns every event name emitted from Go source, mapped to the
// files that emit it.
func collectEmitters(t *testing.T, root string) map[string][]string {
	t.Helper()
	found := make(map[string][]string)

	skipDirs := map[string]bool{
		".git": true, ".workbuddy-ai": true, "node_modules": true,
		"build": true, "frontend": true, "docs": true, "tools": true, "dist": true,
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		for _, m := range eventsEmitRe.FindAllStringSubmatch(string(data), -1) {
			found[m[1]] = append(found[m[1]], filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return found
}

// collectSubscribers returns every event name the frontend listens for.
func collectSubscribers(t *testing.T, root string) map[string]bool {
	t.Helper()
	found := make(map[string]bool)
	src := filepath.Join(root, "frontend", "src")

	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".tsx") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		for _, m := range eventsOnRe.FindAllStringSubmatch(string(data), -1) {
			found[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", src, err)
	}
	return found
}

// TestEveryEmittedEventHasASubscriber is the contract: the backend must not
// announce anything the UI cannot hear.
func TestEveryEmittedEventHasASubscriber(t *testing.T) {
	root := repoRoot(t)
	emitters := collectEmitters(t, root)
	subscribers := collectSubscribers(t, root)

	if len(emitters) == 0 {
		t.Fatal("found no EventsEmit calls — the scan is broken, not the code")
	}
	if len(subscribers) == 0 {
		t.Fatal("found no frontend event subscriptions — the scan is broken, not the code")
	}

	var orphans []string
	for name := range emitters {
		if subscribers[name] {
			continue
		}
		if _, ok := eventsEmittedButNotConsumed[name]; ok {
			continue
		}
		orphans = append(orphans, name)
	}
	sort.Strings(orphans)

	if len(orphans) > 0 {
		for _, name := range orphans {
			t.Errorf("event %q is emitted by %s but nothing in frontend/src subscribes to it; "+
				"either subscribe to it or add it to eventsEmittedButNotConsumed with a reason",
				name, strings.Join(emitters[name], ", "))
		}
	}
}

// TestAllowListHasNoStaleEntries keeps the exception list honest: an entry for an
// event that no longer exists, or that is now subscribed to, is dead weight that
// hides the next real gap.
func TestAllowListHasNoStaleEntries(t *testing.T) {
	root := repoRoot(t)
	emitters := collectEmitters(t, root)
	subscribers := collectSubscribers(t, root)

	for name := range eventsEmittedButNotConsumed {
		if _, emitted := emitters[name]; !emitted {
			t.Errorf("eventsEmittedButNotConsumed lists %q, but nothing emits it any more — remove the entry", name)
			continue
		}
		if subscribers[name] {
			t.Errorf("eventsEmittedButNotConsumed lists %q, but the frontend now subscribes to it — remove the entry", name)
		}
	}
}

// TestEventNamesAreNamespaced asserts the `namespace:event` shape. The prefix is
// what makes an event traceable back to the file that emits it.
func TestEventNamesAreNamespaced(t *testing.T) {
	root := repoRoot(t)
	for name := range collectEmitters(t, root) {
		if !strings.Contains(name, ":") {
			t.Errorf("event %q has no `namespace:` prefix", name)
		}
	}
}
