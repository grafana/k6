/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package x509

import (
	"context"
	"crypto/rsa"
	"crypto/sha1" // #nosec G505
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"time"

	"github.com/loadimpact/k6/js/common"
)

// X509 certificate functionality
type X509 struct{}

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
}

// Issuer is a certificate issuer
type Issuer struct {
	CommonName          string `js:"commonName"`
	Country             string
	StateOrProvinceName string `js:"stateOrProvinceName"`
	LocalityName        string `js:"localityName"`
	OrganizationName    string `js:"organizationName"`
}

// PublicKey is a public key
type PublicKey struct {
	Algorithm string
	E         int
	N         []byte
}

// New constructs the X509 interface
func New() *X509 {
	return &X509{}
}

// Parse produces an entire X.509 certificate
func (X509) Parse(ctx context.Context, encoded string) Certificate {
	parsed := parseCertificate(ctx, encoded)
	return makeCertificate(parsed)
}

// GetAltNames extracts alt names
func (X509) GetAltNames(ctx context.Context, encoded string) []string {
	parsed := parseCertificate(ctx, encoded)
	return altNames(parsed)
}

// GetIssuer extracts certificate issuer
func (X509) GetIssuer(ctx context.Context, encoded string) Issuer {
	parsed := parseCertificate(ctx, encoded)
	return makeIssuer(parsed.Issuer)
}

// GetSubject extracts certificate subject
func (X509) GetSubject(ctx context.Context, encoded string) Subject {
	parsed := parseCertificate(ctx, encoded)
	return makeSubject(parsed.Subject)
}

func parseCertificate(ctx context.Context, encoded string) *x509.Certificate {
	decoded, _ := pem.Decode([]byte(encoded))
	if decoded == nil {
		err := errors.New("failed to decode certificate PEM file")
		throw(ctx, err)
	}
	parsed, err := x509.ParseCertificate(decoded.Bytes)
	if err != nil {
		err := errors.New("failed to parse certificate")
		throw(ctx, err)
	}
	return parsed
}

func makeCertificate(parsed *x509.Certificate) Certificate {
	return Certificate{
		Subject:            makeSubject(parsed.Subject),
		Issuer:             makeIssuer(parsed.Issuer),
		NotBefore:          iso8601(parsed.NotBefore),
		NotAfter:           iso8601(parsed.NotAfter),
		AltNames:           altNames(parsed),
		SignatureAlgorithm: signatureAlgorithm(parsed.SignatureAlgorithm),
		FingerPrint:        fingerPrint(parsed),
		PublicKey:          makePublicKey(parsed),
	}
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
	}
}

func makeIssuer(issuer pkix.Name) Issuer {
	return Issuer{
		CommonName:          issuer.CommonName,
		Country:             first(issuer.Country),
		StateOrProvinceName: first(issuer.Province),
		LocalityName:        first(issuer.Locality),
		OrganizationName:    first(issuer.Organization),
	}
}

func makePublicKey(parsed *x509.Certificate) PublicKey {
	key := parsed.PublicKey.(*rsa.PublicKey)
	return PublicKey{
		Algorithm: publicKeyAlgorithm(parsed.PublicKeyAlgorithm),
		E:         key.E,
		N:         key.N.Bytes(),
	}
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

func publicKeyAlgorithm(value x509.PublicKeyAlgorithm) string {
	if value == x509.UnknownPublicKeyAlgorithm {
		return "UnknownPublicKeyAlgorithm"
	}
	return value.String()
}

func throw(ctx context.Context, err error) {
	common.Throw(common.GetRuntime(ctx), err)
}
