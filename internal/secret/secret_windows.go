//go:build windows

package secret

import (
	"encoding/base64"
	"fmt"
	"syscall"
	"unsafe"
)

// Windows DPAPI. The encryption key is derived by the OS from the logged-in
// user's credentials, so the ciphertext can only be read back by that same user
// on that same machine — which is exactly the property a saved VPN account
// password needs and the reason no application-held key is involved.
//
// `CRYPTPROTECT_UI_FORBIDDEN` is set so the call can never put a dialog on
// screen; a background process that blocks on a prompt nobody can see is worse
// than an error. `CRYPTPROTECT_LOCAL_MACHINE` is deliberately NOT set: machine
// scope would let any account on the box read the secret.

const cryptProtectUIForbidden = 0x01

var (
	crypt32       = syscall.NewLazyDLL("crypt32.dll")
	kernel32      = syscall.NewLazyDLL("kernel32.dll")
	procProtect   = crypt32.NewProc("CryptProtectData")
	procUnprotect = crypt32.NewProc("CryptUnprotectData")
	procLocalFree = kernel32.NewProc("LocalFree")
)

// dataBlob mirrors the Win32 DATA_BLOB.
type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newBlob(d []byte) *dataBlob {
	if len(d) == 0 {
		return &dataBlob{}
	}
	return &dataBlob{cbData: uint32(len(d)), pbData: &d[0]}
}

func (b *dataBlob) bytes() []byte {
	if b.pbData == nil || b.cbData == 0 {
		return nil
	}
	return unsafe.Slice(b.pbData, b.cbData)
}

// Seal encrypts plaintext for the current user on this machine.
func Seal(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}

	in := newBlob([]byte(plain))
	var out dataBlob

	ret, _, err := procProtect.Call(
		uintptr(unsafe.Pointer(in)),
		0,
		0,
		0,
		0,
		uintptr(cryptProtectUIForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return "", fmt.Errorf("CryptProtectData failed: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	return base64.StdEncoding.EncodeToString(out.bytes()), nil
}

// Open reverses Seal.
func Open(sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}

	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", fmt.Errorf("sealed value is not valid base64: %w", err)
	}

	in := newBlob(raw)
	var out dataBlob

	ret, _, callErr := procUnprotect.Call(
		uintptr(unsafe.Pointer(in)),
		0,
		0,
		0,
		0,
		uintptr(cryptProtectUIForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		// The usual cause is a vault written by a different Windows user, or a
		// copied config directory. Say so, because "decryption failed" alone
		// sends people looking for a bug in the wrong place.
		return "", fmt.Errorf("CryptUnprotectData failed (the value was sealed by a different Windows user or machine): %w", callErr)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	return string(out.bytes()), nil
}

// Backend names the mechanism, for the diagnostic report and the UI.
func Backend() string { return "Windows DPAPI (current user)" }
