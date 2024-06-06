// Package data implements `k6/data` js module for k6.
// This modules provide utility types to work with data in an efficient way.
package data

import (
	"errors"
	"strconv"
	"sync"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct {
		shared sharedArrays
	}

	// Data represents an instance of the data module.
	Data struct {
		vu     modules.VU
		shared *sharedArrays
	}

	sharedArrays struct {
		data map[string]sharedArray
		mu   sync.RWMutex
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &Data{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{
		shared: sharedArrays{
			data: make(map[string]sharedArray),
		},
	}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (rm *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &Data{
		vu:     vu,
		shared: &rm.shared,
	}
}

// Exports returns the exports of the data module.
func (d *Data) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"SharedArray": d.sharedArray,
		},
	}
}

const asyncFunctionNotSupportedMsg = "SharedArray constructor does not support async functions as second argument"

// sharedArray is a constructor returning a shareable read-only array
// indentified by the name and having their contents be whatever the call returns
func (d *Data) sharedArray(call sobek.ConstructorCall) *sobek.Object {
	rt := d.vu.Runtime()

	if d.vu.State() != nil {
		common.Throw(rt, errors.New("new SharedArray must be called in the init context"))
	}

	name := call.Argument(0).String()
	if name == "" {
		common.Throw(rt, errors.New("empty name provided to SharedArray's constructor"))
	}
	val := call.Argument(1)

	if common.IsAsyncFunction(rt, val) {
		common.Throw(rt, errors.New(asyncFunctionNotSupportedMsg))
	}

	fn, ok := sobek.AssertFunction(val)
	if !ok {
		common.Throw(rt, errors.New("a function is expected as the second argument of SharedArray's constructor"))
	}

	array := d.shared.get(rt, name, fn)
	return array.wrap(rt).ToObject(rt)
}

func (s *sharedArrays) get(rt *sobek.Runtime, name string, call sobek.Callable) sharedArray {
	s.mu.RLock()
	array, ok := s.data[name]
	s.mu.RUnlock()
	if !ok {
		s.mu.Lock()
		defer s.mu.Unlock()
		array, ok = s.data[name]
		if !ok {
			array = getShareArrayFromCall(rt, call)
			s.data[name] = array
		}
	}

	return array
}

func getShareArrayFromCall(rt *sobek.Runtime, call sobek.Callable) sharedArray {
	gojaValue, err := call(sobek.Undefined())
	if err != nil {
		common.Throw(rt, err)
	}
	obj := gojaValue.ToObject(rt)
	if obj.ClassName() != "Array" {
		common.Throw(rt, errors.New("only arrays can be made into SharedArray")) // TODO better error
	}
	arr := make([]string, obj.Get("length").ToInteger())

	stringify, _ := sobek.AssertFunction(rt.GlobalObject().Get("JSON").ToObject(rt).Get("stringify"))
	var val sobek.Value
	for i := range arr {
		val, err = stringify(sobek.Undefined(), obj.Get(strconv.Itoa(i)))
		if err != nil {
			panic(err)
		}
		arr[i] = val.String()
	}

	return sharedArray{arr: arr}
}
