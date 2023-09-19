package goja

import (
	"reflect"
	"strconv"

	"github.com/dop251/goja/unistring"
)

type objectGoArrayReflect struct {
	objectGoReflect
	lengthProp valueProperty

	valueCache valueArrayCache

	putIdx func(idx int, v Value, throw bool) bool
}

type valueArrayCache []reflectValueWrapper

func (c *valueArrayCache) get(idx int) reflectValueWrapper {
	if idx < len(*c) {
		return (*c)[idx]
	}
	return nil
}

func (c *valueArrayCache) grow(newlen int) {
	oldcap := cap(*c)
	if oldcap < newlen {
		a := make([]reflectValueWrapper, newlen, growCap(newlen, len(*c), oldcap))
		copy(a, *c)
		*c = a
	} else {
		*c = (*c)[:newlen]
	}
}

func (c *valueArrayCache) put(idx int, w reflectValueWrapper) {
	if len(*c) <= idx {
		c.grow(idx + 1)
	}
	(*c)[idx] = w
}

func (c *valueArrayCache) shrink(newlen int) {
	if len(*c) > newlen {
		tail := (*c)[newlen:]
		for i, item := range tail {
			if item != nil {
				copyReflectValueWrapper(item)
				tail[i] = nil
			}
		}
		*c = (*c)[:newlen]
	}
}

func (o *objectGoArrayReflect) _init() {
	o.objectGoReflect.init()
	o.class = classArray
	o.prototype = o.val.runtime.getArrayPrototype()
	o.baseObject._put("length", &o.lengthProp)
}

func (o *objectGoArrayReflect) init() {
	o._init()
	o.updateLen()
	o.putIdx = o._putIdx
}

func (o *objectGoArrayReflect) updateLen() {
	o.lengthProp.value = intToValue(int64(o.fieldsValue.Len()))
}

func (o *objectGoArrayReflect) _hasIdx(idx valueInt) bool {
	if idx := int64(idx); idx >= 0 && idx < int64(o.fieldsValue.Len()) {
		return true
	}
	return false
}

func (o *objectGoArrayReflect) _hasStr(name unistring.String) bool {
	if idx := strToIdx64(name); idx >= 0 && idx < int64(o.fieldsValue.Len()) {
		return true
	}
	return false
}

func (o *objectGoArrayReflect) _getIdx(idx int) Value {
	if v := o.valueCache.get(idx); v != nil {
		return v.esValue()
	}

	v := o.fieldsValue.Index(idx)

	res, w := o.elemToValue(v)
	if w != nil {
		o.valueCache.put(idx, w)
	}

	return res
}

func (o *objectGoArrayReflect) getIdx(idx valueInt, receiver Value) Value {
	if idx := toIntStrict(int64(idx)); idx >= 0 && idx < o.fieldsValue.Len() {
		return o._getIdx(idx)
	}
	return o.objectGoReflect.getStr(idx.string(), receiver)
}

func (o *objectGoArrayReflect) getStr(name unistring.String, receiver Value) Value {
	var ownProp Value
	if idx := strToGoIdx(name); idx >= 0 && idx < o.fieldsValue.Len() {
		ownProp = o._getIdx(idx)
	} else if name == "length" {
		if o.fieldsValue.Kind() == reflect.Slice {
			o.updateLen()
		}
		ownProp = &o.lengthProp
	} else {
		ownProp = o.objectGoReflect.getOwnPropStr(name)
	}
	return o.getStrWithOwnProp(ownProp, name, receiver)
}

func (o *objectGoArrayReflect) getOwnPropStr(name unistring.String) Value {
	if idx := strToGoIdx(name); idx >= 0 {
		if idx < o.fieldsValue.Len() {
			return &valueProperty{
				value:      o._getIdx(idx),
				writable:   true,
				enumerable: true,
			}
		}
		return nil
	}
	if name == "length" {
		if o.fieldsValue.Kind() == reflect.Slice {
			o.updateLen()
		}
		return &o.lengthProp
	}
	return o.objectGoReflect.getOwnPropStr(name)
}

func (o *objectGoArrayReflect) getOwnPropIdx(idx valueInt) Value {
	if idx := toIntStrict(int64(idx)); idx >= 0 && idx < o.fieldsValue.Len() {
		return &valueProperty{
			value:      o._getIdx(idx),
			writable:   true,
			enumerable: true,
		}
	}
	return nil
}

func (o *objectGoArrayReflect) _putIdx(idx int, v Value, throw bool) bool {
	cached := o.valueCache.get(idx)
	if cached != nil {
		copyReflectValueWrapper(cached)
	}

	rv := o.fieldsValue.Index(idx)
	err := o.val.runtime.toReflectValue(v, rv, &objectExportCtx{})
	if err != nil {
		if cached != nil {
			cached.setReflectValue(rv)
		}
		o.val.runtime.typeErrorResult(throw, "Go type conversion error: %v", err)
		return false
	}
	if cached != nil {
		o.valueCache[idx] = nil
	}
	return true
}

func (o *objectGoArrayReflect) setOwnIdx(idx valueInt, val Value, throw bool) bool {
	if i := toIntStrict(int64(idx)); i >= 0 {
		if i >= o.fieldsValue.Len() {
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
		if idx >= o.fieldsValue.Len() {
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
	return o.objectGoReflect.hasOwnPropertyStr(name)
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

func (o *objectGoArrayReflect) _deleteIdx(idx int) {
	if idx < o.fieldsValue.Len() {
		if cv := o.valueCache.get(idx); cv != nil {
			copyReflectValueWrapper(cv)
			o.valueCache[idx] = nil
		}

		o.fieldsValue.Index(idx).Set(reflect.Zero(o.fieldsValue.Type().Elem()))
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
	if i.idx < i.limit && i.idx < i.o.fieldsValue.Len() {
		name := strconv.Itoa(i.idx)
		i.idx++
		return propIterItem{name: asciiString(name), enumerable: _ENUM_TRUE}, i.next
	}

	return i.o.objectGoReflect.iterateStringKeys()()
}

func (o *objectGoArrayReflect) stringKeys(all bool, accum []Value) []Value {
	for i := 0; i < o.fieldsValue.Len(); i++ {
		accum = append(accum, asciiString(strconv.Itoa(i)))
	}

	return o.objectGoReflect.stringKeys(all, accum)
}

func (o *objectGoArrayReflect) iterateStringKeys() iterNextFunc {
	return (&goArrayReflectPropIter{
		o:     o,
		limit: o.fieldsValue.Len(),
	}).next
}

func (o *objectGoArrayReflect) sortLen() int {
	return o.fieldsValue.Len()
}

func (o *objectGoArrayReflect) sortGet(i int) Value {
	return o.getIdx(valueInt(i), nil)
}

func (o *objectGoArrayReflect) swap(i int, j int) {
	vi := o.fieldsValue.Index(i)
	vj := o.fieldsValue.Index(j)
	tmp := reflect.New(o.fieldsValue.Type().Elem()).Elem()
	tmp.Set(vi)
	vi.Set(vj)
	vj.Set(tmp)

	cachedI := o.valueCache.get(i)
	cachedJ := o.valueCache.get(j)
	if cachedI != nil {
		cachedI.setReflectValue(vj)
		o.valueCache.put(j, cachedI)
	} else {
		if j < len(o.valueCache) {
			o.valueCache[j] = nil
		}
	}

	if cachedJ != nil {
		cachedJ.setReflectValue(vi)
		o.valueCache.put(i, cachedJ)
	} else {
		if i < len(o.valueCache) {
			o.valueCache[i] = nil
		}
	}
}
