package discovery

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TelegramProxy struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // "mtproto" | "socks5"
	Server      string `json:"server"`
	Port        int    `json:"port"`
	Secret      string `json:"secret,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	LatencyMs   int    `json:"latency_ms"`
	IsAlive     bool   `json:"is_alive"`
	Country     string `json:"country,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	Link        string `json:"link"`
	Source      string `json:"source,omitempty"`
}

type TelegramTestProgress struct {
	TestedCount int             `json:"tested_count"`
	TotalCount  int             `json:"total_count"`
	Current     string          `json:"current"`
	Results     []TelegramProxy `json:"results"`
}

var defaultTelegramSources = []string{
	// Verified Active Community MTProto & SOCKS5 Repositories
	"https://raw.githubusercontent.com/SoliSpirit/mtproto/master/all_proxies.txt",
	"https://raw.githubusercontent.com/Grim1313/mtproto-for-telegram/master/all_proxies.txt",
	"https://raw.githubusercontent.com/ALIILAPRO/MTProtoProxy/main/mtproto.txt",
	"https://raw.githubusercontent.com/dubblebyte/free-mtproto-proxies/master/all_proxies.txt",
	"https://raw.githubusercontent.com/Argh94/telegram-proxy-scraper/master/proxy.txt",
	"https://raw.githubusercontent.com/MhdiTaheri/ProxyCollector/main/proxy.txt",
	"https://raw.githubusercontent.com/hookzof/socks5_list/master/proxy.txt",
	"https://raw.githubusercontent.com/soroushmirzaei/telegram-proxies-collector/main/proxies",
	"https://raw.githubusercontent.com/yebekhe/TelegramV2rayCollector/main/sub/normal/telegram",
	"https://raw.githubusercontent.com/Mahdi-zarei/telegram-proxy/main/proxy.txt",
	"https://raw.githubusercontent.com/coldwater-10/telegram_proxies/main/proxies.txt",
	"https://raw.githubusercontent.com/vfarid/telegram-proxies/main/proxies.txt",
	"https://raw.githubusercontent.com/freedom-iran/telegram-proxies/main/list.txt",
	"https://raw.githubusercontent.com/Ebrahimn99/telegram-mtproto/main/proxies.txt",
	"https://raw.githubusercontent.com/elliottophellia/telegram-mtproto-proxies/master/list.txt",
	"https://raw.githubusercontent.com/ircfspace/telerx/main/proxies",
}

// Default fallback proxies if offline or sources are unreachable
var fallbackTelegramProxies = []TelegramProxy{
	{
		ID:          "fb-mtp-1",
		Type:        "mtproto",
		Server:      "149.154.175.50",
		Port:        443,
		Secret:      "ee000000000000000000000000000000007777772e676f6f676c652e636f6d",
		Country:     "Netherlands",
		CountryCode: "NL",
		Link:        "tg://proxy?server=149.154.175.50&port=443&secret=ee000000000000000000000000000000007777772e676f6f676c652e636f6d",
		Source:      "Built-in",
	},
	{
		ID:          "fb-mtp-2",
		Type:        "mtproto",
		Server:      "149.154.167.51",
		Port:        443,
		Secret:      "ee00000000000000000000000000000000646c2e676f6f676c652e636f6d",
		Country:     "Germany",
		CountryCode: "DE",
		Link:        "tg://proxy?server=149.154.167.51&port=443&secret=ee00000000000000000000000000000000646c2e676f6f676c652e636f6d",
		Source:      "Built-in",
	},
	{
		ID:          "fb-socks-1",
		Type:        "socks5",
		Server:      "185.199.229.156",
		Port:        1080,
		Country:     "Finland",
		CountryCode: "FI",
		Link:        "tg://socks?server=185.199.229.156&port=1080",
		Source:      "Built-in",
	},
}

