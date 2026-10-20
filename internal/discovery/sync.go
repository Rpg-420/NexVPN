package discovery

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"

	"nexvpn/internal/config"
)

const (
	SyncBroadcastPort   = 10816
	maxSyncProfiles     = 500
	maxSyncPayloadBytes = 1 << 20
	SyncMagicHeader     = "NEXVPN_P2P_SYNC_V2:"
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
	now := time.Now().Unix()
	return payload.Timestamp >= now-10*60 && payload.Timestamp <= now+60
}

// syncSalt is a domain-separation constant, not a per-message secret. The
// pairing PIN is what carries the entropy, so the KDF cost is what has to make
// brute force impractical (see syncKDFIterations).
var syncSalt = []byte("NexVPN-Nearby-P2P-Sync-Salt-v2")

// MinSyncPINLength is the minimum accepted pairing PIN length.
//
// SECURITY: raised from 4 to 6. A 4-digit PIN is only 10,000 candidates, which
// at a modest 4096 PBKDF2 iterations is exhaustible in seconds; a 6-digit PIN
// multiplies the work by 100 and, combined with syncKDFIterations below, puts a
// full sweep firmly out of reach for the short window a sync broadcast exists.
const MinSyncPINLength = 6

// syncKDFIterations is the PBKDF2-HMAC-SHA256 iteration count used to turn the
// pairing PIN into an AES-256 key.
//
// SECURITY: raised from 4096. OWASP's current floor for PBKDF2-HMAC-SHA256 is
// 600,000 iterations; we use that so an attacker who captures the broadcast
// cannot cheaply try the whole PIN keyspace offline.
const syncKDFIterations = 600000

// ErrSyncPINRequired is returned when a P2P sync operation is attempted without a PIN.
//
// SECURITY: this used to fall back to a hardcoded shared key
// ("NexVPN-Default-LAN-Shared-Key-2026") whenever the PIN was left empty, which made
// the AES-GCM payload effectively public - anyone on the same LAN who knew the
// constant could decrypt the broadcast and harvest proxy credentials and private
// keys. A PIN is now mandatory.
var ErrSyncPINRequired = fmt.Errorf("a pairing PIN of at least %d characters is required for LAN profile sharing", MinSyncPINLength)

// GenerateSyncPIN returns a random 6-digit pairing PIN.
func GenerateSyncPIN() string {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%06d", n.Int64()+100000)
}

func deriveSyncKey(pin string) ([]byte, error) {
	pass := strings.TrimSpace(pin)
	if len(pass) < MinSyncPINLength {
		return nil, ErrSyncPINRequired
	}
	return pbkdf2.Key([]byte(pass), syncSalt, syncKDFIterations, 32, sha256.New), nil
}

func encryptSyncPayload(data []byte, pin string) (string, error) {
	key, err := deriveSyncKey(pin)
	if err != nil {
		return "", err
	}
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

func decryptSyncPayload(b64Str string, pin string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64Str)
	if err != nil {
		return nil, err
	}
	key, err := deriveSyncKey(pin)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("payload too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func compressPayload(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decompressPayload(data []byte) ([]byte, error) {
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer func() { _ = zr.Close() }()
		return io.ReadAll(zr)
	}
	return data, nil
}

func BroadcastProfilesToLAN(profiles []config.Profile, deviceName string, pin string) (int, error) {
	if len(profiles) == 0 {
		return 0, fmt.Errorf("no profiles to broadcast")
	}
	if len(profiles) > maxSyncProfiles {
		return 0, fmt.Errorf("too many profiles to broadcast")
	}
	// Fail fast on a missing PIN instead of silently broadcasting with a public key.
	if strings.TrimSpace(pin) == "" || len(strings.TrimSpace(pin)) < MinSyncPINLength {
		return 0, ErrSyncPINRequired
	}
	// Limit batch to 50 profiles per broadcast to keep packet well below MTU
	if len(profiles) > 50 {
		profiles = profiles[:50]
	}
	if deviceName == "" {
		deviceName = "NexVPN Node"
	}
	payload := SyncPayload{DeviceName: deviceName, Timestamp: time.Now().Unix(), Profiles: profiles}
	data, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	// Compress payload to minimize UDP packet size
	if compressed, cErr := compressPayload(data); cErr == nil {
		data = compressed
	}

	encPayload, err := encryptSyncPayload(data, pin)
	if err != nil {
		return 0, err
	}
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("255.255.255.255:%d", SyncBroadcastPort))
	if err != nil {
		return 0, err
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close() }()
	msg := []byte(SyncMagicHeader + encPayload)
	for i := 0; i < 3; i++ {
		if _, err := conn.Write(msg); err != nil {
			return 0, err
		}
		if i < 2 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	return len(profiles), nil
}

func ListenForNearbyProfiles(ctx context.Context, timeout time.Duration, pin string) (*SyncPayload, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	// Fail fast on a missing PIN instead of listening for payloads we could never decrypt.
	if strings.TrimSpace(pin) == "" || len(strings.TrimSpace(pin)) < MinSyncPINLength {
		return nil, ErrSyncPINRequired
	}
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("0.0.0.0:%d", SyncBroadcastPort))
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
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
		if len(data) <= len(SyncMagicHeader) || !strings.HasPrefix(data, SyncMagicHeader) {
			continue
		}
		decrypted, err := decryptSyncPayload(data[len(SyncMagicHeader):], pin)
		if err != nil {
			continue
		}
		if decompressed, dErr := decompressPayload(decrypted); dErr == nil {
			decrypted = decompressed
		}
		var payload SyncPayload
		if err := json.Unmarshal(decrypted, &payload); err == nil && validSyncPayload(payload) {
			return &payload, nil
		}
	}
}
