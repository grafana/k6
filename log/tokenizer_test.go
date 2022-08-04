package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenizer(t *testing.T) {
	tokens, err := tokenize("loki=something,s.e=2231,s=12,12=3,a=[1,2,3],b=[1],s=c")
	assert.Equal(t, []token{
		{
			key:   "loki",
			value: "something",
		},
		{
			key:   "s.e",
			value: "2231",
		},
		{
			key:   "s",
			value: "12",
		},
		{
			key:   "12",
			value: "3",
		},
		{
			key:    "a",
			value:  "1,2,3",
			inside: '[',
		},
		{
			key:    "b",
			value:  "1",
			inside: '[',
		},
		{
			key:   "s",
			value: "c",
		},
	}, tokens)
	assert.NoError(t, err)

	_, err = tokenize("empty=")
	assert.EqualError(t, err, "key `empty=` with no value")
}