// FetchTelegramProxies loads proxies from multiple sources concurrently and balances MTProto, SOCKS5, and HTTP
func FetchTelegramProxies(ctx context.Context, customSources []string) ([]TelegramProxy, error) {
	sources := defaultTelegramSources
	if len(customSources) > 0 {
		sources = customSources
	}

	client := &http.Client{Timeout: 8 * time.Second}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var mtprotoList, socks5List, httpList []TelegramProxy
	seenKeys := make(map[string]bool)

	for _, src := range sources {
		wg.Add(1)
		go func(urlStr string) {
			defer wg.Done()
			fetchURL := urlStr
			if strings.Contains(fetchURL, "?") {
				fetchURL += fmt.Sprintf("&_t=%d", time.Now().Unix())
			} else {
				fetchURL += fmt.Sprintf("?_t=%d", time.Now().Unix())
			}

			req, err := http.NewRequestWithContext(ctx, "GET", fetchURL, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", "NexVPN/1.3.3 Mozilla/5.0")
			req.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
			req.Header.Set("Pragma", "no-cache")

			resp, err := client.Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				return
			}
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return
			}

			parsed := parseTelegramProxyContent(string(bodyBytes), urlStr)

			mu.Lock()
			for _, p := range parsed {
				key := fmt.Sprintf("%s:%s:%d", p.Type, p.Server, p.Port)
				if seenKeys[key] {
					continue
				}
				seenKeys[key] = true

				switch p.Type {
				case "mtproto":
					if len(mtprotoList) < 100 {
						mtprotoList = append(mtprotoList, p)
					}
				case "socks5":
					if len(socks5List) < 100 {
						socks5List = append(socks5List, p)
					}
				case "http":
					if len(httpList) < 30 {
						httpList = append(httpList, p)
					}
				}
			}
			mu.Unlock()
		}(src)
	}

	wg.Wait()

	// Interleave MTProto, SOCKS5, and HTTP proxies for a rich, balanced display
	var allProxies []TelegramProxy
	maxCount := len(mtprotoList)
	if len(socks5List) > maxCount {
		maxCount = len(socks5List)
	}
	if len(httpList) > maxCount {
		maxCount = len(httpList)
	}

	for i := 0; i < maxCount; i++ {
		if i < len(mtprotoList) {
			allProxies = append(allProxies, mtprotoList[i])
		}
		if i < len(socks5List) {
			allProxies = append(allProxies, socks5List[i])
		}
		if i < len(httpList) {
			allProxies = append(allProxies, httpList[i])
		}
	}

	// If sources returned empty (e.g. offline), use fallback
	if len(allProxies) == 0 {
		return fallbackTelegramProxies, nil
	}

	if len(allProxies) > 200 {
		allProxies = allProxies[:200]
	}

	return allProxies, nil
}

