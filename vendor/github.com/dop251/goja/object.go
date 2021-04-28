package goja

import (
	"fmt"
	"math"
	"reflect"
	"sort"

	"github.com/dop251/goja/unistring"
)

const (
	classObject   = "Object"
	classArray    = "Array"
	classWeakSet  = "WeakSet"
	classWeakMap  = "WeakMap"
	classMap      = "Map"
	classMath     = "Math"
	classSet      = "Set"
	classFunction = "Function"
	classNumber   = "Number"
	classString   = "String"
	classBoolean  = "Boolean"
	classError    = "Error"
	classRegExp   = "RegExp"
	classDate     = "Date"
	classJSON     = "JSON"
	classGlobal   = "global"

	classArrayIterator        = "Array Iterator"
	classMapIterator          = "Map Iterator"
	classSetIterator          = "Set Iterator"
	classStringIterator       = "String Iterator"
	classRegExpStringIterator = "RegExp String Iterator"
)

var (
	hintDefault Value = asciiString("default")
	hintNumber  Value = asciiString("number")
	hintString  Value = asciiString("string")
)

type Object struct {
	id      uint64
	runtime *Runtime
	self    objectImpl

	weakRefs map[weakMap]Value
}

type iterNextFunc func() (propIterItem, iterNextFunc)

type PropertyDescriptor struct {
	jsDescriptor *Object

	Value Value

	Writable, Configurable, Enumerable Flag

	Getter, Setter Value
}

func (p *PropertyDescriptor) Empty() bool {
	var empty PropertyDescriptor
	return *p == empty
}

func (p *PropertyDescriptor) toValue(r *Runtime) Value {
	if p.jsDescriptor != nil {
		return p.jsDescriptor
	}
	if p.Empty() {
		return _undefined
	}
	o := r.NewObject()
	s := o.self

	if p.Value != nil {
		s._putProp("value", p.Value, true, true, true)
	}

	if p.Writable != FLAG_NOT_SET {
		s._putProp("writable", valueBool(p.Writable.Bool()), true, true, true)
	}

	if p.Enumerable != FLAG_NOT_SET {
		s._putProp("enumerable", valueBool(p.Enumerable.Bool()), true, true, true)
	}

	if p.Configurable != FLAG_NOT_SET {
		s._putProp("configurable", valueBool(p.Configurable.Bool()), true, true, true)
	}

	if p.Getter != nil {
		s._putProp("get", p.Getter, true, true, true)
	}
	if p.Setter != nil {
		s._putProp("set", p.Setter, true, true, true)
	}

	return o
}

func (p *PropertyDescriptor) complete() {
	if p.Getter == nil && p.Setter == nil {
		if p.Value == nil {
			p.Value = _undefined
		}
		if p.Writable == FLAG_NOT_SET {
			p.Writable = FLAG_FALSE
		}
	} else {
		if p.Getter == nil {
			p.Getter = _undefined
		}
		if p.Setter == nil {
			p.Setter = _undefined
		}
	}
	if p.Enumerable == FLAG_NOT_SET {
		p.Enumerable = FLAG_FALSE
	}
	if p.Configurable == FLAG_NOT_SET {
		p.Configurable = FLAG_FALSE
	}
}

type objectExportCacheItem map[reflect.Type]interface{}

type objectExportCtx struct {
	cache map[objectImpl]interface{}
}

type objectImpl interface {
	sortable
	className() string
	getStr(p unistring.String, receiver Value) Value
	getIdx(p valueInt, receiver Value) Value
	getSym(p *Symbol, receiver Value) Value

	getOwnPropStr(unistring.String) Value
	getOwnPropIdx(valueInt) Value
	getOwnPropSym(*Symbol) Value

	setOwnStr(p unistring.String, v Value, throw bool) bool
	setOwnIdx(p valueInt, v Value, throw bool) bool
	setOwnSym(p *Symbol, v Value, throw bool) bool

	setForeignStr(p unistring.String, v, receiver Value, throw bool) (res bool, handled bool)
	setForeignIdx(p valueInt, v, receiver Value, throw bool) (res bool, handled bool)
	setForeignSym(p *Symbol, v, receiver Value, throw bool) (res bool, handled bool)

	hasPropertyStr(unistring.String) bool
	hasPropertyIdx(idx valueInt) bool
	hasPropertySym(s *Symbol) bool

	hasOwnPropertyStr(unistring.String) bool
	hasOwnPropertyIdx(valueInt) bool
	hasOwnPropertySym(s *Symbol) bool

	defineOwnPropertyStr(name unistring.String, desc PropertyDescriptor, throw bool) bool
	defineOwnPropertyIdx(name valueInt, desc PropertyDescriptor, throw bool) bool
	defineOwnPropertySym(name *Symbol, desc PropertyDescriptor, throw bool) bool

	deleteStr(name unistring.String, throw bool) bool
	deleteIdx(idx valueInt, throw bool) bool
	deleteSym(s *Symbol, throw bool) bool

	toPrimitiveNumber() Value
	toPrimitiveString() Value
	toPrimitive() Value
	assertCallable() (call func(FunctionCall) Value, ok bool)
	assertConstructor() func(args []Value, newTarget *Object) *Object
	proto() *Object
	setProto(proto *Object, throw bool) bool
	hasInstance(v Value) bool
	isExtensible() bool
	preventExtensions(throw bool) bool
	enumerateOwnKeys() iterNextFunc
	export(ctx *objectExportCtx) interface{}
	exportType() reflect.Type
	equal(objectImpl) bool
	ownKeys(all bool, accum []Value) []Value
	ownSymbols(all bool, accum []Value) []Value
	ownPropertyKeys(all bool, accum []Value) []Value

	_putProp(name unistring.String, value Value, writable, enumerable, configurable bool) Value
	_putSym(s *Symbol, prop Value)
}

