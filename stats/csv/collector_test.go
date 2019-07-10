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

package csv

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/loadimpact/k6/stats"

	"github.com/loadimpact/k6/lib"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestMakeHeader(t *testing.T) {
	testdata := map[string][]string{
		"One tag": {
			"tag1",
		},
		"Two tags": {
			"tag1", "tag2",
		},
	}

	for testname, tags := range testdata {
		func(testname string, tags []string) {
			t.Run(testname, func(t *testing.T) {
				header := MakeHeader(tags)
				assert.Equal(t, len(tags)+4, len(header))
				assert.Equal(t, "metric_name", header[0])
				assert.Equal(t, "timestamp", header[1])
				assert.Equal(t, "metric_value", header[2])
				assert.Equal(t, "extra_tags", header[len(header)-1])
			})
		}(testname, tags)
	}
}

func TestSampleToRow(t *testing.T) {
	testSamples := []stats.Sample{
		{
			Time:   time.Now(),
			Metric: stats.New("my_metric", stats.Gauge),
			Value:  1,
			Tags: stats.NewSampleTags(map[string]string{
				"tag1": "val1",
				"tag2": "val2",
				"tag3": "val3",
			}),
		},
		{
			Time:   time.Now(),
			Metric: stats.New("my_metric", stats.Gauge),
			Value:  1,
			Tags: stats.NewSampleTags(map[string]string{
				"tag1": "val1",
				"tag2": "val2",
				"tag3": "val3",
				"tag4": "val4",
				"tag5": "val5",
			}),
		},
	}

	enabledTags := map[string][][]string{
		"One tag": {
			{"tag1"},
			{"tag2"},
		},
		"Two tags": {
			{"tag1", "tag2"},
			{},
		},
		"Two tags, one ignored": {
			{"tag1", "tag2"},
			{"tag3"},
		},
	}

	for testname, tags := range enabledTags {
		for _, sample := range testSamples {
			func(testname string, sample stats.Sample, resTags []string, ignoredTags []string) {
				t.Run(testname, func(t *testing.T) {
					row := SampleToRow(&sample, resTags, ignoredTags)
					assert.Equal(t, len(resTags)+4, len(row))
					for _, tag := range ignoredTags {
						assert.False(t, strings.Contains(row[len(row)-1], tag))
					}
				})
			}(testname, sample, tags[0], tags[1])
		}
	}
}
func TestCollect(t *testing.T) {
	testSamples := []stats.SampleContainer{
		stats.Sample{
			Time:   time.Unix(1562324643, 0),
			Metric: stats.New("my_metric", stats.Gauge),
			Value:  1,
			Tags: stats.NewSampleTags(map[string]string{
				"tag1": "val1",
				"tag2": "val2",
				"tag3": "val3",
			}),
		},
		stats.Sample{
			Time:   time.Unix(1562324644, 0),
			Metric: stats.New("my_metric", stats.Gauge),
			Value:  1,
			Tags: stats.NewSampleTags(map[string]string{
				"tag1": "val1",
				"tag2": "val2",
				"tag3": "val3",
				"tag4": "val4",
			}),
		},
	}
	t.Run("Collect", func(t *testing.T) {
		mem := afero.NewMemMapFs()
		collector, err := New(mem, "path", lib.TagSet{"tag1": true, "tag2": false, "tag3": true})
		assert.NoError(t, err)
		assert.NotNil(t, collector)

		collector.Collect(testSamples)

		assert.Equal(t, len(testSamples), len(collector.buffer))
	})
}

func TestRun(t *testing.T) {
	t.Run("Run", func(t *testing.T) {
		collector, err := New(afero.NewMemMapFs(), "path", lib.TagSet{"tag1": true, "tag2": false, "tag3": true})
		assert.NoError(t, err)
		assert.NotNil(t, collector)

		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := collector.Init()
			assert.NoError(t, err)
			collector.Run(ctx)
		}()
		cancel()
		wg.Wait()
	})
}

func TestRunCollect(t *testing.T) {
	testSamples := []stats.SampleContainer{
		stats.Sample{
			Time:   time.Unix(1562324643, 0),
			Metric: stats.New("my_metric", stats.Gauge),
			Value:  1,
			Tags: stats.NewSampleTags(map[string]string{
				"tag1": "val1",
				"tag2": "val2",
				"tag3": "val3",
			}),
		},
		stats.Sample{
			Time:   time.Unix(1562324644, 0),
			Metric: stats.New("my_metric", stats.Gauge),
			Value:  1,
			Tags: stats.NewSampleTags(map[string]string{
				"tag1": "val1",
				"tag2": "val2",
				"tag3": "val3",
				"tag4": "val4",
			}),
		},
	}

	t.Run("Run and Collect", func(t *testing.T) {
		mem := afero.NewMemMapFs()
		collector, err := New(mem, "path", lib.TagSet{"tag1": true, "tag2": false, "tag3": true})
		assert.NoError(t, err)
		assert.NotNil(t, collector)

		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			collector.Run(ctx)
			wg.Done()
		}()
		err = collector.Init()
		assert.NoError(t, err)
		collector.Collect(testSamples)
		time.Sleep(1 * time.Second)
		cancel()
		wg.Wait()
		csvbytes, _ := afero.ReadFile(mem, "path")
		csvstr := fmt.Sprintf("%s", csvbytes)
		assert.Equal(t,
			"metric_name,timestamp,metric_value,tag1,tag3,extra_tags\n"+
				"my_metric,1562324643,1.000000,val1,val3,\n"+
				"my_metric,1562324644,1.000000,val1,val3,tag4=val4\n",
			csvstr)
	})
}
