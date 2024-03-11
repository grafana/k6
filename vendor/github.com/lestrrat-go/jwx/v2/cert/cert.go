package cert

import (
	"crypto/x509"
	stdlibb64 "encoding/base64"
	"fmt"
	"io"

	"github.com/lestrrat-go/jwx/v2/internal/base64"
)

// Create is a wrapper around x509.CreateCertificate, but it additionally
// encodes it in base64 so that it can be easily added to `x5c` fields
func Create(rand io.Reader, template, parent *x509.Certificate, pub, priv interface{}) ([]byte, error) {
	der, err := x509.CreateCertificate(rand, template, parent, pub, priv)
	if err != nil {
		return nil, fmt.Errorf(`failed to create x509 certificate: %w`, err)
	}
	return EncodeBase64(der)
}

// EncodeBase64 is a utility function to encode ASN.1 DER certificates
// using base64 encoding. This operation is normally done by `pem.Encode`
// but since PEM would include the markers (`-----BEGIN`, and the like)
// while `x5c` fields do not need this, this function can be used to
// shave off a few lines
func EncodeBase64(der []byte) ([]byte, error) {
	enc := stdlibb64.StdEncoding
	dst := make([]byte, enc.EncodedLen(len(der)))
	enc.Encode(dst, der)
	return dst, nil
}

// Parse is a utility function to decode a base64 encoded
// ASN.1 DER format certificate, and to parse the byte sequence.
// The certificate must be in PKIX format, and it must not contain PEM markers
func Parse(src []byte) (*x509.Certificate, error) {
	dst, err := base64.Decode(src)
	if err != nil {
		return nil, fmt.Errorf(`failed to base64 decode the certificate: %w`, err)
	}

	cert, err := x509.ParseCertificate(dst)
	if err != nil {
		return nil, fmt.Errorf(`failed to parse x509 certificate: %w`, err)
	}
	return cert, nil
}
