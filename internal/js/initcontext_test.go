package js

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

func TestRequire(t *testing.T) {
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
			b, err := getSimpleBundle(t, "/script.js", `
					import k6 from "k6";
					export let _k6 = k6;
					export let dummy = "abc123";
					export default function() {}
			`)
			require.NoError(t, err, "bundle error")

			bi, err := b.Instantiate(context.Background(), 0)
			assert.NoError(t, err, "instance error")

			_, defaultOk := sobek.AssertFunction(bi.getExported("default"))
			assert.True(t, defaultOk, "default export is not a function")
			assert.Equal(t, "abc123", bi.getExported("dummy").String())

			k6 := bi.getExported("_k6").ToObject(bi.Runtime)
			require.NotNil(t, k6)
			_, groupOk := sobek.AssertFunction(k6.Get("group"))
			assert.True(t, groupOk, "k6.group is not a function")
		})

		t.Run("group", func(t *testing.T) {
			t.Parallel()
			b, err := getSimpleBundle(t, "/script.js", `
						import { group } from "k6";
						export let _group = group;
						export let dummy = "abc123";
						export default function() {}
				`)
			require.NoError(t, err)

			bi, err := b.Instantiate(context.Background(), 0)
			require.NoError(t, err)

			_, defaultOk := sobek.AssertFunction(bi.getExported("default"))
			assert.True(t, defaultOk, "default export is not a function")
			assert.Equal(t, "abc123", bi.getExported("dummy").String())

			_, groupOk := sobek.AssertFunction(bi.getExported("_group"))
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
			fs := fsext.NewMemMapFs()
			require.NoError(t, fsext.WriteFile(fs, "/file.js", []byte{0x00}, 0o755))
			_, err := getSimpleBundle(t, "/script.js", `import "/file.js"; export default function() {}`, fs)
			assert.ErrorContains(t, err, "file:///file.js: Line 1:1 Unexpected token ILLEGAL (and 1 more errors)")
		})
		t.Run("Error", func(t *testing.T) {
			t.Parallel()
			fs := fsext.NewMemMapFs()
			require.NoError(t, fsext.WriteFile(fs, "/file.js", []byte(`throw new Error("aaaa")`), 0o755))
			_, err := getSimpleBundle(t, "/script.js", `import "/file.js"; export default function() {}`, fs)
			assert.EqualError(t, err,
				"Error: aaaa\n\tat file:///file.js:1:34(3)\n")
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
						fs := fsext.NewMemMapFs()

						jsLib := `export default function() { return 12345; }`
						if constName != "" {
							jsLib = fmt.Sprintf(
								`import { c } from "%s"; export default function() { return c; }`,
								constName,
							)

							constsrc := `export let c = 12345;`
							require.NoError(t, fs.MkdirAll(filepath.Dir(constPath), 0o755))
							require.NoError(t, fsext.WriteFile(fs, constPath, []byte(constsrc), 0o644))
						}

						require.NoError(t, fs.MkdirAll(filepath.Dir(data.LibPath), 0o755))
						require.NoError(t, fsext.WriteFile(fs, data.LibPath, []byte(jsLib), 0o644))

						data := fmt.Sprintf(`
								import fn from "%s";
								let v = fn();
								export default function() {};`,
							libName)
						b, err := getSimpleBundle(t, "/path/to/script.js", data, fs)
						require.NoError(t, err)

						_, err = b.Instantiate(context.Background(), 0)
						require.NoError(t, err)
					})
				}
			})
		}

		t.Run("Isolation", func(t *testing.T) {
			t.Parallel()
			fs := fsext.NewMemMapFs()
			require.NoError(t, fsext.WriteFile(fs, "/a.js", []byte(`const myvar = "a";`), 0o644))
			require.NoError(t, fsext.WriteFile(fs, "/b.js", []byte(`const myvar = "b";`), 0o644))
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

			bi, err := b.Instantiate(context.Background(), 0)
			require.NoError(t, err)
			_, err = bi.getCallableExport(consts.DefaultFn)(sobek.Undefined())
			assert.NoError(t, err)
		})
	})
}

