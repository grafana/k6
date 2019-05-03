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
MIID8jCCAtqgAwIBAgICBNIwDQYJKoZIhvcNAQELBQAwgbcxCzAJBgNVBAYTAlpa
MRkwFwYDVQQIExBLb3B1bmNlemlzIEtyYWlzMREwDwYDVQQHEwhBc2h0aW5vazEa
MBgGA1UECRMRMjIxQiBCYWtlciBTdHJlZXQxDjAMBgNVBBETBTk5OTk5MRwwGgYD
VQQKExNFeHVtYnJhbiBDb252ZW50aW9uMRkwFwYDVQQLExBFeHVtYnJhbiBDb3Vu
Y2lsMRUwEwYDVQQDEwxleGNvdW5jaWwuenowIhgPMDAwMTAxMDEwMDAwMDBaGA8w
MDAxMDEwMTAwMDAwMFowgbcxCzAJBgNVBAYTAlpaMRkwFwYDVQQIExBLb3B1bmNl
emlzIEtyYWlzMREwDwYDVQQHEwhBc2h0aW5vazEaMBgGA1UECRMRMjIxQiBCYWtl
ciBTdHJlZXQxDjAMBgNVBBETBTk5OTk5MRwwGgYDVQQKExNFeHVtYnJhbiBDb252
ZW50aW9uMRkwFwYDVQQLExBFeHVtYnJhbiBDb3VuY2lsMRUwEwYDVQQDEwxleGNv
dW5jaWwuenowggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDOIAR7WIw+
Ci7FDsJjIKLsU5rmtEg9jDx+nnBHhIYoQH0ekNmDqaHjfQ9m2f0vNWl2zKT5bZ74
wlK84ZQdLki9cYPwQ2xWInXDxaYr4SslPchZWyDlm9r1YUKcOsXww5YnkbmvuB/x
tLpUv1jHMJXGMxCsWQbUPFLwHlSSPGGICb0mh+dppvXrLgeyS20P7m+tglDf2MZT
/bkSADKvPJUzxopUUhmvgkBRLa00Z85pEzhECaV6wIhb+iOTzFqom0z1Yjab50Yx
rIYSrVVXIh0wzqWOYq9BBruIBZG91+7UG4JP67wXl9YicOnb5M9EJNgwA2nPqgWY
jzHFDPWTqsQfAgMBAAGjAjAAMA0GCSqGSIb3DQEBCwUAA4IBAQAEcrqIPcS6Fdj7
zGcIcBGRIisnJZ6VnPR11977HU40lhz4mJZOjG1Xws6iTE75T5CSEX+YY0LxM6Z4
oPFUB0fhxRX71d0MvE4EfiU/h+cvyjOvFPywCNfl+UUqYm3Pg1+KYxRXS/zsHo9S
KDWX680pyoxoahLESf/XaU12mgC21ZlKCfkgZudknKh5laQTZKsrMqEvXNjSKmoz
Ewk4pwoVKbp89/MQmr28GIwAAuWUZumTx5De+7C3gXqweYlZ2wH0/PwRG5WD04Pq
kN9dNqkkyk3hVpu24y8D850/4k3CeA0AfFOXFXa6E6v5YGJ+8m0qcxvsGrPIt0K9
rxYbUhqc
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
		const value = cert.subject;
		if (typeof value !== "object") {
			throw new Error("Bad subject");
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseSubjectCountry", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.countryName : null;
		if (value !== "ZZ") {
			throw new Error("Bad country: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseSubjectPostalCode", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.postalCode : null;
		if (value !== "99999") {
			throw new Error("Bad postal code: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})

	t.Run("ParseSubjectProvince", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pem = %s;
		const cert = x509.parse(pem);
		const value = cert.subject ? cert.subject.stateOrProvinceName : null;
		if (value !== "Kopuncezis Krais") {
			throw new Error("Bad province: " + value);
		}`, pemTemplate))
		assert.NoError(t, err)
	})
}
