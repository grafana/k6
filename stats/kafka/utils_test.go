/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package kafka

import (
	"testing"

	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
)

func TestFormatSamples(t *testing.T) {
	metric := stats.New("my_metric", stats.Gauge)
	samples := stats.Samples{
		{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1"})},
		{Metric: metric, Value: 2, Tags: stats.IntoSampleTags(&map[string]string{"b": "2"})},
	}

	fmtdSamples, err := formatSamples("influx", samples)

	assert.Nil(t, err)
	assert.Equal(t, []string{"my_metric,a=1 value=1.25", "my_metric,b=2 value=2"}, fmtdSamples)

	fmtdSamples, err = formatSamples("json", samples)

	expJSON1 := "{\"type\":\"Point\",\"data\":{\"time\":\"0001-01-01T00:00:00Z\",\"value\":1.25,\"tags\":{\"a\":\"1\"}},\"metric\":\"my_metric\"}"
	expJSON2 := "{\"type\":\"Point\",\"data\":{\"time\":\"0001-01-01T00:00:00Z\",\"value\":2,\"tags\":{\"b\":\"2\"}},\"metric\":\"my_metric\"}"

	assert.Nil(t, err)
	assert.Equal(t, []string{expJSON1, expJSON2}, fmtdSamples)
}
