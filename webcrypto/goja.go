package webcrypto

import "github.com/dop251/goja"

// IsInstanceOf returns true if the given value is an instance of the given constructor
// This uses the technique described in https://github.com/dop251/goja/issues/379#issuecomment-1164441879
func IsInstanceOf(rt *goja.Runtime, v goja.Value, instanceOf JSType) bool {
	instanceOfConstructor := rt.Get(string(instanceOf))
	return v.ToObject(rt).Get("constructor").SameAs(instanceOfConstructor)
}

// IsTypedArray returns true if the given value is an instance of a Typed Array
func IsTypedArray(rt *goja.Runtime, v goja.Value) bool {
	asObject := v.ToObject(rt)

	return IsInstanceOf(rt, asObject, Int8ArrayConstructor) ||
		IsInstanceOf(rt, asObject, Uint8ArrayConstructor) ||
		IsInstanceOf(rt, asObject, Uint8ClampedArrayConstructor) ||
		IsInstanceOf(rt, asObject, Int16ArrayConstructor) ||
		IsInstanceOf(rt, asObject, Uint16ArrayConstructor) ||
		IsInstanceOf(rt, asObject, Int32ArrayConstructor) ||
		IsInstanceOf(rt, asObject, Uint32ArrayConstructor) ||
		IsInstanceOf(rt, asObject, Float32ArrayConstructor) ||
		IsInstanceOf(rt, asObject, Float64ArrayConstructor) ||
		IsInstanceOf(rt, asObject, BigInt64ArrayConstructor) ||
		IsInstanceOf(rt, asObject, BigUint64ArrayConstructor)
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
