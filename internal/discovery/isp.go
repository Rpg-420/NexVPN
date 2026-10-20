package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// StatusScore is 0 - 100 and always derived from measured facts (not invented).
// It is computed from the network's observed responsiveness while detecting the
// ISP, so the UI can no longer show a hard-coded "quality" number.
type ISPRecommendation struct {
	ISPName             string `json:"isp_name"`
	ISPNameEN           string `json:"isp_name_en"`
	ISPCategory         string `json:"isp_category"` // "mci" | "mtn" | "rtl" | "fixed" | "other"
	IP                  string `json:"ip"`
	Country             string `json:"country"`
	CountryCode         string `json:"country_code"`
	ASN                 string `json:"asn"`
	RecommendedProtocol string `json:"recommended_protocol"`
	RecommendedFragment string `json:"recommended_fragment"`
	FragmentSize        string `json:"fragment_size"`
	FragmentSleep       string `json:"fragment_sleep"`
	RecommendedDNS      string `json:"recommended_dns"`
	StatusScore         int    `json:"status_score"`
	DetectionMs         int    `json:"detection_ms"` // real round-trip of the detection request
	Detected            bool   `json:"detected"`     // false when the lookup failed
	Advice              string `json:"advice"`
	AdviceEN            string `json:"advice_en"`
}

// DetectCurrentISP detects the ISP and returns custom-tailored bypass recommendations.
// When detection fails it returns a recommendation with Detected=false and the
// underlying error, so the UI shows "could not detect" instead of fabricated data.
func DetectCurrentISP(ctx context.Context) (*ISPRecommendation, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "http://ip-api.com/json/?fields=status,message,country,countryCode,isp,org,as,query", nil)
	if err != nil {
		return undetectedRecommendation("could not build detection request: " + err.Error()), err
	}

	start := time.Now()
	resp, err := client.Do(req)
	detectionMs := int(time.Since(start).Milliseconds())
	if err != nil {
		return undetectedRecommendation("ISP detection request failed: " + err.Error()), err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return undetectedRecommendation(fmt.Sprintf("ISP detection service returned HTTP %d", resp.StatusCode)), nil
	}
	defer resp.Body.Close()

	// A JSON object with five short fields. The cap is generous for the real
	// response and small enough that a hostile endpoint cannot make this
	// background probe allocate without bound.
	const maxISPResponseBytes = 64 << 10
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxISPResponseBytes+1))
	if err != nil {
		return undetectedRecommendation("could not read ISP detection response: " + err.Error()), err
	}
	if len(body) > maxISPResponseBytes {
		return undetectedRecommendation("ISP detection response was unexpectedly large"), nil
	}

	var res struct {
		Status      string `json:"status"`
		Country     string `json:"country"`
		CountryCode string `json:"countryCode"`
		ISP         string `json:"isp"`
		Org         string `json:"org"`
		AS          string `json:"as"`
		Query       string `json:"query"`
	}

	if err := json.Unmarshal(body, &res); err != nil || res.Status != "success" {
		return undetectedRecommendation("ISP detection service returned an unusable response"), nil
	}

	rawText := strings.ToLower(res.ISP + " " + res.Org + " " + res.AS)

	rec := &ISPRecommendation{
		IP:          res.Query,
		Country:     res.Country,
		CountryCode: res.CountryCode,
		ASN:         res.AS,
		Detected:    true,
		DetectionMs: detectionMs,
	}

	if strings.Contains(rawText, "mobile telecommunication") || strings.Contains(rawText, "mci") || strings.Contains(rawText, "as197207") {
		rec.ISPName = "همراه اول (MCI)"
		rec.ISPNameEN = "Hamrah-e Aval (MCI)"
		rec.ISPCategory = "mci"
		rec.RecommendedProtocol = "VLESS Reality (TCP + Vision)"
		rec.RecommendedFragment = "10-30 (2-5ms)"
		rec.FragmentSize = "10-30"
		rec.FragmentSleep = "2-5"
		rec.RecommendedDNS = "1.1.1.1 (Cloudflare)"
		rec.Advice = "پروتکل VLESS Reality با پورت 443 و فرگمنت ۱۰-۳۰ بهترین پایداری را روی همراه اول دارد."
		rec.AdviceEN = "VLESS Reality protocol with port 443 and 10-30 fragment provides the best stability on MCI."
	} else if strings.Contains(rawText, "irancell") || strings.Contains(rawText, "mtn") || strings.Contains(rawText, "as44244") {
		rec.ISPName = "ایرانسل (MTN Irancell)"
		rec.ISPNameEN = "MTN Irancell"
		rec.ISPCategory = "mtn"
		rec.RecommendedProtocol = "Hysteria 2 / Cloudflare WARP"
		rec.RecommendedFragment = "50-100 (10-20ms)"
		rec.FragmentSize = "50-100"
		rec.FragmentSleep = "10-20"
		rec.RecommendedDNS = "8.8.8.8 (Google)"
		rec.Advice = "پروتکل‌های مبتنی بر UDP (مانند Hysteria 2 و WARP on WARP) سرعت بالاتری روی ایرانسل ثبت می‌کنند."
		rec.AdviceEN = "UDP-based protocols like Hysteria 2 and WARP-on-WARP achieve highest speeds on Irancell."
	} else if strings.Contains(rawText, "rightel") || strings.Contains(rawText, "as57218") {
		rec.ISPName = "رایتل (Rightel)"
		rec.ISPNameEN = "Rightel"
		rec.ISPCategory = "rtl"
		rec.RecommendedProtocol = "VLESS Reality / Trojan TLS"
		rec.RecommendedFragment = "30-60 (5-15ms)"
		rec.FragmentSize = "30-60"
		rec.FragmentSleep = "5-15"
		rec.RecommendedDNS = "1.1.1.1"
		rec.Advice = "پروتکل‌های Reality و Trojan با پورت‌های استاندارد بدون قطعی عمل می‌کنند."
		rec.AdviceEN = "Reality and Trojan protocols with standard ports operate seamlessly on Rightel."
	} else if strings.Contains(rawText, "telecommunication company of iran") || strings.Contains(rawText, "tci") || strings.Contains(rawText, "shatel") || strings.Contains(rawText, "asiatech") || strings.Contains(rawText, "pars online") {
		rec.ISPName = "اینترنت ثابت (مخابرات / شاتل / آسیاتک)"
		rec.ISPNameEN = "Fixed Broadband (TCI / Shatel / Asiatech)"
		rec.ISPCategory = "fixed"
		rec.RecommendedProtocol = "Cloudflare WARP / VLESS gRPC"
		rec.RecommendedFragment = "100-200 (5-15ms)"
		rec.FragmentSize = "100-200"
		rec.FragmentSleep = "5-15"
		rec.RecommendedDNS = "1.1.1.1 (Cloudflare DoH)"
		rec.Advice = "آی‌پی‌های تمیز رنج 188.114.96.1 روی شبکه‌های ثابت کمترین پینگ و تاخیر را ارائه می‌دهند."
		rec.AdviceEN = "Cloudflare Clean IP range 188.114.96.1 offers lowest ping and latency on Fixed Broadband."
	} else {
		rec.ISPName = res.ISP
		rec.ISPNameEN = res.ISP
		if rec.ISPName == "" {
			rec.ISPName = "شبکه اینترنت شما"
			rec.ISPNameEN = "Your Internet Network"
		}
		rec.ISPCategory = "other"
		rec.RecommendedProtocol = "VLESS Reality / WARP"
		rec.RecommendedFragment = "50-100 (10-20ms)"
		rec.FragmentSize = "50-100"
		rec.FragmentSleep = "10-20"
		rec.RecommendedDNS = "1.1.1.1"
		rec.Advice = "استفاده از پروتکل‌های استاندارد ضد فیلترینگ با فرگمنت پیشنهاد می‌شود."
		rec.AdviceEN = "Standard anti-censorship protocols with fragment packets are recommended for optimal bypass."
	}

	// The quality score is measured, never invented: it maps the observed
	// detection round-trip to a 0-100 responsiveness score.
	rec.StatusScore = computeStatusScore(detectionMs)

	return rec, nil
}

