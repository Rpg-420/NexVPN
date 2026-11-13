// Command keygen generates the Ed25519 release-signing keypair.
//
// Run it once, in a directory OUTSIDE this repository:
//
//	go run <repo>/tools/keygen            # or: go run ./tools/keygen from the repo, then move the files
//
// It writes:
//
//	private.hex  → add as the APP_SIGNING_PRIVATE_KEY repository secret. NEVER commit.
//	public.hex   → pin in docs/signing.md so anyone can verify a release offline.
//
// Losing the private key only means generating a new pair and re-pinning the
// public key. Leaking it means rotating immediately (a leaked key can sign a
// malicious installer that verifies as genuine).
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	// -out exists so the tool can be run from inside the repository
	// (`go run ./tools/keygen`) while still writing the keys somewhere else —
	// running it from outside the module cannot resolve the package path.
	out := flag.String("out", ".", "directory to write private.hex and public.hex into (must be OUTSIDE the repo)")
	flag.Parse()

	if err := os.MkdirAll(*out, 0700); err != nil {
		fmt.Fprintln(os.Stderr, "cannot create output directory:", err)
		os.Exit(1)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "key generation failed:", err)
		os.Exit(1)
	}
	privPath := filepath.Join(*out, "private.hex")
	pubPath := filepath.Join(*out, "public.hex")
	if err := os.WriteFile(privPath, []byte(hex.EncodeToString(priv)), 0600); err != nil {
		fmt.Fprintln(os.Stderr, "cannot write private.hex:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(pubPath, []byte(hex.EncodeToString(pub)), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "cannot write public.hex:", err)
		os.Exit(1)
	}
	fmt.Println("wrote:")
	fmt.Println("  ", privPath, "(SECRET - add as the APP_SIGNING_PRIVATE_KEY repository secret)")
	fmt.Println("  ", pubPath, "(pin in docs/signing.md)")
	fmt.Println("public key:", hex.EncodeToString(pub))
}
