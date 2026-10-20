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

	// 1. Empty PIN must be rejected (regression guard).
	//    Previously an empty PIN silently fell back to a hardcoded, publicly known
	//    shared key, which made the AES-GCM payload readable by anyone on the LAN.
	if _, err := encryptSyncPayload([]byte(`{"test":"hello"}`), ""); err == nil {
		t.Fatalf("Expected an error when encrypting with an empty PIN, but it succeeded")
	}
	if _, err := decryptSyncPayload("AAAA", ""); err == nil {
		t.Fatalf("Expected an error when decrypting with an empty PIN, but it succeeded")
	}

	// 2. A PIN shorter than MinSyncPINLength must also be rejected.
	if _, err := encryptSyncPayload([]byte(`{"test":"hello"}`), "123"); err == nil {
		t.Fatalf("Expected an error when encrypting with a too-short PIN, but it succeeded")
	}

	// 3. Custom PIN test (Device A to Device B with a matching PIN).
	//    The PIN must satisfy MinSyncPINLength (6), so the historical 4-digit
	//    "8492" is no longer accepted by encryptSyncPayload.
	pin := "849213"
	dataWithPIN, err := encryptSyncPayload([]byte(`{"test":"secret-node"}`), pin)
	if err != nil {
		t.Fatalf("Failed to encrypt payload with PIN: %v", err)
	}

	decryptedWithPIN, err := decryptSyncPayload(dataWithPIN, pin)
	if err != nil {
		t.Fatalf("Failed to decrypt payload with matching PIN: %v", err)
	}

	if string(decryptedWithPIN) != `{"test":"secret-node"}` {
		t.Fatalf("Decrypted content with PIN mismatch: got %s", string(decryptedWithPIN))
	}

	// 4. Wrong PIN should fail decryption
	_, err = decryptSyncPayload(dataWithPIN, "wrong-pin")
	if err == nil {
		t.Fatalf("Expected decryption failure with wrong PIN, but succeeded")
	}

	// 5. GenerateSyncPIN must produce a 6-digit PIN
	if generated := GenerateSyncPIN(); len(generated) != 6 {
		t.Fatalf("GenerateSyncPIN returned %q, expected 6 digits", generated)
	}

	_ = payload
}
