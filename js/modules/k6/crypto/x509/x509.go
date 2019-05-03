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
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"time"

	"github.com/loadimpact/k6/js/common"
)

type X509 struct{}

type Certificate struct {
	Subject Subject
	Issuer Issuer
	NotBefore string `js:"notBefore"`
	NotAfter string `js:"notAfter"`
	AltNames []string `js:"altNames"`
	SignatureAlgorithm string `js:"signatureAlgorithm"`
	FingerPrint []byte `js:"fingerPrint"`
	PublicKey PublicKey `js:"publicKey"`
}

type Subject struct {
	CommonName string `js:"commonName"`
	Country string
	PostalCode string `js:"postalCode"`
	StateOrProvinceName string `js:"stateOrProvinceName"`
	LocalityName string `js:"localityName"`
	StreetAddress string `js:"streetAddress"`
	OrganizationName string `js:"organizationName"`
	OrganizationalUnitName []string `js:"organizationalUnitName"`
}

type Issuer struct {
	CommonName string `js:"commonName"`
	Country string
	StateOrProvinceName string `js:"stateOrProvinceName"`
	LocalityName string `js:"localityName"`
	OrganizationName string `js:"organizationName"`
}

type PublicKey struct {
	Algorithm string
	E int
	N []byte
}

func New() *X509 {
	return &X509{}
}

func (X509) Parse(ctx context.Context, encoded string) (Certificate) {
	parsed := ParseCertificate(ctx, encoded)
	return MakeCertificate(parsed)
}

func (X509) GetAltNames(ctx context.Context, encoded string) ([]string) {
	parsed := ParseCertificate(ctx, encoded)
	return AltNames(parsed)
}

func (X509) GetIssuer(ctx context.Context, encoded string) (Issuer) {
	parsed := ParseCertificate(ctx, encoded)
	return MakeIssuer(parsed.Issuer)
}

func (X509) GetSubject(ctx context.Context, encoded string) (Subject) {
	parsed := ParseCertificate(ctx, encoded)
	return MakeSubject(parsed.Subject)
}

func ParseCertificate(ctx context.Context, encoded string) (*x509.Certificate) {
	decoded, _ := pem.Decode([]byte(encoded))
	if decoded == nil {
		err := errors.New("failed to decode certificate PEM file")
		common.Throw(common.GetRuntime(ctx), err)
	}
	parsed, err := x509.ParseCertificate(decoded.Bytes)
	if err != nil {
		err := errors.New("failed to parse certificate")
		common.Throw(common.GetRuntime(ctx), err)
	}
	return parsed
}

func MakeCertificate(parsed *x509.Certificate) (Certificate) {
	return Certificate{
		Subject: MakeSubject(parsed.Subject),
		Issuer: MakeIssuer(parsed.Issuer),
		NotBefore: ISO8601(parsed.NotBefore),
		NotAfter: ISO8601(parsed.NotAfter),
		AltNames: AltNames(parsed),
		SignatureAlgorithm: SignatureAlgorithm(parsed.SignatureAlgorithm),
		FingerPrint: FingerPrint(parsed),
		PublicKey: MakePublicKey(parsed),
	}
}

func MakeSubject(subject pkix.Name) (Subject) {
	return Subject{
		CommonName: subject.CommonName,
		Country: First(subject.Country),
		PostalCode: First(subject.PostalCode),
		StateOrProvinceName: First(subject.Province),
		LocalityName: First(subject.Locality),
		StreetAddress: First(subject.StreetAddress),
		OrganizationName: First(subject.Organization),
		OrganizationalUnitName: subject.OrganizationalUnit,
	}
}

func MakeIssuer(issuer pkix.Name) (Issuer) {
	return Issuer{
		CommonName: issuer.CommonName,
		Country: First(issuer.Country),
		StateOrProvinceName: First(issuer.Province),
		LocalityName: First(issuer.Locality),
		OrganizationName: First(issuer.Organization),
	}
}

func MakePublicKey(parsed *x509.Certificate) (PublicKey) {
	key := parsed.PublicKey.(*rsa.PublicKey)
	return PublicKey{
		Algorithm: PublicKeyAlgorithm(parsed.PublicKeyAlgorithm),
		E: key.E,
		N: key.N.Bytes(),
	}
}

func First(values []string) (string) {
	if (len(values) > 0) {
		return values[0]
	} else {
		return ""
	}
}

func ISO8601(value time.Time) (string) {
	return value.Format(time.RFC3339)
}

func AltNames(parsed *x509.Certificate) ([]string) {
	var names []string
	names = append(names, parsed.DNSNames...)
	names = append(names, parsed.EmailAddresses...)
	names = append(names, IPAddresses(parsed)...)
	names = append(names, URIs(parsed)...)
	return names
}

func IPAddresses(parsed *x509.Certificate) ([]string) {
	strings := make([]string, len(parsed.IPAddresses))
	for i, item := range parsed.IPAddresses {
		strings[i] = item.String()
	}
	return strings
}

func URIs(parsed *x509.Certificate) ([]string) {
	strings := make([]string, len(parsed.URIs))
	for i, item := range parsed.URIs {
		strings[i] = item.String()
	}
	return strings
}

func SignatureAlgorithm(value x509.SignatureAlgorithm) (string) {
	if (value == x509.UnknownSignatureAlgorithm) {
		return "UnknownSignatureAlgorithm"
	} else {
		return value.String()
	}
}

func FingerPrint(parsed *x509.Certificate) ([]byte) {
	bytes := sha1.Sum(parsed.Raw)
	return bytes[:]
}

func PublicKeyAlgorithm(value x509.PublicKeyAlgorithm) (string) {
	if (value == x509.UnknownPublicKeyAlgorithm) {
		return "UnknownPublicKeyAlgorithm"
	} else {
		return value.String()
	}
}
