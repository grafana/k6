package goja

import (
	"math"
	"reflect"
	"strconv"
	"unsafe"

	"github.com/dop251/goja/unistring"
)

type byteOrder bool

const (
	bigEndian    byteOrder = false
	littleEndian byteOrder = true
)

var (
	nativeEndian byteOrder

	arrayBufferType = reflect.TypeOf(ArrayBuffer{})
)

type typedArrayObjectCtor func(buf *arrayBufferObject, offset, length int, proto *Object) *typedArrayObject

type arrayBufferObject struct {
	baseObject
	detached bool
	data     []byte
}

// ArrayBuffer is a Go wrapper around ECMAScript ArrayBuffer. Calling Runtime.ToValue() on it
// returns the underlying ArrayBuffer. Calling Export() on an ECMAScript ArrayBuffer returns a wrapper.
// Use Runtime.NewArrayBuffer([]byte) to create one.
type ArrayBuffer struct {
	buf *arrayBufferObject
}

type dataViewObject struct {
	baseObject
	viewedArrayBuf      *arrayBufferObject
	byteLen, byteOffset int
}

type typedArray interface {
	toRaw(Value) uint64
	get(idx int) Value
	set(idx int, value Value)
	getRaw(idx int) uint64
	setRaw(idx int, raw uint64)
	less(i, j int) bool
	swap(i, j int)
	typeMatch(v Value) bool
}

type uint8Array []uint8
type uint8ClampedArray []uint8
type int8Array []int8
type uint16Array []uint16
type int16Array []int16
type uint32Array []uint32
type int32Array []int32
type float32Array []float32
type float64Array []float64

type typedArrayObject struct {
	baseObject
	viewedArrayBuf *arrayBufferObject
	defaultCtor    *Object
	length, offset int
	elemSize       int
	typedArray     typedArray
}

func (a ArrayBuffer) toValue(r *Runtime) Value {
	if a.buf == nil {
		return _null
	}
	v := a.buf.val
	if v.runtime != r {
		panic(r.NewTypeError("Illegal runtime transition of an ArrayBuffer"))
	}
	return v
}

// Bytes returns the underlying []byte for this ArrayBuffer.
// For detached ArrayBuffers returns nil.
func (a ArrayBuffer) Bytes() []byte {
	return a.buf.data
}

// Detach the ArrayBuffer. After this, the underlying []byte becomes unreferenced and any attempt
// to use this ArrayBuffer results in a TypeError.
// Returns false if it was already detached, true otherwise.
// Note, this method may only be called from the goroutine that 'owns' the Runtime, it may not
// be called concurrently.
func (a ArrayBuffer) Detach() bool {
	if a.buf.detached {
		return false
	}
	a.buf.detach()
	return true
}

// Detached returns true if the ArrayBuffer is detached.
func (a ArrayBuffer) Detached() bool {
	return a.buf.detached
}

func (r *Runtime) NewArrayBuffer(data []byte) ArrayBuffer {
	buf := r._newArrayBuffer(r.global.ArrayBufferPrototype, nil)
	buf.data = data
	return ArrayBuffer{
		buf: buf,
	}
}

func (a *uint8Array) get(idx int) Value {
	return intToValue(int64((*a)[idx]))
}

func (a *uint8Array) getRaw(idx int) uint64 {
	return uint64((*a)[idx])
}

func (a *uint8Array) set(idx int, value Value) {
	(*a)[idx] = toUint8(value)
}

func (a *uint8Array) toRaw(v Value) uint64 {
	return uint64(toUint8(v))
}

func (a *uint8Array) setRaw(idx int, v uint64) {
	(*a)[idx] = uint8(v)
}

func (a *uint8Array) less(i, j int) bool {
	return (*a)[i] < (*a)[j]
}

func (a *uint8Array) swap(i, j int) {
	(*a)[i], (*a)[j] = (*a)[j], (*a)[i]
}

func (a *uint8Array) typeMatch(v Value) bool {
	if i, ok := v.(valueInt); ok {
		return i >= 0 && i <= 255
	}
	return false
}

func (a *uint8ClampedArray) get(idx int) Value {
	return intToValue(int64((*a)[idx]))
}