type baseObject struct {
	class      string
	val        *Object
	prototype  *Object
	extensible bool

	values    map[unistring.String]Value
	propNames []unistring.String

	lastSortedPropLen, idxPropCount int

	symValues *orderedMap
}

type guardedObject struct {
	baseObject
	guardedProps map[unistring.String]struct{}
}

type primitiveValueObject struct {
	baseObject
	pValue Value
}

func (o *primitiveValueObject) export(*objectExportCtx) interface{} {
	return o.pValue.Export()
}

func (o *primitiveValueObject) exportType() reflect.Type {
	return o.pValue.ExportType()
}

type FunctionCall struct {
	This      Value
	Arguments []Value
}

type ConstructorCall struct {
	This      *Object
	Arguments []Value
	NewTarget *Object
}

func (f FunctionCall) Argument(idx int) Value {
	if idx < len(f.Arguments) {
		return f.Arguments[idx]
	}
	return _undefined
}

func (f ConstructorCall) Argument(idx int) Value {
	if idx < len(f.Arguments) {
		return f.Arguments[idx]
	}
	return _undefined
}

func (o *baseObject) init() {
	o.values = make(map[unistring.String]Value)
}

func (o *baseObject) className() string {
	return o.class
}

func (o *baseObject) hasPropertyStr(name unistring.String) bool {
	if o.val.self.hasOwnPropertyStr(name) {
		return true
	}
	if o.prototype != nil {
		return o.prototype.self.hasPropertyStr(name)
	}
	return false
}

func (o *baseObject) hasPropertyIdx(idx valueInt) bool {
	return o.val.self.hasPropertyStr(idx.string())
}

func (o *baseObject) hasPropertySym(s *Symbol) bool {
	if o.hasOwnPropertySym(s) {
		return true
	}
	if o.prototype != nil {
		return o.prototype.self.hasPropertySym(s)
	}
	return false
}

func (o *baseObject) getWithOwnProp(prop, p, receiver Value) Value {
	if prop == nil && o.prototype != nil {
		if receiver == nil {
			return o.prototype.get(p, o.val)
		}
		return o.prototype.get(p, receiver)
	}
	if prop, ok := prop.(*valueProperty); ok {
		if receiver == nil {
			return prop.get(o.val)
		}
		return prop.get(receiver)
	}
	return prop
}

func (o *baseObject) getStrWithOwnProp(prop Value, name unistring.String, receiver Value) Value {
	if prop == nil && o.prototype != nil {
		if receiver == nil {
			return o.prototype.self.getStr(name, o.val)
		}
		return o.prototype.self.getStr(name, receiver)
	}
	if prop, ok := prop.(*valueProperty); ok {
		if receiver == nil {
			return prop.get(o.val)
		}
		return prop.get(receiver)
	}
	return prop
}

func (o *baseObject) getIdx(idx valueInt, receiver Value) Value {
	return o.val.self.getStr(idx.string(), receiver)
}

func (o *baseObject) getSym(s *Symbol, receiver Value) Value {
	return o.getWithOwnProp(o.getOwnPropSym(s), s, receiver)
}

func (o *baseObject) getStr(name unistring.String, receiver Value) Value {
	prop := o.values[name]
	if prop == nil {
		if o.prototype != nil {
			if receiver == nil {
				return o.prototype.self.getStr(name, o.val)
			}
			return o.prototype.self.getStr(name, receiver)
		}
	}
	if prop, ok := prop.(*valueProperty); ok {
		if receiver == nil {
			return prop.get(o.val)
		}
		return prop.get(receiver)
	}
	return prop
}

func (o *baseObject) getOwnPropIdx(idx valueInt) Value {
	return o.val.self.getOwnPropStr(idx.string())
}

func (o *baseObject) getOwnPropSym(s *Symbol) Value {
	if o.symValues != nil {
		return o.symValues.get(s)
	}
	return nil
}

func (o *baseObject) getOwnPropStr(name unistring.String) Value {
	return o.values[name]
}

func (o *baseObject) checkDeleteProp(name unistring.String, prop *valueProperty, throw bool) bool {
	if !prop.configurable {
		o.val.runtime.typeErrorResult(throw, "Cannot delete property '%s' of %s", name, o.val.toString())
		return false
	}
	return true
}

func (o *baseObject) checkDelete(name unistring.String, val Value, throw bool) bool {
	if val, ok := val.(*valueProperty); ok {
		return o.checkDeleteProp(name, val, throw)
	}
	return true
}

func (o *baseObject) _delete(name unistring.String) {
	delete(o.values, name)
	for i, n := range o.propNames {
		if n == name {
			names := o.propNames
			if namesMarkedForCopy(names) {
				newNames := make([]unistring.String, len(names)-1, shrinkCap(len(names), cap(names)))
				copy(newNames, names[:i])
				copy(newNames[i:], names[i+1:])
				o.propNames = newNames
			} else {
				copy(names[i:], names[i+1:])
				names[len(names)-1] = ""
				o.propNames = names[:len(names)-1]
			}
			if i < o.lastSortedPropLen {
				o.lastSortedPropLen--
				if i < o.idxPropCount {
					o.idxPropCount--
				}
			}
			break
		}
	}
}

func (o *baseObject) deleteIdx(idx valueInt, throw bool) bool {
	return o.val.self.deleteStr(idx.string(), throw)
}

