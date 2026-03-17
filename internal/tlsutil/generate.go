package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// GenerateResult holds the output of GenerateSelfSigned, including raw
// cryptographic material needed for platform-specific store imports.
type GenerateResult struct {
	CertPath string
	KeyPath  string
	CertDER  []byte
	PrivKey  *ecdsa.PrivateKey
}

// GenerateSelfSigned creates a self-signed TLS certificate and key pair,
// writes them to the given directory, and returns the result including file
// paths and raw crypto material for optional cert store import.
func GenerateSelfSigned(dir string) (*GenerateResult, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "Limiz", Organization: []string{"Limiz Self-Signed"}},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(5 * 365 * 24 * time.Hour), // 5 years
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost", hostname},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	// Write certificate
	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate file: %w", err)
	}
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		certFile.Close()
		return nil, fmt.Errorf("failed to write certificate: %w", err)
	}
	certFile.Close()

	// Write private key
	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("failed to encode key: %w", err)
	}
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create key file: %w", err)
	}
	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		keyFile.Close()
		return nil, fmt.Errorf("failed to write key: %w", err)
	}
	keyFile.Close()

	return &GenerateResult{
		CertPath: certPath,
		KeyPath:  keyPath,
		CertDER:  certDER,
		PrivKey:  priv,
	}, nil
}

// WritableCertDir tries platform-default directories in order and returns
// the first one that can be created and written to. Extra candidate directories
// (e.g. config-file-relative paths) are appended after the platform defaults.
func WritableCertDir(extraDirs ...string) (string, error) {
	candidates := defaultCertDirs()
	candidates = append(candidates, extraDirs...)

	// Deduplicate while preserving order
	seen := make(map[string]bool, len(candidates))
	unique := candidates[:0]
	for _, d := range candidates {
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		unique = append(unique, d)
	}
	candidates = unique

	for _, dir := range candidates {
		if err := os.MkdirAll(dir, 0750); err != nil {
			continue
		}
		// Verify write permission
		testFile := filepath.Join(dir, ".write-test")
		if err := os.WriteFile(testFile, []byte{}, 0600); err != nil {
			continue
		}
		os.Remove(testFile)
		return dir, nil
	}
	return "", fmt.Errorf("no writable TLS directory found, tried: %v", candidates)
}

func defaultCertDirs() []string {
	if runtime.GOOS == "windows" {
		return []string{
			`C:\Program Files\limiz\certs`,
			filepath.Join(os.Getenv("ProgramData"), "limiz", "certs"),
		}
	}

	dirs := []string{
		"/etc/limiz/tls",
		"/usr/lib/limiz/tls",
		"/var/lib/limiz/tls",
	}

	// tls/ directory under the binary's location
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(exe), "tls"))
	}

	return dirs
}
