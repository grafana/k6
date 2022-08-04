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

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

func TestInitContextRequire(t *testing.T) {
	t.Parallel()
	t.Run("Modules", func(t *testing.T) {
		t.Run("Nonexistent", func(t *testing.T) {
			t.Parallel()
			_, err := getSimpleBundle(t, "/script.js", `import "k6/NONEXISTENT";`)
			require.Error(t, err)
			require.Contains(t, err.Error(), "unknown module: k6/NONEXISTENT")
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
			require.NoError(t, err, "bundle error")

			bi, err := b.Instantiate(logger, 0)
			assert.NoError(t, err, "instance error")

			exports := bi.pgm.exports
			require.NotNil(t, exports)
			_, defaultOk := goja.AssertFunction(exports.Get("default"))
			assert.True(t, defaultOk, "default export is not a function")
			assert.Equal(t, "abc123", exports.Get("dummy").String())

			k6 := exports.Get("_k6").ToObject(bi.Runtime)
			require.NotNil(t, k6)
			_, groupOk := goja.AssertFunction(k6.Get("group"))
			assert.True(t, groupOk, "k6.group is not a function")
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

			exports := bi.pgm.exports
			require.NotNil(t, exports)
			_, defaultOk := goja.AssertFunction(exports.Get("default"))
			assert.True(t, defaultOk, "default export is not a function")
			assert.Equal(t, "abc123", exports.Get("dummy").String())

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
			require.NotNil(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf(`"%s" couldn't be found on local disk`, filepath.ToSlash(path)))
		})
		t.Run("Invalid", func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			require.NoError(t, afero.WriteFile(fs, "/file.js", []byte{0x00}, 0o755))
			_, err := getSimpleBundle(t, "/script.js", `import "/file.js"; export default function() {}`, fs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "SyntaxError: file:///file.js: Unexpected character '\x00' (1:0)\n> 1 | \x00\n")
		})
		t.Run("Error", func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			require.NoError(t, afero.WriteFile(fs, "/file.js", []byte(`throw new Error("aaaa")`), 0o755))
			_, err := getSimpleBundle(t, "/script.js", `import "/file.js"; export default function() {}`, fs)
			assert.EqualError(t, err,
				"Error: aaaa\n\tat file:///file.js:2:7(3)\n\tat go.k6.io/k6/js.(*InitContext).Require-fm (native)\n\tat file:///script.js:1:0(14)\n\tat native\n")
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
							require.NoError(t, fs.MkdirAll(filepath.Dir(constPath), 0o755))
							require.NoError(t, afero.WriteFile(fs, constPath, []byte(constsrc), 0o644))
						}

						require.NoError(t, fs.MkdirAll(filepath.Dir(data.LibPath), 0o755))
						require.NoError(t, afero.WriteFile(fs, data.LibPath, []byte(jsLib), 0o644))

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
			require.NoError(t, afero.WriteFile(fs, "/a.js", []byte(`const myvar = "a";`), 0o644))
			require.NoError(t, afero.WriteFile(fs, "/b.js", []byte(`const myvar = "b";`), 0o644))
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
	require.NoError(t, fs.MkdirAll("/path/to", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/path/to/"+file, content, 0o644))

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
	if err != nil {
		return nil, err
	}

	bi, err := b.Instantiate(testutils.NewLogger(t), 0)
	if err != nil {
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
			assert.Equal(t, string(tc.content), bi.pgm.exports.Get("data").Export())
		})
	}

	t.Run("Binary", func(t *testing.T) {
		t.Parallel()
		bi, err := createAndReadFile(t, "/path/to/file.bin", []byte("hi!\x0f\xff\x01"), 6, "b")
		require.NoError(t, err)
		buf := bi.Runtime.NewArrayBuffer([]byte{104, 105, 33, 15, 255, 1})
		assert.Equal(t, buf, bi.pgm.exports.Get("data").Export())
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
		require.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("open %s: file does not exist", path))
	})

	t.Run("Directory", func(t *testing.T) {
		t.Parallel()
		path := filepath.FromSlash("/some/dir")
		fs := afero.NewMemMapFs()
		require.NoError(t, fs.MkdirAll(path, 0o755))
		_, err := getSimpleBundle(t, "/script.js", `open("/some/dir"); export default function() {}`, fs)
		require.Error(t, err)
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

		require.NoError(t, r.ParseMultipartForm(32<<20))
		file, _, err := r.FormFile("file")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, file.Close())
		}()
		bytes := make([]byte, 3)
		_, err = file.Read(bytes)
		require.NoError(t, err)
		assert.Equal(t, []byte("hi!"), bytes)
		assert.Equal(t, "this is a standard form field", r.FormValue("field"))
	}

	srv := httptest.NewServer(http.HandlerFunc(h))
	defer srv.Close()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/path/to", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/path/to/file.bin", []byte("hi!"), 0o644))

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
	require.NoError(t, err)

	root, err := lib.NewGroup("", nil)
	require.NoError(t, err)

	logger := logrus.New()
	logger.Level = logrus.DebugLevel
	logger.Out = ioutil.Discard

	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	bi.moduleVUImpl.state = &lib.State{
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
		Samples:        make(chan metrics.SampleContainer, 500),
		BuiltinMetrics: builtinMetrics,
		Tags:           lib.NewTagMap(nil),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bi.moduleVUImpl.ctx = ctx

	v, err := bi.exports[consts.DefaultFn](goja.Undefined())
	require.NoError(t, err)
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
			defer func() { require.NoError(t, f.Close()) }()
			bytes := make([]byte, 5)
			_, err := f.Read(bytes)
			require.NoError(t, err)
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
	require.NoError(t, fs.MkdirAll("/path/to", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/path/to/file1.bin", []byte("file1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/path/to/file2.bin", []byte("file2"), 0o644))

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
	require.NoError(t, err)

	root, err := lib.NewGroup("", nil)
	require.NoError(t, err)

	logger := logrus.New()
	logger.Level = logrus.DebugLevel
	logger.Out = ioutil.Discard

	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	bi.moduleVUImpl.state = &lib.State{
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
		Samples:        make(chan metrics.SampleContainer, 500),
		BuiltinMetrics: builtinMetrics,
		Tags:           lib.NewTagMap(nil),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bi.moduleVUImpl.ctx = ctx

	v, err := bi.exports[consts.DefaultFn](goja.Undefined())
	require.NoError(t, err)
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

func TestSourceMaps(t *testing.T) {
	t.Parallel()
	logger := testutils.NewLogger(t)
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/module1.js", []byte(`
export function f2(){
    throw "exception in line 2"
    console.log("in f2")
}
export function f1(){
    throw "exception in line 6"
    console.log("in f1")
}
`[1:]), 0o644))
	data := `
import * as module1 from "./module1.js"

export default function(){
//    throw "exception in line 4"
    module1.f2()
    console.log("in default")
}
`[1:]
	b, err := getSimpleBundle(t, "/script.js", data, fs)
	require.NoError(t, err)

	bi, err := b.Instantiate(logger, 0)
	require.NoError(t, err)
	_, err = bi.exports[consts.DefaultFn](goja.Undefined())
	require.Error(t, err)
	exception := new(goja.Exception)
	require.ErrorAs(t, err, &exception)
	require.Equal(t, exception.String(), "exception in line 2\n\tat f2 (file:///module1.js:2:4(2))\n\tat file:///script.js:5:4(3)\n\tat native\n")
}

func TestSourceMapsExternal(t *testing.T) {
	t.Parallel()
	logger := testutils.NewLogger(t)
	fs := afero.NewMemMapFs()
	// This example is created through the template-typescript
	require.NoError(t, afero.WriteFile(fs, "/test1.js", []byte(`
(()=>{"use strict";var e={};(()=>{var o=e;Object.defineProperty(o,"__esModule",{value:!0}),o.default=function(){!function(e){throw"cool is cool"}()}})();var o=exports;for(var r in e)o[r]=e[r];e.__esModule&&Object.defineProperty(o,"__esModule",{value:!0})})();
//# sourceMappingURL=test1.js.map
`[1:]), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/test1.js.map", []byte(`
{"version":3,"sources":["webpack:///./test1.ts"],"names":["s","coolThrow"],"mappings":"2FAGA,sBAHA,SAAmBA,GACf,KAAM,eAGNC,K","file":"test1.js","sourcesContent":["function coolThrow(s: string) {\n    throw \"cool \"+ s\n}\nexport default () => {\n    coolThrow(\"is cool\")\n};\n"],"sourceRoot":""}
`[1:]), 0o644))
	data := `
import l from "./test1.js"

export default function () {
		l()
};
`[1:]
	b, err := getSimpleBundle(t, "/script.js", data, fs)
	require.NoError(t, err)

	bi, err := b.Instantiate(logger, 0)
	require.NoError(t, err)
	_, err = bi.exports[consts.DefaultFn](goja.Undefined())
	require.Error(t, err)
	exception := new(goja.Exception)
	require.ErrorAs(t, err, &exception)
	require.Equal(t, "cool is cool\n\tat webpack:///./test1.ts:2:4(2)\n\tat webpack:///./test1.ts:5:4(3)\n\tat file:///script.js:4:2(4)\n\tat native\n", exception.String())
}

func TestSourceMapsExternalExtented(t *testing.T) {
	t.Parallel()
	logger := testutils.NewLogger(t)
	fs := afero.NewMemMapFs()
	// This example is created through the template-typescript
	// but was exported to use import/export syntax so it has to go through babel
	require.NoError(t, afero.WriteFile(fs, "/test1.js", []byte(`
var o={d:(e,r)=>{for(var t in r)o.o(r,t)&&!o.o(e,t)&&Object.defineProperty(e,t,{enumerable:!0,get:r[t]})},o:(o,e)=>Object.prototype.hasOwnProperty.call(o,e)},e={};o.d(e,{Z:()=>r});const r=()=>{!function(o){throw"cool is cool"}()};var t=e.Z;export{t as default};
//# sourceMappingURL=test1.js.map
`[1:]), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/test1.js.map", []byte(`
{"version":3,"sources":["webpack:///webpack/bootstrap","webpack:///webpack/runtime/define property getters","webpack:///webpack/runtime/hasOwnProperty shorthand","webpack:///./test1.ts"],"names":["__webpack_require__","exports","definition","key","o","Object","defineProperty","enumerable","get","obj","prop","prototype","hasOwnProperty","call","s","coolThrow"],"mappings":"AACA,IAAIA,EAAsB,CCA1B,EAAwB,CAACC,EAASC,KACjC,IAAI,IAAIC,KAAOD,EACXF,EAAoBI,EAAEF,EAAYC,KAASH,EAAoBI,EAAEH,EAASE,IAC5EE,OAAOC,eAAeL,EAASE,EAAK,CAAEI,YAAY,EAAMC,IAAKN,EAAWC,MCJ3E,EAAwB,CAACM,EAAKC,IAAUL,OAAOM,UAAUC,eAAeC,KAAKJ,EAAKC,I,sBCGlF,cAHA,SAAmBI,GACf,KAAM,eAGNC,I","file":"test1.js","sourcesContent":["// The require scope\nvar __webpack_require__ = {};\n\n","// define getter functions for harmony exports\n__webpack_require__.d = (exports, definition) => {\n\tfor(var key in definition) {\n\t\tif(__webpack_require__.o(definition, key) && !__webpack_require__.o(exports, key)) {\n\t\t\tObject.defineProperty(exports, key, { enumerable: true, get: definition[key] });\n\t\t}\n\t}\n};","__webpack_require__.o = (obj, prop) => (Object.prototype.hasOwnProperty.call(obj, prop))","function coolThrow(s: string) {\n    throw \"cool \"+ s\n}\nexport default () => {\n    coolThrow(\"is cool\")\n};\n"],"sourceRoot":""}
`[1:]), 0o644))
	data := `
import l from "./test1.js"

export default function () {
		l()
};
`[1:]
	b, err := getSimpleBundle(t, "/script.js", data, fs)
	require.NoError(t, err)

	bi, err := b.Instantiate(logger, 0)
	require.NoError(t, err)
	_, err = bi.exports[consts.DefaultFn](goja.Undefined())
	require.Error(t, err)
	exception := new(goja.Exception)
	require.ErrorAs(t, err, &exception)
	// TODO figure out why those are not the same as the one in the previous test TestSourceMapsExternal
	// likely settings in the transpilers
	require.Equal(t, "cool is cool\n\tat webpack:///./test1.ts:2:4(2)\n\tat r (webpack:///./test1.ts:5:4(3))\n\tat file:///script.js:4:2(4)\n\tat native\n", exception.String())
}

func TestSourceMapsExternalExtentedInlined(t *testing.T) {
	t.Parallel()
	logger := testutils.NewLogger(t)
	fs := afero.NewMemMapFs()
	// This example is created through the template-typescript
	// but was exported to use import/export syntax so it has to go through babel
	require.NoError(t, afero.WriteFile(fs, "/test1.js", []byte(`
var o={d:(e,r)=>{for(var t in r)o.o(r,t)&&!o.o(e,t)&&Object.defineProperty(e,t,{enumerable:!0,get:r[t]})},o:(o,e)=>Object.prototype.hasOwnProperty.call(o,e)},e={};o.d(e,{Z:()=>r});const r=()=>{!function(o){throw"cool is cool"}()};var t=e.Z;export{t as default};
//# sourceMappingURL=data:application/json;charset=utf-8;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly8vd2VicGFjay9ib290c3RyYXAiLCJ3ZWJwYWNrOi8vL3dlYnBhY2svcnVudGltZS9kZWZpbmUgcHJvcGVydHkgZ2V0dGVycyIsIndlYnBhY2s6Ly8vd2VicGFjay9ydW50aW1lL2hhc093blByb3BlcnR5IHNob3J0aGFuZCIsIndlYnBhY2s6Ly8vLi90ZXN0MS50cyJdLCJuYW1lcyI6WyJfX3dlYnBhY2tfcmVxdWlyZV9fIiwiZXhwb3J0cyIsImRlZmluaXRpb24iLCJrZXkiLCJvIiwiT2JqZWN0IiwiZGVmaW5lUHJvcGVydHkiLCJlbnVtZXJhYmxlIiwiZ2V0Iiwib2JqIiwicHJvcCIsInByb3RvdHlwZSIsImhhc093blByb3BlcnR5IiwiY2FsbCIsInMiLCJjb29sVGhyb3ciXSwibWFwcGluZ3MiOiJBQUNBLElBQUlBLEVBQXNCLENDQTFCLEVBQXdCLENBQUNDLEVBQVNDLEtBQ2pDLElBQUksSUFBSUMsS0FBT0QsRUFDWEYsRUFBb0JJLEVBQUVGLEVBQVlDLEtBQVNILEVBQW9CSSxFQUFFSCxFQUFTRSxJQUM1RUUsT0FBT0MsZUFBZUwsRUFBU0UsRUFBSyxDQUFFSSxZQUFZLEVBQU1DLElBQUtOLEVBQVdDLE1DSjNFLEVBQXdCLENBQUNNLEVBQUtDLElBQVVMLE9BQU9NLFVBQVVDLGVBQWVDLEtBQUtKLEVBQUtDLEksc0JDR2xGLGNBSEEsU0FBbUJJLEdBQ2YsS0FBTSxlQUdOQyxJIiwiZmlsZSI6InRlc3QxLmpzIiwic291cmNlc0NvbnRlbnQiOlsiLy8gVGhlIHJlcXVpcmUgc2NvcGVcbnZhciBfX3dlYnBhY2tfcmVxdWlyZV9fID0ge307XG5cbiIsIi8vIGRlZmluZSBnZXR0ZXIgZnVuY3Rpb25zIGZvciBoYXJtb255IGV4cG9ydHNcbl9fd2VicGFja19yZXF1aXJlX18uZCA9IChleHBvcnRzLCBkZWZpbml0aW9uKSA9PiB7XG5cdGZvcih2YXIga2V5IGluIGRlZmluaXRpb24pIHtcblx0XHRpZihfX3dlYnBhY2tfcmVxdWlyZV9fLm8oZGVmaW5pdGlvbiwga2V5KSAmJiAhX193ZWJwYWNrX3JlcXVpcmVfXy5vKGV4cG9ydHMsIGtleSkpIHtcblx0XHRcdE9iamVjdC5kZWZpbmVQcm9wZXJ0eShleHBvcnRzLCBrZXksIHsgZW51bWVyYWJsZTogdHJ1ZSwgZ2V0OiBkZWZpbml0aW9uW2tleV0gfSk7XG5cdFx0fVxuXHR9XG59OyIsIl9fd2VicGFja19yZXF1aXJlX18ubyA9IChvYmosIHByb3ApID0+IChPYmplY3QucHJvdG90eXBlLmhhc093blByb3BlcnR5LmNhbGwob2JqLCBwcm9wKSkiLCJmdW5jdGlvbiBjb29sVGhyb3coczogc3RyaW5nKSB7XG4gICAgdGhyb3cgXCJjb29sIFwiKyBzXG59XG5leHBvcnQgZGVmYXVsdCAoKSA9PiB7XG4gICAgY29vbFRocm93KFwiaXMgY29vbFwiKVxufTtcbiJdLCJzb3VyY2VSb290IjoiIn0=
`[1:]), 0o644))
	data := `
import l from "./test1.js"

export default function () {
		l()
};
`[1:]
	b, err := getSimpleBundle(t, "/script.js", data, fs)
	require.NoError(t, err)

	bi, err := b.Instantiate(logger, 0)
	require.NoError(t, err)
	_, err = bi.exports[consts.DefaultFn](goja.Undefined())
	require.Error(t, err)
	exception := new(goja.Exception)
	require.ErrorAs(t, err, &exception)
	// TODO figure out why those are not the same as the one in the previous test TestSourceMapsExternal
	// likely settings in the transpilers
	require.Equal(t, "cool is cool\n\tat webpack:///./test1.ts:2:4(2)\n\tat r (webpack:///./test1.ts:5:4(3))\n\tat file:///script.js:4:2(4)\n\tat native\n", exception.String())
}
