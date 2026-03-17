//go:build windows

package tlsutil

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"strings"
	"syscall"
	"unsafe"
)

var (
	crypt32 = syscall.NewLazyDLL("crypt32.dll")
	ncrypt  = syscall.NewLazyDLL("ncrypt.dll")

	procCertOpenStore                     = crypt32.NewProc("CertOpenStore")
	procCertCloseStore                    = crypt32.NewProc("CertCloseStore")
	procCertFindCertificateInStore        = crypt32.NewProc("CertFindCertificateInStore")
	procCertFreeCertificateContext        = crypt32.NewProc("CertFreeCertificateContext")
	procCryptAcquireCertificatePrivateKey = crypt32.NewProc("CryptAcquireCertificatePrivateKey")
	procCertAddEncodedCertificateToStore  = crypt32.NewProc("CertAddEncodedCertificateToStore")
	procCertSetCertificateContextProperty = crypt32.NewProc("CertSetCertificateContextProperty")

	procNCryptSignHash            = ncrypt.NewProc("NCryptSignHash")
	procNCryptFreeObject          = ncrypt.NewProc("NCryptFreeObject")
	procNCryptOpenStorageProvider = ncrypt.NewProc("NCryptOpenStorageProvider")
	procNCryptImportKey           = ncrypt.NewProc("NCryptImportKey")
	procNCryptSetProperty         = ncrypt.NewProc("NCryptSetProperty")
	procNCryptFinalizeKey         = ncrypt.NewProc("NCryptFinalizeKey")
	procNCryptDeleteKey           = ncrypt.NewProc("NCryptDeleteKey")
	procNCryptOpenKey             = ncrypt.NewProc("NCryptOpenKey")
)

const (
	certStoreProvSystem         = 10
	certSystemStoreLocalMachine = 0x00020000
	certSystemStoreCurrentUser  = 0x00010000

	certFindSubjectStr = 0x00080007
	certFindHash       = 0x00010000

	certEncodingX509ASN  = 0x00000001
	certEncodingPKCS7ASN = 0x00010000

	cryptAcquireOnlyNCryptKeyFlag = 0x00040000
	cryptAcquireSilentFlag        = 0x00000040

	ncryptPadPKCS1Flag = 0x00000002
	ncryptSilentFlag   = 0x00000040

	// ImportToStore constants
	certStoreAddReplaceExisting = 3
	certKeyProvInfoPropID       = 2

	ncryptOverwriteKeyFlag     = 0x00000080
	ncryptMachineKeyFlag       = 0x00000020
	ncryptAllowExportFlag      = 0x00000001
	ncryptExportPolicyProperty = "Export Policy"
	ncryptLengthProperty       = "Length"

	bcryptEccPrivateBlob = "ECCPRIVATEBLOB"
	bcryptEcdhP256Magic  = 0x324B4345 // ECK2
	bcryptEcdsaP256Magic = 0x32534345 // ECS2

	msSoftwareKSP = "Microsoft Software Key Storage Provider"
)

// certContext mirrors the Windows CERT_CONTEXT structure.
type certContext struct {
	CertEncodingType uint32
	CertEncoded      *byte
	CertEncodedLen   uint32
	CertInfo         uintptr
	Store            uintptr
}

type cryptHashBlob struct {
	Size uint32
	Data *byte
}

type pkcs1PaddingInfo struct {
	AlgId *uint16
}

// ncryptSigner implements crypto.Signer using a Windows NCrypt key handle.
// The key never leaves the Windows key store; signing is performed via NCryptSignHash.
type ncryptSigner struct {
	handle  uintptr
	certCtx uintptr // prevent premature release
	pub     crypto.PublicKey
}

func (s *ncryptSigner) Public() crypto.PublicKey { return s.pub }

func (s *ncryptSigner) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	if len(digest) == 0 {
		return nil, fmt.Errorf("empty digest")
	}
	switch s.pub.(type) {
	case *rsa.PublicKey:
		return s.signRSAPKCS1(digest, opts.HashFunc())
	case *ecdsa.PublicKey:
		return s.signECDSA(digest)
	default:
		return nil, fmt.Errorf("unsupported key type: %T", s.pub)
	}
}

