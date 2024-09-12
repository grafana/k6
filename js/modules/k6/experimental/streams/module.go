// Package streams provides support for the Web Streams API.
package streams

import (
	"errors"
	"io"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

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
		"ReadableStream":              mi.NewReadableStream,
		"CountQueuingStrategy":        mi.NewCountQueuingStrategy,
		"ReadableStreamDefaultReader": mi.NewReadableStreamDefaultReader,
	}}
}

// NewReadableStream is the constructor for the ReadableStream object.
func (mi *ModuleInstance) NewReadableStream(call sobek.ConstructorCall) *sobek.Object {
	return newReadableStream(mi.vu, call)
}

func newReadableStream(vu modules.VU, call sobek.ConstructorCall) *sobek.Object {
	var (
		// 1. If underlyingSource is missing, set it to null.
		underlyingSource *sobek.Object

		rt = vu.Runtime()

		err                  error
		strategy             *sobek.Object
		underlyingSourceDict UnderlyingSource
	)

	// We look for the queuing strategy first, and validate it before
	// the underlying source, in order to pass the Web Platform Tests
	// constructor tests.
	strategy = initializeStrategy(rt, call)

	// 2. Let underlyingSourceDict be underlyingSource, converted to an IDL value of type UnderlyingSource.
	if len(call.Arguments) > 0 && !sobek.IsUndefined(call.Arguments[0]) {
		// We first assert that it is an object (requirement)
		if !isObject(call.Arguments[0]) {
			throw(rt, newTypeError(rt, "underlyingSource must be an object"))
		}

		// Then we try to convert it to an UnderlyingSource
		underlyingSource = call.Arguments[0].ToObject(rt)
		underlyingSourceDict, err = NewUnderlyingSourceFromObject(rt, underlyingSource)
		if err != nil {
			throw(rt, err)
		}
	}

	// 3. Perform ! InitializeReadableStream(this).
	stream := &ReadableStream{
		runtime: rt,
		vu:      vu,
	}
	stream.initialize()

	// 4. If underlyingSourceDict["type"] is "bytes":
	if underlyingSourceDict.Type == "bytes" {
		common.Throw(stream.runtime, newError(NotSupportedError, "'bytes' stream is not supported yet"))
	} else { // 5. Otherwise,
		// 5.1. Assert: underlyingSourceDict["type"] does not exist.
		if underlyingSourceDict.Type != "" {
			common.Throw(rt, newError(AssertionError, "type must not be set for non-byte streams"))
		}

		// 5.2. Let sizeAlgorithm be ! ExtractSizeAlgorithm(strategy).
		sizeAlgorithm := extractSizeAlgorithm(rt, strategy)

		// 5.3. Let highWaterMark be ? ExtractHighWaterMark(strategy, 1).
		highWaterMark := extractHighWaterMark(rt, strategy, 1)

		// 5.4. Perform ? SetUpReadableStreamDefaultControllerFromUnderlyingSource(...).
		stream.setupReadableStreamDefaultControllerFromUnderlyingSource(
			underlyingSource,
			underlyingSourceDict,
			highWaterMark,
			sizeAlgorithm,
		)
	}

	streamObj := rt.ToValue(stream).ToObject(rt)

	proto := call.This.Prototype()
	if proto.Get("locked") == nil {
		err = proto.DefineAccessorProperty("locked", rt.ToValue(func() sobek.Value {
			return rt.ToValue(stream.Locked)
		}), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE)
		if err != nil {
			common.Throw(rt, newError(RuntimeError, err.Error()))
		}
	}

	err = streamObj.SetPrototype(proto)
	if err != nil {
		common.Throw(rt, newError(RuntimeError, err.Error()))
	}

	return streamObj
}
func defaultSizeFunc(_ sobek.Value) (float64, error) { return 1.0, nil }

