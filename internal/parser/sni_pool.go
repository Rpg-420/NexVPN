package parser

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

type SNIPoolItem struct {
	Domain    string `json:"domain"`
	Provider  string `json:"provider"`
	LatencyMs int    `json:"latency_ms"`
	Status    string `json:"status"` // "working", "blocked", "untested"
}

var defaultSNIPool = []SNIPoolItem{
	{Domain: "gateway.icloud.com", Provider: "Apple Cloud"},
	{Domain: "images.apple.com", Provider: "Apple CDN"},
	{Domain: "swdist.apple.com", Provider: "Apple Updates"},
	{Domain: "www.microsoft.com", Provider: "Microsoft"},
	{Domain: "learn.microsoft.com", Provider: "Microsoft Docs"},
	{Domain: "azure.microsoft.com", Provider: "Microsoft Azure"},
	{Domain: "aws.amazon.com", Provider: "Amazon AWS"},
	{Domain: "cdn.cloudflare.com", Provider: "Cloudflare CDN"},
	{Domain: "dl.google.com", Provider: "Google CDN"},
	{Domain: "www.samsung.com", Provider: "Samsung Global"},
}

// GetDefaultSNIPool returns a copy of the built-in curated SNI pool
func GetDefaultSNIPool() []SNIPoolItem {
	res := make([]SNIPoolItem, len(defaultSNIPool))
	copy(res, defaultSNIPool)
	return res
}

// GetCombinedSNIPool returns built-in and user-custom SNI domains
func GetCombinedSNIPool(customDomains []string) []SNIPoolItem {
	pool := GetDefaultSNIPool()
	seen := make(map[string]bool)
	for _, item := range pool {
		seen[strings.ToLower(item.Domain)] = true
	}

	for _, d := range customDomains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		pool = append(pool, SNIPoolItem{
			Domain:   d,
			Provider: "User Custom",
			Status:   "untested",
		})
	}
	return pool
}

// TestSNIPoolLatency performs a real TLS 1.3 handshake to check if the SNI domain is reachable and unthrottled
func TestSNIPoolLatency(domain string) (int, bool) {
	start := time.Now()

	dialer := &net.Dialer{
		Timeout: 2500 * time.Millisecond,
	}

	rawConn, err := dialer.DialContext(context.Background(), "tcp", net.JoinHostPort(domain, "443"))
	if err != nil {
		return 0, false
	}
	defer rawConn.Close()

	tlsConfig := &tls.Config{
		ServerName: domain,
		// Certificate verification is deliberately off, and this is the ONLY
		// place in the project that sets it.
		//
		// The question this probe answers is "can a TLS 1.3 ClientHello for this
		// SNI complete a handshake to this address", not "is this address who it
		// claims to be" — the candidate list is a pool of SNIs to *offer*, and
		// the certificate chain is irrelevant to whether the handshake succeeds.
		// Verifying here would reject the many CDN and Reality front-ends whose
		// certificate is not issued for the borrowed name, which is precisely the
		// case the pool exists to test.
		//
		// Consequence to keep in mind: a `working` result means "the handshake
		// completed", NOT "the endpoint presented a valid certificate for this
		// domain". Do not reuse this config for anything that carries data.
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		NextProtos:         []string{"h2", "http/1.1"},
	}

	tlsConn := tls.Client(rawConn, tlsConfig)
	_ = tlsConn.SetDeadline(time.Now().Add(2500 * time.Millisecond))

	if err := tlsConn.Handshake(); err != nil {
		return 0, false
	}

	latency := int(time.Since(start).Milliseconds())
	return latency, true
}

// AuditSNIPoolWithCustom tests all candidate SNIs (built-in + custom) concurrently
func AuditSNIPoolWithCustom(customDomains []string) []SNIPoolItem {
	pool := GetCombinedSNIPool(customDomains)
	var wg sync.WaitGroup
	var mu sync.Mutex

	results := make([]SNIPoolItem, len(pool))

	for i, item := range pool {
		wg.Add(1)
		go func(idx int, target SNIPoolItem) {
			defer wg.Done()
			lat, ok := TestSNIPoolLatency(target.Domain)
			mu.Lock()
			defer mu.Unlock()
			if ok {
				target.LatencyMs = lat
				target.Status = "working"
			} else {
				target.LatencyMs = 9999
				target.Status = "blocked"
			}
			results[idx] = target
		}(i, item)
	}

	wg.Wait()
	return results
}

// AuditSNIPool tests all candidate SNIs concurrently and returns their live latency and status
func AuditSNIPool() []SNIPoolItem {
	return AuditSNIPoolWithCustom(nil)
}

// OptimizeProfileSNI tests the profile's current SNI and if slow or failing, replaces it with the fastest working SNI from the pool
func OptimizeProfileSNI(rawURI string) (string, string, error) {
	rawURI = strings.TrimSpace(rawURI)
	if rawURI == "" {
		return "", "", fmt.Errorf("empty raw URI")
	}

	u, err := url.Parse(rawURI)
	if err != nil {
		return rawURI, "", err
	}

	q := u.Query()

	// Check if this profile uses VLESS Reality / PBK (Public Key)
	// For Reality configs, changing SNI will break the cryptographic handshake!
	if strings.EqualFold(q.Get("security"), "reality") || q.Get("pbk") != "" {
		currentSNI := q.Get("sni")
		if currentSNI == "" {
			currentSNI = q.Get("host")
		}
		if currentSNI == "" {
			currentSNI = u.Hostname()
		}
		return rawURI, currentSNI, fmt.Errorf("reality profiles cannot have their SNI changed (SNI is cryptographically bound to the server public key)")
	}

	currentSNI := q.Get("sni")
	if currentSNI == "" {
		currentSNI = q.Get("host")
	}
	if currentSNI == "" {
		currentSNI = u.Hostname()
	}

	// Test current SNI
	currentLat, currentOk := TestSNIPoolLatency(currentSNI)
	if currentOk && currentLat < 350 {
		// Current SNI is already fast and healthy
		return rawURI, currentSNI, nil
	}

	// Current SNI is failing or slow; find the fastest working replacement
	audited := AuditSNIPool()
	var fastestItem *SNIPoolItem
	for _, item := range audited {
		if item.Status == "working" {
			if fastestItem == nil || item.LatencyMs < fastestItem.LatencyMs {
				itemCopy := item
				fastestItem = &itemCopy
			}
		}
	}

	if fastestItem == nil {
		return rawURI, currentSNI, fmt.Errorf("no working SNI found in pool")
	}

	// Patch URI with the fastest SNI
	q.Set("sni", fastestItem.Domain)
	if q.Get("host") != "" {
		q.Set("host", fastestItem.Domain)
	}
	u.RawQuery = q.Encode()

	return u.String(), fastestItem.Domain, nil
}