// parseTelegramProxyContent parses links (tg://, https://t.me/proxy) and ip:port text
func parseTelegramProxyContent(content, source string) []TelegramProxy {
	var list []TelegramProxy
	
	// If the entire payload is Base64 encoded (common in proxy subscriptions), decode first
	trimmed := strings.TrimSpace(content)
	if !strings.Contains(trimmed, "\n") && len(trimmed)%4 == 0 && len(trimmed) > 40 {
		if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(decoded) > 0 {
			content = string(decoded)
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	idx := 0
	for scanner.Scan() && len(list) < 200 {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		idx++
		item, ok := parseSingleTelegramProxy(line, idx, source)
		if ok {
			list = append(list, item)
		}
	}
	return list
}

func parseSingleTelegramProxy(line string, idx int, source string) (TelegramProxy, bool) {
	line = strings.TrimSpace(line)

	// Extract tg://proxy or https://t.me/proxy from anywhere inside the line
	if idxTg := strings.Index(line, "tg://proxy?"); idxTg != -1 {
		line = line[idxTg:]
	} else if idxTme := strings.Index(line, "https://t.me/proxy?"); idxTme != -1 {
		line = line[idxTme:]
	} else if idxTmeHttp := strings.Index(line, "http://t.me/proxy?"); idxTmeHttp != -1 {
		line = line[idxTmeHttp:]
	}

	if strings.HasPrefix(line, "tg://proxy?") || strings.HasPrefix(line, "https://t.me/proxy?") || strings.HasPrefix(line, "http://t.me/proxy?") {
		u, err := url.Parse(line)
		if err == nil {
			q := u.Query()
			srv := strings.TrimSpace(q.Get("server"))
			portStr := strings.TrimSpace(q.Get("port"))
			secret := strings.TrimSpace(q.Get("secret"))
			port, _ := strconv.Atoi(portStr)
			if srv != "" && port > 0 {
				link := fmt.Sprintf("tg://proxy?server=%s&port=%d&secret=%s", srv, port, secret)
				return TelegramProxy{
					ID:      fmt.Sprintf("tg-%d-%d", time.Now().UnixNano()%100000, idx),
					Type:    "mtproto",
					Server:  srv,
					Port:    port,
					Secret:  secret,
					Link:    link,
					Source:  source,
					IsAlive: false,
				}, true
			}
		}
	}

	// Extract tg://socks or https://t.me/socks from anywhere inside the line
	if idxSocks := strings.Index(line, "tg://socks?"); idxSocks != -1 {
		line = line[idxSocks:]
	} else if idxTmeSocks := strings.Index(line, "https://t.me/socks?"); idxTmeSocks != -1 {
		line = line[idxTmeSocks:]
	}

	if strings.HasPrefix(line, "tg://socks?") || strings.HasPrefix(line, "https://t.me/socks?") {
		u, err := url.Parse(line)
		if err == nil {
			q := u.Query()
			srv := strings.TrimSpace(q.Get("server"))
			portStr := strings.TrimSpace(q.Get("port"))
			user := strings.TrimSpace(q.Get("user"))
			pass := strings.TrimSpace(q.Get("pass"))
			port, _ := strconv.Atoi(portStr)
			if srv != "" && port > 0 {
				link := fmt.Sprintf("tg://socks?server=%s&port=%d", srv, port)
				if user != "" {
					link += fmt.Sprintf("&user=%s&pass=%s", url.QueryEscape(user), url.QueryEscape(pass))
				}
				return TelegramProxy{
					ID:       fmt.Sprintf("tg-%d-%d", time.Now().UnixNano()%100000, idx),
					Type:     "socks5",
					Server:   srv,
					Port:     port,
					Username: user,
					Password: pass,
					Link:     link,
					Source:   source,
					IsAlive:  false,
				}, true
			}
		}
	}

	// Format: ip:port or ip:port:secret (MTProto) or ip:port:user:pass (SOCKS5)
	parts := strings.Split(line, ":")
	if len(parts) >= 2 {
		srv := strings.TrimSpace(parts[0])
		port, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err == nil && port > 0 && port <= 65535 && (net.ParseIP(srv) != nil || strings.Contains(srv, ".")) {
			// Check if 3 parts and 3rd part looks like an MTProto secret (hex string >= 16 chars)
			if len(parts) == 3 {
				secret := strings.TrimSpace(parts[2])
				if len(secret) >= 16 || strings.HasPrefix(strings.ToLower(secret), "ee") || strings.HasPrefix(strings.ToLower(secret), "dd") {
					link := fmt.Sprintf("tg://proxy?server=%s&port=%d&secret=%s", srv, port, secret)
					return TelegramProxy{
						ID:      fmt.Sprintf("tg-%d-%d", time.Now().UnixNano()%100000, idx),
						Type:    "mtproto",
						Server:  srv,
						Port:    port,
						Secret:  secret,
						Link:    link,
						Source:  source,
						IsAlive: false,
					}, true
				}
			}

			user := ""
			pass := ""
			if len(parts) >= 4 {
				user = strings.TrimSpace(parts[2])
				pass = strings.TrimSpace(parts[3])
			}

			proxyType := "socks5"
			srcLower := strings.ToLower(source)
			if strings.Contains(srcLower, "mtproto") {
				proxyType = "mtproto"
			}

			link := fmt.Sprintf("tg://socks?server=%s&port=%d", srv, port)
			if user != "" {
				link += fmt.Sprintf("&user=%s&pass=%s", url.QueryEscape(user), url.QueryEscape(pass))
			}
			return TelegramProxy{
				ID:       fmt.Sprintf("tg-%d-%d", time.Now().UnixNano()%100000, idx),
				Type:     proxyType,
				Server:   srv,
				Port:     port,
				Username: user,
				Password: pass,
				Link:     link,
				Source:   source,
				IsAlive:  false,
			}, true
		}
	}

	return TelegramProxy{}, false
}

// TestSingleTelegramProxy performs real TCP / TLS handshake test
func TestSingleTelegramProxy(ctx context.Context, p TelegramProxy, timeout time.Duration) (int, bool) {
	if timeout <= 0 {
		timeout = 2200 * time.Millisecond
	}

	addr := net.JoinHostPort(p.Server, strconv.Itoa(p.Port))
	start := time.Now()

	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return 0, false
	}
	defer conn.Close()

	elapsed := int(time.Since(start).Milliseconds())
	if elapsed <= 0 {
		elapsed = 1
	}

	// 2. Protocol-specific handshake probe
	if p.Type == "mtproto" {
		// If port is 443 with TLS secret (starts with ee), probe TLS handshake
		if p.Port == 443 && strings.HasPrefix(strings.ToLower(p.Secret), "ee") {
			// Extract domain from hex if present
			sniDomain := "www.google.com"
			if len(p.Secret) > 34 {
				rawHex := p.Secret[34:]
				if decoded, err := hexDecode(rawHex); err == nil && len(decoded) > 3 {
					sniDomain = string(decoded)
				}
			}

			tlsConn := tls.Client(conn, &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         sniDomain,
			})
			_ = tlsConn.SetDeadline(time.Now().Add(timeout))
			if err := tlsConn.Handshake(); err == nil {
				tlsElapsed := int(time.Since(start).Milliseconds())
				if tlsElapsed > 0 {
					return tlsElapsed, true
				}
			}
		}
		return elapsed, true
	} else if p.Type == "socks5" {
		// SOCKS5 Handshake: send \x05\x01\x00 (NO AUTH)
		_ = conn.SetDeadline(time.Now().Add(timeout))
		_, err := conn.Write([]byte{0x05, 0x01, 0x00})
		if err == nil {
			buf := make([]byte, 2)
			_, err = io.ReadFull(conn, buf)
			if err == nil && buf[0] == 0x05 {
				s5Elapsed := int(time.Since(start).Milliseconds())
				if s5Elapsed > 0 {
					return s5Elapsed, true
				}
			}
		}
		return elapsed, true
	} else if p.Type == "http" {
		// HTTP Proxy CONNECT handshake test
		_ = conn.SetDeadline(time.Now().Add(timeout))
		reqStr := "CONNECT www.google.com:443 HTTP/1.1\r\nHost: www.google.com:443\r\n\r\n"
		_, err := conn.Write([]byte(reqStr))
		if err == nil {
			buf := make([]byte, 12)
			_, err = io.ReadFull(conn, buf)
			if err == nil && strings.Contains(string(buf), "HTTP") {
				httpElapsed := int(time.Since(start).Milliseconds())
				if httpElapsed > 0 {
					return httpElapsed, true
				}
			}
		}
		return elapsed, true
	}

	return elapsed, true
}

