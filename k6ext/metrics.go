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

	fidName  = "browser_web_vital_fid"
	ttfbName = "browser_web_vital_ttfb"
	lcpName  = "browser_web_vital_lcp"
	clsName  = "browser_web_vital_cls"
	inpName  = "browser_web_vital_inp"
	fcpName  = "browser_web_vital_fcp"

	browserDataSentName = "browser_data_sent"
	browserHTTPReqsName = "browser_http_reqs"
)

// CustomMetrics are the custom k6 metrics used by xk6-browser.
type CustomMetrics struct {
	WebVitals map[string]*k6metrics.Metric

	BrowserDataSent *k6metrics.Metric
	BrowserHTTPReqs *k6metrics.Metric
}

// RegisterCustomMetrics creates and registers our custom metrics with the k6
// VU Registry and returns our internal struct pointer.
func RegisterCustomMetrics(registry *k6metrics.Registry) *CustomMetrics {
	wvs := map[string]string{
		webVitalFID:  fidName,  // first input delay
		webVitalTTFB: ttfbName, // time to first byte
		webVitalLCP:  lcpName,  // largest content paint
		webVitalCLS:  clsName,  // cumulative layout shift
		webVitalINP:  inpName,  // interaction to next paint
		webVitalFCP:  fcpName,  // first contentful paint
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
		WebVitals:       webVitals,
		BrowserDataSent: registry.MustNewMetric(browserDataSentName, k6metrics.Counter, k6metrics.Data),
		BrowserHTTPReqs: registry.MustNewMetric(browserHTTPReqsName, k6metrics.Counter),
	}
}

// ConcatWebVitalNameRating can be used
// to create the correct metric key name
// to retrieve the corresponding metric
// from the registry.
func ConcatWebVitalNameRating(name, rating string) string {
	return fmt.Sprintf("%s:%s", name, rating)
}
