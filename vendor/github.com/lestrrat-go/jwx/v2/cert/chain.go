package cert

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Chain represents a certificate chain as used in the `x5c` field of
// various objects within JOSE.
//
// It stores the certificates as a list of base64 encoded []byte
// sequence. By definition these values must PKIX encoded.
type Chain struct {
	certificates [][]byte
}

func (cc Chain) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, cert := range cc.certificates {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteByte('"')
		buf.Write(cert)
		buf.WriteByte('"')
	}
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

func (cc *Chain) UnmarshalJSON(data []byte) error {
	var tmp []string
	if err := json.Unmarshal(data, &tmp); err != nil {
		return fmt.Errorf(`failed to unmarshal certificate chain: %w`, err)
	}

	certs := make([][]byte, len(tmp))
	for i, cert := range tmp {
		certs[i] = []byte(cert)
	}
	cc.certificates = certs
	return nil
}

// Get returns the n-th ASN.1 DER + base64 encoded certificate
// stored. `false` will be returned in the second argument if
// the corresponding index is out of range.
func (cc *Chain) Get(index int) ([]byte, bool) {
	if index < 0 || index >= len(cc.certificates) {
		return nil, false
	}

	return cc.certificates[index], true
}

// Len returns the number of certificates stored in this Chain
func (cc *Chain) Len() int {
	return len(cc.certificates)
}

var pemStart = []byte("----- BEGIN CERTIFICATE -----")
var pemEnd = []byte("----- END CERTIFICATE -----")

func (cc *Chain) AddString(der string) error {
	return cc.Add([]byte(der))
}

func (cc *Chain) Add(der []byte) error {
	// We're going to be nice and remove marker lines if they
	// give it to us
	der = bytes.TrimPrefix(der, pemStart)
	der = bytes.TrimSuffix(der, pemEnd)
	der = bytes.TrimSpace(der)
	cc.certificates = append(cc.certificates, der)
	return nil
}
