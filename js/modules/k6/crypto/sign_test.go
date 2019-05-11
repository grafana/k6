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
	messageString          string
	messageBytes           []byte
	messageHex             string
	messagePart1           string
	messagePart2           string
	messagePart3           string
	rsaPublicKey           string
	rsaPrivateKey          string
	dsaPublicKey           string
	dsaPrivateKey          string
	ecdsaPublicKey         string
	ecdsaPrivateKey        string
	pkcsSignatureHex       string
	pkcsSignatureBase64    string
	pkcsSignatureByteArray string
	pssSignature           string
	dsaSignature           string
	ecdsaSignature         string
}
type Expected struct {
	hexSignature    string
	base64Signature string
	binarySignature string
}

const message = "They know, get out now!"

var material = Material{
	messageString: stringify(message),
	messageBytes:  []byte(message),
	messageHex:    stringify(enhex([]byte(message))),
	messagePart1:  stringify("54686579206b6e6f772c"),
	messagePart2:  stringify("206765"),
	messagePart3:  stringify("74206f7574206e6f7721"),
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
	dsaPublicKey: template(`-----BEGIN PUBLIC KEY-----
MIIDRjCCAjkGByqGSM44BAEwggIsAoIBAQCKv/tJtwgLJGrvas2YQmqgjfoQ5s9u
QRO9+9ELCu4Lstn9nsjmER/+CgXCrAQG/jUKdT6tpz9bUVYspcn+gF2YkDugSbMb
4UciFbWjuFSYD7xIe7APGprgogJZeNO6v9vFWLJak4d35Olej1HLQpVPtz5NdR9u
nh1aB9sODuRtnZDJsGWuEYbNN0nbjQReBLwbnJCRo5p8nL+FVfKmFGKC9KK3P3TN
M6u5XU8KLlXZ40VNbtiIfzKr4aeHy8ob1+0Jy4nirxt2WJPxYW/tbawhHkJXB8R5
73CRoxH+2xx5WTsYjSIdI3h+mGufi+nmnO2YQguVMrCJ5AaGlrw7V/KRAiEA9d+j
pJs9sAccw/DWAnRm+UuvcZ9CFT4ttoPc0UWLKmkCggEAVKjNXly/1gzzKUGZVqYS
GwigArV109g1vCWg4QavGgVLLqlbSxiz6+0kuLK6vauMipr0i57FRzh0EpZ6faah
wQ/LbhqXQU+S55m5rFh1eUh428htlOhG5hQYe/EWiqD7nsWzl6z8+/Y5uuq6BksT
Lej3nQJWiLY09SNnKcjVZYe5vs+8ASx6vT2qkGV/UvEEKma1I+0MUDJcHFjnTNwU
f7GTRmtt7YgDlYX13e9ar89lSPxjXo+r3BSm5iNMC8eG3e91yoT0G4ShI6zf8LzZ
baN2PK+Bwa+pBYEqvtp69G0NPO+jadx62HoAKFw+BXh2XU1fS09tX7Z2lfsIhIZS
lQOCAQUAAoIBADJjifXbelASJEgBC9MNcFQM3aeLpMhSXcsIBR7mDGISnig84Hwo
qJT76lQznzqBrGjYTaNEA6UC6XXda19wugKSDWJ6SvnMekkvOfIeqUom2sd43fYE
SXJZX6gnbiqShNVwIK+aKpAWn1sqbWkzCcIL2BJdm7ETJeW3+yOXdLCa6p3JbZQS
gVDZ+GPviNHSX1hyF4FjQW2rrQix5RhEJSV988j6NEZFbuTf7INwpDOg9htRoRih
Rk3eh9kiR6iHl4ZUqSlefyVS40mzlKpPEXtKW2PFE6QLcQLPDzX+JjjAomgs/DIK
ia2TF+3H94NY/zOOcerq+BXwVYZmxhDSOrI=
-----END PUBLIC KEY-----`),
	dsaPrivateKey: template(`-----BEGIN DSA PRIVATE KEY-----
MIIDVQIBAAKCAQEAir/7SbcICyRq72rNmEJqoI36EObPbkETvfvRCwruC7LZ/Z7I
5hEf/goFwqwEBv41CnU+rac/W1FWLKXJ/oBdmJA7oEmzG+FHIhW1o7hUmA+8SHuw
Dxqa4KICWXjTur/bxViyWpOHd+TpXo9Ry0KVT7c+TXUfbp4dWgfbDg7kbZ2QybBl
rhGGzTdJ240EXgS8G5yQkaOafJy/hVXyphRigvSitz90zTOruV1PCi5V2eNFTW7Y
iH8yq+Gnh8vKG9ftCcuJ4q8bdliT8WFv7W2sIR5CVwfEee9wkaMR/tsceVk7GI0i
HSN4fphrn4vp5pztmEILlTKwieQGhpa8O1fykQIhAPXfo6SbPbAHHMPw1gJ0ZvlL
r3GfQhU+LbaD3NFFiyppAoIBAFSozV5cv9YM8ylBmVamEhsIoAK1ddPYNbwloOEG
rxoFSy6pW0sYs+vtJLiyur2rjIqa9IuexUc4dBKWen2mocEPy24al0FPkueZuaxY
dXlIeNvIbZToRuYUGHvxFoqg+57Fs5es/Pv2ObrqugZLEy3o950CVoi2NPUjZynI
1WWHub7PvAEser09qpBlf1LxBCpmtSPtDFAyXBxY50zcFH+xk0Zrbe2IA5WF9d3v
Wq/PZUj8Y16Pq9wUpuYjTAvHht3vdcqE9BuEoSOs3/C82W2jdjyvgcGvqQWBKr7a
evRtDTzvo2nceth6AChcPgV4dl1NX0tPbV+2dpX7CISGUpUCggEAMmOJ9dt6UBIk
SAEL0w1wVAzdp4ukyFJdywgFHuYMYhKeKDzgfCiolPvqVDOfOoGsaNhNo0QDpQLp
dd1rX3C6ApINYnpK+cx6SS858h6pSibax3jd9gRJcllfqCduKpKE1XAgr5oqkBaf
WyptaTMJwgvYEl2bsRMl5bf7I5d0sJrqncltlBKBUNn4Y++I0dJfWHIXgWNBbaut
CLHlGEQlJX3zyPo0RkVu5N/sg3CkM6D2G1GhGKFGTd6H2SJHqIeXhlSpKV5/JVLj
SbOUqk8Re0pbY8UTpAtxAs8PNf4mOMCiaCz8MgqJrZMX7cf3g1j/M45x6ur4FfBV
hmbGENI6sgIgYgr/yUCfYfJQlBj9d9WXfpeJxgiknTSkwB2hjJKsYBg=
-----END DSA PRIVATE KEY-----`),
	ecdsaPublicKey: template(`-----BEGIN PUBLIC KEY-----
MIGbMBAGByqGSM49AgEGBSuBBAAjA4GGAAQBcX9o+j4axEZ3TzUq2DjuvAboLKTW
lco1HZxjO51MNekI64fDkQIYp7PsbNM2lPvZQt3oglDHxlp2Au1qVcZAs1sBhd09
cbsUjd2HQce8c8B+xoxp4H0PvCGeNxdDqo0ibuPjvutma0IxcJEidxgFRHZ868EU
gl27czkKiDZRgtLjEDE=
-----END PUBLIC KEY-----`),
	ecdsaPrivateKey: template(`-----BEGIN EC PRIVATE KEY-----
MIHcAgEBBEIBrRcLjkYGU/3aWL05hmivvGCc2xIzRkZd6IUamAuL4pR1kMLlW0ui
pYKpBBJhUY6ucUI5mPOzV7CcU9rCER/msb+gBwYFK4EEACOhgYkDgYYABAFxf2j6
PhrERndPNSrYOO68BugspNaVyjUdnGM7nUw16Qjrh8ORAhins+xs0zaU+9lC3eiC
UMfGWnYC7WpVxkCzWwGF3T1xuxSN3YdBx7xzwH7GjGngfQ+8IZ43F0OqjSJu4+O+
62ZrQjFwkSJ3GAVEdnzrwRSCXbtzOQqINlGC0uMQMQ==
-----END EC PRIVATE KEY-----`),
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
	dsaSignature: stringify("" +
		"MEUCIQCyOt6wOt5muIc9Id2LD/sq1zwvZXKX2dnBEwh6BcA/OAIgMaCJyX/KCWqo" +
		"khCpc0x8THK/vBLrR8xKRBA2Ji6xlHo="),
	ecdsaSignature: stringify("" +
		"MIGIAkIB7qRptN3LJw48PgUXe5HmFcxZCjlN0k+X38kngixiQl6FUemtLpMgx74m" +
		"+7F+OTepOVO+hKi0QHU05zqk8/mDe4wCQgH1eT04Gw2ggjxY+qBf2+RfVHlGk1un" +
		"Qs6cZEu32hLYIfNmA8ujlIFeApRV5SohmAoeN7jqXewYGszPH82t4Nvmfw=="),
}
var expected = Expected{
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

	t.Run("sha256", func(t *testing.T) {
		function, err := decodeFunction("sha256")
		assert.NoError(t, err)
		assert.Equal(t, gocrypto.SHA256, function)
	})

	t.Run("sha512", func(t *testing.T) {
		function, err := decodeFunction("sha512")
		assert.NoError(t, err)
		assert.Equal(t, gocrypto.SHA512, function)
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
		const result = crypto.verify(signer, "sha256", message, signature);
		`, material.messageHex, material.pkcsSignatureHex))
		assert.EqualError(t, err, "GoError: invalid public key")
	})

	t.Run("HexSignature", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const pem = %s;
		const signer = x509.parsePublicKey(pem);
		const signature = %s;
		const result = crypto.verify(signer, "sha256", message, signature);
		if (!result) {
			throw new Error("Verification failure");
		}`,
			material.messageHex,
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
		const result = crypto.verify(signer, "sha256", message, signature);
		if (!result) {
			throw new Error("Verification failure");
		}`,
			material.messageHex,
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
		const result = crypto.verify(signer, "sha256", message, signature);
		if (!result) {
			throw new Error("Verification failure");
		}`,
			material.messageHex,
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
		const result = crypto.verify(signer, "sha256", message, signature);
		if (!result) {
			throw new Error("Verification failure");
		}`,
			material.messageHex,
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
			signer, "sha256", message, signature, options);
		if (!result) {
			throw new Error("Verification failure");
		}`,
			material.messageHex,
			material.rsaPublicKey,
			material.pssSignature,
		))
		assert.NoError(t, err)
	})

	t.Run("DSA", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const signer = x509.parsePublicKey(%s);
		const signature = %s;
		const verified = crypto.verify(signer, "sha256", message, signature);
		if (!verified) {
			throw new Error("Verification failure");
		}`,
			material.messageHex,
			material.dsaPublicKey,
			material.dsaSignature,
		))
		assert.NoError(t, err)
	})

	t.Run("ECDSA", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const signer = x509.parsePublicKey(%s);
		const signature = %s;
		const verified = crypto.verify(signer, "sha1", message, signature);
		if (!verified) {
			throw new Error("Verification failure");
		}`,
			material.messageHex,
			material.ecdsaPublicKey,
			material.ecdsaSignature,
		))
		assert.NoError(t, err)
	})
}

