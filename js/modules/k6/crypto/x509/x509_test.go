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

func Material() (string) {
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
	template := fmt.Sprintf("`%s`", pem)
	return template
}

func TestParse(t *testing.T) {
	if testing.Short() {
		return
	}
	rt := MakeRuntime()
	pem := Material()

	t.Run("Failure", func(t *testing.T) {
		_, err := common.RunString(rt, `
		x509.parse("bad-certificate");`)
		assert.Error(t, err)
	})

	t.Run("SignatureAlgorithm", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.signatureAlgorithm;
		if (value !== "SHA256-RSA") {
			throw new Error("Bad signature algorithm: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("Subject", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		if (typeof cert.subject !== "object") {
			throw new Error("Bad subject: " + typeof cert.subject);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("SubjectCommonName", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.commonName : null;
		if (value !== "excouncil.zz") {
			throw new Error("Bad subject common name: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("SubjectCountry", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.country : null;
		if (value !== "ZZ") {
			throw new Error("Bad subject country: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("SubjectPostalCode", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.postalCode : null;
		if (value !== "99999") {
			throw new Error("Bad subject postal code: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("SubjectProvince", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.stateOrProvinceName : null;
		if (value !== "Kopuncezis Krais") {
			throw new Error("Bad subject province: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("SubjectLocality", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.localityName : null;
		if (value !== "Ashtinok") {
			throw new Error("Bad subject locality: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("SubjectStreetAddress", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.streetAddress : null;
		if (value !== "221B Baker Street") {
			throw new Error("Bad subject street address: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("SubjectOrganization", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.organizationName : null;
		if (value !== "Exumbran Convention") {
			throw new Error("Bad subject organization: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("SubjectOrganizationalUnit", func(t *testing.T) {
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
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("Issuer", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		if (typeof cert.issuer !== "object") {
			throw new Error("Bad issuer: " + typeof cert.issuer);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("IssuerCommonName", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.issuer ? cert.issuer.commonName : null;
		if (value !== "excouncil.zz") {
			throw new Error("Bad issuer common name: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("IssuerCountry", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.issuer ? cert.issuer.country : null;
		if (value !== "ZZ") {
			throw new Error("Bad issuer country: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("IssuerProvince", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.issuer ? cert.issuer.stateOrProvinceName : null;
		if (value !== "Kopuncezis Krais") {
			throw new Error("Bad issuer province: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("IssuerLocality", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.issuer ? cert.issuer.localityName : null;
		if (value !== "Ashtinok") {
			throw new Error("Bad issuer locality: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("IssuerOrganization", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.issuer ? cert.issuer.organizationName : null;
		if (value !== "Exumbran Convention") {
			throw new Error("Bad issuer organization: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("NotBefore", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.notBefore;
		if (value !== "2019-01-01T00:00:00Z") {
			throw new Error("Bad lower bound: " + value)
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("NotAfter", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.notAfter;
		if (value !== "2020-01-01T00:00:00Z") {
			throw new Error("Bad upper bound: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("AltNames", func(t *testing.T) {
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
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("FingerPrint", func(t *testing.T) {
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
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("PublicKey", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		if (typeof cert.publicKey !== "object") {
			throw new Error("Bad public key: " + typeof cert.publicKey);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("PublicKeyAlgorithm", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.publicKey ? cert.publicKey.algorithm : null;
		if (value !== "RSA") {
			throw new Error("Bad public key algorithm: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("PublicKeyExponent", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.publicKey ? cert.publicKey.e : null;
		if (value !== 65537) {
			throw new Error("Bad public key exponent: " + value);
		}`, pem))
		assert.NoError(t, err)
	})

	t.Run("PublicKeyModulus", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.publicKey ? cert.publicKey.n : null;
		const expected = [
			223, 249, 234, 71, 180, 36, 28, 62, 84, 141, 177, 118, 53, 2, 175,
			45, 167, 89, 155, 216, 103, 86, 32, 216, 42, 92, 84, 125, 183, 102,
			217, 40, 255, 129, 38, 203, 175, 98, 209, 147, 151, 106, 250, 12,
			123, 236, 135, 45, 80, 80, 76, 253, 148, 141, 13, 36, 76, 47, 182,
			58, 100, 153, 114, 13, 141, 96, 12, 109, 126, 74, 68, 119, 88, 15,
			52, 107, 80, 117, 190, 163, 230, 120, 26, 44, 139, 245, 222, 188,
			210, 237, 123, 83, 91, 98, 160, 218, 229, 244, 128, 31, 24, 139,
			125, 25, 184, 108, 180, 23, 149, 136, 47, 140, 168, 51, 159, 35,
			65, 150, 7, 51, 134, 138, 188, 225, 105, 92, 9, 91, 72, 133, 70,
			195, 131, 105, 184, 191, 198, 177, 130, 191, 70, 6, 37, 159, 255,
			244, 109, 238, 128, 138, 0, 201, 41, 87, 67, 87, 165, 118, 4, 253,
			209, 31, 177, 1, 1, 116, 64, 14, 17, 19, 217, 6, 117, 75, 24, 174,
			236, 129, 134, 115, 245, 226, 203, 222, 17, 113, 197, 215, 138,
			241, 99, 169, 68, 77, 243, 36, 116, 69, 143, 150, 60, 245, 75, 200,
			30, 207, 141, 139, 136, 35, 125, 184, 255, 174, 26, 6, 148, 56,
			253, 124, 249, 158, 249, 117, 3, 126, 124, 89, 27, 96, 209, 0, 207,
			152, 233, 166, 81, 7, 253, 155, 165, 96, 29, 72, 28, 172, 24, 104,
			70, 229, 201, 107, 200, 169, 220, 35
		]
		if (value.join(":") !== expected.join(":")) {
			throw new Error("Bad public key modulus: " + value.join(":"));
		}`, pem))
		assert.NoError(t, err)
	})
}

func TestGetAltNames(t *testing.T) {
	if testing.Short() {
		return
	}
	rt := MakeRuntime()
	pem := Material()

	t.Run("Failure", func(t *testing.T) {
		_, err := common.RunString(rt, `
		x509.getAltNames("bad-certificate");`)
		assert.Error(t, err)
	})

	t.Run("Success", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const altNames = x509.getAltNames(pem);
		if (!(
			Array.isArray(altNames) &&
			altNames.length === 8 &&
			altNames[0] === "council.exumbran.zz" &&
			altNames[1] === "about.excouncil.zz" &&
			altNames[2] === "inquiries@excouncil.zz" &&
			altNames[3] === "press@excouncil.zz" &&
			altNames[4] === "192.0.2.0" &&
			altNames[5] === "192.0.2.25" &&
			altNames[6] === "http://press.excouncil.zz" &&
			altNames[7] === "http://learning.excouncil.zz/index.html"
		)) {
			throw new Error("Bad alt names");
		}`, pem))
		assert.NoError(t, err)
	})
}

func TestGetIssuer(t *testing.T) {
	if testing.Short() {
		return
	}
	rt := MakeRuntime()
	pem := Material()

	t.Run("Failure", func(t *testing.T) {
		_, err := common.RunString(rt, `
		x509.getIssuer("bad-certificate");`)
		assert.Error(t, err)
	})

	t.Run("Success", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const issuer = x509.getIssuer(pem);
		if (!(
			typeof issuer === "object" &&
			issuer.commonName === "excouncil.zz" &&
			issuer.country === "ZZ" &&
			issuer.stateOrProvinceName === "Kopuncezis Krais" &&
			issuer.localityName === "Ashtinok" &&
			issuer.organizationName === "Exumbran Convention"
		)) {
			throw new Error("Bad issuer");
		}`, pem))
		assert.NoError(t, err)
	})
}

func TestGetSubject(t *testing.T) {
	if testing.Short() {
		return
	}
	rt := MakeRuntime()
	pem := Material()

	t.Run("Failure", func(t *testing.T) {
		_, err := common.RunString(rt, `
		x509.getSubject("bad-certificate");`)
		assert.Error(t, err)
	})

	t.Run("Success", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const subject = x509.getSubject(pem);
		if (!(
			typeof subject === "object" &&
			subject.commonName === "excouncil.zz" &&
			subject.country === "ZZ" &&
			subject.postalCode === "99999" &&
			subject.stateOrProvinceName === "Kopuncezis Krais" &&
			subject.localityName === "Ashtinok" &&
			subject.streetAddress === "221B Baker Street" &&
			subject.organizationName === "Exumbran Convention" &&
			Array.isArray(subject.organizationalUnitName) &&
			subject.organizationalUnitName.length === 2 &&
			subject.organizationalUnitName[0] === "Exumbran Council" &&
			subject.organizationalUnitName[1] ===
				"Exumbran Janitorial Service"
		)) {
			throw new Error("Bad subject");
		}`, pem))
		assert.NoError(t, err)
	})
}
