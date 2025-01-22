package js

import (
	"bytes"
	"context"
	"io/fs"
	"testing"
	"time"

	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/types"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/internal/lib/testutils/httpmultibin"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

func newDevNullSampleChannel() chan metrics.SampleContainer {
	ch := make(chan metrics.SampleContainer, 100)
	go func() {
		for range ch { //nolint:revive
		}
	}()
	return ch
}

func TestLoadOnceGlobalVars(t *testing.T) {
	t.Parallel()
	testCases := map[string]string{
		"module.exports": `
			var globalVar;
			if (!globalVar) {
				globalVar = Math.random();
			}
			function C() {
				return globalVar;
			}
			module.exports = {
				C: C,
			}
		`,
		"direct export": `

			var globalVar;
			if (!globalVar) {
				globalVar = Math.random();
			}
			export function C() {
				return globalVar;
			}
		`,
	}
	for name, data := range testCases {
		cData := data
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fileSystem := fsext.NewMemMapFs()
			require.NoError(t, fsext.WriteFile(fileSystem, "/C.js", []byte(cData), fs.ModePerm))

			require.NoError(t, fsext.WriteFile(fileSystem, "/A.js", []byte(`
		import { C } from "./C.js";
		export function A() {
			return C();
		}
	`), fs.ModePerm))
			require.NoError(t, fsext.WriteFile(fileSystem, "/B.js", []byte(`
		var  c = require("./C.js");
		export function B() {
			return c.C();
		}
	`), fs.ModePerm))
			r1, err := getSimpleRunner(t, "/script.js", `
			import { A } from "./A.js";
			import { B } from "./B.js";

			export default function(data) {
				if (A() === undefined) {
					throw new Error("A() is undefined");
				}
				if (A() != B()) {
					throw new Error("A() != B()    (" + A() + ") != (" + B() + ")");
				}
			}
		`, fileSystem, lib.RuntimeOptions{CompatibilityMode: null.StringFrom("extended")})
			require.NoError(t, err)

			arc := r1.MakeArchive()
			r2, err := getSimpleArchiveRunner(t, arc)
			require.NoError(t, err)

			runners := map[string]*Runner{"Source": r1, "Archive": r2}
			for name, r := range runners {
				r := r
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					ch := newDevNullSampleChannel()
					defer close(ch)

					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					initVU, err := r.NewVU(ctx, 1, 1, ch)
					require.NoError(t, err)
					vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
					require.NoError(t, vu.RunOnce())
				})
			}
		})
	}
}

func TestLoadExportsIsntUsableInModule(t *testing.T) {
	t.Parallel()
	fileSystem := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fileSystem, "/A.js", []byte(`
		export function A() {
			return "A";
		}
		export function B() {
			return exports.A() + "B";
		}
	`), fs.ModePerm))
	r1, err := getSimpleRunner(t, "/script.js", `
			import { A, B } from "./A.js";

			export default function(data) {
				if (A() != "A") {
					throw new Error("wrong value of A() " + A());
				}

				if (B() != "AB") {
					throw new Error("wrong value of B() " + B());
				}
			}
		`, fileSystem, lib.RuntimeOptions{CompatibilityMode: null.StringFrom("extended")})
	require.NoError(t, err)

	arc := r1.MakeArchive()
	r2, err := getSimpleArchiveRunner(t, arc)
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ch := newDevNullSampleChannel()
			defer close(ch)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, ch)
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			require.ErrorContains(t, vu.RunOnce(), `you are trying to access identifier "exports"`)
		})
	}
}

func TestLoadDoesntBreakHTTPGet(t *testing.T) {
	t.Parallel()
	// This test that functions such as http.get which require context still work if they are called
	// inside script that is imported

	tb := httpmultibin.NewHTTPMultiBin(t)
	fileSystem := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fileSystem, "/A.js", []byte(tb.Replacer.Replace(`
		import http from "k6/http";
		export function A() {
			return http.get("HTTPBIN_URL/get");
		}
	`)), fs.ModePerm))
	r1, err := getSimpleRunner(t, "/script.js", `
			import { A } from "./A.js";

			export default function(data) {
				let resp = A();
				if (resp.status != 200) {
					throw new Error("wrong status "+ resp.status);
				}
			}
		`, fileSystem, lib.RuntimeOptions{CompatibilityMode: null.StringFrom("extended")})
	require.NoError(t, err)

	require.NoError(t, r1.SetOptions(lib.Options{Hosts: types.NullHosts{Trie: tb.Dialer.Hosts}}))
	arc := r1.MakeArchive()
	r2, err := getSimpleArchiveRunner(t, arc)
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ch := newDevNullSampleChannel()
			defer close(ch)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, ch)
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			require.NoError(t, vu.RunOnce())
		})
	}
}

