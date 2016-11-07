package js

import (
	"context"
	"github.com/loadimpact/speedboat/lib"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewRunner(t *testing.T) {
	rt, err := New()
	assert.NoError(t, err)
	exp, err := rt.load("test.js", []byte(`export default function() {}`))
	assert.NoError(t, err)
	r, err := NewRunner(rt, exp)
	assert.NoError(t, err)
	if !assert.NotNil(t, r) {
		return
	}

	t.Run("GetGroups", func(t *testing.T) {
		g := r.GetGroups()
		assert.Len(t, g, 1)
		assert.Equal(t, r.DefaultGroup, g[0])
	})

	t.Run("GetTests", func(t *testing.T) {
		assert.Len(t, r.GetChecks(), 0)
	})

	t.Run("VU", func(t *testing.T) {
		vu_, err := r.NewVU()
		assert.NoError(t, err)
		vu := vu_.(*VU)

		t.Run("Reconfigure", func(t *testing.T) {
			vu.Reconfigure(12345)
			assert.Equal(t, int64(12345), vu.ID)
		})

		t.Run("RunOnce", func(t *testing.T) {
			_, err := vu.RunOnce(context.Background(), &lib.Status{})
			assert.NoError(t, err)
		})
	})
}
