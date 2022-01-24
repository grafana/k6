package goja

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/dop251/goja/unistring"
)

/*
DynamicObject is an interface representing a handler for a dynamic Object. Such an object can be created
using the Runtime.NewDynamicObject() method.

Note that Runtime.ToValue() does not have any special treatment for DynamicObject. The only way to create
a dynamic object is by using the Runtime.NewDynamicObject() method. This is done deliberately to avoid
silent code breaks when this interface changes.
*/
type DynamicObject interface {
	// Get a property value for the key. May return nil if the property does not exist.
	Get(key string) Value
	// Set a property value for the key. Return true if success, false otherwise.
	Set(key string, val Value) bool
	// Has should return true if and only if the property exists.
	Has(key string) bool
	// Delete the property for the key. Returns true on success (note, that includes missing property).
	Delete(key string) bool
	// Keys returns a list of all existing property keys. There are no checks for duplicates or to make sure
	// that the order conforms to https://262.ecma-international.org/#sec-ordinaryownpropertykeys
	Keys() []string
}

/*
DynamicArray is an interface representing a handler for a dynamic array Object. Such an object can be created
using the Runtime.NewDynamicArray() method.

Any integer property key or a string property key that can be parsed into an int value (including negative
ones) is treated as an index and passed to the trap methods of the DynamicArray. Note this is different from
the regular ECMAScript arrays which only support positive indexes up to 2^32-1.

DynamicArray cannot be sparse, i.e. hasOwnProperty(num) will return true for num >= 0 && num < Len(). Deleting
such a property is equivalent to setting it to undefined. Note that this creates a slight peculiarity because
hasOwnProperty() will still return true, even after deletion.

Note that Runtime.ToValue() does not have any special treatment for DynamicArray. The only way to create
a dynamic array is by using the Runtime.NewDynamicArray() method. This is done deliberately to avoid
silent code breaks when this interface changes.
*/
type DynamicArray interface {
	// Len returns the current array length.
	Len() int
	// Get an item at index idx. Note that idx may be any integer, negative or beyond the current length.
	Get(idx int) Value
	// Set an item at index idx. Note that idx may be any integer, negative or beyond the current length.
	// The expected behaviour when it's beyond length is that the array's length is increased to accommodate
	// the item. All elements in the 'new' section of the array should be zeroed.
	Set(idx int, val Value) bool
	// SetLen is called when the array's 'length' property is changed. If the length is increased all elements in the
	// 'new' section of the array should be zeroed.
	SetLen(int) bool
}

type baseDynamicObject struct {
	val       *Object
	prototype *Object
}

type dynamicObject struct {
	baseDynamicObject
	d DynamicObject
}

type dynamicArray struct {
	baseDynamicObject
	a DynamicArray
}

/*
NewDynamicObject creates an Object backed by the provided DynamicObject handler.

All properties of this Object are Writable, Enumerable and Configurable data properties. Any attempt to define
a property that does not conform to this will fail.

The Object is always extensible and cannot be made non-extensible. Object.preventExtensions() will fail.

The Object's prototype is initially set to Object.prototype, but can be changed using regular mechanisms
(Object.SetPrototype() in Go or Object.setPrototypeOf() in JS).

The Object cannot have own Symbol properties, however its prototype can. If you need an iterator support for
example, you could create a regular object, set Symbol.iterator on that object and then use it as a
prototype. See TestDynamicObjectCustomProto for more details.

Export() returns the original DynamicObject.

This mechanism is similar to ECMAScript Proxy, however because all properties are enumerable and the object
is always extensible there is no need for invariant checks which removes the need to have a target object and
makes it a lot more efficient.
*/
func (r *Runtime) NewDynamicObject(d DynamicObject) *Object {
	v := &Object{runtime: r}
	o := &dynamicObject{
		d: d,
		baseDynamicObject: baseDynamicObject{
			val:       v,
			prototype: r.global.ObjectPrototype,
		},
	}
	v.self = o
	return v
}

