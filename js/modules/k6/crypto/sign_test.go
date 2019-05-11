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
	gocrypto "crypto"
	"fmt"
	"testing"

	"github.com/loadimpact/k6/js/common"
	"github.com/stretchr/testify/assert"
)

type Material struct {
	message                string
	messagePart1           string
	messagePart2           string
	messagePart3           string
	rsaPublicKey           string
	rsaPrivateKey          string
	pkcsSignatureHex       string
	pkcsSignatureBase64    string
	pkcsSignatureByteArray string
	pssSignature           string
}
type Expected struct {
	digest          ExpectedDigest
	hexSignature    string
	base64Signature string
	binarySignature string
}
type ExpectedDigest struct {
	SHA256 []byte
}

var message = []byte("They know, get out now!")

var material = Material{
	message:      stringify(enhex(message)),
	messagePart1: stringify("54686579206b6e6f772c"),
	messagePart2: stringify("206765"),
	messagePart3: stringify("74206f7574206e6f7721"),
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
	pkcsSignatureHex: stringify("" +
		"befd8b0a92a44b03324d1908b9e16d209328c38b14b71f8960f5c97c68a00437" +
		"390cc42acab32ce70097a215163917ba28c3dbaa1a88a96e2443fa9abb442082" +
		"2d1e02dcb90b9499741e468316b49a71162871a62a606f07860656f3d33e7ad7" +
		"95a68e21d50aac7d9d79a2e1214fffb36c06e056ebcfe32f30f61838b848f359"),
	pkcsSignatureBase64: stringify("" +
		"vv2LCpKkSwMyTRkIueFtIJMow4sUtx+JYPXJfGigBDc5DMQqyrMs5wCXohUWORe6" +
		"KMPbqhqIqW4kQ/qau0Qggi0eAty5C5SZdB5Ggxa0mnEWKHGmKmBvB4YGVvPTPnrX" +
		"laaOIdUKrH2deaLhIU//s2wG4Fbrz+MvMPYYOLhI81k="),
	pkcsSignatureByteArray: "" +
		"[190,253,139,10,146,164,75,3,50,77,25,8,185,225,109,32,147,40,195," +
		"139,20,183,31,137,96,245,201,124,104,160,4,55,57,12,196,42,202,179," +
		"44,231,0,151,162,21,22,57,23,186,40,195,219,170,26,136,169,110,36," +
		"67,250,154,187,68,32,130,45,30,2,220,185,11,148,153,116,30,70,131," +
		"22,180,154,113,22,40,113,166,42,96,111,7,134,6,86,243,211,62,122," +
		"215,149,166,142,33,213,10,172,125,157,121,162,225,33,79,255,179," +
		"108,6,224,86,235,207,227,47,48,246,24,56,184,72,243,89]",
	pssSignature: stringify("" +
		"9f1d1a9fe59285a4e6c7bba0437bd9fc08aef515db2d7f764700753b93197a53" +
		"f7dc31e37493f7e4a4d5f83958d409ca293accfc0e86d64b65e6049b1112fa19" +
		"445f4ae536fe19dda069db8d68799883af7fea8f1aa638a40c82c4f025e1a94d" +
		"c5e033d9d5f67bf740118f62a112140f317c1e7b1efa821a10359c933696376b"),
}
var expected = Expected{
	digest: ExpectedDigest{
		SHA256: dehex("" +
			"cec66fa2e0ad6286b01c5d975631664f" +
			"54ad80e0ab46907769823e0c33264e8a"),
	},
	hexSignature: stringify("" +
		"befd8b0a92a44b03324d1908b9e16d209328c38b14b71f8960f5c97c68a00437" +
		"390cc42acab32ce70097a215163917ba28c3dbaa1a88a96e2443fa9abb442082" +
		"2d1e02dcb90b9499741e468316b49a71162871a62a606f07860656f3d33e7ad7" +
		"95a68e21d50aac7d9d79a2e1214fffb36c06e056ebcfe32f30f61838b848f359"),
	base64Signature: stringify("" +
		"vv2LCpKkSwMyTRkIueFtIJMow4sUtx+JYPXJfGigBDc5DMQqyrMs5wCXohUWORe6" +
		"KMPbqhqIqW4kQ/qau0Qggi0eAty5C5SZdB5Ggxa0mnEWKHGmKmBvB4YGVvPTPnrX" +
		"laaOIdUKrH2deaLhIU//s2wG4Fbrz+MvMPYYOLhI81k="),
	binarySignature: stringify("" +
		"190:253:139:10:146:164:75:3:50:77:25:8:185:225:109:32:147:40:195" +
		":139:20:183:31:137:96:245:201:124:104:160:4:55:57:12:196:42:202" +
		":179:44:231:0:151:162:21:22:57:23:186:40:195:219:170:26:136:169" +
		":110:36:67:250:154:187:68:32:130:45:30:2:220:185:11:148:153:116" +
		":30:70:131:22:180:154:113:22:40:113:166:42:96:111:7:134:6:86:243" +
		":211:62:122:215:149:166:142:33:213:10:172:125:157:121:162:225:33" +
		":79:255:179:108:6:224:86:235:207:227:47:48:246:24:56:184:72:243:89"),
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

func TestHashPlaintext(t *testing.T) {
	if testing.Short() {
		return
	}

	t.Run("Unsupported", func(t *testing.T) {
		_, err := hashPlaintext(0, message)
		assert.EqualError(t, err, "unsupported hash function: 0")
	})

	t.Run("SHA256", func(t *testing.T) {
		digest, err := hashPlaintext(gocrypto.SHA256, message)
		assert.NoError(t, err)
		assert.Equal(t, expected.digest.SHA256, digest)
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
		`, material.message, material.pkcsSignatureHex))
		assert.EqualError(t, err, "GoError: invalid public key")
	})

	t.Run("HexSignature", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const pem = %s;
		const signer = x509.parsePublicKey(pem);
		const signature = %s;
		const result = crypto.verify(signer, "SHA256", message, signature);
		if (!result) {
			throw new Error("Verification failure");
		}`,
			material.message,
			material.rsaPublicKey,
			material.pkcsSignatureHex,
		))
		assert.NoError(t, err)
	})

	t.Run("Base64Signature", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const pem = %s;
		const signer = x509.parsePublicKey(pem);
		const signature = %s;
		const result = crypto.verify(signer, "SHA256", message, signature);
		if (!result) {
			throw new Error("Verification failure");
		}`,
			material.message,
			material.rsaPublicKey,
			material.pkcsSignatureBase64,
		))
		assert.NoError(t, err)
	})

	t.Run("ByteArraySignature", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const pem = %s;
		const signer = x509.parsePublicKey(pem);
		const signature = %s;
		const result = crypto.verify(signer, "SHA256", message, signature);
		if (!result) {
			throw new Error("Verification failure");
		}`,
			material.message,
			material.rsaPublicKey,
			material.pkcsSignatureByteArray,
		))
		assert.NoError(t, err)
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
		}`,
			material.message,
			material.rsaPublicKey,
			material.pkcsSignatureHex,
		))
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
		}`,
			material.message,
			material.rsaPublicKey,
			material.pssSignature,
		))
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

	t.Run("HexOutput", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const priv = x509.parsePrivateKey(%s);
		const pub = x509.parsePublicKey(%s);
		const hash = "SHA256";
		const signature = crypto.sign(priv, hash, message, "hex");
		const expected = %s;
		if (signature !== expected) {
			throw new Error("Bad hex output");
		}`,
			material.message,
			material.rsaPrivateKey,
			material.rsaPublicKey,
			expected.hexSignature,
		))
		assert.NoError(t, err)
	})

	t.Run("Base64Output", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const priv = x509.parsePrivateKey(%s);
		const pub = x509.parsePublicKey(%s);
		const hash = "SHA256";
		const signature = crypto.sign(priv, hash, message, "base64");
		const expected = %s;
		if (signature !== expected) {
			throw new Error("Bad Base64 output");
		}`,
			material.message,
			material.rsaPrivateKey,
			material.rsaPublicKey,
			expected.base64Signature,
		))
		assert.NoError(t, err)
	})

	t.Run("BinaryOutput", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const priv = x509.parsePrivateKey(%s);
		const pub = x509.parsePublicKey(%s);
		const hash = "SHA256";
		const signature = crypto.sign(priv, hash, message, "binary");
		const expected = %s;
		if (signature.join(":") !== expected) {
			throw new Error("Bad binary output");
		}`,
			material.message,
			material.rsaPrivateKey,
			material.rsaPublicKey,
			expected.binarySignature,
		))
		assert.NoError(t, err)
	})

	t.Run("DefaultOutput", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const priv = x509.parsePrivateKey(%s);
		const pub = x509.parsePublicKey(%s);
		const hash = "SHA256";
		const signature = crypto.sign(priv, hash, message);
		const expected = %s;
		if (signature.join(":") !== expected) {
			throw new Error("Bad binary output");
		}`,
			material.message,
			material.rsaPrivateKey,
			material.rsaPublicKey,
			expected.binarySignature,
		))
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
		crypto.createVerify("SHA256");`))
		assert.NoError(t, err)
	})

	t.Run("SingleUpdate", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const pub = x509.parsePublicKey(%s);
		const signature = %s;
		const verifier = crypto.createVerify("SHA256");
		verifier.update(message);
		const verified = verifier.verify(pub, signature);
		if (!verified) {
			throw new Error("Verification failed");
		}`,
			material.message,
			material.rsaPublicKey,
			material.pkcsSignatureHex,
		))
		assert.NoError(t, err)
	})

	t.Run("MultipleUpdates", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pub = x509.parsePublicKey(%s);
		const signature = %s;
		const verifier = crypto.createVerify("SHA256");
		verifier.update(%s);
		verifier.update(%s);
		verifier.update(%s);
		const verified = verifier.verify(pub, signature);
		if (!verified) {
			throw new Error("Verification failed");
		}`,
			material.rsaPublicKey,
			material.pkcsSignatureHex,
			material.messagePart1,
			material.messagePart2,
			material.messagePart3,
		))
		assert.NoError(t, err)
	})
}

func TestSigner(t *testing.T) {
	if testing.Short() {
		return
	}
	rt := makeRuntime()

	t.Run("Create", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		crypto.createSign("SHA256");`))
		assert.NoError(t, err)
	})

	t.Run("SingleUpdate", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const priv = x509.parsePrivateKey(%s);
		const signer = crypto.createSign("SHA256");
		signer.update(message);
		const signature = signer.sign(priv, "hex");
		const expected = %s;
		if (signature !== expected) {
			throw new Error("Incorrect signature: " + signature);
		}`,
			material.message,
			material.rsaPrivateKey,
			expected.hexSignature,
		))
		assert.NoError(t, err)
	})

	t.Run("MultipleUpdates", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const priv = x509.parsePrivateKey(%s);
		const signer = crypto.createSign("SHA256");
		signer.update(%s);
		signer.update(%s);
		signer.update(%s);
		const signature = signer.sign(priv, "hex");
		const expected = %s;
		if (signature !== expected) {
			throw new Error("Incorrect signature: " + signature);
		}`,
			material.rsaPrivateKey,
			material.messagePart1,
			material.messagePart2,
			material.messagePart3,
			expected.hexSignature,
		))
		assert.NoError(t, err)
	})
}
