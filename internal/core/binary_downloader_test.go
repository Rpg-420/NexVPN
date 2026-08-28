package core

import (
	"strings"
	"testing"
)

func TestGetCoreDownloadURL_NotEmpty(t *testing.T) {
	keys := []string{"xray", "singbox", "geoip", "geosite", "aether"}
	for _, k := range keys {
		url := GetCoreDownloadURL(k)
		if url == "" {
			t.Errorf("GetCoreDownloadURL(%q) returned empty", k)
		}
		if !strings.Contains(url, "https://") {
			t.Errorf("GetCoreDownloadURL(%q) missing https: %q", k, url)
		}
		// Should contain mirrors
		if !strings.Contains(url, ",") {
			t.Errorf("GetCoreDownloadURL(%q) should contain mirrors (comma-separated), got %q", k, url)
		}
	}
}

func TestGetCoreDownloadURL_PsiphonWindows(t *testing.T) {
	// On windows psiphon should be bundled (empty URL is expected)
	// This test just ensures function doesn't panic
	_ = GetCoreDownloadURL("psiphon")
	_ = GetCoreDownloadURL("unknown-key")
}

func TestExeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"xray", "xray.exe"},
		{"geoip", "geoip.dat"},
		{"geosite", "geosite.dat"},
	}
	for _, tt := range tests {
		got := exeName(tt.input)
		if got != tt.want {
			// On non-windows exeName returns without .exe, so allow both
			if tt.want == "xray.exe" && got == "xray" {
				continue
			}
			t.Errorf("exeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGithubMirrors(t *testing.T) {
	primary := "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-windows-64.zip"
	mirrored := githubMirrors(primary)
	if !strings.Contains(mirrored, primary) {
		t.Error("mirrored should contain primary URL")
	}
	if !strings.Contains(mirrored, "ghproxy.net") {
		t.Error("mirrored should contain ghproxy mirror")
	}
	parts := strings.Split(mirrored, ",")
	if len(parts) < 3 {
		t.Errorf("expected at least 3 mirrors, got %d", len(parts))
	}
}
