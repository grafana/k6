package strvals

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser(t *testing.T) {
	t.Parallel()

	input := "loki=something,s.e=2231,s=12,12=3,a=[1,2,3],b=[1],s=c"
	tokens, err := Parse(input)
	require.NoError(t, err)

	expected := []Token{
		{Key: "loki", Value: "something"},
		{Key: "s.e", Value: "2231"},
		{Key: "s", Value: "12"},
		{Key: "12", Value: "3"},
		{Key: "a", Value: "1,2,3", inside: '['},
		{Key: "b", Value: "1", inside: '['},
		{Key: "s", Value: "c"},
	}
	assert.Equal(t, expected, tokens)
}

func TestParserInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		errorMsg string
	}{
		{
			name:     "empty value",
			input:    "empty=",
			errorMsg: "key `empty=` with no value",
		},
		{
			name:     "unclosed array",
			input:    "foo=[1,2,3",
			errorMsg: "array value for key `foo` didn't end",
		},
		{
			name:     "bad characters after array",
			input:    "foo=[1,2,3]bar",
			errorMsg: "there was no ',' after an array with key 'foo'",
		},
		{
			name:     "empty key",
			input:    ",foo=bar",
			errorMsg: "key `` with no value",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(test.input)
			require.EqualError(t, err, test.errorMsg)
		})
	}
}
