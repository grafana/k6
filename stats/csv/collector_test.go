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
	"compress/gzip"
	"context"
	"fmt"
	"io/ioutil"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
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
		testname, tags := testname, tags
		t.Run(testname, func(t *testing.T) {
			header := MakeHeader(tags)
			assert.Equal(t, len(tags)+4, len(header))
			assert.Equal(t, "metric_name", header[0])
			assert.Equal(t, "timestamp", header[1])
			assert.Equal(t, "metric_value", header[2])
			assert.Equal(t, "extra_tags", header[len(header)-1])
		})
	}
}

func TestSampleToRow(t *testing.T) {
	testData := []struct {
		testname    string
		sample      *stats.Sample
		resTags     []string
		ignoredTags []string
	}{
		{
			testname: "One res tag, one ignored tag, one extra tag",
			sample: &stats.Sample{
				Time:   time.Unix(1562324644, 0),
				Metric: stats.New("my_metric", stats.Gauge),
				Value:  1,
				Tags: stats.NewSampleTags(map[string]string{
					"tag1": "val1",
					"tag2": "val2",
					"tag3": "val3",
				}),
			},
			resTags:     []string{"tag1"},
			ignoredTags: []string{"tag2"},
		},
		{
			testname: "Two res tags, three extra tags",
			sample: &stats.Sample{
				Time:   time.Unix(1562324644, 0),
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
			resTags:     []string{"tag1", "tag2"},
			ignoredTags: []string{},
		},
		{
			testname: "Two res tags, two ignored",
			sample: &stats.Sample{
				Time:   time.Unix(1562324644, 0),
				Metric: stats.New("my_metric", stats.Gauge),
				Value:  1,
				Tags: stats.NewSampleTags(map[string]string{
					"tag1": "val1",
					"tag2": "val2",
					"tag3": "val3",
					"tag4": "val4",
					"tag5": "val5",
					"tag6": "val6",
				}),
			},
			resTags:     []string{"tag1", "tag3"},
			ignoredTags: []string{"tag4", "tag6"},
		},
	}

	expected := []struct {
		baseRow  []string
		extraRow []string
	}{
		{
			baseRow: []string{
				"my_metric",
				"1562324644",
				"1.000000",
				"val1",
			},
			extraRow: []string{
				"tag3=val3",
			},
		},
		{
			baseRow: []string{
				"my_metric",
				"1562324644",
				"1.000000",
				"val1",
				"val2",
			},
			extraRow: []string{
				"tag3=val3",
				"tag4=val4",
				"tag5=val5",
			},
		},
		{
			baseRow: []string{
				"my_metric",
				"1562324644",
				"1.000000",
				"val1",
				"val3",
			},
			extraRow: []string{
				"tag2=val2",
				"tag5=val5",
			},
		},
	}

	for i := range testData {
		testname, sample := testData[i].testname, testData[i].sample
		resTags, ignoredTags := testData[i].resTags, testData[i].ignoredTags
		expectedRow := expected[i]

		t.Run(testname, func(t *testing.T) {
			row := SampleToRow(sample, resTags, ignoredTags, make([]string, 3+len(resTags)+1))
			for ind, cell := range expectedRow.baseRow {
				assert.Equal(t, cell, row[ind])
			}
			for _, cell := range expectedRow.extraRow {
				assert.Contains(t, row[len(row)-1], cell)
			}
		})
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

	mem := afero.NewMemMapFs()
	collector, err := New(
		testutils.NewLogger(t),
		mem,
		stats.TagSet{"tag1": true, "tag2": false, "tag3": true},
		Config{FileName: null.StringFrom("name"), SaveInterval: types.NewNullDuration(time.Duration(1), true)},
	)
	assert.NoError(t, err)
	assert.NotNil(t, collector)

	collector.Collect(testSamples)

	assert.Equal(t, len(testSamples), len(collector.buffer))
}

func TestRun(t *testing.T) {
	collector, err := New(
		testutils.NewLogger(t),
		afero.NewMemMapFs(),
		stats.TagSet{"tag1": true, "tag2": false, "tag3": true},
		Config{FileName: null.StringFrom("name"), SaveInterval: types.NewNullDuration(time.Duration(1), true)},
	)
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
}

func readUnCompressedFile(fileName string, fs afero.Fs) string {
	csvbytes, err := afero.ReadFile(fs, fileName)
	if err != nil {
		return err.Error()
	}

	return fmt.Sprintf("%s", csvbytes)
}

func readCompressedFile(fileName string, fs afero.Fs) string {
	file, err := fs.Open(fileName)
	if err != nil {
		return err.Error()
	}

	gzf, err := gzip.NewReader(file)
	if err != nil {
		return err.Error()
	}

	csvbytes, err := ioutil.ReadAll(gzf)
	if err != nil {
		return err.Error()
	}

	return fmt.Sprintf("%s", csvbytes)
}

func TestRunCollect(t *testing.T) {
	testData := []struct {
		samples        []stats.SampleContainer
		fileName       string
		fileReaderFunc func(fileName string, fs afero.Fs) string
		outputContent  string
	}{
		{
			samples: []stats.SampleContainer{
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
			},
			fileName:       "test",
			fileReaderFunc: readUnCompressedFile,
			outputContent:  "metric_name,timestamp,metric_value,tag1,tag3,extra_tags\n" + "my_metric,1562324643,1.000000,val1,val3,\n" + "my_metric,1562324644,1.000000,val1,val3,tag4=val4\n",
		},
		{
			samples: []stats.SampleContainer{
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
			},
			fileName:       "test.gz",
			fileReaderFunc: readCompressedFile,
			outputContent:  "metric_name,timestamp,metric_value,tag1,tag3,extra_tags\n" + "my_metric,1562324643,1.000000,val1,val3,\n" + "my_metric,1562324644,1.000000,val1,val3,tag4=val4\n",
		},
	}

	for _, data := range testData {
		mem := afero.NewMemMapFs()
		collector, err := New(
			testutils.NewLogger(t),
			mem,
			stats.TagSet{"tag1": true, "tag2": false, "tag3": true},
			Config{FileName: null.StringFrom(data.fileName), SaveInterval: types.NewNullDuration(time.Duration(1), true)},
		)
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
		collector.Collect(data.samples)
		time.Sleep(1 * time.Second)
		cancel()
		wg.Wait()

		assert.Equal(t, data.outputContent, data.fileReaderFunc(data.fileName, mem))
	}
}

func TestNew(t *testing.T) {
	configs := []struct {
		cfg  Config
		tags stats.TagSet
	}{
		{
			cfg: Config{FileName: null.StringFrom("name"), SaveInterval: types.NewNullDuration(time.Duration(1), true)},
			tags: stats.TagSet{
				"tag1": true,
				"tag2": false,
				"tag3": true,
			},
		},
		{
			cfg: Config{FileName: null.StringFrom("name.csv.gz"), SaveInterval: types.NewNullDuration(time.Duration(1), true)},
			tags: stats.TagSet{
				"tag1": true,
				"tag2": false,
				"tag3": true,
			},
		},
		{
			cfg: Config{FileName: null.StringFrom("-"), SaveInterval: types.NewNullDuration(time.Duration(1), true)},
			tags: stats.TagSet{
				"tag1": true,
			},
		},
		{
			cfg: Config{FileName: null.StringFrom(""), SaveInterval: types.NewNullDuration(time.Duration(1), true)},
			tags: stats.TagSet{
				"tag1": false,
				"tag2": false,
			},
		},
	}
	expected := []struct {
		fname       string
		resTags     []string
		ignoredTags []string
		closeFn     func() error
	}{
		{
			fname: "name",
			resTags: []string{
				"tag1", "tag3",
			},
			ignoredTags: []string{
				"tag2",
			},
		},
		{
			fname: "name.csv.gz",
			resTags: []string{
				"tag1", "tag3",
			},
			ignoredTags: []string{
				"tag2",
			},
		},
		{
			fname: "-",
			resTags: []string{
				"tag1",
			},
			ignoredTags: []string{},
		},
		{
			fname:   "-",
			resTags: []string{},
			ignoredTags: []string{
				"tag1", "tag2",
			},
		},
	}

	for i := range configs {
		config, expected := configs[i], expected[i]
		t.Run(config.cfg.FileName.String, func(t *testing.T) {
			collector, err := New(testutils.NewLogger(t), afero.NewMemMapFs(), config.tags, config.cfg)
			assert.NoError(t, err)
			assert.NotNil(t, collector)
			assert.Equal(t, expected.fname, collector.fname)
			sort.Strings(expected.resTags)
			sort.Strings(collector.resTags)
			assert.Equal(t, expected.resTags, collector.resTags)
			sort.Strings(expected.ignoredTags)
			sort.Strings(collector.ignoredTags)
			assert.Equal(t, expected.ignoredTags, collector.ignoredTags)
			assert.NoError(t, collector.closeFn())
		})
	}
}

func TestGetRequiredSystemTags(t *testing.T) {
	collector, err := New(
		testutils.NewLogger(t),
		afero.NewMemMapFs(),
		stats.TagSet{"tag1": true, "tag2": false, "tag3": true},
		Config{FileName: null.StringFrom("name"), SaveInterval: types.NewNullDuration(time.Duration(1), true)},
	)
	assert.NoError(t, err)
	assert.NotNil(t, collector)
	assert.Equal(t, stats.SystemTagSet(0), collector.GetRequiredSystemTags())
}

func TestLink(t *testing.T) {
	collector, err := New(
		testutils.NewLogger(t),
		afero.NewMemMapFs(),
		stats.TagSet{"tag1": true, "tag2": false, "tag3": true},
		Config{FileName: null.StringFrom("path"), SaveInterval: types.NewNullDuration(time.Duration(1), true)},
	)
	assert.NoError(t, err)
	assert.NotNil(t, collector)
	assert.Equal(t, "path", collector.Link())
}
