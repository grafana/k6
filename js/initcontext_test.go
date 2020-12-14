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

package js

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/oxtoacart/bpool"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/consts"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
)

func TestInitContextRequire(t *testing.T) {
	logger := testutils.NewLogger(t)
	t.Run("Modules", func(t *testing.T) {
		t.Run("Nonexistent", func(t *testing.T) {
			_, err := getSimpleBundle(t, "/script.js", `import "k6/NONEXISTENT";`)
			assert.Contains(t, err.Error(), "unknown module: k6/NONEXISTENT")
		})

		t.Run("k6", func(t *testing.T) {
			b, err := getSimpleBundle(t, "/script.js", `
					import k6 from "k6";
					export let _k6 = k6;
					export let dummy = "abc123";
					export default function() {}
			`)
			if !assert.NoError(t, err, "bundle error") {
				return
			}

			bi, err := b.Instantiate(logger, 0)
			if !assert.NoError(t, err, "instance error") {
				return
			}

			exports := bi.Runtime.Get("exports").ToObject(bi.Runtime)
			if assert.NotNil(t, exports) {
				_, defaultOk := goja.AssertFunction(exports.Get("default"))
				assert.True(t, defaultOk, "default export is not a function")
				assert.Equal(t, "abc123", exports.Get("dummy").String())
			}

			k6 := bi.Runtime.Get("_k6").ToObject(bi.Runtime)
			if assert.NotNil(t, k6) {
				_, groupOk := goja.AssertFunction(k6.Get("group"))
				assert.True(t, groupOk, "k6.group is not a function")
			}

			t.Run("group", func(t *testing.T) {
				b, err := getSimpleBundle(t, "/script.js", `
						import { group } from "k6";
						export let _group = group;
						export let dummy = "abc123";
						export default function() {}
				`)
				if !assert.NoError(t, err) {
					return
				}

				bi, err := b.Instantiate(logger, 0)
				if !assert.NoError(t, err) {
					return
				}

				exports := bi.Runtime.Get("exports").ToObject(bi.Runtime)
				if assert.NotNil(t, exports) {
					_, defaultOk := goja.AssertFunction(exports.Get("default"))
					assert.True(t, defaultOk, "default export is not a function")
					assert.Equal(t, "abc123", exports.Get("dummy").String())
				}

				_, groupOk := goja.AssertFunction(exports.Get("_group"))
				assert.True(t, groupOk, "{ group } is not a function")
			})
		})
	})

	t.Run("Files", func(t *testing.T) {
		t.Run("Nonexistent", func(t *testing.T) {
			path := filepath.FromSlash("/nonexistent.js")
			_, err := getSimpleBundle(t, "/script.js", `import "/nonexistent.js"; export default function() {}`)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf(`"%s" couldn't be found on local disk`, filepath.ToSlash(path)))
		})
		t.Run("Invalid", func(t *testing.T) {
			fs := afero.NewMemMapFs()
			assert.NoError(t, afero.WriteFile(fs, "/file.js", []byte{0x00}, 0o755))
			_, err := getSimpleBundle(t, "/script.js", `import "/file.js"; export default function() {}`, fs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "SyntaxError: file:///file.js: Unexpected character '\x00' (1:0)\n> 1 | \x00\n")
		})
		t.Run("Error", func(t *testing.T) {
			fs := afero.NewMemMapFs()
			assert.NoError(t, afero.WriteFile(fs, "/file.js", []byte(`throw new Error("aaaa")`), 0o755))
			_, err := getSimpleBundle(t, "/script.js", `import "/file.js"; export default function() {}`, fs)
			assert.EqualError(t, err, "Error: aaaa at file:///file.js:2:7(4)")
		})

		imports := map[string]struct {
			LibPath    string
			ConstPaths map[string]string
		}{
			"./lib.js": {"/path/to/lib.js", map[string]string{
				"":               "",
				"./const.js":     "/path/to/const.js",
				"../const.js":    "/path/const.js",
				"./sub/const.js": "/path/to/sub/const.js",
			}},
			"../lib.js": {"/path/lib.js", map[string]string{
				"":               "",
				"./const.js":     "/path/const.js",
				"../const.js":    "/const.js",
				"./sub/const.js": "/path/sub/const.js",
			}},
			"./dir/lib.js": {"/path/to/dir/lib.js", map[string]string{
				"":               "",
				"./const.js":     "/path/to/dir/const.js",
				"../const.js":    "/path/to/const.js",
				"./sub/const.js": "/path/to/dir/sub/const.js",
			}},
			"/path/to/lib.js": {"/path/to/lib.js", map[string]string{
				"":               "",
				"./const.js":     "/path/to/const.js",
				"../const.js":    "/path/const.js",
				"./sub/const.js": "/path/to/sub/const.js",
			}},
		}
		for libName, data := range imports {
			libName, data := libName, data
			t.Run("lib=\""+libName+"\"", func(t *testing.T) {
				for constName, constPath := range data.ConstPaths {
					constName, constPath := constName, constPath
					name := "inline"
					if constName != "" {
						name = "const=\"" + constName + "\""
					}
					t.Run(name, func(t *testing.T) {
						fs := afero.NewMemMapFs()

						jsLib := `export default function() { return 12345; }`
						if constName != "" {
							jsLib = fmt.Sprintf(
								`import { c } from "%s"; export default function() { return c; }`,
								constName,
							)

							constsrc := `export let c = 12345;`
							assert.NoError(t, fs.MkdirAll(filepath.Dir(constPath), 0o755))
							assert.NoError(t, afero.WriteFile(fs, constPath, []byte(constsrc), 0o644))
						}

						assert.NoError(t, fs.MkdirAll(filepath.Dir(data.LibPath), 0o755))
						assert.NoError(t, afero.WriteFile(fs, data.LibPath, []byte(jsLib), 0o644))

						data := fmt.Sprintf(`
								import fn from "%s";
								let v = fn();
								export default function() {};`,
							libName)
						b, err := getSimpleBundle(t, "/path/to/script.js", data, fs)
						if !assert.NoError(t, err) {
							return
						}
						if constPath != "" {
							assert.Contains(t, b.BaseInitContext.programs, "file://"+constPath)
						}

						_, err = b.Instantiate(logger, 0)
						if !assert.NoError(t, err) {
							return
						}
					})
				}
			})
		}

		t.Run("Isolation", func(t *testing.T) {
			fs := afero.NewMemMapFs()
			assert.NoError(t, afero.WriteFile(fs, "/a.js", []byte(`const myvar = "a";`), 0o644))
			assert.NoError(t, afero.WriteFile(fs, "/b.js", []byte(`const myvar = "b";`), 0o644))
			data := `
				import "./a.js";
				import "./b.js";
				export default function() {
					if (typeof myvar != "undefined") {
						throw new Error("myvar is set in global scope");
					}
				};`
			b, err := getSimpleBundle(t, "/script.js", data, fs)
			if !assert.NoError(t, err) {
				return
			}

			bi, err := b.Instantiate(logger, 0)
			if !assert.NoError(t, err) {
				return
			}
			_, err = bi.exports[consts.DefaultFn](goja.Undefined())
			assert.NoError(t, err)
		})
	})
}

