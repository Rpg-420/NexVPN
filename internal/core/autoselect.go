package core

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/parser"
)

type AutoSelectResult struct {
	Profile   *config.Profile `json:"profile"`
	LatencyMs int             `json:"latency_ms"`
}

// FindFastestProfile concurrently tests responsive latency of profiles and returns the optimal node
func FindFastestProfile(ctx context.Context, profiles []config.Profile, testURL string) (*config.Profile, int, error) {
	if len(profiles) == 0 {
		return nil, -1, fmt.Errorf("no profiles available for auto-selection")
	}

	if testURL == "" {
		testURL = "http://cp.cloudflare.com/generate_204"
	}

	type pingRes struct {
		profile config.Profile
		latency int
	}

	resChan := make(chan pingRes, len(profiles))
	var wg sync.WaitGroup

	sem := make(chan struct{}, 20) // concurrency limit

	for _, p := range profiles {
		if p.Server == "" && p.RawURI == "" && p.ConfigContent == "" {
			continue
		}

		wg.Add(1)
		go func(prof config.Profile) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				return
			default:
			}

			server := prof.Server
			port := prof.Port
			if port <= 0 {
				port = 443
			}

			latency := -1
			if server != "" {
				latency = parser.TCPPing(server, port, 1800*time.Millisecond)
			}

			if latency > 0 {
				resChan <- pingRes{profile: prof, latency: latency}
			}
		}(p)
	}

	wg.Wait()
	close(resChan)

	var valid []pingRes
	for r := range resChan {
		valid = append(valid, r)
	}

	if len(valid) == 0 {
		// Fallback to first profile if all pings timed out
		return &profiles[0], 999, nil
	}

	sort.Slice(valid, func(i, j int) bool {
		return valid[i].latency < valid[j].latency
	})

	best := valid[0].profile
	best.LatencyMs = valid[0].latency
	return &best, valid[0].latency, nil
}
