/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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
	"crypto/rand"
	"errors"
	"testing"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/stretchr/testify/assert"
)

type MockReader struct{}

func (MockReader) Read(p []byte) (n int, err error) {
	return -1, errors.New("Contrived failure")
}

func TestCryptoAlgorithms(t *testing.T) {
	if testing.Short() {
		return
	}

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	ctx := context.Background()
	ctx = common.WithRuntime(ctx, rt)
	rt.Set("crypto", common.Bind(rt, New(), &ctx))

	t.Run("RandomBytesSuccess", func(t *testing.T) {
		_, err := common.RunString(rt, `
		let bytes = crypto.randomBytes(5);
		if (bytes.length !== 5) {
			throw new Error("Incorrect size: " + bytes.length);
		}`)

		assert.NoError(t, err)
	})

	t.Run("RandomBytesInvalidSize", func(t *testing.T) {
		_, err := common.RunString(rt, `
		crypto.randomBytes(-1);`)

		assert.Error(t, err)
	})

	t.Run("RandomBytesFailure", func(t *testing.T) {
		SavedReader := rand.Reader
		rand.Reader = MockReader{}
		_, err := common.RunString(rt, `
		crypto.randomBytes(5);`)
		rand.Reader = SavedReader

		assert.Error(t, err)
	})

	t.Run("MD4", func(t *testing.T) {
		_, err := common.RunString(rt, `
		const correct = "aa010fbc1d14c795d86ef98c95479d17";
		let hash = crypto.md4("hello world", "hex");
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)
		assert.NoError(t, err)
	})

	t.Run("MD5", func(t *testing.T) {
		_, err := common.RunString(rt, `
		const correct = "5eb63bbbe01eeed093cb22bb8f5acdc3";
		let hash = crypto.md5("hello world", "hex");
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)

		assert.NoError(t, err)
	})

	t.Run("SHA1", func(t *testing.T) {
		_, err := common.RunString(rt, `
		const correct = "2aae6c35c94fcfb415dbe95f408b9ce91ee846ed";
		let hash = crypto.sha1("hello world", "hex");
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)

		assert.NoError(t, err)
	})

	t.Run("SHA256", func(t *testing.T) {
		_, err := common.RunString(rt, `
		const correct = "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9";
		let hash = crypto.sha256("hello world", "hex");
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)

		assert.NoError(t, err)
	})

	t.Run("SHA384", func(t *testing.T) {
		_, err := common.RunString(rt, `
		const correct = "fdbd8e75a67f29f701a4e040385e2e23986303ea10239211af907fcbb83578b3e417cb71ce646efd0819dd8c088de1bd";
		let hash = crypto.sha384("hello world", "hex");
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)

		assert.NoError(t, err)
	})

	t.Run("SHA512", func(t *testing.T) {
		_, err := common.RunString(rt, `
		const correct = "309ecc489c12d6eb4cc40f50c902f2b4d0ed77ee511a7c7a9bcd3ca86d4cd86f989dd35bc5ff499670da34255b45b0cfd830e81f605dcf7dc5542e93ae9cd76f";
		let hash = crypto.sha512("hello world", "hex");
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)

		assert.NoError(t, err)
	})

	t.Run("SHA512_224", func(t *testing.T) {
		_, err := common.RunString(rt, `
		let hash = crypto.sha512_224("hello world", "hex");
		const correct = "22e0d52336f64a998085078b05a6e37b26f8120f43bf4db4c43a64ee";
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)

		assert.NoError(t, err)
	})

	t.Run("SHA512_256", func(t *testing.T) {
		_, err := common.RunString(rt, `
		let hash = crypto.sha512_256("hello world", "hex");
		const correct = "0ac561fac838104e3f2e4ad107b4bee3e938bf15f2b15f009ccccd61a913f017";
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)

		assert.NoError(t, err)
	})

	t.Run("RIPEMD160", func(t *testing.T) {
		_, err := common.RunString(rt, `
		let hash = crypto.ripemd160("hello world", "hex");
		const correct = "98c615784ccb5fe5936fbc0cbe9dfdb408d92f0f";
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)

		assert.NoError(t, err)
	})
}

func TestStreamingApi(t *testing.T) {
	if testing.Short() {
		return
	}

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	root, _ := lib.NewGroup("", nil)
	state := &lib.State{Group: root}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("crypto", common.Bind(rt, New(), &ctx))

	// Empty strings are still hashable
	t.Run("Empty", func(t *testing.T) {
		_, err := common.RunString(rt, `
		const correctHex = "d41d8cd98f00b204e9800998ecf8427e";

		let hasher = crypto.createHash("md5");

		const resultHex = hasher.digest("hex");
		if (resultHex !== correctHex) {
			throw new Error("Hex encoding mismatch: " + resultHex);
		}`)

		assert.NoError(t, err)
	})

	t.Run("UpdateOnce", func(t *testing.T) {
		_, err := common.RunString(rt, `
		const correctHex = "5eb63bbbe01eeed093cb22bb8f5acdc3";

		let hasher = crypto.createHash("md5");
		hasher.update("hello world");

		const resultHex = hasher.digest("hex");
		if (resultHex !== correctHex) {
			throw new Error("Hex encoding mismatch: " + resultHex);
		}`)

		assert.NoError(t, err)
	})

	t.Run("UpdateMultiple", func(t *testing.T) {
		_, err := common.RunString(rt, `
		const correctHex = "5eb63bbbe01eeed093cb22bb8f5acdc3";

		let hasher = crypto.createHash("md5");
		hasher.update("hello");
		hasher.update(" ");
		hasher.update("world");

		const resultHex = hasher.digest("hex");
		if (resultHex !== correctHex) {
			throw new Error("Hex encoding mismatch: " + resultHex);
		}`)

		assert.NoError(t, err)
	})
}

func TestOutputEncoding(t *testing.T) {
	if testing.Short() {
		return
	}

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	root, _ := lib.NewGroup("", nil)
	state := &lib.State{Group: root}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("crypto", common.Bind(rt, New(), &ctx))

	t.Run("Valid", func(t *testing.T) {
		_, err := common.RunString(rt, `
		const correctHex = "5eb63bbbe01eeed093cb22bb8f5acdc3";
		const correctBase64 = "XrY7u+Ae7tCTyyK7j1rNww==";
		const correctBase64URL = "XrY7u-Ae7tCTyyK7j1rNww=="
		const correctBase64RawURL = "XrY7u-Ae7tCTyyK7j1rNww";

		let hasher = crypto.createHash("md5");
		hasher.update("hello world");

		const resultHex = hasher.digest("hex");
		if (resultHex !== correctHex) {
			throw new Error("Hex encoding mismatch: " + resultHex);
		}

		const resultBase64 = hasher.digest("base64");
		if (resultBase64 !== correctBase64) {
			throw new Error("Base64 encoding mismatch: " + resultBase64);
		}

		const resultBase64URL = hasher.digest("base64url");
		if (resultBase64URL !== correctBase64URL) {
			throw new Error("Base64 URL encoding mismatch: " + resultBase64URL);
		}

		const resultBase64RawURL = hasher.digest("base64rawurl");
		if (resultBase64RawURL !== correctBase64RawURL) {
			throw new Error("Base64 raw URL encoding mismatch: " + resultBase64RawURL);
		}
		`)

		assert.NoError(t, err)
	})

	t.Run("Invalid", func(t *testing.T) {
		_, err := common.RunString(rt, `
		let hasher = crypto.createHash("md5");
		hasher.update("hello world");
		hasher.digest("someInvalidEncoding");
		`)
		assert.EqualError(t, err, "GoError: Invalid output encoding: someInvalidEncoding")
	})
}

func TestHMac(t *testing.T) {
	if testing.Short() {
		return
	}

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	root, _ := lib.NewGroup("", nil)
	state := &lib.State{Group: root}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("crypto", common.Bind(rt, New(), &ctx))

	testData := map[string]string{
		"md4":        "92d8f5c302cf04cca0144d7a9feb1596",
		"md5":        "e04f2ec05c8b12e19e46936b171c9d03",
		"sha1":       "c113b62711ff5d8e8100bbb17b998591af81dc24",
		"sha256":     "7fd04df92f636fd450bc841c9418e5825c17f33ad9c87c518115a45971f7f77e",
		"sha384":     "d331e169e2dcfc742e80a3bf4dcc76d0e6425ab3777a3ac217ac6b2552aad5529ed4d40135b06e53a495ac7425d1e462",
		"sha512_224": "bac4e6256bdbf81d029aec48af4fdd4b14001db6721f07c429a80817",
		"sha512_256": "e3d0763ba92a4f40676c3d5b234d9842b71951e6e0767082cfb3f5e14c124b22",
		"sha512":     "cd3146f96a3005024108ff56b025517552435589a4c218411f165da0a368b6f47228b20a1a4bf081e4aae6f07e2790f27194fc77f0addc890e98ce1951cacc9f",
		"ripemd160":  "00bb4ce0d6afd4c7424c9d01b8a6caa3e749b08b",
	}
	for algorithm, value := range testData {
		rt.Set("correctHex", rt.ToValue(value))
		rt.Set("algorithm", rt.ToValue(algorithm))
		t.Run(algorithm+" hasher: valid", func(t *testing.T) {
			_, err := common.RunString(rt, `
			let hasher = crypto.createHMAC(algorithm, "a secret");
			hasher.update("some data to hash");

			const resultHex = hasher.digest("hex");
			if (resultHex !== correctHex) {
				throw new Error("Hex encoding mismatch: " + resultHex);
			}`)

			assert.NoError(t, err)
		})

		t.Run(algorithm+" wrapper: valid", func(t *testing.T) {
			_, err := common.RunString(rt, `
			let resultHex = crypto.hmac(algorithm, "a secret", "some data to hash", "hex");
			if (resultHex !== correctHex) {
				throw new Error("Hex encoding mismatch: " + resultHex);
			}`)

			assert.NoError(t, err)
		})
	}

	// Algorithms not supported or typing error
	invalidData := map[string]string{
		"md6":    "e04f2ec05c8b12e19e46936b171c9d03",
		"sha526": "7fd04df92f636fd450bc841c9418e5825c17f33ad9c87c518115a45971f7f77e",
		"sha348": "d331e169e2dcfc742e80a3bf4dcc76d0e6425ab3777a3ac217ac6b2552aad5529ed4d40135b06e53a495ac7425d1e462",
	}
	for algorithm, value := range invalidData {
		rt.Set("correctHex", rt.ToValue(value))
		rt.Set("algorithm", rt.ToValue(algorithm))
		t.Run(algorithm+" hasher: invalid", func(t *testing.T) {
			_, err := common.RunString(rt, `
			let hasher = crypto.createHMAC(algorithm, "a secret");
			hasher.update("some data to hash");

			const resultHex = hasher.digest("hex");
			if (resultHex !== correctHex) {
				throw new Error("Hex encoding mismatch: " + resultHex);
			}`)

			assert.EqualError(t, err, "GoError: Invalid algorithm: "+algorithm)
		})

		t.Run(algorithm+" wrapper: invalid", func(t *testing.T) {
			_, err := common.RunString(rt, `
			let resultHex = crypto.hmac(algorithm, "a secret", "some data to hash", "hex");
			if (resultHex !== correctHex) {
				throw new Error("Hex encoding mismatch: " + resultHex);
			}`)

			assert.EqualError(t, err, "GoError: Invalid algorithm: "+algorithm)
		})
	}
}
