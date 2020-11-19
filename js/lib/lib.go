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
	"sync"
	"time"

	rice "github.com/GeertJohan/go.rice"
	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/compiler"
	"github.com/loadimpact/k6/lib"
)

//nolint:gochecknoglobals
var (
	once   sync.Once
	coreJs *goja.Program
)

func GetCoreJS() *goja.Program {
	once.Do(func() {
		coreJs = goja.MustCompile(
			"core-js/shim.min.js",
			rice.MustFindBox("core-js").MustString("shim.min.js"),
			true,
		)
	})

	return coreJs
}

type programWithSource struct {
	pgm    *goja.Program
	src    string
	module *goja.Object
}

// AddPolyfills adds the polyfils to the provided runtime
func AddPolyfills(rt *goja.Runtime) error {
	// TODO refactor this ... maybe merge with the init context code for this ...
	var mainPwd string
	comp := compiler.New(nil)

	// Cache of loaded programs and files.
	programs := make(map[string]programWithSource)
	rt.Set("require", func(str string) (goja.Value, error) {
		// fmt.Println(str)
		pwd := mainPwd
		fileURL := path.Join(pwd, str) + ".js"

		// First, check if we have a cached program already.
		pgm, ok := programs[fileURL]
		if !ok || pgm.module == nil {
			mainPwd = path.Dir(fileURL)
			defer func() { mainPwd = pwd }()
			exports := rt.NewObject()
			pgm.module = rt.NewObject()
			_ = pgm.module.Set("exports", exports)
			var err error

			if pgm.pgm == nil {
				// Load the sources; the loader takes care of remote loading, etc.
				// TODO: don't use the Global logger
				data, err := rice.MustFindBox("core-js").String(fileURL)
				if err != nil {
					fileURL := fileURL
					go func() { panic(fileURL) }()
					time.Sleep(time.Second)
				}

				pgm.src = string(data)

				pgm.pgm, _, err = comp.Compile(pgm.src, fileURL,
					"(function(module, exports){\n", "\n})\n", true, lib.CompatibilityModeBase)
				if err != nil {
					return goja.Undefined(), err
				}
			}

			programs[fileURL] = pgm

			// Run the program.
			f, err := rt.RunProgram(pgm.pgm)
			if err != nil {
				delete(programs, fileURL)
				return goja.Undefined(), err
			}
			if call, ok := goja.AssertFunction(f); ok {
				if _, err = call(exports, pgm.module, exports); err != nil {
					return nil, err
				}
			}
		}

		return pgm.module.Get("exports"), nil
	})

	defer func() {
		_ = rt.GlobalObject().Delete("require")
	}()

	_, err := rt.RunScript("core-js.shim.js", `
require('es6.promise');
require('es7.array.flat-map');
require('es7.array.flatten');
require('es7.string.at');
require('es7.string.pad-start');
require('es7.string.pad-end');
require('es7.string.trim-left');
require('es7.string.trim-right');
require('es7.string.match-all');
require('es7.symbol.async-iterator');
require('es7.symbol.observable');
require('es7.object.get-own-property-descriptors');
require('es7.object.values');
require('es7.object.entries');
require('es7.object.define-getter');
require('es7.object.define-setter');
require('es7.object.lookup-getter');
require('es7.object.lookup-setter');
require('es7.map.to-json');
require('es7.set.to-json');
require('es7.map.of');
require('es7.set.of');
require('es7.weak-map.of');
require('es7.weak-set.of');
require('es7.map.from');
require('es7.set.from');
require('es7.weak-map.from');
require('es7.weak-set.from');
require('es7.global');
require('es7.system.global');
require('es7.error.is-error');
require('es7.promise.finally');
require('es7.promise.try');
require('es7.reflect.define-metadata');
require('es7.reflect.delete-metadata');
require('es7.reflect.get-metadata');
require('es7.reflect.get-metadata-keys');
require('es7.reflect.get-own-metadata');
require('es7.reflect.get-own-metadata-keys');
require('es7.reflect.has-metadata');
require('es7.reflect.has-own-metadata');
require('es7.reflect.metadata');
require('es7.asap');
require('es7.observable');
	`)
	if err != nil {
		return err
	}
	/* This code is here to check if we should remove some files
	// TODO to be moved to a test
	var notNeeded []string
	rice.MustFindBox("core-js").Walk("", func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			if _, ok := programs[path]; !ok {
				notNeeded = append(notNeeded, path)
			}
		}
		return nil
	})
	fmt.Println(notNeeded)
	*/

	return nil
}
