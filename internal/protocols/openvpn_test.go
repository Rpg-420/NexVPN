package protocols

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSanitizeOpenVPNConfig covers the config-normalisation step that runs
// before OpenVPN is launched. It guards three behaviours that previously caused
// WINTUN_IN_USE failures or silently broken profiles.
func TestSanitizeOpenVPNConfig(t *testing.T) {
	t.Run("strips dev-node and windows-driver", func(t *testing.T) {
		in := "client\ndev-node MyOtherVPN\nwindows-driver wintun\nremote example.com 1194\n"
		out := sanitizeOpenVPNConfig(in)
		if strings.Contains(out, "dev-node") {
			t.Errorf("dev-node line was not removed: %q", out)
		}
		if strings.Contains(out, "windows-driver") {
			t.Errorf("windows-driver line was not removed: %q", out)
		}
		if !strings.Contains(out, "remote example.com 1194") {
			t.Errorf("unrelated lines must be preserved: %q", out)
		}
	})

	t.Run("drops explicit-exit-notify on TCP", func(t *testing.T) {
		in := "client\nproto tcp\nexplicit-exit-notify 3\nremote example.com 443\n"
		out := sanitizeOpenVPNConfig(in)
		if strings.Contains(out, "explicit-exit-notify") {
			t.Errorf("explicit-exit-notify must be dropped for TCP: %q", out)
		}
	})

	t.Run("keeps explicit-exit-notify on UDP", func(t *testing.T) {
		in := "client\nproto udp\nexplicit-exit-notify 3\nremote example.com 1194\n"
		out := sanitizeOpenVPNConfig(in)
		if !strings.Contains(out, "explicit-exit-notify 3") {
			t.Errorf("explicit-exit-notify must be kept for UDP: %q", out)
		}
	})

	t.Run("balances unclosed quotes", func(t *testing.T) {
		in := "client\nca \"C:\\certs\\ca.crt\nremote example.com 1194\n"
		out := sanitizeOpenVPNConfig(in)
		if strings.Count(out, "\"")%2 != 0 {
			t.Errorf("quotes must be balanced after sanitisation: %q", out)
		}
	})

	t.Run("drops blank lines and normalises CRLF", func(t *testing.T) {
		in := "client\r\n\r\nremote example.com 1194\r\n"
		out := sanitizeOpenVPNConfig(in)
		if strings.Contains(out, "\r") {
			t.Errorf("carriage returns must be removed: %q", out)
		}
		for _, line := range strings.Split(out, "\n") {
			if strings.TrimSpace(line) == "" {
				t.Errorf("blank lines must be dropped: %q", out)
			}
		}
	})
}

// TestTempFilesSurviveTheDriverFallback pins the contract the Wintun→TAP
// fallback depends on: tearing the failed process down must not delete the
// config OpenVPN is about to be restarted with.
//
// The regression this guards is silent. `Start()` used to call `a.Stop()` before
// the retry, and `launchDriverInstance` calls it internally on every failure
// path too — both of which remove tmpConfig. The retry then ran OpenVPN against
// a path that no longer existed and failed with
// `Options error: Error opening configuration file: ...\openvpn-NNNN.ovpn`,
// which reads like a bad profile rather than a self-inflicted deletion. Every
// TAP fallback therefore failed, so OpenVPN could never connect on a machine
// whose Wintun adapter was busy.
func TestTempFilesSurviveTheDriverFallback(t *testing.T) {
	writeTemp := func(t *testing.T, dir string) (string, string) {
		t.Helper()
		cfg, err := os.CreateTemp(dir, "openvpn-*.ovpn")
		if err != nil {
			t.Fatalf("create temp config: %v", err)
		}
		_, _ = cfg.WriteString("client\nremote example.com 1194\n")
		_ = cfg.Close()
		creds, err := os.CreateTemp(dir, "openvpn-creds-*.txt")
		if err != nil {
			t.Fatalf("create temp creds: %v", err)
		}
		_, _ = creds.WriteString("user\npass\n")
		_ = creds.Close()
		return cfg.Name(), creds.Name()
	}

	t.Run("stopInternal(false) keeps the files for the retry", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath, credsPath := writeTemp(t, dir)

		a := NewOpenVPNAdapter()
		a.mu.Lock()
		a.tmpConfig = cfgPath
		a.tmpCreds = credsPath
		a.mu.Unlock()

		if err := a.stopInternal(false); err != nil {
			t.Fatalf("stopInternal(false): %v", err)
		}
		for _, p := range []string{cfgPath, credsPath} {
			if _, err := os.Stat(p); err != nil {
				t.Errorf("retry needs %s to still exist, got: %v", filepath.Base(p), err)
			}
		}

		// The paths must be forgotten even though the files survive, or a later
		// Stop() would delete a file a retry has already replaced.
		a.mu.RLock()
		gotConfig := a.tmpConfig
		gotCreds := a.tmpCreds
		a.mu.RUnlock()
		if gotConfig != "" || gotCreds != "" {
			t.Errorf("temp paths must be cleared after teardown (config=%q creds=%q)", gotConfig, gotCreds)
		}
	})

	t.Run("Stop removes the files", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath, credsPath := writeTemp(t, dir)

		a := NewOpenVPNAdapter()
		a.mu.Lock()
		a.tmpConfig = cfgPath
		a.tmpCreds = credsPath
		a.mu.Unlock()

		if err := a.Stop(); err != nil {
			t.Fatalf("Stop: %v", err)
		}
		for _, p := range []string{cfgPath, credsPath} {
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Errorf("%s must be deleted by Stop, stat err = %v", filepath.Base(p), err)
			}
		}
	})

	t.Run("a second teardown is a no-op", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath, _ := writeTemp(t, dir)

		a := NewOpenVPNAdapter()
		a.mu.Lock()
		a.tmpConfig = cfgPath
		a.mu.Unlock()

		if err := a.stopInternal(false); err != nil {
			t.Fatalf("first stopInternal: %v", err)
		}
		// After the fallback has failed, Start() removes the files itself. A
		// later Stop() must not blow up on the now-empty path.
		if err := a.Stop(); err != nil {
			t.Fatalf("second Stop: %v", err)
		}
		if _, err := os.Stat(cfgPath); err != nil {
			t.Errorf("file should still be present (Stop had no path to remove): %v", err)
		}
	})
}
