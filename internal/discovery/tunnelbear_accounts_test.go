package discovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempTunnelBearDir points the vault at a throwaway directory so the tests
// never touch the accounts a real user has saved.
func withTempTunnelBearDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	previous := tunnelBearDataDir
	tunnelBearDataDir = dir
	t.Cleanup(func() { tunnelBearDataDir = previous })
	return dir
}

func TestTunnelBearAccountLifecycle(t *testing.T) {
	withTempTunnelBearDir(t)

	if got := ListTunnelBearAccounts(); len(got) != 0 {
		t.Fatalf("a fresh vault should be empty, got %d", len(got))
	}

	id, err := SaveTunnelBearAccount("", "user@example.com", "s3cret")
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if id == "" {
		t.Fatal("save returned an empty id")
	}

	list := ListTunnelBearAccounts()
	if len(list) != 1 {
		t.Fatalf("expected one account, got %d", len(list))
	}
	if list[0].Username != "user@example.com" {
		t.Errorf("username not stored: %q", list[0].Username)
	}
	if list[0].Auth != TunnelBearAuthUnknown {
		t.Errorf("a new account should start unknown, got %q", list[0].Auth)
	}
	// The first account saved becomes the active one, so the servers tab works
	// immediately after adding a single account.
	if !list[0].Active {
		t.Error("the first saved account should be active")
	}

	gotID, username, password, err := ActiveTunnelBearCredentials()
	if err != nil {
		t.Fatalf("credentials failed: %v", err)
	}
	if gotID != id || username != "user@example.com" || password != "s3cret" {
		t.Errorf("credentials round-trip failed: %q %q %q", gotID, username, password)
	}

	if err := DeleteTunnelBearAccount(id); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if got := ListTunnelBearAccounts(); len(got) != 0 {
		t.Errorf("delete left %d accounts behind", len(got))
	}
	if _, _, _, err := ActiveTunnelBearCredentials(); err == nil {
		t.Error("credentials should fail once the only account is gone")
	}
}

// The whole point of the vault: the password is not in the list the UI reads,
// and it is not sitting in the JSON on disk either.
func TestTunnelBearPasswordIsNeverStoredInTheClear(t *testing.T) {
	dir := withTempTunnelBearDir(t)

	if _, err := SaveTunnelBearAccount("", "user@example.com", "hunter2-plaintext"); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("could not read the vault directory: %v", err)
	}
	for _, e := range entries {
		raw, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if readErr != nil {
			t.Fatalf("could not read %s: %v", e.Name(), readErr)
		}
		if strings.Contains(string(raw), "hunter2-plaintext") {
			t.Errorf("%s contains the password in the clear", e.Name())
		}
	}
}

// Editing the username must not require re-typing a password the app is already
// holding — the UI never has it to re-enter.
func TestSaveTunnelBearAccountKeepsThePasswordWhenItIsOmitted(t *testing.T) {
	withTempTunnelBearDir(t)

	id, err := SaveTunnelBearAccount("", "old@example.com", "original-password")
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if _, err := SaveTunnelBearAccount(id, "new@example.com", ""); err != nil {
		t.Fatalf("username-only edit failed: %v", err)
	}

	list := ListTunnelBearAccounts()
	if len(list) != 1 || list[0].Username != "new@example.com" {
		t.Fatalf("the username was not updated: %+v", list)
	}

	_, username, password, err := ActiveTunnelBearCredentials()
	if err != nil {
		t.Fatalf("credentials failed: %v", err)
	}
	if username != "new@example.com" {
		t.Errorf("username is %q", username)
	}
	if password != "original-password" {
		t.Errorf("the stored password was lost or replaced: %q", password)
	}
}

// "Leave the password blank to keep it" is only meaningful while a password is
// actually stored. Accepting a blank field against an empty vault would report
// success and leave an account that can never authenticate, with nothing on
// screen to explain it.
func TestSaveTunnelBearAccountRefusesToKeepAPasswordThatIsNotThere(t *testing.T) {
	dir := withTempTunnelBearDir(t)

	id, err := SaveTunnelBearAccount("", "user@example.com", "original-password")
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Simulate a vault that has been emptied out from under the account list.
	if err := os.WriteFile(filepath.Join(dir, tunnelBearVaultFile), []byte("{}"), 0o600); err != nil {
		t.Fatalf("could not empty the vault: %v", err)
	}

	if _, err := SaveTunnelBearAccount(id, "renamed@example.com", ""); err == nil {
		t.Fatal("a blank password was accepted against an empty vault")
	}

	// And the account list must not have been half-written by the attempt.
	list := ListTunnelBearAccounts()
	if len(list) != 1 || list[0].Username != "user@example.com" {
		t.Fatalf("the failed edit still changed the account list: %+v", list)
	}
}

