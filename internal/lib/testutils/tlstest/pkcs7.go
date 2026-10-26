package tlstest

import (
	"crypto/x509"
	"encoding/asn1"
	"testing"
)

// Object identifiers from PKCS#7 / CMS (RFC 5652).
var (
	oidData       = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1} //nolint:gochecknoglobals
	oidSignedData = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2} //nolint:gochecknoglobals
)

// BuildPKCS7Bundle builds a DER-encoded, certs-only PKCS#7 SignedData bundle —
// the "degenerate" CMS form (Content-Type application/pkcs7-mime, .p7c) that
// public CAs such as Sectigo serve from AIA URLs. It carries no signatures or
// signer infos, exactly like the real certs-only bundles.
func BuildPKCS7Bundle(t testing.TB, certs ...*x509.Certificate) []byte {
	t.Helper()
	if len(certs) == 0 {
		t.Fatal("BuildPKCS7Bundle requires at least one certificate")
	}

	var certSet []byte
	for _, cert := range certs {
		certSet = append(certSet, cert.Raw...)
	}

	// SignedData ::= SEQUENCE {
	//   version          INTEGER 1 (v1),
	//   digestAlgorithms SET OF {},               -- empty
	//   encapContentInfo SEQUENCE { OID id-data }, -- eContent absent
	//   certificates [0] IMPLICIT SET OF CertificateChoices, -- certs directly,
	//   signerInfos      SET OF {}                          -- no inner SET header
	// }
	// This mirrors `openssl crl2pkcs7 -nocrl` output byte for byte: certificates
	// are the direct content of the [0] element and signerInfos is an empty SET.
	signedData := derTLV(0x30, concat(
		[]byte{asn1.TagInteger, 0x01, 0x01},
		derTLV(0x31),
		derTLV(0x30, marshalOID(t, oidData)),
		derTLV(0xA0, certSet),
		derTLV(0x31),
	))

	// ContentInfo ::= SEQUENCE { OID id-signedData, content [0] EXPLICIT SignedData }
	return derTLV(0x30, concat(
		marshalOID(t, oidSignedData),
		derTLV(0xA0, signedData),
	))
}

func marshalOID(t testing.TB, oid asn1.ObjectIdentifier) []byte {
	t.Helper()
	der, err := asn1.Marshal(oid)
	if err != nil {
		t.Fatalf("marshaling OID %v: %v", oid, err)
	}
	return der
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

// derTLV encodes a single DER element with the given tag byte. Certificates
// are small enough that at most three length octets are needed.
func derTLV(tag byte, content ...[]byte) []byte {
	var body []byte
	for _, part := range content {
		body = append(body, part...)
	}
	l := len(body)
	var length []byte
	switch {
	case l < 0x80:
		length = []byte{byte(l)}
	case l < 0x100:
		length = []byte{0x81, byte(l)}
	case l < 0x10000:
		length = []byte{0x82, byte((l >> 8) & 0xFF), byte(l & 0xFF)}
	default: // unreachable for test certificate material
		length = []byte{0x83, byte((l >> 16) & 0xFF), byte((l >> 8) & 0xFF), byte(l & 0xFF)}
	}
	out := make([]byte, 0, 1+len(length)+l)
	out = append(out, tag)
	out = append(out, length...)
	return append(out, body...)
}
