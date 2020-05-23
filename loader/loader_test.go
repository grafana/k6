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

package loader_test

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
	"github.com/loadimpact/k6/loader"
)

func TestDir(t *testing.T) {
	testdata := map[string]string{
		"/path/to/file.txt": filepath.FromSlash("/path/to/"),
		"-":                 "/",
	}
	for name, dir := range testdata {
		nameURL := &url.URL{Scheme: "file", Path: name}
		dirURL := &url.URL{Scheme: "file", Path: filepath.ToSlash(dir)}
		t.Run("path="+name, func(t *testing.T) {
			assert.Equal(t, dirURL, loader.Dir(nameURL))
		})
	}
}

func TestResolve(t *testing.T) {

	t.Run("Blank", func(t *testing.T) {
		_, err := loader.Resolve(nil, "")
		assert.EqualError(t, err, "local or remote path required")
	})

	t.Run("Protocol", func(t *testing.T) {
		root, err := url.Parse("file:///")
		require.NoError(t, err)

		t.Run("Missing", func(t *testing.T) {
			u, err := loader.Resolve(root, "example.com/html")
			require.NoError(t, err)
			assert.Equal(t, u.String(), "//example.com/html")
			// TODO: check that warning will be emitted if Loaded
		})
		t.Run("WS", func(t *testing.T) {
			moduleSpecifier := "ws://example.com/html"
			_, err := loader.Resolve(root, moduleSpecifier)
			assert.EqualError(t, err,
				"only supported schemes for imports are file and https, "+moduleSpecifier+" has `ws`")
		})

		t.Run("HTTP", func(t *testing.T) {
			moduleSpecifier := "http://example.com/html"
			_, err := loader.Resolve(root, moduleSpecifier)
			assert.EqualError(t, err,
				"only supported schemes for imports are file and https, "+moduleSpecifier+" has `http`")
		})
	})

	t.Run("Remote Lifting Denied", func(t *testing.T) {
		pwdURL, err := url.Parse("https://example.com/")
		require.NoError(t, err)

		_, err = loader.Resolve(pwdURL, "file:///etc/shadow")
		assert.EqualError(t, err, "origin (https://example.com/) not allowed to load local file: file:///etc/shadow")
	})

	t.Run("Fixes missing slash in pwd", func(t *testing.T) {
		pwdURL, err := url.Parse("https://example.com/path/to")
		require.NoError(t, err)

		moduleURL, err := loader.Resolve(pwdURL, "./something")
		require.NoError(t, err)
		require.Equal(t, "https://example.com/path/to/something", moduleURL.String())
		require.Equal(t, "https://example.com/path/to", pwdURL.String())
	})

}
func TestLoad(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	oldHTTPTransport := http.DefaultTransport
	http.DefaultTransport = tb.HTTPTransport

	defer func() {
		tb.Cleanup()
		http.DefaultTransport = oldHTTPTransport
	}()

	t.Run("Local", func(t *testing.T) {
		filesystems := make(map[string]afero.Fs)
		filesystems["file"] = afero.NewMemMapFs()
		assert.NoError(t, filesystems["file"].MkdirAll("/path/to", 0755))
		assert.NoError(t, afero.WriteFile(filesystems["file"], "/path/to/file.txt", []byte("hi"), 0644))

		testdata := map[string]struct{ pwd, path string }{
			"Absolute": {"/path/", "/path/to/file.txt"},
			"Relative": {"/path/", "./to/file.txt"},
			"Adjacent": {"/path/to/", "./file.txt"},
		}
		for name, data := range testdata {
			data := data
			t.Run(name, func(t *testing.T) {
				pwdURL, err := url.Parse("file://" + data.pwd)
				require.NoError(t, err)

				moduleURL, err := loader.Resolve(pwdURL, data.path)
				require.NoError(t, err)

				src, err := loader.Load(filesystems, moduleURL, data.path)
				require.NoError(t, err)

				assert.Equal(t, "file:///path/to/file.txt", src.URL.String())
				assert.Equal(t, "hi", string(src.Data))
			})
		}

		t.Run("Nonexistent", func(t *testing.T) {
			root, err := url.Parse("file:///")
			require.NoError(t, err)

			path := filepath.FromSlash("/nonexistent")
			pathURL, err := loader.Resolve(root, "/nonexistent")
			require.NoError(t, err)

			_, err = loader.Load(filesystems, pathURL, path)
			require.Error(t, err)
			assert.Contains(t, err.Error(),
				fmt.Sprintf(`The moduleSpecifier "file://%s" couldn't be found on local disk. `,
					filepath.ToSlash(path)))
		})

	})

	t.Run("Remote", func(t *testing.T) {
		filesystems := map[string]afero.Fs{"https": afero.NewMemMapFs()}
		t.Run("From local", func(t *testing.T) {
			root, err := url.Parse("file:///")
			require.NoError(t, err)

			moduleSpecifier := sr("HTTPSBIN_URL/html")
			moduleSpecifierURL, err := loader.Resolve(root, moduleSpecifier)
			require.NoError(t, err)

			src, err := loader.Load(filesystems, moduleSpecifierURL, moduleSpecifier)
			require.NoError(t, err)
			assert.Equal(t, src.URL, moduleSpecifierURL)
			assert.Contains(t, string(src.Data), "Herman Melville - Moby-Dick")
		})

		t.Run("Absolute", func(t *testing.T) {
			pwdURL, err := url.Parse(sr("HTTPSBIN_URL"))
			require.NoError(t, err)

			moduleSpecifier := sr("HTTPSBIN_URL/robots.txt")
			moduleSpecifierURL, err := loader.Resolve(pwdURL, moduleSpecifier)
			require.NoError(t, err)

			src, err := loader.Load(filesystems, moduleSpecifierURL, moduleSpecifier)
			require.NoError(t, err)
			assert.Equal(t, src.URL.String(), sr("HTTPSBIN_URL/robots.txt"))
			assert.Equal(t, string(src.Data), "User-agent: *\nDisallow: /deny\n")
		})

		t.Run("Relative", func(t *testing.T) {
			pwdURL, err := url.Parse(sr("HTTPSBIN_URL"))
			require.NoError(t, err)

			moduleSpecifier := ("./robots.txt")
			moduleSpecifierURL, err := loader.Resolve(pwdURL, moduleSpecifier)
			require.NoError(t, err)

			src, err := loader.Load(filesystems, moduleSpecifierURL, moduleSpecifier)
			require.NoError(t, err)
			assert.Equal(t, sr("HTTPSBIN_URL/robots.txt"), src.URL.String())
			assert.Equal(t, "User-agent: *\nDisallow: /deny\n", string(src.Data))
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
		root, err := url.Parse("file:///")
		require.NoError(t, err)

		moduleSpecifier := sr("HTTPSBIN_URL/raw/something")
		moduleSpecifierURL, err := loader.Resolve(root, moduleSpecifier)
		require.NoError(t, err)

		filesystems := map[string]afero.Fs{"https": afero.NewMemMapFs()}
		src, err := loader.Load(filesystems, moduleSpecifierURL, moduleSpecifier)

		require.NoError(t, err)
		assert.Equal(t, src.URL.String(), sr("HTTPSBIN_URL/raw/something"))
		assert.Equal(t, responseStr, string(src.Data))
	})

	tb.Mux.HandleFunc("/invalid", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal server error", 500)
	})

	t.Run("Invalid", func(t *testing.T) {
		root, err := url.Parse("file:///")
		require.NoError(t, err)

		t.Run("IP URL", func(t *testing.T) {
			_, err := loader.Resolve(root, "192.168.0.%31")
			require.Error(t, err)
			require.Contains(t, err.Error(), `invalid URL escape "%31"`)
		})

		var testData = [...]struct {
			name, moduleSpecifier string
		}{
			{"URL", sr("HTTPSBIN_URL/invalid")},
			{"HOST", "some-path-that-doesnt-exist.js"},
		}

		filesystems := map[string]afero.Fs{"https": afero.NewMemMapFs()}
		for _, data := range testData {
			moduleSpecifier := data.moduleSpecifier
			t.Run(data.name, func(t *testing.T) {
				moduleSpecifierURL, err := loader.Resolve(root, moduleSpecifier)
				require.NoError(t, err)

				_, err = loader.Load(filesystems, moduleSpecifierURL, moduleSpecifier)
				require.Error(t, err)
			})
		}
	})
}
