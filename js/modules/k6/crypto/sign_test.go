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

package crypto

import (
	"context"
	gocrypto "crypto"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/modules/k6/crypto/x509"
	"github.com/stretchr/testify/assert"
)

type Material struct {
	message string
	rsaPublicKey string
	rsaPrivateKey string
	pkcsSignature string
	pssSignature string
}
type Expected struct {
	Digest ExpectedDigest
}
type ExpectedDigest struct {
	SHA256 []byte
}

const message = "They know, get out now!"
var material = Material{
	message: stringify(message),
	rsaPublicKey: template(`-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDXMLr/Y/vUtIFY75jj0YXfp6lQ
7iEIbps3BvRE4isTpxs8fXLnLM8LAuJScxiKyrGnj8EMb7LIHkSMBlz6iVj9atY6
EUEm/VHUnElNquzGyBA50TCfpv6NHPaTvOoB45yQbZ/YB4LO+CsT9eIMDZ4tcU9Z
+xD10ifJhhIwpZUFIQIDAQAB
-----END PUBLIC KEY-----`),
	rsaPrivateKey: template(`-----BEGIN RSA PRIVATE KEY-----
MIICXgIBAAKBgQDXMLr/Y/vUtIFY75jj0YXfp6lQ7iEIbps3BvRE4isTpxs8fXLn
LM8LAuJScxiKyrGnj8EMb7LIHkSMBlz6iVj9atY6EUEm/VHUnElNquzGyBA50TCf
pv6NHPaTvOoB45yQbZ/YB4LO+CsT9eIMDZ4tcU9Z+xD10ifJhhIwpZUFIQIDAQAB
AoGBAK42XF2gU2ObktAugUeG++vab5/+eS27ZduBvMX7mEY71jf9k8WGKERQ3GtF
lMvgVz1Bi1eHImUS5Am8qQ+HnEtoD4ewyJKLwGB3tdAA6a2mGY+VoXvRK5GpcBeH
PPGScTA2kJ7Al+ELGcgMuiQHrCLxxxpYNKB90dzE036zmXEBAkEA/0YgJYmBm4J7
/6HQsrvtst6cxQ/JyLWQDvC8T4SONyC4UQWgLzf/eeAl/p09xmcchvV4/A9b5WeF
qkT6V+rl0QJBANfNayXriYzG5YGeUTVEZqd3rIoeSl1g6WIavR6t0W+lgUDWxnJc
buRhgUfDaPzlE6McGBxQPZYt3yrM0F167lECQArrAeb5GZ0AGLOXRSjP1tvGn6fi
A/xcn5uz+ingfoCnGpsEhZRfbcLVrmpUaVb6BANVrmYBdim6osHkj1yBRHECQQCG
5pp8cejiX9NIW7dYHRIuzdjF3nmONe6urRhb/TxXFpbd+WTESJPpoCo4uib/MBQ+
eml4CZD2OGaxUqdOSHKBAkEAtruFjS0IhJstjoOrAS1p5ZAr8Noj5L1DEIgxfAD4
8RbNsyVGZX59oURQ/NqyEs+ME4o/oXuoz8yVBdQqT8G93w==
-----END RSA PRIVATE KEY-----`),
	pkcsSignature: stringify(
		"befd8b0a92a44b03324d1908b9e16d209328c38b14b71f8960f5c97c68a00437" +
		"390cc42acab32ce70097a215163917ba28c3dbaa1a88a96e2443fa9abb442082" +
		"2d1e02dcb90b9499741e468316b49a71162871a62a606f07860656f3d33e7ad7" +
		"95a68e21d50aac7d9d79a2e1214fffb36c06e056ebcfe32f30f61838b848f359"),
	pssSignature: stringify(
		"9f1d1a9fe59285a4e6c7bba0437bd9fc08aef515db2d7f764700753b93197a53" +
		"f7dc31e37493f7e4a4d5f83958d409ca293accfc0e86d64b65e6049b1112fa19" +
		"445f4ae536fe19dda069db8d68799883af7fea8f1aa638a40c82c4f025e1a94d" +
		"c5e033d9d5f67bf740118f62a112140f317c1e7b1efa821a10359c933696376b"),
}
var expected = Expected{
	Digest: ExpectedDigest{
		SHA256: bytes(
			"cec66fa2e0ad6286b01c5d975631664f" +
			"54ad80e0ab46907769823e0c33264e8a"),
	},
}

func bytes (encoded string) []byte {
	decoded, _ := hex.DecodeString(encoded)
	return decoded
}

func stringify(value string) string {
	return fmt.Sprintf(`"%s"`, value)
}

func template(value string) string {
	return fmt.Sprintf("`%s`", value)
}