func TestVerifyString(t *testing.T) {
	if testing.Short() {
		return
	}
	rt := makeRuntime()

	t.Run("Success", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const signer = x509.parsePublicKey(%s);
		const signature = %s;
		const verified = crypto.verifyString(
			signer, "sha256", message, signature);
		if (!verified) {
			throw new Error("Verification failure");
		}`,
			material.messageString,
			material.rsaPublicKey,
			material.pkcsSignatureHex,
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
		crypto.sign(signer, "sha256", message, "hex");
		`, material.messageHex))
		assert.EqualError(t, err, "GoError: invalid private key")
	})

	t.Run("RSA-PKCS", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const priv = x509.parsePrivateKey(%s);
		const pub = x509.parsePublicKey(%s);
		const hash = "sha256";
		const signature = crypto.sign(priv, hash, message, "hex");
		const result = crypto.verify(pub, hash, message, signature);
		if (!result) {
			throw new Error("Verification failure");
		}`,
			material.messageHex,
			material.rsaPrivateKey,
			material.rsaPublicKey,
		))
		assert.NoError(t, err)
	})

	t.Run("RSA-PSS", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const priv = x509.parsePrivateKey(%s);
		const pub = x509.parsePublicKey(%s);
		const hash = "sha256";
		const options = { type: "pss" };
		const signature = crypto.sign(priv, hash, message, "hex", options);
		const result = crypto.verify(pub, hash, message, signature, options);
		if (!result) {
			throw new Error("Verification failure");
		}`,
			material.messageHex,
			material.rsaPrivateKey,
			material.rsaPublicKey,
		))
		assert.NoError(t, err)
	})

	t.Run("DSA", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const priv = x509.parsePrivateKey(%s);
		const pub = x509.parsePublicKey(%s);
		const signature = crypto.sign(priv, "sha256", message, "hex");
		const verified = crypto.verify(pub, "sha256", message, signature);
		if (!verified) {
			throw new Error("Verification failure");
		}`,
			material.messageHex,
			material.rsaPrivateKey,
			material.rsaPublicKey,
		))
		assert.NoError(t, err)
	})

	t.Run("ECDSA", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const priv = x509.parsePrivateKey(%s);
		const pub = x509.parsePublicKey(%s);
		const signature = crypto.sign(priv, "sha1", message, "hex")
		const verified = crypto.verify(pub, "sha1", message, signature);
		if (!verified) {
			throw new Error("Verification failure");
		}`,
			material.messageHex,
			material.ecdsaPrivateKey,
			material.ecdsaPublicKey,
		))
		assert.NoError(t, err)
	})

	t.Run("HexOutput", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const priv = x509.parsePrivateKey(%s);
		const pub = x509.parsePublicKey(%s);
		const hash = "sha256";
		const signature = crypto.sign(priv, hash, message, "hex");
		const expected = %s;
		if (signature !== expected) {
			throw new Error("Bad hex output");
		}`,
			material.messageHex,
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
		const hash = "sha256";
		const signature = crypto.sign(priv, hash, message, "base64");
		const expected = %s;
		if (signature !== expected) {
			throw new Error("Bad Base64 output");
		}`,
			material.messageHex,
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
		const hash = "sha256";
		const signature = crypto.sign(priv, hash, message, "binary");
		const expected = %s;
		if (signature.join(":") !== expected) {
			throw new Error("Bad binary output");
		}`,
			material.messageHex,
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
		const hash = "sha256";
		const signature = crypto.sign(priv, hash, message);
		const expected = %s;
		if (signature.join(":") !== expected) {
			throw new Error("Bad binary output");
		}`,
			material.messageHex,
			material.rsaPrivateKey,
			material.rsaPublicKey,
			expected.binarySignature,
		))
		assert.NoError(t, err)
	})
}

