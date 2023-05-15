package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	k6metrics "go.k6.io/k6/metrics"
)

// TestWebVitalMetric is asserting that web vital metrics
// are being emitted when navigating and interacting with
// a web page.
func TestWebVitalMetric(t *testing.T) {
	var (
		samples  = make(chan k6metrics.SampleContainer)
		browser  = newTestBrowser(t, withFileServer(), withSamplesListener(samples))
		page     = browser.NewPage(nil)
		expected = map[string]bool{
			"browser_web_vital_ttfb":      false,
			"browser_web_vital_ttfb_good": false,
			"browser_web_vital_fcp":       false,
			"browser_web_vital_fcp_good":  false,
			"browser_web_vital_lcp":       false,
			"browser_web_vital_lcp_good":  false,
			"browser_web_vital_fid":       false,
			"browser_web_vital_fid_good":  false,
			"browser_web_vital_cls":       false,
			"browser_web_vital_cls_good":  false,
		}
	)

	count := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	go func() {
		for {
			metric := <-samples
			samples := metric.GetSamples()
			for _, s := range samples {
				if _, ok := expected[s.Metric.Name]; ok {
					expected[s.Metric.Name] = true
					count++
				}
			}
			if count == len(expected) {
				cancel()
			}
		}
	}()

	resp, err := page.Goto(browser.staticURL("/web_vitals.html"), nil)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// A click action helps measure first input delay.
	// The click action also refreshes the page, which
	// also helps the web vital library to measure CLS.
	err = page.Click("#clickMe", nil)
	require.NoError(t, err)

	<-ctx.Done()

	for k, v := range expected {
		assert.True(t, v, "expected %s to have been measured and emitted", k)
	}
}
