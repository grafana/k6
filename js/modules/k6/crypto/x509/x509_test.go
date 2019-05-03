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
MIIGOjCCBCKgAwIBAgIJAKgPBlWkaBXiMA0GCSqGSIb3DQEBCwUAMIGxMQswCQYD
VQQGEwJaWjEZMBcGA1UECAwQS29wdW5jZXppcyBLcmFpczERMA8GA1UEBwwIQXNo
dGlub2sxHDAaBgNVBAoME0V4dW1icmFuIENvbnZlbnRpb24xGTAXBgNVBAsMEEV4
dW1icmFuIENvdW5jaWwxFTATBgNVBAMMDGV4Y291bmNpbC56ejEkMCIGCSqGSIb3
DQEJARYVZXhjb3VuY2lsQGV4YW1wbGUuY29tMB4XDTE5MDMyNzExNTc0MloXDTIw
MDMyNjExNTc0MlowgbExCzAJBgNVBAYTAlpaMRkwFwYDVQQIDBBLb3B1bmNlemlz
IEtyYWlzMREwDwYDVQQHDAhBc2h0aW5vazEcMBoGA1UECgwTRXh1bWJyYW4gQ29u
dmVudGlvbjEZMBcGA1UECwwQRXh1bWJyYW4gQ291bmNpbDEVMBMGA1UEAwwMZXhj
b3VuY2lsLnp6MSQwIgYJKoZIhvcNAQkBFhVleGNvdW5jaWxAZXhhbXBsZS5jb20w
ggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDIzUOPQvctDtoXILQmnj8p
Kq1Q18kVEql/zWgwm1bVnwWoDjXmCO0rkSGDRgYV9he0PcOq7BAbZqGDn7aqR37C
JsC/GN8GNCBp5Vvl7vDQrO8ReB3bTi9fNWNP2amALlEtCdWrtVsNn0kbWh4IOZt2
kzNdNfhRSBXw2L1FcXg+/t5cotJ4EsX36tCK6OsdIJhAL/npZJayqP4jvpTTH+zs
PJRWAyYq5AP0Uc24nTkW4aiXuBMhGQ4oGSbFI11OIo6hKwxzv33pgedCs/NRgdde
k9+ouPx1cRi5S/rkueIe9rTgEFgP54xEV49mMTyOBy84sBBZIN3a+Z5p1yHW8JEg
dDXGEbDzO52RhqQm8DBbqaz+DLzFYqNEw0C2OKYbLRs0cRzoA6h2fsvHoFICCLlj
PYIl0tTnrdPdPENJqPMiM73CB4Gv+YBXGS5fsBRycTP2q4ZBN6Efi2XG2mOqiPkd
QYIMYvpcl0CTfDdhNGO6RStrhG4x/EDOXOSTe+ocYyCQD0+65YTNZM36n5TF+IHC
csZqeSBd3OPLEux7FLZ4NDtKbtur1FdZLU5YfZ2eYo74HMQ+NnxbaHo7Q+FB5ru1
BFEbbD6HV3eO7/LxW0ZjbLNPpzamhKJwHcc3Ha7lx/7n6HeuJqCJa5zZlwQgqEQf
SiVqxOtHr5O60LgpfqK4xQIDAQABo1MwUTAdBgNVHQ4EFgQU2RIeLkEuMZ7pLzsY
RPeHg9/3HLAwHwYDVR0jBBgwFoAU2RIeLkEuMZ7pLzsYRPeHg9/3HLAwDwYDVR0T
AQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAgEAGjhiS5WaT7OmqHYCXZ9VDilG
099aCrN3ujOq0ba/RU8p5dxwG42/RM3ta1ne2/EIIP3brD0K4opv9TmQKvunOmqs
HUMu7t6h9dclPA5zELZHySwXnY2pFOEEtnDPGeCxL5RyzVYWgq3uDlf3Fu/V37uV
zrlS8TNjutrFAX3EeiN39ypf9TDuODB1gVqggXi1IbHfX4d6MrusTUQnFu4IW1Ds
SlmUx63hsJ+UQ13BpwkwlzZThrlAIYZ/iACfYmc3hruizR1RE/fuZmXHVcTo/j65
aEBqDvtawcRXzAXxvW2jafYWBKR3p+dxzyw3pd5cc0tSI/n+dufO8g8nOtXRSG8X
C+q0UVYBLuA5BTsoTu8bmFvvnVbBGd57UYVDKpNvLTLcXIBa1PmdbCTUlerdC65h
H1sJ3neh3jr4V8n1EVxB7Rm/svZ+lIJzghiGYeCyTrJtt57MOB5HDkLD/CUgnM3G
u+okHcPcSabEx+iOxHSIq0uWic6imecsI0fWgF/J51GUfj2PgCTgWV6zgCv/mUqG
MsJ6ZL2PQuLpfZZl2CSWwiGn5XaJ5twexmYz4Q2yXK811nr0hXQJ+Mg07Vn7xR67
ETQ7XJIJJB+/I4LIId4rB3OAkbZ4LcaQW/AmdKJ6y7lLgdTRjT/sTniQXCdKPzrG
WGHBt5f8oA8IPXrCfzI=
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
}
