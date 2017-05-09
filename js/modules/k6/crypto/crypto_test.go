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
	"testing"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/stretchr/testify/assert"
)

func TestCryptoAlgorithms(t *testing.T) {
	if testing.Short() {
		return
	}

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	ctx := context.Background()
	ctx = common.WithRuntime(ctx, rt)
	rt.Set("crypto", common.Bind(rt, &Crypto{}, &ctx))

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
	state := &common.State{Group: root}

	ctx := context.Background()
	ctx = common.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("crypto", common.Bind(rt, &Crypto{}, &ctx))

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
	state := &common.State{Group: root}

	ctx := context.Background()
	ctx = common.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("crypto", common.Bind(rt, &Crypto{}, &ctx))

	t.Run("Valid", func(t *testing.T) {
		_, err := common.RunString(rt, `
		const correctHex = "5eb63bbbe01eeed093cb22bb8f5acdc3";
		const correctBase64 = "XrY7u+Ae7tCTyyK7j1rNww==";

		let hasher = crypto.createHash("md5");
		hasher.update("hello world");

		const resultHex = hasher.digest("hex");
		if (resultHex !== correctHex) {
			throw new Error("Hex encoding mismatch: " + resultHex);
		}

		const resultBase64 = hasher.digest("base64");
		if (resultBase64 !== correctBase64) {
			throw new Error("Base64 encoding mismatch: " + resultBase64);
		}`)

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
