package k6ext

import (
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

	browserDataSentName        = "browser_data_sent"
	browserDataReceivedName    = "browser_data_received"
	browserHTTPReqDurationName = "browser_http_req_duration"
	browserHTTPReqFailedName   = "browser_http_req_failed"
)

// CustomMetrics are the custom k6 metrics used by xk6-browser.
type CustomMetrics struct {
	WebVitals map[string]*k6metrics.Metric

	BrowserDataSent        *k6metrics.Metric
	BrowserDataReceived    *k6metrics.Metric
	BrowserHTTPReqDuration *k6metrics.Metric
	BrowserHTTPReqFailed   *k6metrics.Metric
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
	}

	return &CustomMetrics{
		WebVitals:              webVitals,
		BrowserDataSent:        registry.MustNewMetric(browserDataSentName, k6metrics.Counter, k6metrics.Data),
		BrowserDataReceived:    registry.MustNewMetric(browserDataReceivedName, k6metrics.Counter, k6metrics.Data),
		BrowserHTTPReqDuration: registry.MustNewMetric(browserHTTPReqDurationName, k6metrics.Trend, k6metrics.Time),
		BrowserHTTPReqFailed:   registry.MustNewMetric(browserHTTPReqFailedName, k6metrics.Rate),
	}
}
