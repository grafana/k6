package http

import (
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPFile(t *testing.T) {
	t.Parallel()
	rt, mi, _ := getTestModuleInstance(t)
	input := []byte{104, 101, 108, 108, 111}

	testCases := []struct {
		input    interface{}
		args     []string
		expected FileData
		expErr   string
	}{
		// We can't really test without specifying a filename argument,
		// as File() calls time.Now(), so we'd need some time freezing/mocking
		// or refactoring, or to exclude the field from the assertion.
		{
			input,
			[]string{"test.bin"},
			FileData{Data: input, Filename: "test.bin", ContentType: "application/octet-stream"},
			"",
		},
		{
			string(input),
			[]string{"test.txt", "text/plain"},
			FileData{Data: input, Filename: "test.txt", ContentType: "text/plain"},
			"",
		},
		{
			rt.NewArrayBuffer(input),
			[]string{"test-ab.bin"},
			FileData{Data: input, Filename: "test-ab.bin", ContentType: "application/octet-stream"},
			"",
		},
		{struct{}{}, []string{}, FileData{}, "invalid type struct {}, expected string, []byte or ArrayBuffer"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%T", tc.input), func(t *testing.T) {
			if tc.expErr != "" {
				defer func() {
					err := recover()
					require.NotNil(t, err)
					require.IsType(t, &goja.Object{}, err)
					val := err.(*goja.Object).Export()
					require.EqualError(t, val.(error), tc.expErr)
				}()
			}
			out := mi.file(tc.input, tc.args...)
			assert.Equal(t, tc.expected, out)
		})
	}
}

func TestHTTPFileDataInRequest(t *testing.T) {
	t.Parallel()

	tb, _, _, rt, _ := newRuntime(t) //nolint:dogsled
	_, err := rt.RunString(tb.Replacer.Replace(`
    let f = http.file("something");
    let res = http.request("POST", "HTTPBIN_URL/post", f.data);
    if (res.status != 200) {
      throw new Error("Unexpected status " + res.status)
    }`))
	require.NoError(t, err)
}
