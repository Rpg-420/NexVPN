package config

import (
	"os"
	"testing"
	"time"
)

func TestBackupExportAndImportPlain(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nexvpn-backup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store := NewStoreWithDir(tmpDir)

	testProfile := Profile{
		ID:        "prof_test_1",
		Name:      "Test Server Frankfurt",
		Protocol:  ProtocolSingBox,
		Server:    "198.51.100.1",
		Port:      443,
		CreatedAt: time.Now(),
	}
	_, _ = store.AddProfile(testProfile)

	exported, err := ExportBackup(store, "")
	if err != nil {
		t.Fatalf("ExportBackup plain failed: %v", err)
	}

	// Create a new store to import into
	tmpDir2, _ := os.MkdirTemp("", "nexvpn-backup-test2-*")
	defer func() { _ = os.RemoveAll(tmpDir2) }()
	store2 := NewStoreWithDir(tmpDir2)

	summary, err := ImportBackup(store2, exported, "", true)
	if err != nil {
		t.Fatalf("ImportBackup plain failed: %v", err)
	}

	if summary.ProfilesImported != 1 {
		t.Errorf("Expected 1 profile imported, got %d", summary.ProfilesImported)
	}
	if summary.IsEncrypted {
		t.Errorf("Expected unencrypted backup, marked as encrypted")
	}

	profiles := store2.GetProfiles()
	if len(profiles) != 1 || profiles[0].Name != "Test Server Frankfurt" {
		t.Errorf("Profile verification failed: %+v", profiles)
	}
}

func TestBackupExportAndImportEncrypted(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nexvpn-backup-enc-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store := NewStoreWithDir(tmpDir)

	testProfile := Profile{
		ID:       "prof_secure_1",
		Name:     "Secure Trojan V2",
		Protocol: ProtocolSingBox,
		Server:   "203.0.113.5",
		Port:     8443,
	}
	_, _ = store.AddProfile(testProfile)

	password := "SecretMasterPassword123!"
	exported, err := ExportBackup(store, password)
	if err != nil {
		t.Fatalf("ExportBackup encrypted failed: %v", err)
	}

	// Verify encryption magic header
	if len(exported) < 20 || exported[:len(backupMagicHeader)] != backupMagicHeader {
		t.Fatalf("Exported string does not start with magic header: %s", exported)
	}

	// 1. Test decryption with wrong password (must fail)
	tmpDirWrong, _ := os.MkdirTemp("", "nexvpn-backup-wrong-*")
	defer os.RemoveAll(tmpDirWrong)
	storeWrong := NewStoreWithDir(tmpDirWrong)
	_, err = ImportBackup(storeWrong, exported, "WrongPassword", false)
	if err == nil {
		t.Fatalf("Expected import with wrong password to fail, but succeeded")
	}

	// 2. Test decryption with correct password (must succeed)
	tmpDir2, _ := os.MkdirTemp("", "nexvpn-backup-enc2-*")
	defer os.RemoveAll(tmpDir2)
	store2 := NewStoreWithDir(tmpDir2)
	summary, err := ImportBackup(store2, exported, password, true)
	if err != nil {
		t.Fatalf("ImportBackup encrypted failed with correct password: %v", err)
	}

	if summary.ProfilesImported != 1 {
		t.Errorf("Expected 1 profile imported, got %d", summary.ProfilesImported)
	}
	if !summary.IsEncrypted {
		t.Errorf("Expected IsEncrypted to be true")
	}

	profiles := store2.GetProfiles()
	if len(profiles) != 1 || profiles[0].Name != "Secure Trojan V2" {
		t.Errorf("Profile verification failed: %+v", profiles)
	}
}
