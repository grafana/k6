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
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/mailru/easyjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/netext/httpext"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
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
	collector, err := New(testutils.NewLogger(b), config, script, options, []lib.ExecutionStep{}, "1.0")
	require.NoError(b, err)
	now := time.Now()
	collector.referenceID = "something"
	containersCount := 500000

	for _, tagCount := range []int{1, 5, 35, 315, 3645} {
		tagCount := tagCount
		b.Run(fmt.Sprintf("tags:%d", tagCount), func(b *testing.B) {
			b.ResetTimer()
			for s := 0; s < b.N; s++ {
				b.StopTimer()
				container := make([]stats.SampleContainer, containersCount)
				for i := 1; i <= containersCount; i++ {
					status := "200"
					if i%tagCount%7 == 6 {
						status = "404"
					} else if i%tagCount%7 == 5 {
						status = "500"
					}

					tags := generateTags(i, tagCount, map[string]string{"status": status})
					container[i-1] = generateHTTPExtTrail(now, time.Duration(i), tags)
				}
				collector.Collect(container)
				b.StartTimer()
				collector.aggregateHTTPTrails(time.Millisecond * 200)
				collector.bufferSamples = nil
			}
		})
	}
}

func generateTags(i, tagCount int, additionals ...map[string]string) *stats.SampleTags {
	res := map[string]string{
		"test": "mest", "a": "b",
		"custom": fmt.Sprintf("group%d", i%tagCount%9),
		"group":  fmt.Sprintf("group%d", i%tagCount%5),
		"url":    fmt.Sprintf("something%d", i%tagCount%11),
		"name":   fmt.Sprintf("else%d", i%tagCount%11),
	}
	for _, a := range additionals {
		for k, v := range a {
			res[k] = v
		}
	}

	return stats.IntoSampleTags(&res)
}

func BenchmarkMetricMarshal(b *testing.B) {
	for _, count := range []int{10000, 100000, 500000} {
		count := count
		b.Run(fmt.Sprintf("%d", count), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				s := generateSamples(count)
				b.StartTimer()
				r, err := easyjson.Marshal(samples(s))
				require.NoError(b, err)
				b.SetBytes(int64(len(r)))
			}
		})
	}
}

func BenchmarkMetricMarshalWriter(b *testing.B) {
	for _, count := range []int{10000, 100000, 500000} {
		count := count
		b.Run(fmt.Sprintf("%d", count), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				s := generateSamples(count)
				b.StartTimer()
				n, err := easyjson.MarshalToWriter(samples(s), ioutil.Discard)
				require.NoError(b, err)
				b.SetBytes(int64(n))
			}
		})
	}
}

func BenchmarkMetricMarshalGzip(b *testing.B) {
	for _, count := range []int{10000, 100000, 500000} {
		for name, level := range map[string]int{
			"bestcompression": gzip.BestCompression,
			"default":         gzip.DefaultCompression,
			"bestspeed":       gzip.BestSpeed,
		} {
			count := count
			level := level
			b.Run(fmt.Sprintf("%d_%s", count, name), func(b *testing.B) {
				s := generateSamples(count)
				r, err := easyjson.Marshal(samples(s))
				require.NoError(b, err)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					var buf bytes.Buffer
					buf.Grow(len(r) / 5)
					g, err := gzip.NewWriterLevel(&buf, level)
					require.NoError(b, err)
					b.StartTimer()
					n, err := g.Write(r)
					require.NoError(b, err)
					b.SetBytes(int64(n))
					b.ReportMetric(float64(len(r))/float64(buf.Len()), "ratio")
				}
			})
		}
	}
}

func BenchmarkMetricMarshalGzipAll(b *testing.B) {
	for _, count := range []int{10000, 100000, 500000} {
		for name, level := range map[string]int{
			"bestspeed": gzip.BestSpeed,
		} {
			count := count
			level := level
			b.Run(fmt.Sprintf("%d_%s", count, name), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					b.StopTimer()

					s := generateSamples(count)
					var buf bytes.Buffer
					g, err := gzip.NewWriterLevel(&buf, level)
					require.NoError(b, err)
					b.StartTimer()

					r, err := easyjson.Marshal(samples(s))
					require.NoError(b, err)
					buf.Grow(len(r) / 5)
					n, err := g.Write(r)
					require.NoError(b, err)
					b.SetBytes(int64(n))
				}
			})
		}
	}
}

