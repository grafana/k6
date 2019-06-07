/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package lib

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"
)

func TestNormalizeAndAnonymizePath(t *testing.T) {
	testdata := map[string]string{
		"/tmp":                            "/tmp",
		"/tmp/myfile.txt":                 "/tmp/myfile.txt",
		"/home/myname":                    "/home/nobody",
		"/home/myname/foo/bar/myfile.txt": "/home/nobody/foo/bar/myfile.txt",
		"/Users/myname/myfile.txt":        "/Users/nobody/myfile.txt",
		"/Documents and Settings/myname/myfile.txt":           "/Documents and Settings/nobody/myfile.txt",
		"\\\\MYSHARED\\dir\\dir\\myfile.txt":                  "/nobody/dir/dir/myfile.txt",
		"\\NOTSHARED\\dir\\dir\\myfile.txt":                   "/NOTSHARED/dir/dir/myfile.txt",
		"C:\\Users\\myname\\dir\\myfile.txt":                  "/C/Users/nobody/dir/myfile.txt",
		"D:\\Documents and Settings\\myname\\dir\\myfile.txt": "/D/Documents and Settings/nobody/dir/myfile.txt",
	}
	// TODO: fix this - the issue is that filepath.Clean replaces `/` with whatever the path
	// separator is on the current OS and as such this gets confused for shared folder on
	// windows :( https://github.com/golang/go/issues/16111
	if runtime.GOOS != "windows" {
		testdata["//etc/hosts"] = "/etc/hosts"
	}
	for from, to := range testdata {
		from, to := from, to
		t.Run("path="+from, func(t *testing.T) {
			res := NormalizeAndAnonymizePath(from)
			assert.Equal(t, to, res)
			assert.Equal(t, res, NormalizeAndAnonymizePath(res))
		})
	}
}

func makeMemMapFs(t *testing.T, input map[string][]byte) afero.Fs {
	fs := afero.NewMemMapFs()
	for path, data := range input {
		require.NoError(t, afero.WriteFile(fs, path, data, 0644))
	}
	return fs
}

func getMapKeys(m map[string]afero.Fs) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	return keys
}

func diffMapFSes(t *testing.T, first, second map[string]afero.Fs) bool {
	require.ElementsMatch(t, getMapKeys(first), getMapKeys(second), "fs map keys don't match")
	for key, fs := range first {
		secondFs := second[key]
		diffFSes(t, fs, secondFs)
	}

	return true
}

func diffFSes(t *testing.T, first, second afero.Fs) {
	diffFSesDir(t, first, second, "/")
}

func getInfoNames(infos []os.FileInfo) []string {
	var names = make([]string, len(infos))
	for i, info := range infos {
		names[i] = info.Name()
	}
	return names
}

func diffFSesDir(t *testing.T, first, second afero.Fs, dirname string) {
	firstInfos, err := afero.ReadDir(first, dirname)
	require.NoError(t, err, dirname)

	secondInfos, err := afero.ReadDir(first, dirname)
	require.NoError(t, err, dirname)

	require.ElementsMatch(t, getInfoNames(firstInfos), getInfoNames(secondInfos), "directory: "+dirname)
	for _, info := range firstInfos {
		path := dirname + "/" + info.Name()
		if info.IsDir() {
			diffFSesDir(t, first, second, path)
			continue
		}
		firstData, err := afero.ReadFile(first, path)
		require.NoError(t, err, path)

		secondData, err := afero.ReadFile(second, path)
		require.NoError(t, err, path)

		assert.Equal(t, firstData, secondData, path)
	}
}

