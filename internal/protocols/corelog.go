package protocols

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Core processes write their diagnostics to a pipe, which makes the app the only
// log sink they have — and every line arrives as text formatted for a terminal.
// Two things have to happen before that text is fit to store: the ANSI colour
// codes have to go, and the repeats have to be counted instead of appended one
// by one.
//
// The second is not cosmetic. The engine keeps the last 1000 log lines, and a
// dead server makes sing-box print the same error for every connection: one real
// 2026-10-11 session produced ~670 lines in 24 minutes, 85% of them repeats, and
// the storm pushed the Smart-Core decisions and the verification results — the
// only lines that explain a failure — out of the buffer entirely.

// ansiSequence matches the colour/attribute escapes the cores emit, including
// the 256-colour form (`\x1b[38;5;132m`) and the bare reset (`\x1b[0m`).
var ansiSequence = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// longDigitRun folds the per-connection numbers that make every instance of the
// same failure look like a different message: sing-box stamps each line with a
// connection ID (nine digits) and an elapsed-seconds counter ([0021]). Runs of
// four digits or more are always one of those. Shorter runs are left alone on
// purpose, so a port or an IPv6 suffix that identifies *which* endpoint failed
// stays visible instead of being merged into a generic "#".
var longDigitRun = regexp.MustCompile(`\d{4,}`)

// durationValue folds elapsed times (`10.0s`, `350ms`), which differ on every
// line of a storm while meaning the same thing.
var durationValue = regexp.MustCompile(`\d+(\.\d+)?(?:ns|us|ms|s|m|h)\b`)

// stripCoreANSI removes terminal colour codes from a core log line.
//
// The escapes are more than cosmetic damage: they land in the log view, in the
// diary and in the report file a user attaches to a bug report, where they make
// every core line unreadable and inflate its size.
func stripCoreANSI(text string) string {
	if !strings.Contains(text, "\x1b") {
		return text
	}
	return ansiSequence.ReplaceAllString(text, "")
}

// coreLogLevel maps a core line to one of the app's log levels.
//
// sing-box and xray prefix every line with its level, and the app derives the
// entry's severity from that prefix, so a fatal config error and an
// informational startup line have to be told apart before anything is stored.
func coreLogLevel(text, defaultLevel string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "error"),
		strings.Contains(lower, "fatal"),
		strings.Contains(lower, "panic"):
		return "error"
	case strings.Contains(lower, "warn"):
		return "warning"
	case strings.Contains(lower, "info"):
		return "info"
	}
	return defaultLevel
}

// logDedupeKey is the identity a repeated line is matched on.
func logDedupeKey(level, text string) string {
	normalised := durationValue.ReplaceAllString(stripCoreANSI(text), "#")
	normalised = longDigitRun.ReplaceAllString(normalised, "#")
	return level + "|" + normalised
}

const (
	// coreLogMergeWindow is how long a run of identical lines is absorbed into a
	// single entry. Long enough to swallow an error storm, short enough that a
	// problem which is still happening keeps reporting.
	coreLogMergeWindow = 10 * time.Second

	// coreLogBudgetLines / coreLogBudgetWindow bound the *rate* of distinct core
	// lines. Merging only helps repeats, and a real DNS storm interleaves dozens
	// of different domains — one per failing connection — none of which is a
	// repeat of the previous line. Above the budget the lines are counted and
	// reported as a total, because a log that is 90% noise hides the 10% that
	// matters just as well as one that overflows the buffer.
	coreLogBudgetLines  = 25
	coreLogBudgetWindow = 5 * time.Second

	// suppressedNoticeLevel is the level of the "N more lines" notices. They are
	// bookkeeping, not a fault, and raising them to the level of what they follow
	// would put a summary at the top of an error filter.
	suppressedNoticeLevel = "info"
)

// coreLogMerger collapses runs of identical core log lines into one entry plus a
// count, and caps the rate at which distinct lines are published.
//
// Contract: the first line of a run is always published immediately, so nothing
// is hidden; identical lines arriving inside the window are counted; and when the
// run breaks — a different line, the window expiring, or the core stopping — one
// summary line reports how many repeats there were. A continuous storm therefore
// costs roughly one entry per window instead of one per connection, and the
// summary is the difference between "still failing" and "went quiet because it
// fixed itself".
type coreLogMerger struct {
	mu       sync.Mutex
	window   time.Duration
	budget   int
	cadence  time.Duration
	emit     func(level, text string)
	pending  *mergedLogLine
	flushGen int64 // invalidates timers of runs that already ended

	budgetStart time.Time
	emitted     int
	suppressed  int
}

type mergedLogLine struct {
	key       string
	level     string
	text      string
	count     int
	startedAt time.Time
}