func (a *uint8ClampedArray) getRaw(idx int) uint64 {
	return uint64((*a)[idx])
}

func (a *uint8ClampedArray) set(idx int, value Value) {
	(*a)[idx] = toUint8Clamp(value)
}

func (a *uint8ClampedArray) toRaw(v Value) uint64 {
	return uint64(toUint8Clamp(v))
}

func (a *uint8ClampedArray) setRaw(idx int, v uint64) {
	(*a)[idx] = uint8(v)
}

func (a *uint8ClampedArray) less(i, j int) bool {
	return (*a)[i] < (*a)[j]
}

func (a *uint8ClampedArray) swap(i, j int) {
	(*a)[i], (*a)[j] = (*a)[j], (*a)[i]
}

func (a *uint8ClampedArray) typeMatch(v Value) bool {
	if i, ok := v.(valueInt); ok {
		return i >= 0 && i <= 255
	}
	return false
}

func (a *int8Array) get(idx int) Value {
	return intToValue(int64((*a)[idx]))
}

func (a *int8Array) getRaw(idx int) uint64 {
	return uint64((*a)[idx])
}

func (a *int8Array) set(idx int, value Value) {
	(*a)[idx] = toInt8(value)
}

func (a *int8Array) toRaw(v Value) uint64 {
	return uint64(toInt8(v))
}

func (a *int8Array) setRaw(idx int, v uint64) {
	(*a)[idx] = int8(v)
}

func (a *int8Array) less(i, j int) bool {
	return (*a)[i] < (*a)[j]
}

func (a *int8Array) swap(i, j int) {
	(*a)[i], (*a)[j] = (*a)[j], (*a)[i]
}

func (a *int8Array) typeMatch(v Value) bool {
	if i, ok := v.(valueInt); ok {
		return i >= math.MinInt8 && i <= math.MaxInt8
	}
	return false
}

func (a *uint16Array) get(idx int) Value {
	return intToValue(int64((*a)[idx]))
}

func (a *uint16Array) getRaw(idx int) uint64 {
	return uint64((*a)[idx])
}

func (a *uint16Array) set(idx int, value Value) {
	(*a)[idx] = toUint16(value)
}

func (a *uint16Array) toRaw(v Value) uint64 {
	return uint64(toUint16(v))
}

func (a *uint16Array) setRaw(idx int, v uint64) {
	(*a)[idx] = uint16(v)
}

func (a *uint16Array) less(i, j int) bool {
	return (*a)[i] < (*a)[j]
}

func (a *uint16Array) swap(i, j int) {
	(*a)[i], (*a)[j] = (*a)[j], (*a)[i]
}

func (a *uint16Array) typeMatch(v Value) bool {
	if i, ok := v.(valueInt); ok {
		return i >= 0 && i <= math.MaxUint16
	}
	return false
}

func (a *int16Array) get(idx int) Value {
	return intToValue(int64((*a)[idx]))
}

func (a *int16Array) getRaw(idx int) uint64 {
	return uint64((*a)[idx])
}

func (a *int16Array) set(idx int, value Value) {
	(*a)[idx] = toInt16(value)
}

func (a *int16Array) toRaw(v Value) uint64 {
	return uint64(toInt16(v))
}

func (a *int16Array) setRaw(idx int, v uint64) {
	(*a)[idx] = int16(v)
}

func (a *int16Array) less(i, j int) bool {
	return (*a)[i] < (*a)[j]
}

func (a *int16Array) swap(i, j int) {
	(*a)[i], (*a)[j] = (*a)[j], (*a)[i]
}

func (a *int16Array) typeMatch(v Value) bool {
	if i, ok := v.(valueInt); ok {
		return i >= math.MinInt16 && i <= math.MaxInt16
	}
	return false
}

func (a *uint32Array) get(idx int) Value {
	return intToValue(int64((*a)[idx]))
}

func (a *uint32Array) getRaw(idx int) uint64 {
	return uint64((*a)[idx])
}

func (a *uint32Array) set(idx int, value Value) {
	(*a)[idx] = toUint32(value)
}

func (a *uint32Array) toRaw(v Value) uint64 {
	return uint64(toUint32(v))
}

