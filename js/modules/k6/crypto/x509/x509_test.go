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
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/stretchr/testify/assert"
)

func MakeRuntime() *goja.Runtime {
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	ctx := context.Background()
	ctx = common.WithRuntime(ctx, rt)
	rt.Set("x509", common.Bind(rt, New(), &ctx))
	return rt
}

func TestParse(t *testing.T) {
	rt := MakeRuntime()

	pem := `-----BEGIN CERTIFICATE-----
MIIEOjCCAyKgAwIBAgICBNIwDQYJKoZIhvcNAQELBQAwgdsxCzAJBgNVBAYTAlpa
MRkwFwYDVQQIExBLb3B1bmNlemlzIEtyYWlzMREwDwYDVQQHEwhBc2h0aW5vazEa
MBgGA1UECRMRMjIxQiBCYWtlciBTdHJlZXQxDjAMBgNVBBETBTk5OTk5MRwwGgYD
VQQKExNFeHVtYnJhbiBDb252ZW50aW9uMT0wFwYDVQQLExBFeHVtYnJhbiBDb3Vu
Y2lsMCIGA1UECxMbRXh1bWJyYW4gSmFuaXRvcmlhbCBTZXJ2aWNlMRUwEwYDVQQD
EwxleGNvdW5jaWwuenowIhgPMDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAw
MFowgdsxCzAJBgNVBAYTAlpaMRkwFwYDVQQIExBLb3B1bmNlemlzIEtyYWlzMREw
DwYDVQQHEwhBc2h0aW5vazEaMBgGA1UECRMRMjIxQiBCYWtlciBTdHJlZXQxDjAM
BgNVBBETBTk5OTk5MRwwGgYDVQQKExNFeHVtYnJhbiBDb252ZW50aW9uMT0wFwYD
VQQLExBFeHVtYnJhbiBDb3VuY2lsMCIGA1UECxMbRXh1bWJyYW4gSmFuaXRvcmlh
bCBTZXJ2aWNlMRUwEwYDVQQDEwxleGNvdW5jaWwuenowggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQCdOksn+apGPkW+qVluAxR0EdCNqmIUvyT+YBLAGA8o
vhuxHpbwvLlUZhkYCPDow+SDAtbgODGLlUqpdPWAkCeaFSymLlr/Zounvd4UoTCh
lB5Zj67nOCkOC2TNqiXM13J6VuST5wT0yow5Kue88fjTIJbvdktMKytFQcrU80JM
HEv/oZek8pf1TDI7lPXXme2688xJnP3mSdbcjjH71BiFzK4CbDtbGMdUNShEh6WK
ZIvH0KR5gwjkys80B+jXs27/iqcDCKhqsO6acFbY9twAybUB9twxrwacq4X1ACCl
eKyifc7QMBOJh5PZJsVw/GpdOmsTH2sFTOagwvvQ8V0nAgMBAAGjAjAAMA0GCSqG
SIb3DQEBCwUAA4IBAQBubUTtrqcdK8ULdJQi8gVMEPRoINXWqE3rsSWcRWYfANTL
unjzeTgzRXMqz78QuTg4Gyt/vxcsn1NL2/pAPYTEiM7+RmF9+erZm3ZPBDoI4o2l
ncM/qMGh06N7Bnc3HINkEwA7Gd+j1q9SWAiYKsrZa/Qvpi6RAAo7hgisuJeDB6xq
dkPW2VCdXgM2B/fb2yrX1n3VsWU2UfIPEgGt7zPEx5hjn7qFWO3AX2NvbdC7oifc
xxBhiUaXE50bAJQLwgy/qQ63IjYRnWPzYdZnZIMarpyHEgSln6715WkbdgxAnnGx
ltfk96gUo55F5PpIjQezwcLYjVLmjMF6PNWFQYXt
-----END CERTIFICATE-----`
	pemTemplate := fmt.Sprintf("`%s`", pem)

	t.Run("ParseFailure", func(t *testing.T) {
		_, err := common.RunString(rt, `
		x509.parse("bad-certificate");
		`)
		assert.Error(t, err)
	})

	t.Run("ParseSignatureAlgorithm", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.signatureAlgorithm;
		if (value !== "SHA256-RSA") {
			throw new Error("Bad signature algorithm: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseSubject", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		if (typeof cert.subject !== "object") {
			throw new Error("Bad subject: " + typeof cert.subject);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseSubjectCommonName", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.commonName : null;
		if (value !== "excouncil.zz") {
			throw new Error("Bad subject common name: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseSubjectCountry", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.countryName : null;
		if (value !== "ZZ") {
			throw new Error("Bad subject country: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseSubjectPostalCode", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.postalCode : null;
		if (value !== "99999") {
			throw new Error("Bad subject postal code: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseSubjectProvince", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.stateOrProvinceName : null;
		if (value !== "Kopuncezis Krais") {
			throw new Error("Bad subject province: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseSubjectLocality", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.localityName : null;
		if (value !== "Ashtinok") {
			throw new Error("Bad subject locality: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseSubjectStreetAddress", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.streetAddress : null;
		if (value !== "221B Baker Street") {
			throw new Error("Bad subject street address: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseSubjectOrganization", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.organizationName : null;
		if (value !== "Exumbran Convention") {
			throw new Error("Bad subject organization: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseSubjectOrganizationalUnit", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const values =
			cert.subject ? cert.subject.organizationalUnitName : null;
		if (!(
			values.length === 2 &&
			values[0] === "Exumbran Council" &&
			values[1] === "Exumbran Janitorial Service"
		)) {
			throw new Error(
				"Bad subject organizational unit: " + values.join(", ")
			);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseIssuer", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem)
		if (typeof cert.issuer !== "object") {
			throw new Error("Bad issuer: " + typeof cert.issuer);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseIssuerCountry", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.issuer ? cert.issuer.countryName : null;
		if (value !== "ZZ") {
			throw new Error("Bad issuer country: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseIssuerProvince", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.issuer ? cert.issuer.stateOrProvinceName : null;
		if (value !== "Kopuncezis Krais") {
			throw new Error("Bad issuer province: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})
}
