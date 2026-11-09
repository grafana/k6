package tests

import (
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"

	k6metrics "go.k6.io/k6/v2/metrics"
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

	// A click action helps measure INP (Interaction to Next Paint).
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

// TestWebVitalMetricPerPage asserts that web vitals are recorded for every
// page in a single-tab navigation flow, not just the last one. It navigates
// one page across two distinct URLs and, without interacting with the first
// page, checks that the now-intermediate page still reports its LCP (as well
// as FCP and TTFB), with exactly one sample per (url, metric).
func TestWebVitalMetricPerPage(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("timeouts on windows")
	}

	var (
		samples = make(chan k6metrics.SampleContainer, 1000)
		browser = newTestBrowser(t, withFileServer(), withSamples(samples))
		page    = browser.NewPage(nil)
	)

	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}

	// Navigate to the first page and dwell so its web vitals settle and are
	// reported before navigating away. The page is not interacted with, so
	// without continuous reporting its LCP is never finalized.
	resp, err := page.Goto(browser.staticURL("/web_vitals.html"), opts)
	require.NoError(t, err)
	require.NotNil(t, resp)

	page.WaitForTimeout(1000)

	// Navigate to a second, distinct page. This makes web_vitals.html an
	// intermediate page whose web vitals would be lost without continuous
	// reporting.
	resp, err = page.Goto(browser.staticURL("/usual.html"), opts)
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.NoError(t, page.Close())
	close(samples)

	// Count web vital samples per (url, metric).
	type urlMetric struct{ url, metric string }
	counts := make(map[urlMetric]int)
	for sc := range samples {
		for _, s := range sc.GetSamples() {
			if !strings.HasPrefix(s.Metric.Name, "browser_web_vital_") {
				continue
			}
			url, ok := s.Tags.Get("url")
			require.Truef(t, ok, "web vital sample %s is missing a url tag", s.Metric.Name)
			counts[urlMetric{url: url, metric: s.Metric.Name}]++
		}
	}

	// The intermediate page must have recorded LCP, the key differentiator:
	// without continuous reporting it finalizes only on visibilitychange,
	// which does not fire on a hard navigation. FCP and TTFB are asserted as
	// a sanity check (they finalize early during load).
	intermediateHas := func(metric string) bool {
		for k := range counts {
			if k.metric == metric && strings.Contains(k.url, "web_vitals.html") {
				return true
			}
		}
		return false
	}
	assert.True(t, intermediateHas("browser_web_vital_lcp"),
		"expected intermediate page web_vitals.html to record LCP")
	assert.True(t, intermediateHas("browser_web_vital_fcp"),
		"expected intermediate page web_vitals.html to record FCP")
	assert.True(t, intermediateHas("browser_web_vital_ttfb"),
		"expected intermediate page web_vitals.html to record TTFB")

	// Continuous reporting must not duplicate samples: each (url, metric)
	// pair must be emitted exactly once.
	for k, n := range counts {
		assert.Equalf(t, 1, n,
			"expected exactly one %s sample for %s, got %d", k.metric, k.url, n)
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