func TestLoadGlobalVarsAreNotSharedBetweenVUs(t *testing.T) {
	t.Parallel()
	fileSystem := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fileSystem, "/A.js", []byte(`
		var globalVar = 0;
		export function A() {
			globalVar += 1
			return globalVar;
		}
	`), fs.ModePerm))
	r1, err := getSimpleRunner(t, "/script.js", `
			import { A } from "./A.js";

			export default function(data) {
				var a = A();
				if (a == 1) {
					a = 2;
				} else {
					throw new Error("wrong value of a " + a);
				}
			}
		`, fileSystem, lib.RuntimeOptions{CompatibilityMode: null.StringFrom("extended")})
	require.NoError(t, err)

	arc := r1.MakeArchive()
	r2, err := getSimpleArchiveRunner(t, arc)
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ch := newDevNullSampleChannel()
			defer close(ch)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, ch)
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			require.NoError(t, vu.RunOnce())

			// run a second VU
			ctx, cancel = context.WithCancel(context.Background())
			defer cancel()
			initVU, err = r.NewVU(ctx, 2, 2, ch)
			require.NoError(t, err)
			vu = initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			require.NoError(t, vu.RunOnce())
		})
	}
}

func TestLoadCycle(t *testing.T) {
	t.Parallel()
	// This is mostly the example from https://hacks.mozilla.org/2018/03/es-modules-a-cartoon-deep-dive/
	fileSystem := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fileSystem, "/counter.js", []byte(`
			let main = require("./main.js");
			exports.count = 5;
			exports.a = function() {
				return main.message;
			}
	`), fs.ModePerm))

	require.NoError(t, fsext.WriteFile(fileSystem, "/main.js", []byte(`
			let counter = require("./counter.js");
			let count = counter.count;
			let a = counter.a;
			exports.message = "Eval complete";

			exports.default = function() {
				if (count != 5) {
					throw new Error("Wrong value of count "+ count);
				}
				let aMessage = a();
				if (aMessage != exports.message) {
					throw new Error("Wrong value of a() "+ aMessage);
				}
			}
	`), fs.ModePerm))
	data, err := fsext.ReadFile(fileSystem, "/main.js")
	require.NoError(t, err)
	r1, err := getSimpleRunner(t, "/main.js", string(data), fileSystem, lib.RuntimeOptions{CompatibilityMode: null.StringFrom("extended")})
	require.NoError(t, err)

	arc := r1.MakeArchive()
	r2, err := getSimpleArchiveRunner(t, arc)
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ch := newDevNullSampleChannel()
			defer close(ch)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, ch)
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			require.NoError(t, vu.RunOnce())
		})
	}
}

func TestLoadCycleBinding(t *testing.T) {
	t.Parallel()
	// This is mostly the example from
	// http://2ality.com/2015/07/es6-module-exports.html#why-export-bindings
	fileSystem := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fileSystem, "/a.js", []byte(`
		import {bar} from './b.js';
		export function foo(a) {
				if (a !== undefined) {
					return "foo" + a;
				}
				return "foo" + bar(3);
		}
	`), fs.ModePerm))

	require.NoError(t, fsext.WriteFile(fileSystem, "/b.js", []byte(`
		import {foo} from './a.js';
		export function bar(a) {
				if (a !== undefined) {
					return "bar" + a;
				}
				return "bar" + foo(5);
			}
	`), fs.ModePerm))

	r1, err := getSimpleRunner(t, "/main.js", `
			import {foo} from './a.js';
			import {bar} from './b.js';
			export default function() {
				let fooMessage = foo();
				if (fooMessage != "foobar3") {
					throw new Error("Wrong value of foo() "+ fooMessage);
				}
				let barMessage = bar();
				if (barMessage != "barfoo5") {
					throw new Error("Wrong value of bar() "+ barMessage);
				}
			}
		`, fileSystem, lib.RuntimeOptions{CompatibilityMode: null.StringFrom("extended")})
	require.NoError(t, err)

	arc := r1.MakeArchive()
	r2, err := getSimpleArchiveRunner(t, arc)
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ch := newDevNullSampleChannel()
			defer close(ch)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, ch)
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			require.NoError(t, vu.RunOnce())
		})
	}
}

