package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

// TestSetInputFiles tests the SetInputFiles function.
func TestSetInputFiles(t *testing.T) {
	t.Parallel()

	type indexedFn func(idx int, propName string) any
	type testFn func(page *common.Page, files *common.Files) error
	type setupFn func() *common.Files
	type checkFn func(t *testing.T,
		getFileCountFn func() any,
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

	defaultTestPage := func(page *common.Page, files *common.Files) error {
		return page.SetInputFiles("#upload", files, common.NewFrameSetInputFilesOptions(common.DefaultTimeout))
	}
	defaultTestElementHandle := func(page *common.Page, files *common.Files) error {
		handle, err := page.WaitForSelector("#upload", common.NewFrameWaitForSelectorOptions(common.DefaultTimeout))
		assert.NoError(t, err)
		return handle.SetInputFiles(files, common.NewElementHandleSetInputFilesOptions(common.DefaultTimeout))
	}

	testCases := []struct {
		name  string
		setup setupFn
		tests []testFn
		check checkFn
	}{
		{
			name: "set_one_file_with_object",
			setup: func() *common.Files {
				return &common.Files{
					Payload: []*common.File{
						{
							Name:     "test.json",
							Mimetype: "text/json",
							Buffer:   "MDEyMzQ1Njc4OQ==",
						},
					},
				}
			},
			tests: []testFn{defaultTestPage, defaultTestElementHandle},
			check: func(t *testing.T, getFileCountFn func() any, getFilePropFn indexedFn, err error) {
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
			setup: func() *common.Files {
				return &common.Files{
					Payload: []*common.File{
						{
							Name:     "test.json",
							Mimetype: "text/json",
							Buffer:   "MDEyMzQ1Njc4OQ==",
						},
						{
							Name:     "test.xml",
							Mimetype: "text/xml",
							Buffer:   "MDEyMzQ1Njc4OTAxMjM0",
						},
					},
				}
			},
			tests: []testFn{defaultTestPage, defaultTestElementHandle},
			check: func(t *testing.T, getFileCountFn func() any, getFilePropFn indexedFn, err error) {
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
			setup: func() *common.Files {
				return nil
			},
			tests: []testFn{defaultTestPage, defaultTestElementHandle},
			check: func(t *testing.T, getFileCountFn func() any, _ indexedFn, err error) {
				t.Helper()
				assert.NoError(t, err)
				// check if input has 1 file
				assert.Equal(t, float64(0), getFileCountFn())
			},
		},
		{
			name: "test_injected_script_notinput",
			setup: func() *common.Files {
				return &common.Files{
					Payload: []*common.File{
						{
							Name:     "test.json",
							Mimetype: "text/json",
							Buffer:   "MDEyMzQ1Njc4OQ==",
						},
					},
				}
			},
			tests: []testFn{
				func(page *common.Page, files *common.Files) error {
					return page.SetInputFiles("#button1", files, common.NewFrameSetInputFilesOptions(common.DefaultTimeout))
				},
				func(page *common.Page, files *common.Files) error {
					handle, err := page.WaitForSelector("#button1", common.NewFrameWaitForSelectorOptions(common.DefaultTimeout))
					assert.NoError(t, err)
					return handle.SetInputFiles(files, common.NewElementHandleSetInputFilesOptions(common.DefaultTimeout))
				},
			},
			check: func(t *testing.T, getFileCountFn func() any, _ indexedFn, err error) {
				t.Helper()
				assert.ErrorContains(t, err, "node is not an HTMLInputElement")
				assert.ErrorContains(t, err, "setting input files")
				// check if input has 0 file
				assert.Equal(t, float64(0), getFileCountFn())
			},
		},
		{
			name: "test_injected_script_notfile",
			setup: func() *common.Files {
				return &common.Files{
					Payload: []*common.File{
						{
							Name:     "test.json",
							Mimetype: "text/json",
							Buffer:   "MDEyMzQ1Njc4OQ==",
						},
					},
				}
			},
			tests: []testFn{
				func(page *common.Page, files *common.Files) error {
					return page.SetInputFiles("#textinput", files, common.NewFrameSetInputFilesOptions(common.DefaultTimeout))
				},
				func(page *common.Page, files *common.Files) error {
					handle, err := page.WaitForSelector("#textinput", common.NewFrameWaitForSelectorOptions(common.DefaultTimeout))
					assert.NoError(t, err)
					return handle.SetInputFiles(files, common.NewElementHandleSetInputFilesOptions(common.DefaultTimeout))
				},
			},
			check: func(t *testing.T, getFileCountFn func() any, _ indexedFn, err error) {
				t.Helper()
				assert.ErrorContains(t, err, "node is not an input[type=file] element")
				assert.ErrorContains(t, err, "setting input files")
				// check if input has 0 file
				assert.Equal(t, float64(0), getFileCountFn())
			},
		},
	}

	for _, tc := range testCases {
		for _, test := range tc.tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				tb := newTestBrowser(t)
				defer tb.Browser.Close()
				page := tb.NewPage(nil)

				err := page.SetContent(pageContent, nil)
				require.NoError(t, err)

				getFileCountFn := func() any {
					v, err := page.Evaluate(`() => document.getElementById("upload").files.length`)
					require.NoError(t, err)

					return v
				}

				getFilePropertyFn := func(idx int, propName string) any {
					v, err := page.Evaluate(
						`(idx, propName) => document.getElementById("upload").files[idx][propName]`,
						idx,
						propName)
					require.NoError(t, err)
					return v
				}

				err = test(page, tc.setup())
				tc.check(t, getFileCountFn, getFilePropertyFn, err)
			})
		}
	}
}
