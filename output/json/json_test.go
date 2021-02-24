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

package json

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/output"
	"github.com/loadimpact/k6/stats"
)

func getValidator(t *testing.T, expected []string) func(io.Reader) {
	return func(rawJSONLines io.Reader) {
		s := bufio.NewScanner(rawJSONLines)
		i := 0
		for s.Scan() {
			i++
			if i > len(expected) {
				t.Errorf("Read unexpected line number %d, expected only %d entries", i, len(expected))
				continue
			}
			assert.JSONEq(t, expected[i-1], string(s.Bytes()))
		}
		assert.NoError(t, s.Err())
		assert.Equal(t, len(expected), i)
	}
}

func generateTestMetricSamples(t *testing.T) ([]stats.SampleContainer, func(io.Reader)) {
	metric1 := stats.New("my_metric1", stats.Gauge)
	metric2 := stats.New("my_metric2", stats.Counter, stats.Data)
	time1 := time.Date(2021, time.February, 24, 13, 37, 10, 0, time.UTC)
	time2 := time1.Add(10 * time.Second)
	time3 := time2.Add(10 * time.Second)
	connTags := stats.NewSampleTags(map[string]string{"key": "val"})

	samples := []stats.SampleContainer{
		stats.Sample{Time: time1, Metric: metric1, Value: float64(1), Tags: stats.NewSampleTags(map[string]string{"tag1": "val1"})},
		stats.Sample{Time: time1, Metric: metric1, Value: float64(2), Tags: stats.NewSampleTags(map[string]string{"tag2": "val2"})},
		stats.ConnectedSamples{Samples: []stats.Sample{
			{Time: time2, Metric: metric2, Value: float64(3), Tags: connTags},
			{Time: time2, Metric: metric1, Value: float64(4), Tags: connTags},
		}, Time: time2, Tags: connTags},
		stats.Sample{Time: time3, Metric: metric2, Value: float64(5), Tags: stats.NewSampleTags(map[string]string{"tag3": "val3"})},
	}
	// TODO: fix JSON thresholds (https://github.com/loadimpact/k6/issues/1052)
	expected := []string{
		`{"type":"Metric","data":{"name":"my_metric1","type":"gauge","contains":"default","tainted":null,"thresholds":[],"submetrics":null,"sub":{"name":"","parent":"","suffix":"","tags":null}},"metric":"my_metric1"}`,
		`{"type":"Point","data":{"time":"2021-02-24T13:37:10Z","value":1,"tags":{"tag1":"val1"}},"metric":"my_metric1"}`,
		`{"type":"Point","data":{"time":"2021-02-24T13:37:10Z","value":2,"tags":{"tag2":"val2"}},"metric":"my_metric1"}`,
		`{"type":"Metric","data":{"name":"my_metric2","type":"counter","contains":"data","tainted":null,"thresholds":[],"submetrics":null,"sub":{"name":"","parent":"","suffix":"","tags":null}},"metric":"my_metric2"}`,
		`{"type":"Point","data":{"time":"2021-02-24T13:37:20Z","value":3,"tags":{"key":"val"}},"metric":"my_metric2"}`,
		`{"type":"Point","data":{"time":"2021-02-24T13:37:20Z","value":4,"tags":{"key":"val"}},"metric":"my_metric1"}`,
		`{"type":"Point","data":{"time":"2021-02-24T13:37:30Z","value":5,"tags":{"tag3":"val3"}},"metric":"my_metric2"}`,
	}

	return samples, getValidator(t, expected)
}

func TestJsonOutputStdout(t *testing.T) {
	t.Parallel()

	stdout := new(bytes.Buffer)
	out, err := New(output.Params{
		Logger: testutils.NewLogger(t),
		StdOut: stdout,
	})
	require.NoError(t, err)
	require.NoError(t, out.Start())

	samples, validateResults := generateTestMetricSamples(t)
	out.AddMetricSamples(samples[:2])
	out.AddMetricSamples(samples[2:])
	require.NoError(t, out.Stop())
	validateResults(stdout)
}

func TestJsonOutputFileError(t *testing.T) {
	t.Parallel()

	stdout := new(bytes.Buffer)
	fs := afero.NewReadOnlyFs(afero.NewMemMapFs())
	out, err := New(output.Params{
		Logger:         testutils.NewLogger(t),
		StdOut:         stdout,
		FS:             fs,
		ConfigArgument: "/json-output",
	})
	require.NoError(t, err)
	assert.Error(t, out.Start())
}

func TestJsonOutputFile(t *testing.T) {
	t.Parallel()

	stdout := new(bytes.Buffer)
	fs := afero.NewMemMapFs()
	out, err := New(output.Params{
		Logger:         testutils.NewLogger(t),
		StdOut:         stdout,
		FS:             fs,
		ConfigArgument: "/json-output",
	})
	require.NoError(t, err)
	require.NoError(t, out.Start())

	samples, validateResults := generateTestMetricSamples(t)
	out.AddMetricSamples(samples[:2])
	out.AddMetricSamples(samples[2:])
	require.NoError(t, out.Stop())

	assert.Empty(t, stdout.Bytes())
	file, err := fs.Open("/json-output")
	require.NoError(t, err)
	validateResults(file)
	assert.NoError(t, file.Close())
}

func TestJsonOutputFileGzipped(t *testing.T) {
	t.Parallel()

	stdout := new(bytes.Buffer)
	fs := afero.NewMemMapFs()
	out, err := New(output.Params{
		Logger:         testutils.NewLogger(t),
		StdOut:         stdout,
		FS:             fs,
		ConfigArgument: "/json-output.gz",
	})
	require.NoError(t, err)
	require.NoError(t, out.Start())

	samples, validateResults := generateTestMetricSamples(t)
	out.AddMetricSamples(samples[:2])
	out.AddMetricSamples(samples[2:])
	require.NoError(t, out.Stop())

	assert.Empty(t, stdout.Bytes())
	file, err := fs.Open("/json-output.gz")
	require.NoError(t, err)
	reader, err := gzip.NewReader(file)
	require.NoError(t, err)
	validateResults(reader)
	assert.NoError(t, file.Close())
}

func TestWrapersWithNilArg(t *testing.T) {
	t.Parallel()
	out := WrapSample(nil)
	assert.Equal(t, out, (*Envelope)(nil))
	out = wrapMetric(nil)
	assert.Equal(t, out, (*Envelope)(nil))
}

func TestWrapSampleWithSamplePointer(t *testing.T) {
	t.Parallel()
	out := WrapSample(&stats.Sample{
		Metric: &stats.Metric{},
	})
	assert.NotEqual(t, out, (*Envelope)(nil))
}

func TestWrapMetricWithMetricPointer(t *testing.T) {
	t.Parallel()
	out := wrapMetric(&stats.Metric{})
	assert.NotEqual(t, out, (*Envelope)(nil))
}