func (o *baseObject) deleteSym(s *Symbol, throw bool) bool {
	if o.symValues != nil {
		if val := o.symValues.get(s); val != nil {
			if !o.checkDelete(s.desc.string(), val, throw) {
				return false
			}
			o.symValues.remove(s)
		}
	}
	return true
}

func (o *baseObject) deleteStr(name unistring.String, throw bool) bool {
	if val, exists := o.values[name]; exists {
		if !o.checkDelete(name, val, throw) {
			return false
		}
		o._delete(name)
	}
	return true
}

func (o *baseObject) setProto(proto *Object, throw bool) bool {
	current := o.prototype
	if current.SameAs(proto) {
		return true
	}
	if !o.extensible {
		o.val.runtime.typeErrorResult(throw, "%s is not extensible", o.val)
		return false
	}
	for p := proto; p != nil; p = p.self.proto() {
		if p.SameAs(o.val) {
			o.val.runtime.typeErrorResult(throw, "Cyclic __proto__ value")
			return false
		}
		if _, ok := p.self.(*proxyObject); ok {
			break
		}
	}
	o.prototype = proto
	return true
}

func (o *baseObject) setOwnStr(name unistring.String, val Value, throw bool) bool {
	ownDesc := o.values[name]
	if ownDesc == nil {
		if proto := o.prototype; proto != nil {
			// we know it's foreign because prototype loops are not allowed
			if res, handled := proto.self.setForeignStr(name, val, o.val, throw); handled {
				return res
			}
		}
		// new property
		if !o.extensible {
			o.val.runtime.typeErrorResult(throw, "Cannot add property %s, object is not extensible", name)
			return false
		} else {
			o.values[name] = val
			names := copyNamesIfNeeded(o.propNames, 1)
			o.propNames = append(names, name)
		}
		return true
	}
	if prop, ok := ownDesc.(*valueProperty); ok {
		if !prop.isWritable() {
			o.val.runtime.typeErrorResult(throw, "Cannot assign to read only property '%s'", name)
			return false
		} else {
			prop.set(o.val, val)
		}
	} else {
		o.values[name] = val
	}
	return true
}

func (o *baseObject) setOwnIdx(idx valueInt, val Value, throw bool) bool {
	return o.val.self.setOwnStr(idx.string(), val, throw)
}

func (o *baseObject) setOwnSym(name *Symbol, val Value, throw bool) bool {
	var ownDesc Value
	if o.symValues != nil {
		ownDesc = o.symValues.get(name)
	}
	if ownDesc == nil {
		if proto := o.prototype; proto != nil {
			// we know it's foreign because prototype loops are not allowed
			if res, handled := proto.self.setForeignSym(name, val, o.val, throw); handled {
				return res
			}
		}
		// new property
		if !o.extensible {
			o.val.runtime.typeErrorResult(throw, "Cannot add property %s, object is not extensible", name)
			return false
		} else {
			if o.symValues == nil {
				o.symValues = newOrderedMap(nil)
			}
			o.symValues.set(name, val)
		}
		return true
	}
	if prop, ok := ownDesc.(*valueProperty); ok {
		if !prop.isWritable() {
			o.val.runtime.typeErrorResult(throw, "Cannot assign to read only property '%s'", name)
			return false
		} else {
			prop.set(o.val, val)
		}
	} else {
		o.symValues.set(name, val)
	}
	return true
}

func (o *baseObject) _setForeignStr(name unistring.String, prop, val, receiver Value, throw bool) (bool, bool) {
	if prop != nil {
		if prop, ok := prop.(*valueProperty); ok {
			if !prop.isWritable() {
				o.val.runtime.typeErrorResult(throw, "Cannot assign to read only property '%s'", name)
				return false, true
			}
			if prop.setterFunc != nil {
				prop.set(receiver, val)
				return true, true
			}
		}
	} else {
		if proto := o.prototype; proto != nil {
			if receiver != proto {
				return proto.self.setForeignStr(name, val, receiver, throw)
			}
			return proto.self.setOwnStr(name, val, throw), true
		}
	}
	return false, false
}

func (o *baseObject) _setForeignIdx(idx valueInt, prop, val, receiver Value, throw bool) (bool, bool) {
	if prop != nil {
		if prop, ok := prop.(*valueProperty); ok {
			if !prop.isWritable() {
				o.val.runtime.typeErrorResult(throw, "Cannot assign to read only property '%d'", idx)
				return false, true
			}
			if prop.setterFunc != nil {
				prop.set(receiver, val)
				return true, true
			}
		}
	} else {
		if proto := o.prototype; proto != nil {
			if receiver != proto {
				return proto.self.setForeignIdx(idx, val, receiver, throw)
			}
			return proto.self.setOwnIdx(idx, val, throw), true
		}
	}
	return false, false
}

func (o *baseObject) setForeignStr(name unistring.String, val, receiver Value, throw bool) (bool, bool) {
	return o._setForeignStr(name, o.values[name], val, receiver, throw)
}

func (o *baseObject) setForeignIdx(name valueInt, val, receiver Value, throw bool) (bool, bool) {
	if idx := toIdx(name); idx != math.MaxUint32 {
		if o.lastSortedPropLen != len(o.propNames) {
			o.fixPropOrder()
		}
		if o.idxPropCount == 0 {
			return o._setForeignIdx(name, name, nil, receiver, throw)
		}
	}
	return o.setForeignStr(name.string(), val, receiver, throw)
}

