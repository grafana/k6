package data

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/compiler"
	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modulestest"
)

const makeArrayScript = `
var array = new data.SharedArray("shared",function() {
    var n = 50;
    var arr = new Array(n);
    for (var i = 0 ; i <n; i++) {
        arr[i] = {value: "something" +i};
    }
	return arr;
});
`

const initGlobals = `
	globalThis.data = require("k6/data");
	globalThis.SharedArray = data.SharedArray;
`

func newConfiguredRuntime(t testing.TB) (*modulestest.Runtime, error) {
	runtime := modulestest.NewRuntime(t)

	err := runtime.SetupModuleSystem(map[string]any{"k6/data": New()}, nil, compiler.New(runtime.VU.InitEnv().Logger))
	if err != nil {
		return nil, err
	}
	_, err = runtime.VU.Runtime().RunString(initGlobals)
	return runtime, err
}

func configuredRuntimeFromAnother(t testing.TB, another *modulestest.Runtime) (*modulestest.Runtime, error) {
	runtime := modulestest.NewRuntime(t)
	err := runtime.SetupModuleSystemFromAnother(another)
	if err != nil {
		return nil, err
	}

	_, err = runtime.VU.Runtime().RunString(initGlobals)
	return runtime, err
}

func TestSharedArrayConstructorExceptions(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		code, err string
	}{
		"returning string": {
			code: `new SharedArray("wat", function() {return "whatever"});`,
			err:  "only arrays can be made into SharedArray",
		},
		"empty name": {
			code: `new SharedArray("", function() {return []});`,
			err:  "empty name provided to SharedArray's constructor",
		},
		"function in the data": {
			code: `
			var s = new SharedArray("wat2", function() {return [{s: function() {}}]});
			if (s[0].s !== undefined) {
				throw "s[0].s should be undefined"
			}
		`,
			err: "",
		},
		"not a function": {
			code: `var s = new SharedArray("wat3", "astring");`,
			err:  "a function is expected",
		},
		"async function": {
			code: `var s = new SharedArray("wat3", async function() {});`,
			err:  "SharedArray constructor does not support async functions as second argument",
		},
		"async lambda": {
			code: `var s = new SharedArray("wat3", async () => {});`,
			err:  "SharedArray constructor does not support async functions as second argument",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runtime, err := newConfiguredRuntime(t)
			require.NoError(t, err)
			_, err = runtime.VU.Runtime().RunString(testCase.code)
			if testCase.err == "" {
				require.NoError(t, err)
				return // the t.Run
			}

			require.Error(t, err)
			exc := new(sobek.Exception)
			require.True(t, errors.As(err, &exc))
			require.Contains(t, exc.Error(), testCase.err)
		})
	}
}

func TestSharedArrayAnotherRuntimeExceptions(t *testing.T) {
	t.Parallel()

	// use strict is required as otherwise just nothing happens
	cases := map[string]struct {
		code, err string
	}{
		"setting in for-of": {
			code: `'use strict'; for (var v of array) { v.data = "bad"; }`,
			err:  "Cannot add property data, object is not extensible",
		},
		"setting from index": {
			code: `'use strict'; array[2].data2 = "bad2"`,
			err:  "Cannot add property data2, object is not extensible",
		},
		"setting property on the shared array": {
			code: `'use strict'; array.something = "something"`,
			err:  `Cannot set property "something" on a dynamic array`,
		},
		"setting index on the shared array": {
			code: `'use strict'; array[2] = "something"`,
			err:  "SharedArray is immutable",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			testRuntime, err := newConfiguredRuntime(t)
			rt := testRuntime.VU.Runtime()
			require.NoError(t, err)
			_, err = rt.RunString(makeArrayScript)
			require.NoError(t, err)

			testRuntime, err = configuredRuntimeFromAnother(t, testRuntime)
			rt = testRuntime.VU.Runtime()
			require.NoError(t, err)
			_, err = rt.RunString(makeArrayScript)
			require.NoError(t, err)
			_, err = rt.RunString(testCase.code)
			if testCase.err == "" {
				require.NoError(t, err)
				return // the t.Run
			}

			require.Error(t, err)
			exc := new(sobek.Exception)
			require.True(t, errors.As(err, &exc))
			require.Contains(t, exc.Error(), testCase.err)
		})
	}
}

