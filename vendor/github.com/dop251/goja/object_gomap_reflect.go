package goja

import (
	"reflect"

	"github.com/dop251/goja/unistring"
)

type objectGoMapReflect struct {
	objectGoReflect

	keyType, valueType reflect.Type
}

func (o *objectGoMapReflect) init() {
	o.objectGoReflect.init()
	o.keyType = o.value.Type().Key()
	o.valueType = o.value.Type().Elem()
}

func (o *objectGoMapReflect) toKey(n Value, throw bool) reflect.Value {
	key := reflect.New(o.keyType).Elem()
	err := o.val.runtime.toReflectValue(n, key, &objectExportCtx{})
	if err != nil {
		o.val.runtime.typeErrorResult(throw, "map key conversion error: %v", err)
		return reflect.Value{}
	}
	return key
}

func (o *objectGoMapReflect) strToKey(name string, throw bool) reflect.Value {
	if o.keyType.Kind() == reflect.String {
		return reflect.ValueOf(name).Convert(o.keyType)
	}
	return o.toKey(newStringValue(name), throw)
}

func (o *objectGoMapReflect) _get(n Value) Value {
	key := o.toKey(n, false)
	if !key.IsValid() {
		return nil
	}
	if v := o.value.MapIndex(key); v.IsValid() {
		return o.val.runtime.toValue(v.Interface(), v)
	}

	return nil
}

func (o *objectGoMapReflect) _getStr(name string) Value {
	key := o.strToKey(name, false)
	if !key.IsValid() {
		return nil
	}
	if v := o.value.MapIndex(key); v.IsValid() {
		return o.val.runtime.toValue(v.Interface(), v)
	}

	return nil
}

func (o *objectGoMapReflect) getStr(name unistring.String, receiver Value) Value {
	if v := o._getStr(name.String()); v != nil {
		return v
	}
	return o.objectGoReflect.getStr(name, receiver)
}

func (o *objectGoMapReflect) getIdx(idx valueInt, receiver Value) Value {
	if v := o._get(idx); v != nil {
		return v
	}
	return o.objectGoReflect.getIdx(idx, receiver)
}

func (o *objectGoMapReflect) getOwnPropStr(name unistring.String) Value {
	if v := o._getStr(name.String()); v != nil {
		return &valueProperty{
			value:      v,
			writable:   true,
			enumerable: true,
		}
	}
	return o.objectGoReflect.getOwnPropStr(name)
}

func (o *objectGoMapReflect) getOwnPropIdx(idx valueInt) Value {
	if v := o._get(idx); v != nil {
		return &valueProperty{
			value:      v,
			writable:   true,
			enumerable: true,
		}
	}
	return o.objectGoReflect.getOwnPropStr(idx.string())
}

func (o *objectGoMapReflect) toValue(val Value, throw bool) (reflect.Value, bool) {
	v := reflect.New(o.valueType).Elem()
	err := o.val.runtime.toReflectValue(val, v, &objectExportCtx{})
	if err != nil {
		o.val.runtime.typeErrorResult(throw, "map value conversion error: %v", err)
		return reflect.Value{}, false
	}

	return v, true
}

func (o *objectGoMapReflect) _put(key reflect.Value, val Value, throw bool) bool {
	if key.IsValid() {
		if o.extensible || o.value.MapIndex(key).IsValid() {
			v, ok := o.toValue(val, throw)
			if !ok {
				return false
			}
			o.value.SetMapIndex(key, v)
		} else {
			o.val.runtime.typeErrorResult(throw, "Cannot set property %s, object is not extensible", key.String())
			return false
		}
		return true
	}
	return false
}

func (o *objectGoMapReflect) setOwnStr(name unistring.String, val Value, throw bool) bool {
	n := name.String()
	key := o.strToKey(n, false)
	if !key.IsValid() || !o.value.MapIndex(key).IsValid() {
		if proto := o.prototype; proto != nil {
			// we know it's foreign because prototype loops are not allowed
			if res, ok := proto.self.setForeignStr(name, val, o.val, throw); ok {
				return res
			}
		}
		// new property
		if !o.extensible {
			o.val.runtime.typeErrorResult(throw, "Cannot add property %s, object is not extensible", n)
			return false
		} else {
			if throw && !key.IsValid() {
				o.strToKey(n, true)
				return false
			}
		}
	}
	o._put(key, val, throw)
	return true
}

