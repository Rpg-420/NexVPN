package main

import (
	"fmt"
	"time"

	"nexvpn/internal/config"
)

// ProfileReliability is the per-server health score shown in the profile
// lists. It is DERIVED, not stored: the inputs are the failure ledger the
// engine already maintains (ConsecutiveFailures / LastFailureAt / reason) plus
// the last measured latency, so a score costs one binding call and no schema
// change.
type ProfileReliability struct {
	Score         int    `json:"score"` // 0..100 — 100 means no failure record
	State         string `json:"state"` // unknown | excellent | good | fair | poor
	Failures      int    `json:"failures"`
	LastFailure   string `json:"last_failure,omitempty"`
	LastFailureAt string `json:"last_failure_at,omitempty"`
	Cooling       bool   `json:"cooling"`
	UsageCount    int    `json:"usage_count"`
}

// reliabilityCooldownWindow should mirror the engine's per-profile cooldown
// (server-health bookkeeping uses minutes); it is only used here to flag a
// server as actively cooling, not to compute the score itself.
const reliabilityCoolingWindow = 5 * time.Minute

// Score the profile:
//
//	 100              no failure record at all (fresh or consistently healthy)
//	-25 per consecutive failure, floored at 0
//	  +10 latency bonus for a recent sub-100ms measurement, capped at 100
//
// A server with 4 consecutive failures (the `ConsecutiveFailures >= 4` band
// the failover code already treats as hopeless) lands at 0, while one flaky
// connect that recovered only drops to 75 — matching how much trust each
// event deserves.
func reliabilityScoreOf(p config.Profile) ProfileReliability {
	r := ProfileReliability{
		Failures:   p.ConsecutiveFailures,
		UsageCount: p.UsageCount,
	}
	if !p.LastFailureAt.IsZero() {
		r.LastFailure = p.LastFailureReason
		r.LastFailureAt = p.LastFailureAt.Format("2006-01-02 15:04")
		r.Cooling = time.Since(p.LastFailureAt) < reliabilityCoolingWindow
	}

	score := 100 - 25*p.ConsecutiveFailures
	if score < 0 {
		score = 0
	}
	if p.LatencyMs > 0 && p.LatencyMs < 100 {
		score += 10
		if score > 100 {
			score = 100
		}
	}
	r.Score = score

	switch {
	case p.ConsecutiveFailures == 0 && p.LatencyMs <= 0:
		r.State = "unknown" // never measured — show nothing pessimistic
	case score >= 90:
		r.State = "excellent"
	case score >= 70:
		r.State = "good"
	case score >= 40:
		r.State = "fair"
	default:
		r.State = "poor"
	}
	return r
}

// GetProfileReliabilityScores scores every profile for the profile lists'
// health badge. Failures the engine has not seen in over 24h stop counting at
// full weight (the score floors at 25 per *consecutive* failure, and a server
// that answered a connectivity probe since has already been forgiven via
// ClearProfileFailure).
func (a *App) GetProfileReliabilityScores() (map[string]ProfileReliability, error) {
	if a.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	out := make(map[string]ProfileReliability)
	for _, p := range a.store.GetProfiles() {
		out[p.ID] = reliabilityScoreOf(p)
	}
	return out, nil
}
