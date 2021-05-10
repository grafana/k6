/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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

package data

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"github.com/stretchr/testify/require"
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

func newConfiguredRuntime(moduleInstance interface{}) (*goja.Runtime, error) {
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	ctx := common.WithRuntime(context.Background(), rt)
	err := rt.Set("data", common.Bind(rt, moduleInstance, &ctx))
	if err != nil {
		return rt, err //nolint:wrapcheck
	}
	_, err = rt.RunString("var SharedArray = data.SharedArray;")

	return rt, err //nolint:wrapcheck
}

func TestSharedArrayConstructorExceptions(t *testing.T) {
	t.Parallel()
	rt, err := newConfiguredRuntime(New())
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

	moduleInstance := New()
	rt, err := newConfiguredRuntime(moduleInstance)
	require.NoError(t, err)
	_, err = rt.RunString(makeArrayScript)
	require.NoError(t, err)

	rt, err = newConfiguredRuntime(moduleInstance)
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

	moduleInstance := New()
	rt, err := newConfiguredRuntime(moduleInstance)
	require.NoError(t, err)
	_, err = rt.RunString(makeArrayScript)
	require.NoError(t, err)

	// create another Runtime with new ctx but keep the initEnv
	rt, err = newConfiguredRuntime(moduleInstance)
	require.NoError(t, err)
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
		moduleInstance := New()
		for j := 0; j < instances; j++ {
			rt, err := newConfiguredRuntime(moduleInstance)
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
