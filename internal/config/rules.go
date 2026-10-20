package config

import (
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Network rules (SSID based)
// ---------------------------------------------------------------------------

// NetworkRuleAction values.
const (
	// NetworkActionConnect brings the tunnel up when this network is joined.
	NetworkActionConnect = "connect"
	// NetworkActionDirect forces a disconnect (and suppresses auto-connect) so
	// that trusted networks are not tunnelled.
	NetworkActionDirect = "direct"
	// NetworkActionSkip means "leave whatever is running alone" — the rule
	// matches, so no automatic action is taken either way.
	NetworkActionSkip = "skip"
)

// NetworkRule ties an action to a Wi-Fi network name.
//
// It is deliberately keyed on SSID rather than on a BSSID or an adapter GUID:
// an SSID is what a user recognises, and it is stable across driver reinstall
// and USB adapter swaps, which the other two are not.
type NetworkRule struct {
	ID string `json:"id"`
	// Pattern is the SSID to match. Matching is case-insensitive and supports
	// `*` as a wildcard anywhere in the pattern, so `Home*` covers `Home`,
	// `Home-5G` and `Home_2.4`.
	Pattern string `json:"pattern"`
	Action  string `json:"action"`
	Enabled bool   `json:"enabled"`
}

// MatchesNetwork reports whether the rule applies to ssid.
func (r NetworkRule) MatchesNetwork(ssid string) bool {
	if !r.Enabled {
		return false
	}
	return WildcardMatch(strings.TrimSpace(r.Pattern), strings.TrimSpace(ssid))
}

// NormalizeNetworkRule trims, lowercases the action and fills in a missing ID.
// It returns an error for a rule that could never match or act.
func NormalizeNetworkRule(r NetworkRule) (NetworkRule, error) {
	r.ID = strings.TrimSpace(r.ID)
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	r.Pattern = strings.TrimSpace(r.Pattern)
	if r.Pattern == "" {
		return r, fmt.Errorf("network rule needs an SSID pattern")
	}
	if len(r.Pattern) > 64 {
		return r, fmt.Errorf("SSID pattern is longer than 64 characters")
	}
	r.Action = strings.ToLower(strings.TrimSpace(r.Action))
	switch r.Action {
	case NetworkActionConnect, NetworkActionDirect, NetworkActionSkip:
	default:
		return r, fmt.Errorf("unknown network rule action %q", r.Action)
	}
	return r, nil
}

// ---------------------------------------------------------------------------
// Routing rules (user defined)
// ---------------------------------------------------------------------------

// RoutingRuleKind values.
const (
	RoutingKindDomain = "domain"
	RoutingKindIP     = "ip"
	RoutingKindCIDR   = "cidr"
)

// RoutingRuleAction values.
const (
	RoutingActionProxy  = "proxy"
	RoutingActionDirect = "direct"
	RoutingActionBlock  = "block"
)

// RoutingRule is a user-defined override layered on top of the built-in
// RoutingMode.
//
// The built-in modes answer "how should traffic be split in general"; these
// answer "except for this". They are emitted into the core config *after* the
// built-in rules so that a user entry always wins, which is the only ordering
// that makes a manual override feel like an override.
type RoutingRule struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Value  string `json:"value"`
	Action string `json:"action"`
	// Enabled exists so a rule can be parked without being deleted — the
	// common case when debugging whether a rule is the cause of a problem.
	Enabled bool `json:"enabled"`
}

