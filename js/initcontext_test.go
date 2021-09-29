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

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/stats"
)

func TestInitContextRequire(t *testing.T) {
	t.Parallel()
	t.Run("Modules", func(t *testing.T) {
		t.Run("Nonexistent", func(t *testing.T) {
			t.Parallel()
			_, err := getSimpleBundle(t, "/script.js", `import "k6/NONEXISTENT";`)
			assert.Contains(t, err.Error(), "unknown module: k6/NONEXISTENT")
		})

		t.Run("k6", func(t *testing.T) {
			t.Parallel()
			logger := testutils.NewLogger(t)
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
		})

		t.Run("group", func(t *testing.T) {
			logger := testutils.NewLogger(t)
			t.Parallel()
			b, err := getSimpleBundle(t, "/script.js", `
						import { group } from "k6";
						export let _group = group;
						export let dummy = "abc123";
						export default function() {}
				`)
			require.NoError(t, err)

			bi, err := b.Instantiate(logger, 0)
			require.NoError(t, err)

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

	t.Run("Files", func(t *testing.T) {
		t.Parallel()
		t.Run("Nonexistent", func(t *testing.T) {
			t.Parallel()
			path := filepath.FromSlash("/nonexistent.js")
			_, err := getSimpleBundle(t, "/script.js", `import "/nonexistent.js"; export default function() {}`)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf(`"%s" couldn't be found on local disk`, filepath.ToSlash(path)))
		})
		t.Run("Invalid", func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			assert.NoError(t, afero.WriteFile(fs, "/file.js", []byte{0x00}, 0o755))
			_, err := getSimpleBundle(t, "/script.js", `import "/file.js"; export default function() {}`, fs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "SyntaxError: file:///file.js: Unexpected character '\x00' (1:0)\n> 1 | \x00\n")
		})
		t.Run("Error", func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			assert.NoError(t, afero.WriteFile(fs, "/file.js", []byte(`throw new Error("aaaa")`), 0o755))
			_, err := getSimpleBundle(t, "/script.js", `import "/file.js"; export default function() {}`, fs)
			assert.EqualError(t, err, "Error: aaaa\n\tat file:///file.js:2:7(4)\n\tat reflect.methodValueCall (native)\n\tat file:///script.js:1:117(14)\n")
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
				t.Parallel()
				for constName, constPath := range data.ConstPaths {
					constName, constPath := constName, constPath
					name := "inline"
					if constName != "" {
						name = "const=\"" + constName + "\""
					}
					t.Run(name, func(t *testing.T) {
						t.Parallel()
						fs := afero.NewMemMapFs()
						logger := testutils.NewLogger(t)

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
						require.NoError(t, err)
						if constPath != "" {
							assert.Contains(t, b.BaseInitContext.programs, "file://"+constPath)
						}

						_, err = b.Instantiate(logger, 0)
						require.NoError(t, err)
					})
				}
			})
		}

		t.Run("Isolation", func(t *testing.T) {
			t.Parallel()
			logger := testutils.NewLogger(t)
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
			require.NoError(t, err)

			bi, err := b.Instantiate(logger, 0)
			require.NoError(t, err)
			_, err = bi.exports[consts.DefaultFn](goja.Undefined())
			assert.NoError(t, err)
		})
	})
}

func createAndReadFile(t *testing.T, file string, content []byte, expectedLength int, binary string) (*BundleInstance, error) {
	t.Helper()
	fs := afero.NewMemMapFs()
	assert.NoError(t, fs.MkdirAll("/path/to", 0o755))
	assert.NoError(t, afero.WriteFile(fs, "/path/to/"+file, content, 0o644))

	data := fmt.Sprintf(`
		let binArg = "%s";
		export let data = open("/path/to/%s", binArg);
		var expectedLength = %d;
		var len = binArg === "b" ? "byteLength" : "length";
		if (data[len] != expectedLength) {
			throw new Error("Length not equal, expected: " + expectedLength + ", actual: " + data[len]);
		}
		export default function() {}
	`, binary, file, expectedLength)
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
	t.Parallel()
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
		tc := tc
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			bi, err := createAndReadFile(t, tc.file, tc.content, tc.length, "")
			require.NoError(t, err)
			assert.Equal(t, string(tc.content), bi.Runtime.Get("data").Export())
		})
	}

	t.Run("Binary", func(t *testing.T) {
		t.Parallel()
		bi, err := createAndReadFile(t, "/path/to/file.bin", []byte("hi!\x0f\xff\x01"), 6, "b")
		require.NoError(t, err)
		buf := bi.Runtime.NewArrayBuffer([]byte{104, 105, 33, 15, 255, 1})
		assert.Equal(t, buf, bi.Runtime.Get("data").Export())
	})

	testdata := map[string]string{
		"Absolute": "/path/to/file",
		"Relative": "./file",
	}

	for name, loadPath := range testdata {
		loadPath := loadPath
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := createAndReadFile(t, loadPath, []byte("content"), 7, "")
			require.NoError(t, err)
		})
	}

	t.Run("Nonexistent", func(t *testing.T) {
		t.Parallel()
		path := filepath.FromSlash("/nonexistent.txt")
		_, err := getSimpleBundle(t, "/script.js", `open("/nonexistent.txt"); export default function() {}`)
		assert.Contains(t, err.Error(), fmt.Sprintf("open %s: file does not exist", path))
	})

	t.Run("Directory", func(t *testing.T) {
		t.Parallel()
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

	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
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
		BPool:          bpool.NewBufferPool(1),
		Samples:        make(chan stats.SampleContainer, 500),
		BuiltinMetrics: builtinMetrics,
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, bi.Runtime)
	*bi.Context = ctx

	v, err := bi.exports[consts.DefaultFn](goja.Undefined())
	assert.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, true, v.Export())

	<-ch
}