func (s *ncryptSigner) signRSAPKCS1(digest []byte, hash crypto.Hash) ([]byte, error) {
	algName := hashAlg(hash)
	if algName == "" {
		return nil, fmt.Errorf("unsupported hash algorithm: %v", hash)
	}
	algPtr, _ := syscall.UTF16PtrFromString(algName)
	padding := pkcs1PaddingInfo{AlgId: algPtr}

	// First call: determine output size
	var cbResult uint32
	r, _, _ := procNCryptSignHash.Call(
		s.handle,
		uintptr(unsafe.Pointer(&padding)),
		uintptr(unsafe.Pointer(&digest[0])), uintptr(len(digest)),
		0, 0,
		uintptr(unsafe.Pointer(&cbResult)),
		ncryptPadPKCS1Flag|ncryptSilentFlag,
	)
	if r != 0 {
		return nil, fmt.Errorf("NCryptSignHash size error: 0x%x", r)
	}

	sig := make([]byte, cbResult)
	r, _, _ = procNCryptSignHash.Call(
		s.handle,
		uintptr(unsafe.Pointer(&padding)),
		uintptr(unsafe.Pointer(&digest[0])), uintptr(len(digest)),
		uintptr(unsafe.Pointer(&sig[0])), uintptr(cbResult),
		uintptr(unsafe.Pointer(&cbResult)),
		ncryptPadPKCS1Flag|ncryptSilentFlag,
	)
	if r != 0 {
		return nil, fmt.Errorf("NCryptSignHash error: 0x%x", r)
	}
	return sig[:cbResult], nil
}

func (s *ncryptSigner) signECDSA(digest []byte) ([]byte, error) {
	// First call: determine output size
	var cbResult uint32
	r, _, _ := procNCryptSignHash.Call(
		s.handle,
		0,
		uintptr(unsafe.Pointer(&digest[0])), uintptr(len(digest)),
		0, 0,
		uintptr(unsafe.Pointer(&cbResult)),
		ncryptSilentFlag,
	)
	if r != 0 {
		return nil, fmt.Errorf("NCryptSignHash ECDSA size error: 0x%x", r)
	}

	sig := make([]byte, cbResult)
	r, _, _ = procNCryptSignHash.Call(
		s.handle,
		0,
		uintptr(unsafe.Pointer(&digest[0])), uintptr(len(digest)),
		uintptr(unsafe.Pointer(&sig[0])), uintptr(cbResult),
		uintptr(unsafe.Pointer(&cbResult)),
		ncryptSilentFlag,
	)
	if r != 0 {
		return nil, fmt.Errorf("NCryptSignHash ECDSA error: 0x%x", r)
	}

	// Windows ECDSA raw output: r || s (each component is half the total size)
	sigLen := int(cbResult)
	half := sigLen / 2
	rInt := new(big.Int).SetBytes(sig[:half])
	sInt := new(big.Int).SetBytes(sig[half:sigLen])
	return asn1.Marshal(struct{ R, S *big.Int }{rInt, sInt})
}

func hashAlg(h crypto.Hash) string {
	switch h {
	case crypto.SHA1:
		return "SHA1"
	case crypto.SHA256:
		return "SHA256"
	case crypto.SHA384:
		return "SHA384"
	case crypto.SHA512:
		return "SHA512"
	default:
		return ""
	}
}

// loadFromStore loads a TLS certificate from the Windows Certificate Store.
// It searches Local Machine\My first, then Current User\My.
// subjectOrThumb can be a certificate subject name (CN) or a SHA-1 thumbprint (hex).
func loadFromStore(subjectOrThumb string) (*tls.Certificate, string, error) {
	for _, storeFlag := range []uint32{certSystemStoreLocalMachine, certSystemStoreCurrentUser} {
		cert, src, err := loadFromStoreWithFlag(subjectOrThumb, storeFlag)
		if err == nil {
			return cert, src, nil
		}
	}
	return nil, "", fmt.Errorf("'%s' not found in Windows certificate store (searched LocalMachine\\My and CurrentUser\\My)", subjectOrThumb)
}

