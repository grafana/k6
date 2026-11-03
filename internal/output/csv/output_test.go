package csv

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/fsext"
	"go.k6.io/k6/v2/metrics"
	"go.k6.io/k6/v2/output"
)

func TestMakeHeader(t *testing.T) {
	t.Parallel()

	testdata := map[string][]string{
		"One tag": {
			"tag1",
		},
		"Two tags": {
			"tag1", "tag2",
		},
	}

	for testname, tags := range testdata {
		t.Run(testname, func(t *testing.T) {
			t.Parallel()

			header := MakeHeader(tags)
			assert.Equal(t, len(tags)+5, len(header))
			assert.Equal(t, "metric_name", header[0])
			assert.Equal(t, "timestamp", header[1])
			assert.Equal(t, "metric_value", header[2])
			assert.Equal(t, "extra_tags", header[len(header)-2])
			assert.Equal(t, "metadata", header[len(header)-1])
		})
	}
}

func TestSampleToRow(t *testing.T) {
	t.Parallel()

	registry := metrics.NewRegistry()
	testMetric, err := registry.NewMetric("my_metric", metrics.Gauge)
	require.NoError(t, err)

	testData := []struct {
		testname    string
		sample      *metrics.Sample
		resTags     []string
		ignoredTags []string
		timeFormat  string
	}{
		{
			testname: "One res tag, one ignored tag, one extra tag",
			sample: &metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: testMetric,
					Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
						"tag1": "val1",
						"tag2": "val2",
						"tag3": "val3",
					}),
				},
				Time:  time.Unix(1562324644, 0),
				Value: 1,
			},
			resTags:     []string{"tag1"},
			ignoredTags: []string{"tag2"},
			timeFormat:  "unix",
		},
		{
			testname: "Two res tags, three extra tags",
			sample: &metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: testMetric,
					Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
						"tag1": "val1",
						"tag2": "val2",
						"tag3": "val3",
						"tag4": "val4",
						"tag5": "val5",
					}),
				},
				Time:  time.Unix(1562324644, 0),
				Value: 1,
			},
			resTags:     []string{"tag1", "tag2"},
			ignoredTags: []string{},
			timeFormat:  "unix",
		},
		{
			testname: "Two res tags, two ignored, with RFC3339 timestamp",
			sample: &metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: testMetric,
					Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
						"tag1": "val1",
						"tag2": "val2",
						"tag3": "val3",
						"tag4": "val4",
						"tag5": "val5",
						"tag6": "val6",
					}),
				},
				Time:  time.Unix(1562324644, 0),
				Value: 1,
			},
			resTags:     []string{"tag1", "tag3"},
			ignoredTags: []string{"tag4", "tag6"},
			timeFormat:  "rfc3339",
		},
		{
			testname: "Two res tags, two ignored, with Unix Nano timestamp",
			sample: &metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: testMetric,
					Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
						"tag1": "val1",
						"tag2": "val2",
						"tag3": "val3",
						"tag4": "val4",
						"tag5": "val5",
						"tag6": "val6",
					}),
				},
				Time:  time.Unix(1562324644, 25),
				Value: 1,
			},
			resTags:     []string{"tag1", "tag3"},
			ignoredTags: []string{"tag4", "tag6"},
			timeFormat:  "unix_nano",
		},
		{
			testname: "Two res tags, two ignored, with Unix Micro timestamp",
			sample: &metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: testMetric,
					Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
						"tag1": "val1",
						"tag2": "val2",
						"tag3": "val3",
						"tag4": "val4",
						"tag5": "val5",
						"tag6": "val6",
					}),
				},
				Time:  time.Unix(1562324644, 2125),
				Value: 1,
			},
			resTags:     []string{"tag1", "tag3"},
			ignoredTags: []string{"tag4", "tag6"},
			timeFormat:  "unix_micro",
		},
		{
			testname: "Two res tags, two ignored, with Unix Milli timestamp",
			sample: &metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: testMetric,
					Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
						"tag1": "val1",
						"tag2": "val2",
						"tag3": "val3",
						"tag4": "val4",
						"tag5": "val5",
						"tag6": "val6",
					}),
				},
				Time:  time.Unix(1562324644, 6482125),
				Value: 1,
			},
			resTags:     []string{"tag1", "tag3"},
			ignoredTags: []string{"tag4", "tag6"},
			timeFormat:  "unix_milli",
		},
		{
			testname: "Two res tags, two ignored, with RFC3339Nano timestamp",
			sample: &metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: testMetric,
					Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
						"tag1": "val1",
						"tag2": "val2",
						"tag3": "val3",
						"tag4": "val4",
						"tag5": "val5",
						"tag6": "val6",
					}),
				},
				Time:  time.Unix(1562324644, 0),
				Value: 1,
			},
			resTags:     []string{"tag1", "tag3"},
			ignoredTags: []string{"tag4", "tag6"},
			timeFormat:  "rfc3339_nano",
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
				time.Unix(1562324644, 0).Format(time.RFC3339),
				"1.000000",
				"val1",
				"val3",
			},
			extraRow: []string{
				"tag2=val2",
				"tag5=val5",
			},
		},
		{
			baseRow: []string{
				"my_metric",
				"1562324644000000025",
				"1.000000",
				"val1",
				"val3",
			},
			extraRow: []string{
				"tag2=val2",
				"tag5=val5",
			},
		},
		{
			baseRow: []string{
				"my_metric",
				"1562324644000002",
				"1.000000",
				"val1",
				"val3",
			},
			extraRow: []string{
				"tag2=val2",
				"tag5=val5",
			},
		},
		{
			baseRow: []string{
				"my_metric",
				"1562324644006",
				"1.000000",
				"val1",
				"val3",
			},
			extraRow: []string{
				"tag2=val2",
				"tag5=val5",
			},
		},
		{
			baseRow: []string{
				"my_metric",
				time.Unix(1562324644, 0).Format(time.RFC3339Nano),
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
		timeFormat, err := TimeFormatString(testData[i].timeFormat)
		require.NoError(t, err)
		expectedRow := expected[i]

		t.Run(testname, func(t *testing.T) {
			t.Parallel()

			row := SampleToRow(sample, resTags, ignoredTags, make([]string, 3+len(resTags)+2), timeFormat)
			for ind, cell := range expectedRow.baseRow {
				assert.Equal(t, cell, row[ind])
			}
			for _, cell := range expectedRow.extraRow {
				assert.Contains(t, row[len(row)-2], cell)
			}
		})
	}
}

