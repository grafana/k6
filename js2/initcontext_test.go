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

package js2

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/lib"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestInitContextRequire(t *testing.T) {
	t.Run("Modules", func(t *testing.T) {
		t.Run("k6", func(t *testing.T) {
			b, err := NewBundle(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
				import k6 from "k6";
				export let _k6 = k6;
				export let dummy = "abc123";
				export default function() {}
			`),
			}, afero.NewMemMapFs())
			if !assert.NoError(t, err, "bundle error") {
				return
			}

			bi, err := b.Instantiate()
			if !assert.NoError(t, err, "instance error") {
				return
			}
			assert.Contains(t, b.BaseInitContext.Modules, "k6")
			assert.Contains(t, bi.Modules, "k6")

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
				b, err := NewBundle(&lib.SourceData{
					Filename: "/script.js",
					Data: []byte(`
					import { group } from "k6";
					export let _group = group;
					export let dummy = "abc123";
					export default function() {}
				`),
				}, afero.NewMemMapFs())
				if !assert.NoError(t, err) {
					return
				}

				bi, err := b.Instantiate()
				if !assert.NoError(t, err) {
					return
				}
				assert.Contains(t, b.BaseInitContext.Modules, "k6")
				assert.Contains(t, bi.Modules, "k6")

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
		imports := map[string]struct {
			LibPath    string
			ConstPaths map[string]string
		}{
			"lib.js": {"/path/to/lib.js", map[string]string{
				"":             "",
				"const.js":     "/path/to/const.js",
				"../const.js":  "/path/const.js",
				"sub/const.js": "/path/to/sub/const.js",
			}},
			"./lib.js": {"/path/to/lib.js", map[string]string{
				"":             "",
				"const.js":     "/path/to/const.js",
				"../const.js":  "/path/const.js",
				"sub/const.js": "/path/to/sub/const.js",
			}},
			"../lib.js": {"/path/lib.js", map[string]string{
				"":             "",
				"const.js":     "/path/const.js",
				"../const.js":  "/const.js",
				"sub/const.js": "/path/sub/const.js",
			}},
			"dir/lib.js": {"/path/to/dir/lib.js", map[string]string{
				"":             "",
				"const.js":     "/path/to/dir/const.js",
				"../const.js":  "/path/to/const.js",
				"sub/const.js": "/path/to/dir/sub/const.js",
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

						lib := `export default function() { return 12345; }`
						if constName != "" {
							lib = fmt.Sprintf(
								`import { c } from "%s"; export default function() { return c; }`,
								constName,
							)

							constsrc := `export let c = 12345;`
							assert.NoError(t, fs.MkdirAll(filepath.Dir(constPath), 0755))
							assert.NoError(t, afero.WriteFile(fs, constPath, []byte(constsrc), 0644))
						}

						assert.NoError(t, fs.MkdirAll(filepath.Dir(data.LibPath), 0755))
						assert.NoError(t, afero.WriteFile(fs, data.LibPath, []byte(lib), 0644))

						b, err := NewBundle(src, fs)
						if !assert.NoError(t, err) {
							return
						}

						_, err = b.Instantiate()
						if !assert.NoError(t, err) {
							return
						}
					})
				}
			})
		}
	})
}
