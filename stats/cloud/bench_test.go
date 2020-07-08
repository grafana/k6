/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package cloud

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/netext/httpext"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats"
)

func BenchmarkAggregateHTTP(b *testing.B) {
	script := &loader.SourceData{
		Data: []byte(""),
		URL:  &url.URL{Path: "/script.js"},
	}

	options := lib.Options{
		Duration: types.NullDurationFrom(1 * time.Second),
	}

	config := NewConfig().Apply(Config{
		NoCompress:              null.BoolFrom(true),
		AggregationCalcInterval: types.NullDurationFrom(time.Millisecond * 200),
		AggregationPeriod:       types.NullDurationFrom(time.Millisecond * 200),
	})
	collector, err := New(config, script, options, []lib.ExecutionStep{}, "1.0")
	require.NoError(b, err)
	now := time.Now()
	collector.referenceID = "something"

	for _, tagCount := range []int{1, 5, 10, 100, 1000} {
		tagCount := tagCount
		b.Run(fmt.Sprintf("tags:%d", tagCount), func(b *testing.B) {
			tags := make([]*stats.SampleTags, tagCount)
			for i := range tags {
				tags[i] = stats.IntoSampleTags(&map[string]string{
					"test": "mest", "a": "b",
					"url":  fmt.Sprintf("something%d", i),
					"name": fmt.Sprintf("else%d", i),
				})
			}
			b.ResetTimer()
			for s := 0; s < b.N; s++ {
				for j := time.Duration(1); j <= 200; j++ {
					var container = make([]stats.SampleContainer, 0, 500)
					for i := time.Duration(1); i <= 500; i++ {
						container = append(container, &httpext.Trail{
							Blocked:        i % 200 * 100 * time.Millisecond,
							Connecting:     i % 200 * 200 * time.Millisecond,
							TLSHandshaking: i % 200 * 300 * time.Millisecond,
							Sending:        i * i * 400 * time.Millisecond,
							Waiting:        500 * time.Millisecond,
							Receiving:      600 * time.Millisecond,

							EndTime:      now.Add(i * 100),
							ConnDuration: 500 * time.Millisecond,
							Duration:     j * i * 1500 * time.Millisecond,
							Tags:         stats.NewSampleTags(tags[int(i+j)%len(tags)].CloneTags()),
						})
					}
					collector.Collect(container)
				}
				collector.aggregateHTTPTrails(time.Millisecond * 200)
			}
		})
	}
}