func (o *baseObject) setForeignSym(name *Symbol, val, receiver Value, throw bool) (bool, bool) {
	var prop Value
	if o.symValues != nil {
		prop = o.symValues.get(name)
	}
	if prop != nil {
		if prop, ok := prop.(*valueProperty); ok {
			if !prop.isWritable() {
				o.val.runtime.typeErrorResult(throw, "Cannot assign to read only property '%s'", name)
				return false, true
			}
			if prop.setterFunc != nil {
				prop.set(receiver, val)
				return true, true
			}
		}
	} else {
		if proto := o.prototype; proto != nil {
			if receiver != o.val {
				return proto.self.setForeignSym(name, val, receiver, throw)
			}
			return proto.self.setOwnSym(name, val, throw), true
		}
	}
	return false, false
}

func (o *baseObject) hasOwnPropertySym(s *Symbol) bool {
	if o.symValues != nil {
		return o.symValues.has(s)
	}
	return false
}

func (o *baseObject) hasOwnPropertyStr(name unistring.String) bool {
	_, exists := o.values[name]
	return exists
}

func (o *baseObject) hasOwnPropertyIdx(idx valueInt) bool {
	return o.val.self.hasOwnPropertyStr(idx.string())
}

func (o *baseObject) _defineOwnProperty(name unistring.String, existingValue Value, descr PropertyDescriptor, throw bool) (val Value, ok bool) {

	getterObj, _ := descr.Getter.(*Object)
	setterObj, _ := descr.Setter.(*Object)

	var existing *valueProperty

	if existingValue == nil {
		if !o.extensible {
			o.val.runtime.typeErrorResult(throw, "Cannot define property %s, object is not extensible", name)
			return nil, false
		}
		existing = &valueProperty{}
	} else {
		if existing, ok = existingValue.(*valueProperty); !ok {
			existing = &valueProperty{
				writable:     true,
				enumerable:   true,
				configurable: true,
				value:        existingValue,
			}
		}

		if !existing.configurable {
			if descr.Configurable == FLAG_TRUE {
				goto Reject
			}
			if descr.Enumerable != FLAG_NOT_SET && descr.Enumerable.Bool() != existing.enumerable {
				goto Reject
			}
		}
		if existing.accessor && descr.Value != nil || !existing.accessor && (getterObj != nil || setterObj != nil) {
			if !existing.configurable {
				goto Reject
			}
		} else if !existing.accessor {
			if !existing.configurable {
				if !existing.writable {
					if descr.Writable == FLAG_TRUE {
						goto Reject
					}
					if descr.Value != nil && !descr.Value.SameAs(existing.value) {
						goto Reject
					}
				}
			}
		} else {
			if !existing.configurable {
				if descr.Getter != nil && existing.getterFunc != getterObj || descr.Setter != nil && existing.setterFunc != setterObj {
					goto Reject
				}
			}
		}
	}

	if descr.Writable == FLAG_TRUE && descr.Enumerable == FLAG_TRUE && descr.Configurable == FLAG_TRUE && descr.Value != nil {
		return descr.Value, true
	}

	if descr.Writable != FLAG_NOT_SET {
		existing.writable = descr.Writable.Bool()
	}
	if descr.Enumerable != FLAG_NOT_SET {
		existing.enumerable = descr.Enumerable.Bool()
	}
	if descr.Configurable != FLAG_NOT_SET {
		existing.configurable = descr.Configurable.Bool()
	}

	if descr.Value != nil {
		existing.value = descr.Value
		existing.getterFunc = nil
		existing.setterFunc = nil
	}

	if descr.Value != nil || descr.Writable != FLAG_NOT_SET {
		existing.accessor = false
	}

	if descr.Getter != nil {
		existing.getterFunc = propGetter(o.val, descr.Getter, o.val.runtime)
		existing.value = nil
		existing.accessor = true
	}

	if descr.Setter != nil {
		existing.setterFunc = propSetter(o.val, descr.Setter, o.val.runtime)
		existing.value = nil
		existing.accessor = true
	}

	if !existing.accessor && existing.value == nil {
		existing.value = _undefined
	}

	return existing, true

Reject:
	o.val.runtime.typeErrorResult(throw, "Cannot redefine property: %s", name)
	return nil, false

}

func (o *baseObject) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	existingVal := o.values[name]
	if v, ok := o._defineOwnProperty(name, existingVal, descr, throw); ok {
		o.values[name] = v
		if existingVal == nil {
			names := copyNamesIfNeeded(o.propNames, 1)
			o.propNames = append(names, name)
		}
		return true
	}
	return false
}

func (o *baseObject) defineOwnPropertyIdx(idx valueInt, desc PropertyDescriptor, throw bool) bool {
	return o.val.self.defineOwnPropertyStr(idx.string(), desc, throw)
}

func (o *baseObject) defineOwnPropertySym(s *Symbol, descr PropertyDescriptor, throw bool) bool {
	var existingVal Value
	if o.symValues != nil {
		existingVal = o.symValues.get(s)
	}
	if v, ok := o._defineOwnProperty(s.desc.string(), existingVal, descr, throw); ok {
		if o.symValues == nil {
			o.symValues = newOrderedMap(nil)
		}
		o.symValues.set(s, v)
		return true
	}
	return false
}

func (o *baseObject) _put(name unistring.String, v Value) {
	if _, exists := o.values[name]; !exists {
		names := copyNamesIfNeeded(o.propNames, 1)
		o.propNames = append(names, name)
	}

	o.values[name] = v
}

func valueProp(value Value, writable, enumerable, configurable bool) Value {
	if writable && enumerable && configurable {
		return value
	}
	return &valueProperty{
		value:        value,
		writable:     writable,
		enumerable:   enumerable,
		configurable: configurable,
	}
}