/*
NewDynamicArray creates an array Object backed by the provided DynamicArray handler.
It is similar to NewDynamicObject, the differences are:

- the Object is an array (i.e. Array.isArray() will return true and it will have the length property).

- the prototype will be initially set to Array.prototype.

- the Object cannot have any own string properties except for the 'length'.
*/
func (r *Runtime) NewDynamicArray(a DynamicArray) *Object {
	v := &Object{runtime: r}
	o := &dynamicArray{
		a: a,
		baseDynamicObject: baseDynamicObject{
			val:       v,
			prototype: r.global.ArrayPrototype,
		},
	}
	v.self = o
	return v
}

func (*dynamicObject) sortLen() int64 {
	return 0
}

func (*dynamicObject) sortGet(i int64) Value {
	return nil
}

func (*dynamicObject) swap(i int64, i2 int64) {
}

func (*dynamicObject) className() string {
	return classObject
}

func (o *baseDynamicObject) getParentStr(p unistring.String, receiver Value) Value {
	if proto := o.prototype; proto != nil {
		if receiver == nil {
			return proto.self.getStr(p, o.val)
		}
		return proto.self.getStr(p, receiver)
	}
	return nil
}

func (o *dynamicObject) getStr(p unistring.String, receiver Value) Value {
	prop := o.d.Get(p.String())
	if prop == nil {
		return o.getParentStr(p, receiver)
	}
	return prop
}

func (o *baseDynamicObject) getParentIdx(p valueInt, receiver Value) Value {
	if proto := o.prototype; proto != nil {
		if receiver == nil {
			return proto.self.getIdx(p, o.val)
		}
		return proto.self.getIdx(p, receiver)
	}
	return nil
}

func (o *dynamicObject) getIdx(p valueInt, receiver Value) Value {
	prop := o.d.Get(p.String())
	if prop == nil {
		return o.getParentIdx(p, receiver)
	}
	return prop
}

func (o *baseDynamicObject) getSym(p *Symbol, receiver Value) Value {
	if proto := o.prototype; proto != nil {
		if receiver == nil {
			return proto.self.getSym(p, o.val)
		}
		return proto.self.getSym(p, receiver)
	}
	return nil
}

func (o *dynamicObject) getOwnPropStr(u unistring.String) Value {
	return o.d.Get(u.String())
}

func (o *dynamicObject) getOwnPropIdx(v valueInt) Value {
	return o.d.Get(v.String())
}

func (*baseDynamicObject) getOwnPropSym(*Symbol) Value {
	return nil
}

func (o *dynamicObject) _set(prop string, v Value, throw bool) bool {
	if o.d.Set(prop, v) {
		return true
	}
	o.val.runtime.typeErrorResult(throw, "'Set' on a dynamic object returned false")
	return false
}

func (o *baseDynamicObject) _setSym(throw bool) {
	o.val.runtime.typeErrorResult(throw, "Dynamic objects do not support Symbol properties")
}

func (o *dynamicObject) setOwnStr(p unistring.String, v Value, throw bool) bool {
	prop := p.String()
	if !o.d.Has(prop) {
		if proto := o.prototype; proto != nil {
			// we know it's foreign because prototype loops are not allowed
			if res, handled := proto.self.setForeignStr(p, v, o.val, throw); handled {
				return res
			}
		}
	}
	return o._set(prop, v, throw)
}

func (o *dynamicObject) setOwnIdx(p valueInt, v Value, throw bool) bool {
	prop := p.String()
	if !o.d.Has(prop) {
		if proto := o.prototype; proto != nil {
			// we know it's foreign because prototype loops are not allowed
			if res, handled := proto.self.setForeignIdx(p, v, o.val, throw); handled {
				return res
			}
		}
	}
	return o._set(prop, v, throw)
}

func (o *baseDynamicObject) setOwnSym(s *Symbol, v Value, throw bool) bool {
	if proto := o.prototype; proto != nil {
		// we know it's foreign because prototype loops are not allowed
		if res, handled := proto.self.setForeignSym(s, v, o.val, throw); handled {
			return res
		}
	}
	o._setSym(throw)
	return false
}

