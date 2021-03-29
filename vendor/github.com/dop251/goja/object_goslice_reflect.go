package goja

import (
	"reflect"
	"strconv"

	"github.com/dop251/goja/unistring"
)

type objectGoSliceReflect struct {
	objectGoReflect
	lengthProp valueProperty
}

func (o *objectGoSliceReflect) init() {
	o.objectGoReflect.init()
	o.class = classArray
	o.prototype = o.val.runtime.global.ArrayPrototype
	if !o.value.CanSet() {
		value := reflect.Indirect(reflect.New(o.value.Type()))
		value.Set(o.value)
		o.value = value
	}
	o.lengthProp.writable = true
	o.updateLen()
	o.baseObject._put("length", &o.lengthProp)
}

func (o *objectGoSliceReflect) updateLen() {
	o.lengthProp.value = intToValue(int64(o.value.Len()))
}

func (o *objectGoSliceReflect) _hasIdx(idx valueInt) bool {
	if idx := int64(idx); idx >= 0 && idx < int64(o.value.Len()) {
		return true
	}
	return false
}

func (o *objectGoSliceReflect) _hasStr(name unistring.String) bool {
	if idx := strToIdx64(name); idx >= 0 && idx < int64(o.value.Len()) {
		return true
	}
	return false
}

func (o *objectGoSliceReflect) _getIdx(idx int) Value {
	v := o.value.Index(idx)
	if (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) && v.IsNil() {
		return _null
	}
	return o.val.runtime.ToValue(v.Interface())
}

func (o *objectGoSliceReflect) getIdx(idx valueInt, receiver Value) Value {
	if idx := toIntStrict(int64(idx)); idx >= 0 && idx < o.value.Len() {
		return o._getIdx(idx)
	}
	return o.objectGoReflect.getStr(idx.string(), receiver)
}

func (o *objectGoSliceReflect) getStr(name unistring.String, receiver Value) Value {
	var ownProp Value
	if idx := strToGoIdx(name); idx >= 0 && idx < o.value.Len() {
		ownProp = o._getIdx(idx)
	} else if name == "length" {
		ownProp = &o.lengthProp
	} else {
		ownProp = o.objectGoReflect.getOwnPropStr(name)
	}
	return o.getStrWithOwnProp(ownProp, name, receiver)
}

func (o *objectGoSliceReflect) getOwnPropStr(name unistring.String) Value {
	if idx := strToGoIdx(name); idx >= 0 {
		if idx < o.value.Len() {
			return &valueProperty{
				value:      o._getIdx(idx),
				writable:   true,
				enumerable: true,
			}
		}
		return nil
	}
	if name == "length" {
		return &o.lengthProp
	}
	return o.objectGoReflect.getOwnPropStr(name)
}

func (o *objectGoSliceReflect) getOwnPropIdx(idx valueInt) Value {
	if idx := toIntStrict(int64(idx)); idx >= 0 && idx < o.value.Len() {
		return &valueProperty{
			value:      o._getIdx(idx),
			writable:   true,
			enumerable: true,
		}
	}
	return nil
}

func (o *objectGoSliceReflect) putIdx(idx int, v Value, throw bool) bool {
	if idx >= o.value.Len() {
		o.grow(idx + 1)
	}
	err := o.val.runtime.toReflectValue(v, o.value.Index(idx), &objectExportCtx{})
	if err != nil {
		o.val.runtime.typeErrorResult(throw, "Go type conversion error: %v", err)
		return false
	}
	return true
}

func (o *objectGoSliceReflect) grow(size int) {
	oldcap := o.value.Cap()
	if oldcap < size {
		n := reflect.MakeSlice(o.value.Type(), size, growCap(size, o.value.Len(), oldcap))
		reflect.Copy(n, o.value)
		o.value.Set(n)
	} else {
		tail := o.value.Slice(o.value.Len(), size)
		zero := reflect.Zero(o.value.Type().Elem())
		for i := 0; i < tail.Len(); i++ {
			tail.Index(i).Set(zero)
		}
		o.value.SetLen(size)
	}
	o.updateLen()
}

func (o *objectGoSliceReflect) shrink(size int) {
	tail := o.value.Slice(size, o.value.Len())
	zero := reflect.Zero(o.value.Type().Elem())
	for i := 0; i < tail.Len(); i++ {
		tail.Index(i).Set(zero)
	}
	o.value.SetLen(size)
	o.updateLen()
}

func (o *objectGoSliceReflect) putLength(v Value, throw bool) bool {
	newLen := toIntStrict(toLength(v))
	curLen := o.value.Len()
	if newLen > curLen {
		o.grow(newLen)
	} else if newLen < curLen {
		o.shrink(newLen)
	}
	return true
}