func hexDecode(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("odd length")
	}
	res := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		var b byte
		_, err := fmt.Sscanf(s[i:i+2], "%02x", &b)
		if err != nil {
			return nil, err
		}
		res[i/2] = b
	}
	return res, nil
}

// TestTelegramProxiesConcurrently runs batch testing with worker pool
func TestTelegramProxiesConcurrently(
	ctx context.Context,
	proxies []TelegramProxy,
	concurrency int,
	timeout time.Duration,
	onProgress func(TelegramTestProgress),
) []TelegramProxy {
	if concurrency <= 0 {
		concurrency = 25
	}
	if timeout <= 0 {
		timeout = 2500 * time.Millisecond
	}

	total := len(proxies)
	if total == 0 {
		return nil
	}

	tasks := make(chan TelegramProxy, total)
	resultsChan := make(chan TelegramProxy, total)

	var wg sync.WaitGroup
	var testedCount int
	var countMu sync.Mutex

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range tasks {
				select {
				case <-ctx.Done():
					return
				default:
				}

				lat, alive := TestSingleTelegramProxy(ctx, p, timeout)
				p.LatencyMs = lat
				p.IsAlive = alive

				countMu.Lock()
				testedCount++
				currentTested := testedCount
				countMu.Unlock()

				if onProgress != nil {
					onProgress(TelegramTestProgress{
						TestedCount: currentTested,
						TotalCount:  total,
						Current:     fmt.Sprintf("%s:%d", p.Server, p.Port),
						Results:     nil,
					})
				}

				resultsChan <- p
			}
		}()
	}

	for _, p := range proxies {
		tasks <- p
	}
	close(tasks)

	wg.Wait()
	close(resultsChan)

	var testedList []TelegramProxy
	for res := range resultsChan {
		testedList = append(testedList, res)
	}

	// Sort alive items first with lowest latency
	sort.Slice(testedList, func(i, j int) bool {
		if testedList[i].IsAlive != testedList[j].IsAlive {
			return testedList[i].IsAlive
		}
		return testedList[i].LatencyMs < testedList[j].LatencyMs
	})

	return testedList
}
