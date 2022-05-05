/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package common

import k6metrics "go.k6.io/k6/metrics"

// CustomK6Metrics are the custom k6 metrics used by xk6-browser.
type CustomK6Metrics struct {
	BrowserDOMContentLoaded     *k6metrics.Metric
	BrowserFirstPaint           *k6metrics.Metric
	BrowserFirstContentfulPaint *k6metrics.Metric
	BrowserFirstMeaningfulPaint *k6metrics.Metric
	BrowserLoaded               *k6metrics.Metric
}

// RegisterCustomK6Metrics creates and registers our custom metrics with the k6
// VU Registry and returns our internal struct pointer.
func RegisterCustomK6Metrics(registry *k6metrics.Registry) *CustomK6Metrics {
	return &CustomK6Metrics{
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
