package lib

import (
	"archive/tar"
	"bytes"
	"io/fs"
	"net/url"
	"path"
	"path/filepath"
	"testing"

	"go.k6.io/k6/internal/lib/testutils"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib/fsext"
)

func dumpMemMapFsToBuf(fileSystem fsext.Fs) (*bytes.Buffer, error) {
	b := bytes.NewBuffer(nil)
	w := tar.NewWriter(b)
	err := fsext.Walk(fileSystem, fsext.FilePathSeparator,
		filepath.WalkFunc(func(filePath string, info fs.FileInfo, err error) error {
			if filePath == fsext.FilePathSeparator {
				return nil // skip the root
			}
			if err != nil {
				return err
			}
			if info.IsDir() {
				return w.WriteHeader(&tar.Header{
					Name:     path.Clean(filepath.ToSlash(filePath)[1:]),
					Mode:     0o555,
					Typeflag: tar.TypeDir,
				})
			}
			var data []byte
			data, err = fsext.ReadFile(fileSystem, filePath)
			if err != nil {
				return err
			}
			err = w.WriteHeader(&tar.Header{
				Name:     path.Clean(filepath.ToSlash(filePath)[1:]),
				Mode:     0o644,
				Size:     int64(len(data)),
				Typeflag: tar.TypeReg,
			})
			if err != nil {
				return err
			}
			_, err = w.Write(data)
			if err != nil {
				return err
			}
			return nil
		}))
	if err != nil {
		return nil, err
	}
	return b, w.Close()
}

func TestOldArchive(t *testing.T) {
	t.Parallel()
	testCases := map[string]string{
		// map of filename to data for each main file tested
		"C:/something/path2": `windows script`,
		"/absolulte/path2":   `unix script`,
	}
	for filename, data := range testCases {
		filename, data := filename, data
		t.Run(filename, func(t *testing.T) {
			t.Parallel()
			metadata := `{"filename": "` + filename + `", "options": {}}`
			fs := testutils.MakeMemMapFs(t, map[string][]byte{
				// files
				"/files/example.com/path/to.js": []byte(`example.com file`),
				"/files/_/C/something/path":     []byte(`windows file`),
				"/files/_/absolulte/path":       []byte(`unix file`),

				// scripts
				"/scripts/example.com/path/too.js": []byte(`example.com script`),
				"/scripts/_/C/something/path2":     []byte(`windows script`),
				"/scripts/_/absolulte/path2":       []byte(`unix script`),
				"/data":                            []byte(data),
				"/metadata.json":                   []byte(metadata),
			})

			buf, err := dumpMemMapFsToBuf(fs)
			require.NoError(t, err)

			expectedFilesystems := map[string]fsext.Fs{
				"file": testutils.MakeMemMapFs(t, map[string][]byte{
					"/C:/something/path":  []byte(`windows file`),
					"/absolulte/path":     []byte(`unix file`),
					"/C:/something/path2": []byte(`windows script`),
					"/absolulte/path2":    []byte(`unix script`),
				}),
				"https": testutils.MakeMemMapFs(t, map[string][]byte{
					"/example.com/path/to.js":  []byte(`example.com file`),
					"/example.com/path/too.js": []byte(`example.com script`),
				}),
			}

			arc, err := ReadArchive(buf)
			require.NoError(t, err)

			diffMapFilesystems(t, expectedFilesystems, arc.Filesystems)
		})
	}
}

func TestUnknownPrefix(t *testing.T) {
	t.Parallel()
	fs := testutils.MakeMemMapFs(t, map[string][]byte{
		"/strange/something": []byte(`github file`),
	})
	buf, err := dumpMemMapFsToBuf(fs)
	require.NoError(t, err)

	_, err = ReadArchive(buf)
	require.Error(t, err)
	require.Equal(t, err.Error(),
		"unknown file prefix `strange` for file `strange/something`")
}