func (o *baseDynamicObject) setParentForeignStr(p unistring.String, v, receiver Value, throw bool) (res bool, handled bool) {
	if proto := o.prototype; proto != nil {
		if receiver != proto {
			return proto.self.setForeignStr(p, v, receiver, throw)
		}
		return proto.self.setOwnStr(p, v, throw), true
	}
	return false, false
}

func (o *dynamicObject) setForeignStr(p unistring.String, v, receiver Value, throw bool) (res bool, handled bool) {
	prop := p.String()
	if !o.d.Has(prop) {
		return o.setParentForeignStr(p, v, receiver, throw)
	}
	return false, false
}

func (o *baseDynamicObject) setParentForeignIdx(p valueInt, v, receiver Value, throw bool) (res bool, handled bool) {
	if proto := o.prototype; proto != nil {
		if receiver != proto {
			return proto.self.setForeignIdx(p, v, receiver, throw)
		}
		return proto.self.setOwnIdx(p, v, throw), true
	}
	return false, false
}

func (o *dynamicObject) setForeignIdx(p valueInt, v, receiver Value, throw bool) (res bool, handled bool) {
	prop := p.String()
	if !o.d.Has(prop) {
		return o.setParentForeignIdx(p, v, receiver, throw)
	}
	return false, false
}

func (o *baseDynamicObject) setForeignSym(p *Symbol, v, receiver Value, throw bool) (res bool, handled bool) {
	if proto := o.prototype; proto != nil {
		if receiver != proto {
			return proto.self.setForeignSym(p, v, receiver, throw)
		}
		return proto.self.setOwnSym(p, v, throw), true
	}
	return false, false
}

func (o *dynamicObject) hasPropertyStr(u unistring.String) bool {
	if o.hasOwnPropertyStr(u) {
		return true
	}
	if proto := o.prototype; proto != nil {
		return proto.self.hasPropertyStr(u)
	}
	return false
}

func (o *dynamicObject) hasPropertyIdx(idx valueInt) bool {
	if o.hasOwnPropertyIdx(idx) {
		return true
	}
	if proto := o.prototype; proto != nil {
		return proto.self.hasPropertyIdx(idx)
	}
	return false
}

func (o *baseDynamicObject) hasPropertySym(s *Symbol) bool {
	if proto := o.prototype; proto != nil {
		return proto.self.hasPropertySym(s)
	}
	return false
}

func (o *dynamicObject) hasOwnPropertyStr(u unistring.String) bool {
	return o.d.Has(u.String())
}

func (o *dynamicObject) hasOwnPropertyIdx(v valueInt) bool {
	return o.d.Has(v.String())
}

func (*baseDynamicObject) hasOwnPropertySym(_ *Symbol) bool {
	return false
}

func (o *baseDynamicObject) checkDynamicObjectPropertyDescr(name fmt.Stringer, descr PropertyDescriptor, throw bool) bool {
	if descr.Getter != nil || descr.Setter != nil {
		o.val.runtime.typeErrorResult(throw, "Dynamic objects do not support accessor properties")
		return false
	}
	if descr.Writable == FLAG_FALSE {
		o.val.runtime.typeErrorResult(throw, "Dynamic object field %q cannot be made read-only", name.String())
		return false
	}
	if descr.Enumerable == FLAG_FALSE {
		o.val.runtime.typeErrorResult(throw, "Dynamic object field %q cannot be made non-enumerable", name.String())
		return false
	}
	if descr.Configurable == FLAG_FALSE {
		o.val.runtime.typeErrorResult(throw, "Dynamic object field %q cannot be made non-configurable", name.String())
		return false
	}
	return true
}

func (o *dynamicObject) defineOwnPropertyStr(name unistring.String, desc PropertyDescriptor, throw bool) bool {
	if o.checkDynamicObjectPropertyDescr(name, desc, throw) {
		return o._set(name.String(), desc.Value, throw)
	}
	return false
}

