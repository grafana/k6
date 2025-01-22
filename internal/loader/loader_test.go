package loader_test

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/lib/testutils/httpmultibin"
	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/lib/fsext"
)

func TestDir(t *testing.T) {
	t.Parallel()
	testdata := map[string]string{
		"/path/to/file.txt": filepath.FromSlash("/path/to/"),
		"-":                 "/",
	}
	for name, dir := range testdata {
		nameURL := &url.URL{Scheme: "file", Path: name}
		dirURL := &url.URL{Scheme: "file", Path: filepath.ToSlash(dir)}
		t.Run("path="+name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, dirURL, loader.Dir(nameURL))
		})
	}
}

func TestResolve(t *testing.T) {
	t.Parallel()
	t.Run("Blank", func(t *testing.T) {
		t.Parallel()
		_, err := loader.Resolve(nil, "")
		assert.EqualError(t, err, "local or remote path required")
	})

	t.Run("Protocol", func(t *testing.T) {
		t.Parallel()
		root, err := url.Parse("file:///")
		require.NoError(t, err)

		t.Run("Missing", func(t *testing.T) {
			t.Parallel()
			u, err := loader.Resolve(root, "example.com/html")
			require.ErrorContains(t, err, "The moduleSpecifier \"example.com/html\" couldn't be recognised as something k6 supports")
			require.Nil(t, u)
		})
		t.Run("WS", func(t *testing.T) {
			t.Parallel()
			moduleSpecifier := "ws://example.com/html"
			_, err := loader.Resolve(root, moduleSpecifier)
			assert.EqualError(t, err,
				"only supported schemes for imports are file and https, "+moduleSpecifier+" has `ws`")
		})

		t.Run("HTTP", func(t *testing.T) {
			t.Parallel()
			moduleSpecifier := "http://example.com/html"
			_, err := loader.Resolve(root, moduleSpecifier)
			assert.EqualError(t, err,
				"only supported schemes for imports are file and https, "+moduleSpecifier+" has `http`")
		})
	})

	t.Run("Remote Lifting Denied", func(t *testing.T) {
		t.Parallel()
		pwdURL, err := url.Parse("https://example.com/")
		require.NoError(t, err)

		_, err = loader.Resolve(pwdURL, "file:///etc/shadow")
		assert.EqualError(t, err, "origin (https://example.com/) not allowed to load local file: file:///etc/shadow")
	})

	t.Run("Fixes missing slash in pwd", func(t *testing.T) {
		t.Parallel()
		pwdURL, err := url.Parse("https://example.com/path/to")
		require.NoError(t, err)

		moduleURL, err := loader.Resolve(pwdURL, "./something")
		require.NoError(t, err)
		require.Equal(t, "https://example.com/path/to/something", moduleURL.String())
		require.Equal(t, "https://example.com/path/to", pwdURL.String())
	})
}

