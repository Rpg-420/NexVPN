//go:build !windows

package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Non-Windows fallback: AES-256-GCM under a random key held in a 0600 file in
// the app's data directory.
//
// This is honest about what it is. On Windows the key is derived by DPAPI from
// the logged-in user's credentials and never exists on disk at all; here it is
// a file that any process able to read the user's home directory can read too.
// It is still strictly better than writing the password in clear — a copied
// config file, a backup, or a log does not leak it — and on a single-user
// desktop the practical difference is small. It is not equivalent, and the UI
// reports which backend is in use rather than implying otherwise.

const keyFileName = "secret.key"

var (
	keyOnce sync.Once
	key     []byte
	keyErr  error
)

func keyPath() string {
	home, _ := os.UserConfigDir()
	if home == "" {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, "NexVPN", keyFileName)
}

func loadKey() ([]byte, error) {
	keyOnce.Do(func() {
		p := keyPath()
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			keyErr = fmt.Errorf("could not create the key directory: %w", err)
			return
		}

		if existing, err := os.ReadFile(p); err == nil && len(existing) == 32 {
			key = existing
			return
		}

		fresh := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, fresh); err != nil {
			keyErr = fmt.Errorf("could not generate a key: %w", err)
			return
		}
		if err := os.WriteFile(p, fresh, 0o600); err != nil {
			keyErr = fmt.Errorf("could not store the key: %w", err)
			return
		}
		key = fresh
	})
	return key, keyErr
}

// Seal encrypts plaintext for the current user.
func Seal(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}

	k, err := loadKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(k)
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

	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Open reverses Seal.
func Open(sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}

	k, err := loadKey()
	if err != nil {
		return "", err
	}

	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", fmt.Errorf("sealed value is not valid base64: %w", err)
	}

	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("sealed value is too short to contain a nonce")
	}

	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("could not decrypt (the key file has changed since this value was saved): %w", err)
	}
	return string(plain), nil
}

// Backend names the mechanism, for the diagnostic report and the UI.
func Backend() string { return "AES-256-GCM (key file, user-only permissions)" }
