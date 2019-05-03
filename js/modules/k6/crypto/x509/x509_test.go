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
MIIE6zCCA9OgAwIBAgICBNIwDQYJKoZIhvcNAQELBQAwgdsxCzAJBgNVBAYTAlpa
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
ggEPADCCAQoCggEBAN/56ke0JBw+VI2xdjUCry2nWZvYZ1Yg2CpcVH23Ztko/4Em
y69i0ZOXavoMe+yHLVBQTP2UjQ0kTC+2OmSZcg2NYAxtfkpEd1gPNGtQdb6j5nga
LIv13rzS7XtTW2Kg2uX0gB8Yi30ZuGy0F5WIL4yoM58jQZYHM4aKvOFpXAlbSIVG
w4NpuL/GsYK/RgYln//0be6AigDJKVdDV6V2BP3RH7EBAXRADhET2QZ1Sxiu7IGG
c/Xiy94RccXXivFjqURN8yR0RY+WPPVLyB7PjYuII324/64aBpQ4/Xz5nvl1A358
WRtg0QDPmOmmUQf9m6VgHUgcrBhoRuXJa8ip3CMCAwEAAaOBtjCBszCBsAYDVR0R
BIGoMIGlghNjb3VuY2lsLmV4dW1icmFuLnp6ghJhYm91dC5leGNvdW5jaWwuenqB
FmlucXVpcmllc0BleGNvdW5jaWwuenqBEnByZXNzQGV4Y291bmNpbC56eocEwAAC
AIcEwAACGYYZaHR0cDovL3ByZXNzLmV4Y291bmNpbC56eoYnaHR0cDovL2xlYXJu
aW5nLmV4Y291bmNpbC56ei9pbmRleC5odG1sMA0GCSqGSIb3DQEBCwUAA4IBAQCI
lVr5wEwAznmV/MxE5vc4gfHppYzszssPPvvGs0QjuDe9AbN26nEuriren2bcGbcS
pVhl25tfIJd5rvgvWKz+nTQCEGVI4BDFio0Jt5+7CADOsSSFGYQIu0BrjA3vCs87
gzg3dNaCY65aH0cJE/dVwiS/F2XTr1zvr+uBPExgrA21+FSIlHM0Dot+VGKdCLEO
6HugOCDBdzKF2hsHeI5LvgXUX5zQ0gnsd93+QuxUmiN7QZZs8tDMD/+efo4OWvp/
xytSVXVn+cECQLg9hVn+Zx3XO2FA0eOzaWEONnUGghT/Ivw06lUxis5tkAoAU93d
ddBqJe0XUeAX8Zr6EJ82
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

	t.Run("ParseAltNames", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const values = cert.altNames;
		if (!(
			values.length === 8 &&
			values[0] === "council.exumbran.zz" &&
			values[1] === "about.excouncil.zz" &&
			values[2] === "inquiries@excouncil.zz" &&
			values[3] === "press@excouncil.zz" &&
			values[4] === "192.0.2.0" &&
			values[5] === "192.0.2.25" &&
			values[6] === "http://press.excouncil.zz" &&
			values[7] === "http://learning.excouncil.zz/index.html"
		)) {
			throw new Error("Bad alt names: " + values.join(", "));
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseFingerPrint", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.fingerPrint;
		const expected = [
			85, 119, 3, 199, 150, 144, 202, 145, 178, 46,
			205, 132, 37, 235, 251, 208, 139, 161, 143, 14
		]
		if (value.join("") !== expected.join("")) {
			throw new Error("Bad fingerprint: " + value.join(":"));
		}`, pemTemplate))
		assert.NoError(t, err)
	})
}
