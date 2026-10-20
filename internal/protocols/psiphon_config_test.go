package protocols

import (
	"encoding/base64"
	"strings"
	"testing"
)

// TestPsiphonConfigNeverSetsListenInterface pins the fix for the bug that made
// every Psiphon connection fail.
//
// psiphon-tunnel-core interprets "ListenInterface" as a network interface name
// and resolves it with common.GetInterfaceIPAddresses. The app used to set it to
// "127.0.0.1", which is not an interface, so the core aborted with
//
//	error getting listener IP: route ip+net: no such network interface
//
// roughly one second after start. Omitting the field is what makes the core bind
// its proxies to 127.0.0.1.
func TestPsiphonConfigNeverSetsListenInterface(t *testing.T) {
	cfg := PsiphonConfig{SocksPort: 10808, HttpPort: 10809}
	got := buildPsiphonCoreConfig(cfg, `C:\Users\x\AppData\Roaming\NexVPN\psiphon_data`)

	if v, ok := got["ListenInterface"]; ok {
		t.Fatalf("ListenInterface must not be set (got %#v); the core reads it as an interface name and exits", v)
	}

	// The proxies must still be pinned to the ports the rest of the app expects.
	if got["LocalSocksProxyPort"] != 10808 {
		t.Errorf("LocalSocksProxyPort = %v, want 10808", got["LocalSocksProxyPort"])
	}
	if got["LocalHttpProxyPort"] != 10809 {
		t.Errorf("LocalHttpProxyPort = %v, want 10809", got["LocalHttpProxyPort"])
	}
}

// TestPsiphonConfigBootstrapsItsOwnServerList guards the second half of the fix:
// the bundled core carries no server entries, so the config must tell it where to
// fetch them or it has nothing to dial.
func TestPsiphonConfigBootstrapsItsOwnServerList(t *testing.T) {
	got := buildPsiphonCoreConfig(PsiphonConfig{}, "/tmp/psi")

	if got["RemoteServerListSignaturePublicKey"] != psiphonRemoteServerListPublicKey {
		t.Error("RemoteServerListSignaturePublicKey is missing or wrong")
	}
	if got["RemoteServerListDownloadFilename"] != psiphonRemoteServerListFilename {
		t.Errorf("RemoteServerListDownloadFilename = %v, want %q",
			got["RemoteServerListDownloadFilename"], psiphonRemoteServerListFilename)
	}

	urls, ok := got["RemoteServerListURLs"].([]map[string]interface{})
	if !ok || len(urls) != 1 {
		t.Fatalf("RemoteServerListURLs = %#v, want exactly one entry", got["RemoteServerListURLs"])
	}
	entry := urls[0]

	// At least one entry must have OnlyAfterAttempts == 0, otherwise the core
	// never selects a URL on the first attempt (TransferURLs.Validate enforces
	// this).
	if entry["OnlyAfterAttempts"] != 0 {
		t.Errorf("OnlyAfterAttempts = %v, want 0", entry["OnlyAfterAttempts"])
	}

	// The URL is base64-obfuscated by design. A plain https:// string here is a
	// hard config error ("failed to decode URL: illegal base64 data"), so decode
	// it and assert the result is the expected absolute URL.
	encoded, _ := entry["URL"].(string)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("RemoteServerListURLs[0].URL is not valid base64: %v", err)
	}
	if !strings.HasPrefix(string(decoded), "https://") {
		t.Errorf("decoded server list URL = %q, want an https:// URL", decoded)
	}
}

// TestPsiphonConfigOptionalFields checks the two pass-through options.
func TestPsiphonConfigOptionalFields(t *testing.T) {
	// An unset or "AUTO" region means "let Psiphon choose", which is expressed by
	// leaving the key out rather than sending an empty string.
	for _, region := range []string{"", "AUTO"} {
		got := buildPsiphonCoreConfig(PsiphonConfig{EgressRegion: region}, "/tmp/psi")
		if _, ok := got["EgressRegion"]; ok {
			t.Errorf("EgressRegion %q should not be emitted, got %v", region, got["EgressRegion"])
		}
	}

	got := buildPsiphonCoreConfig(PsiphonConfig{EgressRegion: "de", TunnelProtocol: "OSSH"}, "/tmp/psi")
	if got["EgressRegion"] != "DE" {
		t.Errorf("EgressRegion = %v, want DE (upper-cased)", got["EgressRegion"])
	}
	if got["TunnelProtocol"] != "OSSH" {
		t.Errorf("TunnelProtocol = %v, want OSSH", got["TunnelProtocol"])
	}
}
