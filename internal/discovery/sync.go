package discovery

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"nexvpn/internal/config"
)

const (
	SyncBroadcastPort   = 10816
	maxSyncProfiles     = 500
	maxSyncPayloadBytes = 1 << 20
	SyncMagicHeader     = "NEXVPN_P2P_SYNC_V2:"
	syncSharedKeyPass   = "NexVPN-LAN-P2P-Secure-Key-v2"
)

type SyncPayload struct {
	DeviceName string           `json:"device_name"`
	Timestamp  int64            `json:"timestamp"`
	Profiles   []config.Profile `json:"profiles"`
}

func validSyncPayload(payload SyncPayload) bool {
	if strings.TrimSpace(payload.DeviceName) == "" || payload.Timestamp <= 0 || len(payload.Profiles) == 0 || len(payload.Profiles) > maxSyncProfiles {
		return false
	}
	// Reject obviously stale/replayed broadcasts.
	now := time.Now().Unix()
	return payload.Timestamp >= now-10*60 && payload.Timestamp <= now+60
}

func deriveSyncKey() []byte {
	k := sha256.Sum256([]byte(syncSharedKeyPass))
	return k[:]
}

// encryptSyncPayload encrypts payload bytes with AES-GCM
func encryptSyncPayload(data []byte) (string, error) {
	key := deriveSyncKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptSyncPayload decrypts an AES-GCM encrypted payload string
func decryptSyncPayload(b64Str string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64Str)
	if err != nil {
		return nil, err
	}
	key := deriveSyncKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return nil, fmt.Errorf("payload too short")
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// BroadcastProfilesToLAN broadcasts selected profiles securely to local network
func BroadcastProfilesToLAN(profiles []config.Profile, deviceName string) (int, error) {
	if len(profiles) == 0 {
		return 0, fmt.Errorf("no profiles to broadcast")
	}

	if deviceName == "" {
		deviceName = "NexVPN Node"
	}

	payload := SyncPayload{
		DeviceName: deviceName,
		Timestamp:  time.Now().Unix(),
		Profiles:   profiles,
	}

	if len(profiles) > maxSyncProfiles {
		return 0, fmt.Errorf("too many profiles to broadcast")
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	encPayload, err := encryptSyncPayload(bytes)
	if err != nil {
		return 0, err
	}

	msg := []byte(SyncMagicHeader + encPayload)

	// Broadcast address
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("255.255.255.255:%d", SyncBroadcastPort))
	if err != nil {
		return 0, err
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	// Send 3 broadcast bursts to overcome packet drops
	for i := 0; i < 3; i++ {
		_, _ = conn.Write(msg)
		time.Sleep(100 * time.Millisecond)
	}

	return len(profiles), nil
}

// ListenForNearbyProfiles listens for incoming broadcast for a specific duration
func ListenForNearbyProfiles(ctx context.Context, timeout time.Duration) (*SyncPayload, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("0.0.0.0:%d", SyncBroadcastPort))
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	buf := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			return nil, err
		}

		if n > maxSyncPayloadBytes {
			continue
		}
		data := string(buf[:n])
		if len(data) > len(SyncMagicHeader) && strings.HasPrefix(data, SyncMagicHeader) {
			encContent := data[len(SyncMagicHeader):]
			decrypted, err := decryptSyncPayload(encContent)
			if err == nil {
				var payload SyncPayload
				if err := json.Unmarshal(decrypted, &payload); err == nil && validSyncPayload(payload) {
					return &payload, nil
				}
			}
		}
	}
}
