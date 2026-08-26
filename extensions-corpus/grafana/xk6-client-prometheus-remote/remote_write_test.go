package remotewrite

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
)

func TestEvaluateTemplate(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		template      string
		value         int
		result        string
		expectedError string
	}{
		{template: "something ${series_id} else", value: 12, result: "something 12 else"},
		{template: "something ${series_id else", expectedError: "unsupported template"},
		{template: "something ${series_id/6} else", value: 12, result: "something 2 else"},
		{template: "something ${series_id/6 else", expectedError: "closing bracket"},
		{template: "something ${series_id%6} else", value: 12, result: "something 0 else"},
		{template: "something ${series_id%6 else", expectedError: "closing bracket"},
		{template: "something ${series_id*6} else", expectedError: "unsupported template"},
		{template: "something else", result: "something else"},
	}
	for _, testcase := range testcases {
		t.Run(fmt.Sprintf("template=%q,value=%d", testcase.template, testcase.value), func(t *testing.T) {
			t.Parallel()

			compiled, err := compileTemplate(testcase.template)
			if testcase.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), testcase.expectedError)

				return
			}

			require.NoError(t, err)

			result := string(compiled.AppendByte(nil, testcase.value))
			require.Equal(t, testcase.result, result)
		})
	}
}

func TestGenerateFromTemplates(t *testing.T) {
	t.Parallel()

	type args struct {
		minValue       int
		maxValue       int
		timestamp      int64
		minSeriesID    int
		maxSeriesID    int
		labelsTemplate map[string]string
	}

	type want struct {
		valueMin float64
		valueMax float64
		series   []prompb.TimeSeries
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "11th batch of 5",
			args: args{
				minValue:    123,
				maxValue:    133,
				timestamp:   123456789,
				minSeriesID: 50,
				maxSeriesID: 55,
				labelsTemplate: map[string]string{
					"__name__":        "k6_generated_metric_${series_id}",
					"series_id":       "${series_id}",
					"cardinality_1e1": "${series_id/10}",
					"cardinality_1e3": "${series_id/1000}",
					"cardinality_2":   "${series_id%2}",
					"cardinality_10":  "${series_id%10}",
				},
			},
			want: want{
				valueMin: 123,
				valueMax: 133,
				series: []prompb.TimeSeries{
					{
						Labels: []prompb.Label{
							{Name: "__name__", Value: "k6_generated_metric_50"},
							{Name: "cardinality_10", Value: "0"},
							{Name: "cardinality_1e1", Value: "5"},
							{Name: "cardinality_1e3", Value: "0"},
							{Name: "cardinality_2", Value: "0"},
							{Name: "series_id", Value: "50"},
						},
						Samples: []prompb.Sample{{Timestamp: 123456789}},
					}, {
						Labels: []prompb.Label{
							{Name: "__name__", Value: "k6_generated_metric_51"},
							{Name: "cardinality_10", Value: "1"},
							{Name: "cardinality_1e1", Value: "5"},
							{Name: "cardinality_1e3", Value: "0"},
							{Name: "cardinality_2", Value: "1"},
							{Name: "series_id", Value: "51"},
						},
						Samples: []prompb.Sample{{Timestamp: 123456789}},
					}, {
						Labels: []prompb.Label{
							{Name: "__name__", Value: "k6_generated_metric_52"},
							{Name: "cardinality_10", Value: "2"},
							{Name: "cardinality_1e1", Value: "5"},
							{Name: "cardinality_1e3", Value: "0"},
							{Name: "cardinality_2", Value: "0"},
							{Name: "series_id", Value: "52"},
						},
						Samples: []prompb.Sample{{Timestamp: 123456789}},
					}, {
						Labels: []prompb.Label{
							{Name: "__name__", Value: "k6_generated_metric_53"},
							{Name: "cardinality_10", Value: "3"},
							{Name: "cardinality_1e1", Value: "5"},
							{Name: "cardinality_1e3", Value: "0"},
							{Name: "cardinality_2", Value: "1"},
							{Name: "series_id", Value: "53"},
						},
						Samples: []prompb.Sample{{Timestamp: 123456789}},
					}, {
						Labels: []prompb.Label{
							{Name: "__name__", Value: "k6_generated_metric_54"},
							{Name: "cardinality_10", Value: "4"},
							{Name: "cardinality_1e1", Value: "5"},
							{Name: "cardinality_1e3", Value: "0"},
							{Name: "cardinality_2", Value: "0"},
							{Name: "series_id", Value: "54"},
						},
						Samples: []prompb.Sample{{Timestamp: 123456789}},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// #nosec G404 -- Using math/rand in test code, cryptographic randomness not required
			r := rand.New(rand.NewSource(time.Now().Unix()))
			compiled, err := compileLabelTemplates(tt.args.labelsTemplate)
			require.NoError(t, err)

			buf, err := generateFromPrecompiledTemplates(
				r, tt.args.minValue, tt.args.maxValue, tt.args.timestamp,
				tt.args.minSeriesID, tt.args.maxSeriesID, compiled,
			)
			require.NoError(t, err)

			req := new(prompb.WriteRequest)

			require.NoError(t, proto.Unmarshal(buf.Bytes(), protoadapt.MessageV2Of(req)))
			got := req.Timeseries

			require.NoError(t, err)

			if len(got) != len(tt.want.series) {
				t.Errorf("Differing length, want: %d, got: %d", len(tt.want.series), len(got))
			}

			for seriesID := range got {
				if !reflect.DeepEqual(got[seriesID].Labels, tt.want.series[seriesID].Labels) {
					t.Errorf(
						"Unexpected labels in series %d, want: %v, got: %v",
						seriesID, tt.want.series[seriesID].Labels, got[seriesID].Labels,
					)
				}

				if got[seriesID].Samples[0].Timestamp != tt.want.series[seriesID].Samples[0].Timestamp {
					t.Errorf(
						"Unexpected timestamp in series %d, want: %d, got: %d",
						seriesID,
						tt.want.series[seriesID].Samples[0].Timestamp,
						got[seriesID].Samples[0].Timestamp,
					)
				}

				if got[seriesID].Samples[0].Value < tt.want.valueMin || got[seriesID].Samples[0].Value > tt.want.valueMax {
					t.Errorf(
						"Unexpected value in series %d, want: %f-%f, got: %f",
						seriesID, tt.want.valueMin, tt.want.valueMax, got[seriesID].Samples[0].Value,
					)
				}
			}
		})
	}
}

// this test that the prompb stream marshalling implementation produces the same result as the upstream one.
func TestStreamEncoding(t *testing.T) {
	t.Parallel()

	seed := time.Now().Unix()
	t.Logf("seed=%d", seed)
	// #nosec G404 -- Using math/rand in test code with deterministic seed for reproducible tests
	r := rand.New(rand.NewSource(seed))
	timestamp := int64(valueBetween(r, 10, 100)) // timestamp
	// #nosec G404 -- Using math/rand in test code with deterministic seed for reproducible tests
	r = rand.New(rand.NewSource(seed)) // reset
	minValue := 10
	maxValue := 100000
	// this is the upstream encoding. It is purposefully this "handwritten"
	d, _ := proto.Marshal(protoadapt.MessageV2Of(&prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Samples: []prompb.Sample{{
					Value:     valueBetween(r, minValue, maxValue),
					Timestamp: (timestamp),
				}},
				Labels: []prompb.Label{
					{Name: "fifth", Value: "some 7 thing"},
					{Name: "forth", Value: "some 15 thing"},
					{Name: "here", Value: "else"},
					{Name: "here2", Value: "else2"},
					{Name: "sixth", Value: "some 1 thing"},
					{Name: "third", Value: "some 1 thing"},
				},
			},
			{
				Samples: []prompb.Sample{{
					Value:     valueBetween(r, minValue, maxValue),
					Timestamp: timestamp,
				}},
				Labels: []prompb.Label{
					{Name: "fifth", Value: "some 8 thing"},
					{Name: "forth", Value: "some 16 thing"},
					{Name: "here", Value: "else"},
					{Name: "here2", Value: "else2"},
					{Name: "sixth", Value: "some 1 thing"},
					{Name: "third", Value: "some 0 thing"},
				},
			},
			{
				Samples: []prompb.Sample{{
					Value:     valueBetween(r, minValue, maxValue),
					Timestamp: timestamp,
				}},
				Labels: []prompb.Label{
					{Name: "fifth", Value: "some 8 thing"},
					{Name: "forth", Value: "some 17 thing"},
					{Name: "here", Value: "else"},
					{Name: "here2", Value: "else2"},
					{Name: "sixth", Value: "some 1 thing"},
					{Name: "third", Value: "some 1 thing"},
				},
			},
			{
				Samples: []prompb.Sample{{
					Value:     valueBetween(r, minValue, maxValue),
					Timestamp: timestamp,
				}},
				Labels: []prompb.Label{
					{Name: "fifth", Value: "some 9 thing"},
					{Name: "forth", Value: "some 18 thing"},
					{Name: "here", Value: "else"},
					{Name: "here2", Value: "else2"},
					{Name: "sixth", Value: "some 1 thing"},
					{Name: "third", Value: "some 0 thing"},
				},
			},
			{
				Samples: []prompb.Sample{{
					Value:     valueBetween(r, minValue, maxValue),
					Timestamp: timestamp,
				}},
				Labels: []prompb.Label{
					{Name: "fifth", Value: "some 9 thing"},
					{Name: "forth", Value: "some 19 thing"},
					{Name: "here", Value: "else"},
					{Name: "here2", Value: "else2"},
					{Name: "sixth", Value: "some 1 thing"},
					{Name: "third", Value: "some 1 thing"},
				},
			},
			{
				Samples: []prompb.Sample{{
					Value:     valueBetween(r, minValue, maxValue),
					Timestamp: timestamp,
				}},
				Labels: []prompb.Label{
					{Name: "fifth", Value: "some 10 thing"},
					{Name: "forth", Value: "some 20 thing"},
					{Name: "here", Value: "else"},
					{Name: "here2", Value: "else2"},
					{Name: "sixth", Value: "some 2 thing"},
					{Name: "third", Value: "some 0 thing"},
				},
			},
			{
				Samples: []prompb.Sample{{
					Value:     valueBetween(r, minValue, maxValue),
					Timestamp: timestamp,
				}},
				Labels: []prompb.Label{
					{Name: "fifth", Value: "some 10 thing"},
					{Name: "forth", Value: "some 21 thing"},
					{Name: "here", Value: "else"},
					{Name: "here2", Value: "else2"},
					{Name: "sixth", Value: "some 2 thing"},
					{Name: "third", Value: "some 1 thing"},
				},
			},
		},
	}))

	// #nosec G404 -- Using math/rand in test code with deterministic seed for reproducible tests
	r = rand.New(rand.NewSource(seed)) // reset
	template, err := compileLabelTemplates(map[string]string{
		"here":  "else",
		"here2": "else2",
		"third": "some ${series_id%2} thing",
		"forth": "some ${series_id} thing",
		"fifth": "some ${series_id/2} thing",
		"sixth": "some ${series_id/10} thing",
	})
	require.NoError(t, err)

	buf, err := generateFromPrecompiledTemplates(r, minValue, maxValue, timestamp, 15, 22, template)
	require.NoError(t, err)

	b := buf.Bytes()
	require.Equal(t, d, b)
}

