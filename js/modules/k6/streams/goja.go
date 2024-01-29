package streams

import (
	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/js/promises"
)

// newUint8Array instantiate a new Uint8Array object
func newUint8Array(rt *goja.Runtime, buffer []byte, byteOffset int64, length int64) (goja.Value, error) {
	constructor := rt.Get("Uint8Array")

	uint8Array, err := rt.New(constructor, rt.ToValue(buffer), rt.ToValue(byteOffset), rt.ToValue(length))
	if err != nil {
		return nil, err
	}

	return uint8Array, nil
}

// newResolvedPromise instantiates a new resolved promise.
func newResolvedPromise(vu modules.VU, with goja.Value) *goja.Promise {
	promise, resolve, _ := promises.New(vu)

	go func() {
		resolve(with)
	}()

	return promise
}

// newRejectedPromise instantiates a new rejected promise.
func newRejectedPromise(vu modules.VU, with any) *goja.Promise {
	promise, _, reject := promises.New(vu)

	go func() {
		reject(with)
	}()

	return promise
}

// promiseThen facilitates instantiating a new promise and defining callbacks for to be executed
// on fulfillment as well as rejection, directly from Go.
func promiseThen(
	rt *goja.Runtime,
	promise *goja.Promise,
	onFulfilled, onRejected func(goja.Value),
) (*goja.Promise, error) {
	val, err := rt.RunString(
		`(function(promise, onFulfilled, onRejected) { return promise.then(onFulfilled, onRejected) })`)
	if err != nil {
		return nil, newError(RuntimeError, "unable to initialize promiseThen internal helper function")
	}

	cal, ok := goja.AssertFunction(val)
	if !ok {
		return nil, newError(RuntimeError, "the internal promiseThen helper is not a function")
	}

	if onRejected == nil {
		val, err = cal(goja.Undefined(), rt.ToValue(promise), rt.ToValue(onFulfilled))
	} else {
		val, err = cal(goja.Undefined(), rt.ToValue(promise), rt.ToValue(onFulfilled), rt.ToValue(onRejected))
	}

	if err != nil {
		return nil, err
	}

	newPromise, ok := val.Export().(*goja.Promise)
	if !ok {
		return nil, newError(RuntimeError, "unable to cast the internal promiseThen helper's return value to a promise")
	}

	return newPromise, nil
}

// canTransferArrayBuffer implements the [CanTransferArrayBuffer] algorithm.
//
// [CanTransferArrayBuffer]: https://streams.spec.whatwg.org/#can-transfer-array-buffer
// FIXME: verify and complete the implementation (if necessary)
func canTransferArrayBuffer(_ *goja.Runtime, ab goja.ArrayBuffer) bool {
	// 1. Assert: Type(o) is Object.
	// No need to perform that step

	// 2. Assert: O is has an [[ArrayBufferData]] internal slot.
	// No need to perform that step

	// 3.
	if ab.Detached() {
		return false
	}

	// 4. If SameValue(O.[[ArrayBufferDetachKey]], undefined) is false, return false.
	// No need to perform this step?

	// 5.
	return true
}

// transferArrayBuffer implements the [TransferArrayBuffer] algorithm.
//
// [TransferArrayBuffer]: https://streams.spec.whatwg.org/#transfer-array-buffer
func transferArrayBuffer(rt *goja.Runtime, ab goja.ArrayBuffer) goja.ArrayBuffer {
	// 1. Assert: ! IsDetachedBuffer(O) is false.
	if ab.Detached() {
		common.Throw(rt, newError(AssertionError, "object is a detached ArrayBuffer"))
	}

	// 2. Let arrayBufferData be O.[[ArrayBufferData]].
	// 3. Let arrayBufferByteLength be O.[[ArrayBufferByteLength]].
	// In Goja, there's no direct way to access [[ArrayBufferData]] and [[ArrayBufferByteLength]],
	// so we need to create a new ArrayBuffer and copy the data.
	dataCopy := make([]byte, len(ab.Bytes()))
	copy(dataCopy, ab.Bytes())

	// 4. Perform ? DetachArrayBuffer(O).
	ab.Detach()

	// 5. Check ArrayBufferDetachKey (this part might need to be handled in JavaScript).
	// Since Goja does not expose ArrayBufferDetachKey, this check might be skipped or
	// handled differently?

	// 6. Return a new ArrayBuffer object with the copied data.
	return rt.NewArrayBuffer(dataCopy)
}

// CloneArrayBuffer implements the [CloneArrayBuffer] ECMAScript abstract operation.
//
// [CloneArrayBuffer]: https://tc39.es/ecma262/#sec-clonearraybuffer
// TODO: give this a double look
func CloneArrayBuffer(rt *goja.Runtime, srcBuffer goja.ArrayBuffer, srcByteOffset int64, srcLength int64) (goja.ArrayBuffer, error) {
	if srcBuffer.Detached() {
		return goja.ArrayBuffer{}, newError(AssertionError, "object is a detached ArrayBuffer")
	}

	targetBuffer := make([]byte, srcLength)
	srcData := srcBuffer.Bytes()
	if srcByteOffset+srcLength > int64(len(srcData)) {
		return goja.ArrayBuffer{}, newError(RangeError, "Source byte offset and length exceed buffer size")
	}

	copy(targetBuffer, srcData[srcByteOffset:srcByteOffset+srcLength])

	return rt.NewArrayBuffer(targetBuffer), nil
}

