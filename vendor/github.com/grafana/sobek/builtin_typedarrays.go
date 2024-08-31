package sobek

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"github.com/grafana/sobek/unistring"
)

type typedArraySortCtx struct {
	ta           *typedArrayObject
	compare      func(FunctionCall) Value
	needValidate bool
	detached     bool
}

func (ctx *typedArraySortCtx) Len() int {
	return ctx.ta.length
}

func (ctx *typedArraySortCtx) checkDetached() {
	if !ctx.detached && ctx.needValidate {
		ctx.detached = !ctx.ta.viewedArrayBuf.ensureNotDetached(false)
		ctx.needValidate = false
	}
}

func (ctx *typedArraySortCtx) Less(i, j int) bool {
	ctx.checkDetached()
	if ctx.detached {
		return false
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
	ctx.checkDetached()
	if ctx.detached {
		return
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
	b := r._newArrayBuffer(r.getPrototypeFromCtor(newTarget, r.getArrayBuffer(), r.getArrayBufferPrototype()), nil)
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
		ret := r.speciesConstructor(o, r.getArrayBuffer())([]Value{intToValue(newLen)}, nil)
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
			panic(r.newError(r.getRangeError(), "Start offset %s is outside the bounds of the buffer", offsetArg.String()))
		}
	}
	if len(args) > 2 && args[2] != nil && args[2] != _undefined {
		byteLen = r.toIndex(args[2])
		if byteOffset+byteLen > len(buffer.data) {
			panic(r.newError(r.getRangeError(), "Invalid DataView length %d", byteLen))
		}
	} else {
		byteLen = len(buffer.data) - byteOffset
	}
	proto := r.getPrototypeFromCtor(newTarget, r.getDataView(), r.getDataViewPrototype())
	buffer.ensureNotDetached(true)
	if byteOffset > len(buffer.data) {
		panic(r.newError(r.getRangeError(), "Start offset %d is outside the bounds of the buffer", byteOffset))
	}
	if byteOffset+byteLen > len(buffer.data) {
		panic(r.newError(r.getRangeError(), "Invalid DataView length %d", byteLen))
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

func (r *Runtime) dataViewProto_getBigInt64(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		return (*valueBigInt)(dv.viewedArrayBuf.getBigInt64(dv.getIdxAndByteOrder(r.toIndex(call.Argument(0).ToNumber()), call.Argument(1), 8)))
	}
	panic(r.NewTypeError("Method DataView.prototype.getBigInt64 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_getBigUint64(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		return (*valueBigInt)(dv.viewedArrayBuf.getBigUint64(dv.getIdxAndByteOrder(r.toIndex(call.Argument(0).ToNumber()), call.Argument(1), 8)))
	}
	panic(r.NewTypeError("Method DataView.prototype.getBigUint64 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
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

func (r *Runtime) dataViewProto_setBigInt64(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		idxVal := r.toIndex(call.Argument(0))
		val := toBigInt64(call.Argument(1))
		idx, bo := dv.getIdxAndByteOrder(idxVal, call.Argument(2), 8)
		dv.viewedArrayBuf.setBigInt64(idx, val, bo)
		return _undefined
	}
	panic(r.NewTypeError("Method DataView.prototype.setBigInt64 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) dataViewProto_setBigUint64(call FunctionCall) Value {
	if dv, ok := r.toObject(call.This).self.(*dataViewObject); ok {
		idxVal := r.toIndex(call.Argument(0))
		val := toBigUint64(call.Argument(1))
		idx, bo := dv.getIdxAndByteOrder(idxVal, call.Argument(2), 8)
		dv.viewedArrayBuf.setBigUint64(idx, val, bo)
		return _undefined
	}
	panic(r.NewTypeError("Method DataView.prototype.setBigUint64 called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
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
		ab := r._newArrayBuffer(r.getArrayBufferPrototype(), nil)
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

func (r *Runtime) typedArrayProto_findLast(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		predicate := r.toCallable(call.Argument(0))
		fc := FunctionCall{
			This:      call.Argument(1),
			Arguments: []Value{nil, nil, call.This},
		}
		for k := ta.length - 1; k >= 0; k-- {
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
	panic(r.NewTypeError("Method TypedArray.prototype.findLast called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
}

func (r *Runtime) typedArrayProto_findLastIndex(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		predicate := r.toCallable(call.Argument(0))
		fc := FunctionCall{
			This:      call.Argument(1),
			Arguments: []Value{nil, nil, call.This},
		}
		for k := ta.length - 1; k >= 0; k-- {
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
	panic(r.NewTypeError("Method TypedArray.prototype.findLastIndex called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
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

func (r *Runtime) typedArrayProto_at(call FunctionCall) Value {
	if ta, ok := r.toObject(call.This).self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
		idx := call.Argument(0).ToInteger()
		length := int64(ta.length)
		if idx < 0 {
			idx = length + idx
		}
		if idx >= length || idx < 0 {
			return _undefined
		}
		if ta.viewedArrayBuf.ensureNotDetached(false) {
			return ta.typedArray.get(ta.offset + int(idx))
		}
		return _undefined
	}
	panic(r.NewTypeError("Method TypedArray.prototype.at called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: call.This})))
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
		var sep String
		if s != _undefined {
			sep = s.toString()
		} else {
			sep = asciiString(",")
		}
		l := ta.length
		if l == 0 {
			return stringEmpty
		}

		var buf StringBuilder

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
			panic(r.newError(r.getRangeError(), "offset should be >= 0"))
		}
		ta.viewedArrayBuf.ensureNotDetached(true)
		targetLen := ta.length
		if src, ok := srcObj.self.(*typedArrayObject); ok {
			src.viewedArrayBuf.ensureNotDetached(true)
			srcLen := src.length
			if x := srcLen + targetOffset; x < 0 || x > targetLen {
				panic(r.newError(r.getRangeError(), "Source is too large"))
			}
			if src.defaultCtor == ta.defaultCtor {
				copy(ta.viewedArrayBuf.data[(ta.offset+targetOffset)*ta.elemSize:],
					src.viewedArrayBuf.data[src.offset*src.elemSize:(src.offset+srcLen)*src.elemSize])
			} else {
				checkTypedArrayMixBigInt(src.defaultCtor, ta.defaultCtor)
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
				panic(r.newError(r.getRangeError(), "Source is too large"))
			}
			for i := 0; i < srcLen; i++ {
				val := nilSafe(srcObj.self.getIdx(valueInt(i), nil))
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
		var buf StringBuilder
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

func (r *Runtime) typedArrayProto_with(call FunctionCall) Value {
	o := call.This.ToObject(r)
	ta, ok := o.self.(*typedArrayObject)
	if !ok {
		panic(r.NewTypeError("%s is not a valid TypedArray", r.objectproto_toString(FunctionCall{This: call.This})))
	}
	ta.viewedArrayBuf.ensureNotDetached(true)
	length := ta.length
	relativeIndex := call.Argument(0).ToInteger()
	var actualIndex int

	if relativeIndex >= 0 {
		actualIndex = toIntStrict(relativeIndex)
	} else {
		actualIndex = toIntStrict(int64(length) + relativeIndex)
	}
	if !ta.isValidIntegerIndex(actualIndex) {
		panic(r.newError(r.getRangeError(), "Invalid typed array index"))
	}

	var numericValue Value
	switch ta.typedArray.(type) {
	case *bigInt64Array, *bigUint64Array:
		numericValue = toBigInt(call.Argument(1))
	default:
		numericValue = call.Argument(1).ToNumber()
	}

	a := r.typedArrayCreate(ta.defaultCtor, intToValue(int64(length)))
	for k := 0; k < length; k++ {
		var fromValue Value
		if k == actualIndex {
			fromValue = numericValue
		} else {
			fromValue = ta.typedArray.get(ta.offset + k)
		}
		a.typedArray.set(ta.offset+k, fromValue)
	}
	return a.val
}

func (r *Runtime) typedArrayProto_toReversed(call FunctionCall) Value {
	o := call.This.ToObject(r)
	ta, ok := o.self.(*typedArrayObject)
	if !ok {
		panic(r.NewTypeError("%s is not a valid TypedArray", r.objectproto_toString(FunctionCall{This: call.This})))
	}
	ta.viewedArrayBuf.ensureNotDetached(true)
	length := ta.length

	a := r.typedArrayCreate(ta.defaultCtor, intToValue(int64(length)))

	for k := 0; k < length; k++ {
		from := length - k - 1
		fromValue := ta.typedArray.get(ta.offset + from)
		a.typedArray.set(ta.offset+k, fromValue)
	}

	return a.val
}

func (r *Runtime) typedArrayProto_toSorted(call FunctionCall) Value {
	o := call.This.ToObject(r)
	ta, ok := o.self.(*typedArrayObject)
	if !ok {
		panic(r.NewTypeError("%s is not a valid TypedArray", r.objectproto_toString(FunctionCall{This: call.This})))
	}
	ta.viewedArrayBuf.ensureNotDetached(true)

	var compareFn func(FunctionCall) Value
	arg := call.Argument(0)
	if arg != _undefined {
		if arg, ok := arg.(*Object); ok {
			compareFn, _ = arg.self.assertCallable()
		}
		if compareFn == nil {
			panic(r.NewTypeError("The comparison function must be either a function or undefined"))
		}
	}

	length := ta.length

	a := r.typedArrayCreate(ta.defaultCtor, intToValue(int64(length)))
	copy(a.viewedArrayBuf.data, ta.viewedArrayBuf.data)

	ctx := typedArraySortCtx{
		ta:      a,
		compare: compareFn,
	}

	sort.Stable(&ctx)

	return a.val
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
				ta._putIdx(idx, val)
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
	buf := r._newArrayBuffer(r.getArrayBufferPrototype(), nil)
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
			panic(r.newError(r.getRangeError(), "Start offset of %s should be a multiple of %d", newTarget.self.getStr("name", nil), ta.elemSize))
		}
	}
	var length int
	if len(args) > 2 && args[2] != nil && args[2] != _undefined {
		length = r.toIndex(args[2])
		ab.ensureNotDetached(true)
		if byteOffset+length*ta.elemSize > len(ab.data) {
			panic(r.newError(r.getRangeError(), "Invalid typed array length: %d", length))
		}
	} else {
		ab.ensureNotDetached(true)
		if len(ab.data)%ta.elemSize != 0 {
			panic(r.newError(r.getRangeError(), "Byte length of %s should be a multiple of %d", newTarget.self.getStr("name", nil), ta.elemSize))
		}
		length = (len(ab.data) - byteOffset) / ta.elemSize
		if length < 0 {
			panic(r.newError(r.getRangeError(), "Start offset %d is outside the bounds of the buffer", byteOffset))
		}
	}
	ta.offset = byteOffset / ta.elemSize
	ta.length = length
	return ta.val
}

func checkTypedArrayMixBigInt(src, dst *Object) {
	srcType := src.self.getStr("name", nil).String()
	if strings.HasPrefix(srcType, "Big") {
		if !strings.HasPrefix(dst.self.getStr("name", nil).String(), "Big") {
			panic(errMixBigIntType)
		}
	}
}

func (r *Runtime) _newTypedArrayFromTypedArray(src *typedArrayObject, newTarget *Object, taCtor typedArrayObjectCtor, proto *Object) *Object {
	dst := r.allocateTypedArray(newTarget, 0, taCtor, proto)
	src.viewedArrayBuf.ensureNotDetached(true)
	l := src.length

	dst.viewedArrayBuf.data = allocByteSlice(toIntStrict(int64(l) * int64(dst.elemSize)))
	src.viewedArrayBuf.ensureNotDetached(true)
	if src.defaultCtor == dst.defaultCtor {
		copy(dst.viewedArrayBuf.data, src.viewedArrayBuf.data[src.offset*src.elemSize:])
		dst.length = src.length
		return dst.val
	} else {
		checkTypedArrayMixBigInt(src.defaultCtor, newTarget)
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

func (r *Runtime) newBigInt64Array(args []Value, newTarget, proto *Object) *Object {
	return r._newTypedArray(args, newTarget, r.newBigInt64ArrayObject, proto)
}

func (r *Runtime) newBigUint64Array(args []Value, newTarget, proto *Object) *Object {
	return r._newTypedArray(args, newTarget, r.newBigUint64ArrayObject, proto)
}

func (r *Runtime) createArrayBufferProto(val *Object) objectImpl {
	b := newBaseObjectObj(val, r.global.ObjectPrototype, classObject)
	byteLengthProp := &valueProperty{
		accessor:     true,
		configurable: true,
		getterFunc:   r.newNativeFunc(r.arrayBufferProto_getByteLength, "get byteLength", 0),
	}
	b._put("byteLength", byteLengthProp)
	b._putProp("constructor", r.getArrayBuffer(), true, false, true)
	b._putProp("slice", r.newNativeFunc(r.arrayBufferProto_slice, "slice", 2), true, false, true)
	b._putSym(SymToStringTag, valueProp(asciiString("ArrayBuffer"), false, false, true))
	return b
}

func (r *Runtime) createArrayBuffer(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.builtin_newArrayBuffer, r.getArrayBufferPrototype(), "ArrayBuffer", 1)
	o._putProp("isView", r.newNativeFunc(r.arrayBuffer_isView, "isView", 1), true, false, true)
	r.putSpeciesReturnThis(o)

	return o
}

func (r *Runtime) createDataView(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.newDataView, r.getDataViewPrototype(), "DataView", 1)
	return o
}

func (r *Runtime) createTypedArray(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.newTypedArray, r.getTypedArrayPrototype(), "TypedArray", 0)
	o._putProp("from", r.newNativeFunc(r.typedArray_from, "from", 1), true, false, true)
	o._putProp("of", r.newNativeFunc(r.typedArray_of, "of", 0), true, false, true)
	r.putSpeciesReturnThis(o)

	return o
}

func (r *Runtime) getTypedArray() *Object {
	ret := r.global.TypedArray
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.TypedArray = ret
		r.createTypedArray(ret)
	}
	return ret
}

func (r *Runtime) createTypedArrayCtor(val *Object, ctor func(args []Value, newTarget, proto *Object) *Object, name unistring.String, bytesPerElement int) {
	p := r.newBaseObject(r.getTypedArrayPrototype(), classObject)
	o := r.newNativeConstructOnly(val, func(args []Value, newTarget *Object) *Object {
		return ctor(args, newTarget, p.val)
	}, p.val, name, 3)

	p._putProp("constructor", o.val, true, false, true)

	o.prototype = r.getTypedArray()
	bpe := intToValue(int64(bytesPerElement))
	o._putProp("BYTES_PER_ELEMENT", bpe, false, false, false)
	p._putProp("BYTES_PER_ELEMENT", bpe, false, false, false)
}

func addTypedArrays(t *objectTemplate) {
	t.putStr("ArrayBuffer", func(r *Runtime) Value { return valueProp(r.getArrayBuffer(), true, false, true) })
	t.putStr("DataView", func(r *Runtime) Value { return valueProp(r.getDataView(), true, false, true) })
	t.putStr("Uint8Array", func(r *Runtime) Value { return valueProp(r.getUint8Array(), true, false, true) })
	t.putStr("Uint8ClampedArray", func(r *Runtime) Value { return valueProp(r.getUint8ClampedArray(), true, false, true) })
	t.putStr("Int8Array", func(r *Runtime) Value { return valueProp(r.getInt8Array(), true, false, true) })
	t.putStr("Uint16Array", func(r *Runtime) Value { return valueProp(r.getUint16Array(), true, false, true) })
	t.putStr("Int16Array", func(r *Runtime) Value { return valueProp(r.getInt16Array(), true, false, true) })
	t.putStr("Uint32Array", func(r *Runtime) Value { return valueProp(r.getUint32Array(), true, false, true) })
	t.putStr("Int32Array", func(r *Runtime) Value { return valueProp(r.getInt32Array(), true, false, true) })
	t.putStr("Float32Array", func(r *Runtime) Value { return valueProp(r.getFloat32Array(), true, false, true) })
	t.putStr("Float64Array", func(r *Runtime) Value { return valueProp(r.getFloat64Array(), true, false, true) })
	t.putStr("BigInt64Array", func(r *Runtime) Value { return valueProp(r.getBigInt64Array(), true, false, true) })
	t.putStr("BigUint64Array", func(r *Runtime) Value { return valueProp(r.getBigUint64Array(), true, false, true) })
}

func createTypedArrayProtoTemplate() *objectTemplate {
	t := newObjectTemplate()
	t.protoFactory = func(r *Runtime) *Object {
		return r.global.ObjectPrototype
	}

	t.putStr("buffer", func(r *Runtime) Value {
		return &valueProperty{
			accessor:     true,
			configurable: true,
			getterFunc:   r.newNativeFunc(r.typedArrayProto_getBuffer, "get buffer", 0),
		}
	})

	t.putStr("byteLength", func(r *Runtime) Value {
		return &valueProperty{
			accessor:     true,
			configurable: true,
			getterFunc:   r.newNativeFunc(r.typedArrayProto_getByteLen, "get byteLength", 0),
		}
	})

	t.putStr("byteOffset", func(r *Runtime) Value {
		return &valueProperty{
			accessor:     true,
			configurable: true,
			getterFunc:   r.newNativeFunc(r.typedArrayProto_getByteOffset, "get byteOffset", 0),
		}
	})

	t.putStr("at", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_at, "at", 1) })
	t.putStr("constructor", func(r *Runtime) Value { return valueProp(r.getTypedArray(), true, false, true) })
	t.putStr("copyWithin", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_copyWithin, "copyWithin", 2) })
	t.putStr("entries", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_entries, "entries", 0) })
	t.putStr("every", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_every, "every", 1) })
	t.putStr("fill", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_fill, "fill", 1) })
	t.putStr("filter", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_filter, "filter", 1) })
	t.putStr("find", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_find, "find", 1) })
	t.putStr("findIndex", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_findIndex, "findIndex", 1) })
	t.putStr("findLast", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_findLast, "findLast", 1) })
	t.putStr("findLastIndex", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_findLastIndex, "findLastIndex", 1) })
	t.putStr("forEach", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_forEach, "forEach", 1) })
	t.putStr("includes", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_includes, "includes", 1) })
	t.putStr("indexOf", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_indexOf, "indexOf", 1) })
	t.putStr("join", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_join, "join", 1) })
	t.putStr("keys", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_keys, "keys", 0) })
	t.putStr("lastIndexOf", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_lastIndexOf, "lastIndexOf", 1) })
	t.putStr("length", func(r *Runtime) Value {
		return &valueProperty{
			accessor:     true,
			configurable: true,
			getterFunc:   r.newNativeFunc(r.typedArrayProto_getLength, "get length", 0),
		}
	})
	t.putStr("map", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_map, "map", 1) })
	t.putStr("reduce", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_reduce, "reduce", 1) })
	t.putStr("reduceRight", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_reduceRight, "reduceRight", 1) })
	t.putStr("reverse", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_reverse, "reverse", 0) })
	t.putStr("set", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_set, "set", 1) })
	t.putStr("slice", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_slice, "slice", 2) })
	t.putStr("some", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_some, "some", 1) })
	t.putStr("sort", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_sort, "sort", 1) })
	t.putStr("subarray", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_subarray, "subarray", 2) })
	t.putStr("toLocaleString", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_toLocaleString, "toLocaleString", 0) })
	t.putStr("with", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_with, "with", 2) })
	t.putStr("toReversed", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_toReversed, "toReversed", 0) })
	t.putStr("toSorted", func(r *Runtime) Value { return r.methodProp(r.typedArrayProto_toSorted, "toSorted", 1) })
	t.putStr("toString", func(r *Runtime) Value { return valueProp(r.getArrayToString(), true, false, true) })
	t.putStr("values", func(r *Runtime) Value { return valueProp(r.getTypedArrayValues(), true, false, true) })

	t.putSym(SymIterator, func(r *Runtime) Value { return valueProp(r.getTypedArrayValues(), true, false, true) })
	t.putSym(SymToStringTag, func(r *Runtime) Value {
		return &valueProperty{
			getterFunc:   r.newNativeFunc(r.typedArrayProto_toStringTag, "get [Symbol.toStringTag]", 0),
			accessor:     true,
			configurable: true,
		}
	})

	return t
}

