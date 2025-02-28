package file

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseArg(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		input            string
		expectedName     string
		expectedFilename string
		expectedError    string
	}{
		"simple": {
			input:            "something.secret",
			expectedName:     "",
			expectedFilename: "something.secret",
		},
		"filename": {
			input:            "filename=something.secret",
			expectedName:     "",
			expectedFilename: "something.secret",
		},
		"filename and name": {
			input:            "filename=something.secret,name=cool",
			expectedName:     "cool",
			expectedFilename: "something.secret",
		},
		"unknownfiled": {
			input:         "filename=something.secret,name=cool,random=bad",
			expectedError: "unknown configuration key for file secret source \"random\"",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fss := &fileSecretSource{}
			err := fss.parseArg(testCase.input)
			if testCase.expectedError != "" {
				require.ErrorContains(t, err, testCase.expectedError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, testCase.expectedFilename, fss.filename)
			require.Equal(t, testCase.expectedName, fss.name)
		})
	}
}
