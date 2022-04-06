package goja

import (
	"reflect"
	"strconv"

	"github.com/dop251/goja/unistring"
)

type objectGoArrayReflect struct {
	objectGoReflect
	lengthProp valueProperty

	putIdx func(idx int, v Value, throw bool) bool
}

func (o *objectGoArrayReflect) _init() {
	o.objectGoReflect.init()
	o.class = classArray
	o.prototype = o.val.runtime.global.ArrayPrototype
	o.updateLen()
	o.baseObject._put("length", &o.lengthProp)
}

func (o *objectGoArrayReflect) init() {
	o._init()
	o.putIdx = o._putIdx
}

func (o *objectGoArrayReflect) updateLen() {
	o.lengthProp.value = intToValue(int64(o.value.Len()))
}

func (o *objectGoArrayReflect) _hasIdx(idx valueInt) bool {
	if idx := int64(idx); idx >= 0 && idx < int64(o.value.Len()) {
		return true
	}
	return false
}

func (o *objectGoArrayReflect) _hasStr(name unistring.String) bool {
	if idx := strToIdx64(name); idx >= 0 && idx < int64(o.value.Len()) {
		return true
	}
	return false
}

func (o *objectGoArrayReflect) _getIdx(idx int) Value {
	v := o.value.Index(idx)
	if (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) && v.IsNil() {
		return _null
	}
	return o.val.runtime.toValue(v.Interface(), v)
}

func (o *objectGoArrayReflect) getIdx(idx valueInt, receiver Value) Value {
	if idx := toIntStrict(int64(idx)); idx >= 0 && idx < o.value.Len() {
		return o._getIdx(idx)
	}
	return o.objectGoReflect.getStr(idx.string(), receiver)
}