func (a *uint32Array) setRaw(idx int, v uint64) {
	(*a)[idx] = uint32(v)
}

func (a *uint32Array) less(i, j int) bool {
	return (*a)[i] < (*a)[j]
}

func (a *uint32Array) swap(i, j int) {
	(*a)[i], (*a)[j] = (*a)[j], (*a)[i]
}

func (a *uint32Array) typeMatch(v Value) bool {
	if i, ok := v.(valueInt); ok {
		return i >= 0 && i <= math.MaxUint32
	}
	return false
}

func (a *int32Array) get(idx int) Value {
	return intToValue(int64((*a)[idx]))
}

func (a *int32Array) getRaw(idx int) uint64 {
	return uint64((*a)[idx])
}

func (a *int32Array) set(idx int, value Value) {
	(*a)[idx] = toInt32(value)
}

func (a *int32Array) toRaw(v Value) uint64 {
	return uint64(toInt32(v))
}

func (a *int32Array) setRaw(idx int, v uint64) {
	(*a)[idx] = int32(v)
}

func (a *int32Array) less(i, j int) bool {
	return (*a)[i] < (*a)[j]
}

func (a *int32Array) swap(i, j int) {
	(*a)[i], (*a)[j] = (*a)[j], (*a)[i]
}

func (a *int32Array) typeMatch(v Value) bool {
	if i, ok := v.(valueInt); ok {
		return i >= math.MinInt32 && i <= math.MaxInt32
	}
	return false
}

func (a *float32Array) get(idx int) Value {
	return floatToValue(float64((*a)[idx]))
}

func (a *float32Array) getRaw(idx int) uint64 {
	return uint64(math.Float32bits((*a)[idx]))
}

func (a *float32Array) set(idx int, value Value) {
	(*a)[idx] = toFloat32(value)
}

func (a *float32Array) toRaw(v Value) uint64 {
	return uint64(math.Float32bits(toFloat32(v)))
}

func (a *float32Array) setRaw(idx int, v uint64) {
	(*a)[idx] = math.Float32frombits(uint32(v))
}

func typedFloatLess(x, y float64) bool {
	xNan := math.IsNaN(x)
	yNan := math.IsNaN(y)
	if yNan {
		return !xNan
	} else if xNan {
		return false
	}
	if x == 0 && y == 0 { // handle neg zero
		return math.Signbit(x)
	}
	return x < y
}

func (a *float32Array) less(i, j int) bool {
	return typedFloatLess(float64((*a)[i]), float64((*a)[j]))
}

func (a *float32Array) swap(i, j int) {
	(*a)[i], (*a)[j] = (*a)[j], (*a)[i]
}

func (a *float32Array) typeMatch(v Value) bool {
	switch v.(type) {
	case valueInt, valueFloat:
		return true
	}
	return false
}

func (a *float64Array) get(idx int) Value {
	return floatToValue((*a)[idx])
}

func (a *float64Array) getRaw(idx int) uint64 {
	return math.Float64bits((*a)[idx])
}

func (a *float64Array) set(idx int, value Value) {
	(*a)[idx] = value.ToFloat()
}

func (a *float64Array) toRaw(v Value) uint64 {
	return math.Float64bits(v.ToFloat())
}

func (a *float64Array) setRaw(idx int, v uint64) {
	(*a)[idx] = math.Float64frombits(v)
}

func (a *float64Array) less(i, j int) bool {
	return typedFloatLess((*a)[i], (*a)[j])
}

func (a *float64Array) swap(i, j int) {
	(*a)[i], (*a)[j] = (*a)[j], (*a)[i]
}

func (a *float64Array) typeMatch(v Value) bool {
	switch v.(type) {
	case valueInt, valueFloat:
		return true
	}
	return false
}

func (a *typedArrayObject) _getIdx(idx int) Value {
	if 0 <= idx && idx < a.length {
		if !a.viewedArrayBuf.ensureNotDetached(false) {
			return nil
		}
		return a.typedArray.get(idx + a.offset)
	}
	return nil
}