func createAndReadFile(t *testing.T, file string, content []byte, expectedLength int, binary bool) (*BundleInstance, error) {
	fs := afero.NewMemMapFs()
	assert.NoError(t, fs.MkdirAll("/path/to", 0o755))
	assert.NoError(t, afero.WriteFile(fs, "/path/to/"+file, content, 0o644))

	binaryArg := ""
	if binary {
		binaryArg = ",\"b\""
	}

	data := fmt.Sprintf(`
		export let data = open("/path/to/%s"%s);
		var expectedLength = %d;
		if (data.length != expectedLength) {
			throw new Error("Length not equal, expected: " + expectedLength + ", actual: " + data.length);
		}
		export default function() {}
	`, file, binaryArg, expectedLength)
	b, err := getSimpleBundle(t, "/path/to/script.js", data, fs)

	if !assert.NoError(t, err) {
		return nil, err
	}

	bi, err := b.Instantiate(testutils.NewLogger(t), 0)
	if !assert.NoError(t, err) {
		return nil, err
	}
	return bi, nil
}

func TestInitContextOpen(t *testing.T) {
	testCases := []struct {
		content []byte
		file    string
		length  int
	}{
		{[]byte("hello world!"), "ascii", 12},
		{[]byte("?((¯°·._.• ţ€$ţɨɲǥ µɲɨȼ๏ď€ΣSЫ ɨɲ Ќ6 •._.·°¯))؟•"), "utf", 47},
		{[]byte{0o44, 226, 130, 172}, "utf-8", 2}, // $€
		//{[]byte{00, 36, 32, 127}, "utf-16", 2},   // $€
	}
	for _, tc := range testCases {
		t.Run(tc.file, func(t *testing.T) {
			bi, err := createAndReadFile(t, tc.file, tc.content, tc.length, false)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, string(tc.content), bi.Runtime.Get("data").Export())
		})
	}

	t.Run("Binary", func(t *testing.T) {
		bi, err := createAndReadFile(t, "/path/to/file.bin", []byte("hi!\x0f\xff\x01"), 6, true)
		if !assert.NoError(t, err) {
			return
		}
		bytes := []byte{104, 105, 33, 15, 255, 1}
		assert.Equal(t, bytes, bi.Runtime.Get("data").Export())
	})

	testdata := map[string]string{
		"Absolute": "/path/to/file",
		"Relative": "./file",
	}

	for name, loadPath := range testdata {
		t.Run(name, func(t *testing.T) {
			_, err := createAndReadFile(t, loadPath, []byte("content"), 7, false)
			if !assert.NoError(t, err) {
				return
			}
		})
	}

	t.Run("Nonexistent", func(t *testing.T) {
		path := filepath.FromSlash("/nonexistent.txt")
		_, err := getSimpleBundle(t, "/script.js", `open("/nonexistent.txt"); export default function() {}`)
		assert.Contains(t, err.Error(), fmt.Sprintf("open %s: file does not exist", path))
	})

	t.Run("Directory", func(t *testing.T) {
		path := filepath.FromSlash("/some/dir")
		fs := afero.NewMemMapFs()
		assert.NoError(t, fs.MkdirAll(path, 0o755))
		_, err := getSimpleBundle(t, "/script.js", `open("/some/dir"); export default function() {}`, fs)
		assert.Contains(t, err.Error(), fmt.Sprintf("open() can't be used with directories, path: %q", path))
	})
}

