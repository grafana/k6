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

package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	k6stats "go.k6.io/k6/stats"
)

func assertMetricsEmitted(t *testing.T, samples []k6stats.SampleContainer,
	expMetricTags map[string]map[string]string, callback func(sample k6stats.Sample)) {
	t.Helper()

	metricMap := make(map[string]bool, len(expMetricTags))
	for m := range expMetricTags {
		metricMap[m] = false
	}

	for _, container := range samples {
		for _, sample := range container.GetSamples() {
			tags := sample.Tags.CloneTags()
			v, ok := metricMap[sample.Metric.Name]
			assert.True(t, ok, "unexpected metric %s", sample.Metric.Name)
			// Already seen this metric, skip it.
			// TODO: Fail on repeated metrics?
			if v {
				continue
			}
			metricMap[sample.Metric.Name] = true
			expTags := expMetricTags[sample.Metric.Name]
			assert.EqualValues(t, expTags, tags,
				"tags for metric %s don't match", sample.Metric.Name)
			if callback != nil {
				callback(sample)
			}
		}
	}

	for k, v := range metricMap {
		assert.True(t, v, "didn't emit %s", k)
	}
}