func BenchmarkMetricMarshalGzipAllWriter(b *testing.B) {
	for _, count := range []int{10000, 100000, 500000} {
		for name, level := range map[string]int{
			"bestspeed": gzip.BestSpeed,
		} {
			count := count
			level := level
			b.Run(fmt.Sprintf("%d_%s", count, name), func(b *testing.B) {
				var buf bytes.Buffer
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					buf.Reset()

					s := generateSamples(count)
					g, err := gzip.NewWriterLevel(&buf, level)
					require.NoError(b, err)
					pr, pw := io.Pipe()
					b.StartTimer()

					go func() {
						_, _ = easyjson.MarshalToWriter(samples(s), pw)
						_ = pw.Close()
					}()
					n, err := io.Copy(g, pr)
					require.NoError(b, err)
					b.SetBytes(n)
				}
			})
		}
	}
}

func generateSamples(count int) []*Sample {
	samples := make([]*Sample, count)
	now := time.Now()
	for i := range samples {
		tags := generateTags(i, 200)
		switch i % 3 {
		case 0:
			samples[i] = &Sample{
				Type:   DataTypeSingle,
				Metric: "something",
				Data: &SampleDataSingle{
					Time:  toMicroSecond(now),
					Type:  stats.Counter,
					Tags:  tags,
					Value: float64(i),
				},
			}
		case 1:
			aggrData := &SampleDataAggregatedHTTPReqs{
				Time: toMicroSecond(now),
				Type: "aggregated_trend",
				Tags: tags,
			}
			trail := generateHTTPExtTrail(now, time.Duration(i), tags)
			aggrData.Add(trail)
			aggrData.Add(trail)
			aggrData.Add(trail)
			aggrData.Add(trail)
			aggrData.Add(trail)
			aggrData.CalcAverages()

			samples[i] = &Sample{
				Type:   DataTypeAggregatedHTTPReqs,
				Metric: "something",
				Data:   aggrData,
			}
		default:
			samples[i] = NewSampleFromTrail(generateHTTPExtTrail(now, time.Duration(i), tags))
		}
	}

	return samples
}

func generateHTTPExtTrail(now time.Time, i time.Duration, tags *stats.SampleTags) *httpext.Trail {
	return &httpext.Trail{
		Blocked:        i % 200 * 100 * time.Millisecond,
		Connecting:     i % 200 * 200 * time.Millisecond,
		TLSHandshaking: i % 200 * 300 * time.Millisecond,
		Sending:        i % 200 * 400 * time.Millisecond,
		Waiting:        500 * time.Millisecond,
		Receiving:      600 * time.Millisecond,
		EndTime:        now.Add(i % 100 * 100),
		ConnDuration:   500 * time.Millisecond,
		Duration:       i % 150 * 1500 * time.Millisecond,
		Tags:           tags,
	}
}

func BenchmarkHTTPPush(b *testing.B) {
	script := &loader.SourceData{
		Data: []byte(""),
		URL:  &url.URL{Path: "/script.js"},
	}

	options := lib.Options{
		Duration: types.NullDurationFrom(1 * time.Second),
	}
	tb := httpmultibin.NewHTTPMultiBin(b)
	tb.Mux.HandleFunc("/v1/tests", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprint(w, `{
			"reference_id": "fake",
		}`)
		require.NoError(b, err)
	}))
	defer tb.Cleanup()
	tb.Mux.HandleFunc("/v1/metrics/fake",
		func(w http.ResponseWriter, r *http.Request) {
			_, err := io.Copy(ioutil.Discard, r.Body)
			assert.NoError(b, err)
		},
	)

	config := NewConfig().Apply(Config{
		Host:                    null.StringFrom(tb.ServerHTTP.URL),
		AggregationCalcInterval: types.NullDurationFrom(time.Millisecond * 200),
		AggregationPeriod:       types.NullDurationFrom(time.Millisecond * 200),
	})
	collector, err := New(testutils.NewLogger(b), config, script, options, []lib.ExecutionStep{}, "1.0")
	require.NoError(b, err)
	collector.referenceID = "fake"

	for _, count := range []int{1000, 5000, 50000, 100000, 250000} {
		count := count
		b.Run(fmt.Sprintf("count:%d", count), func(b *testing.B) {
			samples := generateSamples(count)
			b.ResetTimer()
			for s := 0; s < b.N; s++ {
				b.StopTimer()
				toSend := append([]*Sample{}, samples...)
				b.StartTimer()
				require.NoError(b, collector.client.PushMetric("fake", false, toSend))
			}
		})
	}
}