func TestRequestWithBinaryFile(t *testing.T) {
	t.Parallel()

	ch := make(chan bool, 1)

	h := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			ch <- true
		}()

		assert.NoError(t, r.ParseMultipartForm(32<<20))
		file, _, err := r.FormFile("file")
		assert.NoError(t, err)
		defer func() {
			assert.NoError(t, file.Close())
		}()
		bytes := make([]byte, 3)
		_, err = file.Read(bytes)
		assert.NoError(t, err)
		assert.Equal(t, []byte("hi!"), bytes)
		assert.Equal(t, "this is a standard form field", r.FormValue("field"))
	}

	srv := httptest.NewServer(http.HandlerFunc(h))
	defer srv.Close()

	fs := afero.NewMemMapFs()
	assert.NoError(t, fs.MkdirAll("/path/to", 0o755))
	assert.NoError(t, afero.WriteFile(fs, "/path/to/file.bin", []byte("hi!"), 0o644))

	b, err := getSimpleBundle(t, "/path/to/script.js",
		fmt.Sprintf(`
			import http from "k6/http";
			let binFile = open("/path/to/file.bin", "b");
			export default function() {
				var data = {
					field: "this is a standard form field",
					file: http.file(binFile, "test.bin")
				};
				var res = http.post("%s", data);
				return true;
			}
			`, srv.URL), fs)
	require.NoError(t, err)

	bi, err := b.Instantiate(testutils.NewLogger(t), 0)
	assert.NoError(t, err)

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	logger := logrus.New()
	logger.Level = logrus.DebugLevel
	logger.Out = ioutil.Discard

	state := &lib.State{
		Options: lib.Options{},
		Logger:  logger,
		Group:   root,
		Transport: &http.Transport{
			DialContext: (netext.NewDialer(
				net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 60 * time.Second,
					DualStack: true,
				},
				netext.NewResolver(net.LookupIP, 0, types.DNSfirst, types.DNSpreferIPv4),
			)).DialContext,
		},
		BPool:   bpool.NewBufferPool(1),
		Samples: make(chan stats.SampleContainer, 500),
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, bi.Runtime)
	*bi.Context = ctx

	v, err := bi.exports[consts.DefaultFn](goja.Undefined())
	assert.NoError(t, err)
	assert.NotNil(t, v)
	assert.Equal(t, true, v.Export())

	<-ch
}

func TestInitContextVU(t *testing.T) {
	b, err := getSimpleBundle(t, "/script.js", `
		let vu = __VU;
		export default function() { return vu; }
	`)
	require.NoError(t, err)
	bi, err := b.Instantiate(testutils.NewLogger(t), 5)
	require.NoError(t, err)
	v, err := bi.exports[consts.DefaultFn](goja.Undefined())
	require.NoError(t, err)
	assert.Equal(t, int64(5), v.Export())
}
