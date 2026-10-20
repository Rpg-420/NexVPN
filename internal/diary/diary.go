// Package diary writes the on-disk record of how far each NexVPN launch got.
//
// # Why this exists
//
// When NexVPN fails to start, or starts and then dies ten seconds later, the
// only record of what happened used to be the in-memory log ring buffer in
// internal/core — which dies with the process. Diagnosing the TUN crash-loop
// incident meant the user had to hand-copy a four-minute log out of the UI, and
// the most important evidence (the core dying ~15s after launch, long after the
// app had already announced "connected") was only visible because the window
// happened to stay open long enough to read it.
//
// This package writes to disk as the launch progresses, so the record survives
// a crash, a hang before the window is ever drawn, and a hard kill. It is
// deliberately dumb: plain TSV, one line per event, readable in Notepad and
// parseable by PowerShell without any JSON or regex work. The companion tool
// tools/NexVPN-Report.ps1 reads it when the app cannot run at all.
//
// It lives in its own package rather than inside internal/core because
// internal/core imports internal/protocols, so the protocols package cannot
// import core back. The single most valuable line this file can contain — the
// moment a TUN adapter comes up, or fails to — is produced in
// internal/protocols.
//
// Format
//
//	<RFC3339 timestamp> \t <category> \t <event> \t <k=v> <k=v> ...
//
// A line with category "launch" and event "begin" opens a new launch, so the
// report tool can isolate the most recent attempt by scanning backwards for the
// last such line. Everything after it belongs to that launch.
//
// # Failure policy
//
// The diary never returns an error to its callers and never blocks a launch.
// A diagnostic that can itself break startup would be worse than no diagnostic
// at all, so every write failure is swallowed and the diary disables itself.
package diary

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// FileName is the active diary, written inside the NexVPN data directory.
	FileName = "startup.log"
	// prevName holds the previous launch once the active file is rotated.
	prevName = "startup.prev.log"
	// maxBytes caps the active file. One launch is typically a few KB, so this
	// keeps roughly the last few hundred launches — far more than anyone needs,
	// while staying small enough to attach to a bug report.
	maxBytes = 512 * 1024
)

var (
	mu       sync.Mutex
	file     *os.File
	path     string
	disabled bool
	started  time.Time
)

// Init opens (or creates) the diary in dir and writes the opening line for this
// launch. version is recorded so a report can be matched against the release
// that produced it.
//
// It is safe to call more than once; the second call is ignored.
func Init(dir string, version string) error {
	mu.Lock()
	defer mu.Unlock()

	if file != nil {
		return nil
	}

	if dir == "" {
		disabled = true
		return fmt.Errorf("diary: empty data directory")
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		disabled = true
		return err
	}

	p := filepath.Join(dir, FileName)
	rotate(p)

	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		disabled = true
		return err
	}

	file = f
	path = p
	started = time.Now()
	disabled = false

	exePath, _ := os.Executable()
	hostname, _ := os.Hostname()

	writeLocked("launch", "begin",
		"version", version,
		"pid", fmt.Sprintf("%d", os.Getpid()),
		"exe", exePath,
		"host", hostname,
		"args", strings.Join(os.Args[1:], " "),
	)

	return nil
}

// Close flushes and closes the diary. It is safe to call when the diary was
// never opened, and safe to call more than once.
func Close() {
	mu.Lock()
	defer mu.Unlock()

	if file == nil {
		return
	}

	writeLocked("launch", "end", "uptime", time.Since(started).Round(time.Millisecond).String())

	_ = file.Sync()
	_ = file.Close()
	file = nil
	// Clear the path as well: a caller that asks Path() after Close() must get
	// "no diary", not the location of a file that is no longer being written.
	path = ""
}

// Path returns the active diary path, or "" when the diary is not open.
func Path() string {
	mu.Lock()
	defer mu.Unlock()
	return path
}

// Log records one stage of a launch. kv is an even-length list of key/value
// pairs; an odd trailing argument is ignored rather than corrupting the line.
//
// Call it at every point where the launch could plausibly stop, so that the
// last line in the file is always an answer to "what was it doing when it died?".
func Log(category string, event string, kv ...string) {
	mu.Lock()
	defer mu.Unlock()
	writeLocked(category, event, kv...)
}

// LogErr records a failure stage. A nil err is recorded as a plain stage rather
// than dropped, so the timeline stays contiguous.
func LogErr(category string, event string, err error) {
	mu.Lock()
	defer mu.Unlock()

	if err == nil {
		writeLocked(category, event)
		return
	}
	writeLocked(category, event, "err", err.Error())
}

// LogKV is the map-based form, for call sites that already hold a map.
func LogKV(category string, event string, kv map[string]string) {
	mu.Lock()
	defer mu.Unlock()

	pairs := make([]string, 0, len(kv)*2)
	for k, v := range kv {
		pairs = append(pairs, k, v)
	}
	writeLocked(category, event, pairs...)
}

func writeLocked(category string, event string, kv ...string) {
	if disabled || file == nil {
		return
	}

	var b strings.Builder
	b.Grow(128)
	b.WriteString(time.Now().Format(time.RFC3339Nano))
	b.WriteByte('\t')
	b.WriteString(sanitize(category))
	b.WriteByte('\t')
	b.WriteString(sanitize(event))

	for i := 0; i+1 < len(kv); i += 2 {
		b.WriteByte('\t')
		b.WriteString(sanitize(kv[i]))
		b.WriteByte('=')
		b.WriteString(sanitize(kv[i+1]))
	}
	b.WriteByte('\n')

	if _, err := file.WriteString(b.String()); err != nil {
		// A broken diary must not take the app down with it.
		disabled = true
		_ = file.Close()
		file = nil
	}
}

// sanitize keeps every field on one TSV line. Newlines and tabs would both break
// the one-event-per-line contract the report tool relies on.
func sanitize(s string) string {
	if s == "" {
		return "-"
	}
	if !strings.ContainsAny(s, "\t\r\n") {
		return s
	}
	return strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(s)
}

// rotate keeps the previous launch when the active file has grown past the cap.
// One generation is enough: the file that matters when something breaks is
// always the most recent one.
func rotate(p string) {
	info, err := os.Stat(p)
	if err != nil || info.Size() < maxBytes {
		return
	}

	prev := filepath.Join(filepath.Dir(p), prevName)
	_ = os.Remove(prev)
	_ = os.Rename(p, prev)
}
