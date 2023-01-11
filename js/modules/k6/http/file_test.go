package http

import (
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

func TestHTTPFile(t *testing.T) {
	t.Parallel()
	input := []byte{104, 101, 108, 108, 111}

	testCases := []struct {
		input    func(rt *goja.Runtime) interface{}
		args     []string
		expected FileData
		expErr   string
	}{
		// We can't really test without specifying a filename argument,
		// as File() calls time.Now(), so we'd need some time freezing/mocking
		// or refactoring, or to exclude the field from the assertion.
		{
			func(*goja.Runtime) interface{} { return input },
			[]string{"test.bin"},
			FileData{Data: input, Filename: "test.bin", ContentType: "application/octet-stream"},
			"",
		},
		{
			func(*goja.Runtime) interface{} { return string(input) },
			[]string{"test.txt", "text/plain"},
			FileData{Data: input, Filename: "test.txt", ContentType: "text/plain"},
			"",
		},
		{
			func(rt *goja.Runtime) interface{} { return rt.NewArrayBuffer(input) },
			[]string{"test-ab.bin"},
			FileData{Data: input, Filename: "test-ab.bin", ContentType: "application/octet-stream"},
			"",
		},
		{func(*goja.Runtime) interface{} { return struct{}{} }, []string{}, FileData{}, "GoError: invalid type struct {}, expected string, []byte or ArrayBuffer"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Parallel()
			runtime, mi := getTestModuleInstance(t)
			rt := runtime.VU.RuntimeField
			cal, _ := goja.AssertFunction(rt.ToValue(mi.file))
			args := make([]goja.Value, 1, len(tc.args)+1)
			args[0] = rt.ToValue(tc.input(rt))
			for _, arg := range tc.args {
				args = append(args, rt.ToValue(arg))
			}
			_, err := cal(goja.Undefined(), args...)
			if tc.expErr != "" {
				require.EqualError(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHTTPFileDataInRequest(t *testing.T) {
	t.Parallel()
	ts := newTestCase(t)

	_, err := ts.runtime.VU.Runtime().RunString(ts.tb.Replacer.Replace(`
    let f = http.file("something");
    let res = http.request("POST", "HTTPBIN_URL/post", f.data);
    if (res.status != 200) {
      throw new Error("Unexpected status " + res.status)
    }`))
	require.NoError(t, err)
}
