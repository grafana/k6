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

package common

import (
	"github.com/loadimpact/k6/stats"
)

const (
	defaultThresholdName = "default"
)

// Threshold defines a test threshold
type Threshold struct {
	Name   string
	Failed bool
}

func generateThreshold(t stats.Threshold) Threshold {
	tSource := t.Source
	if tSource == "" {
		tSource = defaultThresholdName
	}

	return Threshold{
		Name:   tSource,
		Failed: t.LastFailed,
	}
}

func generateDataPoint(sample stats.Sample) *Sample {
	threshold := stats.Threshold{}
	if len(sample.Metric.Thresholds.Thresholds) > 0 {
		threshold = *sample.Metric.Thresholds.Thresholds[0]
	}
	var tags = sample.Tags.CloneTags()
	return &Sample{
		Type:   sample.Metric.Type,
		Metric: sample.Metric.Name,
		Data: SampleData{
			Time:  sample.Time,
			Value: sample.Value,
			Tags:  sample.Tags.CloneTags(),
		},
		Extra: ExtraData{
			Raw:       sample.Metric,
			Threshold: generateThreshold(threshold),
			Group:     tags["group"],
			Check:     tags["check"],
		},
	}
}
