package file

import (
	"bufio"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/ext"
	"go.k6.io/k6/v2/lib/fsext"
	"go.k6.io/k6/v2/secretsource"
)

func TestParseArg(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		input            string
		expectedFilename string
		expectedError    string
	}{
		"simple": {
			input:            "something.secret",
			expectedFilename: "something.secret",
		},
		"filename": {
			input:            "filename=something.secret",
			expectedFilename: "something.secret",
		},
		"filename and name": {
			input:            "filename=something.secret",
			expectedFilename: "something.secret",
		},
		"unknownfiled": {
			input:         "filename=something.secret,random=bad",
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
		})
	}
}

func TestReadKeyValueLines(t *testing.T) {
	t.Parallel()

	got, err := readKeyValueLines(strings.NewReader("first=ok\ntail=present\n"))
	require.NoError(t, err)
	require.Equal(t, map[string]string{"first": "ok", "tail": "present"}, got)
}

func TestReadKeyValueLinesRejectsOversizedLine(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	b.WriteString("first=ok\n")
	b.WriteString("large=")
	b.WriteString(strings.Repeat("x", bufio.MaxScanTokenSize))
	b.WriteString("\ntail=present\n")

	got, err := readKeyValueLines(strings.NewReader(b.String()))
	require.ErrorIs(t, err, bufio.ErrTooLong)
	require.Nil(t, got)
}

func TestFileSourceRejectsOversizedLine(t *testing.T) {
	t.Parallel()

	const filename = "/secrets.file"
	var b strings.Builder
	b.WriteString("first=ok\n")
	b.WriteString("large=")
	b.WriteString(strings.Repeat("x", bufio.MaxScanTokenSize))
	b.WriteString("\ntail=present\n")

	fs := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fs, filename, []byte(b.String()), 0o600))

	extensions := ext.Get(ext.SecretSourceExtension)
	fileExt, ok := extensions["file"]
	require.True(t, ok, "file secret source not registered")
	constructor, ok := fileExt.Module.(secretsource.Constructor)
	require.True(t, ok, "file secret source has invalid type")

	source, err := constructor(secretsource.Params{
		ConfigArgument: filename,
		FS:             fs,
	})
	require.ErrorIs(t, err, bufio.ErrTooLong)
	require.Nil(t, source)
}