func loadFromStoreWithFlag(subjectOrThumb string, storeFlag uint32) (*tls.Certificate, string, error) {
	storeNamePtr, _ := syscall.UTF16PtrFromString("MY")
	store, _, err := procCertOpenStore.Call(
		certStoreProvSystem, 0, 0,
		uintptr(storeFlag),
		uintptr(unsafe.Pointer(storeNamePtr)),
	)
	if store == 0 {
		return nil, "", fmt.Errorf("CertOpenStore error: %v", err)
	}
	defer procCertCloseStore.Call(store, 0)

	var certCtx uintptr

	// Try thumbprint first (SHA-1 = 40 hex chars)
	cleaned := strings.ReplaceAll(strings.ReplaceAll(subjectOrThumb, " ", ""), ":", "")
	if len(cleaned) == 40 {
		if thumbBytes, hexErr := hex.DecodeString(cleaned); hexErr == nil {
			blob := cryptHashBlob{
				Size: uint32(len(thumbBytes)),
				Data: &thumbBytes[0],
			}
			certCtx, _, _ = procCertFindCertificateInStore.Call(
				store,
				certEncodingX509ASN|certEncodingPKCS7ASN,
				0,
				certFindHash,
				uintptr(unsafe.Pointer(&blob)),
				0,
			)
		}
	}

	// If not found by thumbprint, try subject string
	if certCtx == 0 {
		subjectPtr, _ := syscall.UTF16PtrFromString(subjectOrThumb)
		certCtx, _, _ = procCertFindCertificateInStore.Call(
			store,
			certEncodingX509ASN|certEncodingPKCS7ASN,
			0,
			certFindSubjectStr,
			uintptr(unsafe.Pointer(subjectPtr)),
			0,
		)
	}

	if certCtx == 0 {
		return nil, "", fmt.Errorf("certificate not found: %s", subjectOrThumb)
	}
	// certCtx is intentionally NOT freed — it lives for the server's lifetime

	// Extract DER-encoded certificate
	ctx := (*certContext)(unsafe.Pointer(certCtx))
	certDER := unsafe.Slice(ctx.CertEncoded, ctx.CertEncodedLen)
	certBytes := make([]byte, len(certDER))
	copy(certBytes, certDER)

	x509Cert, parseErr := x509.ParseCertificate(certBytes)
	if parseErr != nil {
		procCertFreeCertificateContext.Call(certCtx)
		return nil, "", fmt.Errorf("x509 parse error: %w", parseErr)
	}

	// Acquire NCrypt private key handle
	var keyHandle uintptr
	var keySpec uint32
	var callerFree int32
	r, _, err := procCryptAcquireCertificatePrivateKey.Call(
		certCtx,
		cryptAcquireOnlyNCryptKeyFlag|cryptAcquireSilentFlag,
		0,
		uintptr(unsafe.Pointer(&keyHandle)),
		uintptr(unsafe.Pointer(&keySpec)),
		uintptr(unsafe.Pointer(&callerFree)),
	)
	if r == 0 {
		procCertFreeCertificateContext.Call(certCtx)
		return nil, "", fmt.Errorf("failed to acquire private key: %v", err)
	}

	signer := &ncryptSigner{
		handle:  keyHandle,
		certCtx: certCtx,
		pub:     x509Cert.PublicKey,
	}

	storeLoc := "LocalMachine"
	if storeFlag == certSystemStoreCurrentUser {
		storeLoc = "CurrentUser"
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{certBytes},
		PrivateKey:  signer,
		Leaf:        x509Cert,
	}

	source := fmt.Sprintf("Windows Certificate Store (%s\\My, Subject: %s)", storeLoc, x509Cert.Subject.CommonName)
	return tlsCert, source, nil
}

