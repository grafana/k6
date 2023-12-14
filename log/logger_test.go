package log

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLogFormatter struct{}

func (f *testLogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(entry.Message), nil
}

func TestConsoleLogFormatter(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		stringObjects []string
		expected      string
	}{
		{stringObjects: nil, expected: ""},
		{
			stringObjects: []string{
				`{"one":1,"two":"two"}`,
				`{"nested":{"sub":7.777}}`,
			},
			expected: `{"one":1,"two":"two"} {"nested":{"sub":7.777}}`,
		},
	}

	fmtr := &consoleLogFormatter{&testLogFormatter{}}

	for _, tc := range testCases {
		var data logrus.Fields
		if tc.stringObjects != nil {
			data = logrus.Fields{"stringObjects": tc.stringObjects}
		}
		out, err := fmtr.Format(&logrus.Entry{Data: data})
		require.NoError(t, err)
		assert.Equal(t, tc.expected, string(out))
	}
}
