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
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
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
	testMetric, err := metrics.NewRegistry().NewMetric("my_metric", metrics.Gauge)
	require.NoError(t, err)

	testData := []struct {
		testname    string
		sample      *metrics.Sample
		resTags     []string
		ignoredTags []string
	}{
		{
			testname: "One res tag, one ignored tag, one extra tag",
			sample: &metrics.Sample{
				Time:   time.Unix(1562324644, 0),
				Metric: testMetric,
				Value:  1,
				Tags: metrics.NewSampleTags(map[string]string{
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
			sample: &metrics.Sample{
				Time:   time.Unix(1562324644, 0),
				Metric: testMetric,
				Value:  1,
				Tags: metrics.NewSampleTags(map[string]string{
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
			sample: &metrics.Sample{
				Time:   time.Unix(1562324644, 0),
				Metric: testMetric,
				Value:  1,
				Tags: metrics.NewSampleTags(map[string]string{
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

func TestRun(t *testing.T) {
	t.Parallel()

	testMetric, err := metrics.NewRegistry().NewMetric("my_metric", metrics.Gauge)
	require.NoError(t, err)

	testData := []struct {
		samples        []metrics.SampleContainer
		fileName       string
		fileReaderFunc func(fileName string, fs afero.Fs) string
		outputContent  string
	}{
		{
			samples: []metrics.SampleContainer{
				metrics.Sample{
					Time:   time.Unix(1562324643, 0),
					Metric: testMetric,
					Value:  1,
					Tags: metrics.NewSampleTags(map[string]string{
						"check": "val1",
						"url":   "val2",
						"error": "val3",
					}),
				},
				metrics.Sample{
					Time:   time.Unix(1562324644, 0),
					Metric: testMetric,
					Value:  1,
					Tags: metrics.NewSampleTags(map[string]string{
						"check": "val1",
						"url":   "val2",
						"error": "val3",
						"tag4":  "val4",
					}),
				},
			},
			fileName:       "test",
			fileReaderFunc: readUnCompressedFile,
			outputContent:  "metric_name,timestamp,metric_value,check,error,extra_tags\n" + "my_metric,1562324643,1.000000,val1,val3,url=val2\n" + "my_metric,1562324644,1.000000,val1,val3,tag4=val4&url=val2\n",
		},
		{
			samples: []metrics.SampleContainer{
				metrics.Sample{
					Time:   time.Unix(1562324643, 0),
					Metric: testMetric,
					Value:  1,
					Tags: metrics.NewSampleTags(map[string]string{
						"check": "val1",
						"url":   "val2",
						"error": "val3",
					}),
				},
				metrics.Sample{
					Time:   time.Unix(1562324644, 0),
					Metric: testMetric,
					Value:  1,
					Tags: metrics.NewSampleTags(map[string]string{
						"check": "val1",
						"url":   "val2",
						"error": "val3",
						"name":  "val4",
					}),
				},
			},
			fileName:       "test.gz",
			fileReaderFunc: readCompressedFile,
			outputContent:  "metric_name,timestamp,metric_value,check,error,extra_tags\n" + "my_metric,1562324643,1.000000,val1,val3,url=val2\n" + "my_metric,1562324644,1.000000,val1,val3,name=val4&url=val2\n",
		},
	}

	for _, data := range testData {
		mem := afero.NewMemMapFs()
		output, err := newOutput(output.Params{
			Logger:         testutils.NewLogger(t),
			FS:             mem,
			ConfigArgument: data.fileName,
			ScriptOptions: lib.Options{
				SystemTags: metrics.NewSystemTagSet(metrics.TagError | metrics.TagCheck),
			},
		})
		require.NoError(t, err)
		require.NotNil(t, output)

		require.NoError(t, output.Start())
		output.AddMetricSamples(data.samples)
		time.Sleep(1 * time.Second)
		require.NoError(t, output.Stop())

		finalOutput := data.fileReaderFunc(data.fileName, mem)
		assert.Equal(t, data.outputContent, sortExtraTagsForTest(t, finalOutput))
	}
}

func sortExtraTagsForTest(t *testing.T, input string) string {
	t.Helper()
	r := csv.NewReader(strings.NewReader(input))
	lines, err := r.ReadAll()
	require.NoError(t, err)
	for i, line := range lines[1:] {
		extraTags := strings.Split(line[len(line)-1], "&")
		sort.Strings(extraTags)
		lines[i+1][len(line)-1] = strings.Join(extraTags, "&")
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	require.NoError(t, w.WriteAll(lines))
	w.Flush()
	return b.String()
}
