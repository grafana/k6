/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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

//go:generate rice embed-go

package lib

import (
	"path"

	rice "github.com/GeertJohan/go.rice"
	"github.com/dop251/goja"
)

//nolint:gochecknoglobals
var (
	compiled map[string]*goja.Program
)

func init() {
	var list = []string{
		"_add-to-unscopables.js",
		"_a-function.js",
		"_an-object.js",
		"_array-from-iterable.js",
		"_array-includes.js",
		"_array-species-constructor.js",
		"_array-species-create.js",
		"_classof.js",
		"_cof.js",
		"core.get-iterator-method.js",
		"_core.js",
		"_create-property.js",
		"_ctx.js",
		"_defined.js",
		"_descriptors.js",
		"_enum-bug-keys.js",
		"es7.array.flat-map.js",
		"es7.array.flatten.js",
		"es7.object.define-getter.js",
		"es7.object.define-setter.js",
		"es7.object.entries.js",
		"es7.object.get-own-property-descriptors.js",
		"es7.object.lookup-getter.js",
		"es7.object.lookup-setter.js",
		"es7.object.values.js",
		"es7.reflect.define-metadata.js",
		"es7.reflect.delete-metadata.js",
		"es7.reflect.get-metadata.js",
		"es7.reflect.get-metadata-keys.js",
		"es7.reflect.get-own-metadata.js",
		"es7.reflect.get-own-metadata-keys.js",
		"es7.reflect.has-metadata.js",
		"es7.reflect.has-own-metadata.js",
		"es7.reflect.metadata.js",
		"es7.string.match-all.js",
		"es7.string.pad-end.js",
		"es7.string.pad-start.js",
		"es7.string.trim-left.js",
		"es7.string.trim-right.js",
		"_export.js",
		"_fails.js",
		"_flags.js",
		"_flatten-into-array.js",
		"_for-of.js",
		"_global.js",
		"_has.js",
		"_hide.js",
		"_ie8-dom-define.js",
		"_iobject.js",
		"_is-array-iter.js",
		"_is-array.js",
		"_is-object.js",
		"_is-regexp.js",
		"_iterators.js",
		"_iter-call.js",
		"_iter-create.js",
		"_library.js",
		"_metadata.js",
		"_object-create.js",
		"_object-dp.js",
		"_object-dps.js",
		"_object-forced-pam.js",
		"_object-gopd.js",
		"_object-gopn.js",
		"_object-gops.js",
		"_object-gpo.js",
		"_object-keys-internal.js",
		"_object-keys.js",
		"_object-pie.js",
		"_object-to-array.js",
		"_own-keys.js",
		"_property-desc.js",
		"_redefine.js",
		"_set-to-string-tag.js",
		"_shared.js",
		"_shared-key.js",
		"_string-pad.js",
		"_string-repeat.js",
		"_string-trim.js",
		"_string-ws.js",
		"_to-absolute-index.js",
		"_to-integer.js",
		"_to-iobject.js",
		"_to-length.js",
		"_to-object.js",
		"_to-primitive.js",
		"_uid.js",
		"_wks.js",
	}
	compiled = make(map[string]*goja.Program, len(list))
	for _, name := range list {
		compiled[name] = goja.MustCompile(
			name,

			"(function(module, exports){\n"+
				rice.MustFindBox("core-js").MustString(name)+"\n})",
			true)
	}
}

// AddPolyfills adds the polyfils to the provided runtime
func AddPolyfills(rt *goja.Runtime) error {
	// TODO refactor this ... maybe merge with the init context code for this ...
	var pwd string // TODO this might not be needed as they are in the same folder

	// Cache of loaded programs and files.
	modules := make(map[string]*goja.Object)
	rt.Set("require", func(str string) (goja.Value, error) {
		switch str {
		case "./es6.set":
			return rt.RunScript("es6.set.js", "Set")
		case "./es6.map":
			return rt.RunScript("es6.map.js", "Map")

		case "./es6.weak-map":
			return rt.RunScript("es6.weak-map.js", "WeakMap")
		}
		// fmt.Println(str)
		filename := path.Join(pwd, str) + ".js"

		// First, check if we have a cached program already.
		module, ok := modules[filename]
		if !ok {
			// TODO this is technically not needed as they are all in the same folder currently
			defer func(backPwd string) { pwd = backPwd }(pwd)
			pwd = path.Dir(filename)
			exports := rt.NewObject()
			module = rt.NewObject()
			_ = module.Set("exports", exports)

			modules[filename] = module

			// Run the program.
			f, err := rt.RunProgram(compiled[filename])
			if err != nil {
				delete(modules, filename)
				return goja.Undefined(), err
			}
			if call, ok := goja.AssertFunction(f); ok {
				if _, err = call(exports, module, exports); err != nil {
					return nil, err
				}
			}
		}

		return module.Get("exports"), nil
	})

	defer func() {
		_ = rt.GlobalObject().Delete("require")
	}()

	_, err := rt.RunScript("core-js.shim.js", `
// require('es6.promise'); // async
require('es7.array.flat-map');
require('es7.array.flatten'); // this is now called flat, so maybe drop it
// require('es7.string.at'); // it is in es2020 but is with completely different semantics
require('es7.string.pad-start');
require('es7.string.pad-end');
require('es7.string.trim-left');
require('es7.string.trim-right');
require('es7.string.match-all');
// require('es7.symbol.async-iterator'); // async
// require('es7.symbol.observable'); // async
require('es7.object.get-own-property-descriptors');
require('es7.object.values');
require('es7.object.entries');
require('es7.object.define-getter');
require('es7.object.define-setter');
require('es7.object.lookup-getter');
require('es7.object.lookup-setter');
// require('es7.map.to-json'); // All of this are dropped
// require('es7.set.to-json');
// require('es7.map.of');
// require('es7.set.of');
// require('es7.weak-map.of');
// require('es7.weak-set.of');
// require('es7.map.from');
// require('es7.set.from');
// require('es7.weak-map.from');
// require('es7.weak-set.from');
// require('es7.global'); // is now globalThis, goja has globasl
// require('es7.system.global'); // dropped .. I think
// require('es7.error.is-error'); // dropped
// require('es7.promise.finally'); // async
// require('es7.promise.try'); // async
require('es7.reflect.define-metadata');
require('es7.reflect.delete-metadata');
require('es7.reflect.get-metadata');
require('es7.reflect.get-metadata-keys');
require('es7.reflect.get-own-metadata');
require('es7.reflect.get-own-metadata-keys');
require('es7.reflect.has-metadata');
require('es7.reflect.has-own-metadata');
require('es7.reflect.metadata');
// require('es7.asap'); // async
// require('es7.observable'); // async
	`)
	if err != nil {
		return err
	}
	/* This code is here to check if we should remove some files
	// TODO to be moved to a test
	var notNeeded []string
	rice.MustFindBox("core-js").Walk("", func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			if _, ok := modules[path]; !ok {
				notNeeded = append(notNeeded, path)
			}
		}
		return nil
	})
	fmt.Println(notNeeded)
	//*/

	return nil
}
