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

package json

import (
	"time"

	"github.com/loadimpact/k6/stats"
)

type Envelope struct {
	Type   string      `json:"type"`
	Data   interface{} `json:"data"`
	Metric string      `json:"metric,omitempty"`
}

type JSONSample struct {
	Time  time.Time         `json:"time"`
	Value float64           `json:"value"`
	Tags  map[string]string `json:"tags"`
}

func NewJSONSample(sample *stats.Sample) *JSONSample {
	return &JSONSample{
		Time:  sample.Time,
		Value: sample.Value,
		Tags:  sample.Tags,
	}
}

func WrapSample(sample *stats.Sample) *Envelope {
	if sample == nil {
		return nil
	}
	return &Envelope{
		Type:   "Point",
		Metric: sample.Metric.Name,
		Data:   NewJSONSample(sample),
	}
}

func WrapMetric(metric *stats.Metric) *Envelope {
	if metric == nil {
		return nil
	}

	return &Envelope{
		Type:   "Metric",
		Metric: metric.Name,
		Data:   metric,
	}
}
