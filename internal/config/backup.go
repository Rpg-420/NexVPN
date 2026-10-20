package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

const (
	backupMagicHeader = "NEXVPN_ENC_V1:"
	// pbkdf2Iterations targets the OWASP floor for PBKDF2-HMAC-SHA256
	// (600,000 as of 2023). It was 100,000, which is roughly 6x cheaper to
	// brute-force against a human-chosen backup password.
	pbkdf2Iterations = 600000
	saltLength       = 16
	nonceLength      = 12
)

// BackupPayload represents a complete export of all application state
type BackupPayload struct {
	Version   int                 `json:"version"`
	CreatedAt time.Time           `json:"created_at"`
	Profiles  []Profile           `json:"profiles"`
	Settings  Settings            `json:"settings"`
	History   []ConnectionSession `json:"history,omitempty"`
}

// BackupSummary summarizes the result of an import operation
type BackupSummary struct {
	ProfilesImported int       `json:"profiles_imported"`
	SettingsRestored bool      `json:"settings_restored"`
	CreatedAt        time.Time `json:"created_at"`
	IsEncrypted      bool      `json:"is_encrypted"`
}

// ExportBackup serializes and optionally encrypts all application data
func ExportBackup(store *Store, password string) (string, error) {
	if store == nil {
		return "", errors.New("store cannot be nil")
	}

	payload := BackupPayload{
		Version:   1,
		CreatedAt: time.Now().UTC(),
		Profiles:  store.GetProfiles(),
		Settings:  store.GetSettings(),
		History:   store.GetHistory(),
	}

	rawJSON, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal backup data: %w", err)
	}

	password = strings.TrimSpace(password)
	if password == "" {
		// Plain unencrypted JSON backup
		return string(rawJSON), nil
	}

	// Encrypted backup using AES-256-GCM + PBKDF2
	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("failed to generate random salt: %w", err)
	}

	key := pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to initialize GCM: %w", err)
	}

	nonce := make([]byte, nonceLength)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	sealed := gcm.Seal(nil, nonce, rawJSON, nil)

	// Combine salt + nonce + ciphertext
	var envelope []byte
	envelope = append(envelope, salt...)
	envelope = append(envelope, nonce...)
	envelope = append(envelope, sealed...)

	encoded := base64.StdEncoding.EncodeToString(envelope)
	return backupMagicHeader + encoded, nil
}

// ImportBackup deserializes, optionally decrypts, and restores data into the store
func ImportBackup(store *Store, rawContent, password string, overwriteSettings bool) (*BackupSummary, error) {
	if store == nil {
		return nil, errors.New("store cannot be nil")
	}

	rawContent = strings.TrimSpace(rawContent)
	if rawContent == "" {
		return nil, errors.New("backup content cannot be empty")
	}

	var jsonBytes []byte
	isEncrypted := false

	if strings.HasPrefix(rawContent, backupMagicHeader) {
		isEncrypted = true
		password = strings.TrimSpace(password)
		if password == "" {
			return nil, errors.New("password required to decrypt backup")
		}

		b64 := strings.TrimPrefix(rawContent, backupMagicHeader)
		envelope, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 backup envelope: %w", err)
		}

		if len(envelope) < saltLength+nonceLength+16 {
			return nil, errors.New("corrupted or truncated backup data")
		}

		salt := envelope[:saltLength]
		nonce := envelope[saltLength : saltLength+nonceLength]
		ciphertext := envelope[saltLength+nonceLength:]

		key := pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, 32, sha256.New)
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("failed to create AES cipher: %w", err)
		}

		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize GCM: %w", err)
		}

		plain, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return nil, errors.New("incorrect password or corrupted backup file")
		}
		jsonBytes = plain
	} else {
		jsonBytes = []byte(rawContent)
	}

	var payload BackupPayload
	if err := json.Unmarshal(jsonBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse backup JSON: %w", err)
	}

	// Restore profiles
	existingProfiles := store.GetProfiles()
	existingIDs := make(map[string]bool)
	for _, p := range existingProfiles {
		existingIDs[p.ID] = true
	}

	importedProfiles := 0
	for _, p := range payload.Profiles {
		if p.ID == "" {
			p.ID = fmt.Sprintf("prof_%d", time.Now().UnixNano())
		}
		if !existingIDs[p.ID] {
			_, _ = store.AddProfile(p)
			existingIDs[p.ID] = true
			importedProfiles++
		} else {
			// Update profile if existing
			_ = store.UpdateProfile(p)
			importedProfiles++
		}
	}

	// Optionally restore settings
	if overwriteSettings && payload.Settings.Language != "" {
		_ = store.SaveSettings(payload.Settings)
	}

	return &BackupSummary{
		ProfilesImported: importedProfiles,
		SettingsRestored: overwriteSettings,
		CreatedAt:        payload.CreatedAt,
		IsEncrypted:      isEncrypted,
	}, nil
}