func asViewedArrayBuffer(rt *goja.Runtime, value goja.Value) (*ViewedArrayBuffer, error) {
	if common.IsNullish(value) {
		return nil, newError(TypeError, "value is nullish")
	}

	asObject := value.ToObject(rt)

	var ab goja.ArrayBuffer
	var byteOffset, byteLength int64
	var ok bool

	if isTypedArray(rt, asObject) {
		ab, ok = asObject.Get("buffer").Export().(goja.ArrayBuffer)
		if !ok {
			return nil, newError(TypeError, "value is not an ArrayBuffer")
		}

		byteOffset = asObject.Get("byteOffset").ToInteger()
		byteLength = asObject.Get("byteLength").ToInteger()
	} else {
		ab, ok = asObject.Export().(goja.ArrayBuffer)
		if !ok {
			return nil, newError(TypeError, "value is not an ArrayBuffer")
		}

		byteLength = int64(len(ab.Bytes()))
	}

	return &ViewedArrayBuffer{
		ArrayBuffer: ab,
		ByteOffset:  byteOffset,
		ByteLength:  byteLength,
	}, nil
}

type ViewedArrayBuffer struct {
	ArrayBuffer            goja.ArrayBuffer
	ByteOffset, ByteLength int64
}

// isTypedArray returns true if the given value is an instance of a Typed Array
func isTypedArray(rt *goja.Runtime, v goja.Value) bool {
	asObject := v.ToObject(rt)

	typedArrayTypes := []JSType{
		Int8ArrayConstructor,
		Uint8ArrayConstructor,
		Uint8ClampedArrayConstructor,
		Int16ArrayConstructor,
		Uint16ArrayConstructor,
		Int32ArrayConstructor,
		Uint32ArrayConstructor,
		Float32ArrayConstructor,
		Float64ArrayConstructor,
		BigInt64ArrayConstructor,
		BigUint64ArrayConstructor,
	}

	return isInstanceOf(rt, asObject, typedArrayTypes...)
}

// JSType is a string representing a JavaScript type
type JSType string

const (
	// ArrayBufferConstructor is the name of the ArrayBufferConstructor constructor
	ArrayBufferConstructor JSType = "ArrayBuffer"

	// DataViewConstructor is the name of the DataView constructor
	DataViewConstructor = "DataView"

	// Int8ArrayConstructor is the name of the Int8ArrayConstructor constructor
	Int8ArrayConstructor = "Int8Array"

	// Uint8ArrayConstructor is the name of the Uint8ArrayConstructor constructor
	Uint8ArrayConstructor = "Uint8Array"

	// Uint8ClampedArrayConstructor is the name of the Uint8ClampedArrayConstructor constructor
	Uint8ClampedArrayConstructor = "Uint8ClampedArray"

	// Int16ArrayConstructor is the name of the Int16ArrayConstructor constructor
	Int16ArrayConstructor = "Int16Array"

	// Uint16ArrayConstructor is the name of the Uint16ArrayConstructor constructor
	Uint16ArrayConstructor = "Uint16Array"

	// Int32ArrayConstructor is the name of the Int32ArrayConstructor constructor
	Int32ArrayConstructor = "Int32Array"

	// Uint32ArrayConstructor is the name of the Uint32ArrayConstructor constructor
	Uint32ArrayConstructor = "Uint32Array"

	// Float32ArrayConstructor is the name of the Float32ArrayConstructor constructor
	Float32ArrayConstructor = "Float32Array"

	// Float64ArrayConstructor is the name of the Float64ArrayConstructor constructor
	Float64ArrayConstructor = "Float64Array"

	// BigInt64ArrayConstructor is the name of the BigInt64ArrayConstructor constructor
	BigInt64ArrayConstructor = "BigInt64Array"

	// BigUint64ArrayConstructor is the name of the BigUint64ArrayConstructor constructor
	BigUint64ArrayConstructor = "BigUint64Array"
)

// isInstanceOf returns true if the given value is an instance of the given constructor
// This uses the technique described in https://github.com/dop251/goja/issues/379#issuecomment-1164441879
func isInstanceOf(rt *goja.Runtime, v goja.Value, instanceOf ...JSType) bool {
	var valid bool

	for _, t := range instanceOf {
		instanceOfConstructor := rt.Get(string(t))
		if valid = v.ToObject(rt).Get("constructor").SameAs(instanceOfConstructor); valid {
			break
		}
	}

	return valid
}

// isNonNegativeNumber implements the [IsNonNegativeNumber] algorithm.
//
// [IsNonNegativeNumber]: https://streams.spec.whatwg.org/#is-non-negative-number
func isNonNegativeNumber(rt *goja.Runtime, value goja.Value) bool {
	if common.IsNullish(value) {
		return false
	}

	var i int64
	if err := rt.ExportTo(value, &i); err != nil {
		return false
	}

	if i < 0 {
		return false
	}

	return true
}
