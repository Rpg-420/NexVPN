package core

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"
)

type CleanIPResult struct {
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	LatencyMs int    `json:"latency_ms"`
	LossRate  int    `json:"loss_rate"`
}

type ScanProgress struct {
	Total     int             `json:"total"`
	Scanned   int             `json:"scanned"`
	Found     int             `json:"found"`
	CurrentIP string          `json:"current_ip"`
	Results   []CleanIPResult `json:"results"`
}

type ProgressCallback func(p ScanProgress)
type CompleteCallback func(results []CleanIPResult)

type CleanIPScanner struct {
	mu         sync.Mutex
	cancel     context.CancelFunc
	isScanning bool
}

func NewCleanIPScanner() *CleanIPScanner {
	return &CleanIPScanner{}
}

var defaultSubnets = []string{
	"162.159.192",
	"162.159.193",
	"162.159.195",
	"188.114.96",
	"188.114.97",
	"188.114.98",
	"188.114.99",
	"172.64.100",
}

var defaultPorts = []int{
	2408, 500, 1701, 4500, 443, 854, 859, 864, 878, 880, 884, 890, 891, 894,
	903, 908, 928, 934, 939, 942, 943, 945, 946, 955, 968, 987, 988, 1001,
	1014, 1028, 1070, 1074, 1180, 1387, 1743, 4017, 4038, 4043, 4087, 5000,
}

type targetTask struct {
	ip   string
	port int
}

func (s *CleanIPScanner) Start(customSubnets []string, customPorts []int, onProgress ProgressCallback, onComplete CompleteCallback) {
	s.mu.Lock()
	if s.isScanning && s.cancel != nil {
		s.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.isScanning = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.isScanning = false
			s.mu.Unlock()
		}()

		subnets := defaultSubnets
		if len(customSubnets) > 0 {
			subnets = customSubnets
		}

		ports := defaultPorts
		if len(customPorts) > 0 {
			ports = customPorts
		}

		// Generate random candidate list
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		var tasks []targetTask

		for _, sub := range subnets {
			// Sample 15 random hosts per subnet
			for i := 0; i < 15; i++ {
				host := r.Intn(253) + 1
				ip := fmt.Sprintf("%s.%d", sub, host)
				for _, p := range ports {
					tasks = append(tasks, targetTask{ip: ip, port: p})
				}
			}
		}

		// Shuffle tasks
		r.Shuffle(len(tasks), func(i, j int) {
			tasks[i], tasks[j] = tasks[j], tasks[i]
		})

		// Limit total probes to 150
		if len(tasks) > 150 {
			tasks = tasks[:150]
		}

		total := len(tasks)
		var results []CleanIPResult
		var resMu sync.Mutex
		var scanned int

		// Worker pool
		taskChan := make(chan targetTask, total)
		for _, t := range tasks {
			taskChan <- t
		}
		close(taskChan)

		numWorkers := 30
		var wg sync.WaitGroup

		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for task := range taskChan {
					select {
					case <-ctx.Done():
						return
					default:
					}

					addr := net.JoinHostPort(task.ip, strconv.Itoa(task.port))
					start := time.Now()
					conn, err := net.DialTimeout("tcp", addr, 1200*time.Millisecond)
					var latMs int
					if err == nil {
						latMs = int(time.Since(start).Milliseconds())
						_ = conn.Close()

						if latMs > 0 && latMs < 999 {
							resMu.Lock()
							results = append(results, CleanIPResult{
								IP:        task.ip,
								Port:      task.port,
								LatencyMs: latMs,
								LossRate:  0,
							})
							resMu.Unlock()
						}
					}

					resMu.Lock()
					scanned++
					currentScanned := scanned
					currentFound := len(results)
					var snapshot []CleanIPResult
					if len(results) > 0 {
						snapshot = make([]CleanIPResult, len(results))
						copy(snapshot, results)
					}
					resMu.Unlock()

					if onProgress != nil && (currentScanned%5 == 0 || currentScanned == total) {
						sort.Slice(snapshot, func(i, j int) bool {
							return snapshot[i].LatencyMs < snapshot[j].LatencyMs
						})
						onProgress(ScanProgress{
							Total:     total,
							Scanned:   currentScanned,
							Found:     currentFound,
							CurrentIP: fmt.Sprintf("%s:%d", task.ip, task.port),
							Results:   snapshot,
						})
					}
				}
			}()
		}

		wg.Wait()

		resMu.Lock()
		sort.Slice(results, func(i, j int) bool {
			return results[i].LatencyMs < results[j].LatencyMs
		})
		finalResults := results
		resMu.Unlock()

		if onComplete != nil {
			onComplete(finalResults)
		}
	}()
}

func (s *CleanIPScanner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.isScanning = false
}
