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

package v1

import (
	"encoding/json"
	"testing"

	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
)

func TestNullMetricTypeJSON(t *testing.T) {
	values := map[NullMetricType]string{
		{}: `null`,
		{stats.Counter, true}: `"counter"`,
		{stats.Gauge, true}:   `"gauge"`,
		{stats.Trend, true}:   `"trend"`,
		{stats.Rate, true}:    `"rate"`,
	}
	t.Run("Marshal", func(t *testing.T) {
		for mt, val := range values {
			t.Run(val, func(t *testing.T) {
				data, err := json.Marshal(mt)
				assert.NoError(t, err)
				assert.Equal(t, val, string(data))
			})
		}
	})
	t.Run("Unmarshal", func(t *testing.T) {
		for mt, val := range values {
			t.Run(val, func(t *testing.T) {
				var value NullMetricType
				assert.NoError(t, json.Unmarshal([]byte(val), &value))
				assert.Equal(t, mt, value)
			})
		}
	})
}

func TestNullValueTypeJSON(t *testing.T) {
	values := map[NullValueType]string{
		{}: `null`,
		{stats.Default, true}: `"default"`,
		{stats.Time, true}:    `"time"`,
	}
	t.Run("Marshal", func(t *testing.T) {
		for mt, val := range values {
			t.Run(val, func(t *testing.T) {
				data, err := json.Marshal(mt)
				assert.NoError(t, err)
				assert.Equal(t, val, string(data))
			})
		}
	})
	t.Run("Unmarshal", func(t *testing.T) {
		for mt, val := range values {
			t.Run(val, func(t *testing.T) {
				var value NullValueType
				assert.NoError(t, json.Unmarshal([]byte(val), &value))
				assert.Equal(t, mt, value)
			})
		}
	})
}

func TestNewMetric(t *testing.T) {
	old := stats.Metric{
		Name:     "name",
		Type:     stats.Trend,
		Contains: stats.Time,
		Tainted:  null.BoolFrom(true),
	}
	sink := old.NewSink()
	m := NewMetric(old, sink)
	assert.Equal(t, "name", m.Name)
	assert.True(t, m.Type.Valid)
	assert.Equal(t, stats.Trend, m.Type.Type)
	assert.True(t, m.Contains.Valid)
	assert.True(t, m.Tainted.Bool)
	assert.True(t, m.Tainted.Valid)
	assert.Equal(t, stats.Time, m.Contains.Type)
	assert.NotEmpty(t, m.Sample)
}
