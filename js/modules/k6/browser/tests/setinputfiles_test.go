package tests

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/common"
)

// TestSetInputFiles tests the SetInputFiles function.
func TestSetInputFiles(t *testing.T) {
	t.Parallel()

	type file map[string]interface{}
	type indexedFn func(idx int, propName string) interface{}
	type testFn func(tb *testBrowser, page *common.Page, files sobek.Value) error
	type setupFn func(tb *testBrowser) (sobek.Value, func())
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
	  <input type="file" id="upload"/>
	  <input type="text" id="textinput"/>
	  <button id="button1">Click</button>
	`

	defaultTestPage := func(tb *testBrowser, page *common.Page, files sobek.Value) error {
		return page.SetInputFiles("#upload", files, tb.toSobekValue(nil))
	}
	defaultTestElementHandle := func(tb *testBrowser, page *common.Page, files sobek.Value) error {
		handle, err := page.WaitForSelector("#upload", tb.toSobekValue(nil))
		assert.NoError(t, err)
		return handle.SetInputFiles(files, tb.toSobekValue(nil))
	}

	testCases := []struct {
		name  string
		setup setupFn
		tests []testFn
		check checkFn
	}{
		{
			name: "set_one_file_with_object",
			setup: func(tb *testBrowser) (sobek.Value, func()) {
				return tb.toSobekValue(file{"name": "test.json", "mimetype": "text/json", "buffer": "MDEyMzQ1Njc4OQ=="}), nil
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
			setup: func(tb *testBrowser) (sobek.Value, func()) {
				return tb.toSobekValue(
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
			name: "set_nil",
			setup: func(tb *testBrowser) (sobek.Value, func()) {
				return tb.toSobekValue(nil), nil
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
			setup: func(tb *testBrowser) (sobek.Value, func()) {
				return tb.toSobekValue([]int{12345}), nil
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
			name: "test_injected_script_notinput",
			setup: func(tb *testBrowser) (sobek.Value, func()) {
				return tb.toSobekValue(file{"name": "test.json", "mimetype": "text/json", "buffer": "MDEyMzQ1Njc4OQ=="}), nil
			},
			tests: []testFn{
				func(tb *testBrowser, page *common.Page, files sobek.Value) error {
					return page.SetInputFiles("#button1", files, tb.toSobekValue(nil))
				},
				func(tb *testBrowser, page *common.Page, files sobek.Value) error {
					handle, err := page.WaitForSelector("#button1", tb.toSobekValue(nil))
					assert.NoError(t, err)
					return handle.SetInputFiles(files, tb.toSobekValue(nil))
				},
			},
			check: func(t *testing.T, getFileCountFn func() interface{}, getFilePropFn indexedFn, err error) {
				t.Helper()
				assert.ErrorContains(t, err, "node is not an HTMLInputElement")
				assert.ErrorContains(t, err, "setting input files")
				// check if input has 0 file
				assert.Equal(t, float64(0), getFileCountFn())
			},
		},
		{
			name: "test_injected_script_notfile",
			setup: func(tb *testBrowser) (sobek.Value, func()) {
				return tb.toSobekValue(file{"name": "test.json", "mimetype": "text/json", "buffer": "MDEyMzQ1Njc4OQ=="}), nil
			},
			tests: []testFn{
				func(tb *testBrowser, page *common.Page, files sobek.Value) error {
					return page.SetInputFiles("#textinput", files, tb.toSobekValue(nil))
				},
				func(tb *testBrowser, page *common.Page, files sobek.Value) error {
					handle, err := page.WaitForSelector("#textinput", tb.toSobekValue(nil))
					assert.NoError(t, err)
					return handle.SetInputFiles(files, tb.toSobekValue(nil))
				},
			},
			check: func(t *testing.T, getFileCountFn func() interface{}, getFilePropFn indexedFn, err error) {
				t.Helper()
				assert.ErrorContains(t, err, "node is not an input[type=file] element")
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

				err := page.SetContent(pageContent, nil)
				require.NoError(t, err)

				getFileCountFn := func() interface{} {
					v, err := page.Evaluate(`() => document.getElementById("upload").files.length`)
					require.NoError(t, err)

					return v
				}

				getFilePropertyFn := func(idx int, propName string) interface{} {
					v, err := page.Evaluate(
						`(idx, propName) => document.getElementById("upload").files[idx][propName]`,
						idx,
						propName)
					require.NoError(t, err)
					return v
				}

				files, cleanup := tc.setup(tb)
				if cleanup != nil {
					defer cleanup()
				}
				err = test(tb, page, files)
				tc.check(t, getFileCountFn, getFilePropertyFn, err)
			})
		}
	}
}
