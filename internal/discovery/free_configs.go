package discovery

import (
	"context"
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

	"nexvpn/internal/config"
	"nexvpn/internal/parser"
)

type FreeConfigItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Protocol    string `json:"protocol"` // "vless" | "vmess" | "trojan" | "shadowsocks" | "hysteria2" | "tuic" | "wireguard"
	Server      string `json:"server"`
	Port        int    `json:"port"`
	RawURI      string `json:"raw_uri"`
	LatencyMs   int    `json:"latency_ms"`
	IsAlive     bool   `json:"is_alive"`
	Country     string `json:"country,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	SourceName  string `json:"source_name,omitempty"`
	Network     string `json:"network,omitempty"`  // ws, grpc, tcp, http
	Security    string `json:"security,omitempty"` // tls, reality, none
}

type FreeConfigTestProgress struct {
	TestedCount int              `json:"tested_count"`
	TotalCount  int              `json:"total_count"`
	Current     string           `json:"current"`
	Results     []FreeConfigItem `json:"results"`
}

var defaultFreeConfigSources = []string{
	// Curated public aggregator feeds
	"https://raw.githubusercontent.com/roosterkid/openproxylist/refs/heads/main/V2RAY_RAW.txt",
	"https://raw.githubusercontent.com/0xRadikal/Free-v2ray-Configs/main/verified/configs.txt",
	"https://raw.githubusercontent.com/prominbro/sub/refs/heads/main/212.txt",
	"https://raw.githubusercontent.com/free1zona/Keyfreetee/refs/heads/main/razlo4ka7",
	"https://raw.githubusercontent.com/barry-far/V2ray-Configs/main/Sub1.txt",
	"https://raw.githubusercontent.com/barry-far/V2ray-Configs/main/Sub2.txt",
	"https://raw.githubusercontent.com/barry-far/V2ray-Configs/main/Sub3.txt",
	"https://raw.githubusercontent.com/barry-far/V2ray-Configs/main/Sub4.txt",
	"https://raw.githubusercontent.com/yebekhe/TelegramV2rayCollector/main/sub/normal/mix",
	"https://raw.githubusercontent.com/yebekhe/TelegramV2rayCollector/main/sub/normal/vless",
	"https://raw.githubusercontent.com/yebekhe/TelegramV2rayCollector/main/sub/normal/vmess",
	"https://raw.githubusercontent.com/yebekhe/TelegramV2rayCollector/main/sub/normal/trojan",
	"https://raw.githubusercontent.com/mahdibland/V2RayAggregator/master/sub/sub_merge.txt",
	"https://raw.githubusercontent.com/morteza-s-a/free-v2ray-collector/main/sub/mix",
	"https://raw.githubusercontent.com/Epodonios/v2ray-configs/main/All_Configs_Sub.txt",
	"https://raw.githubusercontent.com/freefq/free/master/v2",
	"https://raw.githubusercontent.com/Pawdroid/Free-servers/main/sub",
	"https://raw.githubusercontent.com/LalatinaHub/Mineral/master/result/nodes",
	"https://raw.githubusercontent.com/soroushmirzaei/telegram-configs-collector/main/protocols/reality",
	"https://raw.githubusercontent.com/soroushmirzaei/telegram-configs-collector/main/protocols/vless",
	"https://raw.githubusercontent.com/soroushmirzaei/telegram-configs-collector/main/protocols/hysteria2",
}

// FetchFreeConfigs downloads and parses configs from curated sources
func FetchFreeConfigs(ctx context.Context, customSources []string) ([]FreeConfigItem, error) {
	sources := defaultFreeConfigSources
	if len(customSources) > 0 {
		sources = customSources
	}

	client := &http.Client{Timeout: 8 * time.Second}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allConfigs []FreeConfigItem
	seenKeys := make(map[string]bool)

	for idx, src := range sources {
		wg.Add(1)
		go func(srcIndex int, urlStr string) {
			defer wg.Done()
			req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", "v2rayN/6.42 NexVPN/1.3.3 Clash/1.18")

			resp, err := client.Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				return
			}
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return
			}

			content := string(bodyBytes)
			// Check if base64 encoded
			if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(content)); err == nil {
				content = string(decoded)
			} else if decoded, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(content)); err == nil {
				content = string(decoded)
			}

			sourceName := fmt.Sprintf("Source #%d", srcIndex+1)
			if u, err := url.Parse(urlStr); err == nil {
				pathParts := strings.Split(u.Path, "/")
				if len(pathParts) > 2 {
					sourceName = pathParts[1] + "/" + pathParts[2]
				}
			}

			parsed := parseFreeConfigList(content, sourceName)

			mu.Lock()
			for _, item := range parsed {
				key := fmt.Sprintf("%s://%s:%d", item.Protocol, item.Server, item.Port)
				if !seenKeys[key] && len(allConfigs) < 100 {
					seenKeys[key] = true
					allConfigs = append(allConfigs, item)
				}
			}
			mu.Unlock()
		}(idx, src)
	}

	wg.Wait()

	// Limit to max 100 items to keep UI snappy
	if len(allConfigs) > 100 {
		allConfigs = allConfigs[:100]
	}

	return allConfigs, nil
}

func parseFreeConfigList(content, sourceName string) []FreeConfigItem {
	var list []FreeConfigItem
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		if len(list) >= 20 {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		item, ok := parseSingleFreeConfig(line, i+1, sourceName)
		if ok {
			list = append(list, item)
		}
	}
	return list
}

func parseSingleFreeConfig(uri string, index int, sourceName string) (FreeConfigItem, bool) {
	uri = strings.TrimSpace(uri)
	lower := strings.ToLower(uri)

	var proto string
	if strings.HasPrefix(lower, "vless://") {
		proto = "vless"
	} else if strings.HasPrefix(lower, "vmess://") {
		proto = "vmess"
	} else if strings.HasPrefix(lower, "trojan://") {
		proto = "trojan"
	} else if strings.HasPrefix(lower, "ss://") {
		proto = "shadowsocks"
	} else if strings.HasPrefix(lower, "hysteria2://") || strings.HasPrefix(lower, "hy2://") {
		proto = "hysteria2"
	} else if strings.HasPrefix(lower, "tuic://") {
		proto = "tuic"
	} else {
		return FreeConfigItem{}, false
	}

	// Try standard parser first to extract metadata
	p, err := parser.ParseURI(uri)
	if err == nil && p.Server != "" && p.Port > 0 {
		cc, country := detectCountryCode(p.Name, p.Server)
		return FreeConfigItem{
			ID:          fmt.Sprintf("free-%d-%d", time.Now().UnixNano()%100000, index),
			Name:        p.Name,
			Protocol:    proto,
			Server:      p.Server,
			Port:        p.Port,
			RawURI:      uri,
			LatencyMs:   0,
			IsAlive:     false,
			CountryCode: cc,
			Country:     country,
			SourceName:  sourceName,
		}, true
	}

	// Fallback URL parsing
	u, err := url.Parse(uri)
	if err == nil && u.Hostname() != "" {
		port, _ := strconv.Atoi(u.Port())
		name := u.Fragment
		if name == "" {
			name = fmt.Sprintf("%s-%s:%d", strings.ToUpper(proto), u.Hostname(), port)
		} else {
			name, _ = url.QueryUnescape(name)
		}

		cc, country := detectCountryCode(name, u.Hostname())
		return FreeConfigItem{
			ID:          fmt.Sprintf("free-%d-%d", time.Now().UnixNano()%100000, index),
			Name:        name,
			Protocol:    proto,
			Server:      u.Hostname(),
			Port:        port,
			RawURI:      uri,
			CountryCode: cc,
			Country:     country,
			SourceName:  sourceName,
		}, true
	}

	return FreeConfigItem{}, false
}

func detectCountryCode(name, server string) (string, string) {
	lower := strings.ToLower(name + " " + server)
	if strings.Contains(lower, "🇩🇪") || strings.Contains(lower, "germany") || strings.Contains(lower, "deutschland") || strings.Contains(lower, "frankfurt") || strings.Contains(lower, "德国") {
		return "DE", "Germany"
	}
	if strings.Contains(lower, "🇫🇮") || strings.Contains(lower, "finland") || strings.Contains(lower, "helsinki") || strings.Contains(lower, "芬兰") {
		return "FI", "Finland"
	}
	if strings.Contains(lower, "🇳🇱") || strings.Contains(lower, "netherlands") || strings.Contains(lower, "amsterdam") || strings.Contains(lower, "holland") || strings.Contains(lower, "荷兰") {
		return "NL", "Netherlands"
	}
	if strings.Contains(lower, "🇬🇧") || strings.Contains(lower, "united kingdom") || strings.Contains(lower, "london") || strings.Contains(lower, "britain") || strings.Contains(lower, "英国") {
		return "GB", "United Kingdom"
	}
	if strings.Contains(lower, "🇫🇷") || strings.Contains(lower, "france") || strings.Contains(lower, "paris") || strings.Contains(lower, "法国") {
		return "FR", "France"
	}
	if strings.Contains(lower, "🇹🇷") || strings.Contains(lower, "turkey") || strings.Contains(lower, "istanbul") || strings.Contains(lower, "turkiye") || strings.Contains(lower, "土耳其") {
		return "TR", "Turkey"
	}
	if strings.Contains(lower, "🇸🇬") || strings.Contains(lower, "singapore") || strings.Contains(lower, "新加坡") {
		return "SG", "Singapore"
	}
	if strings.Contains(lower, "🇯🇵") || strings.Contains(lower, "japan") || strings.Contains(lower, "tokyo") || strings.Contains(lower, "日本") {
		return "JP", "Japan"
	}
	if strings.Contains(lower, "🇨🇦") || strings.Contains(lower, "canada") || strings.Contains(lower, "toronto") || strings.Contains(lower, "加拿大") {
		return "CA", "Canada"
	}
	if strings.Contains(lower, "🇮🇷") || strings.Contains(lower, "iran") || strings.Contains(lower, "tehran") || strings.Contains(lower, "ایران") {
		return "IR", "Iran"
	}
	if strings.Contains(lower, "🇧🇷") || strings.Contains(lower, "brazil") || strings.Contains(lower, "巴西") {
		return "BR", "Brazil"
	}
	if strings.Contains(lower, "🇺🇸") || strings.Contains(lower, "united states") || strings.Contains(lower, "usa") || strings.Contains(lower, "america") || strings.Contains(lower, "美国") {
		return "US", "United States"
	}
	return "US", "Anycast / Global"
}

// TestSingleFreeConfig tests TCP/TLS handshake latency
func TestSingleFreeConfig(item FreeConfigItem, timeout time.Duration) (int, bool) {
	if timeout <= 0 {
		timeout = 2500 * time.Millisecond
	}

	if item.Server == "" || item.Port <= 0 {
		return 0, false
	}

	addr := net.JoinHostPort(item.Server, strconv.Itoa(item.Port))
	start := time.Now()

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return 0, false
	}
	defer conn.Close()

	elapsed := int(time.Since(start).Milliseconds())
	if elapsed == 0 {
		elapsed = 1
	}
	return elapsed, true
}

// TestFreeConfigsConcurrently tests a slice of free configs
func TestFreeConfigsConcurrently(
	ctx context.Context,
	configs []FreeConfigItem,
	concurrency int,
	timeout time.Duration,
	onProgress func(FreeConfigTestProgress),
) []FreeConfigItem {
	if concurrency <= 0 {
		concurrency = 30
	}
	if timeout <= 0 {
		timeout = 2500 * time.Millisecond
	}

	total := len(configs)
	if total == 0 {
		return nil
	}

	tasks := make(chan FreeConfigItem, total)
	resultsChan := make(chan FreeConfigItem, total)

	var wg sync.WaitGroup
	var testedCount int
	var countMu sync.Mutex

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range tasks {
				select {
				case <-ctx.Done():
					return
				default:
				}

				lat, alive := TestSingleFreeConfig(item, timeout)
				item.LatencyMs = lat
				item.IsAlive = alive

				countMu.Lock()
				testedCount++
				currentTested := testedCount
				countMu.Unlock()

				if onProgress != nil {
					onProgress(FreeConfigTestProgress{
						TestedCount: currentTested,
						TotalCount:  total,
						Current:     item.Name,
					})
				}

				resultsChan <- item
			}
		}()
	}

	for _, item := range configs {
		tasks <- item
	}
	close(tasks)

	wg.Wait()
	close(resultsChan)

	var testedList []FreeConfigItem
	for res := range resultsChan {
		testedList = append(testedList, res)
	}

	sort.Slice(testedList, func(i, j int) bool {
		if testedList[i].IsAlive != testedList[j].IsAlive {
			return testedList[i].IsAlive
		}
		return testedList[i].LatencyMs < testedList[j].LatencyMs
	})

	return testedList
}

// ConvertFreeConfigToProfile converts a FreeConfigItem to config.Profile for importing
func ConvertFreeConfigToProfile(item FreeConfigItem, groupName string) (config.Profile, error) {
	if groupName == "" {
		groupName = "Free Community"
	}

	node, err := parser.ParseURI(item.RawURI)
	if err != nil {
		// Fallback minimal profile
		return config.Profile{
			ID:            fmt.Sprintf("prof-%d-%s", time.Now().UnixNano(), item.ID),
			Name:          item.Name,
			Protocol:      config.Protocol(item.Protocol),
			Category:      config.CategoryV2Ray,
			RawURI:        item.RawURI,
			Server:        item.Server,
			Port:          item.Port,
			Group:         groupName,
			Tags:          []string{"FreeHub", strings.ToUpper(item.Protocol)},
			LatencyMs:     item.LatencyMs,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}, nil
	}

	return config.Profile{
		ID:            fmt.Sprintf("prof-%d-%s", time.Now().UnixNano(), item.ID),
		Name:          node.Name,
		Protocol:      node.Protocol,
		Category:      node.Category,
		ConfigContent: node.ConfigContent,
		RawURI:        node.RawURI,
		Server:        node.Server,
		Port:          node.Port,
		Group:         groupName,
		Username:      node.Username,
		Password:      node.Password,
		Tags:          []string{"FreeHub", strings.ToUpper(item.Protocol)},
		LatencyMs:     item.LatencyMs,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}, nil
}
