package goja

import (
	"reflect"

	"github.com/dop251/goja/unistring"
)

type lazyObject struct {
	val    *Object
	create func(*Object) objectImpl
}

func (o *lazyObject) className() string {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.className()
}

func (o *lazyObject) getIdx(p valueInt, receiver Value) Value {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.getIdx(p, receiver)
}

func (o *lazyObject) getSym(p *Symbol, receiver Value) Value {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.getSym(p, receiver)
}

func (o *lazyObject) getOwnPropIdx(idx valueInt) Value {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.getOwnPropIdx(idx)
}

func (o *lazyObject) getOwnPropSym(s *Symbol) Value {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.getOwnPropSym(s)
}

func (o *lazyObject) hasPropertyIdx(idx valueInt) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.hasPropertyIdx(idx)
}

func (o *lazyObject) hasPropertySym(s *Symbol) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.hasPropertySym(s)
}

func (o *lazyObject) hasOwnPropertyIdx(idx valueInt) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.hasOwnPropertyIdx(idx)
}

func (o *lazyObject) hasOwnPropertySym(s *Symbol) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.hasOwnPropertySym(s)
}

func (o *lazyObject) defineOwnPropertyStr(name unistring.String, desc PropertyDescriptor, throw bool) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.defineOwnPropertyStr(name, desc, throw)
}

func (o *lazyObject) defineOwnPropertyIdx(name valueInt, desc PropertyDescriptor, throw bool) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.defineOwnPropertyIdx(name, desc, throw)
}

func (o *lazyObject) defineOwnPropertySym(name *Symbol, desc PropertyDescriptor, throw bool) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.defineOwnPropertySym(name, desc, throw)
}

func (o *lazyObject) deleteIdx(idx valueInt, throw bool) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.deleteIdx(idx, throw)
}

func (o *lazyObject) deleteSym(s *Symbol, throw bool) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.deleteSym(s, throw)
}

func (o *lazyObject) getStr(name unistring.String, receiver Value) Value {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.getStr(name, receiver)
}

func (o *lazyObject) getOwnPropStr(name unistring.String) Value {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.getOwnPropStr(name)
}

func (o *lazyObject) setOwnStr(p unistring.String, v Value, throw bool) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.setOwnStr(p, v, throw)
}

func (o *lazyObject) setOwnIdx(p valueInt, v Value, throw bool) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.setOwnIdx(p, v, throw)
}

func (o *lazyObject) setOwnSym(p *Symbol, v Value, throw bool) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.setOwnSym(p, v, throw)
}

func (o *lazyObject) setForeignStr(p unistring.String, v, receiver Value, throw bool) (bool, bool) {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.setForeignStr(p, v, receiver, throw)
}

func (o *lazyObject) setForeignIdx(p valueInt, v, receiver Value, throw bool) (bool, bool) {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.setForeignIdx(p, v, receiver, throw)
}

func (o *lazyObject) setForeignSym(p *Symbol, v, receiver Value, throw bool) (bool, bool) {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.setForeignSym(p, v, receiver, throw)
}

func (o *lazyObject) hasPropertyStr(name unistring.String) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.hasPropertyStr(name)
}

func (o *lazyObject) hasOwnPropertyStr(name unistring.String) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.hasOwnPropertyStr(name)
}

func (o *lazyObject) _putProp(unistring.String, Value, bool, bool, bool) Value {
	panic("cannot use _putProp() in lazy object")
}

func (o *lazyObject) _putSym(*Symbol, Value) {
	panic("cannot use _putSym() in lazy object")
}

func (o *lazyObject) toPrimitiveNumber() Value {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.toPrimitiveNumber()
}

func (o *lazyObject) toPrimitiveString() Value {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.toPrimitiveString()
}

func (o *lazyObject) toPrimitive() Value {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.toPrimitive()
}

func (o *lazyObject) assertCallable() (call func(FunctionCall) Value, ok bool) {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.assertCallable()
}

func (o *lazyObject) assertConstructor() func(args []Value, newTarget *Object) *Object {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.assertConstructor()
}

func (o *lazyObject) deleteStr(name unistring.String, throw bool) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.deleteStr(name, throw)
}

func (o *lazyObject) proto() *Object {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.proto()
}

func (o *lazyObject) hasInstance(v Value) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.hasInstance(v)
}

func (o *lazyObject) isExtensible() bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.isExtensible()
}

func (o *lazyObject) preventExtensions(throw bool) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.preventExtensions(throw)
}

func (o *lazyObject) iterateStringKeys() iterNextFunc {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.iterateStringKeys()
}

func (o *lazyObject) iterateSymbols() iterNextFunc {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.iterateSymbols()
}

func (o *lazyObject) iterateKeys() iterNextFunc {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.iterateKeys()
}

func (o *lazyObject) export(ctx *objectExportCtx) interface{} {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.export(ctx)
}

func (o *lazyObject) exportType() reflect.Type {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.exportType()
}

func (o *lazyObject) exportToMap(m reflect.Value, typ reflect.Type, ctx *objectExportCtx) error {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.exportToMap(m, typ, ctx)
}

func (o *lazyObject) exportToArrayOrSlice(s reflect.Value, typ reflect.Type, ctx *objectExportCtx) error {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.exportToArrayOrSlice(s, typ, ctx)
}

func (o *lazyObject) equal(other objectImpl) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.equal(other)
}

func (o *lazyObject) stringKeys(all bool, accum []Value) []Value {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.stringKeys(all, accum)
}

func (o *lazyObject) symbols(all bool, accum []Value) []Value {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.symbols(all, accum)
}

func (o *lazyObject) keys(all bool, accum []Value) []Value {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.keys(all, accum)
}

func (o *lazyObject) setProto(proto *Object, throw bool) bool {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.setProto(proto, throw)
}

func (o *lazyObject) sortLen() int64 {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.sortLen()
}

func (o *lazyObject) sortGet(i int64) Value {
	obj := o.create(o.val)
	o.val.self = obj
	return obj.sortGet(i)
}

func (o *lazyObject) swap(i, j int64) {
	obj := o.create(o.val)
	o.val.self = obj
	obj.swap(i, j)
}
