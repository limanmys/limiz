package tlsutil

import (
	"crypto/tls"
	"fmt"
	"os"
)

// LoadCertificate tries to load a TLS certificate from multiple sources in order:
//  1. File paths (certFile + keyFile) if both files exist on disk
//  2. Platform cert store via storeName (Windows Certificate Store)
//  3. Platform-specific auto-search (cert store on Windows, common paths on Linux)
func LoadCertificate(certFile, keyFile, storeName string) (*tls.Certificate, string, error) {
	// Source 1: explicit file paths
	if certFile != "" && keyFile != "" {
		if fileExists(certFile) && fileExists(keyFile) {
			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return nil, "", fmt.Errorf("failed to load certificate files: %w", err)
			}
			return &cert, fmt.Sprintf("file (%s, %s)", certFile, keyFile), nil
		}
	}

	// Source 2: explicit store name
	if storeName != "" {
		cert, src, err := loadFromStore(storeName)
		if err == nil {
			return cert, src, nil
		}
	}

	// Source 3: platform-specific auto-search
	defaultStore := storeName
	if defaultStore == "" {
		defaultStore = "Limiz"
	}
	return platformSearch(defaultStore)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
