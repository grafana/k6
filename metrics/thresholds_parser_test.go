package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
)

func TestParseThresholdExpression(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantExpression *thresholdExpression
		wantErr        bool
	}{
		{
			name:           "unknown expression's operator fails",
			input:          "count!20",
			wantExpression: nil,
			wantErr:        true,
		},
		{
			name:           "unknown expression's method fails",
			input:          "foo>20",
			wantExpression: nil,
			wantErr:        true,
		},
		{
			name:           "non numerical expression's value fails",
			input:          "count>abc",
			wantExpression: nil,
			wantErr:        true,
		},
		{
			name:           "valid threshold expression syntax",
			input:          "count>20",
			wantExpression: &thresholdExpression{AggregationMethod: "count", Operator: ">", Value: 20},
			wantErr:        false,
		},
	}
	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gotExpression, gotErr := parseThresholdExpression(testCase.input)

			assert.Equal(t,
				testCase.wantErr,
				gotErr != nil,
				"parseThresholdExpression() error = %v, wantErr %v", gotErr, testCase.wantErr,
			)

			assert.Equal(t,
				testCase.wantExpression,
				gotExpression,
				"parseThresholdExpression() gotExpression = %v, want %v", gotExpression, testCase.wantExpression,
			)
		})
	}
}

func BenchmarkParseThresholdExpression(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = parseThresholdExpression("count>20")
	}
}

func TestParseThresholdAggregationMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		input           string
		wantMethod      string
		wantMethodValue null.Float
		wantErr         bool
	}{
		{
			name:            "count method is parsed",
			input:           "count",
			wantMethod:      tokenCount,
			wantMethodValue: null.Float{},
			wantErr:         false,
		},
		{
			name:            "rate method is parsed",
			input:           "rate",
			wantMethod:      tokenRate,
			wantMethodValue: null.Float{},
			wantErr:         false,
		},
		{
			name:            "value method is parsed",
			input:           "value",
			wantMethod:      tokenValue,
			wantMethodValue: null.Float{},
			wantErr:         false,
		},
		{
			name:            "avg method is parsed",
			input:           "avg",
			wantMethod:      tokenAvg,
			wantMethodValue: null.Float{},
			wantErr:         false,
		},
		{
			name:            "min method is parsed",
			input:           "min",
			wantMethod:      tokenMin,
			wantMethodValue: null.Float{},
			wantErr:         false,
		},
		{
			name:            "max method is parsed",
			input:           "max",
			wantMethod:      tokenMax,
			wantMethodValue: null.Float{},
			wantErr:         false,
		},
		{
			name:            "med method is parsed",
			input:           "med",
			wantMethod:      tokenMed,
			wantMethodValue: null.Float{},
			wantErr:         false,
		},
		{
			name:            "percentile method with integer value is parsed",
			input:           "p(99)",
			wantMethod:      tokenPercentile,
			wantMethodValue: null.FloatFrom(99),
			wantErr:         false,
		},
		{
			name:            "percentile method with floating point value is parsed",
			input:           "p(99.9)",
			wantMethod:      tokenPercentile,
			wantMethodValue: null.FloatFrom(99.9),
			wantErr:         false,
		},
		{
			name:            "parsing invalid method fails",
			input:           "foo",
			wantMethod:      "",
			wantMethodValue: null.Float{},
			wantErr:         true,
		},
		{
			name:            "parsing empty percentile expression fails",
			input:           "p()",
			wantMethod:      "",
			wantMethodValue: null.Float{},
			wantErr:         true,
		},
		{
			name:            "parsing incomplete percentile expression fails",
			input:           "p(99",
			wantMethod:      "",
			wantMethodValue: null.Float{},
			wantErr:         true,
		},
		{
			name:            "parsing non-numerical percentile value fails",
			input:           "p(foo)",
			wantMethod:      "",
			wantMethodValue: null.Float{},
			wantErr:         true,
		},
	}
	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gotMethod, gotMethodValue, gotErr := parseThresholdAggregationMethod(testCase.input)

			assert.Equal(t,
				testCase.wantErr,
				gotErr != nil,
				"parseThresholdAggregationMethod() error = %v, wantErr %v", gotErr, testCase.wantErr,
			)

			assert.Equal(t,
				testCase.wantMethod,
				gotMethod,
				"parseThresholdAggregationMethod() gotMethod = %v, want %v", gotMethod, testCase.wantMethod,
			)

			assert.Equal(t,
				testCase.wantMethodValue,
				gotMethodValue,
				"parseThresholdAggregationMethod() gotMethodValue = %v, want %v", gotMethodValue, testCase.wantMethodValue,
			)
		})
	}
}

