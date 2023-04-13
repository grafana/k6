package tracing

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTraceID(t *testing.T) {
	t.Parallel()

	testTime := time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)
	testRandSourceFn := func() io.Reader { return bytes.NewReader([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}) }

	// Precomputed hexadecimal representation of the binary values
	// of the traceID components.
	wantPrefixHexString := "dc07"
	wantCloudCodeHexString := "18"
	wantLocalCodeHexString := "42"
	wantTimeHexString := "8080f8e1949cfec52d"
	wantRandHexString := "01020304"

	testCases := []struct {
		name       string
		prefix     int16
		code       int8
		t          time.Time
		randSource io.Reader
		wantErr    bool
	}{
		{
			name:       "valid traceID with cloud code should succeed",
			prefix:     k6Prefix,
			code:       k6CloudCode,
			t:          testTime,
			randSource: testRandSourceFn(),
			wantErr:    false,
		},
		{
			name:       "valid traceID with local code should succeed",
			prefix:     k6Prefix,
			code:       k6LocalCode,
			t:          testTime,
			randSource: testRandSourceFn(),
			wantErr:    false,
		},
		{
			name:       "traceID with invalid prefix should fail",
			prefix:     0o123,
			code:       k6CloudCode,
			t:          testTime,
			randSource: testRandSourceFn(),
			wantErr:    true,
		},
		{
			name:       "traceID with invalid code should fail",
			prefix:     k6Prefix,
			code:       0o123,
			t:          testTime,
			randSource: testRandSourceFn(),
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotTraceID, gotErr := newTraceID(tc.prefix, tc.code, tc.t, tc.randSource)

			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			prefixEndOffset := len(wantPrefixHexString)
			assert.Equal(t, wantPrefixHexString, gotTraceID[:prefixEndOffset])

			codeEndOffset := prefixEndOffset + len(wantCloudCodeHexString)
			if tc.code == k6CloudCode {
				assert.Equal(t, wantCloudCodeHexString, gotTraceID[prefixEndOffset:codeEndOffset])
			} else {
				assert.Equal(t, wantLocalCodeHexString, gotTraceID[prefixEndOffset:codeEndOffset])
			}

			timeEndOffset := codeEndOffset + len(wantTimeHexString)
			assert.Equal(t, wantTimeHexString, gotTraceID[codeEndOffset:timeEndOffset])

			assert.Equal(t, wantRandHexString, gotTraceID[timeEndOffset:])
		})
	}
}
