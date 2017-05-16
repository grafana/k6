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
}
