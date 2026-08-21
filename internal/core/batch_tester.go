package core

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/parser"
)

type BatchTestProgress struct {
	TestedCount    int    `json:"tested_count"`
	TotalCount     int    `json:"total_count"`
	CurrentProfile string `json:"current_profile"`
	LatencyMs      int    `json:"latency_ms"`
}

type BatchTestResult struct {
	ProfileID   string `json:"profile_id"`
	ProfileName string `json:"profile_name"`
	LatencyMs   int    `json:"latency_ms"`
	Success     bool   `json:"success"`
}

// BatchTestProfiles tests a list of profiles concurrently using a worker pool
func BatchTestProfiles(
	ctx context.Context,
	profiles []config.Profile,
	concurrency int,
	timeout time.Duration,
	onProgress func(BatchTestProgress),
) []BatchTestResult {
	if concurrency <= 0 {
		concurrency = 25
	}
	if timeout <= 0 {
		timeout = 2500 * time.Millisecond
	}

	total := len(profiles)
	if total == 0 {
		return nil
	}

	type testTask struct {
		profile config.Profile
		index   int
	}

	tasks := make(chan testTask, total)
	resultsChan := make(chan BatchTestResult, total)

	var wg sync.WaitGroup
	var testedCount int64
	var countMu sync.Mutex

	// Worker Pool
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				select {
				case <-ctx.Done():
					return
				default:
				}

				p := task.profile
				lat := testProfileLatency(p, timeout)

				res := BatchTestResult{
					ProfileID:   p.ID,
					ProfileName: p.Name,
					LatencyMs:   lat,
					Success:     lat > 0,
				}

				countMu.Lock()
				testedCount++
				currentTested := int(testedCount)
				countMu.Unlock()

				if onProgress != nil {
					onProgress(BatchTestProgress{
						TestedCount:    currentTested,
						TotalCount:     total,
						CurrentProfile: p.Name,
						LatencyMs:      lat,
					})
				}

				resultsChan <- res
			}
		}()
	}

	// Dispatch tasks
	for idx, p := range profiles {
		tasks <- testTask{profile: p, index: idx}
	}
	close(tasks)

	wg.Wait()
	close(resultsChan)

	var results []BatchTestResult
	for r := range resultsChan {
		results = append(results, r)
	}

	// Sort results: fastest first (positive latencies sorted ascending, failed at end)
	sort.Slice(results, func(i, j int) bool {
		latI := results[i].LatencyMs
		latJ := results[j].LatencyMs
		if latI <= 0 {
			return false
		}
		if latJ <= 0 {
			return true
		}
		return latI < latJ
	})

	return results
}

// BatchTestProfilesRealDelay tests real HTTP internet roundtrip latency
func BatchTestProfilesRealDelay(
	ctx context.Context,
	profiles []config.Profile,
	concurrency int,
	timeout time.Duration,
	onProgress func(BatchTestProgress),
) []BatchTestResult {
	if concurrency <= 0 {
		concurrency = 20
	}
	if timeout <= 0 {
		timeout = 3500 * time.Millisecond
	}

	total := len(profiles)
	if total == 0 {
		return nil
	}

	type testTask struct {
		profile config.Profile
		index   int
	}

	tasks := make(chan testTask, total)
	resultsChan := make(chan BatchTestResult, total)

	var wg sync.WaitGroup
	var testedCount int64
	var countMu sync.Mutex

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				select {
				case <-ctx.Done():
					return
				default:
				}

				p := task.profile
				lat := testProfileRealDelay(ctx, p, timeout)

				res := BatchTestResult{
					ProfileID:   p.ID,
					ProfileName: p.Name,
					LatencyMs:   lat,
					Success:     lat > 0,
				}

				countMu.Lock()
				testedCount++
				currentTested := int(testedCount)
				countMu.Unlock()

				if onProgress != nil {
					onProgress(BatchTestProgress{
						TestedCount:    currentTested,
						TotalCount:     total,
						CurrentProfile: p.Name,
						LatencyMs:      lat,
					})
				}

				resultsChan <- res
			}
		}()
	}

	for idx, p := range profiles {
		tasks <- testTask{profile: p, index: idx}
	}
	close(tasks)

	wg.Wait()
	close(resultsChan)

	var results []BatchTestResult
	for r := range resultsChan {
		results = append(results, r)
	}

	sort.Slice(results, func(i, j int) bool {
		latI := results[i].LatencyMs
		latJ := results[j].LatencyMs
		if latI <= 0 {
			return false
		}
		if latJ <= 0 {
			return true
		}
		return latI < latJ
	})

	return results
}

func testProfileLatency(p config.Profile, timeout time.Duration) int {
	server := p.Server
	port := p.Port

	if server == "" && p.RawURI != "" {
		if u, err := url.Parse(p.RawURI); err == nil {
			server = u.Hostname()
			if pStr := u.Port(); pStr != "" {
				port, _ = strconv.Atoi(pStr)
			}
		}
	}

	if server == "" {
		return -1
	}
	if port <= 0 {
		port = 443
	}

	return parser.TCPPing(server, port, timeout)
}

func testProfileRealDelay(ctx context.Context, p config.Profile, timeout time.Duration) int {
	server := p.Server
	port := p.Port

	if server == "" && p.RawURI != "" {
		if u, err := url.Parse(p.RawURI); err == nil {
			server = u.Hostname()
			if pStr := u.Port(); pStr != "" {
				port, _ = strconv.Atoi(pStr)
			}
		}
	}

	if server == "" {
		return -1
	}
	if port <= 0 {
		port = 443
	}

	return parser.SmartPing(server, port, p.Protocol, timeout)
}
