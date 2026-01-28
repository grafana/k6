// Package data implements `k6/data` js module for k6.
// This modules provide utility types to work with data in an efficient way.
package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	builder := func() sharedArray { return getSharedArrayFromCall(rt, fn) }
	array := d.shared.loadOrStore(name, builder)
	return array.wrap(rt).ToObject(rt)
}

// RecordReader is the interface that wraps the action of reading records from a resource.
//
// The data module RecordReader interface is implemented by types that can read data that can be
// treated as records, from data sources such as a CSV file, etc.
type RecordReader interface {
	Read() (any, error)
}

// NewSharedArrayFrom creates a new shared array from the provided [RecordReader].
//
// This function is not exposed to the JS runtime. It is used internally to instantiate
// shared arrays without having to go through the whole JS runtime machinery, which effectively has
// a big performance impact (e.g. when filling a shared array from a CSV file).
//
// The rt argument is an explicit runtime parameter to retain control over which VU runtime the
// function is executed in. This is important because the shared array underlying implementation
// relies on maintaining a single instance of arrays for the whole test setup and VUs.
//
// The name argument uniquely identifies the shared array and must not be empty.
//
// The r argument is a [RecordReader] from which records are read and stored in the shared array.
func (d *Data) NewSharedArrayFrom(rt *sobek.Runtime, name string, r RecordReader) *sobek.Object {
	if name == "" {
		common.Throw(rt, errors.New("empty name provided to SharedArray's constructor"))
	}

	// The builder function is passed to loadOrStore, which ensures it is only
	// executed once for a given name. This guarantees the underlying data is
	// initialized exactly once, even if multiple VUs call this function concurrently
	// with the same name.
	builder := func() sharedArray {
		var arr []string
		for {
			record, err := r.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				common.Throw(rt, fmt.Errorf("failed to read record; reason: %w", err))
			}

			marshaled, err := json.Marshal(record)
			if err != nil {
				common.Throw(rt, fmt.Errorf("failed to marshal record; reason: %w", err))
			}

			arr = append(arr, string(marshaled))
		}

		return sharedArray{arr: arr}
	}

	array := d.shared.loadOrStore(name, builder)
	return array.wrap(rt).ToObject(rt)
}

// loadOrStore returns the shared array associated with the given name. If no array
// exists for that name, it invokes the builder function to create one and stores it.
//
// This method is thread-safe. The builder function is guaranteed to be called at most
// once for each unique name, even if multiple goroutines call loadOrStore concurrently
// with the same name.
func (s *sharedArrays) loadOrStore(name string, builder func() sharedArray) sharedArray {
	// Try read-only lookup first
	s.mu.RLock()
	arr, ok := s.data[name]
	s.mu.RUnlock()
	if ok {
		return arr
	}

	// Not found, acquire write lock
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if arr, ok = s.data[name]; ok {
		return arr
	}

	arr = builder()
	s.data[name] = arr
	return arr
}

func getSharedArrayFromCall(rt *sobek.Runtime, call sobek.Callable) sharedArray {
	sobekValue, err := call(sobek.Undefined())
	if err != nil {
		common.Throw(rt, err)
	}
	obj := sobekValue.ToObject(rt)
	if obj.ClassName() != "Array" {
		common.Throw(rt, errors.New("only arrays can be made into SharedArray")) // TODO better error
	}
	arr := make([]string, obj.Get("length").ToInteger())

	stringifyFunc, _ := sobek.AssertFunction(rt.GlobalObject().Get("JSON").ToObject(rt).Get("stringify"))
	var val sobek.Value
	for i := range arr {
		val, err = stringifyFunc(sobek.Undefined(), obj.Get(strconv.Itoa(i)))
		if err != nil {
			panic(err)
		}
		arr[i] = val.String()
	}

	return sharedArray{arr: arr}
}