func (o *dynamicObject) defineOwnPropertyIdx(name valueInt, desc PropertyDescriptor, throw bool) bool {
	if o.checkDynamicObjectPropertyDescr(name, desc, throw) {
		return o._set(name.String(), desc.Value, throw)
	}
	return false
}

func (o *baseDynamicObject) defineOwnPropertySym(name *Symbol, desc PropertyDescriptor, throw bool) bool {
	o._setSym(throw)
	return false
}

func (o *dynamicObject) _delete(prop string, throw bool) bool {
	if o.d.Delete(prop) {
		return true
	}
	o.val.runtime.typeErrorResult(throw, "Could not delete property %q of a dynamic object", prop)
	return false
}

func (o *dynamicObject) deleteStr(name unistring.String, throw bool) bool {
	return o._delete(name.String(), throw)
}

func (o *dynamicObject) deleteIdx(idx valueInt, throw bool) bool {
	return o._delete(idx.String(), throw)
}

func (*baseDynamicObject) deleteSym(_ *Symbol, _ bool) bool {
	return true
}

func (o *baseDynamicObject) toPrimitiveNumber() Value {
	return o.val.genericToPrimitiveNumber()
}

func (o *baseDynamicObject) toPrimitiveString() Value {
	return o.val.genericToPrimitiveString()
}

func (o *baseDynamicObject) toPrimitive() Value {
	return o.val.genericToPrimitive()
}

func (o *baseDynamicObject) assertCallable() (call func(FunctionCall) Value, ok bool) {
	return nil, false
}

func (*baseDynamicObject) assertConstructor() func(args []Value, newTarget *Object) *Object {
	return nil
}

func (o *baseDynamicObject) proto() *Object {
	return o.prototype
}

func (o *baseDynamicObject) setProto(proto *Object, throw bool) bool {
	o.prototype = proto
	return true
}

func (o *baseDynamicObject) hasInstance(v Value) bool {
	panic(o.val.runtime.NewTypeError("Expecting a function in instanceof check, but got a dynamic object"))
}

func (*baseDynamicObject) isExtensible() bool {
	return true
}

func (o *baseDynamicObject) preventExtensions(throw bool) bool {
	o.val.runtime.typeErrorResult(throw, "Cannot make a dynamic object non-extensible")
	return false
}

type dynamicObjectPropIter struct {
	o         *dynamicObject
	propNames []string
	idx       int
}

func (i *dynamicObjectPropIter) next() (propIterItem, iterNextFunc) {
	for i.idx < len(i.propNames) {
		name := i.propNames[i.idx]
		i.idx++
		if i.o.d.Has(name) {
			return propIterItem{name: newStringValue(name), enumerable: _ENUM_TRUE}, i.next
		}
	}
	return propIterItem{}, nil
}

func (o *dynamicObject) iterateStringKeys() iterNextFunc {
	keys := o.d.Keys()
	return (&dynamicObjectPropIter{
		o:         o,
		propNames: keys,
	}).next
}

func (o *baseDynamicObject) iterateSymbols() iterNextFunc {
	return func() (propIterItem, iterNextFunc) {
		return propIterItem{}, nil
	}
}

func (o *dynamicObject) iterateKeys() iterNextFunc {
	return o.iterateStringKeys()
}

func (o *dynamicObject) export(ctx *objectExportCtx) interface{} {
	return o.d
}

func (o *dynamicObject) exportType() reflect.Type {
	return reflect.TypeOf(o.d)
}

func (o *baseDynamicObject) exportToMap(dst reflect.Value, typ reflect.Type, ctx *objectExportCtx) error {
	return genericExportToMap(o.val, dst, typ, ctx)
}

func (o *baseDynamicObject) exportToArrayOrSlice(dst reflect.Value, typ reflect.Type, ctx *objectExportCtx) error {
	return genericExportToArrayOrSlice(o.val, dst, typ, ctx)
}

