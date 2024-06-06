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
