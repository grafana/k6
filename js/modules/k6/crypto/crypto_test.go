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
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
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
		_, err := rt.RunString(`
		var buf = crypto.randomBytes(5);
		if (buf.byteLength !== 5) {
			throw new Error("Incorrect size: " + buf.byteLength);
		}`)

		assert.NoError(t, err)
	})

	t.Run("RandomBytesInvalidSize", func(t *testing.T) {
		_, err := rt.RunString(`
		crypto.randomBytes(-1);`)

		assert.Error(t, err)
	})

	t.Run("RandomBytesFailure", func(t *testing.T) {
		SavedReader := rand.Reader
		rand.Reader = MockReader{}
		_, err := rt.RunString(`
		crypto.randomBytes(5);`)
		rand.Reader = SavedReader

		assert.Error(t, err)
	})

	t.Run("MD4", func(t *testing.T) {
		_, err := rt.RunString(`
		var correct = "aa010fbc1d14c795d86ef98c95479d17";
		var hash = crypto.md4("hello world", "hex");
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)
		assert.NoError(t, err)
	})

	t.Run("MD5", func(t *testing.T) {
		_, err := rt.RunString(`
		var correct = "5eb63bbbe01eeed093cb22bb8f5acdc3";
		var hash = crypto.md5("hello world", "hex");
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)

		assert.NoError(t, err)
	})

	t.Run("SHA1", func(t *testing.T) {
		_, err := rt.RunString(`
		var correct = "2aae6c35c94fcfb415dbe95f408b9ce91ee846ed";
		var hash = crypto.sha1("hello world", "hex");
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)

		assert.NoError(t, err)
	})

	t.Run("SHA256", func(t *testing.T) {
		_, err := rt.RunString(`
		var correct = "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9";
		var hash = crypto.sha256("hello world", "hex");
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)

		assert.NoError(t, err)
	})

	t.Run("SHA384", func(t *testing.T) {
		_, err := rt.RunString(`
		var correct = "fdbd8e75a67f29f701a4e040385e2e23986303ea10239211af907fcbb83578b3e417cb71ce646efd0819dd8c088de1bd";
		var hash = crypto.sha384("hello world", "hex");
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)

		assert.NoError(t, err)
	})

	t.Run("SHA512", func(t *testing.T) {
		_, err := rt.RunString(`
		var correct = "309ecc489c12d6eb4cc40f50c902f2b4d0ed77ee511a7c7a9bcd3ca86d4cd86f989dd35bc5ff499670da34255b45b0cfd830e81f605dcf7dc5542e93ae9cd76f";
		var hash = crypto.sha512("hello world", "hex");
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)

		assert.NoError(t, err)
	})

	t.Run("SHA512_224", func(t *testing.T) {
		_, err := rt.RunString(`
		var hash = crypto.sha512_224("hello world", "hex");
		var correct = "22e0d52336f64a998085078b05a6e37b26f8120f43bf4db4c43a64ee";
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)

		assert.NoError(t, err)
	})

	t.Run("SHA512_256", func(t *testing.T) {
		_, err := rt.RunString(`
		var hash = crypto.sha512_256("hello world", "hex");
		var correct = "0ac561fac838104e3f2e4ad107b4bee3e938bf15f2b15f009ccccd61a913f017";
		if (hash !== correct) {
			throw new Error("Hash mismatch: " + hash);
		}`)

		assert.NoError(t, err)
	})

	t.Run("RIPEMD160", func(t *testing.T) {
		_, err := rt.RunString(`
		var hash = crypto.ripemd160("hello world", "hex");
		var correct = "98c615784ccb5fe5936fbc0cbe9dfdb408d92f0f";
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
		_, err := rt.RunString(`
		var correctHex = "d41d8cd98f00b204e9800998ecf8427e";

		var hasher = crypto.createHash("md5");

		var resultHex = hasher.digest("hex");
		if (resultHex !== correctHex) {
			throw new Error("Hex encoding mismatch: " + resultHex);
		}`)

		assert.NoError(t, err)
	})

	t.Run("UpdateOnce", func(t *testing.T) {
		_, err := rt.RunString(`
		var correctHex = "5eb63bbbe01eeed093cb22bb8f5acdc3";

		var hasher = crypto.createHash("md5");
		hasher.update("hello world");

		var resultHex = hasher.digest("hex");
		if (resultHex !== correctHex) {
			throw new Error("Hex encoding mismatch: " + resultHex);
		}`)

		assert.NoError(t, err)
	})

	t.Run("UpdateMultiple", func(t *testing.T) {
		_, err := rt.RunString(`
		var correctHex = "5eb63bbbe01eeed093cb22bb8f5acdc3";

		var hasher = crypto.createHash("md5");
		hasher.update("hello");
		hasher.update(" ");
		hasher.update("world");

		var resultHex = hasher.digest("hex");
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
		_, err := rt.RunString(`
		var correctHex = "5eb63bbbe01eeed093cb22bb8f5acdc3";
		var correctBase64 = "XrY7u+Ae7tCTyyK7j1rNww==";
		var correctBase64URL = "XrY7u-Ae7tCTyyK7j1rNww=="
		var correctBase64RawURL = "XrY7u-Ae7tCTyyK7j1rNww";
		var correctBinary = [94,182,59,187,224,30,238,208,147,203,34,187,143,90,205,195];

		var hasher = crypto.createHash("md5");
		hasher.update("hello world");

		var resultHex = hasher.digest("hex");
		if (resultHex !== correctHex) {
			throw new Error("Hex encoding mismatch: " + resultHex);
		}

		var resultBase64 = hasher.digest("base64");
		if (resultBase64 !== correctBase64) {
			throw new Error("Base64 encoding mismatch: " + resultBase64);
		}

		var resultBase64URL = hasher.digest("base64url");
		if (resultBase64URL !== correctBase64URL) {
			throw new Error("Base64 URL encoding mismatch: " + resultBase64URL);
		}

		var resultBase64RawURL = hasher.digest("base64rawurl");
		if (resultBase64RawURL !== correctBase64RawURL) {
			throw new Error("Base64 raw URL encoding mismatch: " + resultBase64RawURL);
		}

		// https://stackoverflow.com/a/16436975/5427244
		function arraysEqual(a, b) {
		  if (a === b) return true;
		  if (a == null || b == null) return false;
		  if (a.length != b.length) return false;

		  for (var i = 0; i < a.length; ++i) {
			if (a[i] !== b[i]) return false;
		  }
		  return true;
		}

		var resultBinary = new Uint8Array(hasher.digest("binary"));
		if (!arraysEqual(resultBinary,  correctBinary)) {
			throw new Error("Binary encoding mismatch: " + JSON.stringify(resultBinary));
		}
		`)

		assert.NoError(t, err)
	})

	t.Run("Invalid", func(t *testing.T) {
		_, err := rt.RunString(`
		var hasher = crypto.createHash("md5");
		hasher.update("hello world");
		hasher.digest("someInvalidEncoding");
		`)
		assert.Contains(t, err.Error(), "GoError: Invalid output encoding: someInvalidEncoding")
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
			_, err := rt.RunString(`
			var hasher = crypto.createHMAC(algorithm, "a secret");
			hasher.update("some data to hash");

			var resultHex = hasher.digest("hex");
			if (resultHex !== correctHex) {
				throw new Error("Hex encoding mismatch: " + resultHex);
			}`)

			assert.NoError(t, err)
		})

		t.Run(algorithm+" wrapper: valid", func(t *testing.T) {
			_, err := rt.RunString(`
			var resultHex = crypto.hmac(algorithm, "a secret", "some data to hash", "hex");
			if (resultHex !== correctHex) {
				throw new Error("Hex encoding mismatch: " + resultHex);
			}`)

			assert.NoError(t, err)
		})

		t.Run(algorithm+" ArrayBuffer: valid", func(t *testing.T) {
			_, err := rt.RunString(`
			var data = new Uint8Array([115,111,109,101,32,100,97,116,97,32,116,
										111,32,104,97,115,104]).buffer;
			var resultHex = crypto.hmac(algorithm, "a secret", data, "hex");
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
		algorithm := algorithm
		rt.Set("correctHex", rt.ToValue(value))
		rt.Set("algorithm", rt.ToValue(algorithm))
		t.Run(algorithm+" hasher: invalid", func(t *testing.T) {
			_, err := rt.RunString(`
			var hasher = crypto.createHMAC(algorithm, "a secret");
			hasher.update("some data to hash");

			var resultHex = hasher.digest("hex");
			if (resultHex !== correctHex) {
				throw new Error("Hex encoding mismatch: " + resultHex);
			}`)

			assert.Contains(t, err.Error(), "GoError: Invalid algorithm: "+algorithm)
		})

		t.Run(algorithm+" wrapper: invalid", func(t *testing.T) {
			_, err := rt.RunString(`
			var resultHex = crypto.hmac(algorithm, "a secret", "some data to hash", "hex");
			if (resultHex !== correctHex) {
				throw new Error("Hex encoding mismatch: " + resultHex);
			}`)

			assert.Contains(t, err.Error(), "GoError: Invalid algorithm: "+algorithm)
		})
	}
}

func TestHexEncodeOK(t *testing.T) {
	rt := goja.New()
	input := []byte{104, 101, 108, 108, 111}
	testCases := []interface{}{
		input, string(input), rt.NewArrayBuffer(input),
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%T", tc), func(t *testing.T) {
			c := New()
			ctx := common.WithRuntime(context.Background(), rt)
			out := c.HexEncode(ctx, tc)
			assert.Equal(t, "68656c6c6f", out)
		})
	}
}