func (o *objectGoSliceReflect) setOwnIdx(idx valueInt, val Value, throw bool) bool {
	if i := toIntStrict(int64(idx)); i >= 0 {
		if i >= o.value.Len() {
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

func (o *objectGoSliceReflect) setOwnStr(name unistring.String, val Value, throw bool) bool {
	if idx := strToGoIdx(name); idx >= 0 {
		if idx >= o.value.Len() {
			if res, ok := o._setForeignStr(name, nil, val, o.val, throw); ok {
				return res
			}
		}
		o.putIdx(idx, val, throw)
	} else {
		if name == "length" {
			return o.putLength(val, throw)
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

func (o *objectGoSliceReflect) setForeignIdx(idx valueInt, val, receiver Value, throw bool) (bool, bool) {
	return o._setForeignIdx(idx, trueValIfPresent(o._hasIdx(idx)), val, receiver, throw)
}

func (o *objectGoSliceReflect) setForeignStr(name unistring.String, val, receiver Value, throw bool) (bool, bool) {
	return o._setForeignStr(name, trueValIfPresent(o._hasStr(name)), val, receiver, throw)
}

func (o *objectGoSliceReflect) hasOwnPropertyIdx(idx valueInt) bool {
	return o._hasIdx(idx)
}

func (o *objectGoSliceReflect) hasOwnPropertyStr(name unistring.String) bool {
	if o._hasStr(name) {
		return true
	}
	return o.objectGoReflect._has(name.String())
}

func (o *objectGoSliceReflect) defineOwnPropertyIdx(idx valueInt, descr PropertyDescriptor, throw bool) bool {
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

func (o *objectGoSliceReflect) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
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
	o.val.runtime.typeErrorResult(throw, "Cannot define property '%s' on a Go slice", name)
	return false
}

func (o *objectGoSliceReflect) toPrimitiveNumber() Value {
	return o.toPrimitiveString()
}

func (o *objectGoSliceReflect) toPrimitiveString() Value {
	return o.val.runtime.arrayproto_join(FunctionCall{
		This: o.val,
	})
}

func (o *objectGoSliceReflect) toPrimitive() Value {
	return o.toPrimitiveString()
}

func (o *objectGoSliceReflect) _deleteIdx(idx int) {
	if idx < o.value.Len() {
		o.value.Index(idx).Set(reflect.Zero(o.value.Type().Elem()))
	}
}

func (o *objectGoSliceReflect) deleteStr(name unistring.String, throw bool) bool {
	if idx := strToGoIdx(name); idx >= 0 {
		o._deleteIdx(idx)
		return true
	}

	return o.objectGoReflect.deleteStr(name, throw)
}

func (o *objectGoSliceReflect) deleteIdx(i valueInt, throw bool) bool {
	idx := toIntStrict(int64(i))
	if idx >= 0 {
		o._deleteIdx(idx)
	}
	return true
}

type gosliceReflectPropIter struct {
	o          *objectGoSliceReflect
	idx, limit int
}

func (i *gosliceReflectPropIter) next() (propIterItem, iterNextFunc) {
	if i.idx < i.limit && i.idx < i.o.value.Len() {
		name := strconv.Itoa(i.idx)
		i.idx++
		return propIterItem{name: unistring.String(name), enumerable: _ENUM_TRUE}, i.next
	}

	return i.o.objectGoReflect.enumerateOwnKeys()()
}

func (o *objectGoSliceReflect) ownKeys(all bool, accum []Value) []Value {
	for i := 0; i < o.value.Len(); i++ {
		accum = append(accum, asciiString(strconv.Itoa(i)))
	}

	return o.objectGoReflect.ownKeys(all, accum)
}

func (o *objectGoSliceReflect) enumerateOwnKeys() iterNextFunc {
	return (&gosliceReflectPropIter{
		o:     o,
		limit: o.value.Len(),
	}).next
}

func (o *objectGoSliceReflect) equal(other objectImpl) bool {
	if other, ok := other.(*objectGoSliceReflect); ok {
		return o.value.Interface() == other.value.Interface()
	}
	return false
}

func (o *objectGoSliceReflect) sortLen() int64 {
	return int64(o.value.Len())
}

func (o *objectGoSliceReflect) sortGet(i int64) Value {
	return o.getIdx(valueInt(i), nil)
}

func (o *objectGoSliceReflect) swap(i, j int64) {
	ii := valueInt(i)
	jj := valueInt(j)
	x := o.getIdx(ii, nil)
	y := o.getIdx(jj, nil)

	o.setOwnIdx(ii, y, false)
	o.setOwnIdx(jj, x, false)
}
