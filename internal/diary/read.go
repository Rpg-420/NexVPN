package diary

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// Entry is one parsed diary line.
//
// The diary is a diagnostic file first and a data source second, so parsing is
// deliberately forgiving: a line that cannot be understood is skipped rather
// than aborting the read. A truncated final line is the normal state of a file
// that is being appended to, and refusing to show the rest of it because of
// that would make the viewer useless exactly when it is needed.
type Entry struct {
	Time     time.Time         `json:"time"`
	Category string            `json:"category"`
	Event    string            `json:"event"`
	Fields   map[string]string `json:"fields"`
}

// Read returns up to `limit` of the most recent entries, oldest first.
//
// Oldest-first is the order the viewer wants: the diary tells a story
// (`connect/begin` then `connect/phase` then `connect/connected`), and reversing
// it would make every sequence read backwards. A limit of zero or less means
// "everything".
func Read(limit int) ([]Entry, error) {
	path := Path()
	if path == "" {
		return nil, os.ErrNotExist
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// A ring buffer rather than "read all then slice": the diary is capped at
	// 512 KB, but the cap is enforced on write, so a rotated or hand-edited file
	// can be larger, and there is no reason to hold all of it in memory to show
	// the last few hundred lines.
	var ring []Entry
	if limit > 0 {
		ring = make([]Entry, 0, limit)
	}

	scanner := bufio.NewScanner(file)
	// The default 64 KB line cap is far above anything the diary writes (fields
	// are flattened to a single line and are short), but a hand-edited file
	// should not silently truncate the read.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		entry, ok := parseLine(scanner.Text())
		if !ok {
			continue
		}
		if limit <= 0 {
			ring = append(ring, entry)
			continue
		}
		if len(ring) == limit {
			copy(ring, ring[1:])
			ring = ring[:limit-1]
		}
		ring = append(ring, entry)
	}

	// A scanner error on a file that is being appended to means "we read what
	// was there", not "the read failed" — return what was parsed.
	if err := scanner.Err(); err != nil && len(ring) == 0 {
		return nil, err
	}
	if ring == nil {
		ring = []Entry{}
	}
	return ring, nil
}

// parseLine decodes one TSV line: time, category, event, then key=value pairs.
func parseLine(line string) (Entry, bool) {
	line = strings.TrimRight(line, "\r")
	if strings.TrimSpace(line) == "" {
		return Entry{}, false
	}

	parts := strings.Split(line, "\t")
	if len(parts) < 3 {
		return Entry{}, false
	}

	entry := Entry{
		Category: parts[1],
		Event:    parts[2],
		Fields:   map[string]string{},
	}

	if ts, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
		entry.Time = ts
	}

	for _, pair := range parts[3:] {
		key, value, found := strings.Cut(pair, "=")
		if !found || key == "" {
			continue
		}
		entry.Fields[key] = value
	}

	return entry, true
}

// ParseDurationField reads a field written by phaseTimer.mark, which stores a
// Go duration string such as "4.123s". It returns 0 for anything unparseable,
// which the viewer renders as "unknown" rather than as an instant step.
func (e Entry) ParseDurationField(key string) time.Duration {
	raw, ok := e.Fields[key]
	if !ok || raw == "" || raw == "-" {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0
	}
	return d
}

// ParseIntField reads an integer field, returning 0 when it is absent or not a
// number.
func (e Entry) ParseIntField(key string) int {
	n, err := strconv.Atoi(e.Fields[key])
	if err != nil {
		return 0
	}
	return n
}