func TestSaveTunnelBearAccountRotatesTheVaultReference(t *testing.T) {
	dir := withTempTunnelBearDir(t)

	id, err := SaveTunnelBearAccount("", "user@example.com", "first-password")
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	vaultPath := filepath.Join(dir, tunnelBearVaultFile)
	before, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatalf("could not read the vault: %v", err)
	}

	if _, err := SaveTunnelBearAccount(id, "user@example.com", "second-password"); err != nil {
		t.Fatalf("re-save failed: %v", err)
	}

	after, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatalf("could not read the vault: %v", err)
	}
	if string(before) == string(after) {
		t.Error("the vault reference was not rotated on edit, so a stale copy still resolves")
	}

	_, _, password, err := ActiveTunnelBearCredentials()
	if err != nil {
		t.Fatalf("credentials failed: %v", err)
	}
	if password != "second-password" {
		t.Errorf("the new password did not take effect: %q", password)
	}
}

// A newline in either field silently shifts the password into the username in
// OpenVPN's auth file, producing a failure that looks like a wrong password.
func TestSaveTunnelBearAccountRejectsLineBreaks(t *testing.T) {
	withTempTunnelBearDir(t)

	if _, err := SaveTunnelBearAccount("", "user\n@example.com", "password"); err == nil {
		t.Error("a username containing a newline should be rejected")
	}
	if _, err := SaveTunnelBearAccount("", "user@example.com", "pass\nword"); err == nil {
		t.Error("a password containing a newline should be rejected")
	}
	if _, err := SaveTunnelBearAccount("", "", "password"); err == nil {
		t.Error("an empty username should be rejected")
	}
	if _, err := SaveTunnelBearAccount("", "user@example.com", ""); err == nil {
		t.Error("a new account without a password should be rejected")
	}
}

func TestSetActiveTunnelBearAccount(t *testing.T) {
	withTempTunnelBearDir(t)

	first, err := SaveTunnelBearAccount("", "first@example.com", "one")
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	second, err := SaveTunnelBearAccount("", "second@example.com", "two")
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if err := SetActiveTunnelBearAccount(second); err != nil {
		t.Fatalf("set active failed: %v", err)
	}
	_, username, password, err := ActiveTunnelBearCredentials()
	if err != nil {
		t.Fatalf("credentials failed: %v", err)
	}
	if username != "second@example.com" || password != "two" {
		t.Errorf("the active account was not switched: %q %q", username, password)
	}

	for _, acc := range ListTunnelBearAccounts() {
		if acc.ID == second && !acc.Active {
			t.Error("the second account should be flagged active")
		}
		if acc.ID == first && acc.Active {
			t.Error("the first account should no longer be active")
		}
	}

	if err := SetActiveTunnelBearAccount("does-not-exist"); err == nil {
		t.Error("selecting an unknown account should fail")
	}
}

// Deleting the active account has to leave the vault in a usable state rather
// than pointing at something that no longer exists.
func TestDeletingTheActiveAccountReassigns(t *testing.T) {
	withTempTunnelBearDir(t)

	first, _ := SaveTunnelBearAccount("", "first@example.com", "one")
	second, _ := SaveTunnelBearAccount("", "second@example.com", "two")

	if err := SetActiveTunnelBearAccount(second); err != nil {
		t.Fatalf("set active failed: %v", err)
	}
	if err := DeleteTunnelBearAccount(second); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, username, password, err := ActiveTunnelBearCredentials()
	if err != nil {
		t.Fatalf("credentials failed after deleting the active account: %v", err)
	}
	if username != "first@example.com" || password != "one" {
		t.Errorf("expected the remaining account to take over, got %q %q", username, password)
	}
	_ = first
}

func TestNoteTunnelBearAuthResult(t *testing.T) {
	withTempTunnelBearDir(t)

	id, err := SaveTunnelBearAccount("", "user@example.com", "password")
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	NoteTunnelBearAuthResult(id, TunnelBearAuthFailed, "AUTH_FAILED: nope")
	list := ListTunnelBearAccounts()
	if list[0].Auth != TunnelBearAuthFailed {
		t.Errorf("auth should be failed, got %q", list[0].Auth)
	}
	if !strings.Contains(list[0].LastError, "AUTH_FAILED") {
		t.Errorf("the failure reason was not kept: %q", list[0].LastError)
	}

	NoteTunnelBearAuthResult(id, TunnelBearAuthOK, "")
	list = ListTunnelBearAccounts()
	if list[0].Auth != TunnelBearAuthOK {
		t.Errorf("auth should be ok, got %q", list[0].Auth)
	}
	// A success clears the stale error, so the badge cannot show "signed in"
	// with a failure message underneath it.
	if list[0].LastError != "" {
		t.Errorf("a successful sign-in should clear the old error, got %q", list[0].LastError)
	}
	if list[0].LastConnected == "" {
		t.Error("a successful sign-in should record when it happened")
	}
}

func TestTunnelBearSecretBackendIsReported(t *testing.T) {
	if TunnelBearSecretBackend() == "" {
		t.Error("the UI has nothing to display for the storage backend")
	}
}