func TestSharedArrayAnotherRuntimeWorking(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	vu := &modulestest.VU{
		RuntimeField: rt,
		InitEnvField: &common.InitEnvironment{},
		CtxField:     context.Background(),
		StateField:   nil,
	}
	m, ok := New().NewModuleInstance(vu).(*Data)
	require.True(t, ok)
	require.NoError(t, rt.Set("data", m.Exports().Named))

	_, err := rt.RunString(makeArrayScript)
	require.NoError(t, err)

	// create another Runtime with new ctx but keep the initEnv
	rt = sobek.New()
	vu.RuntimeField = rt
	vu.CtxField = context.Background()
	require.NoError(t, rt.Set("data", m.Exports().Named))

	_, err = rt.RunString(`var array = new data.SharedArray("shared", function() {throw "wat";});`)
	require.NoError(t, err)

	_, err = rt.RunString(`
	if (array[2].value !== "something2") {
		throw new Error("bad array[2]="+array[2].value);
	}
	if (array.length != 50) {
		throw new Error("bad length " +array.length);
	}

	var i = 0;
	for (var v of array) {
		if (v.value !== "something"+i) {
			throw new Error("bad v.value="+v.value+" for i="+i);
		}
		i++;
	}

	i = 0;
	array.forEach(function(v){
		if (v.value !== "something"+i) {
			throw new Error("bad v.value="+v.value+" for i="+i);
		}
		i++;
	});


	`)
	require.NoError(t, err)
}

// TestSharedArrayReadValuesIntact verifies that values read back through the
// (now primitive-skipping) deepFreeze path are intact: primitives of every JSON
// kind, an ASCII and a non-ASCII string (different internal representations),
// and a large string (the shape that used to blow up memory). Repeated reads
// must be consistent.
func TestSharedArrayReadValuesIntact(t *testing.T) {
	t.Parallel()
	runtime, err := newConfiguredRuntime(t)
	require.NoError(t, err)
	rt := runtime.VU.Runtime()

	_, err = rt.RunString(`'use strict';
		var LEN = 200000;
		var s = new SharedArray("reads", function() {
			return ["hello", "", "ünïçödé 🚀", 42, 3.5, -7, true, false, null, "A".repeat(LEN)];
		});
		if (s.length !== 10) { throw new Error("length="+s.length); }
		if (s[0] !== "hello" || typeof s[0] !== "string") { throw new Error("s[0]="+s[0]); }
		if (s[1] !== "" || typeof s[1] !== "string") { throw new Error("s[1]="+s[1]); }
		if (s[2] !== "ünïçödé 🚀") { throw new Error("s[2]="+s[2]); }
		if (s[3] !== 42) { throw new Error("s[3]="+s[3]); }
		if (s[4] !== 3.5) { throw new Error("s[4]="+s[4]); }
		if (s[5] !== -7) { throw new Error("s[5]="+s[5]); }
		if (s[6] !== true) { throw new Error("s[6]="+s[6]); }
		if (s[7] !== false) { throw new Error("s[7]="+s[7]); }
		if (s[8] !== null) { throw new Error("s[8]="+s[8]); }
		if (s[9].length !== LEN || s[9][0] !== "A" || s[9][LEN-1] !== "A") { throw new Error("bad large string"); }
	`)
	require.NoError(t, err)
}