func (o *baseObject) _putProp(name unistring.String, value Value, writable, enumerable, configurable bool) Value {
	prop := valueProp(value, writable, enumerable, configurable)
	o._put(name, prop)
	return prop
}

func (o *baseObject) _putSym(s *Symbol, prop Value) {
	if o.symValues == nil {
		o.symValues = newOrderedMap(nil)
	}
	o.symValues.set(s, prop)
}

func (o *Object) tryPrimitive(methodName unistring.String) Value {
	if method, ok := o.self.getStr(methodName, nil).(*Object); ok {
		if call, ok := method.self.assertCallable(); ok {
			v := call(FunctionCall{
				This: o,
			})
			if _, fail := v.(*Object); !fail {
				return v
			}
		}
	}
	return nil
}

func (o *Object) genericToPrimitiveNumber() Value {
	if v := o.tryPrimitive("valueOf"); v != nil {
		return v
	}

	if v := o.tryPrimitive("toString"); v != nil {
		return v
	}

	panic(o.runtime.NewTypeError("Could not convert %v to primitive", o.self))
}

func (o *baseObject) toPrimitiveNumber() Value {
	return o.val.genericToPrimitiveNumber()
}

func (o *Object) genericToPrimitiveString() Value {
	if v := o.tryPrimitive("toString"); v != nil {
		return v
	}

	if v := o.tryPrimitive("valueOf"); v != nil {
		return v
	}

	panic(o.runtime.NewTypeError("Could not convert %v to primitive", o.self))
}

func (o *Object) genericToPrimitive() Value {
	return o.genericToPrimitiveNumber()
}

func (o *baseObject) toPrimitiveString() Value {
	return o.val.genericToPrimitiveString()
}

func (o *baseObject) toPrimitive() Value {
	return o.val.genericToPrimitiveNumber()
}

func (o *Object) tryExoticToPrimitive(hint Value) Value {
	exoticToPrimitive := toMethod(o.self.getSym(SymToPrimitive, nil))
	if exoticToPrimitive != nil {
		ret := exoticToPrimitive(FunctionCall{
			This:      o,
			Arguments: []Value{hint},
		})
		if _, fail := ret.(*Object); !fail {
			return ret
		}
		panic(o.runtime.NewTypeError("Cannot convert object to primitive value"))
	}
	return nil
}

func (o *Object) toPrimitiveNumber() Value {
	if v := o.tryExoticToPrimitive(hintNumber); v != nil {
		return v
	}

	return o.self.toPrimitiveNumber()
}

func (o *Object) toPrimitiveString() Value {
	if v := o.tryExoticToPrimitive(hintString); v != nil {
		return v
	}

	return o.self.toPrimitiveString()
}

func (o *Object) toPrimitive() Value {
	if v := o.tryExoticToPrimitive(hintDefault); v != nil {
		return v
	}
	return o.self.toPrimitive()
}

func (o *baseObject) assertCallable() (func(FunctionCall) Value, bool) {
	return nil, false
}

func (o *baseObject) assertConstructor() func(args []Value, newTarget *Object) *Object {
	return nil
}

func (o *baseObject) proto() *Object {
	return o.prototype
}

func (o *baseObject) isExtensible() bool {
	return o.extensible
}

func (o *baseObject) preventExtensions(bool) bool {
	o.extensible = false
	return true
}

func (o *baseObject) sortLen() int64 {
	return toLength(o.val.self.getStr("length", nil))
}

func (o *baseObject) sortGet(i int64) Value {
	return o.val.self.getIdx(valueInt(i), nil)
}

func (o *baseObject) swap(i, j int64) {
	ii := valueInt(i)
	jj := valueInt(j)

	x := o.val.self.getIdx(ii, nil)
	y := o.val.self.getIdx(jj, nil)

	o.val.self.setOwnIdx(ii, y, false)
	o.val.self.setOwnIdx(jj, x, false)
}

func (o *baseObject) export(ctx *objectExportCtx) interface{} {
	if v, exists := ctx.get(o); exists {
		return v
	}
	keys := o.ownKeys(false, nil)
	m := make(map[string]interface{}, len(keys))
	ctx.put(o, m)
	for _, itemName := range keys {
		itemNameStr := itemName.String()
		v := o.val.self.getStr(itemName.string(), nil)
		if v != nil {
			m[itemNameStr] = exportValue(v, ctx)
		} else {
			m[itemNameStr] = nil
		}
	}

	return m
}

func (o *baseObject) exportType() reflect.Type {
	return reflectTypeMap
}

type enumerableFlag int

const (
	_ENUM_UNKNOWN enumerableFlag = iota
	_ENUM_FALSE
	_ENUM_TRUE
)

type propIterItem struct {
	name       unistring.String
	value      Value // set only when enumerable == _ENUM_UNKNOWN
	enumerable enumerableFlag
}

type objectPropIter struct {
	o         *baseObject
	propNames []unistring.String
	idx       int
}

type recursivePropIter struct {
	o    objectImpl
	cur  iterNextFunc
	seen map[unistring.String]struct{}
}

type enumerableIter struct {
	wrapped iterNextFunc
}

func (i *enumerableIter) next() (propIterItem, iterNextFunc) {
	for {
		var item propIterItem
		item, i.wrapped = i.wrapped()
		if i.wrapped == nil {
			return item, nil
		}
		if item.enumerable == _ENUM_FALSE {
			continue
		}
		if item.enumerable == _ENUM_UNKNOWN {
			if prop, ok := item.value.(*valueProperty); ok {
				if !prop.enumerable {
					continue
				}
			}
		}
		return item, i.next
	}
}

