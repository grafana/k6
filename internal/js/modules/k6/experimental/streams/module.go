// Package streams provides support for the Web Streams API.
package streams

import (
	"errors"
	"io"
	"math"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
)

type (
	// RootModule is the module that will be registered with the runtime.
	RootModule struct{}

	// ModuleInstance is the module instance that will be created for each VU.
	ModuleInstance struct {
		vu modules.VU

		exports modules.Exports

		readableStreamPrototype              *sobek.Object
		readableStreamDefaultReaderPrototype *sobek.Object
		writableStreamPrototype              *sobek.Object
		writableStreamDefaultWriterPrototype *sobek.Object
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
	mi := &ModuleInstance{vu: vu}
	rt := vu.Runtime()

	// Convert the stream constructors once per VU. Besides keeping the exported
	// constructor identities stable, this gives the implementation canonical prototypes that
	// cannot be replaced by a caller-controlled `this` value.
	readableStreamConstructor, err := newWebIDLConstructor(rt, "ReadableStream", mi.NewReadableStream)
	if err != nil {
		throw(rt, err)
	}
	readableStreamDefaultReaderConstructor, err := newWebIDLConstructor(
		rt,
		"ReadableStreamDefaultReader",
		mi.NewReadableStreamDefaultReader,
	)
	if err != nil {
		throw(rt, err)
	}
	writableStreamConstructor, err := newWebIDLConstructor(rt, "WritableStream", mi.NewWritableStream)
	if err != nil {
		throw(rt, err)
	}
	writableStreamDefaultWriterConstructor, err := newWebIDLConstructor(
		rt,
		"WritableStreamDefaultWriter",
		mi.NewWritableStreamDefaultWriter,
	)
	if err != nil {
		throw(rt, err)
	}
	transformStreamConstructor, err := newWebIDLConstructor(
		rt,
		"TransformStream",
		mi.NewTransformStream,
	)
	if err != nil {
		throw(rt, err)
	}

	mi.readableStreamPrototype = constructorPrototype(rt, readableStreamConstructor)
	mi.readableStreamDefaultReaderPrototype = constructorPrototype(
		rt,
		readableStreamDefaultReaderConstructor,
	)
	mi.writableStreamPrototype = constructorPrototype(rt, writableStreamConstructor)
	mi.writableStreamDefaultWriterPrototype = constructorPrototype(
		rt,
		writableStreamDefaultWriterConstructor,
	)
	transformStreamPrototype := constructorPrototype(rt, transformStreamConstructor)
	if err := installReadableStreamPrototype(rt, mi.readableStreamPrototype); err != nil {
		throw(rt, err)
	}
	if err := installReadableStreamDefaultReaderPrototype(
		rt,
		mi.readableStreamDefaultReaderPrototype,
	); err != nil {
		throw(rt, err)
	}
	if err := installWritableStreamPrototype(rt, mi.writableStreamPrototype); err != nil {
		throw(rt, err)
	}
	if err := installWritableStreamDefaultWriterPrototype(
		rt,
		mi.writableStreamDefaultWriterPrototype,
	); err != nil {
		throw(rt, err)
	}
	if err := installTransformStreamPrototype(rt, transformStreamPrototype); err != nil {
		throw(rt, err)
	}
	mi.exports = modules.Exports{Named: map[string]any{
		"ReadableStream":              readableStreamConstructor,
		"CountQueuingStrategy":        mi.NewCountQueuingStrategy,
		"ReadableStreamDefaultReader": readableStreamDefaultReaderConstructor,
		"WritableStream":              writableStreamConstructor,
		"WritableStreamDefaultWriter": writableStreamDefaultWriterConstructor,
		"TransformStream":             transformStreamConstructor,
	}}

	return mi
}

func constructorPrototype(rt *sobek.Runtime, constructor sobek.Value) *sobek.Object {
	return constructor.ToObject(rt).Get("prototype").ToObject(rt)
}

func requireNewTarget(rt *sobek.Runtime, call sobek.ConstructorCall, name string) {
	// Web IDL interface constructors must be invoked with `new`. Rejecting every ordinary
	// function call also prevents a caller-controlled `this` value from being used as an
	// instance prototype.
	if call.NewTarget == nil {
		throw(rt, newTypeError(rt, name+" constructor must be called with new"))
	}
}

// Exports returns the module exports, that will be available in the runtime.
func (mi *ModuleInstance) Exports() modules.Exports {
	return mi.exports
}

// NewReadableStream is the constructor for the ReadableStream object.
func (mi *ModuleInstance) NewReadableStream(call sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()
	requireNewTarget(rt, call, "ReadableStream")

	return newReadableStream(mi.vu, call, mi.readableStreamDefaultReaderPrototype)
}

func newReadableStream(
	vu modules.VU,
	call sobek.ConstructorCall,
	readerPrototype *sobek.Object,
) *sobek.Object {
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
		runtime:         rt,
		vu:              vu,
		readerPrototype: readerPrototype,
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

	return stream.toObject(call.This.Prototype())
}

// NewWritableStream is the constructor for the WritableStream object.
func (mi *ModuleInstance) NewWritableStream(call sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()
	requireNewTarget(rt, call, "WritableStream")

	return newWritableStream(mi.vu, call, mi.writableStreamDefaultWriterPrototype)
}

func newWritableStream(
	vu modules.VU,
	call sobek.ConstructorCall,
	writerPrototype *sobek.Object,
) *sobek.Object {
	rt := vu.Runtime()

	var (
		// 1. If underlyingSink is missing, set it to null.
		underlyingSink *sobek.Object

		err                error
		strategy           *sobek.Object
		underlyingSinkDict UnderlyingSink
	)

	// We look for the queuing strategy first, and validate it before the underlying sink,
	// in order to match the specification's argument conversion order and pass the Web
	// Platform Tests constructor tests.
	strategy = initializeStrategy(rt, call)

	// 2. Let underlyingSinkDict be underlyingSink, converted to an IDL value of type UnderlyingSink.
	if len(call.Arguments) > 0 && !sobek.IsUndefined(call.Arguments[0]) {
		// We first assert that it is an object (requirement).
		if !isObject(call.Arguments[0]) {
			throw(rt, newTypeError(rt, "underlyingSink must be an object"))
		}

		// Then we try to convert it to an UnderlyingSink.
		underlyingSink = call.Arguments[0].ToObject(rt)
		underlyingSinkDict, err = NewUnderlyingSinkFromObject(rt, underlyingSink)
		if err != nil {
			throw(rt, err)
		}
	}

	// 3. If underlyingSinkDict["type"] exists, throw a RangeError exception.
	if underlyingSinkDict.Type != nil && !sobek.IsUndefined(underlyingSinkDict.Type) {
		throw(rt, newRangeError(rt, "'type' is not supported by WritableStream"))
	}

	// 4. Perform ! InitializeWritableStream(this).
	stream := &WritableStream{
		runtime:         rt,
		vu:              vu,
		writerPrototype: writerPrototype,
	}
	stream.initialize()

	// 5. Let sizeAlgorithm be ! ExtractSizeAlgorithm(strategy).
	sizeAlgorithm := extractSizeAlgorithm(rt, strategy)

	// 6. Let highWaterMark be ? ExtractHighWaterMark(strategy, 1).
	highWaterMark := extractHighWaterMark(rt, strategy, 1)

	// 7. Perform ? SetUpWritableStreamDefaultControllerFromUnderlyingSink(...).
	stream.setupWritableStreamDefaultControllerFromUnderlyingSink(
		underlyingSink,
		underlyingSinkDict,
		highWaterMark,
		sizeAlgorithm,
	)

	return stream.toObject(call.This.Prototype())
}

// NewTransformStream is the constructor for the TransformStream object.
func (mi *ModuleInstance) NewTransformStream(call sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()
	requireNewTarget(rt, call, "TransformStream")

	return newTransformStream(
		mi.vu,
		call,
		mi.readableStreamPrototype,
		mi.readableStreamDefaultReaderPrototype,
		mi.writableStreamPrototype,
		mi.writableStreamDefaultWriterPrototype,
	)
}

func newTransformStream(
	vu modules.VU,
	call sobek.ConstructorCall,
	readableStreamPrototype *sobek.Object,
	readableStreamDefaultReaderPrototype *sobek.Object,
	writableStreamPrototype *sobek.Object,
	writableStreamDefaultWriterPrototype *sobek.Object,
) *sobek.Object {
	rt := vu.Runtime()

	var (
		transformerObj  *sobek.Object
		transformerDict Transformer
		err             error
	)

	// 1. If transformer is missing, set it to null.
	// 2. Let transformerDict be transformer, converted to an IDL value of type Transformer.
	if len(call.Arguments) > 0 && !common.IsNullish(call.Arguments[0]) {
		// We first assert that it is an object (requirement).
		if !isObject(call.Arguments[0]) {
			throw(rt, newTypeError(rt, "transformer must be an object"))
		}

		// Then we try to convert it to a Transformer.
		transformerObj = call.Arguments[0].ToObject(rt)
		transformerDict, err = NewTransformerFromObject(rt, transformerObj)
		if err != nil {
			throw(rt, err)
		}
	}

	// 3. If transformerDict["readableType"] exists, throw a RangeError exception.
	if transformerDict.ReadableType != nil && !sobek.IsUndefined(transformerDict.ReadableType) {
		throw(rt, newRangeError(rt, "'readableType' is not supported"))
	}

	// 4. If transformerDict["writableType"] exists, throw a RangeError exception.
	if transformerDict.WritableType != nil && !sobek.IsUndefined(transformerDict.WritableType) {
		throw(rt, newRangeError(rt, "'writableType' is not supported"))
	}

	// 5. Let readableHighWaterMark be ? ExtractHighWaterMark(readableStrategy, 0).
	readableStrategy := transformStrategyObject(rt, call, 2)
	readableHighWaterMark := extractHighWaterMark(rt, readableStrategy, 0)

	// 6. Let readableSizeAlgorithm be ! ExtractSizeAlgorithm(readableStrategy).
	readableSizeAlgorithm := extractSizeAlgorithm(rt, readableStrategy)

	// 7. Let writableHighWaterMark be ? ExtractHighWaterMark(writableStrategy, 1).
	writableStrategy := transformStrategyObject(rt, call, 1)
	writableHighWaterMark := extractHighWaterMark(rt, writableStrategy, 1)

	// 8. Let writableSizeAlgorithm be ! ExtractSizeAlgorithm(writableStrategy).
	writableSizeAlgorithm := extractSizeAlgorithm(rt, writableStrategy)

	// 9. Let startPromise be a new promise.
	startPromise := newPromiseWrapper(rt)

	// 10. Perform ! InitializeTransformStream(this, startPromise, ...).
	stream := &TransformStream{runtime: rt, vu: vu}
	stream.initialize(
		startPromise,
		writableHighWaterMark,
		writableSizeAlgorithm,
		readableHighWaterMark,
		readableSizeAlgorithm,
	)
	stream.readable.readerPrototype = readableStreamDefaultReaderPrototype
	stream.writable.writerPrototype = writableStreamDefaultWriterPrototype

	// 11. Perform ? SetUpTransformStreamDefaultControllerFromTransformer(this, transformer, transformerDict).
	stream.setupDefaultControllerFromTransformer(transformerObj, transformerDict)

	// Build the JavaScript objects for the readable and writable sides using the canonical
	// prototypes cached by the module instance.
	stream.readableObj = stream.readable.toObject(readableStreamPrototype)
	stream.writableObj = stream.writable.toObject(writableStreamPrototype)

	// 12. If transformerDict["start"] exists, then resolve startPromise with the result of
	// invoking transformerDict["start"] with argument list « this.[[controller]] » and callback
	// this value transformer.
	if transformerDict.Start != nil && !sobek.IsUndefined(transformerDict.Start) {
		startFn, ok := sobek.AssertFunction(transformerDict.Start)
		if !ok {
			throw(rt, newTypeError(rt, "transformer.start must be a function"))
		}

		res, startErr := startFn(transformerObj, stream.controller.object)
		if startErr != nil {
			// Any thrown exceptions are re-thrown by the TransformStream constructor.
			throw(rt, startErr)
		}
		stream.resolveStartPromiseAsync(startPromise, res)
	} else {
		// 13. Otherwise, resolve startPromise with undefined.
		stream.resolveStartPromiseAsync(startPromise, sobek.Undefined())
	}

	return stream.toObject(call.This.Prototype())
}

// transformStrategyObject returns the queuing strategy argument at the given index as an object,
// or a new empty object if it is missing or nullish. It never writes to the strategy, so that
// the TransformStream constructor does not invoke user-defined setters (as verified by the Web
// Platform Tests).
func transformStrategyObject(rt *sobek.Runtime, call sobek.ConstructorCall, index int) *sobek.Object {
	if len(call.Arguments) > index && !common.IsNullish(call.Arguments[index]) {
		return call.Arguments[index].ToObject(rt)
	}
	return rt.NewObject()
}

// NewWritableStreamDefaultWriter is the constructor for the [WritableStreamDefaultWriter] object.
//
// [WritableStreamDefaultWriter]: https://streams.spec.whatwg.org/#writablestreamdefaultwriter
func (mi *ModuleInstance) NewWritableStreamDefaultWriter(call sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()
	requireNewTarget(rt, call, "WritableStreamDefaultWriter")

	if len(call.Arguments) != 1 {
		throw(rt, newTypeError(rt, "WritableStreamDefaultWriter takes a single argument"))
	}

	stream := writableStreamFromValue(rt, call.Argument(0))
	if stream == nil {
		throw(rt, newTypeError(rt, "WritableStreamDefaultWriter argument must be a WritableStream"))
	}

	// 1. Perform ? SetUpWritableStreamDefaultWriter(this, stream).
	writer := &WritableStreamDefaultWriter{}
	writer.setup(stream)

	object, err := NewWritableStreamDefaultWriterObject(writer, call.This.Prototype())
	if err != nil {
		throw(rt, err)
	}

	return object
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
	hwmValue := strategy.Get("highWaterMark")

	// 1. If strategy["highWaterMark"] does not exist, return defaultHWM.
	if common.IsNullish(hwmValue) {
		return defaultHWM
	}

	// 2. Let highWaterMark be strategy["highWaterMark"], converted to a number. The WebIDL type
	// of QueuingStrategy's highWaterMark member is an unrestricted double, so values such as an
	// object with a toString()/valueOf() are coerced here rather than rejected outright.
	highWaterMark := hwmValue.ToFloat()

	// 3. If highWaterMark is NaN or highWaterMark < 0, throw a RangeError exception.
	if math.IsNaN(highWaterMark) || highWaterMark < 0 {
		throw(rt, newRangeError(rt, "highWaterMark must be a non-negative number"))
	}

	// 4. Return highWaterMark.
	return highWaterMark
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
	requireNewTarget(rt, call, "ReadableStreamDefaultReader")

	if len(call.Arguments) != 1 {
		throw(rt, newTypeError(rt, "ReadableStreamDefaultReader takes a single argument"))
	}

	stream := readableStreamFromValue(rt, call.Argument(0))
	if stream == nil {
		throw(rt, newTypeError(rt, "ReadableStreamDefaultReader argument must be a ReadableStream"))
	}

	// 1. Perform ? SetUpReadableStreamDefaultReader(this, stream).
	reader := &ReadableStreamDefaultReader{}
	reader.setup(stream)

	object, err := NewReadableStreamDefaultReaderObject(reader, call.This.Prototype())
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
	proto := rt.NewObject()
	readerProto := rt.NewObject()
	if err := installReadableStreamPrototype(rt, proto); err != nil {
		throw(rt, err)
	}
	if err := installReadableStreamDefaultReaderPrototype(rt, readerProto); err != nil {
		throw(rt, err)
	}
	return newReadableStream(vu, sobek.ConstructorCall{
		Arguments: []sobek.Value{rt.ToValue(underlyingSourceFromReader(vu, reader))},
		This:      objectWithPrototype(rt, proto),
	}, readerProto)
}

func objectWithPrototype(rt *sobek.Runtime, proto *sobek.Object) *sobek.Object {
	obj := rt.NewObject()
	if err := obj.SetPrototype(proto); err != nil {
		throw(rt, err)
	}
	return obj
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