func (r *Runtime) getTypedArrayValues() *Object {
	ret := r.global.typedArrayValues
	if ret == nil {
		ret = r.newNativeFunc(r.typedArrayProto_values, "values", 0)
		r.global.typedArrayValues = ret
	}
	return ret
}

var typedArrayProtoTemplate *objectTemplate
var typedArrayProtoTemplateOnce sync.Once

func getTypedArrayProtoTemplate() *objectTemplate {
	typedArrayProtoTemplateOnce.Do(func() {
		typedArrayProtoTemplate = createTypedArrayProtoTemplate()
	})
	return typedArrayProtoTemplate
}

func (r *Runtime) getTypedArrayPrototype() *Object {
	ret := r.global.TypedArrayPrototype
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.TypedArrayPrototype = ret
		r.newTemplatedObject(getTypedArrayProtoTemplate(), ret)
	}
	return ret
}

func (r *Runtime) getUint8Array() *Object {
	ret := r.global.Uint8Array
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Uint8Array = ret
		r.createTypedArrayCtor(ret, r.newUint8Array, "Uint8Array", 1)
	}
	return ret
}

func (r *Runtime) getUint8ClampedArray() *Object {
	ret := r.global.Uint8ClampedArray
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Uint8ClampedArray = ret
		r.createTypedArrayCtor(ret, r.newUint8ClampedArray, "Uint8ClampedArray", 1)
	}
	return ret
}