func BenchmarkParseThresholdAggregationMethod(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _ = parseThresholdAggregationMethod("p(99.9)")
	}
}

func TestScanThresholdExpression(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantMethod   string
		wantOperator string
		wantValue    string
		wantErr      bool
	}{
		{
			name:         "expression with <= operator is scanned",
			input:        "foo<=bar",
			wantMethod:   "foo",
			wantOperator: "<=",
			wantValue:    "bar",
			wantErr:      false,
		},
		{
			name:         "expression with < operator is scanned",
			input:        "foo<bar",
			wantMethod:   "foo",
			wantOperator: "<",
			wantValue:    "bar",
			wantErr:      false,
		},
		{
			name:         "expression with >= operator is scanned",
			input:        "foo>=bar",
			wantMethod:   "foo",
			wantOperator: ">=",
			wantValue:    "bar",
			wantErr:      false,
		},
		{
			name:         "expression with > operator is scanned",
			input:        "foo>bar",
			wantMethod:   "foo",
			wantOperator: ">",
			wantValue:    "bar",
			wantErr:      false,
		},
		{
			name:         "expression with === operator is scanned",
			input:        "foo===bar",
			wantMethod:   "foo",
			wantOperator: "===",
			wantValue:    "bar",
			wantErr:      false,
		},
		{
			name:         "expression with == operator is scanned",
			input:        "foo==bar",
			wantMethod:   "foo",
			wantOperator: "==",
			wantValue:    "bar",
			wantErr:      false,
		},
		{
			name:         "expression with != operator is scanned",
			input:        "foo!=bar",
			wantMethod:   "foo",
			wantOperator: "!=",
			wantValue:    "bar",
			wantErr:      false,
		},
		{
			name:         "expression's method is trimmed",
			input:        "  foo  <=bar",
			wantMethod:   "foo",
			wantOperator: "<=",
			wantValue:    "bar",
			wantErr:      false,
		},
		{
			name:         "expression's value is trimmed",
			input:        "foo<=  bar  ",
			wantMethod:   "foo",
			wantOperator: "<=",
			wantValue:    "bar",
			wantErr:      false,
		},
		{
			name:         "expression with unknown operator fails",
			input:        "foo!bar",
			wantMethod:   "",
			wantOperator: "",
			wantValue:    "",
			wantErr:      true,
		},
	}
	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gotMethod, gotOperator, gotValue, gotErr := scanThresholdExpression(testCase.input)

			assert.Equal(t,
				testCase.wantErr,
				gotErr != nil,
				"scanThresholdExpression() error = %v, wantErr %v", gotErr, testCase.wantErr,
			)

			assert.Equal(t,
				testCase.wantMethod,
				gotMethod,
				"scanThresholdExpression() gotMethod = %v, want %v", gotMethod, testCase.wantMethod,
			)

			assert.Equal(t,
				testCase.wantOperator,
				gotOperator,
				"scanThresholdExpression() gotOperator = %v, want %v", gotOperator, testCase.wantOperator,
			)

			assert.Equal(t,
				testCase.wantValue,
				gotValue,
				"scanThresholdExpression() gotValue = %v, want %v", gotValue, testCase.wantValue,
			)
		})
	}
}

func BenchmarkScanThresholdExpression(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _, _ = scanThresholdExpression("foo<=bar")
	}
}