func TestBrowserified(t *testing.T) {
	t.Parallel()
	fileSystem := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fileSystem, "/browserified.js", []byte(`
		(function(f){if(typeof exports==="object"&&typeof module!=="undefined"){module.exports=f()}else if(typeof define==="function"&&define.amd){define([],f)}else{var g;if(typeof window!=="undefined"){g=window}else if(typeof global!=="undefined"){g=global}else if(typeof self!=="undefined"){g=self}else{g=this}g.npmlibs = f()}})(function(){var define,module,exports;return (function(){function r(e,n,t){function o(i,f){if(!n[i]){if(!e[i]){var c="function"==typeof require&&require;if(!f&&c)return c(i,!0);if(u)return u(i,!0);var a=new Error("Cannot find module '"+i+"'");throw a.code="MODULE_NOT_FOUND",a}var p=n[i]={exports:{}};e[i][0].call(p.exports,function(r){var n=e[i][1][r];return o(n||r)},p,p.exports,r,e,n,t)}return n[i].exports}for(var u="function"==typeof require&&require,i=0;i<t.length;i++)o(t[i]);return o}return r})()({1:[function(require,module,exports){
		module.exports.A = function () {
			return "a";
		}

		},{}],2:[function(require,module,exports){
		exports.B = function() {
		return "b";
		}

		},{}],3:[function(require,module,exports){
		exports.alpha = require('./a.js');
		exports.bravo = require('./b.js');

		},{"./a.js":1,"./b.js":2}]},{},[3])(3)
		});
	`), fs.ModePerm))

	r1, err := getSimpleRunner(t, "/script.js", `
			import {alpha, bravo } from "./browserified.js";

			export default function(data) {
				if (alpha.A === undefined) {
					throw new Error("alpha.A is undefined");
				}
				if (alpha.A() != "a") {
					throw new Error("alpha.A() != 'a'    (" + alpha.A() + ") != 'a'");
				}

				if (bravo.B === undefined) {
					throw new Error("bravo.B is undefined");
				}
				if (bravo.B() != "b") {
					throw new Error("bravo.B() != 'b'    (" + bravo.B() + ") != 'b'");
				}
			}
		`, fileSystem, lib.RuntimeOptions{CompatibilityMode: null.StringFrom("extended")})
	require.NoError(t, err)

	arc := r1.MakeArchive()
	r2, err := getSimpleArchiveRunner(t, arc)
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ch := make(chan metrics.SampleContainer, 100)
			defer close(ch)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, ch)
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			require.NoError(t, vu.RunOnce())
		})
	}
}

func TestLoadingUnexistingModuleDoesntPanic(t *testing.T) {
	t.Parallel()
	fileSystem := fsext.NewMemMapFs()
	data := `var b;
			try {
				b = eval("require('buffer')");
			} catch (err) {
				b = "correct";
			}
			exports.default = function() {
				if (b != "correct") {
					throw new Error("wrong b "+ JSON.stringify(b));
				}
			}`
	require.NoError(t, fsext.WriteFile(fileSystem, "/script.js", []byte(data), 0o644))
	r1, err := getSimpleRunner(t, "/script.js", data, fileSystem)
	require.NoError(t, err)

	arc := r1.MakeArchive()
	buf := &bytes.Buffer{}
	require.NoError(t, arc.Write(buf))
	arc, err = lib.ReadArchive(buf)
	require.NoError(t, err)
	r2, err := getSimpleArchiveRunner(t, arc)
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ch := newDevNullSampleChannel()
			defer close(ch)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, ch)
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			require.NoError(t, vu.RunOnce())
		})
	}
}

func TestLoadingSourceMapsDoesntErrorOut(t *testing.T) {
	t.Parallel()

	fileSystem := fsext.NewMemMapFs()
	data := `exports.default = function() {}
//# sourceMappingURL=test.min.js.map`
	require.NoError(t, fsext.WriteFile(fileSystem, "/script.js", []byte(data), 0o644))
	r1, err := getSimpleRunner(t, "/script.js", data, fileSystem)
	require.NoError(t, err)

	arc := r1.MakeArchive()
	buf := &bytes.Buffer{}
	require.NoError(t, arc.Write(buf))
	arc, err = lib.ReadArchive(buf)
	require.NoError(t, err)

	r2, err := getSimpleArchiveRunner(t, arc)
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ch := newDevNullSampleChannel()
			defer close(ch)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, ch)
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			require.NoError(t, vu.RunOnce())
		})
	}
}