func makeRuntime() *goja.Runtime {
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	ctx := context.Background()
	ctx = common.WithRuntime(ctx, rt)
	rt.Set("crypto", common.Bind(rt, New(), &ctx))
	rt.Set("x509", common.Bind(rt, x509.New(), &ctx))
	return rt
}

func TestDecodeFunction(t *testing.T) {
	t.Run("Unsupported", func(t *testing.T) {
		_, err := decodeFunction("HyperQuantumHash")
		assert.EqualError(
			t, err, "unsupported hash function: HyperQuantumHash",
		)
	})

	t.Run("SHA256", func(t *testing.T) {
		function, err := decodeFunction("SHA256")
		assert.NoError(t, err)
		assert.Equal(t, gocrypto.SHA256, function)
	})

	t.Run("SHA512", func(t *testing.T) {
		function, err := decodeFunction("SHA512")
		assert.NoError(t, err)
		assert.Equal(t, gocrypto.SHA512, function)
	})
}

func TestHashMessage(t *testing.T) {
	if testing.Short() {
		return
	}

	t.Run("Unsupported", func(t *testing.T) {
		_, err := hashMessage(0, message)
		assert.EqualError(t, err, "unsupported hash function: 0")
	})

	t.Run("SHA256", func(t *testing.T) {
		digest, err := hashMessage(gocrypto.SHA256, message)
		assert.NoError(t, err)
		assert.Equal(t, expected.Digest.SHA256, digest)
	})
}

func TestDecodeSignature(t *testing.T) {
	t.Run("BadFormat", func(t *testing.T) {
		_, err := decodeSignature("bad-signature")
		assert.EqualError(t, err, "unrecognized signature encoding")
	})

	t.Run("Base64", func(t *testing.T) {
		signature, err := decodeSignature("AQIDBA==")
		assert.NoError(t, err)
		assert.Equal(t, bytes("01020304"), signature)
	})

	t.Run("Hex", func(t *testing.T) {
		signature, err := decodeSignature("01020304")
		assert.NoError(t, err)
		assert.Equal(t, bytes("01020304"), signature)
	})
}

func TestVerify(t *testing.T) {
	if testing.Short() {
		return
	}
	rt := makeRuntime()

	t.Run("UnsupportedType", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const signer = { type: "HyperQuantumAlgorithm" };
		const signature = %s;
		const result = crypto.verify(signer, "SHA256", message, signature);
		`, material.message, material.pkcsSignature))
		assert.EqualError(t, err, "GoError: invalid public key")
	})

	t.Run("RSA-PKCS", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const pem = %s;
		const signer = x509.parsePublicKey(pem);
		const signature = %s;
		const result = crypto.verify(signer, "SHA256", message, signature);
		if (!result) {
			throw new Error("Verification failure");
		}`, material.message, material.rsaPublicKey, material.pkcsSignature))
		assert.NoError(t, err)
	})

	t.Run("RSA-PSS", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const pem = %s;
		const signer = x509.parsePublicKey(pem);
		const signature = %s;
		const options = { type: "pss" };
		const result = crypto.verify(
			signer, "SHA256", message, signature, options);
		if (!result) {
			throw new Error("Verification failure");
		}`, material.message, material.rsaPublicKey, material.pssSignature))
		assert.NoError(t, err)
	})
}

func TestSign(t *testing.T) {
	if testing.Short() {
		return
	}
	rt := makeRuntime()

	t.Run("UnsupportedType", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const signer = { type: "HyperQuantumAlgorithm" };
		crypto.sign(signer, "SHA256", message, "hex");
		`, material.message))
		assert.EqualError(t, err, "GoError: invalid private key")
	})

	t.Run("RSA-PKCS", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const priv = x509.parsePrivateKey(%s);
		const pub = x509.parsePublicKey(%s);
		const hash = "SHA256";
		const signature = crypto.sign(priv, hash, message, "hex");
		const result = crypto.verify(pub, hash, message, signature);
		if (!result) {
			throw new Error("Verification failure");
		}`, material.message, material.rsaPrivateKey, material.rsaPublicKey))
		assert.NoError(t, err)
	})

	t.Run("RSA-PSS", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const priv = x509.parsePrivateKey(%s);
		const pub = x509.parsePublicKey(%s);
		const hash = "SHA256";
		const options = { type: "pss" };
		const signature = crypto.sign(priv, hash, message, "hex", options);
		const result = crypto.verify(pub, hash, message, signature, options);
		if (!result) {
			throw new Error("Verification failure");
		}`, material.message, material.rsaPrivateKey, material.rsaPublicKey))
		assert.NoError(t, err)
	})
}

func TestVerifier(t *testing.T) {
	if testing.Short() {
		return
	}
	rt := makeRuntime()

	t.Run("Create", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const verifier = crypto.createVerify("SHA256");`))
		assert.NoError(t, err)
	})
}
