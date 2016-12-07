package lib

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
	"testing"
)

func TestOptionsApply(t *testing.T) {
	t.Run("Paused", func(t *testing.T) {
		opts := Options{}.Apply(Options{Paused: null.BoolFrom(true)})
		assert.True(t, opts.Paused.Valid)
		assert.True(t, opts.Paused.Bool)
	})
	t.Run("VUs", func(t *testing.T) {
		opts := Options{}.Apply(Options{VUs: null.IntFrom(12345)})
		assert.True(t, opts.VUs.Valid)
		assert.Equal(t, int64(12345), opts.VUs.Int64)
	})
	t.Run("VUsMax", func(t *testing.T) {
		opts := Options{}.Apply(Options{VUsMax: null.IntFrom(12345)})
		assert.True(t, opts.VUsMax.Valid)
		assert.Equal(t, int64(12345), opts.VUsMax.Int64)
	})
	t.Run("Duration", func(t *testing.T) {
		opts := Options{}.Apply(Options{Duration: null.StringFrom("2m")})
		assert.True(t, opts.Duration.Valid)
		assert.Equal(t, "2m", opts.Duration.String)
	})
	t.Run("Linger", func(t *testing.T) {
		opts := Options{}.Apply(Options{Linger: null.BoolFrom(true)})
		assert.True(t, opts.Linger.Valid)
		assert.True(t, opts.Linger.Bool)
	})
	t.Run("AbortOnTaint", func(t *testing.T) {
		opts := Options{}.Apply(Options{AbortOnTaint: null.BoolFrom(true)})
		assert.True(t, opts.AbortOnTaint.Valid)
		assert.True(t, opts.AbortOnTaint.Bool)
	})
	t.Run("Acceptance", func(t *testing.T) {
		opts := Options{}.Apply(Options{Acceptance: null.FloatFrom(12345.0)})
		assert.True(t, opts.Acceptance.Valid)
		assert.Equal(t, float64(12345.0), opts.Acceptance.Float64)
	})
	t.Run("MaxRedirects", func(t *testing.T) {
		opts := Options{}.Apply(Options{MaxRedirects: null.IntFrom(12345)})
		assert.True(t, opts.MaxRedirects.Valid)
		assert.Equal(t, int64(12345), opts.MaxRedirects.Int64)
	})
	t.Run("Thresholds", func(t *testing.T) {
		opts := Options{}.Apply(Options{Thresholds: map[string][]*Threshold{
			"metric": []*Threshold{&Threshold{Source: "1+1==2"}},
		}})
		assert.NotNil(t, opts.Thresholds)
		assert.NotEmpty(t, opts.Thresholds)
	})
}
