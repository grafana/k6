package netext

import (
	"crypto/x509"
	"encoding/asn1"
	"fmt"
)

// RFC 5652 (CMS) / RFC 2315 (PKCS#7) content-type object identifiers.
var oidSignedData = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2} //nolint:gochecknoglobals

// pkcs7ContentInfo is the outermost PKCS#7 structure:
//
//	ContentInfo ::= SEQUENCE {
//	  contentType   OBJECT IDENTIFIER,
//	  content  [0]  EXPLICIT ANY DEFINED BY contentType
//	}
type pkcs7ContentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"explicit,optional,tag:0"`
}

// pkcs7SignedData holds only the certificate set of a SignedData structure;
// digest algorithms, signer infos and CRLs are irrelevant for chain building:
//
//	SignedData ::= SEQUENCE {
//	  version            CMSVersion,
//	  digestAlgorithms   SET OF DigestAlgorithmIdentifier,
//	  encapContentInfo   EncapsulatedContentInfo,
//	  certificates  [0]  IMPLICIT CertificateSet OPTIONAL,
//	  ...crls/signerInfos — ignored here
//	}
type pkcs7SignedData struct {
	Version          int
	DigestAlgorithms []asn1.RawValue `asn1:"set"`
	EncapContentInfo asn1.RawValue
	Certificates     asn1.RawValue `asn1:"implicit,optional,tag:0"`
}

// parsePKCS7Certificates extracts X.509 certificates from a DER-encoded PKCS#7
// SignedData bundle — the "certs-only" CMS form (Content-Type
// application/pkcs7-mime, .p7c) that some public CAs (Sectigo, legacy
// Verisign) serve from AIA URLs instead of a bare certificate.
//
// It deliberately does no cryptographic validation: signatures, digest
// algorithms and signer infos are not checked. The extracted certificates are
// only ever fed into x509 chain verification, which performs all the
// cryptographic checks — an attacker-controlled AIA response gains nothing
// unless the certificates it carries actually validate against the trusted
// roots. Non-certificate entries of the CertificateSet (attribute
// certificates, other formats) are skipped, and any entry that does not parse
// as an X.509 certificate makes the whole bundle fail, keeping the parser
// strict about what it accepts.
func parsePKCS7Certificates(der []byte) ([]*x509.Certificate, error) {
	var info pkcs7ContentInfo
	rest, err := asn1.Unmarshal(der, &info)
	if err != nil {
		return nil, fmt.Errorf("parsing PKCS#7 content info: %w", err)
	}
	if len(rest) != 0 {
		return nil, fmt.Errorf("%d trailing bytes after PKCS#7 content info", len(rest))
	}
	if !info.ContentType.Equal(oidSignedData) {
		return nil, fmt.Errorf("unsupported PKCS#7 content type %v, want signedData", info.ContentType)
	}

	var signedData pkcs7SignedData
	// Content is [0] EXPLICIT, so its Bytes field is the content of the [0]
	// wrapper: the complete SignedData SEQUENCE, header included.
	if _, err = asn1.Unmarshal(info.Content.Bytes, &signedData); err != nil {
		return nil, fmt.Errorf("parsing PKCS#7 signedData: %w", err)
	}
	if len(signedData.Certificates.Bytes) == 0 {
		return nil, fmt.Errorf("PKCS#7 bundle contains no certificates")
	}
	return parsePKCS7CertificateSet(signedData.Certificates.Bytes)
}

// parsePKCS7CertificateSet walks a CertificateSet (SET OF CertificateChoices)
// and returns the X.509 certificates it contains:
//
//	CertificateChoices ::= CHOICE {
//	  certificate             Certificate,          -- universal SEQUENCE
//	  extendedCertificate [0] ...,                  -- obsolete
//	  v2AttrCert          [2] ...,                  -- not chain material
//	  other           [3] ...
//	}
func parsePKCS7CertificateSet(set []byte) ([]*x509.Certificate, error) {
	certs := []*x509.Certificate{}
	for len(set) > 0 {
		var choice asn1.RawValue
		var err error
		if set, err = asn1.Unmarshal(set, &choice); err != nil {
			return nil, fmt.Errorf("parsing PKCS#7 certificate set entry: %w", err)
		}
		if choice.Class != asn1.ClassUniversal || choice.Tag != asn1.TagSequence {
			continue // attribute certificates and other non-chain formats
		}
		cert, err := x509.ParseCertificate(choice.FullBytes)
		if err != nil {
			return nil, fmt.Errorf("parsing X.509 certificate from PKCS#7 bundle: %w", err)
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return nil, fmt.Errorf("PKCS#7 bundle contains no X.509 certificates")
	}
	return certs, nil
}