func (o *dynamicObject) equal(impl objectImpl) bool {
	if other, ok := impl.(*dynamicObject); ok {
		return o.d == other.d
	}
	return false
}

func (o *dynamicObject) stringKeys(all bool, accum []Value) []Value {
	keys := o.d.Keys()
	if l := len(accum) + len(keys); l > cap(accum) {
		oldAccum := accum
		accum = make([]Value, len(accum), l)
		copy(accum, oldAccum)
	}
	for _, key := range keys {
		accum = append(accum, newStringValue(key))
	}
	return accum
}

func (*baseDynamicObject) symbols(all bool, accum []Value) []Value {
	return accum
}

func (o *dynamicObject) keys(all bool, accum []Value) []Value {
	return o.stringKeys(all, accum)
}

func (*baseDynamicObject) _putProp(name unistring.String, value Value, writable, enumerable, configurable bool) Value {
	return nil
}

func (*baseDynamicObject) _putSym(s *Symbol, prop Value) {
}

func (a *dynamicArray) sortLen() int64 {
	return int64(a.a.Len())
}

func (a *dynamicArray) sortGet(i int64) Value {
	return a.a.Get(int(i))
}

func (a *dynamicArray) swap(i int64, j int64) {
	x := a.sortGet(i)
	y := a.sortGet(j)
	a.a.Set(int(i), y)
	a.a.Set(int(j), x)
}

func (a *dynamicArray) className() string {
	return classArray
}

func (a *dynamicArray) getStr(p unistring.String, receiver Value) Value {
	if p == "length" {
		return intToValue(int64(a.a.Len()))
	}
	if idx, ok := strToInt(p); ok {
		return a.a.Get(idx)
	}
	return a.getParentStr(p, receiver)
}

func (a *dynamicArray) getIdx(p valueInt, receiver Value) Value {
	if val := a.getOwnPropIdx(p); val != nil {
		return val
	}
	return a.getParentIdx(p, receiver)
}

func (a *dynamicArray) getOwnPropStr(u unistring.String) Value {
	if u == "length" {
		return &valueProperty{
			value:    intToValue(int64(a.a.Len())),
			writable: true,
		}
	}
	if idx, ok := strToInt(u); ok {
		return a.a.Get(idx)
	}
	return nil
}

func (a *dynamicArray) getOwnPropIdx(v valueInt) Value {
	return a.a.Get(toIntStrict(int64(v)))
}

func (a *dynamicArray) _setLen(v Value, throw bool) bool {
	if a.a.SetLen(toIntStrict(v.ToInteger())) {
		return true
	}
	a.val.runtime.typeErrorResult(throw, "'SetLen' on a dynamic array returned false")
	return false
}

func (a *dynamicArray) setOwnStr(p unistring.String, v Value, throw bool) bool {
	if p == "length" {
		return a._setLen(v, throw)
	}
	if idx, ok := strToInt(p); ok {
		return a._setIdx(idx, v, throw)
	}
	a.val.runtime.typeErrorResult(throw, "Cannot set property %q on a dynamic array", p.String())
	return false
}

func (a *dynamicArray) _setIdx(idx int, v Value, throw bool) bool {
	if a.a.Set(idx, v) {
		return true
	}
	a.val.runtime.typeErrorResult(throw, "'Set' on a dynamic array returned false")
	return false
}

func (a *dynamicArray) setOwnIdx(p valueInt, v Value, throw bool) bool {
	return a._setIdx(toIntStrict(int64(p)), v, throw)
}

func (a *dynamicArray) setForeignStr(p unistring.String, v, receiver Value, throw bool) (res bool, handled bool) {
	return a.setParentForeignStr(p, v, receiver, throw)
}

func (a *dynamicArray) setForeignIdx(p valueInt, v, receiver Value, throw bool) (res bool, handled bool) {
	return a.setParentForeignIdx(p, v, receiver, throw)
}

func (a *dynamicArray) hasPropertyStr(u unistring.String) bool {
	if a.hasOwnPropertyStr(u) {
		return true
	}
	if proto := a.prototype; proto != nil {
		return proto.self.hasPropertyStr(u)
	}
	return false
}