// NormalizeRoutingRule trims and validates a rule, filling in a missing ID.
func NormalizeRoutingRule(r RoutingRule) (RoutingRule, error) {
	r.ID = strings.TrimSpace(r.ID)
	if r.ID == "" {
		r.ID = uuid.NewString()
	}

	r.Kind = strings.ToLower(strings.TrimSpace(r.Kind))
	r.Action = strings.ToLower(strings.TrimSpace(r.Action))
	r.Value = strings.TrimSpace(r.Value)

	if r.Value == "" {
		return r, fmt.Errorf("routing rule needs a value")
	}

	switch r.Action {
	case RoutingActionProxy, RoutingActionDirect, RoutingActionBlock:
	default:
		return r, fmt.Errorf("unknown routing action %q", r.Action)
	}

	// A rule whose kind does not match its value is rejected rather than
	// coerced: silently turning a mistyped CIDR into a domain rule produces a
	// rule that never matches and no error, which is the worst outcome.
	switch r.Kind {
	case RoutingKindDomain:
		value := strings.ToLower(r.Value)
		value = strings.TrimPrefix(value, "*.")
		value = strings.TrimPrefix(value, ".")
		if strings.ContainsAny(value, "/ ") {
			return r, fmt.Errorf("%q is not a valid domain", r.Value)
		}
		if _, err := url.Parse("http://" + value); err != nil {
			return r, fmt.Errorf("%q is not a valid domain", r.Value)
		}
		r.Value = value
	case RoutingKindIP:
		ip := net.ParseIP(r.Value)
		if ip == nil {
			return r, fmt.Errorf("%q is not a valid IP address", r.Value)
		}
		r.Value = ip.String()
	case RoutingKindCIDR:
		_, network, err := net.ParseCIDR(r.Value)
		if err != nil {
			return r, fmt.Errorf("%q is not a valid CIDR block", r.Value)
		}
		r.Value = network.String()
	default:
		return r, fmt.Errorf("unknown routing rule kind %q", r.Kind)
	}

	return r, nil
}