func TestOptionsAreGloballyReadable(t *testing.T) {
	t.Parallel()
	fileSystem := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fileSystem, "/A.js", []byte(`
        export function A() {
        // we can technically get a field set from outside of js this way
            return options.someField;
        }`), fs.ModePerm))
	r1, err := getSimpleRunner(t, "/script.js", `
     import { A } from "./A.js";
     export let options = {
       someField: "here is an option",
     }

        export default function(data) {
            var caught = false;
            try{
                if (A() == "here is an option") {
                  throw "oops"
                }
            } catch(e) {
                if (e.message != "options is not defined") {
                    throw e;
                }
                caught = true;
            }
            if (!caught) {
                throw "expected exception"
            }
        } `, fileSystem, lib.RuntimeOptions{CompatibilityMode: null.StringFrom("extended")})
	require.NoError(t, err)

	arc := r1.MakeArchive()
	r2, err := getSimpleArchiveRunner(t, arc)
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ch := newDevNullSampleChannel()
			defer close(ch)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, ch)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			require.NoError(t, err)
			require.NoError(t, vu.RunOnce())
		})
	}
}

func TestOptionsAreNotGloballyWritable(t *testing.T) {
	t.Parallel()
	fileSystem := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fileSystem, "/A.js", []byte(`
    export function A() {
        // this requires that this is defined
        options.minIterationDuration = "1h"
    }`), fs.ModePerm))
	r1, err := getSimpleRunner(t, "/script.js", `
    import {A} from "/A.js"
    export let options = {minIterationDuration: "5m"}

    export default () =>{}
    var caught = false;
    try{
        A()
    } catch(e) {
        if (e.message != "options is not defined") {
            throw e;
        }
        caught = true;
    }

    if (!caught) {
        throw "expected exception"
    }`, fileSystem, lib.RuntimeOptions{CompatibilityMode: null.StringFrom("extended")})
	require.NoError(t, err)

	// here it exists
	require.EqualValues(t, time.Minute*5, r1.GetOptions().MinIterationDuration.Duration)
	arc := r1.MakeArchive()
	r2, err := getSimpleArchiveRunner(t, arc)
	require.NoError(t, err)

	require.EqualValues(t, time.Minute*5, r2.GetOptions().MinIterationDuration.Duration)
}

func TestDefaultNamedExports(t *testing.T) {
	t.Parallel()
	_, err := getSimpleRunner(t, "/main.js", `export default function main() {}`,
		lib.RuntimeOptions{CompatibilityMode: null.StringFrom("extended")})
	require.NoError(t, err)
}

func TestStarImport(t *testing.T) {
	t.Parallel()

	t.Run("esm_spec", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()
		err := writeToFs(fs, map[string]any{
			"/commonjs_file.js": `exports.something = 5;`,
		})
		require.NoError(t, err)

		r1, err := getSimpleRunner(t, "/script.js", `
			import * as cjs from "./commonjs_file.js"; // commonjs
			import * as k6 from "k6"; // "new" go module
			// TODO: test with basic go module maybe
	
			if (cjs.something != 5) {
				throw "cjs.something has wrong value" + cjs.something;
			}
			if (typeof k6.sleep != "function") {
				throw "k6.sleep has wrong type" + typeof k6.sleep;
			}
			export default () => {}
		`, fs, lib.RuntimeOptions{CompatibilityMode: null.StringFrom("extended")})
		require.NoError(t, err)

		arc := r1.MakeArchive()
		_, err = getSimpleArchiveRunner(t, arc)
		require.NoError(t, err)
	})

	t.Run("default_to_namespaced_object", func(t *testing.T) {
		t.Parallel()

		r1, err := getSimpleRunner(t, "/script.js", `
			import http from "k6/http";
			import * as httpNO from "k6/http";

			const httpKeys = Object.keys(http);
			const httpNOKeys = Object.keys(httpNO);

			// 1. Check if both have the same number of properties
			if (httpKeys.length !== httpNOKeys.length) {
				throw "Objects have a different number of properties.";
			}

			// 2. Check if all properties match
			for (const key of httpKeys) {
				if (!Object.prototype.hasOwnProperty.call(httpNO, key)) {
					throw `+"`Property ${key} is missing in the second object.`"+`;
				}

				if (http[key] !== httpNO[key]) {
					throw `+"`Property ${key} does not match between the objects.`"+`;
				}
			}

			export default () => {}
		`, lib.RuntimeOptions{CompatibilityMode: null.StringFrom("extended")})
		require.NoError(t, err)

		arc := r1.MakeArchive()
		_, err = getSimpleArchiveRunner(t, arc)
		require.NoError(t, err)
	})
}

func TestIndirectExportDefault(t *testing.T) {
	t.Parallel()
	fs := fsext.NewMemMapFs()
	err := writeToFs(fs, map[string]any{
		"/other.js": `export function some() {};`,
	})
	require.NoError(t, err)

	r1, err := getSimpleRunner(t, "/script.js", `
		import { some as default } from "./other.js";
		export { default };
	`, fs)
	require.NoError(t, err)

	arc := r1.MakeArchive()
	_, err = getSimpleArchiveRunner(t, arc)
	require.NoError(t, err)
}
