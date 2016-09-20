package js

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNew(t *testing.T) {
	r, err := New()
	assert.NoError(t, err)

	t.Run("Polyfill: Symbol", func(t *testing.T) {
		v, err := r.VM.Get("Symbol")
		assert.NoError(t, err)
		assert.False(t, v.IsUndefined())
	})
}

func TestLoad(t *testing.T) {
	r, err := New()
	assert.NoError(t, err)
	assert.NoError(t, r.VM.Set("require", r.require))

	t.Run("Importing Libraries", func(t *testing.T) {
		_, err := r.load("test.js", []byte(`
			import "speedboat";
		`))
		assert.NoError(t, err)
		assert.Contains(t, r.Lib, "speedboat.js")
	})
}
