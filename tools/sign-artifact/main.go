// Command sign-artifact produces the hex Ed25519 signature for a release
// artifact. Used by the release workflow; the private key comes from the
// APP_SIGNING_PRIVATE_KEY secret (64 hex chars = ed25519.PrivateKey).
//
// Usage:
//
//	sign-artifact <file>           # writes <file>.sig next to it; prints hex
//
// The matching public key is pinned in the repo (RELEASE_SIGNING_PUBLIC_KEY
// in .github/workflows/release.yml) so anyone can verify a download with
// tools/verify-signature.
package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: sign-artifact <file>")
		os.Exit(1)
	}
	keyHex := strings.TrimSpace(os.Getenv("APP_SIGNING_PRIVATE_KEY"))
	if keyHex == "" {
		fmt.Fprintln(os.Stderr, "APP_SIGNING_PRIVATE_KEY is not set")
		os.Exit(1)
	}
	priv, err := hex.DecodeString(keyHex)
	if err != nil || len(priv) != ed25519.PrivateKeySize {
		fmt.Fprintln(os.Stderr, "APP_SIGNING_PRIVATE_KEY is not a valid ed25519 private key (want 128 hex chars = 64 bytes):", err)
		os.Exit(1)
	}
	msg, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read artifact:", err)
		os.Exit(1)
	}
	sig := ed25519.Sign(ed25519.PrivateKey(priv), msg)
	sigPath := os.Args[1] + ".sig"
	if err := os.WriteFile(sigPath, []byte(hex.EncodeToString(sig)), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "cannot write signature:", err)
		os.Exit(1)
	}
	fmt.Println("wrote", filepath.Base(sigPath))
}
