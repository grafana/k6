package js

import (
	"github.com/loadimpact/speedboat/lib"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
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

func TestExtractOptions(t *testing.T) {
	r, err := New()
	assert.NoError(t, err)

	t.Run("nothing", func(t *testing.T) {
		exp, err := r.load("test.js", []byte(``))
		assert.NoError(t, err)

		var opts lib.Options
		assert.NoError(t, r.ExtractOptions(exp, &opts))
	})

	t.Run("vus", func(t *testing.T) {
		exp, err := r.load("test.js", []byte(`
			export let options = { vus: 12345 };
		`))
		assert.NoError(t, err)

		var opts lib.Options
		assert.NoError(t, r.ExtractOptions(exp, &opts))
		assert.Equal(t, int64(12345), opts.VUs)
	})
	t.Run("vusMax", func(t *testing.T) {
		exp, err := r.load("test.js", []byte(`
			export let options = { vusMax: 12345 };
		`))
		assert.NoError(t, err)

		var opts lib.Options
		assert.NoError(t, r.ExtractOptions(exp, &opts))
		assert.Equal(t, int64(12345), opts.VUsMax)
	})
	t.Run("duration", func(t *testing.T) {
		exp, err := r.load("test.js", []byte(`
			export let options = { duration: 120 };
		`))
		assert.NoError(t, err)

		var opts lib.Options
		assert.NoError(t, r.ExtractOptions(exp, &opts))
		assert.Equal(t, 120*time.Second, opts.Duration)
	})
}
