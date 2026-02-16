package tests

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"

	k6metrics "go.k6.io/k6/metrics"
)

// TestWebVitalMetric is asserting that web vital metrics
// are being emitted when navigating and interacting with
// a web page.
func TestWebVitalMetric(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("timeouts on windows")
	}
	var (
		samples  = make(chan k6metrics.SampleContainer, 1000)
		browser  = newTestBrowser(t, withFileServer(), withSamples(samples))
		page     = browser.NewPage(nil)
		expected = map[string]bool{
			"browser_web_vital_ttfb": false,
			"browser_web_vital_fcp":  false,
			"browser_web_vital_lcp":  false,
			"browser_web_vital_fid":  false,
			"browser_web_vital_cls":  false,
		}
	)

	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	resp, err := page.Goto(
		browser.staticURL("/web_vitals.html"),
		opts,
	)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// A click action helps measure first input delay.
	// The click action also refreshes the page, which
	// also helps the web vital library to measure CLS.
	err = browser.run(
		browser.context(),
		func() error { return page.Click("#clickMe", common.NewFrameClickOptions(page.Timeout())) },
		func() error {
			_, err := page.WaitForNavigation(
				common.NewFrameWaitForNavigationOptions(page.Timeout()), nil)
			return err
		},
	)
	require.NoError(t, err)

	require.NoError(t, page.Close())
	close(samples)
	markExpectedWebVitalsFromSamples(samples, expected)

	for k, v := range expected {
		assert.True(t, v, "expected %s to have been measured and emitted", k)
	}
}

func TestWebVitalMetricNoInteraction(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("timeouts on windows")
	}
	var (
		samples  = make(chan k6metrics.SampleContainer, 1000)
		browser  = newTestBrowser(t, withFileServer(), withSamples(samples))
		expected = map[string]bool{
			"browser_web_vital_ttfb": false,
			"browser_web_vital_fcp":  false,
			"browser_web_vital_lcp":  false,
			"browser_web_vital_cls":  false,
		}
	)

	page := browser.NewPage(nil)
	opts := &common.FrameGotoOptions{
		// Wait for both load and network idle events
		WaitUntil: common.LifecycleEventNetworkIdle,
		Timeout:   common.DefaultTimeout,
	}
	resp, err := page.Goto(
		browser.staticURL("web_vitals.html"),
		opts,
	)
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.NoError(t, page.Close())
	close(samples)
	markExpectedWebVitalsFromSamples(samples, expected)

	for k, v := range expected {
		assert.True(t, v, "expected %s to have been measured and emitted", k)
	}
}

func markExpectedWebVitalsFromSamples(samples <-chan k6metrics.SampleContainer, expected map[string]bool) {
	for metric := range samples {
		for _, s := range metric.GetSamples() {
			if _, ok := expected[s.Metric.Name]; ok {
				expected[s.Metric.Name] = true
			}
		}
	}
}