func readUnCompressedFile(fileName string, fs fsext.Fs) string {
	csvbytes, err := fsext.ReadFile(fs, fileName)
	if err != nil {
		return err.Error()
	}

	return string(csvbytes)
}

func readCompressedFile(fileName string, fs fsext.Fs) string {
	file, err := fs.Open(fileName)
	if err != nil {
		return err.Error()
	}

	gzf, err := gzip.NewReader(file)
	if err != nil {
		return err.Error()
	}

	csvbytes, err := io.ReadAll(gzf)
	if err != nil {
		return err.Error()
	}

	return string(csvbytes)
}

func TestRun(t *testing.T) {
	t.Parallel()

	registry := metrics.NewRegistry()
	testMetric, err := registry.NewMetric("my_metric", metrics.Gauge)
	require.NoError(t, err)

	testData := []struct {
		samples        []metrics.SampleContainer
		fileName       string
		fileReaderFunc func(fileName string, fs fsext.Fs) string
		timeFormat     string
		outputContent  string
	}{
		{
			samples: []metrics.SampleContainer{
				metrics.Sample{
					TimeSeries: metrics.TimeSeries{
						Metric: testMetric,
						Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
							"check": "val1",
							"url":   "val2",
							"error": "val3",
						}),
					},
					Time:  time.Unix(1562324643, 0),
					Value: 1,
				},
				metrics.Sample{
					TimeSeries: metrics.TimeSeries{
						Metric: testMetric,
						Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
							"check": "val1",
							"url":   "val2",
							"error": "val3",
							"tag4":  "val4",
							"vu":    "1",
						}),
					},
					Metadata: map[string]string{},
					Time:     time.Unix(1562324644, 0),
					Value:    1,
				},
			},
			fileName:       "test",
			fileReaderFunc: readUnCompressedFile,
			timeFormat:     "",
			outputContent: "metric_name,timestamp,metric_value,check,error,extra_tags,metadata\n" +
				"my_metric,1562324643,1.000000,val1,val3,url=val2,\n" +
				"my_metric,1562324644,1.000000,val1,val3,tag4=val4&url=val2&vu=1,\n",
		},
		{
			samples: []metrics.SampleContainer{
				metrics.Sample{
					TimeSeries: metrics.TimeSeries{
						Metric: testMetric,
						Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
							"check": "val1",
							"url":   "val2",
							"error": "val3",
						}),
					},
					Time:     time.Unix(1562324643, 0),
					Metadata: map[string]string{"y": "1"},
					Value:    1,
				},
				metrics.Sample{
					TimeSeries: metrics.TimeSeries{
						Metric: testMetric,
						Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
							"check": "val1",
							"url":   "val2",
							"error": "val3",
							"name":  "val4",
						}),
					},
					Time:  time.Unix(1562324644, 0),
					Value: 1,
				},
			},
			fileName:       "test.gz",
			fileReaderFunc: readCompressedFile,
			timeFormat:     "unix",
			outputContent: "metric_name,timestamp,metric_value,check,error,extra_tags,metadata\n" +
				"my_metric,1562324643,1.000000,val1,val3,url=val2,y=1\n" +
				"my_metric,1562324644,1.000000,val1,val3,name=val4&url=val2,\n",
		},
		{
			samples: []metrics.SampleContainer{
				metrics.Sample{
					TimeSeries: metrics.TimeSeries{
						Metric: testMetric,
						Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
							"check": "val1",
							"url":   "val2",
							"error": "val3",
						}),
					},
					Time:  time.Unix(1562324644, 0),
					Value: 1,
				},
				metrics.Sample{
					TimeSeries: metrics.TimeSeries{
						Metric: testMetric,
						Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
							"check": "val1",
							"url":   "val2",
							"error": "val3",
							"name":  "val4",
						}),
					},
					Time:     time.Unix(1562324644, 0),
					Metadata: map[string]string{"y": "2", "z": "3"},
					Value:    1,
				},
			},
			fileName:       "test",
			fileReaderFunc: readUnCompressedFile,
			timeFormat:     "rfc3339",
			outputContent: "metric_name,timestamp,metric_value,check,error,extra_tags,metadata\n" +
				"my_metric," + time.Unix(1562324644, 0).Format(time.RFC3339) + ",1.000000,val1,val3,url=val2,\n" +
				"my_metric," + time.Unix(1562324644, 0).Format(time.RFC3339) + ",1.000000,val1,val3,name=val4&url=val2,y=2&z=3\n",
		},
	}

	for i, data := range testData {
		name := fmt.Sprint(i)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mem := fsext.NewMemMapFs()
			env := make(map[string]string)
			if data.timeFormat != "" {
				env["K6_CSV_TIME_FORMAT"] = data.timeFormat
			}

			output, err := newOutput(output.Params{
				Logger:         testutils.NewLogger(t),
				FS:             mem,
				Environment:    env,
				ConfigArgument: data.fileName,
				ScriptOptions: lib.Options{
					SystemTags: metrics.NewSystemTagSet(metrics.TagError | metrics.TagCheck | metrics.TagVU),
				},
			})
			require.NoError(t, err)
			require.NotNil(t, output)

			require.NoError(t, output.Start())
			output.AddMetricSamples(data.samples)
			require.NoError(t, output.Stop())

			finalOutput := data.fileReaderFunc(data.fileName, mem)
			assert.Equal(t, data.outputContent, sortExtraTagsForTest(t, finalOutput))
		})
	}
}