func initializeStrategy(rt *sobek.Runtime, call sobek.ConstructorCall) *sobek.Object {
	// Either if the strategy is not provided or if it doesn't have a 'highWaterMark',
	// we need to set its default value (highWaterMark=1).
	// https://streams.spec.whatwg.org/#rs-prototype
	strArg := rt.NewObject()
	if len(call.Arguments) > 1 && !common.IsNullish(call.Arguments[1]) {
		strArg = call.Arguments[1].ToObject(rt)
	}
	if common.IsNullish(strArg.Get("highWaterMark")) {
		if err := strArg.Set("highWaterMark", rt.ToValue(1)); err != nil {
			common.Throw(rt, newError(RuntimeError, err.Error()))
		}
	}

	// If the stream type is 'bytes', we don't want the size function.
	// Except, when it is manually specified.
	size := rt.ToValue(defaultSizeFunc)
	if len(call.Arguments) > 0 && !common.IsNullish(call.Arguments[0]) {
		srcArg := call.Arguments[0].ToObject(rt)
		srcTypeArg := srcArg.Get("type")
		if !common.IsNullish(srcTypeArg) && srcTypeArg.String() == ReadableStreamTypeBytes {
			size = nil
		}
	}
	if strArg.Get("size") != nil {
		size = strArg.Get("size")
	}

	strCall := sobek.ConstructorCall{Arguments: []sobek.Value{strArg}}
	return newCountQueuingStrategy(rt, strCall, size)
}

// NewCountQueuingStrategy is the constructor for the [CountQueuingStrategy] object.
//
// [CountQueuingStrategy]: https://streams.spec.whatwg.org/#cqs-class
func (mi *ModuleInstance) NewCountQueuingStrategy(call sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()
	// By default, the CountQueuingStrategy has a pre-defined 'size' property.
	// It cannot be overwritten by the user.
	return newCountQueuingStrategy(rt, call, rt.ToValue(defaultSizeFunc))
}

// newCountQueuingStrategy is the underlying constructor for the [CountQueuingStrategy] object.
//
// It allows to create a CountQueuingStrategy with or without the 'size' property,
// depending on how the containing ReadableStream is initialized.
func newCountQueuingStrategy(
	rt *sobek.Runtime,
	call sobek.ConstructorCall,
	size sobek.Value,
) *sobek.Object {
	obj := rt.NewObject()
	objName := "CountQueuingStrategy"

	if len(call.Arguments) != 1 {
		throw(rt, newTypeError(rt, objName+" takes a single argument"))
	}

	if !isObject(call.Argument(0)) {
		throw(rt, newTypeError(rt, objName+" argument must be an object"))
	}

	argObj := call.Argument(0).ToObject(rt)
	if common.IsNullish(argObj.Get("highWaterMark")) {
		throw(rt, newTypeError(rt, objName+" argument must have 'highWaterMark' property"))
	}

	highWaterMark := argObj.Get("highWaterMark")
	if err := setReadOnlyPropertyOf(obj, objName, "highWaterMark", highWaterMark); err != nil {
		throw(rt, newTypeError(rt, err.Error()))
	}

	if !common.IsNullish(size) {
		if err := setReadOnlyPropertyOf(obj, objName, "size", size); err != nil {
			throw(rt, newTypeError(rt, err.Error()))
		}
	}

	return obj
}

// extractHighWaterMark returns the high watermark for the given queuing strategy.
//
// It implements the [ExtractHighWaterMark] algorithm.
//
// [ExtractHighWaterMark]: https://streams.spec.whatwg.org/#validate-and-normalize-high-water-mark
func extractHighWaterMark(rt *sobek.Runtime, strategy *sobek.Object, defaultHWM float64) float64 {
	// 1. If strategy["highWaterMark"] does not exist, return defaultHWM.
	if common.IsNullish(strategy.Get("highWaterMark")) {
		return defaultHWM
	}

	// 2. Let highWaterMark be strategy["highWaterMark"].
	highWaterMark := strategy.Get("highWaterMark")

	// 3. If highWaterMark is NaN or highWaterMark < 0, throw a RangeError exception.
	if sobek.IsNaN(strategy.Get("highWaterMark")) ||
		!isNumber(strategy.Get("highWaterMark")) ||
		!isNonNegativeNumber(strategy.Get("highWaterMark")) {
		throw(rt, newRangeError(rt, "highWaterMark must be a non-negative number"))
	}

	// 4. Return highWaterMark.
	return highWaterMark.ToFloat()
}