func TestSignString(t *testing.T) {
	if testing.Short() {
		return
	}
	rt := makeRuntime()

	t.Run("Success", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const priv = x509.parsePrivateKey(%s);
		const pub = x509.parsePublicKey(%s);
		const hash = "sha256";
		const signature = crypto.signString(priv, hash, message, "hex");
		const verified = crypto.verifyString(pub, hash, message, signature);
		if (!verified) {
			throw new Error("Verification failure");
		}`,
			material.messageString,
			material.rsaPrivateKey,
			material.rsaPublicKey,
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
		crypto.createVerify("sha256");`))
		assert.NoError(t, err)
	})

	t.Run("SingleUpdate", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const pub = x509.parsePublicKey(%s);
		const signature = %s;
		const verifier = crypto.createVerify("sha256");
		verifier.update(message);
		const verified = verifier.verify(pub, signature);
		if (!verified) {
			throw new Error("Verification failed");
		}`,
			material.messageHex,
			material.rsaPublicKey,
			material.pkcsSignatureHex,
		))
		assert.NoError(t, err)
	})

	t.Run("MultipleUpdates", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const pub = x509.parsePublicKey(%s);
		const signature = %s;
		const verifier = crypto.createVerify("sha256");
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
		crypto.createSign("sha256");`))
		assert.NoError(t, err)
	})

	t.Run("SingleUpdate", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const message = %s;
		const priv = x509.parsePrivateKey(%s);
		const signer = crypto.createSign("sha256");
		signer.update(message);
		const signature = signer.sign(priv, "hex");
		const expected = %s;
		if (signature !== expected) {
			throw new Error("Incorrect signature: " + signature);
		}`,
			material.messageHex,
			material.rsaPrivateKey,
			expected.hexSignature,
		))
		assert.NoError(t, err)
	})

	t.Run("MultipleUpdates", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const priv = x509.parsePrivateKey(%s);
		const signer = crypto.createSign("sha256");
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
