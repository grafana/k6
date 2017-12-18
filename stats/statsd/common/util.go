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

package statsd

import (
	"fmt"
	"strings"

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
		Failed: t.Failed,
	}
}

func generateDataPoint(sample stats.Sample) *Sample {
	threshold := stats.Threshold{}
	if len(sample.Metric.Thresholds.Thresholds) > 0 {
		threshold = *sample.Metric.Thresholds.Thresholds[0]
	}
	return &Sample{
		Type:   sample.Metric.Type,
		Metric: sample.Metric.Name,
		Data: SampleData{
			Time:  sample.Time,
			Value: sample.Value,
			Tags:  sample.Tags,
		},
		Extra: ExtraData{
			Raw:       sample.Metric,
			Threshold: generateThreshold(threshold),
			Group:     sample.Tags["group"],
			Check:     sample.Tags["check"],
		},
	}
}

// MapToSlice converts a map of tags into a slice of tags
func MapToSlice(tags map[string]string) []string {
	res := []string{}
	for key, value := range tags {
		if value != "" {
			res = append(res, fmt.Sprintf("%s:%v", key, value))
		}
	}
	return res
}

// TakeOnly receives a tag map and a whitelist string and outputs keys that match the whitelist
// This also ignores empty values
func TakeOnly(tags map[string]string, whitelist string) map[string]string {
	res := map[string]string{}
	for key, value := range tags {
		if strings.Contains(whitelist, key) && value != "" {
			res[key] = value
		}
	}
	return res
}
