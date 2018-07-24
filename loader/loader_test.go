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
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/loadimpact/k6/lib/testutils"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestDir(t *testing.T) {
	testdata := map[string]string{
		"/path/to/file.txt": filepath.FromSlash("/path/to"),
		"-":                 "/",
	}
	for name, dir := range testdata {
		t.Run("path="+name, func(t *testing.T) {
			assert.Equal(t, dir, Dir(name))
		})
	}
}

func TestLoad(t *testing.T) {
	tb := testutils.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	oldHTTPTransport := http.DefaultTransport
	http.DefaultTransport = tb.HTTPTransport

	defer func() {
		tb.Cleanup()
		http.DefaultTransport = oldHTTPTransport
	}()

	t.Run("Blank", func(t *testing.T) {
		_, err := Load(nil, "/", "")
		assert.EqualError(t, err, "local or remote path required")
	})

	t.Run("Protocol", func(t *testing.T) {
		_, err := Load(nil, "/", sr("HTTPSBIN_URL/html"))
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
			path := filepath.FromSlash("/nonexistent")
			_, err := Load(fs, "/", "/nonexistent")
			assert.EqualError(t, err, fmt.Sprintf("open %s: file does not exist", path))
		})

		t.Run("Remote Lifting Denied", func(t *testing.T) {
			_, err := Load(fs, "example.com", "/etc/shadow")
			assert.EqualError(t, err, "origin (example.com) not allowed to load local file: /etc/shadow")
		})
	})

	t.Run("Remote", func(t *testing.T) {
		src, err := Load(nil, "/", sr("HTTPSBIN_DOMAIN:HTTPSBIN_PORT/html"))
		if assert.NoError(t, err) {
			assert.Equal(t, src.Filename, sr("HTTPSBIN_DOMAIN:HTTPSBIN_PORT/html"))
			assert.Contains(t, string(src.Data), "Herman Melville - Moby-Dick")
		}

		t.Run("Absolute", func(t *testing.T) {
			src, err := Load(nil, sr("HTTPSBIN_DOMAIN:HTTPSBIN_PORT"), sr("HTTPSBIN_DOMAIN:HTTPSBIN_PORT/robots.txt"))
			if assert.NoError(t, err) {
				assert.Equal(t, src.Filename, sr("HTTPSBIN_DOMAIN:HTTPSBIN_PORT/robots.txt"))
				assert.Equal(t, string(src.Data), "User-agent: *\nDisallow: /deny\n")
			}
		})

		t.Run("Relative", func(t *testing.T) {
			src, err := Load(nil, sr("HTTPSBIN_DOMAIN:HTTPSBIN_PORT"), "./robots.txt")
			if assert.NoError(t, err) {
				assert.Equal(t, src.Filename, sr("HTTPSBIN_DOMAIN:HTTPSBIN_PORT/robots.txt"))
				assert.Equal(t, string(src.Data), "User-agent: *\nDisallow: /deny\n")
			}
		})
	})

	const responseStr = "export function fn() {\r\n    return 1234;\r\n}"
	tb.Mux.HandleFunc("/raw/something", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.URL.Query()["_k6"]; ok {
			http.Error(w, "Internal server error", 500)
			return
		}
		_, err := fmt.Fprint(w, responseStr)
		assert.NoError(t, err)
	})

	t.Run("No _k6=1 Fallback", func(t *testing.T) {
		src, err := Load(nil, "/", sr("HTTPSBIN_DOMAIN:HTTPSBIN_PORT/raw/something"))
		if assert.NoError(t, err) {
			assert.Equal(t, src.Filename, sr("HTTPSBIN_DOMAIN:HTTPSBIN_PORT/raw/something"))
			assert.Equal(t, responseStr, string(src.Data))
		}
	})

	tb.Mux.HandleFunc("/invalid", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal server error", 500)
	})

	t.Run("Invalid", func(t *testing.T) {
		src, err := Load(nil, "/", sr("HTTPSBIN_DOMAIN:HTTPSBIN_PORT/invalid"))
		assert.Nil(t, src)
		assert.Error(t, err)

		t.Run("Host", func(t *testing.T) {
			src, err := Load(nil, "/", "some-path-that-doesnt-exist.js")
			assert.Nil(t, src)
			assert.Error(t, err)
		})
		t.Run("URL", func(t *testing.T) {
			src, err := Load(nil, "/", "192.168.0.%31")
			assert.Nil(t, src)
			assert.Error(t, err)
		})
	})
}