func (r *Runtime) getInt8Array() *Object {
	ret := r.global.Int8Array
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Int8Array = ret
		r.createTypedArrayCtor(ret, r.newInt8Array, "Int8Array", 1)
	}
	return ret
}

func (r *Runtime) getUint16Array() *Object {
	ret := r.global.Uint16Array
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Uint16Array = ret
		r.createTypedArrayCtor(ret, r.newUint16Array, "Uint16Array", 2)
	}
	return ret
}

func (r *Runtime) getInt16Array() *Object {
	ret := r.global.Int16Array
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Int16Array = ret
		r.createTypedArrayCtor(ret, r.newInt16Array, "Int16Array", 2)
	}
	return ret
}

func (r *Runtime) getUint32Array() *Object {
	ret := r.global.Uint32Array
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Uint32Array = ret
		r.createTypedArrayCtor(ret, r.newUint32Array, "Uint32Array", 4)
	}
	return ret
}

func (r *Runtime) getInt32Array() *Object {
	ret := r.global.Int32Array
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Int32Array = ret
		r.createTypedArrayCtor(ret, r.newInt32Array, "Int32Array", 4)
	}
	return ret
}

func (r *Runtime) getFloat32Array() *Object {
	ret := r.global.Float32Array
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Float32Array = ret
		r.createTypedArrayCtor(ret, r.newFloat32Array, "Float32Array", 4)
	}
	return ret
}

