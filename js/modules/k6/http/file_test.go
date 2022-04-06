/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

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
	rt, mi := getTestModuleInstance(t, nil, nil)
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
