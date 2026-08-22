package webcrypto

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/common"
)

// exportArrayBuffer interprets the given value as an ArrayBuffer, TypedArray or DataView
// and returns a copy of the underlying byte slice.
func exportArrayBuffer(rt *sobek.Runtime, v sobek.Value, arrayBufferIsView sobek.Callable) ([]byte, error) {
	if common.IsNullish(v) {
		return nil, NewError(TypeError, "data is null or undefined")
	}

	if v.ExportType() != reflect.TypeFor[sobek.ArrayBuffer]() {
		isView, err := isArrayBufferView(v, arrayBufferIsView)
		if err != nil {
			return nil, NewError(OperationError, err.Error())
		}
		if !isView {
			return nil, NewError(OperationError, "data is neither an ArrayBuffer, nor a TypedArray nor DataView")
		}
	}

	var bytes []byte
	if err := rt.ExportTo(v, &bytes); err != nil {
		return nil, NewError(OperationError, err.Error())
	}

	// Copy the underlying byte slice to avoid the caller modifying it.
	// Ensures this step complies with the expactations of the
	// specification: "Let [...] be the result of getting a copy of the
	// bytes held by the [...] parameter"
	bytesCopy := make([]byte, len(bytes))
	copy(bytesCopy, bytes)

	return bytesCopy, nil
}

func getArrayBufferIsView(rt *sobek.Runtime) (sobek.Callable, error) {
	var value sobek.Value
	if exception := rt.Try(func() {
		value = rt.Get(string(ArrayBufferConstructor)).ToObject(rt).Get("isView")
	}); exception != nil {
		return nil, exception
	}

	isView, ok := sobek.AssertFunction(value)
	if !ok {
		return nil, fmt.Errorf("ArrayBuffer.isView is not a function")
	}
	return isView, nil
}

func isArrayBufferView(v sobek.Value, isView sobek.Callable) (bool, error) {
	if isView == nil {
		return false, fmt.Errorf("ArrayBuffer.isView was not captured")
	}

	result, err := isView(nil, v)
	if err != nil {
		return false, err
	}
	return result.ToBoolean(), nil
}

// traverseObject traverses the given object using the given fields and returns the value
// at the end of the traversal. It assumes that all the traversed fields are Objects.
func traverseObject(rt *sobek.Runtime, src sobek.Value, fields ...string) (sobek.Value, error) {
	if common.IsNullish(src) {
		return nil, NewError(TypeError, "Object is null or undefined")
	}

	obj := src.ToObject(rt)
	if common.IsNullish(obj) {
		return nil, NewError(TypeError, "Object is null or undefined")
	}

	for idx, field := range fields {
		src = obj.Get(field)
		if common.IsNullish(src) {
			return nil, NewError(
				TypeError,
				fmt.Sprintf("field %s is null or undefined", strings.Join(fields[:idx+1], ".")),
			)
		}

		obj = src.ToObject(rt)
		if common.IsNullish(obj) {
			return nil, NewError(
				TypeError,
				fmt.Sprintf("field %s is not an Object", strings.Join(fields[:idx+1], ".")),
			)
		}
	}

	return src, nil
}

// IsInstanceOf returns true if the given value is an instance of the given constructor
// This uses the technique described in https://github.com/dop251/goja/issues/379#issuecomment-1164441879
func IsInstanceOf(rt *sobek.Runtime, v sobek.Value, instanceOf ...JSType) bool {
	var valid bool

	for _, t := range instanceOf {
		instanceOfConstructor := rt.Get(string(t))
		if valid = v.ToObject(rt).Get("constructor").SameAs(instanceOfConstructor); valid {
			break
		}
	}

	return valid
}

// IsTypedArray returns true if the given value is an instance of a Typed Array
func IsTypedArray(rt *sobek.Runtime, v sobek.Value) bool {
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

	return IsInstanceOf(rt, asObject, typedArrayTypes...)
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