func (r *Runtime) getFloat64Array() *Object {
	ret := r.global.Float64Array
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Float64Array = ret
		r.createTypedArrayCtor(ret, r.newFloat64Array, "Float64Array", 8)
	}
	return ret
}

func (r *Runtime) getBigInt64Array() *Object {
	ret := r.global.BigInt64Array
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.BigInt64Array = ret
		r.createTypedArrayCtor(ret, r.newBigInt64Array, "BigInt64Array", 8)
	}
	return ret
}

func (r *Runtime) getBigUint64Array() *Object {
	ret := r.global.BigUint64Array
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.BigUint64Array = ret
		r.createTypedArrayCtor(ret, r.newBigUint64Array, "BigUint64Array", 8)
	}
	return ret
}

func createDataViewProtoTemplate() *objectTemplate {
	t := newObjectTemplate()
	t.protoFactory = func(r *Runtime) *Object {
		return r.global.ObjectPrototype
	}

	t.putStr("buffer", func(r *Runtime) Value {
		return &valueProperty{
			accessor:     true,
			configurable: true,
			getterFunc:   r.newNativeFunc(r.dataViewProto_getBuffer, "get buffer", 0),
		}
	})
	t.putStr("byteLength", func(r *Runtime) Value {
		return &valueProperty{
			accessor:     true,
			configurable: true,
			getterFunc:   r.newNativeFunc(r.dataViewProto_getByteLen, "get byteLength", 0),
		}
	})
	t.putStr("byteOffset", func(r *Runtime) Value {
		return &valueProperty{
			accessor:     true,
			configurable: true,
			getterFunc:   r.newNativeFunc(r.dataViewProto_getByteOffset, "get byteOffset", 0),
		}
	})

	t.putStr("constructor", func(r *Runtime) Value { return valueProp(r.getDataView(), true, false, true) })

	t.putStr("getFloat32", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_getFloat32, "getFloat32", 1) })
	t.putStr("getFloat64", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_getFloat64, "getFloat64", 1) })
	t.putStr("getInt8", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_getInt8, "getInt8", 1) })
	t.putStr("getInt16", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_getInt16, "getInt16", 1) })
	t.putStr("getInt32", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_getInt32, "getInt32", 1) })
	t.putStr("getUint8", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_getUint8, "getUint8", 1) })
	t.putStr("getUint16", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_getUint16, "getUint16", 1) })
	t.putStr("getUint32", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_getUint32, "getUint32", 1) })
	t.putStr("getBigInt64", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_getBigInt64, "getBigInt64", 1) })
	t.putStr("getBigUint64", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_getBigUint64, "getBigUint64", 1) })
	t.putStr("setFloat32", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_setFloat32, "setFloat32", 2) })
	t.putStr("setFloat64", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_setFloat64, "setFloat64", 2) })
	t.putStr("setInt8", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_setInt8, "setInt8", 2) })
	t.putStr("setInt16", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_setInt16, "setInt16", 2) })
	t.putStr("setInt32", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_setInt32, "setInt32", 2) })
	t.putStr("setUint8", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_setUint8, "setUint8", 2) })
	t.putStr("setUint16", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_setUint16, "setUint16", 2) })
	t.putStr("setUint32", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_setUint32, "setUint32", 2) })
	t.putStr("setBigInt64", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_setBigInt64, "setBigInt64", 2) })
	t.putStr("setBigUint64", func(r *Runtime) Value { return r.methodProp(r.dataViewProto_setBigUint64, "setBigUint64", 2) })

	t.putSym(SymToStringTag, func(r *Runtime) Value { return valueProp(asciiString("DataView"), false, false, true) })

	return t
}

var dataViewProtoTemplate *objectTemplate
var dataViewProtoTemplateOnce sync.Once

func getDataViewProtoTemplate() *objectTemplate {
	dataViewProtoTemplateOnce.Do(func() {
		dataViewProtoTemplate = createDataViewProtoTemplate()
	})
	return dataViewProtoTemplate
}

func (r *Runtime) getDataViewPrototype() *Object {
	ret := r.global.DataViewPrototype
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.DataViewPrototype = ret
		r.newTemplatedObject(getDataViewProtoTemplate(), ret)
	}
	return ret
}

func (r *Runtime) getDataView() *Object {
	ret := r.global.DataView
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.DataView = ret
		ret.self = r.createDataView(ret)
	}
	return ret
}

func (r *Runtime) getArrayBufferPrototype() *Object {
	ret := r.global.ArrayBufferPrototype
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.ArrayBufferPrototype = ret
		ret.self = r.createArrayBufferProto(ret)
	}
	return ret
}

func (r *Runtime) getArrayBuffer() *Object {
	ret := r.global.ArrayBuffer
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.ArrayBuffer = ret
		ret.self = r.createArrayBuffer(ret)
	}
	return ret
}
