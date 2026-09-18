package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTouchscreenTapRequiresHasTouch(t *testing.T) {
	t.Parallel()

	ts := NewTouchscreen(t.Context(), nil, &Keyboard{}, false)
	err := ts.Tap(5, 5)
	require.Error(t, err)
	assert.ErrorContains(t, err, "hasTouch must be enabled on the browser context")
}
