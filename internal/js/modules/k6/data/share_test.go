package data

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/compiler"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modulestest"
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

	err := runtime.SetupModuleSystem(map[string]interface{}{"k6/data": New()}, nil, compiler.New(runtime.VU.InitEnv().Logger))
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
		name, testCase := name, testCase
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
		name, testCase := name, testCase
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

func TestSharedArrayRaceInInitialization(t *testing.T) {
	t.Parallel()

	const instances = 10
	const repeats = 100
	for i := 0; i < repeats; i++ {
		runtimes := make([]*sobek.Runtime, instances)
		for j := 0; j < instances; j++ {
			runtime, err := newConfiguredRuntime(t)
			require.NoError(t, err)
			runtimes[j] = runtime.VU.Runtime()
		}
		var wg sync.WaitGroup
		for _, rt := range runtimes {
			rt := rt
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := rt.RunString(`var array = new data.SharedArray("shared", function() {return [1,2,3,4,5,6,7,8,9, 10]});`)
				require.NoError(t, err)
			}()
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