func newCoreLogMerger(window time.Duration, emit func(level, text string)) *coreLogMerger {
	if window <= 0 {
		window = coreLogMergeWindow
	}
	return &coreLogMerger{
		window:      window,
		budget:      coreLogBudgetLines,
		cadence:     coreLogBudgetWindow,
		emit:        emit,
		budgetStart: time.Now(),
	}
}

// add records one core log line.
func (m *coreLogMerger) add(level, text string) {
	if m == nil || m.emit == nil {
		return
	}

	key := logDedupeKey(level, text)
	now := time.Now()

	m.mu.Lock()
	if p := m.pending; p != nil && p.key == key && now.Sub(p.startedAt) < m.window {
		p.count++
		m.mu.Unlock()
		return
	}

	// A different line ends the previous run, so its count has to be published
	// first — otherwise the log shows one instance of an error that happened
	// sixty times and silently drops the other fifty-nine.
	runLevel, summary := m.closePendingLocked()

	// Rate limit, measured in published entries rather than in incoming lines.
	// Nothing is lost: the suppressed lines are counted and reported as a total.
	if m.emitted >= m.budget && now.Sub(m.budgetStart) < m.cadence {
		m.suppressed++
		m.mu.Unlock()
		if summary != "" {
			m.emit(runLevel, summary)
		}
		return
	}

	notice := ""
	if m.suppressed > 0 {
		notice = fmt.Sprintf("(%d further core lines suppressed over the last %v)", m.suppressed, m.cadence)
		m.suppressed = 0
	}
	if now.Sub(m.budgetStart) >= m.cadence {
		m.budgetStart = now
		m.emitted = 0
	}

	m.pending = &mergedLogLine{key: key, level: level, text: text, count: 1, startedAt: now}
	m.flushGen++
	gen := m.flushGen
	m.emitted++
	if summary != "" {
		m.emitted++
	}
	if notice != "" {
		m.emitted++
	}
	m.mu.Unlock()

	if summary != "" {
		m.emit(runLevel, summary)
	}
	if notice != "" {
		m.emit(suppressedNoticeLevel, notice)
	}
	// The first line of a run always goes out, so a new problem is never delayed.
	m.emit(level, text)

	// A window also closes on a timer, because an error storm delivers nothing
	// but identical lines and would otherwise be absorbed in silence until the
	// core stopped.
	time.AfterFunc(m.window, func() { m.expire(gen) })
}

// expire closes the run that gen identifies, if it is still the open one.
func (m *coreLogMerger) expire(gen int64) {
	if m == nil || m.emit == nil {
		return
	}

	m.mu.Lock()
	if m.flushGen != gen {
		// Something newer owns the merger now; its own timer will report it.
		m.mu.Unlock()
		return
	}
	runLevel, summary := m.closePendingLocked()
	notice := ""
	if m.suppressed > 0 {
		notice = fmt.Sprintf("(%d further core lines suppressed over the last %v)", m.suppressed, m.cadence)
		m.suppressed = 0
	}
	m.mu.Unlock()

	if summary != "" {
		m.emit(runLevel, summary)
	}
	if notice != "" {
		m.emit(suppressedNoticeLevel, notice)
	}
}

// Flush publishes whatever a run has counted so far. Called on process teardown
// so a report never ends mid-sentence.
func (m *coreLogMerger) Flush() {
	if m == nil || m.emit == nil {
		return
	}

	m.mu.Lock()
	runLevel, summary := m.closePendingLocked()
	notice := ""
	if m.suppressed > 0 {
		notice = fmt.Sprintf("(%d further core lines suppressed)", m.suppressed)
		m.suppressed = 0
	}
	m.mu.Unlock()

	if summary != "" {
		m.emit(runLevel, summary)
	}
	if notice != "" {
		m.emit(suppressedNoticeLevel, notice)
	}
}

// closePendingLocked clears the open run and returns the level and text of the
// summary to emit, or an empty text when nothing was merged. Emitting happens
// outside the lock: the sink is the engine's log function, which takes its own
// mutex, and holding this one across that is how a core that logs from its own
// teardown path would deadlock.
//
// The summary keeps the level of the run it describes: collapsing sixty error
// lines into an "info" notice would hide the severity from the log filter.
func (m *coreLogMerger) closePendingLocked() (string, string) {
	p := m.pending
	m.pending = nil
	if p == nil {
		return suppressedNoticeLevel, ""
	}
	if p.count <= 1 {
		return p.level, ""
	}
	return p.level, fmt.Sprintf("%s (×%d — %d identical lines merged over %v)",
		p.text, p.count, p.count-1, m.window.Truncate(time.Second))
}
