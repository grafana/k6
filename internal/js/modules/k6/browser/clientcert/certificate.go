// Package clientcert attaches TLS client certificates to browser connections.
package clientcert

import (
	"crypto/tls"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/lib"
)

// LoadCertificate builds a TLS client certificate from PEM cert and key data. passphrase
// decrypts a legacy-encrypted (Proc-Type: 4,ENCRYPTED) key; it is ignored for an unencrypted key.
func LoadCertificate(cert, key []byte, passphrase string) (tls.Certificate, error) {
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
