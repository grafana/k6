package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMouseClickOptions(t *testing.T) {
	t.Parallel()

	opts := NewMouseClickOptions()

	assert.Equal(t, "left", opts.Button)
	assert.Equal(t, int64(1), opts.ClickCount)
	assert.Equal(t, int64(0), opts.Delay)
}

func TestNewMouseDblClickOptions(t *testing.T) {
	t.Parallel()

	opts := NewMouseDblClickOptions()

	assert.Equal(t, "left", opts.Button)
	assert.Equal(t, int64(0), opts.Delay)
}

func TestNewMouseDownUpOptions(t *testing.T) {
	t.Parallel()

	opts := NewMouseDownUpOptions()

	assert.Equal(t, "left", opts.Button)
	assert.Equal(t, int64(1), opts.ClickCount)
}

func TestNewMouseMoveOptions(t *testing.T) {
	t.Parallel()

	opts := NewMouseMoveOptions()

	assert.Equal(t, int64(1), opts.Steps)
}

func TestMouseClickOptionsToMouseDownUpOptions(t *testing.T) {
	t.Parallel()

	src := &MouseClickOptions{Button: "right", ClickCount: 3, Delay: 50}

	got := src.ToMouseDownUpOptions()

	// Button and ClickCount are carried over. MouseDownUpOptions has no Delay
	// field, so Delay is intentionally dropped.
	assert.Equal(t, "right", got.Button)
	assert.Equal(t, int64(3), got.ClickCount)
}

func TestMouseDblClickOptionsToMouseClickOptions(t *testing.T) {
	t.Parallel()

	src := &MouseDblClickOptions{Button: "right", Delay: 50}

	got := src.ToMouseClickOptions()

	// Button and Delay are copied, while ClickCount is always forced to 2
	// because this represents a double click.
	assert.Equal(t, "right", got.Button)
	assert.Equal(t, int64(50), got.Delay)
	assert.Equal(t, int64(2), got.ClickCount)
}
