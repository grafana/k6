package goja

import (
	"fmt"
	"math"
	"sort"
	"unsafe"

	"github.com/dop251/goja/unistring"
)

type typedArraySortCtx struct {
	ta           *typedArrayObject
	compare      func(FunctionCall) Value
	needValidate bool
}

func (ctx *typedArraySortCtx) Len() int {
	return ctx.ta.length
}

func (ctx *typedArraySortCtx) Less(i, j int) bool {
	if ctx.needValidate {
		ctx.ta.viewedArrayBuf.ensureNotDetached(true)
		ctx.needValidate = false
	}
	offset := ctx.ta.offset
	if ctx.compare != nil {
		x := ctx.ta.typedArray.get(offset + i)
		y := ctx.ta.typedArray.get(offset + j)
		res := ctx.compare(FunctionCall{
			This:      _undefined,
			Arguments: []Value{x, y},
		}).ToNumber()
		ctx.needValidate = true
		if i, ok := res.(valueInt); ok {
			return i < 0
		}
		f := res.ToFloat()
		if f < 0 {
			return true
		}
		if f > 0 {
			return false
		}
		if math.Signbit(f) {
			return true
		}
		return false
	}

	return ctx.ta.typedArray.less(offset+i, offset+j)
}

func (ctx *typedArraySortCtx) Swap(i, j int) {
	if ctx.needValidate {
		ctx.ta.viewedArrayBuf.ensureNotDetached(true)
		ctx.needValidate = false
	}
	offset := ctx.ta.offset
	ctx.ta.typedArray.swap(offset+i, offset+j)
}

func allocByteSlice(size int) (b []byte) {
	defer func() {
		if x := recover(); x != nil {
			panic(rangeError(fmt.Sprintf("Buffer size is too large: %d", size)))
		}
	}()
	if size < 0 {
		panic(rangeError(fmt.Sprintf("Invalid buffer size: %d", size)))
	}
	b = make([]byte, size)
	return
}

func (r *Runtime) builtin_newArrayBuffer(args []Value, newTarget *Object) *Object {
	if newTarget == nil {
		panic(r.needNew("ArrayBuffer"))
	}
	b := r._newArrayBuffer(r.getPrototypeFromCtor(newTarget, r.global.ArrayBuffer, r.global.ArrayBufferPrototype), nil)
	if len(args) > 0 {
		b.data = allocByteSlice(r.toIndex(args[0]))
	}
	return b.val
}

func (r *Runtime) arrayBufferProto_getByteLength(call FunctionCall) Value {
	o := r.toObject(call.This)
	if b, ok := o.self.(*arrayBufferObject); ok {
		if b.ensureNotDetached(false) {
			return intToValue(int64(len(b.data)))
		}
		return intToValue(0)
	}
	panic(r.NewTypeError("Object is not ArrayBuffer: %s", o))
}

func (r *Runtime) arrayBufferProto_slice(call FunctionCall) Value {
	o := r.toObject(call.This)
	if b, ok := o.self.(*arrayBufferObject); ok {
		l := int64(len(b.data))
		start := relToIdx(call.Argument(0).ToInteger(), l)
		var stop int64
		if arg := call.Argument(1); arg != _undefined {
			stop = arg.ToInteger()
		} else {
			stop = l
		}
		stop = relToIdx(stop, l)
		newLen := max(stop-start, 0)
		ret := r.speciesConstructor(o, r.global.ArrayBuffer)([]Value{intToValue(newLen)}, nil)
		if ab, ok := ret.self.(*arrayBufferObject); ok {
			if newLen > 0 {
				b.ensureNotDetached(true)
				if ret == o {
					panic(r.NewTypeError("Species constructor returned the same ArrayBuffer"))
				}
				if int64(len(ab.data)) < newLen {
					panic(r.NewTypeError("Species constructor returned an ArrayBuffer that is too small: %d", len(ab.data)))
				}
				ab.ensureNotDetached(true)
				copy(ab.data, b.data[start:stop])
			}
			return ret
		}
		panic(r.NewTypeError("Species constructor did not return an ArrayBuffer: %s", ret.String()))
	}
	panic(r.NewTypeError("Object is not ArrayBuffer: %s", o))
}

func (r *Runtime) arrayBuffer_isView(call FunctionCall) Value {
	if o, ok := call.Argument(0).(*Object); ok {
		if _, ok := o.self.(*dataViewObject); ok {
			return valueTrue
		}
		if _, ok := o.self.(*typedArrayObject); ok {
			return valueTrue
		}
	}
	return valueFalse
}

func (r *Runtime) newDataView(args []Value, newTarget *Object) *Object {
	if newTarget == nil {
		panic(r.needNew("DataView"))
	}
	proto := r.getPrototypeFromCtor(newTarget, r.global.DataView, r.global.DataViewPrototype)
	var bufArg Value
	if len(args) > 0 {
		bufArg = args[0]
	}
	var buffer *arrayBufferObject
	if o, ok := bufArg.(*Object); ok {
		if b, ok := o.self.(*arrayBufferObject); ok {
			buffer = b
		}
	}
	if buffer == nil {
		panic(r.NewTypeError("First argument to DataView constructor must be an ArrayBuffer"))
	}
	var byteOffset, byteLen int
	if len(args) > 1 {
		offsetArg := nilSafe(args[1])
		byteOffset = r.toIndex(offsetArg)
		buffer.ensureNotDetached(true)
		if byteOffset > len(buffer.data) {
			panic(r.newError(r.global.RangeError, "Start offset %s is outside the bounds of the buffer", offsetArg.String()))
		}
	}
	if len(args) > 2 && args[2] != nil && args[2] != _undefined {
		byteLen = r.toIndex(args[2])
		if byteOffset+byteLen > len(buffer.data) {
			panic(r.newError(r.global.RangeError, "Invalid DataView length %d", byteLen))
		}
	} else {
		byteLen = len(buffer.data) - byteOffset
	}
	o := &Object{runtime: r}
	b := &dataViewObject{
		baseObject: baseObject{
			class:      classObject,
			val:        o,
			prototype:  proto,
			extensible: true,
		},
		viewedArrayBuf: buffer,
		byteOffset:     byteOffset,
		byteLen:        byteLen,
	}
	o.self = b
	b.init()
	return o
}

