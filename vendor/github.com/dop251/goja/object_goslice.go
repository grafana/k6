package goja

import (
	"math"
	"math/bits"
	"reflect"
	"strconv"

	"github.com/dop251/goja/unistring"
)

type objectGoSlice struct {
	baseObject
	data       *[]interface{}
	lengthProp valueProperty
	origIsPtr  bool
}

func (r *Runtime) newObjectGoSlice(data *[]interface{}, isPtr bool) *objectGoSlice {
	obj := &Object{runtime: r}
	a := &objectGoSlice{
		baseObject: baseObject{
			val: obj,
		},
		data:      data,
		origIsPtr: isPtr,
	}
	obj.self = a
	a.init()

	return a
}

func (o *objectGoSlice) init() {
	o.baseObject.init()
	o.class = classArray
	o.prototype = o.val.runtime.getArrayPrototype()
	o.lengthProp.writable = true
	o.extensible = true
	o.baseObject._put("length", &o.lengthProp)
}

func (o *objectGoSlice) updateLen() {
	o.lengthProp.value = intToValue(int64(len(*o.data)))
}

func (o *objectGoSlice) _getIdx(idx int) Value {
	return o.val.runtime.ToValue((*o.data)[idx])
}

func (o *objectGoSlice) getStr(name unistring.String, receiver Value) Value {
	var ownProp Value
	if idx := strToGoIdx(name); idx >= 0 && idx < len(*o.data) {
		ownProp = o._getIdx(idx)
	} else if name == "length" {
		o.updateLen()
		ownProp = &o.lengthProp
	}

	return o.getStrWithOwnProp(ownProp, name, receiver)
}

func (o *objectGoSlice) getIdx(idx valueInt, receiver Value) Value {
	if idx := int64(idx); idx >= 0 && idx < int64(len(*o.data)) {
		return o._getIdx(int(idx))
	}
	if o.prototype != nil {
		if receiver == nil {
			return o.prototype.self.getIdx(idx, o.val)
		}
		return o.prototype.self.getIdx(idx, receiver)
	}
	return nil
}

func (o *objectGoSlice) getOwnPropStr(name unistring.String) Value {
	if idx := strToGoIdx(name); idx >= 0 {
		if idx < len(*o.data) {
			return &valueProperty{
				value:      o._getIdx(idx),
				writable:   true,
				enumerable: true,
			}
		}
		return nil
	}
	if name == "length" {
		o.updateLen()
		return &o.lengthProp
	}
	return nil
}

func (o *objectGoSlice) getOwnPropIdx(idx valueInt) Value {
	if idx := int64(idx); idx >= 0 && idx < int64(len(*o.data)) {
		return &valueProperty{
			value:      o._getIdx(int(idx)),
			writable:   true,
			enumerable: true,
		}
	}
	return nil
}

func (o *objectGoSlice) grow(size int) {
	oldcap := cap(*o.data)
	if oldcap < size {
		n := make([]interface{}, size, growCap(size, len(*o.data), oldcap))
		copy(n, *o.data)
		*o.data = n
	} else {
		tail := (*o.data)[len(*o.data):size]
		for k := range tail {
			tail[k] = nil
		}
		*o.data = (*o.data)[:size]
	}
}

func (o *objectGoSlice) shrink(size int) {
	tail := (*o.data)[size:]
	for k := range tail {
		tail[k] = nil
	}
	*o.data = (*o.data)[:size]
}

func (o *objectGoSlice) putIdx(idx int, v Value, throw bool) {
	if idx >= len(*o.data) {
		o.grow(idx + 1)
	}
	(*o.data)[idx] = v.Export()
}

func (o *objectGoSlice) putLength(v uint32, throw bool) bool {
	if bits.UintSize == 32 && v > math.MaxInt32 {
		panic(rangeError("Integer value overflows 32-bit int"))
	}
	newLen := int(v)
	curLen := len(*o.data)
	if newLen > curLen {
		o.grow(newLen)
	} else if newLen < curLen {
		o.shrink(newLen)
	}
	return true
}

func (o *objectGoSlice) setOwnIdx(idx valueInt, val Value, throw bool) bool {
	if i := toIntStrict(int64(idx)); i >= 0 {
		if i >= len(*o.data) {
			if res, ok := o._setForeignIdx(idx, nil, val, o.val, throw); ok {
				return res
			}
		}
		o.putIdx(i, val, throw)
	} else {
		name := idx.string()
		if res, ok := o._setForeignStr(name, nil, val, o.val, throw); !ok {
			o.val.runtime.typeErrorResult(throw, "Can't set property '%s' on Go slice", name)
			return false
		} else {
			return res
		}
	}
	return true
}

func (o *objectGoSlice) setOwnStr(name unistring.String, val Value, throw bool) bool {
	if idx := strToGoIdx(name); idx >= 0 {
		if idx >= len(*o.data) {
			if res, ok := o._setForeignStr(name, nil, val, o.val, throw); ok {
				return res
			}
		}
		o.putIdx(idx, val, throw)
	} else {
		if name == "length" {
			return o.putLength(o.val.runtime.toLengthUint32(val), throw)
		}
		if res, ok := o._setForeignStr(name, nil, val, o.val, throw); !ok {
			o.val.runtime.typeErrorResult(throw, "Can't set property '%s' on Go slice", name)
			return false
		} else {
			return res
		}
	}
	return true
}

