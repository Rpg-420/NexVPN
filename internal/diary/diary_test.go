package diary

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTempDiary resets the package-level state and opens a diary in a fresh
// directory. The diary is a singleton by design, so tests must not overlap.
func newTempDiary(t *testing.T) string {
	t.Helper()

	Close() // drop any state left by a previous test

	dir := t.TempDir()
	if err := Init(dir, "9.9.9-test"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(Close)

	return dir
}

func readLines(t *testing.T, path string) []string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	text := strings.TrimRight(string(raw), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func findLine(t *testing.T, lines []string, category, event string) string {
	t.Helper()

	for _, l := range lines {
		fields := strings.Split(l, "\t")
		if len(fields) >= 3 && fields[1] == category && fields[2] == event {
			return l
		}
	}
	t.Fatalf("no %s/%s line in:\n%s", category, event, strings.Join(lines, "\n"))
	return ""
}

func TestInitWritesLaunchBoundaries(t *testing.T) {
	dir := newTempDiary(t)
	Log("connect", "begin", "profile", "Frankfurt", "mode", "tun")
	Close()

	lines := readLines(t, filepath.Join(dir, FileName))
	if len(lines) != 3 {
		t.Fatalf("want launch/begin, connect/begin and launch/end, got %d:\n%s",
			len(lines), strings.Join(lines, "\n"))
	}

	begin := findLine(t, lines, "launch", "begin")
	if !strings.Contains(begin, "version=9.9.9-test") {
		t.Errorf("begin line must record the version, got %q", begin)
	}
	if !strings.Contains(begin, "pid=") {
		t.Errorf("begin line must record the pid, got %q", begin)
	}

	end := findLine(t, lines, "launch", "end")
	if !strings.Contains(end, "uptime=") {
		t.Errorf("end line must record the uptime, got %q", end)
	}

	// The launch header must be the first line, so the report tool can find the
	// most recent launch by scanning backwards for it.
	if strings.Split(lines[0], "\t")[2] != "begin" {
		t.Errorf("the launch header must come first, got %q", lines[0])
	}
}

// The report tool splits on tabs and reads field 0 as the timestamp, 1 as the
// category and 2 as the event, then treats everything after as key=value. This
// pins that contract so a formatting change cannot silently break the tool.
func TestLogEmitsTsvContract(t *testing.T) {
	dir := newTempDiary(t)
	Log("connect", "engine-select", "core", "sing-box", "protocol", "vmess")
	Close()

	line := findLine(t, readLines(t, filepath.Join(dir, FileName)), "connect", "engine-select")
	fields := strings.Split(line, "\t")

	if len(fields) != 5 {
		t.Fatalf("want 5 tab-separated fields, got %d: %q", len(fields), line)
	}
	if !strings.Contains(fields[0], "T") {
		t.Errorf("field 0 should be an RFC3339 timestamp, got %q", fields[0])
	}
	if fields[3] != "core=sing-box" {
		t.Errorf("field 3 = %q, want core=sing-box", fields[3])
	}
	if fields[4] != "protocol=vmess" {
		t.Errorf("field 4 = %q, want protocol=vmess", fields[4])
	}
}

// An error message with embedded newlines or tabs would otherwise split one
// event across several lines and desynchronise the report tool's parser.
func TestLogFlattensMultilineValues(t *testing.T) {
	dir := newTempDiary(t)
	LogErr("connect", "error", errors.New("first line\nsecond\tline"))
	Close()

	line := findLine(t, readLines(t, filepath.Join(dir, FileName)), "connect", "error")
	if got := strings.Count(line, "\t"); got != 3 {
		t.Fatalf("flattening failed, line has %d tabs: %q", got, line)
	}
	if !strings.Contains(line, "err=first line second line") {
		t.Errorf("newline and tab should both become spaces, got %q", line)
	}
}

func TestLogErrWithNilErrorStillRecordsTheStage(t *testing.T) {
	dir := newTempDiary(t)
	LogErr("connect", "verify-ok", nil)
	Close()

	line := findLine(t, readLines(t, filepath.Join(dir, FileName)), "connect", "verify-ok")
	if strings.Contains(line, "err=") {
		t.Errorf("a nil error must not produce an err= field, got %q", line)
	}
}

func TestEmptyFieldsBecomePlaceholders(t *testing.T) {
	dir := newTempDiary(t)
	Log("connect", "spawn", "core", "")
	Close()

	line := findLine(t, readLines(t, filepath.Join(dir, FileName)), "connect", "spawn")
	if !strings.HasSuffix(line, "\tcore=-") {
		t.Errorf("empty value should become \"-\" so the field count stays stable, got %q", line)
	}
}

func TestOddTrailingArgumentIsIgnored(t *testing.T) {
	dir := newTempDiary(t)
	Log("connect", "begin", "profile", "Frankfurt", "dangling")
	Close()

	line := findLine(t, readLines(t, filepath.Join(dir, FileName)), "connect", "begin")
	if strings.Contains(line, "dangling") {
		t.Errorf("a key with no value must be dropped, got %q", line)
	}
	// Three fixed fields (timestamp, category, event) plus one pair: 4 fields,
	// so 3 tabs.
	if got := strings.Count(line, "\t"); got != 3 {
		t.Errorf("want 3 tabs (3 fixed fields + 1 pair), got %d: %q", got, line)
	}
}

func TestRotateKeepsThePreviousLaunch(t *testing.T) {
	Close()
	dir := t.TempDir()

	oversized := strings.Repeat("x", maxBytes+1)
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(oversized), 0600); err != nil {
		t.Fatalf("seed oversized diary: %v", err)
	}

	if err := Init(dir, "1.0.0"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	Close()

	if _, err := os.Stat(filepath.Join(dir, prevName)); err != nil {
		t.Fatalf("previous launch was not preserved: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("stat active diary: %v", err)
	}
	if info.Size() >= int64(len(oversized)) {
		t.Errorf("active diary was not rotated: %d bytes", info.Size())
	}
}

func TestSmallDiaryIsNotRotated(t *testing.T) {
	dir := newTempDiary(t)
	Log("connect", "begin")
	Close()

	if _, err := os.Stat(filepath.Join(dir, prevName)); err == nil {
		t.Errorf("a small diary must not be rotated")
	}
}

// The diary is opened from main() before anything else, but a code path that
// logs before Init must not panic.
func TestLogBeforeInitIsSafe(t *testing.T) {
	Close()

	Log("connect", "begin")
	LogErr("connect", "error", errors.New("boom"))
	LogKV("connect", "spawn", map[string]string{"core": "sing-box"})

	if Path() != "" {
		t.Errorf("Path() should be empty when no diary is open, got %q", Path())
	}
}

func TestInitIsIdempotent(t *testing.T) {
	dir := newTempDiary(t)

	// A second Init must not truncate or re-open the file.
	if err := Init(dir, "9.9.9-test"); err != nil {
		t.Fatalf("second Init: %v", err)
	}
	Log("connect", "begin")
	Close()

	lines := readLines(t, filepath.Join(dir, FileName))
	if len(lines) != 3 {
		t.Fatalf("second Init added an extra launch header: got %d lines:\n%s",
			len(lines), strings.Join(lines, "\n"))
	}
	if got := strings.Count(strings.Join(lines, "\n"), "launch\tbegin"); got != 1 {
		t.Errorf("want exactly one launch header, got %d", got)
	}
}

func TestInitWithEmptyDirFails(t *testing.T) {
	Close()
	if err := Init("", "1.0.0"); err == nil {
		t.Fatalf("Init with an empty directory should fail")
	}
	// The failed Init must leave the diary disabled rather than half-open.
	Log("connect", "begin")
	if Path() != "" {
		t.Errorf("Path() should be empty after a failed Init, got %q", Path())
	}
}
