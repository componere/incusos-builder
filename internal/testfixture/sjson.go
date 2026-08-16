package testfixture

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const (
	sjsonBoundary = "----=_Fixture_SJSON_1"
	certCN        = "incusos-builder testfixture"
	keySeed       = "incusos-builder/internal/testfixture"
	b64LineLen    = 64
)

// signedSJSON wraps payload in a multipart/signed S/MIME document whose
// first part is payload and whose second part is a self-signed throwaway
// certificate generated from a fixed seed. The update adapter validates
// MIME structure and JSON binding, not the PKCS#7 signature. Ed25519 is
// used so the certificate DER is deterministic under Go 1.26's
// cryptocustomrand rules (ECDSA signatures ignore a supplied Reader).
func signedSJSON(payload []byte) ([]byte, error) {
	certPEM, err := fixtureCertPEM()
	if err != nil {
		return nil, err
	}

	var b strings.Builder
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/signed;")
	b.WriteString(" protocol=\"application/x-pkcs7-signature\";")
	b.WriteString(" micalg=sha-256;")
	b.WriteString(" boundary=\"")
	b.WriteString(sjsonBoundary)
	b.WriteString("\"\r\n\r\n")
	b.WriteString("--")
	b.WriteString(sjsonBoundary)
	b.WriteString("\r\nContent-Type: text/plain\r\n\r\n")
	b.Write(payload)
	b.WriteString("\r\n--")
	b.WriteString(sjsonBoundary)
	b.WriteString("\r\nContent-Type: application/x-pkcs7-signature; name=\"smime.p7s\"\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	b.WriteString(wrapBase64(base64.StdEncoding.EncodeToString(certPEM)))
	b.WriteString("\r\n--")
	b.WriteString(sjsonBoundary)
	b.WriteString("--\r\n")

	return []byte(b.String()), nil
}

// fixtureCertPEM returns a deterministic self-signed certificate in PEM.
func fixtureCertPEM() ([]byte, error) {
	seed := sha256.Sum256([]byte(keySeed))
	priv := ed25519.NewKeyFromSeed(seed[:])

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: certCN, Organization: []string{"incusos-builder"}},
		NotBefore:             time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:              time.Date(2040, time.January, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(nil, template, template, priv.Public(), priv)
	if err != nil {
		return nil, fmt.Errorf("create fixture certificate: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

// wrapBase64 inserts CRLF every [b64LineLen] characters.
func wrapBase64(s string) string {
	if s == "" {
		return s
	}

	var b strings.Builder
	for len(s) > b64LineLen {
		b.WriteString(s[:b64LineLen])
		b.WriteString("\r\n")
		s = s[b64LineLen:]
	}
	b.WriteString(s)

	return b.String()
}
