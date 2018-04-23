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
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/oxtoacart/bpool"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestInitContextRequire(t *testing.T) {
	t.Run("Modules", func(t *testing.T) {
		t.Run("Nonexistent", func(t *testing.T) {
			_, err := getSimpleBundle("/script.js", `import "k6/NONEXISTENT";`)
			assert.EqualError(t, err, "GoError: unknown builtin module: k6/NONEXISTENT")
		})

		t.Run("k6", func(t *testing.T) {
			b, err := getSimpleBundle("/script.js", `
					import k6 from "k6";
					export let _k6 = k6;
					export let dummy = "abc123";
					export default function() {}
			`)
			if !assert.NoError(t, err, "bundle error") {
				return
			}

			bi, err := b.Instantiate()
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
				b, err := getSimpleBundle("/script.js", `
						import { group } from "k6";
						export let _group = group;
						export let dummy = "abc123";
						export default function() {}
				`)
				if !assert.NoError(t, err) {
					return
				}

				bi, err := b.Instantiate()
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
			_, err := getSimpleBundle("/script.js", `import "/nonexistent.js"; export default function() {}`)
			assert.EqualError(t, err, "GoError: open /nonexistent.js: file does not exist")
		})
		t.Run("Invalid", func(t *testing.T) {
			fs := afero.NewMemMapFs()
			assert.NoError(t, afero.WriteFile(fs, "/file.js", []byte{0x00}, 0755))
			_, err := NewBundle(&lib.SourceData{
				Filename: "/script.js",
				Data:     []byte(`import "/file.js"; export default function() {}`),
			}, fs, lib.RuntimeOptions{})
			assert.EqualError(t, err, "SyntaxError: /file.js: Unexpected character '\x00' (1:0)\n> 1 | \x00\n    | ^ at <eval>:2:26853(114)")
		})
		t.Run("Error", func(t *testing.T) {
			fs := afero.NewMemMapFs()
			assert.NoError(t, afero.WriteFile(fs, "/file.js", []byte(`throw new Error("aaaa")`), 0755))
			_, err := NewBundle(&lib.SourceData{
				Filename: "/script.js",
				Data:     []byte(`import "/file.js"; export default function() {}`),
			}, fs, lib.RuntimeOptions{})
			assert.EqualError(t, err, "Error: aaaa at /file.js:1:19(4)")
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
			t.Run("lib=\""+libName+"\"", func(t *testing.T) {
				for constName, constPath := range data.ConstPaths {
					name := "inline"
					if constName != "" {
						name = "const=\"" + constName + "\""
					}
					t.Run(name, func(t *testing.T) {
						fs := afero.NewMemMapFs()
						src := &lib.SourceData{
							Filename: `/path/to/script.js`,
							Data: []byte(fmt.Sprintf(`
								import fn from "%s";
								let v = fn();
								export default function() {
								};
							`, libName)),
						}

						jsLib := `export default function() { return 12345; }`
						if constName != "" {
							jsLib = fmt.Sprintf(
								`import { c } from "%s"; export default function() { return c; }`,
								constName,
							)

							constsrc := `export let c = 12345;`
							assert.NoError(t, fs.MkdirAll(filepath.Dir(constPath), 0755))
							assert.NoError(t, afero.WriteFile(fs, constPath, []byte(constsrc), 0644))
						}

						assert.NoError(t, fs.MkdirAll(filepath.Dir(data.LibPath), 0755))
						assert.NoError(t, afero.WriteFile(fs, data.LibPath, []byte(jsLib), 0644))

						b, err := NewBundle(src, fs, lib.RuntimeOptions{})
						if !assert.NoError(t, err) {
							return
						}
						if constPath != "" {
							assert.Contains(t, b.BaseInitContext.programs, constPath)
						}

						_, err = b.Instantiate()
						if !assert.NoError(t, err) {
							return
						}
					})
				}
			})
		}

		t.Run("Isolation", func(t *testing.T) {
			fs := afero.NewMemMapFs()
			assert.NoError(t, afero.WriteFile(fs, "/a.js", []byte(`const myvar = "a";`), 0644))
			assert.NoError(t, afero.WriteFile(fs, "/b.js", []byte(`const myvar = "b";`), 0644))
			b, err := NewBundle(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
				import "./a.js";
				import "./b.js";
				export default function() {
					if (typeof myvar != "undefined") {
						throw new Error("myvar is set in global scope");
					}
				};
				`),
			}, fs, lib.RuntimeOptions{})
			if !assert.NoError(t, err) {
				return
			}

			bi, err := b.Instantiate()
			if !assert.NoError(t, err) {
				return
			}
			_, err = bi.Default(goja.Undefined())
			assert.NoError(t, err)
		})
	})
}

func TestInitContextOpen(t *testing.T) {
	fs := afero.NewMemMapFs()
	assert.NoError(t, fs.MkdirAll("/path/to", 0755))
	assert.NoError(t, afero.WriteFile(fs, "/path/to/file.txt", []byte("hi!"), 0644))

	testdata := map[string]string{
		"Absolute": "/path/to/file.txt",
		"Relative": "./file.txt",
	}
	for name, loadPath := range testdata {
		t.Run(name, func(t *testing.T) {
			b, err := NewBundle(&lib.SourceData{
				Filename: "/path/to/script.js",
				Data: []byte(fmt.Sprintf(`
				export let data = open("%s");
				export default function() {}
				`, loadPath)),
			}, fs, lib.RuntimeOptions{})
			if !assert.NoError(t, err) {
				return
			}

			bi, err := b.Instantiate()
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, "hi!", bi.Runtime.Get("data").Export())
		})
	}

	t.Run("Nonexistent", func(t *testing.T) {
		_, err := NewBundle(&lib.SourceData{
			Filename: "/script.js",
			Data:     []byte(`open("/nonexistent.txt"); export default function() {}`),
		}, fs, lib.RuntimeOptions{})
		assert.EqualError(t, err, "GoError: open /nonexistent.txt: file does not exist")
	})
}

func TestInitContextOpenBinary(t *testing.T) {
	fs := afero.NewMemMapFs()
	assert.NoError(t, fs.MkdirAll("/path/to", 0755))
	assert.NoError(t, afero.WriteFile(fs, "/path/to/file.bin", []byte("hi!"), 0644))

	b, err := NewBundle(&lib.SourceData{
		Filename: "/path/to/script.js",
		Data: []byte(`
		export let data = open("/path/to/file.bin", "b");
		export default function() { console.log(data); }
		`),
	}, fs, lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	bi, err := b.Instantiate()
	if !assert.NoError(t, err) {
		t.Log(err)
		return
	}

	bytes := []byte{104, 105, 33}
	assert.Equal(t, bytes, bi.Runtime.Get("data").Export())
}

func TestRequestWithBinaryFile(t *testing.T) {
	t.Parallel()

	ch := make(chan bool, 1)

	h := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			ch <- true
		}()

		r.ParseMultipartForm(32 << 20)
		file, _, err := r.FormFile("file")
		assert.NoError(t, err)
		defer file.Close()
		bytes := make([]byte, 3)
		_, err = file.Read(bytes)
		assert.NoError(t, err)
		assert.Equal(t, []byte("hi!"), bytes)
		assert.Equal(t, "this is a standard form field", r.FormValue("field"))
	}

	srv := httptest.NewServer(http.HandlerFunc(h))
	defer srv.Close()

	fs := afero.NewMemMapFs()
	assert.NoError(t, fs.MkdirAll("/path/to", 0755))
	assert.NoError(t, afero.WriteFile(fs, "/path/to/file.bin", []byte("hi!"), 0644))

	b, err := NewBundle(&lib.SourceData{
		Filename: "/path/to/script.js",
		Data: []byte(fmt.Sprintf(`
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
			`, srv.URL)),
	}, fs, lib.RuntimeOptions{})
	assert.NoError(t, err)

	bi, err := b.Instantiate()
	assert.NoError(t, err)

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	logger := log.New()
	logger.Level = log.DebugLevel
	logger.Out = ioutil.Discard

	state := &common.State{
		Options: lib.Options{},
		Logger:  logger,
		Group:   root,
		HTTPTransport: &http.Transport{
			DialContext: (netext.NewDialer(net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 60 * time.Second,
				DualStack: true,
			})).DialContext,
		},
		BPool: bpool.NewBufferPool(1),
	}

	ctx := context.Background()
	ctx = common.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, bi.Runtime)
	*bi.Context = ctx

	v, err := bi.Default(goja.Undefined())
	assert.NoError(t, err)
	assert.NotNil(t, v)
	assert.Equal(t, true, v.Export())

	<-ch
}
