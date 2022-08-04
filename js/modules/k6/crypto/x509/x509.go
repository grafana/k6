package x509

import (
	"crypto/dsa"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha1" // #nosec G505
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// X509 represents an instance of the X509 certificate module.
	X509 struct {
		vu modules.VU
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &X509{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &X509{vu: vu}
}

// Exports returns the exports of the execution module.
func (mi *X509) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"parse":       mi.parse,
			"getAltNames": mi.altNames,
			"getIssuer":   mi.issuer,
			"getSubject":  mi.subject,
		},
	}
}

// Certificate is an X.509 certificate
type Certificate struct {
	Subject            Subject
	Issuer             Issuer
	NotBefore          string    `js:"notBefore"`
	NotAfter           string    `js:"notAfter"`
	AltNames           []string  `js:"altNames"`
	SignatureAlgorithm string    `js:"signatureAlgorithm"`
	FingerPrint        []byte    `js:"fingerPrint"`
	PublicKey          PublicKey `js:"publicKey"`
}

// RDN is a component of an X.509 distinguished name
type RDN struct {
	Type  string
	Value string
}

// Subject is a certificate subject
type Subject struct {
	CommonName             string `js:"commonName"`
	Country                string
	PostalCode             string   `js:"postalCode"`
	StateOrProvinceName    string   `js:"stateOrProvinceName"`
	LocalityName           string   `js:"localityName"`
	StreetAddress          string   `js:"streetAddress"`
	OrganizationName       string   `js:"organizationName"`
	OrganizationalUnitName []string `js:"organizationalUnitName"`
	Names                  []RDN
}

// Issuer is a certificate issuer
type Issuer struct {
	CommonName          string `js:"commonName"`
	Country             string
	StateOrProvinceName string `js:"stateOrProvinceName"`
	LocalityName        string `js:"localityName"`
	OrganizationName    string `js:"organizationName"`
	Names               []RDN
}

// PublicKey is used for decryption and signature verification
type PublicKey struct {
	Algorithm string
	Key       interface{}
}

// parse produces an entire X.509 certificate
func (mi X509) parse(encoded []byte) (Certificate, error) {
	parsed, err := parseCertificate(encoded)
	if err != nil {
		return Certificate{}, err
	}
	certificate, err := makeCertificate(parsed)
	if err != nil {
		return Certificate{}, err
	}
	return certificate, nil
}

// altNames extracts alt names
func (mi X509) altNames(encoded []byte) ([]string, error) {
	parsed, err := parseCertificate(encoded)
	if err != nil {
		return nil, err
	}
	return altNames(parsed), nil
}

// issuer extracts certificate issuer
func (mi X509) issuer(encoded []byte) (Issuer, error) {
	parsed, err := parseCertificate(encoded)
	if err != nil {
		return Issuer{}, err
	}
	return makeIssuer(parsed.Issuer), nil
}

// subject extracts certificate subject
func (mi X509) subject(encoded []byte) Subject {
	parsed, err := parseCertificate(encoded)
	if err != nil {
		common.Throw(mi.vu.Runtime(), err)
	}
	return makeSubject(parsed.Subject)
}

func parseCertificate(encoded []byte) (*x509.Certificate, error) {
	decoded, _ := pem.Decode(encoded)
	if decoded == nil {
		return nil, fmt.Errorf("failed to decode certificate PEM file")
	}
	parsed, err := x509.ParseCertificate(decoded.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}
	return parsed, nil
}

func makeCertificate(parsed *x509.Certificate) (Certificate, error) {
	publicKey, err := makePublicKey(parsed.PublicKey)
	if err != nil {
		return Certificate{}, err
	}
	return Certificate{
		Subject:            makeSubject(parsed.Subject),
		Issuer:             makeIssuer(parsed.Issuer),
		NotBefore:          iso8601(parsed.NotBefore),
		NotAfter:           iso8601(parsed.NotAfter),
		AltNames:           altNames(parsed),
		SignatureAlgorithm: signatureAlgorithm(parsed.SignatureAlgorithm),
		FingerPrint:        fingerPrint(parsed),
		PublicKey:          publicKey,
	}, nil
}

func makeSubject(subject pkix.Name) Subject {
	return Subject{
		CommonName:             subject.CommonName,
		Country:                first(subject.Country),
		PostalCode:             first(subject.PostalCode),
		StateOrProvinceName:    first(subject.Province),
		LocalityName:           first(subject.Locality),
		StreetAddress:          first(subject.StreetAddress),
		OrganizationName:       first(subject.Organization),
		OrganizationalUnitName: subject.OrganizationalUnit,
		Names:                  makeRdns(subject.Names),
	}
}

func makeIssuer(issuer pkix.Name) Issuer {
	return Issuer{
		CommonName:          issuer.CommonName,
		Country:             first(issuer.Country),
		StateOrProvinceName: first(issuer.Province),
		LocalityName:        first(issuer.Locality),
		OrganizationName:    first(issuer.Organization),
		Names:               makeRdns(issuer.Names),
	}
}

func makePublicKey(parsed interface{}) (PublicKey, error) {
	var algorithm string
	switch parsed.(type) {
	case *dsa.PublicKey:
		algorithm = "DSA"
	case *ecdsa.PublicKey:
		algorithm = "ECDSA"
	case *rsa.PublicKey:
		algorithm = "RSA"
	default:
		err := errors.New("unsupported public key algorithm")
		return PublicKey{}, err
	}
	return PublicKey{
		Algorithm: algorithm,
		Key:       parsed,
	}, nil
}

func first(values []string) string {
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func iso8601(value time.Time) string {
	return value.Format(time.RFC3339)
}

func makeRdns(names []pkix.AttributeTypeAndValue) []RDN {
	result := make([]RDN, len(names))
	for i, name := range names {
		result[i] = makeRdn(name)
	}
	return result
}

func makeRdn(name pkix.AttributeTypeAndValue) RDN {
	return RDN{
		Type:  name.Type.String(),
		Value: fmt.Sprintf("%v", name.Value),
	}
}

func altNames(parsed *x509.Certificate) []string {
	var names []string
	names = append(names, parsed.DNSNames...)
	names = append(names, parsed.EmailAddresses...)
	names = append(names, ipAddresses(parsed)...)
	names = append(names, uris(parsed)...)
	return names
}

func ipAddresses(parsed *x509.Certificate) []string {
	strings := make([]string, len(parsed.IPAddresses))
	for i, item := range parsed.IPAddresses {
		strings[i] = item.String()
	}
	return strings
}

func uris(parsed *x509.Certificate) []string {
	strings := make([]string, len(parsed.URIs))
	for i, item := range parsed.URIs {
		strings[i] = item.String()
	}
	return strings
}

func signatureAlgorithm(value x509.SignatureAlgorithm) string {
	if value == x509.UnknownSignatureAlgorithm {
		return "UnknownSignatureAlgorithm"
	}
	return value.String()
}

func fingerPrint(parsed *x509.Certificate) []byte {
	bytes := sha1.Sum(parsed.Raw) // #nosec G401
	return bytes[:]
}
