/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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
	"os"
	"testing"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/stats"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func newDevNullSampleChannel() chan stats.SampleContainer {
	var ch = make(chan stats.SampleContainer, 100)
	go func() {
		for range ch {
		}
	}()
	return ch
}

func TestLoadOnceGlobalVars(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/C.js", []byte(`
		var globalVar;
		if (!globalVar) {
			globalVar = Math.random();
		}
		export function C() {
			return globalVar;
		}
	`), os.ModePerm))

	require.NoError(t, afero.WriteFile(fs, "/A.js", []byte(`
		import { C } from "./C.js";
		export function A() {
			return C();
		}
	`), os.ModePerm))
	require.NoError(t, afero.WriteFile(fs, "/B.js", []byte(`
		import { C } from "./C.js";
		export function B() {
			return C();
		}
	`), os.ModePerm))
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
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
		`),
	}, fs, lib.RuntimeOptions{})
	require.NoError(t, err)

	arc := r1.MakeArchive()
	arc.Files = make(map[string][]byte)
	r2, err := NewFromArchive(arc, lib.RuntimeOptions{})
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			ch := newDevNullSampleChannel()
			defer close(ch)
			vu, err := r.NewVU(ch)
			require.NoError(t, err)
			err = vu.RunOnce(context.Background())
			require.NoError(t, err)
		})
	}
}

func TestLoadDoesntBreakHTTPGet(t *testing.T) {
	// This test that functions such as http.get which require context still work if they are called
	// inside script that is imported

	tb := testutils.NewHTTPMultiBin(t)
	defer tb.Cleanup()
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/A.js", []byte(tb.Replacer.Replace(`
		import http from "k6/http";
		export function A() {
			return http.get("HTTPBIN_URL/get");
		}
	`)), os.ModePerm))
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
			import { A } from "./A.js";

			export default function(data) {
				let resp = A();
				if (resp.status != 200) {
					throw new Error("wrong status "+ resp.status);
				}
			}
		`),
	}, fs, lib.RuntimeOptions{})
	require.NoError(t, err)

	require.NoError(t, r1.SetOptions(lib.Options{Hosts: tb.Dialer.Hosts}))
	arc := r1.MakeArchive()
	arc.Files = make(map[string][]byte)
	r2, err := NewFromArchive(arc, lib.RuntimeOptions{})
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			ch := newDevNullSampleChannel()
			defer close(ch)
			vu, err := r.NewVU(ch)
			require.NoError(t, err)
			err = vu.RunOnce(context.Background())
			require.NoError(t, err)
		})
	}
}

func TestLoadGlobalVarsAreNotSharedBetweenVUs(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/A.js", []byte(`
		var globalVar = 0;
		export function A() {
			globalVar += 1
			return globalVar;
		}
	`), os.ModePerm))
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
			import { A } from "./A.js";

			export default function(data) {
				var a = A();
				if (a == 1) {
					a = 2;
				} else {
					throw new Error("wrong value of a " + a);
				}
			}
		`),
	}, fs, lib.RuntimeOptions{})
	require.NoError(t, err)

	arc := r1.MakeArchive()
	arc.Files = make(map[string][]byte)
	r2, err := NewFromArchive(arc, lib.RuntimeOptions{})
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			ch := newDevNullSampleChannel()
			defer close(ch)
			vu, err := r.NewVU(ch)
			require.NoError(t, err)
			err = vu.RunOnce(context.Background())
			require.NoError(t, err)

			// run a second VU
			vu, err = r.NewVU(ch)
			require.NoError(t, err)
			err = vu.RunOnce(context.Background())
			require.NoError(t, err)
		})
	}
}

func TestLoadCycle(t *testing.T) {
	// This is mostly the example from https://hacks.mozilla.org/2018/03/es-modules-a-cartoon-deep-dive/
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/counter.js", []byte(`
			let message = require("./main.js").message;
			exports.count = 5;
			export function a() {
				return message;
			}
	`), os.ModePerm))

	require.NoError(t, afero.WriteFile(fs, "/main.js", []byte(`
			let counter = require("./counter.js");
			let count = counter.count;
			let a = counter.a;
			let message= "Eval complete";
			exports.message = message;

			export default function() {
				if (count != 5) {
					throw new Error("Wrong value of count "+ count);
				}
				let aMessage = a();
				if (aMessage != message) {
					throw new Error("Wrong value of a() "+ aMessage);
				}
			}
	`), os.ModePerm))
	data, err := afero.ReadFile(fs, "/main.js")
	require.NoError(t, err)
	r1, err := New(&lib.SourceData{
		Filename: "/main.js",
		Data:     data,
	}, fs, lib.RuntimeOptions{})
	require.NoError(t, err)

	arc := r1.MakeArchive()
	arc.Files = make(map[string][]byte)
	r2, err := NewFromArchive(arc, lib.RuntimeOptions{})
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			ch := newDevNullSampleChannel()
			defer close(ch)
			vu, err := r.NewVU(ch)
			require.NoError(t, err)
			err = vu.RunOnce(context.Background())
			require.NoError(t, err)
		})
	}

}

func TestLoadCycleBinding(t *testing.T) {
	// This is mostly the example from
	// http://2ality.com/2015/07/es6-module-exports.html#why-export-bindings
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/a.js", []byte(`
		import {bar} from './b.js';
		export function foo(a) {
				if (a !== undefined) {
					return "foo" + a;
				}
				return "foo" + bar(3);
		}
	`), os.ModePerm))

	require.NoError(t, afero.WriteFile(fs, "/b.js", []byte(`
		import {foo} from './a.js';
		export function bar(a) {
				if (a !== undefined) {
					return "bar" + a;
				}
				return "bar" + foo(5);
			}
	`), os.ModePerm))

	r1, err := New(&lib.SourceData{
		Filename: "/main.js",
		Data: []byte(`
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
		`),
	}, fs, lib.RuntimeOptions{})
	require.NoError(t, err)

	arc := r1.MakeArchive()
	arc.Files = make(map[string][]byte)
	r2, err := NewFromArchive(arc, lib.RuntimeOptions{})
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			ch := newDevNullSampleChannel()
			defer close(ch)
			vu, err := r.NewVU(ch)
			require.NoError(t, err)
			err = vu.RunOnce(context.Background())
			require.NoError(t, err)
		})
	}
}