func TestRequestWithMultipleBinaryFiles(t *testing.T) {
	t.Parallel()

	ch := make(chan bool, 1)

	h := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			ch <- true
		}()

		require.NoError(t, r.ParseMultipartForm(32<<20))
		require.Len(t, r.MultipartForm.File["files"], 2)
		for i, fh := range r.MultipartForm.File["files"] {
			f, _ := fh.Open()
			defer func() { assert.NoError(t, f.Close()) }()
			bytes := make([]byte, 5)
			_, err := f.Read(bytes)
			assert.NoError(t, err)
			switch i {
			case 0:
				assert.Equal(t, []byte("file1"), bytes)
			case 1:
				assert.Equal(t, []byte("file2"), bytes)
			}
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(h))
	defer srv.Close()

	fs := afero.NewMemMapFs()
	assert.NoError(t, fs.MkdirAll("/path/to", 0o755))
	assert.NoError(t, afero.WriteFile(fs, "/path/to/file1.bin", []byte("file1"), 0o644))
	assert.NoError(t, afero.WriteFile(fs, "/path/to/file2.bin", []byte("file2"), 0o644))

	b, err := getSimpleBundle(t, "/path/to/script.js",
		fmt.Sprintf(`
	import http from 'k6/http';

	function toByteArray(obj) {
		let arr = [];
		if (typeof obj === 'string') {
			for (let i=0; i < obj.length; i++) {
			  arr.push(obj.charCodeAt(i) & 0xff);
			}
		} else {
			obj = new Uint8Array(obj);
			for (let i=0; i < obj.byteLength; i++) {
			  arr.push(obj[i] & 0xff);
			}
		}
		return arr;
	}

	// A more robust version of this polyfill is available here:
	// https://jslib.k6.io/formdata/0.0.1/index.js
	function FormData() {
		this.boundary = '----boundary';
		this.files = [];
	}

	FormData.prototype.append = function(name, value, filename) {
		this.files.push({
			name: name,
			value: value,
			filename: filename,
		});
	}

	FormData.prototype.body = function(name, value, filename) {
		let body = [];
		let barr = toByteArray('--' + this.boundary + '\r\n');
		for (let i=0; i < this.files.length; i++) {
			body.push(...barr);
			let cdarr = toByteArray('Content-Disposition: form-data; name="'
							+ this.files[i].name + '"; filename="'
							+ this.files[i].filename
							+ '"\r\nContent-Type: application/octet-stream\r\n\r\n');
			body.push(...cdarr);
			body.push(...toByteArray(this.files[i].value));
			body.push(...toByteArray('\r\n'));
		}
		body.push(...toByteArray('--' + this.boundary + '--\r\n'));
		return new Uint8Array(body).buffer;
	}

	const file1 = open('/path/to/file1.bin', 'b');
	const file2 = open('/path/to/file2.bin', 'b');

	export default function () {
		const fd = new FormData();
		fd.append('files', file1, 'file1.bin');
		fd.append('files', file2, 'file2.bin');
		let res = http.post('%s', fd.body(),
				{ headers: { 'Content-Type': 'multipart/form-data; boundary=' + fd.boundary }});
		if (res.status !== 200) {
			throw new Error('Expected HTTP 200 response, received: ' + res.status);
		}
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

	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
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
		BPool:          bpool.NewBufferPool(1),
		Samples:        make(chan stats.SampleContainer, 500),
		BuiltinMetrics: builtinMetrics,
	}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, bi.Runtime)
	*bi.Context = ctx

	v, err := bi.exports[consts.DefaultFn](goja.Undefined())
	assert.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, true, v.Export())

	<-ch
}

func TestInitContextVU(t *testing.T) {
	t.Parallel()
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
