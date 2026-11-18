// Package clientcert attaches TLS client certificates to browser connections.
package clientcert

import (
	"crypto/tls"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/pkcs12"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/lib"
)

// LoadCertificate builds a TLS client certificate from PEM cert and key data, or from PFX
// (PKCS#12) data. passphrase decrypts an encrypted PEM key or the PFX; it is ignored for an
// unencrypted PEM key.
func LoadCertificate(cert, key, pfx []byte, passphrase string) (tls.Certificate, error) {
	if len(pfx) > 0 {
		return loadPFX(pfx, passphrase)
	}
	return loadPEM(cert, key, passphrase)
}

func loadPFX(pfx []byte, passphrase string) (tls.Certificate, error) {
	blocks, err := pkcs12.ToPEM(pfx, passphrase)
	if err != nil {
		if errors.Is(err, pkcs12.ErrIncorrectPassword) {
			return tls.Certificate{}, errors.New("incorrect PFX passphrase")
		}
		if errors.As(err, new(pkcs12.NotImplementedError)) {
			return tls.Certificate{}, fmt.Errorf(
				"unsupported PFX encryption; re-export with 'openssl pkcs12 -legacy' or use PEM: %w", err)
		}
		return tls.Certificate{}, fmt.Errorf("decoding PFX: %w", err)
	}

	var (
		certs  []*pem.Block
		keyPEM []byte
	)
	for _, b := range blocks {
		switch b.Type {
		case "CERTIFICATE":
			certs = append(certs, &pem.Block{Type: b.Type, Bytes: b.Bytes})
		case "PRIVATE KEY":
			keyPEM = pem.EncodeToMemory(&pem.Block{Type: b.Type, Bytes: b.Bytes})
		}
	}
	certs = leafFirst(certs, keyPEM)

	var certPEM []byte
	for _, b := range certs {
		certPEM = append(certPEM, pem.EncodeToMemory(b)...)
	}

	c, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}
	return c, nil
}

// leafFirst moves the certificate whose public key matches the private key to the front,
// keeping the other certificates in their original order.
func leafFirst(certs []*pem.Block, keyPEM []byte) []*pem.Block {
	// X509KeyPair with a single certificate parses the key and checks that it matches.
	for i, b := range certs {
		if _, err := tls.X509KeyPair(pem.EncodeToMemory(b), keyPEM); err != nil {
			continue
		}
		ordered := make([]*pem.Block, 0, len(certs))
		ordered = append(ordered, b)
		ordered = append(ordered, certs[:i]...)
		return append(ordered, certs[i+1:]...)
	}
	return certs
}

func loadPEM(cert, key []byte, passphrase string) (tls.Certificate, error) {
	var encrypted bool
	if b, _ := pem.Decode(key); b != nil {
		if b.Type == "ENCRYPTED PRIVATE KEY" {
			return tls.Certificate{}, errors.New(
				"unsupported encrypted PKCS#8 key; convert to an unencrypted or legacy-encrypted PEM key")
		}
		encrypted = b.Headers["Proc-Type"] == "4,ENCRYPTED"
	}
	if encrypted && passphrase == "" {
		return tls.Certificate{}, errors.New("private key is encrypted; passphrase is required")
	}

	auth := &lib.TLSAuth{TLSAuthFields: lib.TLSAuthFields{
		Cert:     string(cert),
		Key:      string(key),
		Password: null.NewString(passphrase, encrypted),
	}}
	c, err := auth.Certificate()
	if err != nil {
		// Key decryption errors come from crypto/x509; key pair errors come from crypto/tls.
		if encrypted && strings.HasPrefix(err.Error(), "x509: ") {
			return tls.Certificate{}, fmt.Errorf("decrypting private key: %w", err)
		}
		return tls.Certificate{}, err
	}
	return *c, nil
}
