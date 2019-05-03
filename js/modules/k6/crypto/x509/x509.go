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
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"time"

	"github.com/loadimpact/k6/js/common"
)

type X509 struct{}

type Certificate struct {
	Subject CertificateSubject
	Issuer CertificateIssuer
	NotBefore string `js:"notBefore"`
	NotAfter string `js:"notAfter"`
	AltNames []string `js:"altNames"`
	SignatureAlgorithm string `js:"signatureAlgorithm"`
}

type CertificateSubject struct {
	CommonName string `js:"commonName"`
	CountryName string `js:"countryName"`
	PostalCode string `js:"postalCode"`
	StateOrProvinceName string `js:"stateOrProvinceName"`
	LocalityName string `js:"localityName"`
	StreetAddress string `js:"streetAddress"`
	OrganizationName string `js:"organizationName"`
	OrganizationalUnitName []string `js:"organizationalUnitName"`
}

type CertificateIssuer struct {
	CommonName string `js:"commonName"`
	CountryName string `js:"countryName"`
	StateOrProvinceName string `js:"stateOrProvinceName"`
	LocalityName string `js:"localityName"`
	OrganizationName string `js:"organizationName"`
}

func New() *X509 {
	return &X509{}
}

func (X509) Parse(ctx context.Context, encoded string) (Certificate) {
	decoded, _ := pem.Decode([]byte(encoded))
	if decoded == nil {
		err := errors.New("Failed to decode certificate PEM file")
		common.Throw(common.GetRuntime(ctx), err)
	}
	parsed, err := x509.ParseCertificate(decoded.Bytes)
	if err != nil {
		err := errors.New("Failed to parse certificate")
		common.Throw(common.GetRuntime(ctx), err)
	}
	return MakeCertificate(parsed)
}

func MakeCertificate(parsed *x509.Certificate) (Certificate) {
	return Certificate{
		Subject: MakeSubject(parsed.Subject),
		Issuer: MakeIssuer(parsed.Issuer),
		NotBefore: ISO8601(parsed.NotBefore),
		NotAfter: ISO8601(parsed.NotAfter),
		AltNames: AltNames(parsed),
		SignatureAlgorithm: SignatureAlgorithm(parsed.SignatureAlgorithm),
	}
}

func MakeSubject(subject pkix.Name) (CertificateSubject) {
	return CertificateSubject{
		CommonName: subject.CommonName,
		CountryName: First(subject.Country),
		PostalCode: First(subject.PostalCode),
		StateOrProvinceName: First(subject.Province),
		LocalityName: First(subject.Locality),
		StreetAddress: First(subject.StreetAddress),
		OrganizationName: First(subject.Organization),
		OrganizationalUnitName: subject.OrganizationalUnit,
	}
}

func MakeIssuer(issuer pkix.Name) (CertificateIssuer) {
	return CertificateIssuer{
		CommonName: issuer.CommonName,
		CountryName: First(issuer.Country),
		StateOrProvinceName: First(issuer.Province),
		LocalityName: First(issuer.Locality),
		OrganizationName: First(issuer.Organization),
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
