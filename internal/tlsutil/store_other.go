//go:build !windows

package tlsutil

import (
	"crypto/ecdsa"
	"crypto/tls"
	"fmt"
)

// searchPaths lists standard Linux paths where Limiz TLS certificates may be placed.
var searchPaths = []struct{ cert, key string }{
	{"/etc/limiz/tls/server.crt", "/etc/limiz/tls/server.key"},
	{"/etc/limiz/server.crt", "/etc/limiz/server.key"},
	{"/etc/ssl/certs/limiz.crt", "/etc/ssl/private/limiz.key"},
	{"/etc/pki/tls/certs/limiz.crt", "/etc/pki/tls/private/limiz.key"},
}

func loadFromStore(_ string) (*tls.Certificate, string, error) {
	return nil, "", fmt.Errorf("Windows certificate store is not supported on this platform")
}

func ImportToStore(_ []byte, _ *ecdsa.PrivateKey) error {
	return fmt.Errorf("certificate store import is not supported on this platform")
}

func platformSearch(_ string) (*tls.Certificate, string, error) {
	for _, p := range searchPaths {
		if fileExists(p.cert) && fileExists(p.key) {
			cert, err := tls.LoadX509KeyPair(p.cert, p.key)
			if err != nil {
				continue
			}
			return &cert, fmt.Sprintf("system path (%s, %s)", p.cert, p.key), nil
		}
	}
	return nil, "", fmt.Errorf(
		"TLS certificate files not found. Searched locations:\n" +
			"  - /etc/limiz/tls/server.crt + server.key\n" +
			"  - /etc/limiz/server.crt + server.key\n" +
			"  - /etc/ssl/certs/limiz.crt + /etc/ssl/private/limiz.key\n" +
			"  - /etc/pki/tls/certs/limiz.crt + /etc/pki/tls/private/limiz.key")
}
