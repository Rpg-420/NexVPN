package discovery

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/secret"
)

// TunnelBear accounts — the user's OWN logins, stored apart from the catalogue.
//
// The username lives in a plain JSON list; the password never does. It is
// sealed by internal/secret (Windows DPAPI under the current user, or AES-GCM
// under a 0600 key file elsewhere) and referenced from the list by a random
// handle. The handle is regenerated on every edit, so a stale reference left
// behind in a backup, a log, or an old copy of the list resolves to nothing
// rather than to a live credential.
//
// Nothing in this file returns a password to the caller. The one function that
// reads one — ActiveTunnelBearCredentials — is called by the connect path and
// its result goes straight into the temp auth file OpenVPN is handed.

const (
	tunnelBearAccountsFile = "tunnelbear_accounts.json"
	tunnelBearVaultFile    = "tunnelbear_vault.json"
)

// Auth outcomes, as reported by the core.
const (
	TunnelBearAuthUnknown = "unknown"
	TunnelBearAuthOK      = "ok"
	TunnelBearAuthFailed  = "failed"
)

// tunnelBearDataDir overrides the storage directory. Empty means the real app
// data directory; tests point it at a temp dir so they never touch the user's
// saved accounts.
var tunnelBearDataDir string

func tunnelBearDir() string {
	if tunnelBearDataDir != "" {
		return tunnelBearDataDir
	}
	return config.DefaultDataDir()
}

func tunnelBearAccountsPath() string { return filepath.Join(tunnelBearDir(), tunnelBearAccountsFile) }
func tunnelBearVaultPath() string    { return filepath.Join(tunnelBearDir(), tunnelBearVaultFile) }

// TunnelBearAccount is one stored login. It is the on-disk shape.
type TunnelBearAccount struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	// CredRef is the handle into the vault, never the password itself.
	CredRef       string `json:"cred_ref"`
	Auth          string `json:"auth"`
	LastError     string `json:"last_error,omitempty"`
	LastConnected string `json:"last_connected,omitempty"`
	AddedAt       int64  `json:"added_at"`
}

// TunnelBearAccountInfo is what the UI is allowed to see. CredRef is absent by
// construction rather than by omission at the call site.
type TunnelBearAccountInfo struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Auth          string `json:"auth"`
	LastError     string `json:"last_error,omitempty"`
	LastConnected string `json:"last_connected,omitempty"`
	Active        bool   `json:"active"`
}

type tunnelBearAccountsData struct {
	List     []TunnelBearAccount `json:"list"`
	ActiveID string              `json:"active_id"`
}

var tunnelBearStoreMu sync.Mutex

func loadTunnelBearAccounts() tunnelBearAccountsData {
	var data tunnelBearAccountsData
	raw, err := os.ReadFile(tunnelBearAccountsPath())
	if err != nil {
		return data
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		// A corrupt list is not worth losing the app over; the user re-adds an
		// account. Say so rather than silently presenting an empty list.
		return tunnelBearAccountsData{}
	}
	return data
}

func saveTunnelBearAccounts(data tunnelBearAccountsData) error {
	dir := tunnelBearDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("could not create the data directory: %w", err)
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tunnelBearAccountsPath(), raw, 0o600)
}

func loadTunnelBearVault() map[string]string {
	vault := map[string]string{}
	raw, err := os.ReadFile(tunnelBearVaultPath())
	if err != nil {
		return vault
	}
	_ = json.Unmarshal(raw, &vault)
	return vault
}

func saveTunnelBearVault(vault map[string]string) error {
	dir := tunnelBearDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(vault, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tunnelBearVaultPath(), raw, 0o600)
}

func newTunnelBearRef() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("could not generate a credential reference: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// validateTunnelBearUsername rejects what would corrupt the auth file.
//
// OpenVPN reads the credentials file as one line of username followed by one
// line of password, so a newline in either field silently shifts the password
// into the username and produces an authentication failure that looks like a
// wrong password. Refusing the input is far more useful than passing it on.
func validateTunnelBearUsername(username string) error {
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("a TunnelBear username is required")
	}
	if len(username) > 320 {
		return fmt.Errorf("the username is too long (maximum 320 characters)")
	}
	if strings.ContainsAny(username, "\r\n\x00") {
		return fmt.Errorf("the username cannot contain a line break")
	}
	return nil
}

