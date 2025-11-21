package encoding

import (
	"errors"
	"fmt"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
)

// setReadOnlyPropertyOf sets a read-only property on the given [sobek.Object].
func setReadOnlyPropertyOf(obj *sobek.Object, name string, value sobek.Value) error {
	err := obj.DefineDataProperty(name,
		value,
		sobek.FLAG_FALSE,
		sobek.FLAG_FALSE,
		sobek.FLAG_TRUE,
	)
	if err != nil {
		return fmt.Errorf("unable to define %s read-only property on TextEncoder object; reason: %w", name, err)
	}

	return nil
}

// exportArrayBuffer interprets the given value as an ArrayBuffer, TypedArray or DataView
// and returns a copy of the underlying byte slice.
func exportArrayBuffer(rt *sobek.Runtime, v sobek.Value) ([]byte, error) {
	if common.IsNullish(v) {
		return []byte{}, nil
	}

	// Handle undefined specifically (Web API spec requires undefined to be treated as empty buffer)
	if v == nil || sobek.IsUndefined(v) {
		return []byte{}, nil
	}

	asObject := v.ToObject(rt)

	var ab sobek.ArrayBuffer
	var ok bool

	if IsTypedArray(rt, v) { //nolint:nestif
		ab, ok = asObject.Get("buffer").Export().(sobek.ArrayBuffer)
		if !ok {
			return nil, errors.New("TypedArray.buffer is not an ArrayBuffer")
		}
	} else if IsInstanceOf(rt, v, DataViewConstructor) {
		// Handle DataView objects
		ab, ok = asObject.Get("buffer").Export().(sobek.ArrayBuffer)
		if !ok {
			return nil, errors.New("DataView.buffer is not an ArrayBuffer")
		}

		// Get the byte offset and length from the DataView
		byteOffset := asObject.Get("byteOffset").ToInteger()
		byteLength := asObject.Get("byteLength").ToInteger()

		// Extract the relevant portion of the ArrayBuffer
		allBytes := ab.Bytes()
		if byteOffset < 0 || byteOffset >= int64(len(allBytes)) {
			return nil, errors.New("DataView byteOffset out of bounds")
		}

		end := byteOffset + byteLength
		if end > int64(len(allBytes)) {
			end = int64(len(allBytes))
		}

		return allBytes[byteOffset:end], nil
	} else {
		ab, ok = asObject.Export().(sobek.ArrayBuffer)
		if !ok {
			return nil, errors.New("data is neither an ArrayBuffer, nor a TypedArray nor DataView")
		}
	}

	return ab.Bytes(), nil
}

// IsInstanceOf returns true if the given value is an instance of the given constructor
// This uses the technique described in https://github.com/dop251/sobek/issues/379#issuecomment-1164441879
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
