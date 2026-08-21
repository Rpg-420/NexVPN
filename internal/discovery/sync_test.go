package discovery

import (
	"testing"
	"time"

	"nexvpn/internal/config"
)

func TestP2PPayloadEncryptionDecryption(t *testing.T) {
	sampleProfiles := []config.Profile{
		{
			ID:       "test-p2p-1",
			Name:     "Test Server DE",
			Protocol: config.ProtocolSingBox,
			Category: config.CategoryV2Ray,
			Server:   "1.2.3.4",
			Port:     443,
			RawURI:   "vless://uuid-test@1.2.3.4:443?security=reality",
		},
	}

	payload := SyncPayload{
		DeviceName: "My-PC",
		Timestamp:  time.Now().Unix(),
		Profiles:   sampleProfiles,
	}

	data, err := encryptSyncPayload([]byte(`{"test":"hello"}`))
	if err != nil {
		t.Fatalf("Failed to encrypt payload: %v", err)
	}

	decrypted, err := decryptSyncPayload(data)
	if err != nil {
		t.Fatalf("Failed to decrypt payload: %v", err)
	}

	if string(decrypted) != `{"test":"hello"}` {
		t.Fatalf("Decrypted content mismatch: got %s, want %s", string(decrypted), `{"test":"hello"}`)
	}
	_ = payload
}