func sortExtraTagsForTest(t *testing.T, input string) string {
	t.Helper()
	r := csv.NewReader(strings.NewReader(input))
	lines, err := r.ReadAll()
	require.NoError(t, err)
	for i, line := range lines[1:] {
		extraTags := strings.Split(line[len(line)-2], "&")
		sort.Strings(extraTags)
		lines[i+1][len(line)-2] = strings.Join(extraTags, "&")
		extraMetadata := strings.Split(line[len(line)-1], "&")
		sort.Strings(extraMetadata)
		lines[i+1][len(line)-1] = strings.Join(extraMetadata, "&")
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	require.NoError(t, w.WriteAll(lines))
	w.Flush()
	return b.String()
}

// gzipErrorFS injects failures in writes and file closure without filling a real disk.
type gzipErrorFS struct {
	fsext.Fs
	file *gzipErrorFile
}

func (fs gzipErrorFS) Create(name string) (afero.File, error) {
	file, err := fs.Fs.Create(name)
	if err != nil {
		return nil, err
	}
	fs.file.File = file
	return fs.file, nil
}

type gzipErrorFile struct {
	afero.File
	writeErr error
	closeErr error
	closed   bool
}

func (f *gzipErrorFile) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return f.File.Write(p)
}

func (f *gzipErrorFile) Close() error {
	f.closed = true
	return errors.Join(f.File.Close(), f.closeErr)
}

func TestCSVOutputGzipStopReturnsErrors(t *testing.T) {
	t.Parallel()

	errWrite := errors.New("disk full")
	errClose := errors.New("file close failed")
	for _, tc := range []struct {
		name     string
		writeErr error
		closeErr error
	}{
		{name: "write", writeErr: errWrite},
		{name: "close", closeErr: errClose},
		{name: "write and close", writeErr: errWrite, closeErr: errClose},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			file := &gzipErrorFile{closeErr: tc.closeErr}
			out, err := New(output.Params{
				Logger:         testutils.NewLogger(t),
				FS:             gzipErrorFS{Fs: fsext.NewMemMapFs(), file: file},
				ConfigArgument: "results.gz",
				ScriptOptions:  lib.Options{SystemTags: metrics.NewSystemTagSet(metrics.TagCheck)},
			})
			require.NoError(t, err)
			require.NoError(t, out.Start())

			// Writes succeed while starting the output, then fail during finalization.
			file.writeErr = tc.writeErr
			err = out.Stop()
			if tc.writeErr != nil {
				assert.ErrorIs(t, err, tc.writeErr)
			}
			if tc.closeErr != nil {
				assert.ErrorIs(t, err, tc.closeErr)
			}
			assert.True(t, file.closed, "the file must be closed even when finalizing gzip fails")
		})
	}
}
