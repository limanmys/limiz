package datas

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/limanmys/limiz/internal/signing"
	"os"
	"strings"
)

// AllowedDataPlugins is set at compile time via -ldflags.
// Comma-separated data plugin names: "folder-size,cert-check"
// If empty, no data plugins will run.
var AllowedDataPlugins string

// isAllowedDataPlugin checks whether the given plugin name is in the
// compile-time allowlist.
func isAllowedDataPlugin(name string) bool {
	if AllowedDataPlugins == "" {
		return false
	}
	for _, allowed := range strings.Split(AllowedDataPlugins, ",") {
		if strings.TrimSpace(allowed) == name {
			return true
		}
	}
	return false
}

// VerifyDataPlugin performs two-layer security verification:
//  1. Compile-time allowlist check
//  2. Ed25519 digital signature verification
//
// If an error is returned, the plugin must not be executed.
func VerifyDataPlugin(name, binaryPath string) error {
	// Layer 1: allowlist
	if !isAllowedDataPlugin(name) {
		return fmt.Errorf("data plugin '%s' is not defined in the allowlist — binary must be recompiled", name)
	}

	// Layer 2: signature verification
	if signing.EmbeddedPublicKey == "" {
		return fmt.Errorf("EmbeddedPublicKey is not embedded in the binary — -ldflags missing")
	}

	pubBytes, err := base64.StdEncoding.DecodeString(signing.EmbeddedPublicKey)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid EmbeddedPublicKey format (expected %d bytes)", ed25519.PublicKeySize)
	}

	// Read plugin binary and compute SHA256 hash
	data, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to read data plugin binary '%s': %v", binaryPath, err)
	}
	hash := sha256.Sum256(data)

	// Read .sig file
	sigPath := binaryPath + ".sig"
	sig, err := os.ReadFile(sigPath)
	if err != nil {
		return fmt.Errorf("signature file not found '%s': %v", sigPath, err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature size: expected %d, got %d", ed25519.SignatureSize, len(sig))
	}

	// Verify Ed25519 signature
	if !ed25519.Verify(ed25519.PublicKey(pubBytes), hash[:], sig) {
		return fmt.Errorf("signature verification FAILED: '%s' is unauthorized or has been modified", name)
	}

	return nil
}
