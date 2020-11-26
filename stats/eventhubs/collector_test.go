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

package eventhubs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/loadimpact/k6/stats"
)

func TestSampleToRow(t *testing.T) {
	sample := &stats.Sample{
		Time:   time.Unix(1562324644, 0),
		Metric: stats.New("my_metric", stats.Gauge),
		Value:  1,
	}

	expected := HubEvent{
		Time:     time.Unix(1562324644, 0),
		Value:    1,
		Tags:     (*stats.SampleTags)(nil),
		Name:     "my_metric",
		Contains: "\"default\"",
	}

	t.Run("test", func(t *testing.T) {
		row := HubEvent{
			Time:     sample.Time,
			Value:    sample.Value,
			Tags:     sample.Tags,
			Name:     sample.Metric.Name,
			Contains: sample.Metric.Contains.String(),
		}
		assert.Equal(t, expected, row)
	})
}
