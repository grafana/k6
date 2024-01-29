// Package streams provides support for the Web Streams API.
package streams

import (
	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

// FIXME: figure out what "return a promise rejects/fulfilled with" means in practice for us:
//   - Should we execute the promise's resolve/reject functions in a go func() { ... } still?
//   - So far I've made a mix of both, but I'm not sure what's the best approach, if any

// TODO: have an `Assert` helper function that throws an assertion error if the condition is not met
// TODO: Document we do not support the following:
// - static `from` constructor as it's expected to take an `asyncIterable` as input we do not support

type (
	// RootModule is the module that will be registered with the runtime.
	RootModule struct{}

	// ModuleInstance is the module instance that will be created for each VU.
	ModuleInstance struct {
		vu modules.VU
	}
)

// Ensure the interfaces are implemented correctly
var (
	_ modules.Instance = &ModuleInstance{}
	_ modules.Module   = &RootModule{}
)

// New creates a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance creates a new instance of the module for a specific VU.
func (rm *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &ModuleInstance{
		vu: vu,
	}
}

// Exports returns the module exports, that will be available in the runtime.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{Named: map[string]interface{}{
		"ReadableStream": mi.NewReadableStream,
	}}
}

// NewReadableStream is the constructor for the ReadableStream object.
func (mi *ModuleInstance) NewReadableStream(call goja.ConstructorCall) *goja.Object {
	runtime := mi.vu.Runtime()
	var err error

	// 1.
	var underlyingSource UnderlyingSource
	if len(call.Arguments) > 0 {
		firstArgObj := call.Arguments[0].ToObject(runtime)

		// 2.
		underlyingSource, err = NewUnderlyingSourceFromObject(runtime, firstArgObj)
		if err != nil {
			common.Throw(runtime, newError(TypeError, "invalid UnderlyingSource object passed to ReadableStream constructor"))
		}
	}
	// FIXME: should we have an else, in case the argument is not provided here?
	// Because what should happen if underlyingSource is null as described in the spec? this
	// is rather unclear at the moment...

	var strategy QueuingStrategy
	if len(call.Arguments) > 1 {
		var err error
		strategy, err = NewQueuingStrategyFrom(runtime, call.Arguments[1].ToObject(runtime))
		if err != nil {
			common.Throw(runtime, err)
		}
	} else {
		strategy = NewCountQueuingStrategy(1)
	}

	// 3.
	stream := &ReadableStream{
		runtime: mi.vu.Runtime(),
		vu:      mi.vu,
	}
	stream.initialize()

	// 4.
	if underlyingSource.Type == ReadableStreamTypeBytes {
		if strategy.Size != nil {
			common.Throw(runtime, newError(RangeError, "size function must not be set for byte streams"))
		}

		highWaterMark := strategy.extractHighWaterMark(0)

		stream.setupReadableByteStreamControllerFromUnderlyingSource(underlyingSource, highWaterMark)
	} else { // 5.
		if underlyingSource.Type != "" {
			common.Throw(runtime, newError(AssertionError, "type must not be set for non-byte streams"))
		}

		sizeAlgorithm := strategy.extractSizeAlgorithm()
		highWaterMark := strategy.extractHighWaterMark(1)
		stream.setupDefaultControllerFromUnderlyingSource(underlyingSource, highWaterMark, sizeAlgorithm)
	}

	return runtime.ToValue(stream).ToObject(runtime)
}
