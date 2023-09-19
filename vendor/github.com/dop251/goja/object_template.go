package goja

import (
	"fmt"
	"github.com/dop251/goja/unistring"
	"math"
	"reflect"
	"sort"
)

type templatePropFactory func(*Runtime) Value

type objectTemplate struct {
	propNames []unistring.String
	props     map[unistring.String]templatePropFactory

	symProps     map[*Symbol]templatePropFactory
	symPropNames []*Symbol

	protoFactory func(*Runtime) *Object
}

type templatedObject struct {
	baseObject
	tmpl *objectTemplate

	protoMaterialised bool
}

type templatedFuncObject struct {
	templatedObject

	f         func(FunctionCall) Value
	construct func(args []Value, newTarget *Object) *Object
}

// This type exists because Array.prototype is supposed to be an array itself and I could not find
// a different way of implementing it without either introducing another layer of interfaces or hoisting
// the templates to baseObject both of which would have had a negative effect on the performance.
// The implementation is as simple as possible and is not optimised in any way, but I very much doubt anybody
// uses Array.prototype as an actual array.
type templatedArrayObject struct {
	templatedObject
}

func newObjectTemplate() *objectTemplate {
	return &objectTemplate{
		props: make(map[unistring.String]templatePropFactory),
	}
}

func (t *objectTemplate) putStr(name unistring.String, f templatePropFactory) {
	t.props[name] = f
	t.propNames = append(t.propNames, name)
}

func (t *objectTemplate) putSym(s *Symbol, f templatePropFactory) {
	if t.symProps == nil {
		t.symProps = make(map[*Symbol]templatePropFactory)
	}
	t.symProps[s] = f
	t.symPropNames = append(t.symPropNames, s)
}

func (r *Runtime) newTemplatedObject(tmpl *objectTemplate, obj *Object) *templatedObject {
	if obj == nil {
		obj = &Object{runtime: r}
	}
	o := &templatedObject{
		baseObject: baseObject{
			class:      classObject,
			val:        obj,
			extensible: true,
		},
		tmpl: tmpl,
	}
	obj.self = o
	o.init()
	return o
}

func (o *templatedObject) materialiseProto() {
	if !o.protoMaterialised {
		if o.tmpl.protoFactory != nil {
			o.prototype = o.tmpl.protoFactory(o.val.runtime)
		}
		o.protoMaterialised = true
	}
}

func (o *templatedObject) getStr(name unistring.String, receiver Value) Value {
	ownProp := o.getOwnPropStr(name)
	if ownProp == nil {
		o.materialiseProto()
	}
	return o.getStrWithOwnProp(ownProp, name, receiver)
}

func (o *templatedObject) getSym(s *Symbol, receiver Value) Value {
	ownProp := o.getOwnPropSym(s)
	if ownProp == nil {
		o.materialiseProto()
	}
	return o.getWithOwnProp(ownProp, s, receiver)
}

func (o *templatedObject) getOwnPropStr(p unistring.String) Value {
	if v, exists := o.values[p]; exists {
		return v
	}
	if f := o.tmpl.props[p]; f != nil {
		v := f(o.val.runtime)
		o.values[p] = v
		return v
	}
	return nil
}

func (o *templatedObject) materialiseSymbols() {
	if o.symValues == nil {
		o.symValues = newOrderedMap(nil)
		for _, p := range o.tmpl.symPropNames {
			o.symValues.set(p, o.tmpl.symProps[p](o.val.runtime))
		}
	}
}

func (o *templatedObject) getOwnPropSym(s *Symbol) Value {
	if o.symValues == nil && o.tmpl.symProps[s] == nil {
		return nil
	}
	o.materialiseSymbols()
	return o.baseObject.getOwnPropSym(s)
}

func (o *templatedObject) materialisePropNames() {
	if o.propNames == nil {
		o.propNames = append(([]unistring.String)(nil), o.tmpl.propNames...)
	}
}

func (o *templatedObject) setOwnStr(p unistring.String, v Value, throw bool) bool {
	existing := o.getOwnPropStr(p) // materialise property (in case it's an accessor)
	if existing == nil {
		o.materialiseProto()
		o.materialisePropNames()
	}
	return o.baseObject.setOwnStr(p, v, throw)
}

func (o *templatedObject) setOwnSym(name *Symbol, val Value, throw bool) bool {
	o.materialiseSymbols()
	o.materialiseProto()
	return o.baseObject.setOwnSym(name, val, throw)
}

