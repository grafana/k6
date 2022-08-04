package data

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"

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

func newConfiguredRuntime() (*goja.Runtime, error) {
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	m, ok := New().NewModuleInstance(
		&modulestest.VU{
			RuntimeField: rt,
			InitEnvField: &common.InitEnvironment{},
			CtxField:     context.Background(),
			StateField:   nil,
		},
	).(*Data)
	if !ok {
		return rt, fmt.Errorf("not a Data module instance")
	}

	err := rt.Set("data", m.Exports().Named)
	if err != nil {
		return rt, err //nolint:wrapcheck
	}
	_, err = rt.RunString("var SharedArray = data.SharedArray;")
	return rt, err //nolint:wrapcheck
}

func TestSharedArrayConstructorExceptions(t *testing.T) {
	t.Parallel()
	rt, err := newConfiguredRuntime()
	require.NoError(t, err)
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
	}

	for name, testCase := range cases {
		name, testCase := name, testCase
		t.Run(name, func(t *testing.T) {
			_, err := rt.RunString(testCase.code)
			if testCase.err == "" {
				require.NoError(t, err)
				return // the t.Run
			}

			require.Error(t, err)
			exc := err.(*goja.Exception)
			require.Contains(t, exc.Error(), testCase.err)
		})
	}
}

func TestSharedArrayAnotherRuntimeExceptions(t *testing.T) {
	t.Parallel()

	rt, err := newConfiguredRuntime()
	require.NoError(t, err)
	_, err = rt.RunString(makeArrayScript)
	require.NoError(t, err)

	rt, err = newConfiguredRuntime()
	require.NoError(t, err)
	_, err = rt.RunString(makeArrayScript)
	require.NoError(t, err)

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
			_, err := rt.RunString(testCase.code)
			if testCase.err == "" {
				require.NoError(t, err)
				return // the t.Run
			}

			require.Error(t, err)
			exc := err.(*goja.Exception)
			require.Contains(t, exc.Error(), testCase.err)
		})
	}
}

func TestSharedArrayAnotherRuntimeWorking(t *testing.T) {
	t.Parallel()

	rt := goja.New()
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
	rt = goja.New()
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
		runtimes := make([]*goja.Runtime, instances)
		for j := 0; j < instances; j++ {
			rt, err := newConfiguredRuntime()
			require.NoError(t, err)
			runtimes[j] = rt
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
