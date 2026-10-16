package webcrypto

import (
	"errors"
	"strings"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTraverseObject(t *testing.T) {
	t.Parallel()

	t.Run("empty object and empty fields", func(t *testing.T) {
		t.Parallel()

		rt := sobek.New()
		obj := rt.NewObject()

		gotVal, gotErr := traverseObject(rt, obj)

		require.NoError(t, gotErr)
		assert.Equal(t, obj, gotVal)
	})

	t.Run("empty object and non-empty fields", func(t *testing.T) {
		t.Parallel()

		rt := sobek.New()
		obj := rt.NewObject()

		_, gotErr := traverseObject(rt, obj, "foo")
		var gotWebCryptoError *Error
		errors.As(gotErr, &gotWebCryptoError)

		assert.Error(t, gotErr)
		assert.True(t, strings.Contains(gotWebCryptoError.Message, "foo"))
	})

	t.Run("non-empty object and empty fields", func(t *testing.T) {
		t.Parallel()

		rt := sobek.New()
		obj := rt.NewObject()
		childObj := rt.NewObject()
		err := obj.Set("foo", childObj)
		require.NoError(t, err)

		_, gotErr := traverseObject(rt, obj)

		assert.NoError(t, gotErr)
	})

	t.Run("non-empty object and non-empty fields", func(t *testing.T) {
		t.Parallel()

		rt := sobek.New()
		obj := rt.NewObject()
		childValue := rt.NewObject()
		err := obj.Set("foo", childValue)
		require.NoError(t, err)

		gotVal, gotErr := traverseObject(rt, obj, "foo")

		require.NoError(t, gotErr)
		assert.Equal(t, childValue, gotVal)
	})

	t.Run("non-empty object and non-empty fields with non-object leaf", func(t *testing.T) {
		t.Parallel()

		rt := sobek.New()
		obj := rt.NewObject()
		childValue := rt.ToValue("bar")
		err := obj.Set("foo", childValue)
		require.NoError(t, err)

		gotValue, gotErr := traverseObject(rt, obj, "foo")

		assert.NoError(t, gotErr)
		assert.Equal(t, childValue, gotValue)
	})

	t.Run("non-empty object and non-empty fields with non-existent leaf", func(t *testing.T) {
		t.Parallel()

		rt := sobek.New()
		obj := rt.NewObject()
		childValue := rt.ToValue("bar")
		err := obj.Set("foo", childValue)
		require.NoError(t, err)

		_, gotErr := traverseObject(rt, obj, "foo", "babar")
		var gotWebCryptoError *Error
		errors.As(gotErr, &gotWebCryptoError)

		assert.Error(t, gotErr)
		assert.True(t, strings.Contains(gotWebCryptoError.Message, "foo.babar"))
	})

	t.Run("non-empty object and non-empty fields with non-object intermediate", func(t *testing.T) {
		t.Parallel()

		rt := sobek.New()
		obj := rt.NewObject()
		childValue := rt.ToValue("bar")
		err := obj.Set("foo", childValue)
		require.NoError(t, err)

		_, gotErr := traverseObject(rt, obj, "foo", "bar", "bonjour")
		var gotWebCryptoError *Error
		errors.As(gotErr, &gotWebCryptoError)

		assert.Error(t, gotErr)
		assert.True(t, strings.Contains(gotWebCryptoError.Message, "foo.bar"))
	})

	t.Run("nil object", func(t *testing.T) {
		t.Parallel()

		rt := sobek.New()

		_, gotErr := traverseObject(rt, nil)

		assert.Error(t, gotErr)
	})
}

func TestExportArrayBufferDetachedView(t *testing.T) {
	t.Parallel()

	// Detaching a buffer is only reachable from an xk6/Go extension that shares
	// the runtime and calls the exported sobek.ArrayBuffer.Detach(); stock
	// JavaScript has no detach entry point. A non-zero offset view keeps its
	// captured byteOffset/byteLength after detach while the backing data becomes
	// nil. WebIDL copies detached BufferSources as empty bytes; exporting a view
	// must not panic the Go runtime.
	cases := []struct {
		name   string
		script string
		shadow string
	}{
		{
			name:   "non-zero offset Uint8Array",
			script: `new Uint8Array(new ArrayBuffer(8), 2)`,
		},
		{
			name:   "non-zero offset multibyte Uint16Array",
			script: `new Uint16Array(new ArrayBuffer(8), 2, 2)`,
		},
		{
			name:   "non-zero offset DataView",
			script: `new DataView(new ArrayBuffer(8), 2, 4)`,
		},
		{
			name:   "shadowed Uint8Array buffer",
			script: `new Uint8Array(new ArrayBuffer(8), 2)`,
			shadow: `Object.defineProperty(view, "buffer", {value: new ArrayBuffer(8)})`,
		},
		{
			name:   "shadowed empty Uint8Array buffer",
			script: `new Uint8Array(new ArrayBuffer(8), 0, 0)`,
			shadow: `Object.defineProperty(view, "buffer", {value: new ArrayBuffer(8)})`,
		},
		{
			name:   "throwing DataView buffer getter",
			script: `new DataView(new ArrayBuffer(8), 2, 4)`,
			shadow: `Object.defineProperty(view, "buffer", {get() { throw new Error("shadow getter must not be used") }})`,
		},
		{
			name:   "overridden TypedArray prototype buffer getter",
			script: `new Uint8Array(new ArrayBuffer(8), 2)`,
			shadow: `Object.defineProperty(Object.getPrototypeOf(Uint8Array.prototype), "buffer", {get() { throw new Error("prototype getter must not be used") }})`,
		},
		{
			name:   "overridden DataView prototype buffer getter",
			script: `new DataView(new ArrayBuffer(8), 2, 4)`,
			shadow: `Object.defineProperty(DataView.prototype, "buffer", {get() { throw new Error("prototype getter must not be used") }})`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rt := sobek.New()

			accessors, err := getBufferSourceAccessors(rt)
			require.NoError(t, err)

			view, err := rt.RunString(tc.script)
			require.NoError(t, err)

			buffer, ok := view.ToObject(rt).Get("buffer").Export().(sobek.ArrayBuffer)
			require.True(t, ok)
			require.True(t, buffer.Detach())
			if tc.shadow != "" {
				require.NoError(t, rt.Set("view", view))
				_, err = rt.RunString(tc.shadow)
				require.NoError(t, err)
			}

			var (
				gotBytes []byte
				gotErr   error
				panicked any
			)
			func() {
				defer func() { panicked = recover() }()
				gotBytes, gotErr = exportArrayBuffer(rt, view, accessors)
			}()

			require.Nil(t, panicked, "exporting a detached view must not panic the Go runtime")
			require.NoError(t, gotErr)
			assert.Equal(t, []byte{}, gotBytes)
		})
	}
}