func (i *recursivePropIter) next() (propIterItem, iterNextFunc) {
	for {
		var item propIterItem
		item, i.cur = i.cur()
		if i.cur == nil {
			if proto := i.o.proto(); proto != nil {
				i.cur = proto.self.enumerateOwnKeys()
				i.o = proto.self
				continue
			}
			return propIterItem{}, nil
		}
		if _, exists := i.seen[item.name]; !exists {
			i.seen[item.name] = struct{}{}
			return item, i.next
		}
	}
}

func enumerateRecursive(o *Object) iterNextFunc {
	return (&enumerableIter{
		wrapped: (&recursivePropIter{
			o:    o.self,
			cur:  o.self.enumerateOwnKeys(),
			seen: make(map[unistring.String]struct{}),
		}).next,
	}).next
}

func (i *objectPropIter) next() (propIterItem, iterNextFunc) {
	for i.idx < len(i.propNames) {
		name := i.propNames[i.idx]
		i.idx++
		prop := i.o.values[name]
		if prop != nil {
			return propIterItem{name: name, value: prop}, i.next
		}
	}
	clearNamesCopyMarker(i.propNames)
	return propIterItem{}, nil
}

var copyMarker = unistring.String(" ")

// Set a copy-on-write flag so that any subsequent modifications of anything below the current length
// trigger a copy.
// The marker is a special value put at the index position of cap-1. Capacity is set so that the marker is
// beyond the current length (therefore invisible to normal slice operations).
// This function is called before an iteration begins to avoid copying of the names array if
// there are no modifications within the iteration.
// Note that the copying also occurs in two cases: nested iterations (on the same object) and
// iterations after a previously abandoned iteration (because there is currently no mechanism to close an
// iterator). It is still better than copying every time.
func prepareNamesForCopy(names []unistring.String) []unistring.String {
	if len(names) == 0 {
		return names
	}
	if namesMarkedForCopy(names) || cap(names) == len(names) {
		var newcap int
		if cap(names) == len(names) {
			newcap = growCap(len(names)+1, len(names), cap(names))
		} else {
			newcap = cap(names)
		}
		newNames := make([]unistring.String, len(names), newcap)
		copy(newNames, names)
		names = newNames
	}
	names[cap(names)-1 : cap(names)][0] = copyMarker
	return names
}

func namesMarkedForCopy(names []unistring.String) bool {
	return cap(names) > len(names) && names[cap(names)-1 : cap(names)][0] == copyMarker
}

func clearNamesCopyMarker(names []unistring.String) {
	if cap(names) > len(names) {
		names[cap(names)-1 : cap(names)][0] = ""
	}
}

func copyNamesIfNeeded(names []unistring.String, extraCap int) []unistring.String {
	if namesMarkedForCopy(names) && len(names)+extraCap >= cap(names) {
		var newcap int
		newsize := len(names) + extraCap + 1
		if newsize > cap(names) {
			newcap = growCap(newsize, len(names), cap(names))
		} else {
			newcap = cap(names)
		}
		newNames := make([]unistring.String, len(names), newcap)
		copy(newNames, names)
		return newNames
	}
	return names
}

func (o *baseObject) enumerateOwnKeys() iterNextFunc {
	if len(o.propNames) > o.lastSortedPropLen {
		o.fixPropOrder()
	}
	propNames := prepareNamesForCopy(o.propNames)
	o.propNames = propNames
	return (&objectPropIter{
		o:         o,
		propNames: propNames,
	}).next
}

func (o *baseObject) equal(objectImpl) bool {
	// Rely on parent reference comparison
	return false
}

// Reorder property names so that any integer properties are shifted to the beginning of the list
// in ascending order. This is to conform to https://262.ecma-international.org/#sec-ordinaryownpropertykeys.
// Personally I think this requirement is strange. I can sort of understand where they are coming from,
// this way arrays can be specified just as objects with a 'magic' length property. However, I think
// it's safe to assume most devs don't use Objects to store integer properties. Therefore, performing
// property type checks when adding (and potentially looking up) properties would be unreasonable.
// Instead, we keep insertion order and only change it when (if) the properties get enumerated.
func (o *baseObject) fixPropOrder() {
	names := o.propNames
	for i := o.lastSortedPropLen; i < len(names); i++ {
		name := names[i]
		if idx := strToIdx(name); idx != math.MaxUint32 {
			k := sort.Search(o.idxPropCount, func(j int) bool {
				return strToIdx(names[j]) >= idx
			})
			if k < i {
				if namesMarkedForCopy(names) {
					newNames := make([]unistring.String, len(names), cap(names))
					copy(newNames[:k], names)
					copy(newNames[k+1:i+1], names[k:i])
					copy(newNames[i+1:], names[i+1:])
					names = newNames
					o.propNames = names
				} else {
					copy(names[k+1:i+1], names[k:i])
				}
				names[k] = name
			}
			o.idxPropCount++
		}
	}
	o.lastSortedPropLen = len(names)
}

func (o *baseObject) ownKeys(all bool, keys []Value) []Value {
	if len(o.propNames) > o.lastSortedPropLen {
		o.fixPropOrder()
	}
	if all {
		for _, k := range o.propNames {
			keys = append(keys, stringValueFromRaw(k))
		}
	} else {
		for _, k := range o.propNames {
			prop := o.values[k]
			if prop, ok := prop.(*valueProperty); ok && !prop.enumerable {
				continue
			}
			keys = append(keys, stringValueFromRaw(k))
		}
	}
	return keys
}

