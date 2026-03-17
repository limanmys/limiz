// sign-plugin: Signs Limiz plugin binaries with Ed25519.
//
// Usage:
//
//	Generate key pair:         sign-plugin --gen-key keys/plugin-signing
//	Sign single binary:        sign-plugin --key keys/plugin-signing.key plugins/metric/dir-size
//	Sign all metric plugins:   sign-plugin --key keys/plugin-signing.key --all plugins/metric/
//	Sign all data plugins:     sign-plugin --key keys/plugin-signing.key --all plugins/data/
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	genKey := flag.String("gen-key", "", "Generate key pair; specify prefix (e.g. keys/plugin-signing)")
	keyFile := flag.String("key", "", "Ed25519 private key file (.key)")
	all := flag.Bool("all", false, "Sign all binaries in directory")
	flag.Parse()

	switch {
	case *genKey != "":
		must(generateKeys(*genKey))

	case *keyFile != "" && *all && flag.NArg() == 1:
		must(signAll(*keyFile, flag.Arg(0)))

	case *keyFile != "" && flag.NArg() == 1:
		must(signFile(*keyFile, flag.Arg(0)))

	default:
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  sign-plugin --gen-key keys/plugin-signing")
		fmt.Fprintln(os.Stderr, "  sign-plugin --key keys/plugin-signing.key plugins/metric/dir-size")
		fmt.Fprintln(os.Stderr, "  sign-plugin --key keys/plugin-signing.key --all plugins/metric/")
		fmt.Fprintln(os.Stderr, "  sign-plugin --key keys/plugin-signing.key --all plugins/data/")
		os.Exit(2)
	}
}

// generateKeys generates a new Ed25519 key pair.
//
// Output files:
//   - <prefix>.key  — 64 byte Ed25519 private key (binary, mode 0600)
//   - <prefix>.pub  — 32 byte Ed25519 public key  (binary, mode 0644)
//
// The .pub file is read via `base64 -w0` in the Makefile and embedded via -ldflags.
func generateKeys(prefix string) error {
	if err := os.MkdirAll(filepath.Dir(prefix), 0700); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate key: %v", err)
	}

	keyPath := prefix + ".key"
	pubPath := prefix + ".pub"

	if err := os.WriteFile(keyPath, []byte(priv), 0600); err != nil {
		return fmt.Errorf("failed to write private key '%s': %v", keyPath, err)
	}
	if err := os.WriteFile(pubPath, []byte(pub), 0644); err != nil {
		return fmt.Errorf("failed to write public key '%s': %v", pubPath, err)
	}

	fmt.Printf("Private key : %s  (do not add to git!)\n", keyPath)
	fmt.Printf("Public key  : %s\n", pubPath)
	fmt.Println()
	fmt.Println("Add to Makefile:")
	fmt.Printf("  PUB_KEY := $(shell base64 -w0 %s)\n", pubPath)
	fmt.Println()
	fmt.Println("Add to -ldflags when building Limiz:")
	fmt.Printf("  -X 'limiz/collectors.EmbeddedPublicKey=$(PUB_KEY)'\n")
	return nil
}

// signFile signs a single binary and creates a <binary>.sig file.
func signFile(keyPath, binaryPath string) error {
	privKey, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key '%s': %v", keyPath, err)
	}
	if len(privKey) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid private key size: %d (expected %d)", len(privKey), ed25519.PrivateKeySize)
	}

	data, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to read binary '%s': %v", binaryPath, err)
	}

	hash := sha256.Sum256(data)
	sig := ed25519.Sign(ed25519.PrivateKey(privKey), hash[:])

	sigPath := binaryPath + ".sig"
	if err := os.WriteFile(sigPath, sig, 0644); err != nil {
		return fmt.Errorf("failed to write signature '%s': %v", sigPath, err)
	}

	fmt.Printf("SIGNED  %-35s  sha256=%s\n",
		filepath.Base(binaryPath),
		hex.EncodeToString(hash[:8])+"...")
	return nil
}

// signAll signs all binary files in a directory.
// .sig, .json, .md and other text files are skipped.
func signAll(keyPath, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory '%s': %v", dir, err)
	}

	skipExts := map[string]bool{
		".sig": true, ".json": true, ".md": true,
		".txt": true, ".yaml": true, ".yml": true,
	}

	signed, skipped := 0, 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if skipExts[ext] {
			skipped++
			continue
		}

		binaryPath := filepath.Join(dir, e.Name())
		if err := signFile(keyPath, binaryPath); err != nil {
			fmt.Fprintf(os.Stderr, "SKIP    %-35s  error: %v\n", e.Name(), err)
			continue
		}
		signed++
	}

	fmt.Printf("\n%d plugins signed, %d files skipped.\n", signed, skipped)
	return nil
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
}