func (a *typedArrayObject) getOwnPropStr(name unistring.String) Value {
	idx, ok := strToIntNum(name)
	if ok {
		v := a._getIdx(idx)
		if v != nil {
			return &valueProperty{
				value:        v,
				writable:     true,
				enumerable:   true,
				configurable: true,
			}
		}
		return nil
	}
	if idx == 0 {
		return nil
	}
	return a.baseObject.getOwnPropStr(name)
}

func (a *typedArrayObject) getOwnPropIdx(idx valueInt) Value {
	v := a._getIdx(toIntClamp(int64(idx)))
	if v != nil {
		return &valueProperty{
			value:        v,
			writable:     true,
			enumerable:   true,
			configurable: true,
		}
	}
	return nil
}

func (a *typedArrayObject) getStr(name unistring.String, receiver Value) Value {
	idx, ok := strToIntNum(name)
	if ok {
		return a._getIdx(idx)
	}
	if idx == 0 {
		return nil
	}
	return a.baseObject.getStr(name, receiver)
}

func (a *typedArrayObject) getIdx(idx valueInt, receiver Value) Value {
	return a._getIdx(toIntClamp(int64(idx)))
}

func (a *typedArrayObject) isValidIntegerIndex(idx int) bool {
	if a.viewedArrayBuf.ensureNotDetached(false) {
		if idx >= 0 && idx < a.length {
			return true
		}
	}
	return false
}

func (a *typedArrayObject) _putIdx(idx int, v Value) {
	v = v.ToNumber()
	if a.isValidIntegerIndex(idx) {
		a.typedArray.set(idx+a.offset, v)
	}
}

func (a *typedArrayObject) _hasIdx(idx int) bool {
	return a.isValidIntegerIndex(idx)
}

func (a *typedArrayObject) setOwnStr(p unistring.String, v Value, throw bool) bool {
	idx, ok := strToIntNum(p)
	if ok {
		a._putIdx(idx, v)
		return true
	}
	if idx == 0 {
		v.ToNumber() // make sure it throws
		return true
	}
	return a.baseObject.setOwnStr(p, v, throw)
}

func (a *typedArrayObject) setOwnIdx(p valueInt, v Value, throw bool) bool {
	a._putIdx(toIntClamp(int64(p)), v)
	return true
}

func (a *typedArrayObject) setForeignStr(p unistring.String, v, receiver Value, throw bool) (res bool, handled bool) {
	return a._setForeignStr(p, a.getOwnPropStr(p), v, receiver, throw)
}

func (a *typedArrayObject) setForeignIdx(p valueInt, v, receiver Value, throw bool) (res bool, handled bool) {
	return a._setForeignIdx(p, trueValIfPresent(a.hasOwnPropertyIdx(p)), v, receiver, throw)
}

func (a *typedArrayObject) hasOwnPropertyStr(name unistring.String) bool {
	idx, ok := strToIntNum(name)
	if ok {
		return a._hasIdx(idx)
	}
	if idx == 0 {
		return false
	}
	return a.baseObject.hasOwnPropertyStr(name)
}

func (a *typedArrayObject) hasOwnPropertyIdx(idx valueInt) bool {
	return a._hasIdx(toIntClamp(int64(idx)))
}

func (a *typedArrayObject) hasPropertyStr(name unistring.String) bool {
	idx, ok := strToIntNum(name)
	if ok {
		return a._hasIdx(idx)
	}
	if idx == 0 {
		return false
	}
	return a.baseObject.hasPropertyStr(name)
}

func (a *typedArrayObject) hasPropertyIdx(idx valueInt) bool {
	return a.hasOwnPropertyIdx(idx)
}

func (a *typedArrayObject) _defineIdxProperty(idx int, desc PropertyDescriptor, throw bool) bool {
	if desc.Configurable == FLAG_FALSE || desc.Enumerable == FLAG_FALSE || desc.IsAccessor() || desc.Writable == FLAG_FALSE {
		a.val.runtime.typeErrorResult(throw, "Cannot redefine property: %d", idx)
		return false
	}
	_, ok := a._defineOwnProperty(unistring.String(strconv.Itoa(idx)), a.getOwnPropIdx(valueInt(idx)), desc, throw)
	if ok {
		if !a.isValidIntegerIndex(idx) {
			a.val.runtime.typeErrorResult(throw, "Invalid typed array index")
			return false
		}
		a._putIdx(idx, desc.Value)
		return true
	}
	return ok
}

