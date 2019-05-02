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
	"encoding/pem"
	"errors"

	"github.com/loadimpact/k6/js/common"
)

type X509 struct{}

type Certificate struct {
	SignatureAlgorithm string `js:"signatureAlgorithm"`
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
		SignatureAlgorithm: SignatureAlgorithm(parsed.SignatureAlgorithm),
	}
}

func SignatureAlgorithm(value x509.SignatureAlgorithm) (string) {
	if (value == x509.UnknownSignatureAlgorithm) {
		return "UnknownSignatureAlgorithm"
	} else {
		return value.String()
	}
}
