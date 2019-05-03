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
MIIENjCCAx6gAwIBAgICBNIwDQYJKoZIhvcNAQELBQAwgdsxCzAJBgNVBAYTAlpa
MRkwFwYDVQQIExBLb3B1bmNlemlzIEtyYWlzMREwDwYDVQQHEwhBc2h0aW5vazEa
MBgGA1UECRMRMjIxQiBCYWtlciBTdHJlZXQxDjAMBgNVBBETBTk5OTk5MRwwGgYD
VQQKExNFeHVtYnJhbiBDb252ZW50aW9uMT0wFwYDVQQLExBFeHVtYnJhbiBDb3Vu
Y2lsMCIGA1UECxMbRXh1bWJyYW4gSmFuaXRvcmlhbCBTZXJ2aWNlMRUwEwYDVQQD
EwxleGNvdW5jaWwuenowHhcNMTkwMTAxMDAwMDAwWhcNMjAwMTAxMDAwMDAwWjCB
2zELMAkGA1UEBhMCWloxGTAXBgNVBAgTEEtvcHVuY2V6aXMgS3JhaXMxETAPBgNV
BAcTCEFzaHRpbm9rMRowGAYDVQQJExEyMjFCIEJha2VyIFN0cmVldDEOMAwGA1UE
ERMFOTk5OTkxHDAaBgNVBAoTE0V4dW1icmFuIENvbnZlbnRpb24xPTAXBgNVBAsT
EEV4dW1icmFuIENvdW5jaWwwIgYDVQQLExtFeHVtYnJhbiBKYW5pdG9yaWFsIFNl
cnZpY2UxFTATBgNVBAMTDGV4Y291bmNpbC56ejCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBALcTVKZ5qDpfpWCTsl7zdqqMG36WwkCOvRwIL3CK39ZN4kyC
e6EeHG+tZQQZq/ZtOp60KLs3EiFCt8Y4mGbruiQJPYpKdKnlTPVbGmz+BmhJhf+U
+Trr8TcYp7ORs6Iz23ZxbbdnW+eV/yTgPPVD/TGKLnLFeGdopRJf/tGBTKNS5k27
NrtXYUH4QlkmLM0IF+L1nnfkS3eDpi5DdKoCCIv2KTA1j/2ZSOBx/8mGc+rp9noW
bCSLnbBRsHaRkEMtpq+kYRcnza+iT3UaNUQl1SLoWAXBsgxk6HHC09g/csARwwU4
2zhwh+NrBLgHJVqVM9cJe3n8hnfhHoRcLWjEVjECAwEAAaMCMAAwDQYJKoZIhvcN
AQELBQADggEBAJqWfwlMcEA3KnUIk4Qt6LjE2ZCZvXWkFnm899Z95umCaD76DMu4
nj8ttnV6smQk2s+ecKQr959T5FmWv1FlOJCJ8w7Ph946apXhNuS+8mUachSShi0J
IPCnxw7uEZlPCzF6HgSEzcV47VD7SCDuyA20JmDT0dThKe9If76B96Ru7oItHF/S
yuyUrdCYaFBop3o83ba7CoOEy3YvFaMOypjNOganQ9H1hKp8vdgXj+hcuPyBVM9F
7UdusDcOB/IkTwUhRGfqqNXNoYA2NmaU7zxl1eglATElx7S7Z4oR6ikEQPCBXkcK
jMozgMz/EFm4jeVg0ALoTa8+esYf82v4vlc=
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
		const cert = x509.parse(pem);
		if (typeof cert.issuer !== "object") {
			throw new Error("Bad issuer: " + typeof cert.issuer);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseIssuerCommonName", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.issuer ? cert.issuer.commonName : null;
		if (value !== "excouncil.zz") {
			throw new Error("Bad issuer common name: " + value);
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

	t.Run("ParseIssuerLocality", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.issuer ? cert.issuer.localityName : null;
		if (value !== "Ashtinok") {
			throw new Error("Bad issuer locality: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseIssuerOrganization", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.issuer ? cert.issuer.organizationName : null;
		if (value !== "Exumbran Convention") {
			throw new Error("Bad issuer organization: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseNotBefore", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.notBefore;
		if (value !== "2019-01-01T00:00:00Z") {
			throw new Error("Bad lower bound: " + value)
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseNotAfter", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.notAfter;
		if (value !== "2020-01-01T00:00:00Z") {
			throw new Error("Bad upper bound: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})
}
