package js

import (
	"github.com/loadimpact/k6/lib"
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
	assert.NoError(t, r.VM.Set("__initapi__", InitAPI{r: r}))

	t.Run("Importing Libraries", func(t *testing.T) {
		_, err := r.load("test.js", []byte(`
			import "k6";
		`))
		assert.NoError(t, err)
		assert.Contains(t, r.lib, "k6.js")
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
		assert.True(t, opts.VUs.Valid)
		assert.Equal(t, int64(12345), opts.VUs.Int64)
	})
	t.Run("vus-max", func(t *testing.T) {
		exp, err := r.load("test.js", []byte(`
			export let options = { "vus-max": 12345 };
		`))
		assert.NoError(t, err)

		var opts lib.Options
		assert.NoError(t, r.ExtractOptions(exp, &opts))
		assert.True(t, opts.VUsMax.Valid)
		assert.Equal(t, int64(12345), opts.VUsMax.Int64)
	})
	t.Run("duration", func(t *testing.T) {
		exp, err := r.load("test.js", []byte(`
			export let options = { duration: "2m" };
		`))
		assert.NoError(t, err)

		var opts lib.Options
		assert.NoError(t, r.ExtractOptions(exp, &opts))
		assert.True(t, opts.Duration.Valid)
		assert.Equal(t, "2m", opts.Duration.String)
	})
	t.Run("thresholds", func(t *testing.T) {
		exp, err := r.load("test.js", []byte(`
			export let options = {
				thresholds: {
					my_metric: ["value<=1000"],
				}
			}
		`))
		assert.NoError(t, err)

		var opts lib.Options
		assert.NoError(t, r.ExtractOptions(exp, &opts))
		assert.Contains(t, opts.Thresholds, "my_metric")
		if assert.Len(t, opts.Thresholds["my_metric"], 1) {
			assert.Equal(t, &lib.Threshold{Source: "value<=1000"}, opts.Thresholds["my_metric"][0])
		}
	})
}
