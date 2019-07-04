package lib

import (
	"archive/tar"
	"bytes"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/loadimpact/k6/lib/fsext"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func dumpMakeMapFsToBuf(fs afero.Fs) (*bytes.Buffer, error) {
	var b = bytes.NewBuffer(nil)
	var w = tar.NewWriter(b)
	err := fsext.Walk(fs, afero.FilePathSeparator,
		filepath.WalkFunc(func(filePath string, info os.FileInfo, err error) error {
			if filePath == afero.FilePathSeparator {
				return nil // skip the root
			}
			if err != nil {
				return err
			}
			if info.IsDir() {
				return w.WriteHeader(&tar.Header{
					Name:     path.Clean(filepath.ToSlash(filePath)[1:]),
					Mode:     0555,
					Typeflag: tar.TypeDir,
				})
			}
			var data []byte
			data, err = afero.ReadFile(fs, filePath)
			if err != nil {
				return err
			}
			err = w.WriteHeader(&tar.Header{
				Name:     path.Clean(filepath.ToSlash(filePath)[1:]),
				Mode:     0644,
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
	fs := makeMemMapFs(t, map[string][]byte{
		// files
		"/files/github.com/loadimpact/k6/samples/example.js": []byte(`github file`),
		"/files/cdnjs.com/packages/Faker":                    []byte(`faker file`),
		"/files/example.com/path/to.js":                      []byte(`example.com file`),
		"/files/_/C/something/path":                          []byte(`windows file`),
		"/files/_/absolulte/path":                            []byte(`unix file`),

		// scripts
		"/scripts/github.com/loadimpact/k6/samples/example.js2": []byte(`github script`),
		"/scripts/cdnjs.com/packages/Faker2":                    []byte(`faker script`),
		"/scripts/example.com/path/too.js":                      []byte(`example.com script`),
		"/scripts/_/C/something/path2":                          []byte(`windows script`),
		"/scripts/_/absolulte/path2":                            []byte(`unix script`),
	})
	buf, err := dumpMakeMapFsToBuf(fs)
	require.NoError(t, err)

	var (
		expectedFilesystems = map[string]afero.Fs{
			"file": makeMemMapFs(t, map[string][]byte{
				"/C:/something/path":  []byte(`windows file`),
				"/absolulte/path":     []byte(`unix file`),
				"/C:/something/path2": []byte(`windows script`),
				"/absolulte/path2":    []byte(`unix script`),
			}),
			"https": makeMemMapFs(t, map[string][]byte{
				"/example.com/path/to.js":                       []byte(`example.com file`),
				"/example.com/path/too.js":                      []byte(`example.com script`),
				"/github.com/loadimpact/k6/samples/example.js":  []byte(`github file`),
				"/cdnjs.com/packages/Faker":                     []byte(`faker file`),
				"/github.com/loadimpact/k6/samples/example.js2": []byte(`github script`),
				"/cdnjs.com/packages/Faker2":                    []byte(`faker script`),
			}),
		}
	)

	arc, err := ReadArchive(buf)
	require.NoError(t, err)

	diffMapFilesystems(t, expectedFilesystems, arc.Filesystems)
}

func TestUnknownPrefix(t *testing.T) {
	fs := makeMemMapFs(t, map[string][]byte{
		"/strange/something": []byte(`github file`),
	})
	buf, err := dumpMakeMapFsToBuf(fs)
	require.NoError(t, err)

	_, err = ReadArchive(buf)
	require.Error(t, err)
	require.Equal(t, err.Error(),
		"unknown file prefix `strange` for file `strange/something`")
}