func (a *dynamicArray) hasPropertyIdx(idx valueInt) bool {
	if a.hasOwnPropertyIdx(idx) {
		return true
	}
	if proto := a.prototype; proto != nil {
		return proto.self.hasPropertyIdx(idx)
	}
	return false
}

func (a *dynamicArray) _has(idx int) bool {
	return idx >= 0 && idx < a.a.Len()
}

func (a *dynamicArray) hasOwnPropertyStr(u unistring.String) bool {
	if u == "length" {
		return true
	}
	if idx, ok := strToInt(u); ok {
		return a._has(idx)
	}
	return false
}

func (a *dynamicArray) hasOwnPropertyIdx(v valueInt) bool {
	return a._has(toIntStrict(int64(v)))
}

func (a *dynamicArray) defineOwnPropertyStr(name unistring.String, desc PropertyDescriptor, throw bool) bool {
	if a.checkDynamicObjectPropertyDescr(name, desc, throw) {
		if idx, ok := strToInt(name); ok {
			return a._setIdx(idx, desc.Value, throw)
		}
		a.val.runtime.typeErrorResult(throw, "Cannot define property %q on a dynamic array", name.String())
	}
	return false
}

func (a *dynamicArray) defineOwnPropertyIdx(name valueInt, desc PropertyDescriptor, throw bool) bool {
	if a.checkDynamicObjectPropertyDescr(name, desc, throw) {
		return a._setIdx(toIntStrict(int64(name)), desc.Value, throw)
	}
	return false
}

func (a *dynamicArray) _delete(idx int, throw bool) bool {
	if a._has(idx) {
		a._setIdx(idx, _undefined, throw)
	}
	return true
}

func (a *dynamicArray) deleteStr(name unistring.String, throw bool) bool {
	if idx, ok := strToInt(name); ok {
		return a._delete(idx, throw)
	}
	if a.hasOwnPropertyStr(name) {
		a.val.runtime.typeErrorResult(throw, "Cannot delete property %q on a dynamic array", name.String())
		return false
	}
	return true
}

func (a *dynamicArray) deleteIdx(idx valueInt, throw bool) bool {
	return a._delete(toIntStrict(int64(idx)), throw)
}

type dynArrayPropIter struct {
	a          DynamicArray
	idx, limit int
}

func (i *dynArrayPropIter) next() (propIterItem, iterNextFunc) {
	if i.idx < i.limit && i.idx < i.a.Len() {
		name := strconv.Itoa(i.idx)
		i.idx++
		return propIterItem{name: asciiString(name), enumerable: _ENUM_TRUE}, i.next
	}

	return propIterItem{}, nil
}

func (a *dynamicArray) iterateStringKeys() iterNextFunc {
	return (&dynArrayPropIter{
		a:     a.a,
		limit: a.a.Len(),
	}).next
}

func (a *dynamicArray) iterateKeys() iterNextFunc {
	return a.iterateStringKeys()
}

func (a *dynamicArray) export(ctx *objectExportCtx) interface{} {
	return a.a
}

func (a *dynamicArray) exportType() reflect.Type {
	return reflect.TypeOf(a.a)
}

func (a *dynamicArray) equal(impl objectImpl) bool {
	if other, ok := impl.(*dynamicArray); ok {
		return a == other
	}
	return false
}

func (a *dynamicArray) stringKeys(all bool, accum []Value) []Value {
	al := a.a.Len()
	l := len(accum) + al
	if all {
		l++
	}
	if l > cap(accum) {
		oldAccum := accum
		accum = make([]Value, len(oldAccum), l)
		copy(accum, oldAccum)
	}
	for i := 0; i < al; i++ {
		accum = append(accum, asciiString(strconv.Itoa(i)))
	}
	if all {
		accum = append(accum, asciiString("length"))
	}
	return accum
}

func (a *dynamicArray) keys(all bool, accum []Value) []Value {
	return a.stringKeys(all, accum)
}