func validateTunnelBearPassword(password string) error {
	if password == "" {
		return fmt.Errorf("a TunnelBear password is required")
	}
	if len(password) > 4096 {
		return fmt.Errorf("the password is too long (maximum 4096 characters)")
	}
	if strings.ContainsAny(password, "\r\n\x00") {
		return fmt.Errorf("the password cannot contain a line break")
	}
	return nil
}

// ListTunnelBearAccounts returns every saved account, without secrets.
func ListTunnelBearAccounts() []TunnelBearAccountInfo {
	tunnelBearStoreMu.Lock()
	defer tunnelBearStoreMu.Unlock()

	data := loadTunnelBearAccounts()
	out := make([]TunnelBearAccountInfo, 0, len(data.List))
	for _, acc := range data.List {
		auth := acc.Auth
		if auth == "" {
			auth = TunnelBearAuthUnknown
		}
		out = append(out, TunnelBearAccountInfo{
			ID:            acc.ID,
			Username:      acc.Username,
			Auth:          auth,
			LastError:     acc.LastError,
			LastConnected: acc.LastConnected,
			Active:        acc.ID == data.ActiveID,
		})
	}
	return out
}

// SaveTunnelBearAccount adds an account, or updates one when id is non-empty.
// It returns the account's ID.
//
// When id is non-empty and password is empty, only the username changes and the
// stored password is kept. The password is never handed back to the UI, so a
// form that has nothing in the field must not be read as "clear the password" —
// that would force the user to re-type a secret the app is already holding, and
// would make a routine username fix impossible without it.
func SaveTunnelBearAccount(id, username, password string) (string, error) {
	if err := validateTunnelBearUsername(username); err != nil {
		return "", err
	}
	keepPassword := id != "" && password == ""
	if !keepPassword {
		if err := validateTunnelBearPassword(password); err != nil {
			return "", err
		}
	}

	tunnelBearStoreMu.Lock()
	defer tunnelBearStoreMu.Unlock()

	username = strings.TrimSpace(username)
	data := loadTunnelBearAccounts()
	vault := loadTunnelBearVault()

	if keepPassword {
		for i := range data.List {
			if data.List[i].ID != id {
				continue
			}
			// "Keep the stored password" is only a real option while there is
			// one to keep. Accepting an empty field when the vault has no entry
			// would report success and leave an account that cannot connect,
			// with nothing on screen to explain why.
			if _, ok := vault[data.List[i].CredRef]; !ok {
				return "", fmt.Errorf("there is no saved password for %q to keep — enter it again", data.List[i].Username)
			}
			data.List[i].Username = username
			data.List[i].Auth = TunnelBearAuthUnknown
			data.List[i].LastError = ""
			return id, saveTunnelBearAccounts(data)
		}
		return "", fmt.Errorf("that TunnelBear account no longer exists")
	}

	sealed, err := secret.Seal(password)
	if err != nil {
		return "", fmt.Errorf("could not store the password securely: %w", err)
	}

	if id != "" {
		found := false
		for i := range data.List {
			if data.List[i].ID != id {
				continue
			}
			found = true
			// Drop the old handle so an old reference can never resolve.
			delete(vault, data.List[i].CredRef)
			ref, err := newTunnelBearRef()
			if err != nil {
				return "", err
			}
			data.List[i].CredRef = ref
			data.List[i].Username = username
			data.List[i].Auth = TunnelBearAuthUnknown
			data.List[i].LastError = ""
			vault[ref] = sealed
			break
		}
		if !found {
			return "", fmt.Errorf("that TunnelBear account no longer exists")
		}
	} else {
		ref, err := newTunnelBearRef()
		if err != nil {
			return "", err
		}
		newID, err := newTunnelBearRef()
		if err != nil {
			return "", err
		}
		data.List = append(data.List, TunnelBearAccount{
			ID:       "tb_" + newID[:12],
			Username: username,
			CredRef:  ref,
			Auth:     TunnelBearAuthUnknown,
			AddedAt:  time.Now().Unix(),
		})
		vault[ref] = sealed
		id = data.List[len(data.List)-1].ID
		if data.ActiveID == "" {
			data.ActiveID = id
		}
	}

	if err := saveTunnelBearVault(vault); err != nil {
		return "", fmt.Errorf("could not write the credential vault: %w", err)
	}
	if err := saveTunnelBearAccounts(data); err != nil {
		return "", err
	}
	return id, nil
}