func (o *objectGoMapReflect) setOwnIdx(idx valueInt, val Value, throw bool) bool {
	key := o.toKey(idx, false)
	if !key.IsValid() || !o.value.MapIndex(key).IsValid() {
		if proto := o.prototype; proto != nil {
			// we know it's foreign because prototype loops are not allowed
			if res, ok := proto.self.setForeignIdx(idx, val, o.val, throw); ok {
				return res
			}
		}
		// new property
		if !o.extensible {
			o.val.runtime.typeErrorResult(throw, "Cannot add property %d, object is not extensible", idx)
			return false
		} else {
			if throw && !key.IsValid() {
				o.toKey(idx, true)
				return false
			}
		}
	}
	o._put(key, val, throw)
	return true
}

func (o *objectGoMapReflect) setForeignStr(name unistring.String, val, receiver Value, throw bool) (bool, bool) {
	return o._setForeignStr(name, trueValIfPresent(o.hasOwnPropertyStr(name)), val, receiver, throw)
}

func (o *objectGoMapReflect) setForeignIdx(idx valueInt, val, receiver Value, throw bool) (bool, bool) {
	return o._setForeignIdx(idx, trueValIfPresent(o.hasOwnPropertyIdx(idx)), val, receiver, throw)
}

func (o *objectGoMapReflect) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	if !o.val.runtime.checkHostObjectPropertyDescr(name, descr, throw) {
		return false
	}

	return o._put(o.strToKey(name.String(), throw), descr.Value, throw)
}

func (o *objectGoMapReflect) defineOwnPropertyIdx(idx valueInt, descr PropertyDescriptor, throw bool) bool {
	if !o.val.runtime.checkHostObjectPropertyDescr(idx.string(), descr, throw) {
		return false
	}

	return o._put(o.toKey(idx, throw), descr.Value, throw)
}

func (o *objectGoMapReflect) hasOwnPropertyStr(name unistring.String) bool {
	key := o.strToKey(name.String(), false)
	if key.IsValid() && o.value.MapIndex(key).IsValid() {
		return true
	}
	return false
}

func (o *objectGoMapReflect) hasOwnPropertyIdx(idx valueInt) bool {
	key := o.toKey(idx, false)
	if key.IsValid() && o.value.MapIndex(key).IsValid() {
		return true
	}
	return false
}

func (o *objectGoMapReflect) deleteStr(name unistring.String, throw bool) bool {
	key := o.strToKey(name.String(), throw)
	if !key.IsValid() {
		return false
	}
	o.value.SetMapIndex(key, reflect.Value{})
	return true
}

func (o *objectGoMapReflect) deleteIdx(idx valueInt, throw bool) bool {
	key := o.toKey(idx, throw)
	if !key.IsValid() {
		return false
	}
	o.value.SetMapIndex(key, reflect.Value{})
	return true
}

type gomapReflectPropIter struct {
	o    *objectGoMapReflect
	keys []reflect.Value
	idx  int
}

func (i *gomapReflectPropIter) next() (propIterItem, iterNextFunc) {
	for i.idx < len(i.keys) {
		key := i.keys[i.idx]
		v := i.o.value.MapIndex(key)
		i.idx++
		if v.IsValid() {
			return propIterItem{name: newStringValue(key.String()), enumerable: _ENUM_TRUE}, i.next
		}
	}

	return propIterItem{}, nil
}

func (o *objectGoMapReflect) iterateStringKeys() iterNextFunc {
	return (&gomapReflectPropIter{
		o:    o,
		keys: o.value.MapKeys(),
	}).next
}

func (o *objectGoMapReflect) stringKeys(_ bool, accum []Value) []Value {
	// all own keys are enumerable
	for _, key := range o.value.MapKeys() {
		accum = append(accum, newStringValue(key.String()))
	}

	return accum
}

func (o *objectGoMapReflect) equal(other objectImpl) bool {
	if other, ok := other.(*objectGoMapReflect); ok {
		return o.value.Interface() == other.value.Interface()
	}
	return false
}
