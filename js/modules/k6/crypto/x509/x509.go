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

	"github.com/loadimpact/k6/js/common"
)

type X509 struct{}

type Certificate struct {
	Subject CertificateSubject
	SignatureAlgorithm string `js:"signatureAlgorithm"`
}

type CertificateSubject struct {
	CountryName string `js:"countryName"`
	PostalCode string `js:"postalCode"`
	StateOrProvinceName string `js:"stateOrProvinceName"`
	LocalityName string `js:"localityName"`
	StreetAddress string `js:"streetAddress"`
	OrganizationName string `js:"organizationName"`
	OrganizationalUnitName []string `js:"organizationalUnitName"`
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
		SignatureAlgorithm: SignatureAlgorithm(parsed.SignatureAlgorithm),
	}
}

func MakeSubject(subject pkix.Name) (CertificateSubject) {
	return CertificateSubject{
		CountryName: First(subject.Country),
		PostalCode: First(subject.PostalCode),
		StateOrProvinceName: First(subject.Province),
		LocalityName: First(subject.Locality),
		StreetAddress: First(subject.StreetAddress),
		OrganizationName: First(subject.Organization),
		OrganizationalUnitName: subject.OrganizationalUnit,
	}
}

func First(values []string) (string) {
	if (len(values) > 0) {
		return values[0]
	} else {
		return ""
	}
}

func SignatureAlgorithm(value x509.SignatureAlgorithm) (string) {
	if (value == x509.UnknownSignatureAlgorithm) {
		return "UnknownSignatureAlgorithm"
	} else {
		return value.String()
	}
}