func (a *typedArrayObject) defineOwnPropertyStr(name unistring.String, desc PropertyDescriptor, throw bool) bool {
	idx, ok := strToIntNum(name)
	if ok {
		return a._defineIdxProperty(idx, desc, throw)
	}
	if idx == 0 {
		a.viewedArrayBuf.ensureNotDetached(throw)
		a.val.runtime.typeErrorResult(throw, "Invalid typed array index")
		return false
	}
	return a.baseObject.defineOwnPropertyStr(name, desc, throw)
}

func (a *typedArrayObject) defineOwnPropertyIdx(name valueInt, desc PropertyDescriptor, throw bool) bool {
	return a._defineIdxProperty(toIntClamp(int64(name)), desc, throw)
}

func (a *typedArrayObject) deleteStr(name unistring.String, throw bool) bool {
	idx, ok := strToIntNum(name)
	if ok {
		if a.isValidIntegerIndex(idx) {
			a.val.runtime.typeErrorResult(throw, "Cannot delete property '%d' of %s", idx, a.val.String())
			return false
		}
		return true
	}
	if idx == 0 {
		return true
	}
	return a.baseObject.deleteStr(name, throw)
}

func (a *typedArrayObject) deleteIdx(idx valueInt, throw bool) bool {
	if a.viewedArrayBuf.ensureNotDetached(false) && idx >= 0 && int64(idx) < int64(a.length) {
		a.val.runtime.typeErrorResult(throw, "Cannot delete property '%d' of %s", idx, a.val.String())
		return false
	}

	return true
}

func (a *typedArrayObject) stringKeys(all bool, accum []Value) []Value {
	if accum == nil {
		accum = make([]Value, 0, a.length)
	}
	for i := 0; i < a.length; i++ {
		accum = append(accum, asciiString(strconv.Itoa(i)))
	}
	return a.baseObject.stringKeys(all, accum)
}

type typedArrayPropIter struct {
	a   *typedArrayObject
	idx int
}

func (i *typedArrayPropIter) next() (propIterItem, iterNextFunc) {
	if i.idx < i.a.length {
		name := strconv.Itoa(i.idx)
		prop := i.a._getIdx(i.idx)
		i.idx++
		return propIterItem{name: asciiString(name), value: prop}, i.next
	}

	return i.a.baseObject.iterateStringKeys()()
}

func (a *typedArrayObject) iterateStringKeys() iterNextFunc {
	return (&typedArrayPropIter{
		a: a,
	}).next
}

func (r *Runtime) _newTypedArrayObject(buf *arrayBufferObject, offset, length, elemSize int, defCtor *Object, arr typedArray, proto *Object) *typedArrayObject {
	o := &Object{runtime: r}
	a := &typedArrayObject{
		baseObject: baseObject{
			val:        o,
			class:      classObject,
			prototype:  proto,
			extensible: true,
		},
		viewedArrayBuf: buf,
		offset:         offset,
		length:         length,
		elemSize:       elemSize,
		defaultCtor:    defCtor,
		typedArray:     arr,
	}
	o.self = a
	a.init()
	return a

}

func (r *Runtime) newUint8ArrayObject(buf *arrayBufferObject, offset, length int, proto *Object) *typedArrayObject {
	return r._newTypedArrayObject(buf, offset, length, 1, r.global.Uint8Array, (*uint8Array)(&buf.data), proto)
}

func (r *Runtime) newUint8ClampedArrayObject(buf *arrayBufferObject, offset, length int, proto *Object) *typedArrayObject {
	return r._newTypedArrayObject(buf, offset, length, 1, r.global.Uint8ClampedArray, (*uint8ClampedArray)(&buf.data), proto)
}

func (r *Runtime) newInt8ArrayObject(buf *arrayBufferObject, offset, length int, proto *Object) *typedArrayObject {
	return r._newTypedArrayObject(buf, offset, length, 1, r.global.Int8Array, (*int8Array)(unsafe.Pointer(&buf.data)), proto)
}

