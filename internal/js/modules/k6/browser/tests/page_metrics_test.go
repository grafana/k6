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

// TestPageOrderTagAndPageDuration asserts that browser metrics carry a
// page_order tag identifying the main-frame navigation they belong to, and
// that a browser_page_duration sample is emitted per navigation.
func TestPageOrderTagAndPageDuration(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("timeouts on windows")
	}

	var (
		samples = make(chan k6metrics.SampleContainer, 1000)
		browser = newTestBrowser(t, withFileServer(), withSamples(samples))
	)
	page := browser.NewPage(nil)

	// The response is not asserted on: Page.Goto can return a nil response
	// (with a nil error) under load, unrelated to what this test verifies.
	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	_, err := page.Goto(browser.staticURL("page1.html"), opts)
	require.NoError(t, err)

	opts = &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	_, err = page.Goto(browser.staticURL("page2.html"), opts)
	require.NoError(t, err)

	require.NoError(t, page.Close())
	close(samples)

	var (
		// page_order values seen per document url, per metric family.
		durationOrders = make(map[string]map[string]bool)
		networkOrders  = make(map[string]map[string]bool)
		vitalOrders    = make(map[string]map[string]bool)
	)
	record := func(m map[string]map[string]bool, url, order string) {
		if m[url] == nil {
			m[url] = make(map[string]bool)
		}
		m[url][order] = true
	}
	for container := range samples {
		for _, s := range container.GetSamples() {
			url, _ := s.Tags.Get("url")
			// Only assert on the two documents we navigated to, to avoid
			// depending on incidental requests (e.g. favicon).
			var doc string
			switch {
			case strings.HasSuffix(url, "page1.html"):
				doc = "page1"
			case strings.HasSuffix(url, "page2.html"):
				doc = "page2"
			default:
				continue
			}

			order, ok := s.Tags.Get("page_order")
			assert.Truef(t, ok, "sample %s (%s) should have a page_order tag", s.Metric.Name, url)

			switch {
			case s.Metric.Name == "browser_page_duration":
				assert.Positivef(t, s.Value, "%s for %s should be positive", s.Metric.Name, url)
				record(durationOrders, doc, order)
			case strings.HasPrefix(s.Metric.Name, "browser_web_vital_"):
				record(vitalOrders, doc, order)
			case strings.HasPrefix(s.Metric.Name, "browser_http_req_"),
				strings.HasPrefix(s.Metric.Name, "browser_data_"):
				record(networkOrders, doc, order)
			}
		}
	}

	want := map[string]map[string]bool{
		"page1": {"0": true},
		"page2": {"1": true},
	}
	assert.Equal(t, want, durationOrders, "browser_page_duration should be emitted once per navigation with its page_order")
	assert.Equal(t, want, networkOrders, "network metrics should be tagged with the page_order of their navigation")

	// Web vitals for the last page are flushed when the page closes. Vitals
	// for earlier pages are flushed on pagehide while navigating away and
	// may be lost to the CDP binding teardown race, so only assert on the
	// attribution of the vitals that did arrive.
	assert.Contains(t, vitalOrders, "page2", "web vitals should be emitted for the last page")
	for doc, orders := range vitalOrders {
		assert.Equalf(t, want[doc], orders, "web vitals for %s should be tagged with its page_order", doc)
	}
}
