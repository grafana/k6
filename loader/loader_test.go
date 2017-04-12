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

package loader

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	t.Run("Blank", func(t *testing.T) {
		_, err := Load(nil, "/", "")
		assert.EqualError(t, err, "local or remote path required")
	})

	t.Run("Protocol", func(t *testing.T) {
		_, err := Load(nil, "/", "https://httpbin.org/html")
		assert.EqualError(t, err, "imports should not contain a protocol")
	})

	t.Run("Local", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		assert.NoError(t, fs.MkdirAll("/path/to", 0755))
		assert.NoError(t, afero.WriteFile(fs, "/path/to/file.txt", []byte("hi"), 0644))

		testdata := map[string]struct{ pwd, path string }{
			"Absolute": {"/path", "/path/to/file.txt"},
			"Relative": {"/path", "./to/file.txt"},
			"Adjacent": {"/path/to", "./file.txt"},
		}
		for name, data := range testdata {
			t.Run(name, func(t *testing.T) {
				src, err := Load(fs, data.pwd, data.path)
				if assert.NoError(t, err) {
					assert.Equal(t, "/path/to/file.txt", src.Filename)
					assert.Equal(t, "hi", string(src.Data))
				}
			})
		}

		t.Run("Nonexistent", func(t *testing.T) {
			_, err := Load(fs, "/", "/nonexistent")
			assert.EqualError(t, err, "open /nonexistent: file does not exist")
		})

		t.Run("Remote Lifting Denied", func(t *testing.T) {
			_, err := Load(fs, "example.com", "/etc/shadow")
			assert.EqualError(t, err, "origin (example.com) not allowed to load local file: /etc/shadow")
		})
	})

	t.Run("Remote", func(t *testing.T) {
		src, err := Load(nil, "/", "httpbin.org/html")
		if assert.NoError(t, err) {
			assert.Equal(t, src.Filename, "httpbin.org/html")
			assert.Contains(t, string(src.Data), "Herman Melville - Moby-Dick")
		}

		t.Run("Absolute", func(t *testing.T) {
			src, err := Load(nil, "httpbin.org", "httpbin.org/robots.txt")
			if assert.NoError(t, err) {
				assert.Equal(t, src.Filename, "httpbin.org/robots.txt")
				assert.Equal(t, string(src.Data), "User-agent: *\nDisallow: /deny\n")
			}
		})

		t.Run("Relative", func(t *testing.T) {
			src, err := Load(nil, "httpbin.org", "./robots.txt")
			if assert.NoError(t, err) {
				assert.Equal(t, src.Filename, "httpbin.org/robots.txt")
				assert.Equal(t, string(src.Data), "User-agent: *\nDisallow: /deny\n")
			}
		})
	})
}