func createAndReadFile(t *testing.T, file string, content []byte, expectedLength int, binary string) (*BundleInstance, error) {
	t.Helper()
	fs := fsext.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/path/to", 0o755))
	require.NoError(t, fsext.WriteFile(fs, "/path/to/"+file, content, 0o644))

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

	bi, err := b.Instantiate(context.Background(), 0)
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
		// {[]byte{00, 36, 32, 127}, "utf-16", 2},   // $€
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			bi, err := createAndReadFile(t, tc.file, tc.content, tc.length, "")
			require.NoError(t, err)
			assert.Equal(t, string(tc.content), bi.getExported("data").Export())
		})
	}

	t.Run("Binary", func(t *testing.T) {
		t.Parallel()
		bi, err := createAndReadFile(t, "/path/to/file.bin", []byte("hi!\x0f\xff\x01"), 6, "b")
		require.NoError(t, err)
		buf := bi.Runtime.NewArrayBuffer([]byte{104, 105, 33, 15, 255, 1})
		assert.Equal(t, buf, bi.getExported("data").Export())
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
		fs := fsext.NewMemMapFs()
		require.NoError(t, fs.MkdirAll(path, 0o755))
		_, err := getSimpleBundle(t, "/script.js", `open("/some/dir"); export default function() {}`, fs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("open() can't be used with directories, path: %q", path))
	})
}

func TestRequestWithBinaryFile(t *testing.T) {
	t.Parallel()

	ch := make(chan bool, 1)

	h := func(_ http.ResponseWriter, r *http.Request) {
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

	fs := fsext.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/path/to", 0o755))
	require.NoError(t, fsext.WriteFile(fs, "/path/to/file.bin", []byte("hi!"), 0o644))

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

	bi, err := b.Instantiate(context.Background(), 0)
	require.NoError(t, err)

	logger := logrus.New()
	logger.Level = logrus.DebugLevel
	logger.Out = io.Discard

	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	bi.moduleVUImpl.state = &lib.State{
		Options: lib.Options{},
		Logger:  logger,
		Transport: &http.Transport{
			DialContext: (netext.NewDialer(
				net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 60 * time.Second,
				},
				netext.NewResolver(net.LookupIP, 0, types.DNSfirst, types.DNSpreferIPv4),
			)).DialContext,
		},
		BufferPool:     lib.NewBufferPool(),
		Samples:        make(chan metrics.SampleContainer, 500),
		BuiltinMetrics: builtinMetrics,
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bi.moduleVUImpl.ctx = ctx

	v, err := bi.getCallableExport(consts.DefaultFn)(sobek.Undefined())
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, true, v.Export())

	<-ch
}

func TestRequestWithMultipleBinaryFiles(t *testing.T) {
	t.Parallel()

	ch := make(chan bool, 1)

	h := func(_ http.ResponseWriter, r *http.Request) {
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

	fs := fsext.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/path/to", 0o755))
	require.NoError(t, fsext.WriteFile(fs, "/path/to/file1.bin", []byte("file1"), 0o644))
	require.NoError(t, fsext.WriteFile(fs, "/path/to/file2.bin", []byte("file2"), 0o644))

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

	bi, err := b.Instantiate(context.Background(), 0)
	require.NoError(t, err)

	logger := logrus.New()
	logger.Level = logrus.DebugLevel
	logger.Out = io.Discard

	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	bi.moduleVUImpl.state = &lib.State{
		Options: lib.Options{},
		Logger:  logger,
		Transport: &http.Transport{
			DialContext: (netext.NewDialer(
				net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 60 * time.Second,
				},
				netext.NewResolver(net.LookupIP, 0, types.DNSfirst, types.DNSpreferIPv4),
			)).DialContext,
		},
		BufferPool:     lib.NewBufferPool(),
		Samples:        make(chan metrics.SampleContainer, 500),
		BuiltinMetrics: builtinMetrics,
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bi.moduleVUImpl.ctx = ctx

	v, err := bi.getCallableExport(consts.DefaultFn)(sobek.Undefined())
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, true, v.Export())

	<-ch
}

func Test__VU(t *testing.T) {
	t.Parallel()
	b, err := getSimpleBundle(t, "/script.js", `
		let vu = __VU;
		export default function() { return vu; }
	`)
	require.NoError(t, err)
	bi, err := b.Instantiate(context.Background(), 5)
	require.NoError(t, err)
	v, err := bi.getCallableExport(consts.DefaultFn)(sobek.Undefined())
	require.NoError(t, err)
	assert.Equal(t, int64(5), v.Export())
}