// extractSizeAlgorithm returns the size algorithm for the given queuing strategy.
//
// It implements the [ExtractSizeAlgorithm] algorithm.
//
// [ExtractSizeAlgorithm]: https://streams.spec.whatwg.org/#make-size-algorithm-from-size-function
func extractSizeAlgorithm(rt *sobek.Runtime, strategy *sobek.Object) SizeAlgorithm {
	var sizeFunc sobek.Callable
	sizeProp := strategy.Get("size")

	if common.IsNullish(sizeProp) {
		sizeFunc, _ = sobek.AssertFunction(rt.ToValue(func(_ sobek.Value) (float64, error) { return 1.0, nil }))
		return sizeFunc
	}

	sizeFunc, isFunc := sobek.AssertFunction(sizeProp)
	if !isFunc {
		throw(rt, newTypeError(rt, "size must be a function"))
	}

	return sizeFunc
}

// NewReadableStreamDefaultReader is the constructor for the [ReadableStreamDefaultReader] object.
//
// [ReadableStreamDefaultReader]: https://streams.spec.whatwg.org/#readablestreamdefaultreader
func (mi *ModuleInstance) NewReadableStreamDefaultReader(call sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()

	if len(call.Arguments) != 1 {
		throw(rt, newTypeError(rt, "ReadableStreamDefaultReader takes a single argument"))
	}

	stream, ok := call.Argument(0).Export().(*ReadableStream)
	if !ok {
		throw(rt, newTypeError(rt, "ReadableStreamDefaultReader argument must be a ReadableStream"))
	}

	// 1. Perform ? SetUpReadableStreamDefaultReader(this, stream).
	reader := &ReadableStreamDefaultReader{}
	reader.setup(stream)

	object, err := NewReadableStreamDefaultReaderObject(reader)
	if err != nil {
		throw(rt, err)
	}

	return object
}

// NewReadableStreamFromReader is the equivalent of [NewReadableStreamDefaultReader] but to initialize
// a new [ReadableStream] from a given [io.Reader] in Go code.
// It is useful for those situations when a [io.Reader] needs to be surfaced up to the JS runtime.
func NewReadableStreamFromReader(vu modules.VU, reader io.Reader) *sobek.Object {
	rt := vu.Runtime()
	return newReadableStream(vu, sobek.ConstructorCall{
		Arguments: []sobek.Value{rt.ToValue(underlyingSourceFromReader(vu, reader))},
		This:      rt.NewObject(),
	})
}

func underlyingSourceFromReader(vu modules.VU, reader io.Reader) *sobek.Object {
	rt := vu.Runtime()

	underlyingSource := vu.Runtime().NewObject()
	if err := underlyingSource.Set("pull", rt.ToValue(func(controller *sobek.Object) *sobek.Promise {
		// Prepare methods
		cClose, _ := sobek.AssertFunction(controller.Get("close"))
		cEnqueue, _ := sobek.AssertFunction(controller.Get("enqueue"))

		buf := make([]byte, 1024)
		n, err := reader.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			panic(err)
		}

		_, enqueueErr := cEnqueue(nil, rt.ToValue(string(buf[:n])))
		if enqueueErr != nil {
			panic(enqueueErr)
		}

		if err == io.EOF {
			_, closeErr := cClose(nil)
			if closeErr != nil {
				panic(closeErr)
			}
		}

		return newResolvedPromise(vu, sobek.Undefined())
	})); err != nil {
		throw(rt, err)
	}

	return underlyingSource
}