//nolint:tparallel // this touch the global http.DefaultTransport
func TestLoad(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))
	tb := httpmultibin.NewHTTPMultiBin(t)
	sr := tb.Replacer.Replace

	// TODO figure out a way to not replace the default transport globally so we can have this tool be in parallel and
	// then break it into separate tests instead of having one very long one.
	oldHTTPTransport := http.DefaultTransport
	http.DefaultTransport = tb.HTTPTransport
	t.Cleanup(func() {
		http.DefaultTransport = oldHTTPTransport
	})
	// All of the below handlerFuncs are here as they are used through the test
	const responseStr = "export function fn() {\r\n    return 1234;\r\n}"
	tb.Mux.HandleFunc("/raw/something", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.URL.Query()["_k6"]; ok {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		_, err := fmt.Fprint(w, responseStr)
		assert.NoError(t, err)
	})

	tb.Mux.HandleFunc("/invalid", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	})

	t.Run("Local", func(t *testing.T) {
		t.Parallel()
		testdata := map[string]struct{ pwd, path string }{
			"Absolute": {"/path/", "/path/to/file.txt"},
			"Relative": {"/path/", "./to/file.txt"},
			"Adjacent": {"/path/to/", "./file.txt"},
		}

		for name, data := range testdata {
			data := data
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				pwdURL, err := url.Parse("file://" + data.pwd)
				require.NoError(t, err)

				moduleURL, err := loader.Resolve(pwdURL, data.path)
				require.NoError(t, err)

				filesystems := make(map[string]fsext.Fs)
				filesystems["file"] = fsext.NewMemMapFs()
				assert.NoError(t, filesystems["file"].MkdirAll("/path/to", 0o755))
				assert.NoError(t, fsext.WriteFile(filesystems["file"], "/path/to/file.txt", []byte("hi"), 0o644))
				src, err := loader.Load(logger, filesystems, moduleURL, data.path)
				require.NoError(t, err)

				assert.Equal(t, "file:///path/to/file.txt", src.URL.String())
				assert.Equal(t, "hi", string(src.Data))
			})
		}

		t.Run("Nonexistent", func(t *testing.T) {
			t.Parallel()
			filesystems := make(map[string]fsext.Fs)
			filesystems["file"] = fsext.NewMemMapFs()
			assert.NoError(t, filesystems["file"].MkdirAll("/path/to", 0o755))
			assert.NoError(t, fsext.WriteFile(filesystems["file"], "/path/to/file.txt", []byte("hi"), 0o644))

			root, err := url.Parse("file:///")
			require.NoError(t, err)

			path := filepath.FromSlash("/nonexistent")
			pathURL, err := loader.Resolve(root, "/nonexistent")
			require.NoError(t, err)

			_, err = loader.Load(logger, filesystems, pathURL, path)
			require.Error(t, err)
			assert.Contains(t, err.Error(),
				fmt.Sprintf(`The moduleSpecifier "%s" couldn't be found on local disk. `,
					path))
		})
	})

	t.Run("Remote", func(t *testing.T) {
		t.Parallel()
		t.Run("From local", func(t *testing.T) {
			t.Parallel()
			filesystems := map[string]fsext.Fs{"https": fsext.NewMemMapFs()}
			root, err := url.Parse("file:///")
			require.NoError(t, err)

			moduleSpecifier := sr("HTTPSBIN_URL/html")
			moduleSpecifierURL, err := loader.Resolve(root, moduleSpecifier)
			require.NoError(t, err)

			src, err := loader.Load(logger, filesystems, moduleSpecifierURL, moduleSpecifier)
			require.NoError(t, err)
			assert.Equal(t, src.URL, moduleSpecifierURL)
			assert.Contains(t, string(src.Data), "Herman Melville - Moby-Dick")
		})

		t.Run("Absolute", func(t *testing.T) {
			t.Parallel()
			filesystems := map[string]fsext.Fs{"https": fsext.NewMemMapFs()}
			pwdURL, err := url.Parse(sr("HTTPSBIN_URL"))
			require.NoError(t, err)

			moduleSpecifier := sr("HTTPSBIN_URL/robots.txt")
			moduleSpecifierURL, err := loader.Resolve(pwdURL, moduleSpecifier)
			require.NoError(t, err)

			src, err := loader.Load(logger, filesystems, moduleSpecifierURL, moduleSpecifier)
			require.NoError(t, err)
			assert.Equal(t, src.URL.String(), sr("HTTPSBIN_URL/robots.txt"))
			assert.Equal(t, string(src.Data), "User-agent: *\nDisallow: /deny\n")
		})

		t.Run("Relative", func(t *testing.T) {
			t.Parallel()
			filesystems := map[string]fsext.Fs{"https": fsext.NewMemMapFs()}
			pwdURL, err := url.Parse(sr("HTTPSBIN_URL"))
			require.NoError(t, err)

			moduleSpecifier := ("./robots.txt")
			moduleSpecifierURL, err := loader.Resolve(pwdURL, moduleSpecifier)
			require.NoError(t, err)

			src, err := loader.Load(logger, filesystems, moduleSpecifierURL, moduleSpecifier)
			require.NoError(t, err)
			assert.Equal(t, sr("HTTPSBIN_URL/robots.txt"), src.URL.String())
			assert.Equal(t, "User-agent: *\nDisallow: /deny\n", string(src.Data))
		})
	})

	t.Run("No _k6=1 Fallback", func(t *testing.T) {
		t.Parallel()
		root, err := url.Parse("file:///")
		require.NoError(t, err)

		moduleSpecifier := sr("HTTPSBIN_URL/raw/something")
		moduleSpecifierURL, err := loader.Resolve(root, moduleSpecifier)
		require.NoError(t, err)

		filesystems := map[string]fsext.Fs{"https": fsext.NewMemMapFs()}
		src, err := loader.Load(logger, filesystems, moduleSpecifierURL, moduleSpecifier)

		require.NoError(t, err)
		assert.Equal(t, src.URL.String(), sr("HTTPSBIN_URL/raw/something"))
		assert.Equal(t, responseStr, string(src.Data))
	})

	t.Run("Invalid", func(t *testing.T) {
		t.Parallel()
		root, err := url.Parse("file:///")
		require.NoError(t, err)

		testData := [...]struct {
			name, moduleSpecifier string
		}{
			{"URL", sr("HTTPSBIN_URL/invalid")},
			{"HOST", "https://some-path-that-doesnt-exist.js"},
		}

		filesystems := map[string]fsext.Fs{"https": fsext.NewMemMapFs()}
		for _, data := range testData {
			moduleSpecifier := data.moduleSpecifier
			t.Run(data.name, func(t *testing.T) {
				moduleSpecifierURL, err := loader.Resolve(root, moduleSpecifier)
				require.NoError(t, err)

				_, err = loader.Load(logger, filesystems, moduleSpecifierURL, moduleSpecifier)
				require.Error(t, err)
			})
		}
	})
}