func TestSourceMaps(t *testing.T) {
	t.Parallel()
	fs := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fs, "/module1.js", []byte(`
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

	bi, err := b.Instantiate(context.Background(), 0)
	require.NoError(t, err)
	_, err = bi.getCallableExport(consts.DefaultFn)(sobek.Undefined())
	require.Error(t, err)
	exception := new(sobek.Exception)
	require.ErrorAs(t, err, &exception)
	require.Equal(t, exception.String(), "exception in line 2\n\tat f2 (file:///module1.js:2:5(2))\n\tat default (file:///script.js:5:15(3))\n")
}

func TestSourceMapsCJS(t *testing.T) {
	t.Parallel()
	fs := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fs, "/module1.js", []byte(`
exports.f2 = function(){
    throw "exception in line 2"
    console.log("in f2")
}
exports.f1 = function(){
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

	bi, err := b.Instantiate(context.Background(), 0)
	require.NoError(t, err)
	_, err = bi.getCallableExport(consts.DefaultFn)(sobek.Undefined())
	require.Error(t, err)
	exception := new(sobek.Exception)
	require.ErrorAs(t, err, &exception)
	require.Equal(t, exception.String(), "exception in line 2\n\tat file:///module1.js:2:5(2)\n\tat default (file:///script.js:5:15(3))\n")
}

func TestSourceMapsExternal(t *testing.T) {
	t.Parallel()
	fs := fsext.NewMemMapFs()
	// This example is created through the template-typescript
	require.NoError(t, fsext.WriteFile(fs, "/test1.js", []byte(`
(()=>{"use strict";var e={};(()=>{var o=e;Object.defineProperty(o,"__esModule",{value:!0}),o.default=function(){!function(e){throw"cool is cool"}()}})();var o=exports;for(var r in e)o[r]=e[r];e.__esModule&&Object.defineProperty(o,"__esModule",{value:!0})})();
//# sourceMappingURL=test1.js.map
`[1:]), 0o644))
	require.NoError(t, fsext.WriteFile(fs, "/test1.js.map", []byte(`
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

	bi, err := b.Instantiate(context.Background(), 0)
	require.NoError(t, err)
	_, err = bi.getCallableExport(consts.DefaultFn)(sobek.Undefined())
	require.Error(t, err)
	exception := new(sobek.Exception)
	require.ErrorAs(t, err, &exception)
	require.Equal(t, "cool is cool\n\tat webpack:///./test1.ts:2:4(2)\n\tat webpack:///./test1.ts:5:4(3)\n\tat default (file:///script.js:4:4(3))\n", exception.String())
}

func TestSourceMapsInlinedCJS(t *testing.T) {
	t.Parallel()
	fs := fsext.NewMemMapFs()
	// this example is from https://github.com/grafana/k6/issues/3689 generated with k6pack
	data := `
var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __hasOwnProp = Object.prototype.hasOwnProperty;
var __defNormalProp = (obj, key, value) => key in obj ? __defProp(obj, key, { enumerable: true, configurable: true, writable: true, value }) : obj[key] = value;
var __export = (target, all) => {
  for (var name in all)
    __defProp(target, name, { get: all[name], enumerable: true });
};
var __copyProps = (to, from, except, desc) => {
  if (from && typeof from === "object" || typeof from === "function") {
    for (let key of __getOwnPropNames(from))
      if (!__hasOwnProp.call(to, key) && key !== except)
        __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
  }
  return to;
};
var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);
var __publicField = (obj, key, value) => {
  __defNormalProp(obj, typeof key !== "symbol" ? key + "" : key, value);
  return value;
};

// script.ts
var script_exports = {};
__export(script_exports, {
  default: () => script_default
});
module.exports = __toCommonJS(script_exports);

// user.ts
var UserAccount = class {
  constructor(name) {
    __publicField(this, "name");
    __publicField(this, "id");
    this.name = name;
    this.id = Math.floor(Math.random() * Number.MAX_SAFE_INTEGER);
    throw "ooops";
  }
};
function newUser(name) {
  return new UserAccount(name);
}

// script.ts
var script_default = () => {
  const user = newUser("John");
  console.log(user);
};
//# sourceMappingURL=data:application/json;base64,ewogICJ2ZXJzaW9uIjogMywKICAic291cmNlcyI6IFsic2NyaXB0LnRzIiwgInVzZXIudHMiXSwKICAic291cmNlc0NvbnRlbnQiOiBbImltcG9ydCB7IFVzZXIsIG5ld1VzZXIgfSBmcm9tIFwiLi91c2VyXCI7XG5cbmV4cG9ydCBkZWZhdWx0ICgpID0+IHtcbiAgY29uc3QgdXNlcjogVXNlciA9IG5ld1VzZXIoXCJKb2huXCIpO1xuICBjb25zb2xlLmxvZyh1c2VyKTtcbn07XG4iLCAiaW50ZXJmYWNlIFVzZXIge1xuICBuYW1lOiBzdHJpbmc7XG4gIGlkOiBudW1iZXI7XG59XG5cbmNsYXNzIFVzZXJBY2NvdW50IGltcGxlbWVudHMgVXNlciB7XG4gIG5hbWU6IHN0cmluZztcbiAgaWQ6IG51bWJlcjtcblxuICBjb25zdHJ1Y3RvcihuYW1lOiBzdHJpbmcpIHtcbiAgICB0aGlzLm5hbWUgPSBuYW1lO1xuICAgIHRoaXMuaWQgPSBNYXRoLmZsb29yKE1hdGgucmFuZG9tKCkgKiBOdW1iZXIuTUFYX1NBRkVfSU5URUdFUik7XG4gICAgdGhyb3cgXCJvb29wc1wiO1xuICB9XG59XG5cbmZ1bmN0aW9uIG5ld1VzZXIobmFtZTogc3RyaW5nKTogVXNlciB7XG4gIHJldHVybiBuZXcgVXNlckFjY291bnQobmFtZSk7XG59XG5cbmV4cG9ydCB7IFVzZXIsIG5ld1VzZXIgfTtcbiJdLAogICJtYXBwaW5ncyI6ICI7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7OztBQUFBO0FBQUE7QUFBQTtBQUFBO0FBQUE7OztBQ0tBLElBQU0sY0FBTixNQUFrQztBQUFBLEVBSWhDLFlBQVksTUFBYztBQUgxQjtBQUNBO0FBR0UsU0FBSyxPQUFPO0FBQ1osU0FBSyxLQUFLLEtBQUssTUFBTSxLQUFLLE9BQU8sSUFBSSxPQUFPLGdCQUFnQjtBQUM1RCxVQUFNO0FBQUEsRUFDUjtBQUNGO0FBRUEsU0FBUyxRQUFRLE1BQW9CO0FBQ25DLFNBQU8sSUFBSSxZQUFZLElBQUk7QUFDN0I7OztBRGhCQSxJQUFPLGlCQUFRLE1BQU07QUFDbkIsUUFBTSxPQUFhLFFBQVEsTUFBTTtBQUNqQyxVQUFRLElBQUksSUFBSTtBQUNsQjsiLAogICJuYW1lcyI6IFtdCn0K
`[1:]
	b, err := getSimpleBundle(t, "/script.js", data, fs)
	require.NoError(t, err)

	bi, err := b.Instantiate(context.Background(), 0)
	require.NoError(t, err)
	_, err = bi.getCallableExport(consts.DefaultFn)(sobek.Undefined())
	require.Error(t, err)
	exception := new(sobek.Exception)
	require.ErrorAs(t, err, &exception)
	// TODO figure out why those are not the same as the one in the previous test TestSourceMapsExternal
	// likely settings in the transpilers
	require.Equal(t, "ooops\n\tat file:///user.ts:13:4(28)\n\tat newUser (file:///user.ts:18:9(3))\n\tat script_default (file:///script.ts:4:29(4))\n", exception.String())
}

func TestImportModificationsAreConsistentBetweenFiles(t *testing.T) {
	t.Parallel()
	fs := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fs, "/notk6.js", []byte(`export default {group}; function group() {}`), 0o644))
	require.NoError(t, fsext.WriteFile(fs, "/instrument.js", []byte(`
    import k6 from "k6";
    k6.newKey = 5;
    k6.group = 3;

    import notk6 from "./notk6.js";
    notk6.group = 3;
    notk6.newKey = 5;
    `), 0o644))

	b, err := getSimpleBundle(t, "/script.js", `
    import k6 from "k6";
    import notk6 from "./notk6.js";
    import "./instrument.js";
    if (k6.newKey != 5) { throw "k6.newKey is wrong "+ k6.newKey}
    if (k6.group != 3) { throw "k6.group is wrong "+ k6.group}
    if (notk6.newKey != 5) { throw "notk6.newKey is wrong "+ notk6.newKey}
    if (notk6.group != 3) { throw "notk6.group is wrong "+ notk6.group}
    export default () => { throw "this shouldn't be ran" }
`, fs)
	require.NoError(t, err, "bundle error")

	_, err = b.Instantiate(context.Background(), 0)
	require.NoError(t, err)
}

func TestCacheAbsolutePathsNotRelative(t *testing.T) {
	t.Parallel()
	fs := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fs, "/a/interesting.js", []byte(`export default "a.interesting"`), 0o644))
	require.NoError(t, fsext.WriteFile(fs, "/a/import.js", []byte(`export { default as default} from "./interesting.js"`), 0o644))

	require.NoError(t, fsext.WriteFile(fs, "/b/interesting.js", []byte(`export default "b.interesting"`), 0o644))
	require.NoError(t, fsext.WriteFile(fs, "/b/import.js", []byte(`export { default as default} from "./interesting.js"`), 0o644))

	b, err := getSimpleBundle(t, "/script.js", `
    import a from "/a/import.js"
    import b from "/b/import.js"
    if (a != "a.interesting") { throw `+"`"+`'a' has wrong value "${a}" should be "a.interesting"`+"`"+`}

    if (b != "b.interesting") { throw `+"`"+`'b' has wrong value "${b}" should be "b.interesting"`+"`"+`}
    export default () => { throw "this shouldn't be ran" }
`, fs)
	require.NoError(t, err, "bundle error")

	_, err = b.Instantiate(context.Background(), 0)
	require.NoError(t, err)
}
