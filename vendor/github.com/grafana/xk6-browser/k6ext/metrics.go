package k6ext

import (
	"fmt"

	k6metrics "go.k6.io/k6/metrics"
)

const (
	webVitalFID  = "FID"
	webVitalTTFB = "TTFB"
	webVitalLCP  = "LCP"
	webVitalCLS  = "CLS"
	webVitalINP  = "INP"
	webVitalFCP  = "FCP"
)

// CustomMetrics are the custom k6 metrics used by xk6-browser.
type CustomMetrics struct {
	BrowserDOMContentLoaded *k6metrics.Metric
	BrowserFirstPaint       *k6metrics.Metric
	BrowserLoaded           *k6metrics.Metric

	WebVitals map[string]*k6metrics.Metric
}

// RegisterCustomMetrics creates and registers our custom metrics with the k6
// VU Registry and returns our internal struct pointer.
func RegisterCustomMetrics(registry *k6metrics.Registry) *CustomMetrics {
	wvs := map[string]string{
		webVitalFID:  "webvital_first_input_delay",
		webVitalTTFB: "webvital_time_to_first_byte",
		webVitalLCP:  "webvital_largest_content_paint",
		webVitalCLS:  "webvital_cumulative_layout_shift",
		webVitalINP:  "webvital_interaction_to_next_paint",
		webVitalFCP:  "webvital_first_contentful_paint",
	}
	webVitals := make(map[string]*k6metrics.Metric)

	for k, v := range wvs {
		t := k6metrics.Time
		// CLS is not a time based measurement, it is a score,
		// so use the default metric type for CLS.
		if k == webVitalCLS {
			t = k6metrics.Default
		}

		webVitals[k] = registry.MustNewMetric(v, k6metrics.Trend, t)

		webVitals[ConcatWebVitalNameRating(k, "good")] = registry.MustNewMetric(
			v+"_good", k6metrics.Counter)
		webVitals[ConcatWebVitalNameRating(k, "needs-improvement")] = registry.MustNewMetric(
			v+"_needs_improvement", k6metrics.Counter)
		webVitals[ConcatWebVitalNameRating(k, "poor")] = registry.MustNewMetric(
			v+"_poor", k6metrics.Counter)
	}

	return &CustomMetrics{
		BrowserDOMContentLoaded: registry.MustNewMetric(
			"browser_dom_content_loaded", k6metrics.Trend, k6metrics.Time),
		BrowserFirstPaint: registry.MustNewMetric(
			"browser_first_paint", k6metrics.Trend, k6metrics.Time),
		BrowserLoaded: registry.MustNewMetric(
			"browser_loaded", k6metrics.Trend, k6metrics.Time),
		WebVitals: webVitals,
	}
}

// ConcatWebVitalNameRating can be used
// to create the correct metric key name
// to retrieve the corresponding metric
// from the registry.
func ConcatWebVitalNameRating(name, rating string) string {
	return fmt.Sprintf("%s:%s", name, rating)
}
