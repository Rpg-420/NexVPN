package discovery

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

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
	RecommendedDNS      string `json:"recommended_dns"`
	StatusScore         int    `json:"status_score"` // 0 - 100
	Advice              string `json:"advice"`
	AdviceEN            string `json:"advice_en"`
}

// DetectCurrentISP detects the ISP and returns custom-tailored bypass recommendations
func DetectCurrentISP(ctx context.Context) (*ISPRecommendation, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "http://ip-api.com/json/?fields=status,message,country,countryCode,isp,org,as,query", nil)
	if err != nil {
		return defaultRecommendation(), nil
	}

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return defaultRecommendation(), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return defaultRecommendation(), nil
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
		return defaultRecommendation(), nil
	}

	rawText := strings.ToLower(res.ISP + " " + res.Org + " " + res.AS)

	rec := &ISPRecommendation{
		IP:          res.Query,
		Country:     res.Country,
		CountryCode: res.CountryCode,
		ASN:         res.AS,
	}

	if strings.Contains(rawText, "mobile telecommunication") || strings.Contains(rawText, "mci") || strings.Contains(rawText, "as197207") {
		rec.ISPName = "همراه اول (MCI)"
		rec.ISPNameEN = "Hamrah-e Aval (MCI)"
		rec.ISPCategory = "mci"
		rec.RecommendedProtocol = "VLESS Reality (TCP + Vision)"
		rec.RecommendedFragment = "100-200 (10-20ms)"
		rec.RecommendedDNS = "1.1.1.1 (Cloudflare)"
		rec.StatusScore = 92
		rec.Advice = "پروتکل VLESS Reality با پورت 443 و فرگمنت بهترین پایداری را روی همراه اول دارد."
		rec.AdviceEN = "VLESS Reality protocol with port 443 and fragment provides the best stability on MCI."
	} else if strings.Contains(rawText, "irancell") || strings.Contains(rawText, "mtn") || strings.Contains(rawText, "as44244") {
		rec.ISPName = "ایرانسل (MTN Irancell)"
		rec.ISPNameEN = "MTN Irancell"
		rec.ISPCategory = "mtn"
		rec.RecommendedProtocol = "Hysteria 2 / Cloudflare WARP"
		rec.RecommendedFragment = "200-300 (15-25ms)"
		rec.RecommendedDNS = "8.8.8.8 (Google)"
		rec.StatusScore = 88
		rec.Advice = "پروتکل‌های مبتنی بر UDP (مانند Hysteria 2 و WARP on WARP) سرعت بالاتری روی ایرانسل ثبت می‌کنند."
		rec.AdviceEN = "UDP-based protocols like Hysteria 2 and WARP-on-WARP achieve highest speeds on Irancell."
	} else if strings.Contains(rawText, "rightel") || strings.Contains(rawText, "as57218") {
		rec.ISPName = "رایتل (Rightel)"
		rec.ISPNameEN = "Rightel"
		rec.ISPCategory = "rtl"
		rec.RecommendedProtocol = "VLESS Reality / Trojan TLS"
		rec.RecommendedFragment = "150-250 (10-15ms)"
		rec.RecommendedDNS = "1.1.1.1"
		rec.StatusScore = 90
		rec.Advice = "پروتکل‌های Reality و Trojan با پورت‌های استاندارد بدون قطعی عمل می‌کنند."
		rec.AdviceEN = "Reality and Trojan protocols with standard ports operate seamlessly on Rightel."
	} else if strings.Contains(rawText, "telecommunication company of iran") || strings.Contains(rawText, "tci") || strings.Contains(rawText, "shatel") || strings.Contains(rawText, "asiatech") || strings.Contains(rawText, "pars online") {
		rec.ISPName = "اینترنت ثابت (مخابرات / شاتل / آسیاتک)"
		rec.ISPNameEN = "Fixed Broadband (TCI / Shatel / Asiatech)"
		rec.ISPCategory = "fixed"
		rec.RecommendedProtocol = "Cloudflare WARP / VLESS gRPC"
		rec.RecommendedFragment = "50-150 (5-10ms)"
		rec.RecommendedDNS = "1.1.1.1 (Cloudflare DoH)"
		rec.StatusScore = 95
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
		rec.RecommendedFragment = "100-200 (10-20ms)"
		rec.RecommendedDNS = "1.1.1.1"
		rec.StatusScore = 85
		rec.Advice = "استفاده از پروتکل‌های استاندارد ضد فیلترینگ با فرگمنت پیشنهاد می‌شود."
		rec.AdviceEN = "Standard anti-censorship protocols with fragment packets are recommended for optimal bypass."
	}

	return rec, nil
}

func defaultRecommendation() *ISPRecommendation {
	return &ISPRecommendation{
		ISPName:             "تشخیص خودکار شبکه",
		ISPNameEN:           "Network Auto-Detection",
		ISPCategory:         "other",
		RecommendedProtocol: "VLESS Reality / WARP",
		RecommendedFragment: "100-200 (10-20ms)",
		RecommendedDNS:      "1.1.1.1",
		StatusScore:         88,
		Advice:              "اتصال به سرورهای VLESS Reality و Cloudflare WARP با آی‌پی تمیز پیشنهاد می‌شود.",
		AdviceEN:            "Connecting to VLESS Reality and Cloudflare WARP with Clean IP is recommended.",
	}
}