func platformSearch(defaultStore string) (*tls.Certificate, string, error) {
	// On Windows, auto-search tries the cert store with the given name
	cert, src, err := loadFromStore(defaultStore)
	if err == nil {
		return cert, src, nil
	}

	// Also check common Windows file paths
	paths := []struct{ cert, key string }{
		{`C:\Program Files\limiz\certs\server.crt`, `C:\Program Files\limiz\certs\server.key`},
		{`C:\ProgramData\limiz\certs\server.crt`, `C:\ProgramData\limiz\certs\server.key`},
	}
	for _, p := range paths {
		if fileExists(p.cert) && fileExists(p.key) {
			c, loadErr := tls.LoadX509KeyPair(p.cert, p.key)
			if loadErr != nil {
				continue
			}
			return &c, fmt.Sprintf("file (%s, %s)", p.cert, p.key), nil
		}
	}

	return nil, "", fmt.Errorf(
		"TLS certificate not found:\n"+
			"  - Certificate files do not exist at the specified path\n"+
			"  - '%s' not found in Windows Certificate Store\n"+
			"  - No certificate found in standard file paths", defaultStore)
}

// cryptKeyProvInfo mirrors the Windows CRYPT_KEY_PROV_INFO structure.
type cryptKeyProvInfo struct {
	ContainerName *uint16
	ProvName      *uint16
	ProvType      uint32
	Flags         uint32
	ProvParamCnt  uint32
	ProvParam     uintptr
	KeySpec       uint32
}

// ImportToStore imports a self-signed certificate and its ECDSA private key
// into the Windows LocalMachine\My certificate store. The key is stored in
// the Microsoft Software Key Storage Provider under the name "Limiz".
func ImportToStore(certDER []byte, privKey *ecdsa.PrivateKey) error {
	keyName := "Limiz"

	// Step 1: Open the Microsoft Software Key Storage Provider
	provNamePtr, _ := syscall.UTF16PtrFromString(msSoftwareKSP)
	var provHandle uintptr
	r, _, _ := procNCryptOpenStorageProvider.Call(
		uintptr(unsafe.Pointer(&provHandle)),
		uintptr(unsafe.Pointer(provNamePtr)),
		0,
	)
	if r != 0 {
		return fmt.Errorf("NCryptOpenStorageProvider failed: 0x%x", r)
	}
	defer procNCryptFreeObject.Call(provHandle)

	// Step 2: Delete existing key with the same name (if any) to avoid conflicts
	keyNamePtr, _ := syscall.UTF16PtrFromString(keyName)
	var existingKey uintptr
	r, _, _ = procNCryptOpenKey.Call(
		provHandle,
		uintptr(unsafe.Pointer(&existingKey)),
		uintptr(unsafe.Pointer(keyNamePtr)),
		0,
		ncryptMachineKeyFlag|ncryptSilentFlag,
	)
	if r == 0 && existingKey != 0 {
		// Key exists, delete it. NCryptDeleteKey frees the handle on success.
		procNCryptDeleteKey.Call(existingKey, 0)
	}

	// Step 3: Build BCRYPT_ECCKEY_BLOB for P-256 private key
	eccBlob, err := buildECCPrivateBlob(privKey)
	if err != nil {
		return fmt.Errorf("failed to build ECC key blob: %w", err)
	}

	// Step 4: Import the private key
	blobTypePtr, _ := syscall.UTF16PtrFromString(bcryptEccPrivateBlob)
	var keyHandle uintptr
	r, _, _ = procNCryptImportKey.Call(
		provHandle,
		0, // no wrapping key
		uintptr(unsafe.Pointer(blobTypePtr)),
		0, // no parameter list
		uintptr(unsafe.Pointer(&keyHandle)),
		uintptr(unsafe.Pointer(&eccBlob[0])),
		uintptr(len(eccBlob)),
		ncryptMachineKeyFlag|ncryptOverwriteKeyFlag|ncryptSilentFlag,
	)
	if r != 0 {
		return fmt.Errorf("NCryptImportKey failed: 0x%x", r)
	}
	defer procNCryptFreeObject.Call(keyHandle)

	// Step 5: Set the key name property
	r, _, _ = procNCryptSetProperty.Call(
		keyHandle,
		uintptr(unsafe.Pointer(keyNamePtr)),
		uintptr(unsafe.Pointer(keyNamePtr)),
		uintptr(len(keyName)+1)*2, // UTF-16 byte length including null
		ncryptMachineKeyFlag|ncryptSilentFlag,
	)
	// Non-fatal: key name property set failure is logged but not blocking

	// Step 6: Set export policy to allow export
	exportPolicy := uint32(ncryptAllowExportFlag)
	exportPropPtr, _ := syscall.UTF16PtrFromString(ncryptExportPolicyProperty)
	procNCryptSetProperty.Call(
		keyHandle,
		uintptr(unsafe.Pointer(exportPropPtr)),
		uintptr(unsafe.Pointer(&exportPolicy)),
		4, // sizeof(DWORD)
		ncryptSilentFlag,
	)

	// Step 7: Finalize the key (persist it)
	r, _, _ = procNCryptFinalizeKey.Call(keyHandle, ncryptSilentFlag)
	if r != 0 {
		return fmt.Errorf("NCryptFinalizeKey failed: 0x%x", r)
	}

	// Step 8: Open LocalMachine\MY certificate store
	storeNamePtr, _ := syscall.UTF16PtrFromString("MY")
	store, _, sysErr := procCertOpenStore.Call(
		certStoreProvSystem, 0, 0,
		uintptr(certSystemStoreLocalMachine),
		uintptr(unsafe.Pointer(storeNamePtr)),
	)
	if store == 0 {
		return fmt.Errorf("CertOpenStore (LocalMachine\\MY) failed: %v", sysErr)
	}
	defer procCertCloseStore.Call(store, 0)

	// Step 9: Add the DER-encoded certificate to the store
	var certCtx uintptr
	r, _, sysErr = procCertAddEncodedCertificateToStore.Call(
		store,
		certEncodingX509ASN|certEncodingPKCS7ASN,
		uintptr(unsafe.Pointer(&certDER[0])),
		uintptr(len(certDER)),
		uintptr(certStoreAddReplaceExisting),
		uintptr(unsafe.Pointer(&certCtx)),
	)
	if r == 0 {
		return fmt.Errorf("CertAddEncodedCertificateToStore failed: %v", sysErr)
	}
	defer procCertFreeCertificateContext.Call(certCtx)

	// Step 10: Associate the NCrypt key with the certificate via CERT_KEY_PROV_INFO
	containerPtr, _ := syscall.UTF16PtrFromString(keyName)
	provInfo := cryptKeyProvInfo{
		ContainerName: containerPtr,
		ProvName:      provNamePtr,
		ProvType:      0, // 0 for CNG (NCrypt) providers
		Flags:         ncryptMachineKeyFlag,
		KeySpec:       0, // AT_KEYEXCHANGE not needed for CNG
	}
	r, _, sysErr = procCertSetCertificateContextProperty.Call(
		certCtx,
		certKeyProvInfoPropID,
		0,
		uintptr(unsafe.Pointer(&provInfo)),
	)
	if r == 0 {
		return fmt.Errorf("CertSetCertificateContextProperty failed: %v", sysErr)
	}

	return nil
}

