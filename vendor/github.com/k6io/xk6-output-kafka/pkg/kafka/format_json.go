/*
 *
 * k6 - a next-generation load testing tool
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

package kafka

import (
	"time"

	"github.com/loadimpact/k6/stats"
)

// wrapSample is used to package a metric sample in a way that's nice to export
// to JSON.
func wrapSample(sample stats.Sample) envolope {
	return envolope{
		Type:   "Point",
		Metric: sample.Metric.Name,
		Data:   newJSONSample(sample),
	}
}

// envolope is the data format we use to export both metrics and metric samples
// to the JSON file.
type envolope struct {
	Type   string      `json:"type"`
	Data   interface{} `json:"data"`
	Metric string      `json:"metric,omitempty"`
}

// jsonSample is the data format for metric sample data in the JSON file.
type jsonSample struct {
	Time  time.Time         `json:"time"`
	Value float64           `json:"value"`
	Tags  *stats.SampleTags `json:"tags"`
}

func newJSONSample(sample stats.Sample) jsonSample {
	return jsonSample{
		Time:  sample.Time,
		Value: sample.Value,
		Tags:  sample.Tags,
	}
}