func (o *baseObject) ownSymbols(all bool, accum []Value) []Value {
	if o.symValues != nil {
		iter := o.symValues.newIter()
		if all {
			for {
				entry := iter.next()
				if entry == nil {
					break
				}
				accum = append(accum, entry.key)
			}
		} else {
			for {
				entry := iter.next()
				if entry == nil {
					break
				}
				if prop, ok := entry.value.(*valueProperty); ok {
					if !prop.enumerable {
						continue
					}
				}
				accum = append(accum, entry.key)
			}
		}
	}

	return accum
}

func (o *baseObject) ownPropertyKeys(all bool, accum []Value) []Value {
	return o.ownSymbols(all, o.val.self.ownKeys(all, accum))
}

func (o *baseObject) hasInstance(Value) bool {
	panic(o.val.runtime.NewTypeError("Expecting a function in instanceof check, but got %s", o.val.toString()))
}

func toMethod(v Value) func(FunctionCall) Value {
	if v == nil || IsUndefined(v) || IsNull(v) {
		return nil
	}
	if obj, ok := v.(*Object); ok {
		if call, ok := obj.self.assertCallable(); ok {
			return call
		}
	}
	panic(typeError(fmt.Sprintf("%s is not a method", v.String())))
}

func instanceOfOperator(o Value, c *Object) bool {
	if instOfHandler := toMethod(c.self.getSym(SymHasInstance, c)); instOfHandler != nil {
		return instOfHandler(FunctionCall{
			This:      c,
			Arguments: []Value{o},
		}).ToBoolean()
	}

	return c.self.hasInstance(o)
}

func (o *Object) get(p Value, receiver Value) Value {
	switch p := p.(type) {
	case valueInt:
		return o.self.getIdx(p, receiver)
	case *Symbol:
		return o.self.getSym(p, receiver)
	default:
		return o.self.getStr(p.string(), receiver)
	}
}

func (o *Object) getOwnProp(p Value) Value {
	switch p := p.(type) {
	case valueInt:
		return o.self.getOwnPropIdx(p)
	case *Symbol:
		return o.self.getOwnPropSym(p)
	default:
		return o.self.getOwnPropStr(p.string())
	}
}

func (o *Object) hasOwnProperty(p Value) bool {
	switch p := p.(type) {
	case valueInt:
		return o.self.hasOwnPropertyIdx(p)
	case *Symbol:
		return o.self.hasOwnPropertySym(p)
	default:
		return o.self.hasOwnPropertyStr(p.string())
	}
}

func (o *Object) hasProperty(p Value) bool {
	switch p := p.(type) {
	case valueInt:
		return o.self.hasPropertyIdx(p)
	case *Symbol:
		return o.self.hasPropertySym(p)
	default:
		return o.self.hasPropertyStr(p.string())
	}
}

func (o *Object) setStr(name unistring.String, val, receiver Value, throw bool) bool {
	if receiver == o {
		return o.self.setOwnStr(name, val, throw)
	} else {
		if res, ok := o.self.setForeignStr(name, val, receiver, throw); !ok {
			if robj, ok := receiver.(*Object); ok {
				if prop := robj.self.getOwnPropStr(name); prop != nil {
					if desc, ok := prop.(*valueProperty); ok {
						if desc.accessor {
							o.runtime.typeErrorResult(throw, "Receiver property %s is an accessor", name)
							return false
						}
						if !desc.writable {
							o.runtime.typeErrorResult(throw, "Cannot assign to read only property '%s'", name)
							return false
						}
					}
					robj.self.defineOwnPropertyStr(name, PropertyDescriptor{Value: val}, throw)
				} else {
					robj.self.defineOwnPropertyStr(name, PropertyDescriptor{
						Value:        val,
						Writable:     FLAG_TRUE,
						Configurable: FLAG_TRUE,
						Enumerable:   FLAG_TRUE,
					}, throw)
				}
			} else {
				o.runtime.typeErrorResult(throw, "Receiver is not an object: %v", receiver)
				return false
			}
		} else {
			return res
		}
	}
	return true
}

func (o *Object) set(name Value, val, receiver Value, throw bool) bool {
	switch name := name.(type) {
	case valueInt:
		return o.setIdx(name, val, receiver, throw)
	case *Symbol:
		return o.setSym(name, val, receiver, throw)
	default:
		return o.setStr(name.string(), val, receiver, throw)
	}
}

func (o *Object) setOwn(name Value, val Value, throw bool) bool {
	switch name := name.(type) {
	case valueInt:
		return o.self.setOwnIdx(name, val, throw)
	case *Symbol:
		return o.self.setOwnSym(name, val, throw)
	default:
		return o.self.setOwnStr(name.string(), val, throw)
	}
}

func (o *Object) setIdx(name valueInt, val, receiver Value, throw bool) bool {
	if receiver == o {
		return o.self.setOwnIdx(name, val, throw)
	} else {
		if res, ok := o.self.setForeignIdx(name, val, receiver, throw); !ok {
			if robj, ok := receiver.(*Object); ok {
				if prop := robj.self.getOwnPropIdx(name); prop != nil {
					if desc, ok := prop.(*valueProperty); ok {
						if desc.accessor {
							o.runtime.typeErrorResult(throw, "Receiver property %s is an accessor", name)
							return false
						}
						if !desc.writable {
							o.runtime.typeErrorResult(throw, "Cannot assign to read only property '%s'", name)
							return false
						}
					}
					robj.self.defineOwnPropertyIdx(name, PropertyDescriptor{Value: val}, throw)
				} else {
					robj.self.defineOwnPropertyIdx(name, PropertyDescriptor{
						Value:        val,
						Writable:     FLAG_TRUE,
						Configurable: FLAG_TRUE,
						Enumerable:   FLAG_TRUE,
					}, throw)
				}
			} else {
				o.runtime.typeErrorResult(throw, "Receiver is not an object: %v", receiver)
				return false
			}
		} else {
			return res
		}
	}
	return true
}

