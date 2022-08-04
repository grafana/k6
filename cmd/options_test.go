package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTagKeyValue(t *testing.T) {
	t.Parallel()
	testData := []struct {
		input string
		name  string
		value string
		err   error
	}{
		{
			"",
			"",
			"",
			errTagEmptyString,
		},
		{
			"=",
			"",
			"",
			errTagEmptyName,
		},
		{
			"=test",
			"",
			"",
			errTagEmptyName,
		},
		{
			"test",
			"",
			"",
			errTagEmptyValue,
		},
		{
			"test=",
			"",
			"",
			errTagEmptyValue,
		},
		{
			"myTag=foo",
			"myTag",
			"foo",
			nil,
		},
	}

	for _, data := range testData {
		data := data
		t.Run(data.input, func(t *testing.T) {
			t.Parallel()
			name, value, err := parseTagNameValue(data.input)
			assert.Equal(t, name, data.name)
			assert.Equal(t, value, data.value)
			assert.Equal(t, err, data.err)
		})
	}
}
