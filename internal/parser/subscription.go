package parser

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// maxSubscriptionBodyBytes caps every subscription body read anywhere in this
// package. A subscription is a remote document fetched over whatever network
// the user happens to be on; without a cap, a hostile or merely broken endpoint
// can make the process allocate until it is killed. 10 MiB is far above any real
// subscription (the largest known ones are well under 2 MiB) and far below a
// problem.
const maxSubscriptionBodyBytes = 10 << 20

type SubscriptionUserInfo struct {
	UploadBytes   int64 `json:"upload_bytes"`
	DownloadBytes int64 `json:"download_bytes"`
	TotalBytes    int64 `json:"total_bytes"`
	UsedBytes     int64 `json:"used_bytes"`
	ExpireDate    int64 `json:"expire_date"` // Unix timestamp
	RemainingDays int   `json:"remaining_days"`
	PercentUsed   int   `json:"percent_used"`
}

type SubscriptionFetchResult struct {
	Content  string                `json:"content"`
	UserInfo *SubscriptionUserInfo `json:"user_info,omitempty"`
}

// FetchSubscriptionWithUserInfo fetches remote subscription and parses quota headers
func FetchSubscriptionWithUserInfo(ctx context.Context, urlStr string) (*SubscriptionFetchResult, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(urlStr))
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Hostname() == "" {
		return nil, fmt.Errorf("invalid subscription URL")
	}
	if ip := net.ParseIP(parsedURL.Hostname()); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
		return nil, fmt.Errorf("private subscription hosts are not allowed")
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", parsedURL.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "v2rayN/6.42 NexVPN/1.4.0 Clash/1.18")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subscription: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}

	const maxSubscriptionBytes = maxSubscriptionBodyBytes // 10 MiB safety limit
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxSubscriptionBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if int64(len(bodyBytes)) > maxSubscriptionBytes {
		return nil, fmt.Errorf("subscription response exceeds the %d MiB limit", maxSubscriptionBytes/(1<<20))
	}

	res := &SubscriptionFetchResult{
		Content: string(bodyBytes),
	}

	// Parse subscription userinfo header if present (e.g. upload=1000; download=2000; total=10737418240; expire=1700000000)
	userInfoHeader := resp.Header.Get("Subscription-Userinfo")
	if userInfoHeader == "" {
		userInfoHeader = resp.Header.Get("User-Status")
	}

	if userInfoHeader != "" {
		res.UserInfo = parseUserInfoHeader(userInfoHeader)
	}

	return res, nil
}

func parseUserInfoHeader(header string) *SubscriptionUserInfo {
	info := &SubscriptionUserInfo{}
	parts := strings.Split(header, ";")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := strings.TrimSpace(kv[1])

		switch key {
		case "upload":
			info.UploadBytes, _ = strconv.ParseInt(val, 10, 64)
		case "download":
			info.DownloadBytes, _ = strconv.ParseInt(val, 10, 64)
		case "total":
			info.TotalBytes, _ = strconv.ParseInt(val, 10, 64)
		case "expire":
			info.ExpireDate, _ = strconv.ParseInt(val, 10, 64)
		}
	}

	info.UsedBytes = info.UploadBytes + info.DownloadBytes
	if info.TotalBytes > 0 {
		percent := (info.UsedBytes * 100) / info.TotalBytes
		if percent > 100 {
			percent = 100
		}
		if percent < 0 {
			percent = 0
		}
		info.PercentUsed = int(percent)
	}

	if info.ExpireDate > 0 {
		now := time.Now().Unix()
		diff := info.ExpireDate - now
		if diff > 0 {
			info.RemainingDays = int(diff / 86400)
		} else {
			info.RemainingDays = 0
		}
	}

	return info
}
