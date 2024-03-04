package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"

	"github.com/grafana/xk6-browser/common"
)

// TestSetInputFiles tests the SetInputFiles function.
func TestSetInputFiles(t *testing.T) {
	t.Parallel()

	type file map[string]interface{}
	type indexedFn func(idx int, propName string) interface{}
	type testFn func(tb *testBrowser, page *common.Page, files goja.Value) error
	type setupFn func(tb *testBrowser) (goja.Value, func())
	type checkFn func(t *testing.T,
		getFileCountFn func() interface{},
		getFilePropFn indexedFn,
		err error)

	const (
		propName = "name"
		propType = "type"
		propSize = "size"
	)

	pageContent := `
	  <input type="file" id="upload">
	`

	defaultTestPage := func(tb *testBrowser, page *common.Page, files goja.Value) error {
		return page.SetInputFiles("input", files, tb.toGojaValue(nil))
	}
	defaultTestElementHandle := func(tb *testBrowser, page *common.Page, files goja.Value) error {
		handle, err := page.WaitForSelector("input", tb.toGojaValue(nil))
		assert.NoError(t, err)
		return handle.SetInputFiles(files, tb.toGojaValue(nil))
	}

	testCases := []struct {
		name  string
		setup setupFn
		tests []testFn
		check checkFn
	}{
		{
			name: "set_one_file_with_object",
			setup: func(tb *testBrowser) (goja.Value, func()) {
				return tb.toGojaValue(file{"name": "test.json", "mimetype": "text/json", "buffer": "MDEyMzQ1Njc4OQ=="}), nil
			},
			tests: []testFn{defaultTestPage, defaultTestElementHandle},
			check: func(t *testing.T, getFileCountFn func() interface{}, getFilePropFn indexedFn, err error) {
				t.Helper()
				assert.NoError(t, err)
				// check if input has 1 file
				assert.Equal(t, float64(1), getFileCountFn())
				// check added file is correct
				assert.Equal(t, "test.json", getFilePropFn(0, propName))
				assert.Equal(t, float64(10), getFilePropFn(0, propSize))
				assert.Equal(t, "text/json", getFilePropFn(0, propType))
			},
		},
		{
			name: "set_two_files_with_array_of_objects",
			setup: func(tb *testBrowser) (goja.Value, func()) {
				return tb.toGojaValue(
					[]file{
						{"name": "test.json", "mimetype": "text/json", "buffer": "MDEyMzQ1Njc4OQ=="},
						{"name": "test.xml", "mimetype": "text/xml", "buffer": "MDEyMzQ1Njc4OTAxMjM0"},
					}), nil
			},
			tests: []testFn{defaultTestPage, defaultTestElementHandle},
			check: func(t *testing.T, getFileCountFn func() interface{}, getFilePropFn indexedFn, err error) {
				t.Helper()
				assert.NoError(t, err)
				// check if input has 2 files
				assert.Equal(t, float64(2), getFileCountFn())
				// check added files are correct
				assert.Equal(t, "test.json", getFilePropFn(0, propName))
				assert.Equal(t, float64(10), getFilePropFn(0, propSize))
				assert.Equal(t, "text/json", getFilePropFn(0, propType))
				assert.Equal(t, "test.xml", getFilePropFn(1, propName))
				assert.Equal(t, float64(15), getFilePropFn(1, propSize))
				assert.Equal(t, "text/xml", getFilePropFn(1, propType))
			},
		},
		{
			name: "set_one_file_with_path",
			setup: func(tb *testBrowser) (goja.Value, func()) {
				tempFile, err := os.CreateTemp("", "*.json")
				assert.NoError(t, err)
				n, err := tempFile.Write([]byte("0123456789"))
				assert.Equal(t, 10, n)
				assert.NoError(t, err)
				cleanupFunc := func() {
					err := os.Remove(tempFile.Name())
					assert.NoError(t, err)
				}
				return tb.toGojaValue(tempFile.Name()), cleanupFunc
			},
			tests: []testFn{defaultTestPage, defaultTestElementHandle},
			check: func(t *testing.T, getFileCountFn func() interface{}, getFilePropFn indexedFn, err error) {
				t.Helper()
				assert.NoError(t, err)
				// check if input has 1 file
				assert.Equal(t, float64(1), getFileCountFn())
				// check added file is correct
				filename, ok := getFilePropFn(0, propName).(string)
				assert.True(t, ok)
				assert.Equal(t, ".json", filepath.Ext(filename))
				assert.Equal(t, float64(10), getFilePropFn(0, propSize))
				assert.Equal(t, "application/json", getFilePropFn(0, propType))
			},
		},
		{
			name: "set_two_files_with_path",
			setup: func(tb *testBrowser) (goja.Value, func()) {
				tempFile1, err := os.CreateTemp("", "*.json")
				assert.NoError(t, err)
				n, err := tempFile1.Write([]byte("0123456789"))
				assert.Equal(t, 10, n)
				assert.NoError(t, err)

				tempFile2, err := os.CreateTemp("", "*.xml")
				assert.NoError(t, err)
				n, err = tempFile2.Write([]byte("012345678901234"))
				assert.Equal(t, 15, n)
				assert.NoError(t, err)
				cleanupFunc := func() {
					err := os.Remove(tempFile1.Name())
					assert.NoError(t, err)
					err = os.Remove(tempFile2.Name())
					assert.NoError(t, err)
				}

				return tb.toGojaValue([]string{tempFile1.Name(), tempFile2.Name()}), cleanupFunc
			},
			tests: []testFn{defaultTestPage, defaultTestElementHandle},
			check: func(t *testing.T, getFileCountFn func() interface{}, getFilePropFn indexedFn, err error) {
				t.Helper()
				assert.NoError(t, err)
				// check if input has 2 files
				assert.Equal(t, float64(2), getFileCountFn())
				// check added files are correct
				filename1, ok := getFilePropFn(0, propName).(string)
				assert.True(t, ok)
				assert.Equal(t, ".json", filepath.Ext(filename1))
				assert.Equal(t, float64(10), getFilePropFn(0, propSize))
				assert.Equal(t, "application/json", getFilePropFn(0, propType))
				filename2, ok := getFilePropFn(1, propName).(string)
				assert.True(t, ok)
				assert.Equal(t, ".xml", filepath.Ext(filename2))
				assert.Equal(t, float64(15), getFilePropFn(1, propSize))
				assert.Equal(t, "text/xml; charset=utf-8", getFilePropFn(1, propType))
			},
		},
		{
			name: "set_nil",
			setup: func(tb *testBrowser) (goja.Value, func()) {
				return tb.toGojaValue(nil), nil
			},
			tests: []testFn{defaultTestPage, defaultTestElementHandle},
			check: func(t *testing.T, getFileCountFn func() interface{}, getFilePropertyFn indexedFn, err error) {
				t.Helper()
				assert.NoError(t, err)
				// check if input has 1 file
				assert.Equal(t, float64(0), getFileCountFn())
			},
		},
		{
			name: "set_invalid_parameter",
			setup: func(tb *testBrowser) (goja.Value, func()) {
				return tb.toGojaValue([]int{12345}), nil
			},
			tests: []testFn{defaultTestPage, defaultTestElementHandle},
			check: func(t *testing.T, getFileCountFn func() interface{}, getFilePropFn indexedFn, err error) {
				t.Helper()
				assert.ErrorContains(t, err, "invalid parameter type : int64")
				// check if input has 0 file
				assert.Equal(t, float64(0), getFileCountFn())
			},
		},
		{
			name: "set_file_not_exists",
			setup: func(tb *testBrowser) (goja.Value, func()) {
				tempFile, err := os.CreateTemp("", "*.json")
				assert.NoError(t, err)
				err = os.Remove(tempFile.Name())
				assert.NoError(t, err)
				return tb.toGojaValue(tempFile.Name()), nil
			},
			tests: []testFn{defaultTestPage, defaultTestElementHandle},
			check: func(t *testing.T, getFileCountFn func() interface{}, getFilePropFn indexedFn, err error) {
				t.Helper()
				assert.ErrorContains(t, err, "reading file:")
				assert.ErrorContains(t, err, "setting input files")
				// check if input has 0 file
				assert.Equal(t, float64(0), getFileCountFn())
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		for _, test := range tc.tests {
			test := test
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				tb := newTestBrowser(t)
				defer tb.Browser.Close()
				page := tb.NewPage(nil)
				page.SetContent(pageContent, nil)

				getFileCountFn := func() interface{} {
					return page.Evaluate(`() => document.getElementById("upload").files.length`)
				}

				getFilePropertyFn := func(idx int, propName string) interface{} {
					return page.Evaluate(
						`(idx, propName) => document.getElementById("upload").files[idx][propName]`,
						idx,
						propName)
				}

				files, cleanup := tc.setup(tb)
				if cleanup != nil {
					defer cleanup()
				}
				err := test(tb, page, files)
				tc.check(t, getFileCountFn, getFilePropertyFn, err)
			})
		}
	}
}