func (o *templatedObject) setForeignStr(name unistring.String, val, receiver Value, throw bool) (bool, bool) {
	ownProp := o.getOwnPropStr(name)
	if ownProp == nil {
		o.materialiseProto()
	}
	return o._setForeignStr(name, ownProp, val, receiver, throw)
}

func (o *templatedObject) proto() *Object {
	o.materialiseProto()
	return o.prototype
}

func (o *templatedObject) setProto(proto *Object, throw bool) bool {
	o.protoMaterialised = true
	ret := o.baseObject.setProto(proto, throw)
	if ret {
		o.protoMaterialised = true
	}
	return ret
}

func (o *templatedObject) setForeignIdx(name valueInt, val, receiver Value, throw bool) (bool, bool) {
	return o.setForeignStr(name.string(), val, receiver, throw)
}

func (o *templatedObject) setForeignSym(name *Symbol, val, receiver Value, throw bool) (bool, bool) {
	o.materialiseProto()
	o.materialiseSymbols()
	return o.baseObject.setForeignSym(name, val, receiver, throw)
}

func (o *templatedObject) hasPropertyStr(name unistring.String) bool {
	if o.val.self.hasOwnPropertyStr(name) {
		return true
	}
	o.materialiseProto()
	if o.prototype != nil {
		return o.prototype.self.hasPropertyStr(name)
	}
	return false
}

func (o *templatedObject) hasPropertySym(s *Symbol) bool {
	if o.hasOwnPropertySym(s) {
		return true
	}
	o.materialiseProto()
	if o.prototype != nil {
		return o.prototype.self.hasPropertySym(s)
	}
	return false
}

func (o *templatedObject) hasOwnPropertyStr(name unistring.String) bool {
	if v, exists := o.values[name]; exists {
		return v != nil
	}

	_, exists := o.tmpl.props[name]
	return exists
}

func (o *templatedObject) hasOwnPropertySym(s *Symbol) bool {
	if o.symValues != nil {
		return o.symValues.has(s)
	}
	_, exists := o.tmpl.symProps[s]
	return exists
}

func (o *templatedObject) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	existingVal := o.getOwnPropStr(name)
	if v, ok := o._defineOwnProperty(name, existingVal, descr, throw); ok {
		o.values[name] = v
		if existingVal == nil {
			o.materialisePropNames()
			names := copyNamesIfNeeded(o.propNames, 1)
			o.propNames = append(names, name)
		}
		return true
	}
	return false
}

func (o *templatedObject) defineOwnPropertySym(s *Symbol, descr PropertyDescriptor, throw bool) bool {
	o.materialiseSymbols()
	return o.baseObject.defineOwnPropertySym(s, descr, throw)
}

func (o *templatedObject) deleteStr(name unistring.String, throw bool) bool {
	if val := o.getOwnPropStr(name); val != nil {
		if !o.checkDelete(name, val, throw) {
			return false
		}
		o.materialisePropNames()
		o._delete(name)
		if _, exists := o.tmpl.props[name]; exists {
			o.values[name] = nil // white hole
		}
	}
	return true
}

func (o *templatedObject) deleteSym(s *Symbol, throw bool) bool {
	o.materialiseSymbols()
	return o.baseObject.deleteSym(s, throw)
}

func (o *templatedObject) materialiseProps() {
	for name, f := range o.tmpl.props {
		if _, exists := o.values[name]; !exists {
			o.values[name] = f(o.val.runtime)
		}
	}
	o.materialisePropNames()
}

func (o *templatedObject) iterateStringKeys() iterNextFunc {
	o.materialiseProps()
	return o.baseObject.iterateStringKeys()
}

func (o *templatedObject) iterateSymbols() iterNextFunc {
	o.materialiseSymbols()
	return o.baseObject.iterateSymbols()
}

func (o *templatedObject) stringKeys(all bool, keys []Value) []Value {
	if all {
		o.materialisePropNames()
	} else {
		o.materialiseProps()
	}
	return o.baseObject.stringKeys(all, keys)
}

func (o *templatedObject) symbols(all bool, accum []Value) []Value {
	o.materialiseSymbols()
	return o.baseObject.symbols(all, accum)
}

func (o *templatedObject) keys(all bool, accum []Value) []Value {
	return o.symbols(all, o.stringKeys(all, accum))
}

func (r *Runtime) newTemplatedFuncObject(tmpl *objectTemplate, obj *Object, f func(FunctionCall) Value, ctor func([]Value, *Object) *Object) *templatedFuncObject {
	if obj == nil {
		obj = &Object{runtime: r}
	}
	o := &templatedFuncObject{
		templatedObject: templatedObject{
			baseObject: baseObject{
				class:      classFunction,
				val:        obj,
				extensible: true,
			},
			tmpl: tmpl,
		},
		f:         f,
		construct: ctor,
	}
	obj.self = o
	o.init()
	return o
}

