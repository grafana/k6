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

package encoding

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/js/common"
)

func TestEncodingAlgorithms(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	ctx := context.Background()
	ctx = common.WithRuntime(ctx, rt)
	rt.Set("encoding", common.Bind(rt, New(), &ctx))

	t.Run("Base64", func(t *testing.T) {
		t.Run("DefaultEnc", func(t *testing.T) {
			_, err := rt.RunString(`
			var correct = "aGVsbG8gd29ybGQ=";
			var encoded = encoding.b64encode("hello world");
			if (encoded !== correct) {
				throw new Error("Encoding mismatch: " + encoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("DefaultDec", func(t *testing.T) {
			_, err := rt.RunString(`
			var correct = "hello world";
			var decBin = encoding.b64decode("aGVsbG8gd29ybGQ=");

			var decText = String.fromCharCode.apply(null, new Uint8Array(decBin));
			decText = decodeURIComponent(escape(decText));
			if (decText !== correct) {
				throw new Error("Decoding mismatch: " + decText);
			}`)
			assert.NoError(t, err)
		})
		t.Run("DefaultArrayBufferEnc", func(t *testing.T) {
			_, err := rt.RunString(`
			var exp = "aGVsbG8=";
			var input = new Uint8Array([104, 101, 108, 108, 111]); // "hello"
			var encoded = encoding.b64encode(input.buffer);
			if (encoded !== exp) {
				throw new Error("Encoding mismatch: " + encoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("DefaultArrayBufferDec", func(t *testing.T) { //nolint: paralleltest // weird that it triggers here, and these tests can't be parallel
			_, err := rt.RunString(`
			var exp = "hello";
			var decBin = encoding.b64decode("aGVsbG8=");
			var decText = String.fromCharCode.apply(null, new Uint8Array(decBin));
			decText = decodeURIComponent(escape(decText));
			if (decText !== exp) {
				throw new Error("Decoding mismatch: " + decText);
			}`)
			assert.NoError(t, err)
		})
		t.Run("DefaultUnicodeEnc", func(t *testing.T) {
			_, err := rt.RunString(`
			var correct = "44GT44KT44Gr44Gh44Gv5LiW55WM";
			var encoded = encoding.b64encode("こんにちは世界", "std");
			if (encoded !== correct) {
				throw new Error("Encoding mismatch: " + encoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("DefaultUnicodeDec", func(t *testing.T) {
			_, err := rt.RunString(`
			var correct = "こんにちは世界";
			var decBin = encoding.b64decode("44GT44KT44Gr44Gh44Gv5LiW55WM");
			var decText = String.fromCharCode.apply(null, new Uint8Array(decBin));
			decText = decodeURIComponent(escape(decText));
			if (decText !== correct) {
				throw new Error("Decoding mismatch: " + decText);
			}`)
			assert.NoError(t, err)
		})
		t.Run("StdEnc", func(t *testing.T) {
			_, err := rt.RunString(`
			var correct = "aGVsbG8gd29ybGQ=";
			var encoded = encoding.b64encode("hello world", "std");
			if (encoded !== correct) {
				throw new Error("Encoding mismatch: " + encoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("StdDec", func(t *testing.T) {
			_, err := rt.RunString(`
			var correct = "hello world";
			var decoded = encoding.b64decode("aGVsbG8gd29ybGQ=", "std", "s");
			if (decoded !== correct) {
				throw new Error("Decoding mismatch: " + decoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("RawStdEnc", func(t *testing.T) {
			_, err := rt.RunString(`
			var correct = "aGVsbG8gd29ybGQ";
			var encoded = encoding.b64encode("hello world", "rawstd");
			if (encoded !== correct) {
				throw new Error("Encoding mismatch: " + encoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("RawStdDec", func(t *testing.T) {
			_, err := rt.RunString(`
			var correct = "hello world";
			var decoded = encoding.b64decode("aGVsbG8gd29ybGQ", "rawstd", "s");
			if (decoded !== correct) {
				throw new Error("Decoding mismatch: " + decoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("URLEnc", func(t *testing.T) {
			_, err := rt.RunString(`
			var correct = "5bCP6aO85by-Li4=";
			var encoded = encoding.b64encode("小飼弾..", "url");
			if (encoded !== correct) {
				throw new Error("Encoding mismatch: " + encoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("URLDec", func(t *testing.T) {
			_, err := rt.RunString(`
			var correct = "小飼弾..";
			var decoded = encoding.b64decode("5bCP6aO85by-Li4=", "url", "s");
			if (decoded !== correct) {
				throw new Error("Decoding mismatch: " + decoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("RawURLEnc", func(t *testing.T) {
			_, err := rt.RunString(`
			var correct = "5bCP6aO85by-Li4";
			var encoded = encoding.b64encode("小飼弾..", "rawurl");
			if (encoded !== correct) {
				throw new Error("Encoding mismatch: " + encoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("RawURLDec", func(t *testing.T) {
			_, err := rt.RunString(`
			var correct = "小飼弾..";
			var decoded = encoding.b64decode("5bCP6aO85by-Li4", "rawurl", "s");
			if (decoded !== correct) {
				throw new Error("Decoding mismatch: " + decoded);
			}`)
			assert.NoError(t, err)
		})
	})
}
