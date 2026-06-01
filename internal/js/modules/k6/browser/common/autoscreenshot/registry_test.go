package autoscreenshot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

func TestRegistry(t *testing.T) {
	t.Parallel()

	p := newRecordingPersister()
	r := NewRegistry(ModeActions, p, "demo", log.NewNullLogger())
	require.NotNil(t, r)
	assert.Equal(t, ModeActions, r.Mode())

	c1 := r.OnIterStart(1, 5)
	require.NotNil(t, c1)

	// Re-asking for the same (vu, iter) returns the same Capturer.
	assert.Same(t, c1, r.OnIterStart(1, 5))
	assert.Same(t, c1, r.Get(1, 5))

	// A different (vu, iter) gets a fresh Capturer.
	c2 := r.OnIterStart(1, 6)
	require.NotNil(t, c2)
	assert.NotSame(t, c1, c2)

	r.OnIterEnd(1, 5)
	assert.Nil(t, r.Get(1, 5))

	// The other iteration is unaffected.
	assert.Same(t, c2, r.Get(1, 6))

	r.Stop()
	assert.Nil(t, r.Get(1, 6))
}

func TestRegistry_DisabledWhenModeOff(t *testing.T) {
	t.Parallel()

	p := newRecordingPersister()
	r := NewRegistry(ModeOff, p, "demo", log.NewNullLogger())
	assert.Nil(t, r, "ModeOff yields a nil registry so callers can rely on nil-safety")

	// All methods must be nil-safe.
	assert.Nil(t, r.OnIterStart(1, 0))
	assert.Nil(t, r.Get(1, 0))
	r.OnIterEnd(1, 0)
	r.Stop()
	assert.Equal(t, ModeOff, r.Mode())
}

func TestParseMode(t *testing.T) {
	t.Parallel()

	cases := map[string]Mode{
		"actions": ModeActions,
		"":        ModeOff,
		"off":     ModeOff,
		"changes": ModeOff, // historic value; no longer recognised.
		"unknown": ModeOff,
	}
	for in, want := range cases {
		assert.Equal(t, want, ParseMode(in), "ParseMode(%q)", in)
	}
}

func TestMode_String(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "actions", ModeActions.String())
	assert.Equal(t, "off", ModeOff.String())
}