func (o *objectGoArrayReflect) getStr(name unistring.String, receiver Value) Value {
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

func (o *objectGoArrayReflect) getOwnPropStr(name unistring.String) Value {
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

func (o *objectGoArrayReflect) getOwnPropIdx(idx valueInt) Value {
	if idx := toIntStrict(int64(idx)); idx >= 0 && idx < o.value.Len() {
		return &valueProperty{
			value:      o._getIdx(idx),
			writable:   true,
			enumerable: true,
		}
	}
	return nil
}

func (o *objectGoArrayReflect) _putIdx(idx int, v Value, throw bool) bool {
	err := o.val.runtime.toReflectValue(v, o.value.Index(idx), &objectExportCtx{})
	if err != nil {
		o.val.runtime.typeErrorResult(throw, "Go type conversion error: %v", err)
		return false
	}
	return true
}

func (o *objectGoArrayReflect) setOwnIdx(idx valueInt, val Value, throw bool) bool {
	if i := toIntStrict(int64(idx)); i >= 0 {
		if i >= o.value.Len() {
			if res, ok := o._setForeignIdx(idx, nil, val, o.val, throw); ok {
				return res
			}
		}
		return o.putIdx(i, val, throw)
	} else {
		name := idx.string()
		if res, ok := o._setForeignStr(name, nil, val, o.val, throw); !ok {
			o.val.runtime.typeErrorResult(throw, "Can't set property '%s' on Go slice", name)
			return false
		} else {
			return res
		}
	}
}

func (o *objectGoArrayReflect) setOwnStr(name unistring.String, val Value, throw bool) bool {
	if idx := strToGoIdx(name); idx >= 0 {
		if idx >= o.value.Len() {
			if res, ok := o._setForeignStr(name, nil, val, o.val, throw); ok {
				return res
			}
		}
		return o.putIdx(idx, val, throw)
	} else {
		if res, ok := o._setForeignStr(name, nil, val, o.val, throw); !ok {
			o.val.runtime.typeErrorResult(throw, "Can't set property '%s' on Go slice", name)
			return false
		} else {
			return res
		}
	}
}

func (o *objectGoArrayReflect) setForeignIdx(idx valueInt, val, receiver Value, throw bool) (bool, bool) {
	return o._setForeignIdx(idx, trueValIfPresent(o._hasIdx(idx)), val, receiver, throw)
}

func (o *objectGoArrayReflect) setForeignStr(name unistring.String, val, receiver Value, throw bool) (bool, bool) {
	return o._setForeignStr(name, trueValIfPresent(o.hasOwnPropertyStr(name)), val, receiver, throw)
}

func (o *objectGoArrayReflect) hasOwnPropertyIdx(idx valueInt) bool {
	return o._hasIdx(idx)
}

func (o *objectGoArrayReflect) hasOwnPropertyStr(name unistring.String) bool {
	if o._hasStr(name) || name == "length" {
		return true
	}
	return o.objectGoReflect._has(name.String())
}

func (o *objectGoArrayReflect) defineOwnPropertyIdx(idx valueInt, descr PropertyDescriptor, throw bool) bool {
	if i := toIntStrict(int64(idx)); i >= 0 {
		if !o.val.runtime.checkHostObjectPropertyDescr(idx.string(), descr, throw) {
			return false
		}
		val := descr.Value
		if val == nil {
			val = _undefined
		}
		return o.putIdx(i, val, throw)
	}
	o.val.runtime.typeErrorResult(throw, "Cannot define property '%d' on a Go slice", idx)
	return false
}

func (o *objectGoArrayReflect) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	if idx := strToGoIdx(name); idx >= 0 {
		if !o.val.runtime.checkHostObjectPropertyDescr(name, descr, throw) {
			return false
		}
		val := descr.Value
		if val == nil {
			val = _undefined
		}
		return o.putIdx(idx, val, throw)
	}
	o.val.runtime.typeErrorResult(throw, "Cannot define property '%s' on a Go slice", name)
	return false
}

func (o *objectGoArrayReflect) toPrimitiveNumber() Value {
	return o.toPrimitiveString()
}

func (o *objectGoArrayReflect) toPrimitiveString() Value {
	return o.val.runtime.arrayproto_join(FunctionCall{
		This: o.val,
	})
}

func (o *objectGoArrayReflect) toPrimitive() Value {
	return o.toPrimitiveString()
}

func (o *objectGoArrayReflect) _deleteIdx(idx int) {
	if idx < o.value.Len() {
		o.value.Index(idx).Set(reflect.Zero(o.value.Type().Elem()))
	}
}

func (o *objectGoArrayReflect) deleteStr(name unistring.String, throw bool) bool {
	if idx := strToGoIdx(name); idx >= 0 {
		o._deleteIdx(idx)
		return true
	}

	return o.objectGoReflect.deleteStr(name, throw)
}

func (o *objectGoArrayReflect) deleteIdx(i valueInt, throw bool) bool {
	idx := toIntStrict(int64(i))
	if idx >= 0 {
		o._deleteIdx(idx)
	}
	return true
}

type goArrayReflectPropIter struct {
	o          *objectGoArrayReflect
	idx, limit int
}

func (i *goArrayReflectPropIter) next() (propIterItem, iterNextFunc) {
	if i.idx < i.limit && i.idx < i.o.value.Len() {
		name := strconv.Itoa(i.idx)
		i.idx++
		return propIterItem{name: asciiString(name), enumerable: _ENUM_TRUE}, i.next
	}

	return i.o.objectGoReflect.iterateStringKeys()()
}

func (o *objectGoArrayReflect) stringKeys(all bool, accum []Value) []Value {
	for i := 0; i < o.value.Len(); i++ {
		accum = append(accum, asciiString(strconv.Itoa(i)))
	}

	return o.objectGoReflect.stringKeys(all, accum)
}

func (o *objectGoArrayReflect) iterateStringKeys() iterNextFunc {
	return (&goArrayReflectPropIter{
		o:     o,
		limit: o.value.Len(),
	}).next
}

func (o *objectGoArrayReflect) equal(other objectImpl) bool {
	if other, ok := other.(*objectGoArrayReflect); ok {
		return o.value.Interface() == other.value.Interface()
	}
	return false
}

func (o *objectGoArrayReflect) sortLen() int64 {
	return int64(o.value.Len())
}

func (o *objectGoArrayReflect) sortGet(i int64) Value {
	return o.getIdx(valueInt(i), nil)
}

func (o *objectGoArrayReflect) swap(i, j int64) {
	ii := toIntStrict(i)
	jj := toIntStrict(j)
	x := o._getIdx(ii)
	y := o._getIdx(jj)

	o._putIdx(ii, y, false)
	o._putIdx(jj, x, false)
}
