package tests

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

// TestGoroutineLeakOnRepeatedClicks tests that repeated button fetch and clicks
// don't cause goroutine leaks. It fetches and clicks a button 20 times and verifies
// that goroutine count doesn't increase beyond the threshold (10% locally should be
// enough, 30% for GitHub Actions).
func TestGoroutineLeakOnRepeatedClicks(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	buttonHTML := `
		<button id="test-button" onclick="document.getElementById('counter').innerHTML = parseInt(document.getElementById('counter').innerHTML || 0) + 1;">
			Click me
		</button>
		<div id="counter">0</div>
	`
	err := p.SetContent(buttonHTML, nil)
	require.NoError(t, err)

	// Force GC
	runtime.GC()

	// Measure baseline
	baselineGoroutines := runtime.NumGoroutine()

	t.Logf("Baseline goroutines: %d", baselineGoroutines)

	const clickCount = 20
	for i := range clickCount {
		button, err := p.Query("#test-button")
		require.NoError(t, err)

		err = button.Click(common.NewElementHandleClickOptions(time.Duration(1000) * time.Millisecond))
		require.NoError(t, err, "click %d failed", i+1)
	}

	// Force GC again to ensure we're measuring actual goroutines leaks
	runtime.GC()

	// Get final measurements
	finalGoroutines := runtime.NumGoroutine()

	t.Logf("Final goroutines: %d", finalGoroutines)

	// Calculate and assert goroutine increase is within threshold
	goroutineIncrease := finalGoroutines - baselineGoroutines
	goroutineIncreasePercent := float64(goroutineIncrease) / float64(baselineGoroutines) * 100
	t.Logf("Goroutine increase: %d (%.2f%%)", goroutineIncrease, goroutineIncreasePercent)

	// It should be 10%, but it has to be 30% due to the github actions environment.
	thresholdPercent := 0.30

	maxAllowedGoroutineIncrease := int(float64(baselineGoroutines) * thresholdPercent)
	assert.LessOrEqual(t, goroutineIncrease, maxAllowedGoroutineIncrease,
		"Goroutine count increased by %d (%.2f%%), which exceeds the %.0f%% threshold (%d). Possible goroutine leak detected.",
		goroutineIncrease, goroutineIncreasePercent, thresholdPercent*100, maxAllowedGoroutineIncrease)
}
