package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenizer(t *testing.T) {
	t.Parallel()

	input := "loki=something,s.e=2231,s=12,12=3,a=[1,2,3],b=[1],s=c"
	tokens, err := tokenize(input)
	require.NoError(t, err)

	expected := []token{
		{key: "loki", value: "something"},
		{key: "s.e", value: "2231"},
		{key: "s", value: "12"},
		{key: "12", value: "3"},
		{key: "a", value: "1,2,3", inside: '['},
		{key: "b", value: "1", inside: '['},
		{key: "s", value: "c"},
	}
	assert.Equal(t, expected, tokens)
}

func TestTokenizerInvalid(t *testing.T) {
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

			_, err := tokenize(test.input)
			require.EqualError(t, err, test.errorMsg)
		})
	}
}
