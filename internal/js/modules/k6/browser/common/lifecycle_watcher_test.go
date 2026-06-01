package common

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common/js"
)

func TestEventFrameDOMMutationIsUnique(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, EventFrameDOMMutation)
	assert.NotEqual(t, EventFrameAddLifecycle, EventFrameDOMMutation)
}

func TestMutationObserverScript(t *testing.T) {
	t.Parallel()

	s := js.DOMMutationObserverScript
	assert.NotEmpty(t, s, "embedded script must be non-empty")
	// Sanity-check the script references the binding the Go side
	// registers; if these drift, integration fails silently.
	assert.True(t, strings.Contains(s, domMutationBinding),
		"script must call window.%s", domMutationBinding)
	assert.True(t, strings.Contains(s, "MutationObserver"),
		"script must install a MutationObserver")
}

func TestDebouncer(t *testing.T) {
	t.Parallel()

	var fired atomic.Int32
	d := newDebouncer(20*time.Millisecond, func() { fired.Add(1) })

	d.trigger()
	// Wait well past the debounce window.
	time.Sleep(80 * time.Millisecond)

	assert.Equal(t, int32(1), fired.Load())
}

func TestDebouncer_CoalescesBurst(t *testing.T) {
	t.Parallel()

	var fired atomic.Int32
	d := newDebouncer(40*time.Millisecond, func() { fired.Add(1) })

	// Hit the debouncer five times in rapid succession; only one fire
	// should happen because each trigger resets the timer.
	for range 5 {
		d.trigger()
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(80 * time.Millisecond)

	assert.Equal(t, int32(1), fired.Load())
}

func TestDebouncer_StopCancelsPending(t *testing.T) {
	t.Parallel()

	var fired atomic.Int32
	d := newDebouncer(30*time.Millisecond, func() { fired.Add(1) })

	d.trigger()
	d.stop()
	time.Sleep(80 * time.Millisecond)

	assert.Equal(t, int32(0), fired.Load())
}

func TestDebouncer_FiresSeparateSettles(t *testing.T) {
	t.Parallel()

	var fired atomic.Int32
	d := newDebouncer(20*time.Millisecond, func() { fired.Add(1) })

	// First settle.
	d.trigger()
	time.Sleep(60 * time.Millisecond)
	// Second settle.
	d.trigger()
	time.Sleep(60 * time.Millisecond)

	assert.Equal(t, int32(2), fired.Load())
}