func (r *Runtime) newUint16ArrayObject(buf *arrayBufferObject, offset, length int, proto *Object) *typedArrayObject {
	return r._newTypedArrayObject(buf, offset, length, 2, r.global.Uint16Array, (*uint16Array)(unsafe.Pointer(&buf.data)), proto)
}

func (r *Runtime) newInt16ArrayObject(buf *arrayBufferObject, offset, length int, proto *Object) *typedArrayObject {
	return r._newTypedArrayObject(buf, offset, length, 2, r.global.Int16Array, (*int16Array)(unsafe.Pointer(&buf.data)), proto)
}

func (r *Runtime) newUint32ArrayObject(buf *arrayBufferObject, offset, length int, proto *Object) *typedArrayObject {
	return r._newTypedArrayObject(buf, offset, length, 4, r.global.Uint32Array, (*uint32Array)(unsafe.Pointer(&buf.data)), proto)
}

func (r *Runtime) newInt32ArrayObject(buf *arrayBufferObject, offset, length int, proto *Object) *typedArrayObject {
	return r._newTypedArrayObject(buf, offset, length, 4, r.global.Int32Array, (*int32Array)(unsafe.Pointer(&buf.data)), proto)
}

func (r *Runtime) newFloat32ArrayObject(buf *arrayBufferObject, offset, length int, proto *Object) *typedArrayObject {
	return r._newTypedArrayObject(buf, offset, length, 4, r.global.Float32Array, (*float32Array)(unsafe.Pointer(&buf.data)), proto)
}

func (r *Runtime) newFloat64ArrayObject(buf *arrayBufferObject, offset, length int, proto *Object) *typedArrayObject {
	return r._newTypedArrayObject(buf, offset, length, 8, r.global.Float64Array, (*float64Array)(unsafe.Pointer(&buf.data)), proto)
}

func (o *dataViewObject) getIdxAndByteOrder(getIdx int, littleEndianVal Value, size int) (int, byteOrder) {
	o.viewedArrayBuf.ensureNotDetached(true)
	if getIdx+size > o.byteLen {
		panic(o.val.runtime.newError(o.val.runtime.global.RangeError, "Index %d is out of bounds", getIdx))
	}
	getIdx += o.byteOffset
	var bo byteOrder
	if littleEndianVal != nil {
		if littleEndianVal.ToBoolean() {
			bo = littleEndian
		} else {
			bo = bigEndian
		}
	} else {
		bo = nativeEndian
	}
	return getIdx, bo
}

func (o *arrayBufferObject) ensureNotDetached(throw bool) bool {
	if o.detached {
		o.val.runtime.typeErrorResult(throw, "ArrayBuffer is detached")
		return false
	}
	return true
}

func (o *arrayBufferObject) getFloat32(idx int, byteOrder byteOrder) float32 {
	return math.Float32frombits(o.getUint32(idx, byteOrder))
}

func (o *arrayBufferObject) setFloat32(idx int, val float32, byteOrder byteOrder) {
	o.setUint32(idx, math.Float32bits(val), byteOrder)
}

func (o *arrayBufferObject) getFloat64(idx int, byteOrder byteOrder) float64 {
	return math.Float64frombits(o.getUint64(idx, byteOrder))
}

func (o *arrayBufferObject) setFloat64(idx int, val float64, byteOrder byteOrder) {
	o.setUint64(idx, math.Float64bits(val), byteOrder)
}

func (o *arrayBufferObject) getUint64(idx int, byteOrder byteOrder) uint64 {
	var b []byte
	if byteOrder == nativeEndian {
		b = o.data[idx : idx+8]
	} else {
		b = make([]byte, 8)
		d := o.data[idx : idx+8]
		b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7] = d[7], d[6], d[5], d[4], d[3], d[2], d[1], d[0]
	}
	return *((*uint64)(unsafe.Pointer(&b[0])))
}

