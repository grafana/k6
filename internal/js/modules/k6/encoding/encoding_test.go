package encoding

import (
	"context"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modulestest"
)

func makeRuntime(t *testing.T) *sobek.Runtime {
	rt := sobek.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	m, ok := New().NewModuleInstance(
		&modulestest.VU{
			CtxField:     context.Background(),
			RuntimeField: rt,
			InitEnvField: &common.InitEnvironment{},
			StateField:   nil,
		},
	).(*Encoding)
	require.True(t, ok)
	require.NoError(t, rt.Set("encoding", m.Exports().Named))

	return rt
}

func TestEncodingAlgorithms(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		return
	}

	t.Run("Base64", func(t *testing.T) {
		t.Parallel()

		t.Run("DefaultEnc", func(t *testing.T) {
			t.Parallel()

			rt := makeRuntime(t)
			_, err := rt.RunString(`
			var correct = "aGVsbG8gd29ybGQ=";
			var encoded = encoding.b64encode("hello world");
			if (encoded !== correct) {
				throw new Error("Encoding mismatch: " + encoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("DefaultDec", func(t *testing.T) {
			t.Parallel()

			rt := makeRuntime(t)
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
			t.Parallel()

			rt := makeRuntime(t)
			_, err := rt.RunString(`
			var exp = "aGVsbG8=";
			var input = new Uint8Array([104, 101, 108, 108, 111]); // "hello"
			var encoded = encoding.b64encode(input.buffer);
			if (encoded !== exp) {
				throw new Error("Encoding mismatch: " + encoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("DefaultArrayBufferDec", func(t *testing.T) {
			t.Parallel()

			rt := makeRuntime(t)
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
			t.Parallel()

			rt := makeRuntime(t)
			_, err := rt.RunString(`
			var correct = "44GT44KT44Gr44Gh44Gv5LiW55WM";
			var encoded = encoding.b64encode("こんにちは世界", "std");
			if (encoded !== correct) {
				throw new Error("Encoding mismatch: " + encoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("DefaultUnicodeDec", func(t *testing.T) {
			t.Parallel()

			rt := makeRuntime(t)
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
			t.Parallel()

			rt := makeRuntime(t)
			_, err := rt.RunString(`
			var correct = "aGVsbG8gd29ybGQ=";
			var encoded = encoding.b64encode("hello world", "std");
			if (encoded !== correct) {
				throw new Error("Encoding mismatch: " + encoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("StdDec", func(t *testing.T) {
			t.Parallel()

			rt := makeRuntime(t)
			_, err := rt.RunString(`
			var correct = "hello world";
			var decoded = encoding.b64decode("aGVsbG8gd29ybGQ=", "std", "s");
			if (decoded !== correct) {
				throw new Error("Decoding mismatch: " + decoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("RawStdEnc", func(t *testing.T) {
			t.Parallel()

			rt := makeRuntime(t)
			_, err := rt.RunString(`
			var correct = "aGVsbG8gd29ybGQ";
			var encoded = encoding.b64encode("hello world", "rawstd");
			if (encoded !== correct) {
				throw new Error("Encoding mismatch: " + encoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("RawStdDec", func(t *testing.T) {
			t.Parallel()

			rt := makeRuntime(t)
			_, err := rt.RunString(`
			var correct = "hello world";
			var decoded = encoding.b64decode("aGVsbG8gd29ybGQ", "rawstd", "s");
			if (decoded !== correct) {
				throw new Error("Decoding mismatch: " + decoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("URLEnc", func(t *testing.T) {
			t.Parallel()

			rt := makeRuntime(t)
			_, err := rt.RunString(`
			var correct = "5bCP6aO85by-Li4=";
			var encoded = encoding.b64encode("小飼弾..", "url");
			if (encoded !== correct) {
				throw new Error("Encoding mismatch: " + encoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("URLDec", func(t *testing.T) {
			t.Parallel()

			rt := makeRuntime(t)
			_, err := rt.RunString(`
			var correct = "小飼弾..";
			var decoded = encoding.b64decode("5bCP6aO85by-Li4=", "url", "s");
			if (decoded !== correct) {
				throw new Error("Decoding mismatch: " + decoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("RawURLEnc", func(t *testing.T) {
			t.Parallel()

			rt := makeRuntime(t)
			_, err := rt.RunString(`
			var correct = "5bCP6aO85by-Li4";
			var encoded = encoding.b64encode("小飼弾..", "rawurl");
			if (encoded !== correct) {
				throw new Error("Encoding mismatch: " + encoded);
			}`)
			assert.NoError(t, err)
		})
		t.Run("RawURLDec", func(t *testing.T) {
			t.Parallel()

			rt := makeRuntime(t)
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
