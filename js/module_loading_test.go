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
				throw new Error(JSON.stringify(resp));
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
	// This test that functions such as http.get which require context still work if they are called
	// inside script that is imported
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