func TestExportArrayBufferAttachedViewWithOverriddenGetter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		script string
		shadow string
	}{
		{
			name:   "TypedArray",
			script: `new Uint8Array([9, 1, 2, 3, 8]).subarray(1, 4)`,
			shadow: `Object.defineProperty(Object.getPrototypeOf(Uint8Array.prototype), "buffer", {get() { throw new Error("prototype getter must not be used") }})`,
		},
		{
			name:   "DataView",
			script: `new DataView(new Uint8Array([9, 1, 2, 3, 8]).buffer, 1, 3)`,
			shadow: `Object.defineProperty(DataView.prototype, "buffer", {get() { throw new Error("prototype getter must not be used") }})`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rt := sobek.New()
			accessors, err := getBufferSourceAccessors(rt)
			require.NoError(t, err)
			view, err := rt.RunString(tc.script)
			require.NoError(t, err)
			_, err = rt.RunString(tc.shadow)
			require.NoError(t, err)

			data, err := exportArrayBuffer(rt, view, accessors)
			require.NoError(t, err)
			assert.Equal(t, []byte{1, 2, 3}, data)
		})
	}
}

func TestExportArrayBufferDetachedBuffer(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	accessors, err := getBufferSourceAccessors(rt)
	require.NoError(t, err)
	value, err := rt.RunString(`new ArrayBuffer(8)`)
	require.NoError(t, err)
	buffer, ok := value.Export().(sobek.ArrayBuffer)
	require.True(t, ok)
	require.True(t, buffer.Detach())

	data, err := exportArrayBuffer(rt, value, accessors)
	require.NoError(t, err)
	assert.Equal(t, []byte{}, data)
}
