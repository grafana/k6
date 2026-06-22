package errext_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/v2/errext"
)

func TestWithFields(t *testing.T) {
	t.Parallel()

	t.Run("NilError", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, errext.WithFields(nil, map[string]any{"k": "v"}))
	})

	t.Run("EmptyFields", func(t *testing.T) {
		t.Parallel()
		orig := errors.New("plain")
		assert.Equal(t, orig, errext.WithFields(orig, nil))
		assert.Equal(t, orig, errext.WithFields(orig, map[string]any{}))
	})

	t.Run("AttachesFields", func(t *testing.T) {
		t.Parallel()
		wrapped := errext.WithFields(errors.New("boom"), map[string]any{"module": "browser"})
		fields := errext.FieldsFromErr(wrapped)
		assert.Equal(t, map[string]any{"module": "browser"}, fields)
	})

	t.Run("PreservesChain", func(t *testing.T) {
		t.Parallel()
		orig := errors.New("root")
		wrapped := errext.WithFields(orig, map[string]any{"k": "v"})
		assert.True(t, errors.Is(wrapped, orig))
		assert.Equal(t, "root", wrapped.Error())
	})

	t.Run("MergesFields", func(t *testing.T) {
		t.Parallel()
		inner := errext.WithFields(errors.New("err"), map[string]any{"a": "1"})
		outer := errext.WithFields(inner, map[string]any{"b": "2"})
		fields := errext.FieldsFromErr(outer)
		assert.Equal(t, map[string]any{"a": "1", "b": "2"}, fields)
	})
}
