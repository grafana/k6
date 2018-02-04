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
	"testing"

	"github.com/stretchr/testify/assert"
	null "gopkg.in/guregu/null.v3"
)

func TestAnonymizePath(t *testing.T) {
	testdata := map[string]string{
		"/tmp":                                      "/tmp",
		"/tmp/":                                     "/tmp/",
		"/tmp/myfile.txt":                           "/tmp/myfile.txt",
		"/home/myname":                              "/home/nobody",
		"/home/myname/":                             "/home/nobody/",
		"/home/myname/myfile.txt":                   "/home/nobody/myfile.txt",
		"/Users/myname/myfile.txt":                  "/Users/nobody/myfile.txt",
		"/Documents and Settings/myname/myfile.txt": "/Documents and Settings/nobody/myfile.txt",
	}
	for from, to := range testdata {
		t.Run("path="+from, func(t *testing.T) {
			assert.Equal(t, to, AnonymizePath(from))
		})
	}
}

func TestArchiveReadWrite(t *testing.T) {
	t.Run("Roundtrip", func(t *testing.T) {
		arc1 := &Archive{
			Type:     "js",
			Options:  Options{VUs: null.IntFrom(12345)},
			Filename: "/path/to/script.js",
			Data:     []byte(`// contents...`),
			Pwd:      "/path/to",
			Scripts: map[string][]byte{
				"/path/to/a.js":             []byte(`// a contents`),
				"/path/to/b.js":             []byte(`// b contents`),
				"cdnjs.com/libraries/Faker": []byte(`// faker contents`),
			},
			Files: map[string][]byte{
				"/path/to/file1.txt":                 []byte(`hi!`),
				"/path/to/file2.txt":                 []byte(`bye!`),
				"github.com/loadimpact/k6/README.md": []byte(`README`),
			},
		}

		buf := bytes.NewBuffer(nil)
		assert.NoError(t, arc1.Write(buf))

		arc2, err := ReadArchive(buf)
		assert.NoError(t, err)
		assert.Equal(t, arc1, arc2)
	})

	t.Run("Anonymized", func(t *testing.T) {
		arc1 := &Archive{
			Type:     "js",
			Options:  Options{VUs: null.IntFrom(12345)},
			Filename: "/home/myname/script.js",
			Data:     []byte(`// contents...`),
			Pwd:      "/home/myname",
			Scripts: map[string][]byte{
				"/home/myname/a.js":         []byte(`// a contents`),
				"/home/myname/b.js":         []byte(`// b contents`),
				"cdnjs.com/libraries/Faker": []byte(`// faker contents`),
			},
			Files: map[string][]byte{
				"/home/myname/file1.txt":             []byte(`hi!`),
				"/home/myname/file2.txt":             []byte(`bye!`),
				"github.com/loadimpact/k6/README.md": []byte(`README`),
			},
		}
		arc1Anon := &Archive{
			Type:     "js",
			Options:  Options{VUs: null.IntFrom(12345)},
			Filename: "/home/nobody/script.js",
			Data:     []byte(`// contents...`),
			Pwd:      "/home/nobody",
			Scripts: map[string][]byte{
				"/home/nobody/a.js":         []byte(`// a contents`),
				"/home/nobody/b.js":         []byte(`// b contents`),
				"cdnjs.com/libraries/Faker": []byte(`// faker contents`),
			},
			Files: map[string][]byte{
				"/home/nobody/file1.txt":             []byte(`hi!`),
				"/home/nobody/file2.txt":             []byte(`bye!`),
				"github.com/loadimpact/k6/README.md": []byte(`README`),
			},
		}

		buf := bytes.NewBuffer(nil)
		assert.NoError(t, arc1.Write(buf))

		arc2, err := ReadArchive(buf)
		assert.NoError(t, err)
		assert.Equal(t, arc1Anon, arc2)
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

//TODO: write test for environment variables