func TestHexEncodeError(t *testing.T) {
	rt := goja.New()

	expErr := "invalid type struct {}, expected string, []byte or ArrayBuffer"
	defer func() {
		err := recover()
		require.NotNil(t, err)
		require.IsType(t, &goja.Object{}, err)
		require.IsType(t, map[string]interface{}{}, err.(*goja.Object).Export())
		val := err.(*goja.Object).Export().(map[string]interface{})
		assert.Equal(t, expErr, fmt.Sprintf("%s", val["value"]))
	}()

	c := New()
	ctx := common.WithRuntime(context.Background(), rt)
	c.HexEncode(ctx, struct{}{})
}

func TestAWSv4(t *testing.T) {
	// example values from https://docs.aws.amazon.com/general/latest/gr/signature-v4-examples.html
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	ctx := context.Background()
	ctx = common.WithRuntime(ctx, rt)
	rt.Set("crypto", common.Bind(rt, New(), &ctx))

	_, err := rt.RunString(`
		var HexEncode = crypto.hexEncode;
		var HmacSHA256 = function(data, key) {
			return crypto.hmac("sha256", key, data, "binary");
		};

		var expectedKDate    = '969fbb94feb542b71ede6f87fe4d5fa29c789342b0f407474670f0c2489e0a0d'
		var expectedKRegion  = '69daa0209cd9c5ff5c8ced464a696fd4252e981430b10e3d3fd8e2f197d7a70c'
		var expectedKService = 'f72cfd46f26bc4643f06a11eabb6c0ba18780c19a8da0c31ace671265e3c87fa'
		var expectedKSigning = 'f4780e2d9f65fa895f9c67b32ce1baf0b0d8a43505a000a1a9e090d414db404d'

		var key = 'wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY';
		var dateStamp = '20120215';
		var regionName = 'us-east-1';
		var serviceName = 'iam';

		var kDate = HmacSHA256(dateStamp, "AWS4" + key);
		var kRegion = HmacSHA256(regionName, kDate);
		var kService = HmacSHA256(serviceName, kRegion);
		var kSigning = HmacSHA256("aws4_request", kService);


		var hexKDate = HexEncode(kDate);
		if (expectedKDate != hexKDate) {
			throw new Error("Wrong kDate: expected '" + expectedKDate + "' got '" + hexKDate + "'");
		}
		var hexKRegion = HexEncode(kRegion);
		if (expectedKRegion != hexKRegion) {
			throw new Error("Wrong kRegion: expected '" + expectedKRegion + "' got '" + hexKRegion + "'");
		}
		var hexKService = HexEncode(kService);
		if (expectedKService != hexKService) {
			throw new Error("Wrong kService: expected '" + expectedKService + "' got '" + hexKService + "'");
		}
		var hexKSigning = HexEncode(kSigning);
		if (expectedKSigning != hexKSigning) {
			throw new Error("Wrong kSigning: expected '" + expectedKSigning + "' got '" + hexKSigning + "'");
		}
		`)
	assert.NoError(t, err)
}
