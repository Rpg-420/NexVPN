// Command verify-signature checks an Ed25519 signature produced by the release
// pipeline (see .github/workflows/release.yml) against the pinned public key.
//
// Usage:
//
//	verify-signature <public-key-hex> <file> <file.sig>
//
// Exit codes: 0 verified, 1 verification failed / error. Designed to be
// scriptable from package.ps1 and from a user's terminal after downloading a
// NexVPN-Setup-*.exe plus its .sig from the release page.
package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: verify-signature <public-key-hex> <file> <file.sig>")
		os.Exit(1)
	}
	pub, err := hex.DecodeString(os.Args[1])
	if err != nil || len(pub) != ed25519.PublicKeySize {
		fmt.Fprintln(os.Stderr, "invalid public key (want 64 hex chars = 32 bytes):", err)
		os.Exit(1)
	}
	sig, err := hex.DecodeString(readTrimmed(os.Args[3]))
	if err != nil || len(sig) != ed25519.SignatureSize {
		fmt.Fprintln(os.Stderr, "invalid signature file (want 128 hex chars = 64 bytes):", err)
		os.Exit(1)
	}
	msg, err := os.ReadFile(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read signed file:", err)
		os.Exit(1)
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), msg, sig) {
		fmt.Fprintln(os.Stderr, "SIGNATURE INVALID — do not trust this file")
		os.Exit(1)
	}
	fmt.Println("signature OK")
}

func readTrimmed(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	// tolerate trailing newline / whitespace in the .sig sidecar
	return string(trimSpace(raw))
}

func trimSpace(b []byte) []byte {
	start, end := 0, len(b)
	for start < end && (b[start] == ' ' || b[start] == '\n' || b[start] == '\r' || b[start] == '\t') {
		start++
	}
	for end > start && (b[end-1] == ' ' || b[end-1] == '\n' || b[end-1] == '\r' || b[end-1] == '\t') {
		end--
	}
	return b[start:end]
}
