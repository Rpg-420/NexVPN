package parser

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"sort"
	"sync/atomic"
	"time"
)

type SpeedTestProgress struct {
	Phase        string  `json:"phase"` // "ping", "download", "upload", "complete", "error"
	DownloadMbps float64 `json:"download_mbps"`
	UploadMbps   float64 `json:"upload_mbps"`
	PingMs       int     `json:"ping_ms"`
	JitterMs     int     `json:"jitter_ms"`
	Progress     int     `json:"progress"` // 0 to 100
	ErrorMsg     string  `json:"error_msg,omitempty"`
}

// RunSpeedTest performs real network latency, download and upload speed measurement through local proxy
func RunSpeedTest(ctx context.Context, proxyPort int, onProgress func(SpeedTestProgress)) (*SpeedTestProgress, error) {
	// Test if proxy port is listening; if not, use direct network transport (TUN mode / OpenVPN)
	var transport *http.Transport
	isProxyAvailable := false

	if proxyPort > 0 {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort), 400*time.Millisecond)
		if err == nil {
			conn.Close()
			isProxyAvailable = true
		} else {
			// Try fallback HTTP proxy port 10809 or SOCKS5 10808 if specified port didn't respond
			for _, fallbackPort := range []int{10809, 10808} {
				if fallbackPort == proxyPort {
					continue
				}
				if connFallback, errFB := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", fallbackPort), 200*time.Millisecond); errFB == nil {
					connFallback.Close()
					proxyPort = fallbackPort
					isProxyAvailable = true
					break
				}
			}
		}
	}

	if isProxyAvailable {
		// Format proxy URL properly (http:// for HTTP proxy, socks5:// for SOCKS5)
		proxyScheme := "http"
		if proxyPort == 10808 {
			proxyScheme = "socks5"
		}
		proxyUrl, _ := url.Parse(fmt.Sprintf("%s://127.0.0.1:%d", proxyScheme, proxyPort))
		transport = &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
			DialContext: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).DialContext,
			DisableKeepAlives: false,
		}
	} else {
		transport = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).DialContext,
			DisableKeepAlives: false,
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   25 * time.Second,
	}

	result := &SpeedTestProgress{
		Phase:    "ping",
		Progress: 0,
	}

	if onProgress != nil {
		onProgress(*result)
	}

	// 1. Latency & Jitter Test
	var pings []int
	pingTargets := []string{
		"http://cp.cloudflare.com/generate_204",
		"http://www.google.com/generate_204",
		"https://1.1.1.1/cdn-cgi/trace",
	}

	for i := 0; i < 4; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		target := pingTargets[i%len(pingTargets)]
		start := time.Now()
		req, _ := http.NewRequestWithContext(ctx, "GET", target, nil)
		resp, err := client.Do(req)
		if err == nil {
			ms := int(time.Since(start).Milliseconds())
			pings = append(pings, ms)
			resp.Body.Close()
		}
		time.Sleep(80 * time.Millisecond)
	}

	if len(pings) > 0 {
		sort.Ints(pings)
		result.PingMs = pings[len(pings)/2] // median ping

		// Calculate Jitter
		if len(pings) > 1 {
			diffSum := 0
			for i := 1; i < len(pings); i++ {
				diffSum += int(math.Abs(float64(pings[i] - pings[i-1])))
			}
			result.JitterMs = diffSum / (len(pings) - 1)
		}
	} else {
		result.PingMs = 999
	}

	result.Progress = 20
	result.Phase = "download"
	if onProgress != nil {
		onProgress(*result)
	}

	// 2. Download Throughput Test (10MB test payload with CDN fallbacks)
	downloadTargets := []string{
		"https://speed.cloudflare.com/__down?bytes=10000000",
		"https://proof.ovh.net/files/10Mb.dat",
		"https://speedtest.tele2.net/10MB.zip",
	}

	for _, downloadURL := range downloadTargets {
		req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) NexVPN/1.3")
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}

		var downloadedBytes int64
		startTime := time.Now()
		buf := make([]byte, 32*1024)

		ticker := time.NewTicker(150 * time.Millisecond)
		doneChan := make(chan struct{})

		go func() {
			for {
				select {
				case <-doneChan:
					return
				case <-ticker.C:
					elapsed := time.Since(startTime).Seconds()
					if elapsed > 0 {
						bytesSoFar := atomic.LoadInt64(&downloadedBytes)
						mbps := (float64(bytesSoFar) * 8.0) / (elapsed * 1000000.0)
						result.DownloadMbps = math.Round(mbps*100) / 100
						prog := 20 + int((float64(bytesSoFar)/10000000.0)*50)
						if prog > 70 {
							prog = 70
						}
						result.Progress = prog
						if onProgress != nil {
							onProgress(*result)
						}
					}
				}
			}
		}()

		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				atomic.AddInt64(&downloadedBytes, int64(n))
			}
			if readErr != nil {
				break
			}
		}

		close(doneChan)
		ticker.Stop()
		resp.Body.Close()

		elapsed := time.Since(startTime).Seconds()
		if elapsed > 0 && downloadedBytes > 0 {
			mbps := (float64(downloadedBytes) * 8.0) / (elapsed * 1000000.0)
			result.DownloadMbps = math.Round(mbps*100) / 100
			break
		}
	}

	result.Progress = 70
	result.Phase = "upload"
	if onProgress != nil {
		onProgress(*result)
	}

	// 3. Upload Throughput Test (3MB test payload)
	uploadPayload := make([]byte, 3*1024*1024)
	_, _ = rand.Read(uploadPayload)

	uploadURL := "https://speed.cloudflare.com/__up"
	upStart := time.Now()
	upReq, upErr := http.NewRequestWithContext(ctx, "POST", uploadURL, bytes.NewReader(uploadPayload))
	if upErr == nil {
		upReq.Header.Set("Content-Type", "application/octet-stream")
		upResp, err := client.Do(upReq)
		if err == nil {
			io.Copy(io.Discard, upResp.Body)
			upResp.Body.Close()
			upElapsed := time.Since(upStart).Seconds()
			if upElapsed > 0 {
				upMbps := (float64(len(uploadPayload)) * 8.0) / (upElapsed * 1000000.0)
				result.UploadMbps = math.Round(upMbps*100) / 100
			}
		}
	}

	result.Progress = 100
	result.Phase = "complete"
	if onProgress != nil {
		onProgress(*result)
	}

	return result, nil
}