func TestFilenamePwdResolve(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Filename, Pwd, version              string
		expectedFilenameURL, expectedPwdURL *url.URL
		expectedError                       string
	}{
		{
			Filename:            "/home/nobody/something.js",
			Pwd:                 "/home/nobody",
			expectedFilenameURL: &url.URL{Scheme: "file", Path: "/home/nobody/something.js"},
			expectedPwdURL:      &url.URL{Scheme: "file", Path: "/home/nobody"},
		},
		{
			Filename:      "github.com/k6io/k6/samples/http2.js",
			Pwd:           "github.com/k6io/k6/samples",
			expectedError: "are no longer supported",
		},
		{
			Filename:      "cdnjs.com/libraries/Faker",
			Pwd:           "/home/nobody",
			expectedError: "are no longer supported",
		},
		{
			Filename:            "https://example.com/something/dot.js",
			Pwd:                 "https://example.com/something",
			expectedFilenameURL: &url.URL{Host: "example.com", Scheme: "https", Path: "/something/dot.js"},
			expectedPwdURL:      &url.URL{Host: "example.com", Scheme: "https", Path: "/something"},
			version:             "0.25.0",
		},
		{
			Filename:      "ftps://example.com/something/dot.js",
			Pwd:           "https://example.com/something",
			expectedError: "only supported schemes for imports are file and https",
			version:       "0.25.0",
		},
		{
			Filename:      "https://example.com/something/dot.js",
			Pwd:           "ftps://example.com/something",
			expectedError: "only supported schemes for imports are file and https",
			version:       "0.25.0",
		},
	}

	for _, test := range tests {
		metadata := `{
		"filename": "` + test.Filename + `",
		"pwd": "` + test.Pwd + `",
		"k6version": "` + test.version + `",
		"options": {}
	}`

		buf, err := dumpMemMapFsToBuf(testutils.MakeMemMapFs(t, map[string][]byte{
			"/metadata.json": []byte(metadata),
		}))
		require.NoError(t, err)

		arc, err := ReadArchive(buf)
		if test.expectedError != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), test.expectedError)
		} else {
			require.NoError(t, err)
			require.Equal(t, test.expectedFilenameURL, arc.FilenameURL)
			require.Equal(t, test.expectedPwdURL, arc.PwdURL)
		}
	}
}

func TestDerivedExecutionDiscarding(t *testing.T) {
	t.Parallel()
	var emptyConfigMap ScenarioConfigs
	tests := []struct {
		metadata     string
		expScenarios interface{}
		expError     string
	}{
		// Tests to make sure that "execution" in the options, the old name for
		// "scenarios" before #1007 was merged, doesn't mess up the options...
		{
			metadata: `{
				"filename": "/test.js", "pwd": "/",
				"options": { "execution": { "something": "invalid" } }
			}`,
			expScenarios: emptyConfigMap,
		},
		{
			metadata: `{
				"filename": "/test.js", "pwd": "/",
				"k6version": "0.24.0",
				"options": { "execution": { "something": "invalid" } }
			}`,
			expScenarios: emptyConfigMap,
		},
		{
			metadata: `blah`,
			expError: "invalid character",
		},
		{
			metadata: `{
				"filename": "/test.js", "pwd": "/",
				"k6version": "0.24.0",
				"options": "something invalid"
			}`,
			expError: "cannot unmarshal string into Go struct field",
		},
		{
			metadata: `{
				"filename": "/test.js", "pwd": "/",
				"k6version": "0.25.0",
				"options": { "scenarios": { "something": "invalid" } }
			}`,
			expError: "cannot unmarshal string",
		},
		// TODO: test an actual scenarios unmarshalling, which is currently
		// impossible due to import cycles...
	}

	for _, test := range tests {
		buf, err := dumpMemMapFsToBuf(testutils.MakeMemMapFs(t, map[string][]byte{
			"/metadata.json": []byte(test.metadata),
		}))
		require.NoError(t, err)

		arc, err := ReadArchive(buf)
		if test.expError != "" {
			require.Errorf(t, err, "expected error '%s' but got nil", test.expError)
			require.Contains(t, err.Error(), test.expError)
		} else {
			require.NoError(t, err)
			require.Equal(t, test.expScenarios, arc.Options.Scenarios)
		}
	}
}