func (o *Object) setSym(name *Symbol, val, receiver Value, throw bool) bool {
	if receiver == o {
		return o.self.setOwnSym(name, val, throw)
	} else {
		if res, ok := o.self.setForeignSym(name, val, receiver, throw); !ok {
			if robj, ok := receiver.(*Object); ok {
				if prop := robj.self.getOwnPropSym(name); prop != nil {
					if desc, ok := prop.(*valueProperty); ok {
						if desc.accessor {
							o.runtime.typeErrorResult(throw, "Receiver property %s is an accessor", name)
							return false
						}
						if !desc.writable {
							o.runtime.typeErrorResult(throw, "Cannot assign to read only property '%s'", name)
							return false
						}
					}
					robj.self.defineOwnPropertySym(name, PropertyDescriptor{Value: val}, throw)
				} else {
					robj.self.defineOwnPropertySym(name, PropertyDescriptor{
						Value:        val,
						Writable:     FLAG_TRUE,
						Configurable: FLAG_TRUE,
						Enumerable:   FLAG_TRUE,
					}, throw)
				}
			} else {
				o.runtime.typeErrorResult(throw, "Receiver is not an object: %v", receiver)
				return false
			}
		} else {
			return res
		}
	}
	return true
}

func (o *Object) delete(n Value, throw bool) bool {
	switch n := n.(type) {
	case valueInt:
		return o.self.deleteIdx(n, throw)
	case *Symbol:
		return o.self.deleteSym(n, throw)
	default:
		return o.self.deleteStr(n.string(), throw)
	}
}

func (o *Object) defineOwnProperty(n Value, desc PropertyDescriptor, throw bool) bool {
	switch n := n.(type) {
	case valueInt:
		return o.self.defineOwnPropertyIdx(n, desc, throw)
	case *Symbol:
		return o.self.defineOwnPropertySym(n, desc, throw)
	default:
		return o.self.defineOwnPropertyStr(n.string(), desc, throw)
	}
}

func (o *Object) getWeakRefs() map[weakMap]Value {
	refs := o.weakRefs
	if refs == nil {
		refs = make(map[weakMap]Value)
		o.weakRefs = refs
	}
	return refs
}

func (o *Object) getId() uint64 {
	id := o.id
	if id == 0 {
		id = o.runtime.genId()
		o.id = id
	}
	return id
}

func (o *guardedObject) guard(props ...unistring.String) {
	if o.guardedProps == nil {
		o.guardedProps = make(map[unistring.String]struct{})
	}
	for _, p := range props {
		o.guardedProps[p] = struct{}{}
	}
}

func (o *guardedObject) check(p unistring.String) {
	if _, exists := o.guardedProps[p]; exists {
		o.val.self = &o.baseObject
	}
}

func (o *guardedObject) setOwnStr(p unistring.String, v Value, throw bool) bool {
	res := o.baseObject.setOwnStr(p, v, throw)
	if res {
		o.check(p)
	}
	return res
}

func (o *guardedObject) defineOwnPropertyStr(name unistring.String, desc PropertyDescriptor, throw bool) bool {
	res := o.baseObject.defineOwnPropertyStr(name, desc, throw)
	if res {
		o.check(name)
	}
	return res
}

func (o *guardedObject) deleteStr(name unistring.String, throw bool) bool {
	res := o.baseObject.deleteStr(name, throw)
	if res {
		o.check(name)
	}
	return res
}

func (ctx *objectExportCtx) get(key objectImpl) (interface{}, bool) {
	if v, exists := ctx.cache[key]; exists {
		if item, ok := v.(objectExportCacheItem); ok {
			r, exists := item[key.exportType()]
			return r, exists
		} else {
			return v, true
		}
	}
	return nil, false
}

func (ctx *objectExportCtx) getTyped(key objectImpl, typ reflect.Type) (interface{}, bool) {
	if v, exists := ctx.cache[key]; exists {
		if item, ok := v.(objectExportCacheItem); ok {
			r, exists := item[typ]
			return r, exists
		} else {
			if reflect.TypeOf(v) == typ {
				return v, true
			}
		}
	}
	return nil, false
}

func (ctx *objectExportCtx) put(key objectImpl, value interface{}) {
	if ctx.cache == nil {
		ctx.cache = make(map[objectImpl]interface{})
	}
	if item, ok := ctx.cache[key].(objectExportCacheItem); ok {
		item[key.exportType()] = value
	} else {
		ctx.cache[key] = value
	}
}

func (ctx *objectExportCtx) putTyped(key objectImpl, typ reflect.Type, value interface{}) {
	if ctx.cache == nil {
		ctx.cache = make(map[objectImpl]interface{})
	}
	v, exists := ctx.cache[key]
	if exists {
		if item, ok := ctx.cache[key].(objectExportCacheItem); ok {
			item[typ] = value
		} else {
			m := make(objectExportCacheItem, 2)
			m[key.exportType()] = v
			m[typ] = value
			ctx.cache[key] = m
		}
	} else {
		m := make(objectExportCacheItem)
		m[typ] = value
		ctx.cache[key] = m
	}
}
