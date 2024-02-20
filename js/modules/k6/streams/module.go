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

	var underlyingSource *UnderlyingSource
	var strategy *QueuingStrategy

	// We look for the queuing strategy first, and validate it before
	// the underlying source, in order to pass the Web Platform Tests
	// constructor tests.
	if len(call.Arguments) > 1 && !common.IsNullish(call.Arguments[1]) {
		strategy, err = NewQueuingStrategyFrom(runtime, call.Arguments[1].ToObject(runtime))
		if err != nil {
			common.Throw(runtime, err)
		}
	} else {
		strategy = NewCountQueuingStrategy(1)
	}

	if len(call.Arguments) > 0 && !common.IsNullish(call.Arguments[0]) {
		// 2.
		underlyingSource, err = NewUnderlyingSourceFromObject(runtime, call.Arguments[0].ToObject(runtime))
		if err != nil {
			common.Throw(runtime, err)
		}
	} else {
		// 1.
		underlyingSource = nil
	}

	// 3.
	stream := &ReadableStream{
		runtime: mi.vu.Runtime(),
		vu:      mi.vu,
	}
	stream.initialize()

	if underlyingSource != nil && underlyingSource.Type == "bytes" { // 4.
		// 4.1
		if strategy.Size != nil {
			common.Throw(runtime, newError(RangeError, "size function must not be set for byte streams"))
		}

		// 4.2
		highWaterMark := strategy.extractHighWaterMark(0)

		// 4.3
		stream.setupReadableByteStreamControllerFromUnderlyingSource(*underlyingSource, highWaterMark)
	} else { // 5.
		// 5.1
		if underlyingSource != nil && underlyingSource.Type != "" {
			common.Throw(runtime, newError(AssertionError, "type must not be set for non-byte streams"))
		}

		// 5.2
		sizeAlgorithm := strategy.extractSizeAlgorithm()

		// 5.3
		highWaterMark := strategy.extractHighWaterMark(1)

		// 5.4
		stream.setupDefaultControllerFromUnderlyingSource(*underlyingSource, highWaterMark, sizeAlgorithm)
	}

	return runtime.ToValue(stream).ToObject(runtime)
}