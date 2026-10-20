package parser

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ServiceUnblockTarget represents a service endpoint to probe
type ServiceUnblockTarget struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"` // "ai", "streaming", "social", "gaming", "dev"
	URL         string `json:"url"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
}

// ServiceUnblockResult holds status and latency for a single service probe
type ServiceUnblockResult struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	Status     string `json:"status"` // "unlocked", "blocked", "restricted", "error"
	LatencyMs  int    `json:"latency_ms"`
	HTTPStatus int    `json:"http_status"`
	Region     string `json:"region,omitempty"`
	Message    string `json:"message"`
}

// Standard popular services to probe
var defaultServiceTargets = []ServiceUnblockTarget{
	{
		ID:          "chatgpt",
		Name:        "ChatGPT (OpenAI)",
		Category:    "ai",
		URL:         "https://chatgpt.com/favicon.ico",
		Icon:        "Bot",
		Description: "OpenAI ChatGPT & GPT-4o platform",
	},
	{
		ID:          "claude",
		Name:        "Claude AI (Anthropic)",
		Category:    "ai",
		URL:         "https://claude.ai/favicon.ico",
		Icon:        "Sparkles",
		Description: "Anthropic Claude 3.5 Sonnet / Opus",
	},
	{
		ID:          "youtube",
		Name:        "YouTube (Google)",
		Category:    "streaming",
		URL:         "https://www.youtube.com/generate_204",
		Icon:        "PlaySquare",
		Description: "YouTube video streaming & Music",
	},
	{
		ID:          "spotify",
		Name:        "Spotify Web & API",
		Category:    "streaming",
		URL:         "https://api.spotify.com/v1/",
		Icon:        "Music",
		Description: "Spotify audio streaming platform",
	},
	{
		ID:          "telegram",
		Name:        "Telegram Web & API",
		Category:    "social",
		URL:         "https://web.telegram.org/",
		Icon:        "Send",
		Description: "Telegram web client and media servers",
	},
	{
		ID:          "discord",
		Name:        "Discord Gateway",
		Category:    "social",
		URL:         "https://discord.com/api/v9/gateway",
		Icon:        "MessageSquare",
		Description: "Discord voice, video and chat servers",
	},
	{
		ID:          "instagram",
		Name:        "Instagram / Threads",
		Category:    "social",
		URL:         "https://www.instagram.com/",
		Icon:        "Camera",
		Description: "Instagram feeds & direct messaging",
	},
	{
		ID:          "twitter",
		Name:        "X / Twitter",
		Category:    "social",
		URL:         "https://twitter.com/",
		Icon:        "Twitter",
		Description: "X social media feed & media CDN",
	},
	{
		ID:          "steam",
		Name:        "Steam Store & Community",
		Category:    "gaming",
		URL:         "https://store.steampowered.com/",
		Icon:        "Gamepad2",
		Description: "Valve Steam Store and Community hubs",
	},
	{
		ID:          "github",
		Name:        "GitHub & Copilot",
		Category:    "dev",
		URL:         "https://github.com/",
		Icon:        "Github",
		Description: "GitHub repository and API services",
	},
	{
		ID:          "gemini",
		Name:        "Google Gemini AI",
		Category:    "ai",
		URL:         "https://gemini.google.com/favicon.ico",
		Icon:        "Sparkles",
		Description: "Google Gemini generative AI suite",
	},
	{
		ID:          "netflix",
		Name:        "Netflix Streaming",
		Category:    "streaming",
		URL:         "https://www.netflix.com/favicon.ico",
		Icon:        "Film",
		Description: "Netflix global streaming library",
	},
	{
		ID:          "twitch",
		Name:        "Twitch Live Stream",
		Category:    "streaming",
		URL:         "https://www.twitch.tv/",
		Icon:        "Tv",
		Description: "Twitch live broadcasting platform",
	},
}

// CheckServicesUnblock probes each service through the specified HTTP/SOCKS5 proxy port concurrently
func CheckServicesUnblock(ctx context.Context, proxyPort int) ([]ServiceUnblockResult, error) {
	var proxyFunc func(*http.Request) (*url.URL, error)
	if proxyPort > 0 {
		proxyURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", proxyPort))
		if err == nil {
			proxyFunc = http.ProxyURL(proxyURL)
		}
	}

	transport := &http.Transport{
		Proxy: proxyFunc,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
		DisableKeepAlives: true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   6 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	results := make([]ServiceUnblockResult, len(defaultServiceTargets))
	var wg sync.WaitGroup

	for i, target := range defaultServiceTargets {
		wg.Add(1)
		go func(idx int, t ServiceUnblockTarget) {
			defer wg.Done()

			start := time.Now()
			reqCtx, reqCancel := context.WithTimeout(ctx, 5*time.Second)
			defer reqCancel()

			req, err := http.NewRequestWithContext(reqCtx, "GET", t.URL, nil)
			if err != nil {
				results[idx] = ServiceUnblockResult{
					ID:         t.ID,
					Name:       t.Name,
					Category:   t.Category,
					Status:     "error",
					LatencyMs:  -1,
					HTTPStatus: 0,
					Message:    err.Error(),
				}
				return
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
			req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")

			resp, err := client.Do(req)
			lat := int(time.Since(start).Milliseconds())

			if err != nil {
				results[idx] = ServiceUnblockResult{
					ID:         t.ID,
					Name:       t.Name,
					Category:   t.Category,
					Status:     "blocked",
					LatencyMs:  -1,
					HTTPStatus: 0,
					Message:    "Connection timed out or refused",
				}
				return
			}
			defer func() { _ = resp.Body.Close() }()

			status := "unlocked"
			msg := "Available & Fast"

			// Detect Cloudflare / Akamai / Geo blocking
			if resp.StatusCode == 403 || resp.StatusCode == 451 || resp.StatusCode == 1020 {
				status = "restricted"
				msg = fmt.Sprintf("Geo-restricted (HTTP %d)", resp.StatusCode)
			} else if resp.StatusCode >= 500 {
				status = "error"
				msg = fmt.Sprintf("Server error (HTTP %d)", resp.StatusCode)
			} else if resp.StatusCode >= 200 && resp.StatusCode < 400 || resp.StatusCode == 401 || resp.StatusCode == 404 {
				// 401 Unauthorized or 404 on API endpoints still means the service itself is reachable and unblocked
				status = "unlocked"
				msg = fmt.Sprintf("Reachable (HTTP %d)", resp.StatusCode)
			}

			results[idx] = ServiceUnblockResult{
				ID:         t.ID,
				Name:       t.Name,
				Category:   t.Category,
				Status:     status,
				LatencyMs:  lat,
				HTTPStatus: resp.StatusCode,
				Message:    msg,
			}
		}(i, target)
	}

	wg.Wait()
	return results, nil
}