func BenchmarkWriteFor(b *testing.B) {
	tsBuf := new(bytes.Buffer)
	template, err := compileLabelTemplates(map[string]string{
		"__name__":        "k6_generated_metric_${series_id/1000}", // Name of the series.
		"series_id":       "${series_id}",                          // Each value of this label will match 1 series.
		"cardinality_1e1": "${series_id/10}",                       // Each value of this label will match 10 series.
		"cardinality_1e2": "${series_id/100}",                      // Each value of this label will match 100 series.
		"cardinality_1e3": "${series_id/1000}",                     // Each value of this label will match 1000 series.
		"cardinality_1e4": "${series_id/10000}",                    // Each value of this label will match 10000 series.
		"cardinality_1e5": "${series_id/100000}",                   // Each value of this label will match 100000 series.
		"cardinality_1e6": "${series_id/1000000}",                  // Each value of this label will match 1000000 series.
		"cardinality_1e7": "${series_id/10000000}",                 // Each value of this label will match 10000000 series.
		"cardinality_1e8": "${series_id/100000000}",                // Each value of this label will match 100000000 series.
		"cardinality_1e9": "${series_id/1000000000}",               // Each value of this label will match 1000000000 series.
	})
	require.NoError(b, err)
	template.writeFor(tsBuf, 15, 15, 234)
	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		template.writeFor(tsBuf, 15, i, 234)
	}
}