func (o *arrayBufferObject) setUint64(idx int, val uint64, byteOrder byteOrder) {
	if byteOrder == nativeEndian {
		*(*uint64)(unsafe.Pointer(&o.data[idx])) = val
	} else {
		b := (*[8]byte)(unsafe.Pointer(&val))
		d := o.data[idx : idx+8]
		d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7] = b[7], b[6], b[5], b[4], b[3], b[2], b[1], b[0]
	}
}

func (o *arrayBufferObject) getUint32(idx int, byteOrder byteOrder) uint32 {
	var b []byte
	if byteOrder == nativeEndian {
		b = o.data[idx : idx+4]
	} else {
		b = make([]byte, 4)
		d := o.data[idx : idx+4]
		b[0], b[1], b[2], b[3] = d[3], d[2], d[1], d[0]
	}
	return *((*uint32)(unsafe.Pointer(&b[0])))
}

func (o *arrayBufferObject) setUint32(idx int, val uint32, byteOrder byteOrder) {
	o.ensureNotDetached(true)
	if byteOrder == nativeEndian {
		*(*uint32)(unsafe.Pointer(&o.data[idx])) = val
	} else {
		b := (*[4]byte)(unsafe.Pointer(&val))
		d := o.data[idx : idx+4]
		d[0], d[1], d[2], d[3] = b[3], b[2], b[1], b[0]
	}
}

func (o *arrayBufferObject) getUint16(idx int, byteOrder byteOrder) uint16 {
	var b []byte
	if byteOrder == nativeEndian {
		b = o.data[idx : idx+2]
	} else {
		b = make([]byte, 2)
		d := o.data[idx : idx+2]
		b[0], b[1] = d[1], d[0]
	}
	return *((*uint16)(unsafe.Pointer(&b[0])))
}

func (o *arrayBufferObject) setUint16(idx int, val uint16, byteOrder byteOrder) {
	if byteOrder == nativeEndian {
		*(*uint16)(unsafe.Pointer(&o.data[idx])) = val
	} else {
		b := (*[2]byte)(unsafe.Pointer(&val))
		d := o.data[idx : idx+2]
		d[0], d[1] = b[1], b[0]
	}
}

func (o *arrayBufferObject) getUint8(idx int) uint8 {
	return o.data[idx]
}

func (o *arrayBufferObject) setUint8(idx int, val uint8) {
	o.data[idx] = val
}

func (o *arrayBufferObject) getInt32(idx int, byteOrder byteOrder) int32 {
	return int32(o.getUint32(idx, byteOrder))
}

func (o *arrayBufferObject) setInt32(idx int, val int32, byteOrder byteOrder) {
	o.setUint32(idx, uint32(val), byteOrder)
}

func (o *arrayBufferObject) getInt16(idx int, byteOrder byteOrder) int16 {
	return int16(o.getUint16(idx, byteOrder))
}

func (o *arrayBufferObject) setInt16(idx int, val int16, byteOrder byteOrder) {
	o.setUint16(idx, uint16(val), byteOrder)
}

func (o *arrayBufferObject) getInt8(idx int) int8 {
	return int8(o.data[idx])
}

func (o *arrayBufferObject) setInt8(idx int, val int8) {
	o.setUint8(idx, uint8(val))
}

func (o *arrayBufferObject) detach() {
	o.data = nil
	o.detached = true
}

func (o *arrayBufferObject) exportType() reflect.Type {
	return arrayBufferType
}

func (o *arrayBufferObject) export(*objectExportCtx) interface{} {
	return ArrayBuffer{
		buf: o,
	}
}

func (r *Runtime) _newArrayBuffer(proto *Object, o *Object) *arrayBufferObject {
	if o == nil {
		o = &Object{runtime: r}
	}
	b := &arrayBufferObject{
		baseObject: baseObject{
			class:      classObject,
			val:        o,
			prototype:  proto,
			extensible: true,
		},
	}
	o.self = b
	b.init()
	return b
}

func init() {
	buf := [2]byte{}
	*(*uint16)(unsafe.Pointer(&buf[0])) = uint16(0xCAFE)

	switch buf {
	case [2]byte{0xFE, 0xCA}:
		nativeEndian = littleEndian
	case [2]byte{0xCA, 0xFE}:
		nativeEndian = bigEndian
	default:
		panic("Could not determine native endianness.")
	}
}