func TestArchiveReadWrite(t *testing.T) {
	t.Run("Roundtrip", func(t *testing.T) {
		arc1 := &Archive{
			Type: "js",
			Options: Options{
				VUs:        null.IntFrom(12345),
				SystemTags: GetTagSet(DefaultSystemTagList...),
			},
			Filename: "/path/to/script.js",
			Data:     []byte(`// contents...`),
			Pwd:      "/path/to",
			FSes: map[string]afero.Fs{
				"file": makeMemMapFs(t, map[string][]byte{
					"/path/to/a.js":      []byte(`// a contents`),
					"/path/to/b.js":      []byte(`// b contents`),
					"/path/to/file1.txt": []byte(`hi!`),
					"/path/to/file2.txt": []byte(`bye!`),
				}),
				"https": makeMemMapFs(t, map[string][]byte{
					"/cdnjs.com/libraries/Faker":          []byte(`// faker contents`),
					"/github.com/loadimpact/k6/README.md": []byte(`README`),
				}),
			},
		}

		buf := bytes.NewBuffer(nil)
		require.NoError(t, arc1.Write(buf))

		arc1FSes := arc1.FSes
		arc1.FSes = nil

		arc2, err := ReadArchive(buf)
		require.NoError(t, err)

		// arc2.FSes["file"] = arc2.FSes["file"].(*normalizedFS).Fs

		arc2FSes := arc2.FSes
		arc2.FSes = nil

		assert.Equal(t, arc1, arc2)

		diffMapFSes(t, arc1FSes, arc2FSes)
	})

	t.Run("Anonymized", func(t *testing.T) {
		testdata := []struct {
			Pwd, PwdNormAnon string
		}{
			{"/home/myname", "/home/nobody"},
			{"/C:/Users/Administrator", "/C/Users/nobody"},
		}
		for _, entry := range testdata {
			arc1 := &Archive{
				Type: "js",
				Options: Options{
					VUs:        null.IntFrom(12345),
					SystemTags: GetTagSet(DefaultSystemTagList...),
				},
				Filename: fmt.Sprintf("%s/script.js", entry.Pwd),
				Data:     []byte(`// contents...`),
				Pwd:      entry.Pwd,
				FSes: map[string]afero.Fs{
					"file": makeMemMapFs(t, map[string][]byte{
						fmt.Sprintf("%s/a.js", entry.Pwd):      []byte(`// a contents`),
						fmt.Sprintf("%s/b.js", entry.Pwd):      []byte(`// b contents`),
						fmt.Sprintf("%s/file1.txt", entry.Pwd): []byte(`hi!`),
						fmt.Sprintf("%s/file2.txt", entry.Pwd): []byte(`bye!`),
					}),
					"https": makeMemMapFs(t, map[string][]byte{
						"/cdnjs.com/libraries/Faker":          []byte(`// faker contents`),
						"/github.com/loadimpact/k6/README.md": []byte(`README`),
					}),
				},
			}
			arc1Anon := &Archive{
				Type: "js",
				Options: Options{
					VUs:        null.IntFrom(12345),
					SystemTags: GetTagSet(DefaultSystemTagList...),
				},
				Filename: fmt.Sprintf("%s/script.js", entry.PwdNormAnon),
				Data:     []byte(`// contents...`),
				Pwd:      entry.PwdNormAnon,

				FSes: map[string]afero.Fs{
					"file": makeMemMapFs(t, map[string][]byte{
						fmt.Sprintf("%s/a.js", entry.PwdNormAnon):      []byte(`// a contents`),
						fmt.Sprintf("%s/b.js", entry.PwdNormAnon):      []byte(`// b contents`),
						fmt.Sprintf("%s/file1.txt", entry.PwdNormAnon): []byte(`hi!`),
						fmt.Sprintf("%s/file2.txt", entry.PwdNormAnon): []byte(`bye!`),
					}),
					"https": makeMemMapFs(t, map[string][]byte{
						"/cdnjs.com/libraries/Faker":          []byte(`// faker contents`),
						"/github.com/loadimpact/k6/README.md": []byte(`README`),
					}),
				},
			}

			buf := bytes.NewBuffer(nil)
			assert.NoError(t, arc1.Write(buf))

			arc1FSes := arc1Anon.FSes
			arc1Anon.FSes = nil

			arc2, err := ReadArchive(buf)
			assert.NoError(t, err)

			arc2FSes := arc2.FSes
			arc2.FSes = nil

			assert.Equal(t, arc1Anon, arc2)
			diffMapFSes(t, arc1FSes, arc2FSes)
		}
	})
}

func TestArchiveJSONEscape(t *testing.T) {
	t.Parallel()

	arc := &Archive{}
	arc.Filename = "test<.js"
	b, err := arc.json()
	assert.NoError(t, err)
	assert.Contains(t, string(b), "test<.js")
}