// NormalizeRoutingRules normalises a whole list, dropping rules that fail
// validation instead of failing the whole save.
//
// Dropping is the right call here: these lists arrive from a JSON blob the
// frontend round-trips, and one bad entry must not make every other rule
// unsavable. The dropped ones are simply absent from the result, so the UI
// shows the user what actually survived.
func NormalizeRoutingRules(in []RoutingRule) []RoutingRule {
	out := make([]RoutingRule, 0, len(in))
	for _, r := range in {
		if n, err := NormalizeRoutingRule(r); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// NormalizeNetworkRules is NormalizeRoutingRules for network rules.
func NormalizeNetworkRules(in []NetworkRule) []NetworkRule {
	out := make([]NetworkRule, 0, len(in))
	for _, r := range in {
		if n, err := NormalizeNetworkRule(r); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Schedule rules
// ---------------------------------------------------------------------------

// ScheduleRuleAction values.
const (
	ScheduleActionConnect    = "connect"
	ScheduleActionDisconnect = "disconnect"
)

// ScheduleRule connects or disconnects at a wall-clock time on selected
// weekdays.
//
// Times are local wall-clock ("HH:MM") rather than UTC because a schedule is
// almost always written as "every morning at 9", and a DST shift moving the
// action by an hour is less surprising than the same text firing at a different
// local time.
type ScheduleRule struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	// Time is "HH:MM" in 24-hour local time.
	Time string `json:"time"`
	// Days holds weekday numbers, 0 = Sunday … 6 = Saturday. An empty list
	// means every day.
	Days      []int  `json:"days"`
	ProfileID string `json:"profile_id,omitempty"`
	Enabled   bool   `json:"enabled"`
	// LastRun records the "YYYY-MM-DDTHH:MM" slot that already fired, so a
	// ticker that runs more than once inside the same minute does not fire the
	// rule twice.
	LastRun string `json:"last_run,omitempty"`
}

// NormalizeScheduleRule validates a rule and fills in a missing ID.
func NormalizeScheduleRule(r ScheduleRule) (ScheduleRule, error) {
	r.ID = strings.TrimSpace(r.ID)
	if r.ID == "" {
		r.ID = uuid.NewString()
	}

	r.Action = strings.ToLower(strings.TrimSpace(r.Action))
	switch r.Action {
	case ScheduleActionConnect, ScheduleActionDisconnect:
	default:
		return r, fmt.Errorf("unknown schedule action %q", r.Action)
	}

	hour, minute, err := ParseClock(r.Time)
	if err != nil {
		return r, err
	}
	r.Time = fmt.Sprintf("%02d:%02d", hour, minute)

	// Deduplicate and sort the weekday list so that two rules written in a
	// different order compare equal, which keeps the "did this already fire"
	// bookkeeping stable across reloads.
	seen := make(map[int]struct{}, len(r.Days))
	days := make([]int, 0, len(r.Days))
	for _, d := range r.Days {
		if d < 0 || d > 6 {
			return r, fmt.Errorf("weekday %d is out of range 0-6", d)
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		days = append(days, d)
	}
	sort.Ints(days)
	r.Days = days

	return r, nil
}

// NormalizeScheduleRules is NormalizeRoutingRules for schedule rules.
func NormalizeScheduleRules(in []ScheduleRule) []ScheduleRule {
	out := make([]ScheduleRule, 0, len(in))
	for _, r := range in {
		if n, err := NormalizeScheduleRule(r); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// ParseClock parses "HH:MM" (and tolerates "H:MM" and "HH:MM:SS").
func ParseClock(value string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("%q is not a HH:MM time", value)
	}
	hour, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("%q is not a HH:MM time", value)
	}
	minute, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("%q is not a HH:MM time", value)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("%q is not a valid time of day", value)
	}
	return hour, minute, nil
}

// ShouldFireAt reports whether the rule is due at the given moment, and returns
// the slot key to record once it has fired.
//
// The slot key is minute-granular, which is what makes the caller idempotent:
// the ticker may run several times inside the same minute, but only the first
// run sees a slot key different from LastRun.
func (r ScheduleRule) ShouldFireAt(now time.Time) (bool, string) {
	if !r.Enabled {
		return false, ""
	}

	if len(r.Days) > 0 {
		weekday := int(now.Weekday())
		found := false
		for _, d := range r.Days {
			if d == weekday {
				found = true
				break
			}
		}
		if !found {
			return false, ""
		}
	}

	hour, minute, err := ParseClock(r.Time)
	if err != nil {
		return false, ""
	}
	if now.Hour() != hour || now.Minute() != minute {
		return false, ""
	}

	slot := now.Format("2006-01-02T15:04")
	if slot == r.LastRun {
		return false, ""
	}
	return true, slot
}

// WildcardMatch reports whether value matches pattern, where pattern may
// contain `*` (any run of characters) and `?` (exactly one character).
//
// It is case-insensitive because SSIDs are compared by eye far more often than
// by contract, and matching is implemented without regexp so that a pattern
// taken from user input cannot become a catastrophic-backtracking hazard.
func WildcardMatch(pattern, value string) bool {
	pattern = strings.ToLower(pattern)
	value = strings.ToLower(value)

	if pattern == "" {
		return value == ""
	}
	if !strings.ContainsAny(pattern, "*?") {
		return pattern == value
	}

	// Greedy segment match: split the pattern on `*` and require the segments
	// to appear in order. `?` is handled inside each segment.
	segments := strings.Split(pattern, "*")

	if !strings.HasPrefix(pattern, "*") {
		if !matchSegment(segments[0], value) {
			return false
		}
		value = value[len(segments[0]):]
		segments = segments[1:]
	}
	if len(segments) == 0 {
		return true
	}
	if !strings.HasSuffix(pattern, "*") {
		tail := segments[len(segments)-1]
		if !strings.HasSuffix(value, tail) || !matchSegment(tail, tail) {
			return false
		}
		value = value[:len(value)-len(tail)]
		segments = segments[:len(segments)-1]
	}

	for _, segment := range segments {
		if segment == "" {
			continue
		}
		idx := indexSegment(value, segment)
		if idx < 0 {
			return false
		}
		value = value[idx+len(segment):]
	}
	return true
}

// matchSegment matches a single `*`-free pattern segment of equal length.
func matchSegment(segment, value string) bool {
	if len(segment) != len(value) {
		return false
	}
	for i := 0; i < len(segment); i++ {
		if segment[i] == '?' {
			continue
		}
		if segment[i] != value[i] {
			return false
		}
	}
	return true
}

// indexSegment finds the first offset at which a `*`-free segment matches.
func indexSegment(value, segment string) int {
	if segment == "" {
		return 0
	}
	if !strings.Contains(segment, "?") {
		return strings.Index(value, segment)
	}
	for i := 0; i+len(segment) <= len(value); i++ {
		if matchSegment(segment, value[i:i+len(segment)]) {
			return i
		}
	}
	return -1
}