func (o *objectGoSlice) setForeignIdx(idx valueInt, val, receiver Value, throw bool) (bool, bool) {
	return o._setForeignIdx(idx, trueValIfPresent(o.hasOwnPropertyIdx(idx)), val, receiver, throw)
}

func (o *objectGoSlice) setForeignStr(name unistring.String, val, receiver Value, throw bool) (bool, bool) {
	return o._setForeignStr(name, trueValIfPresent(o.hasOwnPropertyStr(name)), val, receiver, throw)
}

func (o *objectGoSlice) hasOwnPropertyIdx(idx valueInt) bool {
	if idx := int64(idx); idx >= 0 {
		return idx < int64(len(*o.data))
	}
	return false
}

func (o *objectGoSlice) hasOwnPropertyStr(name unistring.String) bool {
	if idx := strToIdx64(name); idx >= 0 {
		return idx < int64(len(*o.data))
	}
	return name == "length"
}

func (o *objectGoSlice) defineOwnPropertyIdx(idx valueInt, descr PropertyDescriptor, throw bool) bool {
	if i := toIntStrict(int64(idx)); i >= 0 {
		if !o.val.runtime.checkHostObjectPropertyDescr(idx.string(), descr, throw) {
			return false
		}
		val := descr.Value
		if val == nil {
			val = _undefined
		}
		o.putIdx(i, val, throw)
		return true
	}
	o.val.runtime.typeErrorResult(throw, "Cannot define property '%d' on a Go slice", idx)
	return false
}

func (o *objectGoSlice) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	if idx := strToGoIdx(name); idx >= 0 {
		if !o.val.runtime.checkHostObjectPropertyDescr(name, descr, throw) {
			return false
		}
		val := descr.Value
		if val == nil {
			val = _undefined
		}
		o.putIdx(idx, val, throw)
		return true
	}
	if name == "length" {
		return o.val.runtime.defineArrayLength(&o.lengthProp, descr, o.putLength, throw)
	}
	o.val.runtime.typeErrorResult(throw, "Cannot define property '%s' on a Go slice", name)
	return false
}

func (o *objectGoSlice) _deleteIdx(idx int64) {
	if idx < int64(len(*o.data)) {
		(*o.data)[idx] = nil
	}
}

func (o *objectGoSlice) deleteStr(name unistring.String, throw bool) bool {
	if idx := strToIdx64(name); idx >= 0 {
		o._deleteIdx(idx)
		return true
	}
	return o.baseObject.deleteStr(name, throw)
}

func (o *objectGoSlice) deleteIdx(i valueInt, throw bool) bool {
	idx := int64(i)
	if idx >= 0 {
		o._deleteIdx(idx)
	}
	return true
}

type goslicePropIter struct {
	o          *objectGoSlice
	idx, limit int
}

func (i *goslicePropIter) next() (propIterItem, iterNextFunc) {
	if i.idx < i.limit && i.idx < len(*i.o.data) {
		name := strconv.Itoa(i.idx)
		i.idx++
		return propIterItem{name: newStringValue(name), enumerable: _ENUM_TRUE}, i.next
	}

	return propIterItem{}, nil
}

func (o *objectGoSlice) iterateStringKeys() iterNextFunc {
	return (&goslicePropIter{
		o:     o,
		limit: len(*o.data),
	}).next
}

func (o *objectGoSlice) stringKeys(_ bool, accum []Value) []Value {
	for i := range *o.data {
		accum = append(accum, asciiString(strconv.Itoa(i)))
	}

	return accum
}

func (o *objectGoSlice) export(*objectExportCtx) interface{} {
	if o.origIsPtr {
		return o.data
	}
	return *o.data
}

func (o *objectGoSlice) exportType() reflect.Type {
	if o.origIsPtr {
		return reflectTypeArrayPtr
	}
	return reflectTypeArray
}

func (o *objectGoSlice) equal(other objectImpl) bool {
	if other, ok := other.(*objectGoSlice); ok {
		return o.data == other.data
	}
	return false
}

func (o *objectGoSlice) esValue() Value {
	return o.val
}

func (o *objectGoSlice) reflectValue() reflect.Value {
	return reflect.ValueOf(o.data).Elem()
}

func (o *objectGoSlice) setReflectValue(value reflect.Value) {
	o.data = value.Addr().Interface().(*[]interface{})
}

func (o *objectGoSlice) sortLen() int {
	return len(*o.data)
}

func (o *objectGoSlice) sortGet(i int) Value {
	return o.getIdx(valueInt(i), nil)
}

func (o *objectGoSlice) swap(i int, j int) {
	(*o.data)[i], (*o.data)[j] = (*o.data)[j], (*o.data)[i]
}
