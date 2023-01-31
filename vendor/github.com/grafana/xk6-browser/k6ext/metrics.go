package k6ext

import k6metrics "go.k6.io/k6/metrics"

// CustomMetrics are the custom k6 metrics used by xk6-browser.
type CustomMetrics struct {
	BrowserDOMContentLoaded     *k6metrics.Metric
	BrowserFirstPaint           *k6metrics.Metric
	BrowserFirstContentfulPaint *k6metrics.Metric
	BrowserFirstMeaningfulPaint *k6metrics.Metric
	BrowserLoaded               *k6metrics.Metric
}

// RegisterCustomMetrics creates and registers our custom metrics with the k6
// VU Registry and returns our internal struct pointer.
func RegisterCustomMetrics(registry *k6metrics.Registry) *CustomMetrics {
	return &CustomMetrics{
		BrowserDOMContentLoaded: registry.MustNewMetric(
			"browser_dom_content_loaded", k6metrics.Trend, k6metrics.Time),
		BrowserFirstPaint: registry.MustNewMetric(
			"browser_first_paint", k6metrics.Trend, k6metrics.Time),
		BrowserFirstContentfulPaint: registry.MustNewMetric(
			"browser_first_contentful_paint", k6metrics.Trend, k6metrics.Time),
		BrowserFirstMeaningfulPaint: registry.MustNewMetric(
			"browser_first_meaningful_paint", k6metrics.Trend, k6metrics.Time),
		BrowserLoaded: registry.MustNewMetric(
			"browser_loaded", k6metrics.Trend, k6metrics.Time),
	}
}