func (f *templatedFuncObject) source() String {
	return newStringValue(fmt.Sprintf("function %s() { [native code] }", nilSafe(f.getStr("name", nil)).toString()))
}

func (f *templatedFuncObject) export(*objectExportCtx) interface{} {
	return f.f
}

func (f *templatedFuncObject) assertCallable() (func(FunctionCall) Value, bool) {
	if f.f != nil {
		return f.f, true
	}
	return nil, false
}

func (f *templatedFuncObject) vmCall(vm *vm, n int) {
	var nf nativeFuncObject
	nf.f = f.f
	nf.vmCall(vm, n)
}

func (f *templatedFuncObject) assertConstructor() func(args []Value, newTarget *Object) *Object {
	return f.construct
}

func (f *templatedFuncObject) exportType() reflect.Type {
	return reflectTypeFunc
}

func (f *templatedFuncObject) typeOf() String {
	return stringFunction
}

func (f *templatedFuncObject) hasInstance(v Value) bool {
	return hasInstance(f.val, v)
}

func (r *Runtime) newTemplatedArrayObject(tmpl *objectTemplate, obj *Object) *templatedArrayObject {
	if obj == nil {
		obj = &Object{runtime: r}
	}
	o := &templatedArrayObject{
		templatedObject: templatedObject{
			baseObject: baseObject{
				class:      classArray,
				val:        obj,
				extensible: true,
			},
			tmpl: tmpl,
		},
	}
	obj.self = o
	o.init()
	return o
}

func (a *templatedArrayObject) getLenProp() *valueProperty {
	lenProp, _ := a.getOwnPropStr("length").(*valueProperty)
	if lenProp == nil {
		panic(a.val.runtime.NewTypeError("missing length property"))
	}
	return lenProp
}

func (a *templatedArrayObject) _setOwnIdx(idx uint32) {
	lenProp := a.getLenProp()
	l := uint32(lenProp.value.ToInteger())
	if idx >= l {
		lenProp.value = intToValue(int64(idx) + 1)
	}
}

func (a *templatedArrayObject) setLength(l uint32, throw bool) bool {
	lenProp := a.getLenProp()
	oldLen := uint32(lenProp.value.ToInteger())
	if l == oldLen {
		return true
	}
	if !lenProp.writable {
		a.val.runtime.typeErrorResult(throw, "length is not writable")
		return false
	}
	ret := true
	if l < oldLen {
		a.materialisePropNames()
		a.fixPropOrder()
		i := sort.Search(a.idxPropCount, func(idx int) bool {
			return strToArrayIdx(a.propNames[idx]) >= l
		})
		for j := a.idxPropCount - 1; j >= i; j-- {
			if !a.deleteStr(a.propNames[j], false) {
				l = strToArrayIdx(a.propNames[j]) + 1
				ret = false
				break
			}
		}
	}
	lenProp.value = intToValue(int64(l))
	return ret
}

func (a *templatedArrayObject) setOwnStr(name unistring.String, value Value, throw bool) bool {
	if name == "length" {
		return a.setLength(a.val.runtime.toLengthUint32(value), throw)
	}
	if !a.templatedObject.setOwnStr(name, value, throw) {
		return false
	}
	if idx := strToArrayIdx(name); idx != math.MaxUint32 {
		a._setOwnIdx(idx)
	}
	return true
}

func (a *templatedArrayObject) setOwnIdx(p valueInt, v Value, throw bool) bool {
	if !a.templatedObject.setOwnStr(p.string(), v, throw) {
		return false
	}
	if idx := toIdx(p); idx != math.MaxUint32 {
		a._setOwnIdx(idx)
	}
	return true
}

func (a *templatedArrayObject) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	if name == "length" {
		return a.val.runtime.defineArrayLength(a.getLenProp(), descr, a.setLength, throw)
	}
	if !a.templatedObject.defineOwnPropertyStr(name, descr, throw) {
		return false
	}
	if idx := strToArrayIdx(name); idx != math.MaxUint32 {
		a._setOwnIdx(idx)
	}
	return true
}

func (a *templatedArrayObject) defineOwnPropertyIdx(p valueInt, desc PropertyDescriptor, throw bool) bool {
	if !a.templatedObject.defineOwnPropertyStr(p.string(), desc, throw) {
		return false
	}
	if idx := toIdx(p); idx != math.MaxUint32 {
		a._setOwnIdx(idx)
	}
	return true
}
