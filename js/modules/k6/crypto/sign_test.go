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