// buildECCPrivateBlob builds a BCRYPT_ECCKEY_BLOB for a P-256 private key.
// Format: magic(4) + keyLen(4) + X(keyLen) + Y(keyLen) + D(keyLen)
func buildECCPrivateBlob(privKey *ecdsa.PrivateKey) ([]byte, error) {
	if privKey.Curve != elliptic.P256() {
		return nil, fmt.Errorf("only P-256 keys are supported, got %v", privKey.Curve.Params().Name)
	}

	keyLen := 32 // P-256 = 32 bytes per component
	x := privKey.PublicKey.X.Bytes()
	y := privKey.PublicKey.Y.Bytes()
	d := privKey.D.Bytes()

	// Pad to keyLen
	xPad := make([]byte, keyLen)
	yPad := make([]byte, keyLen)
	dPad := make([]byte, keyLen)
	copy(xPad[keyLen-len(x):], x)
	copy(yPad[keyLen-len(y):], y)
	copy(dPad[keyLen-len(d):], d)

	// Header: magic (BCRYPT_ECDSA_PRIVATE_P256_MAGIC) + cbKey
	blob := make([]byte, 8+3*keyLen)
	binary.LittleEndian.PutUint32(blob[0:4], bcryptEcdsaP256Magic)
	binary.LittleEndian.PutUint32(blob[4:8], uint32(keyLen))
	copy(blob[8:], xPad)
	copy(blob[8+keyLen:], yPad)
	copy(blob[8+2*keyLen:], dPad)

	return blob, nil
}
