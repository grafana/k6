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
	"github.com/loadimpact/k6/stats"
	"time"
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

func Wrap(t interface{}) *Envelope {
	switch data := t.(type) {
	case stats.Sample:
		return &Envelope{
			Type:   "Point",
			Metric: data.Metric.Name,
			Data:   NewJSONSample(&data),
		}
	case *stats.Metric:
		return &Envelope{
			Type:   "Metric",
			Metric: data.Name,
			Data:   data,
		}
	}
	return nil
}