// TestSharedArrayDeepFreezeSemantics verifies the safety property that matters:
// object and array elements (including nested ones) remain deeply frozen after
// the primitive fast-path change, so a VU cannot mutate the shared data.
func TestSharedArrayDeepFreezeSemantics(t *testing.T) {
	t.Parallel()

	const setup = `'use strict';
		var s = new SharedArray("rich", function() {
			return [{
				str: "hello",
				num: 1,
				flag: true,
				nothing: null,
				nested: { deep: "x", arr: [1, 2, 3] },
				list: [ {k: "a"}, {k: "b"} ]
			}, [10, 20, 30]];
		});
	`

	// integrity asserts the shared structure is untouched. It throws (failing
	// the surrounding RunString) if any value changed or a property was added.
	const integrity = `'use strict';
		if (s[0].str !== "hello") { throw new Error("str changed: "+s[0].str); }
		if (s[0].num !== 1) { throw new Error("num changed: "+s[0].num); }
		if ("extra" in s[0]) { throw new Error("property was added to object"); }
		if (s[0].nested.deep !== "x") { throw new Error("nested.deep changed"); }
		if ("extra" in s[0].nested) { throw new Error("property added to nested"); }
		if (s[0].nested.arr.length !== 3 || s[0].nested.arr[0] !== 1) { throw new Error("nested.arr changed"); }
		if (s[0].list.length !== 2 || s[0].list[0].k !== "a") { throw new Error("list changed"); }
		if (s[1].length !== 3 || s[1][0] !== 10) { throw new Error("top-level array changed"); }
	`

	// The deep freeze must reject every one of these mutations. Top-level plain
	// object freezing is already covered by TestSharedArrayAnotherRuntimeExceptions;
	// these focus on the nested and array cases, which exercise the recursion and
	// the values that pass through the *sobek.Object guard as containers.
	cases := map[string]string{
		"mutate nested object":                 `s[0].nested.deep = "bad";`,
		"add property to nested object":        `s[0].nested.extra = "bad";`,
		"delete nested property":               `delete s[0].nested.deep;`,
		"push to nested array":                 `s[0].nested.arr.push(4);`,
		"set nested array index":               `s[0].nested.arr[0] = 99;`,
		"mutate object inside array":           `s[0].list[0].k = "bad";`,
		"push to top-level array element":      `s[1].push(40);`,
		"set index on top-level array element": `s[1][0] = 99;`,
	}

	for name, code := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runtime, err := newConfiguredRuntime(t)
			require.NoError(t, err)
			rt := runtime.VU.Runtime()
			_, err = rt.RunString(setup)
			require.NoError(t, err)

			// Strict mode is required: in sloppy mode, writes to frozen
			// objects fail silently instead of throwing.
			_, err = rt.RunString("'use strict';\n" + code)
			require.Error(t, err, "mutation should have been rejected by the freeze")
			exc := new(sobek.Exception)
			require.True(t, errors.As(err, &exc))
			require.Contains(t, exc.Error(), "TypeError",
				"a frozen write must fail with a TypeError")

			// The shared data must be completely unchanged.
			_, err = rt.RunString(integrity)
			require.NoError(t, err)
		})
	}
}

// TestSharedArrayLargeStringGetAllocations is a regression guard for the memory
// blow-up: reading a large string element must not allocate an amount of memory
// proportional to the string length.
//
//nolint:paralleltest // testing.AllocsPerRun must not run concurrently.
func TestSharedArrayLargeStringGetAllocations(t *testing.T) {
	runtime, err := newConfiguredRuntime(t)
	require.NoError(t, err)
	rt := runtime.VU.Runtime()

	_, err = rt.RunString(`
		var shared = new SharedArray("alloc", function() {
			return ["short", "A".repeat(100000)];
		});
		var get = function(i) { return shared[i]; };
	`)
	require.NoError(t, err)

	get, ok := sobek.AssertFunction(rt.Get("get"))
	require.True(t, ok)

	small := testing.AllocsPerRun(50, func() {
		_, err := get(sobek.Undefined(), rt.ToValue(0))
		if err != nil {
			panic(err)
		}
	})
	large := testing.AllocsPerRun(50, func() {
		_, err := get(sobek.Undefined(), rt.ToValue(1))
		if err != nil {
			panic(err)
		}
	})

	t.Logf("allocations per Get: small string=%.0f, 100k string=%.0f", small, large)

	// With the bug, `large` was on the order of the string length (100k+).
	// After the fix it is a small constant, close to `small`. Use a generous
	// bound that is decisively below the buggy behavior.
	require.Less(t, large, 2000.0,
		"reading a large string element allocates too much; deepFreeze may be enumerating characters again")
}

func TestSharedArrayRaceInInitialization(t *testing.T) {
	t.Parallel()

	const instances = 10
	const repeats = 100
	for range repeats {
		runtimes := make([]*sobek.Runtime, instances)
		for j := range instances {
			runtime, err := newConfiguredRuntime(t)
			require.NoError(t, err)
			runtimes[j] = runtime.VU.Runtime()
		}
		var wg sync.WaitGroup
		for _, rt := range runtimes {
			wg.Go(func() {
				_, err := rt.RunString(`var array = new data.SharedArray("shared", function() {return [1,2,3,4,5,6,7,8,9, 10]});`)
				require.NoError(t, err)
			})
		}
		ch := make(chan struct{})
		go func() {
			wg.Wait()
			close(ch)
		}()

		select {
		case <-ch:
			// everything is fine
		case <-time.After(time.Second * 10):
			t.Fatal("Took too long probably locked up")
		}
	}
}
