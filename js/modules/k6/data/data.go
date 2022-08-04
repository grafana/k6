package data

import (
	"errors"
	"strconv"
	"sync"

	"github.com/dop251/goja"
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

// sharedArray is a constructor returning a shareable read-only array
// indentified by the name and having their contents be whatever the call returns
func (d *Data) sharedArray(call goja.ConstructorCall) *goja.Object {
	rt := d.vu.Runtime()

	if d.vu.State() != nil {
		common.Throw(rt, errors.New("new SharedArray must be called in the init context"))
	}

	name := call.Argument(0).String()
	if name == "" {
		common.Throw(rt, errors.New("empty name provided to SharedArray's constructor"))
	}

	fn, ok := goja.AssertFunction(call.Argument(1))
	if !ok {
		common.Throw(rt, errors.New("a function is expected as the second argument of SharedArray's constructor"))
	}

	array := d.shared.get(rt, name, fn)
	return array.wrap(rt).ToObject(rt)
}

func (s *sharedArrays) get(rt *goja.Runtime, name string, call goja.Callable) sharedArray {
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

func getShareArrayFromCall(rt *goja.Runtime, call goja.Callable) sharedArray {
	gojaValue, err := call(goja.Undefined())
	if err != nil {
		common.Throw(rt, err)
	}
	obj := gojaValue.ToObject(rt)
	if obj.ClassName() != "Array" {
		common.Throw(rt, errors.New("only arrays can be made into SharedArray")) // TODO better error
	}
	arr := make([]string, obj.Get("length").ToInteger())

	stringify, _ := goja.AssertFunction(rt.GlobalObject().Get("JSON").ToObject(rt).Get("stringify"))
	var val goja.Value
	for i := range arr {
		val, err = stringify(goja.Undefined(), obj.Get(strconv.Itoa(i)))
		if err != nil {
			panic(err)
		}
		arr[i] = val.String()
	}

	return sharedArray{arr: arr}
}