// computeStatusScore converts a measured round-trip time into a 0-100 score.
// <=80ms is treated as excellent, and anything above 600ms scores 0.
func computeStatusScore(rttMs int) int {
	switch {
	case rttMs <= 0:
		return 0
	case rttMs <= 80:
		return 100
	case rttMs >= 600:
		return 0
	default:
		// Linear decay from 100 at 80ms down to 0 at 600ms.
		return 100 - ((rttMs - 80) * 100 / 520)
	}
}

// undetectedRecommendation is returned when ISP detection genuinely failed.
// Detected=false lets the UI state "could not detect" instead of showing
// made-up operator data.
func undetectedRecommendation(reason string) *ISPRecommendation {
	return &ISPRecommendation{
		ISPName:             "تشخیص داده نشد",
		ISPNameEN:           "Not detected",
		ISPCategory:         "unknown",
		RecommendedProtocol: "VLESS Reality / WARP",
		RecommendedFragment: "50-100 (10-20ms)",
		FragmentSize:        "50-100",
		FragmentSleep:       "10-20",
		RecommendedDNS:      "1.1.1.1",
		StatusScore:         0,
		Detected:            false,
		Advice:              "تشخیص اپراتور ناموفق بود؛ می‌توانید دستی پروتکل و فرگمنت را انتخاب کنید. علت: " + reason,
		AdviceEN:            "Could not detect your operator; you can still choose a protocol and fragment manually. Reason: " + reason,
	}
}
