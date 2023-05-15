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
	WebVitals map[string]*k6metrics.Metric
}

// RegisterCustomMetrics creates and registers our custom metrics with the k6
// VU Registry and returns our internal struct pointer.
func RegisterCustomMetrics(registry *k6metrics.Registry) *CustomMetrics {
	wvs := map[string]string{
		webVitalFID:  "browser_web_vital_fid",  // first input delay
		webVitalTTFB: "browser_web_vital_ttfb", // time to first byte
		webVitalLCP:  "browser_web_vital_lcp",  // largest content paint
		webVitalCLS:  "browser_web_vital_cls",  // cumulative layout shift
		webVitalINP:  "browser_web_vital_inp",  // interaction to next paint
		webVitalFCP:  "browser_web_vital_fcp",  // first contentful paint
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