func (r *Runtime) dataViewProto_getBuffer(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		return dv.viewedArrayBuf.val
	}
	panic(r.NewTypeError("Method get DataView.prototype.buffer called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_getByteLen(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		dv.viewedArrayBuf.ensureNotDetached(true)
		return intToValue(int64(dv.byteLen))
	}
	panic(r.NewTypeError("Method get DataView.prototype.byteLength called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_getByteOffset(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		dv.viewedArrayBuf.ensureNotDetached(true)
		return intToValue(int64(dv.byteOffset))
	}
	panic(r.NewTypeError("Method get DataView.prototype.byteOffset called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_getFloat32(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		return floatToValue(float64(dv.viewedArrayBuf.getFloat32(dv.getIdxAndByteOrder(r.toIndex(call.Argument(0)), call.Argument(1), 4))))
	}
	panic(r.NewTypeError("Method DataView.prototype.getFloat32 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_getFloat64(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		return floatToValue(dv.viewedArrayBuf.getFloat64(dv.getIdxAndByteOrder(r.toIndex(call.Argument(0)), call.Argument(1), 8)))
	}
	panic(r.NewTypeError("Method DataView.prototype.getFloat64 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_getInt8(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		idx, _ := dv.getIdxAndByteOrder(r.toIndex(call.Argument(0)), call.Argument(1), 1)
		return intToValue(int64(dv.viewedArrayBuf.getInt8(idx)))
	}
	panic(r.NewTypeError("Method DataView.prototype.getInt8 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_getInt16(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		return intToValue(int64(dv.viewedArrayBuf.getInt16(dv.getIdxAndByteOrder(r.toIndex(call.Argument(0)), call.Argument(1), 2))))
	}
	panic(r.NewTypeError("Method DataView.prototype.getInt16 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_getInt32(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		return intToValue(int64(dv.viewedArrayBuf.getInt32(dv.getIdxAndByteOrder(r.toIndex(call.Argument(0)), call.Argument(1), 4))))
	}
	panic(r.NewTypeError("Method DataView.prototype.getInt32 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_getUint8(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		idx, _ := dv.getIdxAndByteOrder(r.toIndex(call.Argument(0)), call.Argument(1), 1)
		return intToValue(int64(dv.viewedArrayBuf.getUint8(idx)))
	}
	panic(r.NewTypeError("Method DataView.prototype.getUint8 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_getUint16(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		return intToValue(int64(dv.viewedArrayBuf.getUint16(dv.getIdxAndByteOrder(r.toIndex(call.Argument(0)), call.Argument(1), 2))))
	}
	panic(r.NewTypeError("Method DataView.prototype.getUint16 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_getUint32(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		return intToValue(int64(dv.viewedArrayBuf.getUint32(dv.getIdxAndByteOrder(r.toIndex(call.Argument(0)), call.Argument(1), 4))))
	}
	panic(r.NewTypeError("Method DataView.prototype.getUint32 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_setFloat32(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		idxVal := r.toIndex(call.Argument(0))
		val := toFloat32(call.Argument(1))
		idx, bo := dv.getIdxAndByteOrder(idxVal, call.Argument(2), 4)
		dv.viewedArrayBuf.setFloat32(idx, val, bo)
		return _undefined
	}
	panic(r.NewTypeError("Method DataView.prototype.setFloat32 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_setFloat64(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		idxVal := r.toIndex(call.Argument(0))
		val := call.Argument(1).ToFloat()
		idx, bo := dv.getIdxAndByteOrder(idxVal, call.Argument(2), 8)
		dv.viewedArrayBuf.setFloat64(idx, val, bo)
		return _undefined
	}
	panic(r.NewTypeError("Method DataView.prototype.setFloat64 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_setInt8(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		idxVal := r.toIndex(call.Argument(0))
		val := toInt8(call.Argument(1))
		idx, _ := dv.getIdxAndByteOrder(idxVal, call.Argument(2), 1)
		dv.viewedArrayBuf.setInt8(idx, val)
		return _undefined
	}
	panic(r.NewTypeError("Method DataView.prototype.setInt8 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_setInt16(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		idxVal := r.toIndex(call.Argument(0))
		val := toInt16(call.Argument(1))
		idx, bo := dv.getIdxAndByteOrder(idxVal, call.Argument(2), 2)
		dv.viewedArrayBuf.setInt16(idx, val, bo)
		return _undefined
	}
	panic(r.NewTypeError("Method DataView.prototype.setInt16 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_setInt32(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		idxVal := r.toIndex(call.Argument(0))
		val := toInt32(call.Argument(1))
		idx, bo := dv.getIdxAndByteOrder(idxVal, call.Argument(2), 4)
		dv.viewedArrayBuf.setInt32(idx, val, bo)
		return _undefined
	}
	panic(r.NewTypeError("Method DataView.prototype.setInt32 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_setUint8(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		idxVal := r.toIndex(call.Argument(0))
		val := toUint8(call.Argument(1))
		idx, _ := dv.getIdxAndByteOrder(idxVal, call.Argument(2), 1)
		dv.viewedArrayBuf.setUint8(idx, val)
		return _undefined
	}
	panic(r.NewTypeError("Method DataView.prototype.setUint8 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_setUint16(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		idxVal := r.toIndex(call.Argument(0))
		val := toUint16(call.Argument(1))
		idx, bo := dv.getIdxAndByteOrder(idxVal, call.Argument(2), 2)
		dv.viewedArrayBuf.setUint16(idx, val, bo)
		return _undefined
	}
	panic(r.NewTypeError("Method DataView.prototype.setUint16 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_setUint32(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		idxVal := r.toIndex(call.Argument(0))
		val := toUint32(call.Argument(1))
		idx, bo := dv.getIdxAndByteOrder(idxVal, call.Argument(2), 4)
		dv.viewedArrayBuf.setUint32(idx, val, bo)
		return _undefined
	}
	panic(r.NewTypeError("Method DataView.prototype.setUint32 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_getBuffer(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		return ta.viewedArrayBuf.val
	}
	panic(r.NewTypeError("Method get TypedArray.prototype.buffer called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_getByteLen(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		if ta.viewedArrayBuf.data == nil {
			return _positiveZero
		}
		return intToValue(int64(ta.length) * int64(ta.elemSize))
	}
	panic(r.NewTypeError("Method get TypedArray.prototype.byteLength called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_getLength(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		if ta.viewedArrayBuf.data == nil {
			return _positiveZero
		}
		return intToValue(int64(ta.length))
	}
	panic(r.NewTypeError("Method get TypedArray.prototype.length called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_getByteOffset(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		if ta.viewedArrayBuf.data == nil {
			return _positiveZero
		}
		return intToValue(int64(ta.offset) * int64(ta.elemSize))
	}
	panic(r.NewTypeError("Method get TypedArray.prototype.byteOffset called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_copyWithin(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		l := int64(ta.length)
		var relEnd int64
		to := toIntStrict(relToIdx(call.Argument(0).ToInteger(), l))
		from := toIntStrict(relToIdx(call.Argument(1).ToInteger(), l))
		if end := call.Argument(2); end != _undefined {
			relEnd = end.ToInteger()
		} else {
			relEnd = l
		}
		final := toIntStrict(relToIdx(relEnd, l))
		data := ta.viewedArrayBuf.data
		offset := ta.offset
		elemSize := ta.elemSize
		if final > from {
			ta.viewedArrayBuf.ensureNotDetached(true)
			copy(data[(offset+to)*elemSize:], data[(offset+from)*elemSize:(offset+final)*elemSize])
		}
		return call.This
	}
	panic(r.NewTypeError("Method TypedArray.prototype.copyWithin called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_entries(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		return r.createArrayIterator(ta.val, iterationKindKeyValue)
	}
	panic(r.NewTypeError("Method TypedArray.prototype.entries called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_every(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		callbackFn := r.toCallable(call.Argument(0))
		fc := FunctionCall{
			This:      call.Argument(1),
			Arguments: []Value{nil, nil, call.This},
		}
		for k := 0; k < ta.length; k++ {
			if ta.isValidIntegerIndex(k) {
				fc.Arguments[0] = ta.typedArray.get(ta.offset + k)
			} else {
				fc.Arguments[0] = _undefined
			}
			fc.Arguments[1] = intToValue(int64(k))
			if !callbackFn(fc).ToBoolean() {
				return valueFalse
			}
		}
		return valueTrue

	}
	panic(r.NewTypeError("Method TypedArray.prototype.every called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_fill(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		l := int64(ta.length)
		k := toIntStrict(relToIdx(call.Argument(1).ToInteger(), l))
		var relEnd int64
		if endArg := call.Argument(2); endArg != _undefined {
			relEnd = endArg.ToInteger()
		} else {
			relEnd = l
		}
		final := toIntStrict(relToIdx(relEnd, l))
		value := ta.typedArray.toRaw(call.Argument(0))
		ta.viewedArrayBuf.ensureNotDetached(true)
		for ; k < final; k++ {
			ta.typedArray.setRaw(ta.offset+k, value)
		}
		return call.This
	}
	panic(r.NewTypeError("Method TypedArray.prototype.fill called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_filter(call FunctionCall) Value {
	o := r.toObject(call.This)
	if ta, ok := o.self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		callbackFn := r.toCallable(call.Argument(0))
		fc := FunctionCall{
			This:      call.Argument(1),
			Arguments: []Value{nil, nil, call.This},
		}
		buf := make([]byte, 0, ta.length*ta.elemSize)
		captured := 0
		rawVal := make([]byte, ta.elemSize)
		for k := 0; k < ta.length; k++ {
			if ta.isValidIntegerIndex(k) {
				fc.Arguments[0] = ta.typedArray.get(ta.offset + k)
				i := (ta.offset + k) * ta.elemSize
				copy(rawVal, ta.viewedArrayBuf.data[i:])
			} else {
				fc.Arguments[0] = _undefined
				for i := range rawVal {
					rawVal[i] = 0
				}
			}
			fc.Arguments[1] = intToValue(int64(k))
			if callbackFn(fc).ToBoolean() {
				buf = append(buf, rawVal...)
				captured++
			}
		}
		c := r.speciesConstructorObj(o, ta.defaultCtor)
		ab := r._newArrayBuffer(r.global.ArrayBufferPrototype, nil)
		ab.data = buf
		kept := r.toConstructor(ta.defaultCtor)([]Value{ab.val}, ta.defaultCtor)
		if c == ta.defaultCtor {
			return kept
		} else {
			ret := r.typedArrayCreate(c, intToValue(int64(captured)))
			keptTa := kept.self.(*typedArrayObject)
			for i := 0; i < captured; i++ {
				ret.typedArray.set(i, keptTa.typedArray.get(keptTa.offset+i))
			}
			return ret.val
		}
	}
	panic(r.NewTypeError("Method TypedArray.prototype.filter called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_find(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		predicate := r.toCallable(call.Argument(0))
		fc := FunctionCall{
			This:      call.Argument(1),
			Arguments: []Value{nil, nil, call.This},
		}
		for k := 0; k < ta.length; k++ {
			var val Value
			if ta.isValidIntegerIndex(k) {
				val = ta.typedArray.get(ta.offset + k)
			}
			fc.Arguments[0] = val
			fc.Arguments[1] = intToValue(int64(k))
			if predicate(fc).ToBoolean() {
				return val
			}
		}
		return _undefined
	}
	panic(r.NewTypeError("Method TypedArray.prototype.find called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_findIndex(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		predicate := r.toCallable(call.Argument(0))
		fc := FunctionCall{
			This:      call.Argument(1),
			Arguments: []Value{nil, nil, call.This},
		}
		for k := 0; k < ta.length; k++ {
			if ta.isValidIntegerIndex(k) {
				fc.Arguments[0] = ta.typedArray.get(ta.offset + k)
			} else {
				fc.Arguments[0] = _undefined
			}
			fc.Arguments[1] = intToValue(int64(k))
			if predicate(fc).ToBoolean() {
				return fc.Arguments[1]
			}
		}
		return intToValue(-1)
	}
	panic(r.NewTypeError("Method TypedArray.prototype.findIndex called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_forEach(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		callbackFn := r.toCallable(call.Argument(0))
		fc := FunctionCall{
			This:      call.Argument(1),
			Arguments: []Value{nil, nil, call.This},
		}
		for k := 0; k < ta.length; k++ {
			var val Value
			if ta.isValidIntegerIndex(k) {
				val = ta.typedArray.get(ta.offset + k)
			}
			fc.Arguments[0] = val
			fc.Arguments[1] = intToValue(int64(k))
			callbackFn(fc)
		}
		return _undefined
	}
	panic(r.NewTypeError("Method TypedArray.prototype.forEach called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_includes(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		length := int64(ta.length)
		if length == 0 {
			return valueFalse
		}

		n := call.Argument(1).ToInteger()
		if n >= length {
			return valueFalse
		}

		if n < 0 {
			n = max(length+n, 0)
		}

		searchElement := call.Argument(0)
		if searchElement == _negativeZero {
			searchElement = _positiveZero
		}
		startIdx := toIntStrict(n)
		if !ta.viewedArrayBuf.ensureNotDetached(false) {
			if searchElement == _undefined && startIdx < ta.length {
				return valueTrue
			}
			return valueFalse
		}
		if ta.typedArray.typeMatch(searchElement) {
			se := ta.typedArray.toRaw(searchElement)
			for k := startIdx; k < ta.length; k++ {
				if ta.typedArray.getRaw(ta.offset+k) == se {
					return valueTrue
				}
			}
		}
		return valueFalse
	}
	panic(r.NewTypeError("Method TypedArray.prototype.includes called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_indexOf(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		length := int64(ta.length)
		if length == 0 {
			return intToValue(-1)
		}

		n := call.Argument(1).ToInteger()
		if n >= length {
			return intToValue(-1)
		}

		if n < 0 {
			n = max(length+n, 0)
		}

		if ta.viewedArrayBuf.ensureNotDetached(false) {
			searchElement := call.Argument(0)
			if searchElement == _negativeZero {
				searchElement = _positiveZero
			}
			if !IsNaN(searchElement) && ta.typedArray.typeMatch(searchElement) {
				se := ta.typedArray.toRaw(searchElement)
				for k := toIntStrict(n); k < ta.length; k++ {
					if ta.typedArray.getRaw(ta.offset+k) == se {
						return intToValue(int64(k))
					}
				}
			}
		}
		return intToValue(-1)
	}
	panic(r.NewTypeError("Method TypedArray.prototype.indexOf called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_join(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		s := call.Argument(0)
		var sep valueString
		if s != _undefined {
			sep = s.toString()
		} else {
			sep = asciiString(",")
		}
		l := ta.length
		if l == 0 {
			return stringEmpty
		}

		var buf valueStringBuilder

		var element0 Value
		if ta.isValidIntegerIndex(0) {
			element0 = ta.typedArray.get(ta.offset + 0)
		}
		if element0 != nil && element0 != _undefined && element0 != _null {
			buf.WriteString(element0.toString())
		}

		for i := 1; i < l; i++ {
			buf.WriteString(sep)
			if ta.isValidIntegerIndex(i) {
				element := ta.typedArray.get(ta.offset + i)
				if element != nil && element != _undefined && element != _null {
					buf.WriteString(element.toString())
				}
			}
		}

		return buf.String()
	}
	panic(r.NewTypeError("Method TypedArray.prototype.join called on incompatible receiver"))
}

func (r *Runtime) typedArrayProto_keys(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		return r.createArrayIterator(ta.val, iterationKindKey)
	}
	panic(r.NewTypeError("Method TypedArray.prototype.keys called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_lastIndexOf(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		length := int64(ta.length)
		if length == 0 {
			return intToValue(-1)
		}

		var fromIndex int64

		if len(call.Arguments) < 2 {
			fromIndex = length - 1
		} else {
			fromIndex = call.Argument(1).ToInteger()
			if fromIndex >= 0 {
				fromIndex = min(fromIndex, length-1)
			} else {
				fromIndex += length
				if fromIndex < 0 {
					fromIndex = -1 // prevent underflow in toIntStrict() on 32-bit platforms
				}
			}
		}

		if ta.viewedArrayBuf.ensureNotDetached(false) {
			searchElement := call.Argument(0)
			if searchElement == _negativeZero {
				searchElement = _positiveZero
			}
			if !IsNaN(searchElement) && ta.typedArray.typeMatch(searchElement) {
				se := ta.typedArray.toRaw(searchElement)
				for k := toIntStrict(fromIndex); k >= 0; k-- {
					if ta.typedArray.getRaw(ta.offset+k) == se {
						return intToValue(int64(k))
					}
				}
			}
		}

		return intToValue(-1)
	}
	panic(r.NewTypeError("Method TypedArray.prototype.lastIndexOf called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_map(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		callbackFn := r.toCallable(call.Argument(0))
		fc := FunctionCall{
			This:      call.Argument(1),
			Arguments: []Value{nil, nil, call.This},
		}
		dst := r.typedArraySpeciesCreate(ta, []Value{intToValue(int64(ta.length))})
		for i := 0; i < ta.length; i++ {
			if ta.isValidIntegerIndex(i) {
				fc.Arguments[0] = ta.typedArray.get(ta.offset + i)
			} else {
				fc.Arguments[0] = _undefined
			}
			fc.Arguments[1] = intToValue(int64(i))
			dst.typedArray.set(i, callbackFn(fc))
		}
		return dst.val
	}
	panic(r.NewTypeError("Method TypedArray.prototype.map called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_reduce(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		callbackFn := r.toCallable(call.Argument(0))
		fc := FunctionCall{
			This:      _undefined,
			Arguments: []Value{nil, nil, nil, call.This},
		}
		k := 0
		if len(call.Arguments) >= 2 {
			fc.Arguments[0] = call.Argument(1)
		} else {
			if ta.length > 0 {
				fc.Arguments[0] = ta.typedArray.get(ta.offset + 0)
				k = 1
			}
		}
		if fc.Arguments[0] == nil {
			panic(r.NewTypeError("Reduce of empty array with no initial value"))
		}
		for ; k < ta.length; k++ {
			if ta.isValidIntegerIndex(k) {
				fc.Arguments[1] = ta.typedArray.get(ta.offset + k)
			} else {
				fc.Arguments[1] = _undefined
			}
			idx := valueInt(k)
			fc.Arguments[2] = idx
			fc.Arguments[0] = callbackFn(fc)
		}
		return fc.Arguments[0]
	}
	panic(r.NewTypeError("Method TypedArray.prototype.reduce called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_reduceRight(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		callbackFn := r.toCallable(call.Argument(0))
		fc := FunctionCall{
			This:      _undefined,
			Arguments: []Value{nil, nil, nil, call.This},
		}
		k := ta.length - 1
		if len(call.Arguments) >= 2 {
			fc.Arguments[0] = call.Argument(1)
		} else {
			if k >= 0 {
				fc.Arguments[0] = ta.typedArray.get(ta.offset + k)
				k--
			}
		}
		if fc.Arguments[0] == nil {
			panic(r.NewTypeError("Reduce of empty array with no initial value"))
		}
		for ; k >= 0; k-- {
			if ta.isValidIntegerIndex(k) {
				fc.Arguments[1] = ta.typedArray.get(ta.offset + k)
			} else {
				fc.Arguments[1] = _undefined
			}
			idx := valueInt(k)
			fc.Arguments[2] = idx
			fc.Arguments[0] = callbackFn(fc)
		}
		return fc.Arguments[0]
	}
	panic(r.NewTypeError("Method TypedArray.prototype.reduceRight called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_reverse(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		l := ta.length
		middle := l / 2
		for lower := 0; lower != middle; lower++ {
			upper := l - lower - 1
			ta.typedArray.swap(ta.offset+lower, ta.offset+upper)
		}

		return call.This
	}
	panic(r.NewTypeError("Method TypedArray.prototype.reverse called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_set(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		srcObj := call.Argument(0).ToObject(r)
		targetOffset := toIntStrict(call.Argument(1).ToInteger())
		if targetOffset < 0 {
			panic(r.newError(r.global.RangeError, "offset should be >= 0"))
		}
		ta.viewedArrayBuf.ensureNotDetached(true)
		targetLen := ta.length
		if src, ok := srcObj.self.(*typedArrayObject); ok {
			src.viewedArrayBuf.ensureNotDetached(true)
			srcLen := src.length
			if x := srcLen + targetOffset; x < 0 || x > targetLen {
				panic(r.newError(r.global.RangeError, "Source is too large"))
			}
			if src.defaultCtor == ta.defaultCtor {
				copy(ta.viewedArrayBuf.data[(ta.offset+targetOffset)*ta.elemSize:],
					src.viewedArrayBuf.data[src.offset*src.elemSize:(src.offset+srcLen)*src.elemSize])
			} else {
				curSrc := uintptr(unsafe.Pointer(&src.viewedArrayBuf.data[src.offset*src.elemSize]))
				endSrc := curSrc + uintptr(srcLen*src.elemSize)
				curDst := uintptr(unsafe.Pointer(&ta.viewedArrayBuf.data[(ta.offset+targetOffset)*ta.elemSize]))
				dstOffset := ta.offset + targetOffset
				srcOffset := src.offset
				if ta.elemSize == src.elemSize {
					if curDst <= curSrc || curDst >= endSrc {
						for i := 0; i < srcLen; i++ {
							ta.typedArray.set(dstOffset+i, src.typedArray.get(srcOffset+i))
						}
					} else {
						for i := srcLen - 1; i >= 0; i-- {
							ta.typedArray.set(dstOffset+i, src.typedArray.get(srcOffset+i))
						}
					}
				} else {
					x := int(curDst-curSrc) / (src.elemSize - ta.elemSize)
					if x < 0 {
						x = 0
					} else if x > srcLen {
						x = srcLen
					}
					if ta.elemSize < src.elemSize {
						for i := x; i < srcLen; i++ {
							ta.typedArray.set(dstOffset+i, src.typedArray.get(srcOffset+i))
						}
						for i := x - 1; i >= 0; i-- {
							ta.typedArray.set(dstOffset+i, src.typedArray.get(srcOffset+i))
						}
					} else {
						for i := 0; i < x; i++ {
							ta.typedArray.set(dstOffset+i, src.typedArray.get(srcOffset+i))
						}
						for i := srcLen - 1; i >= x; i-- {
							ta.typedArray.set(dstOffset+i, src.typedArray.get(srcOffset+i))
						}
					}
				}
			}
		} else {
			targetLen := ta.length
			srcLen := toIntStrict(toLength(srcObj.self.getStr("length", nil)))
			if x := srcLen + targetOffset; x < 0 || x > targetLen {
				panic(r.newError(r.global.RangeError, "Source is too large"))
			}
			for i := 0; i < srcLen; i++ {
				val := nilSafe(srcObj.self.getIdx(valueInt(i), nil))
				ta.viewedArrayBuf.ensureNotDetached(true)
				if ta.isValidIntegerIndex(i) {
					ta.typedArray.set(targetOffset+i, val)
				}
			}
		}
		return _undefined
	}
	panic(r.NewTypeError("Method TypedArray.prototype.set called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_slice(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		length := int64(ta.length)
		start := toIntStrict(relToIdx(call.Argument(0).ToInteger(), length))
		var e int64
		if endArg := call.Argument(1); endArg != _undefined {
			e = endArg.ToInteger()
		} else {
			e = length
		}
		end := toIntStrict(relToIdx(e, length))

		count := end - start
		if count < 0 {
			count = 0
		}
		dst := r.typedArraySpeciesCreate(ta, []Value{intToValue(int64(count))})
		if dst.defaultCtor == ta.defaultCtor {
			if count > 0 {
				ta.viewedArrayBuf.ensureNotDetached(true)
				offset := ta.offset
				elemSize := ta.elemSize
				copy(dst.viewedArrayBuf.data, ta.viewedArrayBuf.data[(offset+start)*elemSize:(offset+start+count)*elemSize])
			}
		} else {
			for i := 0; i < count; i++ {
				ta.viewedArrayBuf.ensureNotDetached(true)
				dst.typedArray.set(i, ta.typedArray.get(ta.offset+start+i))
			}
		}
		return dst.val
	}
	panic(r.NewTypeError("Method TypedArray.prototype.slice called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_some(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		callbackFn := r.toCallable(call.Argument(0))
		fc := FunctionCall{
			This:      call.Argument(1),
			Arguments: []Value{nil, nil, call.This},
		}
		for k := 0; k < ta.length; k++ {
			if ta.isValidIntegerIndex(k) {
				fc.Arguments[0] = ta.typedArray.get(ta.offset + k)
			} else {
				fc.Arguments[0] = _undefined
			}
			fc.Arguments[1] = intToValue(int64(k))
			if callbackFn(fc).ToBoolean() {
				return valueTrue
			}
		}
		return valueFalse
	}
	panic(r.NewTypeError("Method TypedArray.prototype.some called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_sort(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		var compareFn func(FunctionCall) Value

		if arg := call.Argument(0); arg != _undefined {
			compareFn = r.toCallable(arg)
		}

		ctx := typedArraySortCtx{
			ta:      ta,
			compare: compareFn,
		}

		sort.Stable(&ctx)
		return call.This
	}
	panic(r.NewTypeError("Method TypedArray.prototype.sort called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_subarray(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		l := int64(ta.length)
		beginIdx := relToIdx(call.Argument(0).ToInteger(), l)
		var relEnd int64
		if endArg := call.Argument(1); endArg != _undefined {
			relEnd = endArg.ToInteger()
		} else {
			relEnd = l
		}
		endIdx := relToIdx(relEnd, l)
		newLen := max(endIdx-beginIdx, 0)
		return r.typedArraySpeciesCreate(ta, []Value{ta.viewedArrayBuf.val,
			intToValue((int64(ta.offset) + beginIdx) * int64(ta.elemSize)),
			intToValue(newLen),
		}).val
	}
	panic(r.NewTypeError("Method TypedArray.prototype.subarray called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_toLocaleString(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		length := ta.length
		var buf valueStringBuilder
		for i := 0; i < length; i++ {
			ta.viewedArrayBuf.ensureNotDetached(true)
			if i > 0 {
				buf.WriteRune(',')
			}
			item := ta.typedArray.get(ta.offset + i)
			r.writeItemLocaleString(item, &buf)
		}
		return buf.String()
	}
	panic(r.NewTypeError("Method TypedArray.prototype.toLocaleString called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_values(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		return r.createArrayIterator(ta.val, iterationKindValue)
	}
	panic(r.NewTypeError("Method TypedArray.prototype.values called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_toStringTag(call FunctionCall) Value {
	if obj, ok := call.This.(*Object); ok {
		if ta, ok := obj.self.(*typedArrayObject); ok {
			return nilSafe(ta.defaultCtor.self.getStr("name", nil))
		}
	}

	return _undefined
}

func (r *Runtime) newTypedArray([]Value, *Object) *Object {
	panic(r.NewTypeError("Abstract class TypedArray not directly constructable"))
}

func (r *Runtime) typedArray_from(call FunctionCall) Value {
	c := r.toObject(call.This)
	var mapFc func(call FunctionCall) Value
	thisValue := call.Argument(2)
	if mapFn := call.Argument(1); mapFn != _undefined {
		mapFc = r.toCallable(mapFn)
	}
	source := r.toObject(call.Argument(0))
	usingIter := toMethod(source.self.getSym(SymIterator, nil))
	if usingIter != nil {
		values := r.iterableToList(source, usingIter)
		ta := r.typedArrayCreate(c, intToValue(int64(len(values))))
		if mapFc == nil {
			for idx, val := range values {
				ta.typedArray.set(idx, val)
			}
		} else {
			fc := FunctionCall{
				This:      thisValue,
				Arguments: []Value{nil, nil},
			}
			for idx, val := range values {
				fc.Arguments[0], fc.Arguments[1] = val, intToValue(int64(idx))
				val = mapFc(fc)
				ta.typedArray.set(idx, val)
			}
		}
		return ta.val
	}
	length := toIntStrict(toLength(source.self.getStr("length", nil)))
	ta := r.typedArrayCreate(c, intToValue(int64(length)))
	if mapFc == nil {
		for i := 0; i < length; i++ {
			ta.typedArray.set(i, nilSafe(source.self.getIdx(valueInt(i), nil)))
		}
	} else {
		fc := FunctionCall{
			This:      thisValue,
			Arguments: []Value{nil, nil},
		}
		for i := 0; i < length; i++ {
			idx := valueInt(i)
			fc.Arguments[0], fc.Arguments[1] = source.self.getIdx(idx, nil), idx
			ta.typedArray.set(i, mapFc(fc))
		}
	}
	return ta.val
}

func (r *Runtime) typedArray_of(call FunctionCall) Value {
	ta := r.typedArrayCreate(r.toObject(call.This), intToValue(int64(len(call.Arguments))))
	for i, val := range call.Arguments {
		ta.typedArray.set(i, val)
	}
	return ta.val
}

func (r *Runtime) allocateTypedArray(newTarget *Object, length int, taCtor typedArrayObjectCtor, proto *Object) *typedArrayObject {
	buf := r._newArrayBuffer(r.global.ArrayBufferPrototype, nil)
	ta := taCtor(buf, 0, length, r.getPrototypeFromCtor(newTarget, nil, proto))
	if length > 0 {
		buf.data = allocByteSlice(length * ta.elemSize)
	}
	return ta
}

func (r *Runtime) typedArraySpeciesCreate(ta *typedArrayObject, args []Value) *typedArrayObject {
	return r.typedArrayCreate(r.speciesConstructorObj(ta.val, ta.defaultCtor), args...)
}

func (r *Runtime) typedArrayCreate(ctor *Object, args ...Value) *typedArrayObject {
	o := r.toConstructor(ctor)(args, ctor)
	if ta, ok := o.self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		if len(args) == 1 {
			if l, ok := args[0].(valueInt); ok {
				if ta.length < int(l) {
					panic(r.NewTypeError("Derived TypedArray constructor created an array which was too small"))
				}
			}
		}
		return ta
	}
	panic(r.NewTypeError("Invalid TypedArray: %s", o))
}

func (r *Runtime) typedArrayFrom(ctor, items *Object, mapFn, thisValue Value, taCtor typedArrayObjectCtor, proto *Object) *Object {
	var mapFc func(call FunctionCall) Value
	if mapFn != nil {
		mapFc = r.toCallable(mapFn)
		if thisValue == nil {
			thisValue = _undefined
		}
	}
	usingIter := toMethod(items.self.getSym(SymIterator, nil))
	if usingIter != nil {
		values := r.iterableToList(items, usingIter)
		ta := r.allocateTypedArray(ctor, len(values), taCtor, proto)
		if mapFc == nil {
			for idx, val := range values {
				ta.typedArray.set(idx, val)
			}
		} else {
			fc := FunctionCall{
				This:      thisValue,
				Arguments: []Value{nil, nil},
			}
			for idx, val := range values {
				fc.Arguments[0], fc.Arguments[1] = val, intToValue(int64(idx))
				val = mapFc(fc)
				ta.typedArray.set(idx, val)
			}
		}
		return ta.val
	}
	length := toIntStrict(toLength(items.self.getStr("length", nil)))
	ta := r.allocateTypedArray(ctor, length, taCtor, proto)
	if mapFc == nil {
		for i := 0; i < length; i++ {
			ta.typedArray.set(i, nilSafe(items.self.getIdx(valueInt(i), nil)))
		}
	} else {
		fc := FunctionCall{
			This:      thisValue,
			Arguments: []Value{nil, nil},
		}
		for i := 0; i < length; i++ {
			idx := valueInt(i)
			fc.Arguments[0], fc.Arguments[1] = items.self.getIdx(idx, nil), idx
			ta.typedArray.set(i, mapFc(fc))
		}
	}
	return ta.val
}

func (r *Runtime) _newTypedArrayFromArrayBuffer(ab *arrayBufferObject, args []Value, newTarget *Object, taCtor typedArrayObjectCtor, proto *Object) *Object {
	ta := taCtor(ab, 0, 0, r.getPrototypeFromCtor(newTarget, nil, proto))
	var byteOffset int
	if len(args) > 1 && args[1] != nil && args[1] != _undefined {
		byteOffset = r.toIndex(args[1])
		if byteOffset%ta.elemSize != 0 {
			panic(r.newError(r.global.RangeError, "Start offset of %s should be a multiple of %d", newTarget.self.getStr("name", nil), ta.elemSize))
		}
	}
	var length int
	if len(args) > 2 && args[2] != nil && args[2] != _undefined {
		length = r.toIndex(args[2])
		ab.ensureNotDetached(true)
		if byteOffset+length*ta.elemSize > len(ab.data) {
			panic(r.newError(r.global.RangeError, "Invalid typed array length: %d", length))
		}
	} else {
		ab.ensureNotDetached(true)
		if len(ab.data)%ta.elemSize != 0 {
			panic(r.newError(r.global.RangeError, "Byte length of %s should be a multiple of %d", newTarget.self.getStr("name", nil), ta.elemSize))
		}
		length = (len(ab.data) - byteOffset) / ta.elemSize
		if length < 0 {
			panic(r.newError(r.global.RangeError, "Start offset %d is outside the bounds of the buffer", byteOffset))
		}
	}
	ta.offset = byteOffset / ta.elemSize
	ta.length = length
	return ta.val
}

func (r *Runtime) _newTypedArrayFromTypedArray(src *typedArrayObject, newTarget *Object, taCtor typedArrayObjectCtor, proto *Object) *Object {
	dst := r.allocateTypedArray(newTarget, 0, taCtor, proto)
	src.viewedArrayBuf.ensureNotDetached(true)
	l := src.length

	dst.viewedArrayBuf.prototype = r.getPrototypeFromCtor(r.speciesConstructorObj(src.viewedArrayBuf.val, r.global.ArrayBuffer), r.global.ArrayBuffer, r.global.ArrayBufferPrototype)
	dst.viewedArrayBuf.data = allocByteSlice(toIntStrict(int64(l) * int64(dst.elemSize)))
	src.viewedArrayBuf.ensureNotDetached(true)
	if src.defaultCtor == dst.defaultCtor {
		copy(dst.viewedArrayBuf.data, src.viewedArrayBuf.data[src.offset*src.elemSize:])
		dst.length = src.length
		return dst.val
	}
	dst.length = l
	for i := 0; i < l; i++ {
		dst.typedArray.set(i, src.typedArray.get(src.offset+i))
	}
	return dst.val
}

func (r *Runtime) _newTypedArray(args []Value, newTarget *Object, taCtor typedArrayObjectCtor, proto *Object) *Object {
	if newTarget == nil {
		panic(r.needNew("TypedArray"))
	}
	if len(args) > 0 {
		if obj, ok := args[0].(*Object); ok {
			switch o := obj.self.(type) {
			case *arrayBufferObject:
				return r._newTypedArrayFromArrayBuffer(o, args, newTarget, taCtor, proto)
			case *typedArrayObject:
				return r._newTypedArrayFromTypedArray(o, newTarget, taCtor, proto)
			default:
				return r.typedArrayFrom(newTarget, obj, nil, nil, taCtor, proto)
			}
		}
	}
	var l int
	if len(args) > 0 {
		if arg0 := args[0]; arg0 != nil {
			l = r.toIndex(arg0)
		}
	}
	return r.allocateTypedArray(newTarget, l, taCtor, proto).val
}

func (r *Runtime) newUint8Array(args []Value, newTarget, proto *Object) *Object {
	return r._newTypedArray(args, newTarget, r.newUint8ArrayObject, proto)
}

func (r *Runtime) newUint8ClampedArray(args []Value, newTarget, proto *Object) *Object {
	return r._newTypedArray(args, newTarget, r.newUint8ClampedArrayObject, proto)
}

func (r *Runtime) newInt8Array(args []Value, newTarget, proto *Object) *Object {
	return r._newTypedArray(args, newTarget, r.newInt8ArrayObject, proto)
}

func (r *Runtime) newUint16Array(args []Value, newTarget, proto *Object) *Object {
	return r._newTypedArray(args, newTarget, r.newUint16ArrayObject, proto)
}

func (r *Runtime) newInt16Array(args []Value, newTarget, proto *Object) *Object {
	return r._newTypedArray(args, newTarget, r.newInt16ArrayObject, proto)
}

func (r *Runtime) newUint32Array(args []Value, newTarget, proto *Object) *Object {
	return r._newTypedArray(args, newTarget, r.newUint32ArrayObject, proto)
}

func (r *Runtime) newInt32Array(args []Value, newTarget, proto *Object) *Object {
	return r._newTypedArray(args, newTarget, r.newInt32ArrayObject, proto)
}

func (r *Runtime) newFloat32Array(args []Value, newTarget, proto *Object) *Object {
	return r._newTypedArray(args, newTarget, r.newFloat32ArrayObject, proto)
}

func (r *Runtime) newFloat64Array(args []Value, newTarget, proto *Object) *Object {
	return r._newTypedArray(args, newTarget, r.newFloat64ArrayObject, proto)
}

func (r *Runtime) createArrayBufferProto(val *Object) objectImpl {
	b := newBaseObjectObj(val, r.global.ObjectPrototype, classObject)
	byteLengthProp := &valueProperty{
		accessor:     true,
		configurable: true,
		getterFunc:   r.newNativeFunc(r.arrayBufferProto_getByteLength, nil, "get byteLength", nil, 0),
	}
	b._put("byteLength", byteLengthProp)
	b._putProp("constructor", r.global.ArrayBuffer, true, false, true)
	b._putProp("slice", r.newNativeFunc(r.arrayBufferProto_slice, nil, "slice", nil, 2), true, false, true)
	b._putSym(SymToStringTag, valueProp(asciiString("ArrayBuffer"), false, false, true))
	return b
}

func (r *Runtime) createArrayBuffer(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.builtin_newArrayBuffer, r.global.ArrayBufferPrototype, "ArrayBuffer", 1)
	o._putProp("isView", r.newNativeFunc(r.arrayBuffer_isView, nil, "isView", nil, 1), true, false, true)
	r.putSpeciesReturnThis(o)

	return o
}

func (r *Runtime) createDataViewProto(val *Object) objectImpl {
	b := newBaseObjectObj(val, r.global.ObjectPrototype, classObject)
	b._put("buffer", &valueProperty{
		accessor:     true,
		configurable: true,
		getterFunc:   r.newNativeFunc(r.dataViewProto_getBuffer, nil, "get buffer", nil, 0),
	})
	b._put("byteLength", &valueProperty{
		accessor:     true,
		configurable: true,
		getterFunc:   r.newNativeFunc(r.dataViewProto_getByteLen, nil, "get byteLength", nil, 0),
	})
	b._put("byteOffset", &valueProperty{
		accessor:     true,
		configurable: true,
		getterFunc:   r.newNativeFunc(r.dataViewProto_getByteOffset, nil, "get byteOffset", nil, 0),
	})
	b._putProp("constructor", r.global.DataView, true, false, true)
	b._putProp("getFloat32", r.newNativeFunc(r.dataViewProto_getFloat32, nil, "getFloat32", nil, 1), true, false, true)
	b._putProp("getFloat64", r.newNativeFunc(r.dataViewProto_getFloat64, nil, "getFloat64", nil, 1), true, false, true)
	b._putProp("getInt8", r.newNativeFunc(r.dataViewProto_getInt8, nil, "getInt8", nil, 1), true, false, true)
	b._putProp("getInt16", r.newNativeFunc(r.dataViewProto_getInt16, nil, "getInt16", nil, 1), true, false, true)
	b._putProp("getInt32", r.newNativeFunc(r.dataViewProto_getInt32, nil, "getInt32", nil, 1), true, false, true)
	b._putProp("getUint8", r.newNativeFunc(r.dataViewProto_getUint8, nil, "getUint8", nil, 1), true, false, true)
	b._putProp("getUint16", r.newNativeFunc(r.dataViewProto_getUint16, nil, "getUint16", nil, 1), true, false, true)
	b._putProp("getUint32", r.newNativeFunc(r.dataViewProto_getUint32, nil, "getUint32", nil, 1), true, false, true)
	b._putProp("setFloat32", r.newNativeFunc(r.dataViewProto_setFloat32, nil, "setFloat32", nil, 2), true, false, true)
	b._putProp("setFloat64", r.newNativeFunc(r.dataViewProto_setFloat64, nil, "setFloat64", nil, 2), true, false, true)
	b._putProp("setInt8", r.newNativeFunc(r.dataViewProto_setInt8, nil, "setInt8", nil, 2), true, false, true)
	b._putProp("setInt16", r.newNativeFunc(r.dataViewProto_setInt16, nil, "setInt16", nil, 2), true, false, true)
	b._putProp("setInt32", r.newNativeFunc(r.dataViewProto_setInt32, nil, "setInt32", nil, 2), true, false, true)
	b._putProp("setUint8", r.newNativeFunc(r.dataViewProto_setUint8, nil, "setUint8", nil, 2), true, false, true)
	b._putProp("setUint16", r.newNativeFunc(r.dataViewProto_setUint16, nil, "setUint16", nil, 2), true, false, true)
	b._putProp("setUint32", r.newNativeFunc(r.dataViewProto_setUint32, nil, "setUint32", nil, 2), true, false, true)
	b._putSym(SymToStringTag, valueProp(asciiString("DataView"), false, false, true))

	return b
}

func (r *Runtime) createDataView(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.newDataView, r.global.DataViewPrototype, "DataView", 1)
	return o
}

func (r *Runtime) createTypedArrayProto(val *Object) objectImpl {
	b := newBaseObjectObj(val, r.global.ObjectPrototype, classObject)
	b._put("buffer", &valueProperty{
		accessor:     true,
		configurable: true,
		getterFunc:   r.newNativeFunc(r.typedArrayProto_getBuffer, nil, "get buffer", nil, 0),
	})
	b._put("byteLength", &valueProperty{
		accessor:     true,
		configurable: true,
		getterFunc:   r.newNativeFunc(r.typedArrayProto_getByteLen, nil, "get byteLength", nil, 0),
	})
	b._put("byteOffset", &valueProperty{
		accessor:     true,
		configurable: true,
		getterFunc:   r.newNativeFunc(r.typedArrayProto_getByteOffset, nil, "get byteOffset", nil, 0),
	})
	b._putProp("constructor", r.global.TypedArray, true, false, true)
	b._putProp("copyWithin", r.newNativeFunc(r.typedArrayProto_copyWithin, nil, "copyWithin", nil, 2), true, false, true)
	b._putProp("entries", r.newNativeFunc(r.typedArrayProto_entries, nil, "entries", nil, 0), true, false, true)
	b._putProp("every", r.newNativeFunc(r.typedArrayProto_every, nil, "every", nil, 1), true, false, true)
	b._putProp("fill", r.newNativeFunc(r.typedArrayProto_fill, nil, "fill", nil, 1), true, false, true)
	b._putProp("filter", r.newNativeFunc(r.typedArrayProto_filter, nil, "filter", nil, 1), true, false, true)
	b._putProp("find", r.newNativeFunc(r.typedArrayProto_find, nil, "find", nil, 1), true, false, true)
	b._putProp("findIndex", r.newNativeFunc(r.typedArrayProto_findIndex, nil, "findIndex", nil, 1), true, false, true)
	b._putProp("forEach", r.newNativeFunc(r.typedArrayProto_forEach, nil, "forEach", nil, 1), true, false, true)
	b._putProp("includes", r.newNativeFunc(r.typedArrayProto_includes, nil, "includes", nil, 1), true, false, true)
	b._putProp("indexOf", r.newNativeFunc(r.typedArrayProto_indexOf, nil, "indexOf", nil, 1), true, false, true)
	b._putProp("join", r.newNativeFunc(r.typedArrayProto_join, nil, "join", nil, 1), true, false, true)
	b._putProp("keys", r.newNativeFunc(r.typedArrayProto_keys, nil, "keys", nil, 0), true, false, true)
	b._putProp("lastIndexOf", r.newNativeFunc(r.typedArrayProto_lastIndexOf, nil, "lastIndexOf", nil, 1), true, false, true)
	b._put("length", &valueProperty{
		accessor:     true,
		configurable: true,
		getterFunc:   r.newNativeFunc(r.typedArrayProto_getLength, nil, "get length", nil, 0),
	})
	b._putProp("map", r.newNativeFunc(r.typedArrayProto_map, nil, "map", nil, 1), true, false, true)
	b._putProp("reduce", r.newNativeFunc(r.typedArrayProto_reduce, nil, "reduce", nil, 1), true, false, true)
	b._putProp("reduceRight", r.newNativeFunc(r.typedArrayProto_reduceRight, nil, "reduceRight", nil, 1), true, false, true)
	b._putProp("reverse", r.newNativeFunc(r.typedArrayProto_reverse, nil, "reverse", nil, 0), true, false, true)
	b._putProp("set", r.newNativeFunc(r.typedArrayProto_set, nil, "set", nil, 1), true, false, true)
	b._putProp("slice", r.newNativeFunc(r.typedArrayProto_slice, nil, "slice", nil, 2), true, false, true)
	b._putProp("some", r.newNativeFunc(r.typedArrayProto_some, nil, "some", nil, 1), true, false, true)
	b._putProp("sort", r.newNativeFunc(r.typedArrayProto_sort, nil, "sort", nil, 1), true, false, true)
	b._putProp("subarray", r.newNativeFunc(r.typedArrayProto_subarray, nil, "subarray", nil, 2), true, false, true)
	b._putProp("toLocaleString", r.newNativeFunc(r.typedArrayProto_toLocaleString, nil, "toLocaleString", nil, 0), true, false, true)
	b._putProp("toString", r.global.arrayToString, true, false, true)
	valuesFunc := r.newNativeFunc(r.typedArrayProto_values, nil, "values", nil, 0)
	b._putProp("values", valuesFunc, true, false, true)
	b._putSym(SymIterator, valueProp(valuesFunc, true, false, true))
	b._putSym(SymToStringTag, &valueProperty{
		getterFunc:   r.newNativeFunc(r.typedArrayProto_toStringTag, nil, "get [Symbol.toStringTag]", nil, 0),
		accessor:     true,
		configurable: true,
	})

	return b
}

func (r *Runtime) createTypedArray(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.newTypedArray, r.global.TypedArrayPrototype, "TypedArray", 0)
	o._putProp("from", r.newNativeFunc(r.typedArray_from, nil, "from", nil, 1), true, false, true)
	o._putProp("of", r.newNativeFunc(r.typedArray_of, nil, "of", nil, 0), true, false, true)
	r.putSpeciesReturnThis(o)

	return o
}

func (r *Runtime) typedArrayCreator(ctor func(args []Value, newTarget, proto *Object) *Object, name unistring.String, bytesPerElement int) func(val *Object) objectImpl {
	return func(val *Object) objectImpl {
		p := r.newBaseObject(r.global.TypedArrayPrototype, classObject)
		o := r.newNativeConstructOnly(val, func(args []Value, newTarget *Object) *Object {
			return ctor(args, newTarget, p.val)
		}, p.val, name, 3)

		p._putProp("constructor", o.val, true, false, true)

		o.prototype = r.global.TypedArray
		bpe := intToValue(int64(bytesPerElement))
		o._putProp("BYTES_PER_ELEMENT", bpe, false, false, false)
		p._putProp("BYTES_PER_ELEMENT", bpe, false, false, false)
		return o
	}
}

func (r *Runtime) initTypedArrays() {

	r.global.ArrayBufferPrototype = r.newLazyObject(r.createArrayBufferProto)
	r.global.ArrayBuffer = r.newLazyObject(r.createArrayBuffer)
	r.addToGlobal("ArrayBuffer", r.global.ArrayBuffer)

	r.global.DataViewPrototype = r.newLazyObject(r.createDataViewProto)
	r.global.DataView = r.newLazyObject(r.createDataView)
	r.addToGlobal("DataView", r.global.DataView)

	r.global.TypedArrayPrototype = r.newLazyObject(r.createTypedArrayProto)
	r.global.TypedArray = r.newLazyObject(r.createTypedArray)

	r.global.Uint8Array = r.newLazyObject(r.typedArrayCreator(r.newUint8Array, "Uint8Array", 1))
	r.addToGlobal("Uint8Array", r.global.Uint8Array)

	r.global.Uint8ClampedArray = r.newLazyObject(r.typedArrayCreator(r.newUint8ClampedArray, "Uint8ClampedArray", 1))
	r.addToGlobal("Uint8ClampedArray", r.global.Uint8ClampedArray)

	r.global.Int8Array = r.newLazyObject(r.typedArrayCreator(r.newInt8Array, "Int8Array", 1))
	r.addToGlobal("Int8Array", r.global.Int8Array)

	r.global.Uint16Array = r.newLazyObject(r.typedArrayCreator(r.newUint16Array, "Uint16Array", 2))
	r.addToGlobal("Uint16Array", r.global.Uint16Array)

	r.global.Int16Array = r.newLazyObject(r.typedArrayCreator(r.newInt16Array, "Int16Array", 2))
	r.addToGlobal("Int16Array", r.global.Int16Array)

	r.global.Uint32Array = r.newLazyObject(r.typedArrayCreator(r.newUint32Array, "Uint32Array", 4))
	r.addToGlobal("Uint32Array", r.global.Uint32Array)

	r.global.Int32Array = r.newLazyObject(r.typedArrayCreator(r.newInt32Array, "Int32Array", 4))
	r.addToGlobal("Int32Array", r.global.Int32Array)

	r.global.Float32Array = r.newLazyObject(r.typedArrayCreator(r.newFloat32Array, "Float32Array", 4))
	r.addToGlobal("Float32Array", r.global.Float32Array)

	r.global.Float64Array = r.newLazyObject(r.typedArrayCreator(r.newFloat64Array, "Float64Array", 8))
	r.addToGlobal("Float64Array", r.global.Float64Array)
}