// DeleteTunnelBearAccount removes an account and its sealed password.
func DeleteTunnelBearAccount(id string) error {
	tunnelBearStoreMu.Lock()
	defer tunnelBearStoreMu.Unlock()

	data := loadTunnelBearAccounts()
	vault := loadTunnelBearVault()

	next := make([]TunnelBearAccount, 0, len(data.List))
	removed := false
	for _, acc := range data.List {
		if acc.ID == id {
			delete(vault, acc.CredRef)
			removed = true
			continue
		}
		next = append(next, acc)
	}
	if !removed {
		return fmt.Errorf("that TunnelBear account no longer exists")
	}

	data.List = next
	if data.ActiveID == id {
		data.ActiveID = ""
		if len(next) > 0 {
			data.ActiveID = next[0].ID
		}
	}

	if err := saveTunnelBearVault(vault); err != nil {
		return err
	}
	return saveTunnelBearAccounts(data)
}

// SetActiveTunnelBearAccount chooses the account new connections sign in with.
// An empty id clears the selection.
func SetActiveTunnelBearAccount(id string) error {
	tunnelBearStoreMu.Lock()
	defer tunnelBearStoreMu.Unlock()

	data := loadTunnelBearAccounts()
	if id != "" {
		found := false
		for _, acc := range data.List {
			if acc.ID == id {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("that TunnelBear account no longer exists")
		}
	}
	data.ActiveID = id
	return saveTunnelBearAccounts(data)
}

// ActiveTunnelBearCredentials returns the account a connection should sign in
// with. This is the only function that hands a password back, and its caller
// writes it straight into OpenVPN's temporary auth file.
func ActiveTunnelBearCredentials() (id, username, password string, err error) {
	tunnelBearStoreMu.Lock()
	defer tunnelBearStoreMu.Unlock()

	data := loadTunnelBearAccounts()
	if len(data.List) == 0 {
		return "", "", "", fmt.Errorf("no TunnelBear account has been saved yet — add one first")
	}

	var chosen *TunnelBearAccount
	for i := range data.List {
		if data.List[i].ID == data.ActiveID {
			chosen = &data.List[i]
			break
		}
	}
	if chosen == nil {
		chosen = &data.List[0]
	}

	vault := loadTunnelBearVault()
	sealed, ok := vault[chosen.CredRef]
	if !ok {
		return "", "", "", fmt.Errorf("the saved password for %q is missing — open the TunnelBear section and enter it again", chosen.Username)
	}
	plain, err := secret.Open(sealed)
	if err != nil {
		return "", "", "", fmt.Errorf("the saved password for %q could not be read: %w", chosen.Username, err)
	}
	return chosen.ID, chosen.Username, plain, nil
}

// NoteTunnelBearAuthResult records what the core said about the last attempt,
// so the account list can show "signed in" or the failure without the UI having
// to infer it from a connection state.
func NoteTunnelBearAuthResult(id, auth, errMsg string) {
	if id == "" {
		return
	}
	tunnelBearStoreMu.Lock()
	defer tunnelBearStoreMu.Unlock()

	data := loadTunnelBearAccounts()
	for i := range data.List {
		if data.List[i].ID != id {
			continue
		}
		data.List[i].Auth = auth
		data.List[i].LastError = errMsg
		if auth == TunnelBearAuthOK {
			data.List[i].LastConnected = time.Now().Format(time.RFC3339)
			data.List[i].LastError = ""
		}
		break
	}
	_ = saveTunnelBearAccounts(data)
}

// TunnelBearSecretBackend names the mechanism passwords are sealed with, so the
// UI can be honest about it instead of implying every platform is equal.
func TunnelBearSecretBackend() string { return secret.Backend() }
