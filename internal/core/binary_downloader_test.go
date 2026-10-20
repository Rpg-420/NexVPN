package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
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

// sha256Hex returns the hex digest of data, the way the downloader computes it.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// sidecarServer serves body at "<url><suffix>" for every requested suffix,
// mimicking an upstream release page that publishes checksum sidecars.
func sidecarServer(t *testing.T, suffix, body string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if suffix != "" && strings.HasSuffix(r.URL.Path, suffix) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestVerifyDownloadedArchive_PinnedMatch(t *testing.T) {
	url := "https://example.test/core.zip"
	digest := sha256Hex([]byte("genuine payload"))
	pinnedCoreHashes[url] = digest
	defer delete(pinnedCoreHashes, url)

	if err := verifyDownloadedArchive([]byte("genuine payload"), url); err != nil {
		t.Fatalf("payload matching the pin must verify, got: %v", err)
	}
}

func TestVerifyDownloadedArchive_PinnedMismatchIsTampering(t *testing.T) {
	url := "https://example.test/core.zip"
	pinnedCoreHashes[url] = sha256Hex([]byte("genuine payload"))
	defer delete(pinnedCoreHashes, url)

	err := verifyDownloadedArchive([]byte("mirror-substituted payload"), url)
	if err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("payload differing from the pin must be refused, got: %v", err)
	}
}

func TestVerifyDownloadedArchive_SidecarMatches(t *testing.T) {
	digest := sha256Hex([]byte("new upstream release"))
	srv := sidecarServer(t, ".sha256", digest+"  core.zip\n")
	defer delete(pinnedCoreHashes, srv.URL+"/core.zip")

	if err := verifyDownloadedArchive([]byte("new upstream release"), srv.URL+"/core.zip"); err != nil {
		t.Fatalf("payload matching the upstream sidecar must verify, got: %v", err)
	}
}

func TestVerifyDownloadedArchive_SidecarXrayDgstFormat(t *testing.T) {
	// Xray publishes "<name>.zip.dgst" in openssl format:
	//   MD5= ...
	//   SHA1= ...
	//   SHA2-256= <hex>
	// The parser must find the SHA2-256 token, not reject fields[0] ("MD5=").
	digest := sha256Hex([]byte("xray release"))
	body := fmt.Sprintf("MD5= %s\nSHA1= %s\nSHA2-256= %s\n",
		strings.Repeat("a", 32), strings.Repeat("b", 40), digest)
	srv := sidecarServer(t, ".dgst", body)

	if err := verifyDownloadedArchive([]byte("xray release"), srv.URL+"/core.zip"); err != nil {
		t.Fatalf("Xray .dgst sidecar must verify the payload, got: %v", err)
	}
}

func TestVerifyDownloadedArchive_SidecarMismatchIsTampering(t *testing.T) {
	digest := sha256Hex([]byte("genuine payload"))
	srv := sidecarServer(t, ".sha256", digest+"  core.zip\n")

	err := verifyDownloadedArchive([]byte("substituted payload"), srv.URL+"/core.zip")
	if err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("payload differing from the sidecar must be refused, got: %v", err)
	}
}

func TestVerifyDownloadedArchive_SidecarTakesPrecedenceOverStalePin(t *testing.T) {
	// A "latest" URL's pin goes stale on every upstream release. The
	// upstream-published sidecar must let the genuine new release through.
	digest := sha256Hex([]byte("newer release"))
	srv := sidecarServer(t, ".sha256", digest+"  core.zip\n")
	pinnedCoreHashes[srv.URL+"/core.zip"] = sha256Hex([]byte("older release"))
	defer delete(pinnedCoreHashes, srv.URL+"/core.zip")

	if err := verifyDownloadedArchive([]byte("newer release"), srv.URL+"/core.zip"); err != nil {
		t.Fatalf("new upstream release verified by its own sidecar must pass despite a stale pin, got: %v", err)
	}
}

func TestVerifyDownloadedArchive_NoDigestAnywhereIsUnavailable(t *testing.T) {
	srv := sidecarServer(t, "", "")
	err := verifyDownloadedArchive([]byte("payload"), srv.URL+"/core.zip")
	if err != errChecksumUnavailable {
		t.Fatalf("no pin and no sidecar must yield errChecksumUnavailable, got: %v", err)
	}
}

func TestPinnedCoreHashes_PopulatedAndWellFormed(t *testing.T) {
	if len(pinnedCoreHashes) == 0 {
		t.Fatal("pinnedCoreHashes is empty: every shipped core must be pinned")
	}
	for url, digest := range pinnedCoreHashes {
		if !strings.HasPrefix(url, "https://github.com/") {
			t.Errorf("pin key %q is not a GitHub canonical URL", url)
		}
		if len(digest) != 64 || !isHexString(digest) {
			t.Errorf("pin for %q is not a 64-char hex digest: %q", url, digest)
		}
		if strings.Contains(url, "latest/download") {
			// Fine while the pin matches current upstream, but the entry must
			// stay honest: guard with a comment explaining staleness handling.
			t.Logf("pin for latest-URL %q will go stale on next upstream release (sidecar fallback covers it)", url)
		}
	}
}
