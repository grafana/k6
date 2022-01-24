package goja

import (
	"github.com/dop251/goja/unistring"
	"reflect"
)

type destructKeyedSource struct {
	r        *Runtime
	wrapped  Value
	usedKeys map[Value]struct{}
}

func newDestructKeyedSource(r *Runtime, wrapped Value) *destructKeyedSource {
	return &destructKeyedSource{
		r:       r,
		wrapped: wrapped,
	}
}

func (r *Runtime) newDestructKeyedSource(wrapped Value) *Object {
	return &Object{
		runtime: r,
		self:    newDestructKeyedSource(r, wrapped),
	}
}

func (d *destructKeyedSource) w() objectImpl {
	return d.wrapped.ToObject(d.r).self
}

func (d *destructKeyedSource) recordKey(key Value) {
	if d.usedKeys == nil {
		d.usedKeys = make(map[Value]struct{})
	}
	d.usedKeys[key] = struct{}{}
}

func (d *destructKeyedSource) sortLen() int64 {
	return d.w().sortLen()
}

func (d *destructKeyedSource) sortGet(i int64) Value {
	return d.w().sortGet(i)
}

func (d *destructKeyedSource) swap(i int64, i2 int64) {
	d.w().swap(i, i2)
}

func (d *destructKeyedSource) className() string {
	return d.w().className()
}

func (d *destructKeyedSource) getStr(p unistring.String, receiver Value) Value {
	d.recordKey(stringValueFromRaw(p))
	return d.w().getStr(p, receiver)
}

func (d *destructKeyedSource) getIdx(p valueInt, receiver Value) Value {
	d.recordKey(p.toString())
	return d.w().getIdx(p, receiver)
}

func (d *destructKeyedSource) getSym(p *Symbol, receiver Value) Value {
	d.recordKey(p)
	return d.w().getSym(p, receiver)
}

func (d *destructKeyedSource) getOwnPropStr(u unistring.String) Value {
	d.recordKey(stringValueFromRaw(u))
	return d.w().getOwnPropStr(u)
}

func (d *destructKeyedSource) getOwnPropIdx(v valueInt) Value {
	d.recordKey(v.toString())
	return d.w().getOwnPropIdx(v)
}

func (d *destructKeyedSource) getOwnPropSym(symbol *Symbol) Value {
	d.recordKey(symbol)
	return d.w().getOwnPropSym(symbol)
}

func (d *destructKeyedSource) setOwnStr(p unistring.String, v Value, throw bool) bool {
	return d.w().setOwnStr(p, v, throw)
}

func (d *destructKeyedSource) setOwnIdx(p valueInt, v Value, throw bool) bool {
	return d.w().setOwnIdx(p, v, throw)
}

func (d *destructKeyedSource) setOwnSym(p *Symbol, v Value, throw bool) bool {
	return d.w().setOwnSym(p, v, throw)
}

func (d *destructKeyedSource) setForeignStr(p unistring.String, v, receiver Value, throw bool) (res bool, handled bool) {
	return d.w().setForeignStr(p, v, receiver, throw)
}

func (d *destructKeyedSource) setForeignIdx(p valueInt, v, receiver Value, throw bool) (res bool, handled bool) {
	return d.w().setForeignIdx(p, v, receiver, throw)
}

func (d *destructKeyedSource) setForeignSym(p *Symbol, v, receiver Value, throw bool) (res bool, handled bool) {
	return d.w().setForeignSym(p, v, receiver, throw)
}

func (d *destructKeyedSource) hasPropertyStr(u unistring.String) bool {
	return d.w().hasPropertyStr(u)
}

func (d *destructKeyedSource) hasPropertyIdx(idx valueInt) bool {
	return d.w().hasPropertyIdx(idx)
}

func (d *destructKeyedSource) hasPropertySym(s *Symbol) bool {
	return d.w().hasPropertySym(s)
}

func (d *destructKeyedSource) hasOwnPropertyStr(u unistring.String) bool {
	return d.w().hasOwnPropertyStr(u)
}

func (d *destructKeyedSource) hasOwnPropertyIdx(v valueInt) bool {
	return d.w().hasOwnPropertyIdx(v)
}

func (d *destructKeyedSource) hasOwnPropertySym(s *Symbol) bool {
	return d.w().hasOwnPropertySym(s)
}

func (d *destructKeyedSource) defineOwnPropertyStr(name unistring.String, desc PropertyDescriptor, throw bool) bool {
	return d.w().defineOwnPropertyStr(name, desc, throw)
}

func (d *destructKeyedSource) defineOwnPropertyIdx(name valueInt, desc PropertyDescriptor, throw bool) bool {
	return d.w().defineOwnPropertyIdx(name, desc, throw)
}

func (d *destructKeyedSource) defineOwnPropertySym(name *Symbol, desc PropertyDescriptor, throw bool) bool {
	return d.w().defineOwnPropertySym(name, desc, throw)
}

func (d *destructKeyedSource) deleteStr(name unistring.String, throw bool) bool {
	return d.w().deleteStr(name, throw)
}

func (d *destructKeyedSource) deleteIdx(idx valueInt, throw bool) bool {
	return d.w().deleteIdx(idx, throw)
}

func (d *destructKeyedSource) deleteSym(s *Symbol, throw bool) bool {
	return d.w().deleteSym(s, throw)
}

func (d *destructKeyedSource) toPrimitiveNumber() Value {
	return d.w().toPrimitiveNumber()
}

func (d *destructKeyedSource) toPrimitiveString() Value {
	return d.w().toPrimitiveString()
}

func (d *destructKeyedSource) toPrimitive() Value {
	return d.w().toPrimitive()
}

func (d *destructKeyedSource) assertCallable() (call func(FunctionCall) Value, ok bool) {
	return d.w().assertCallable()
}

func (d *destructKeyedSource) assertConstructor() func(args []Value, newTarget *Object) *Object {
	return d.w().assertConstructor()
}

func (d *destructKeyedSource) proto() *Object {
	return d.w().proto()
}

func (d *destructKeyedSource) setProto(proto *Object, throw bool) bool {
	return d.w().setProto(proto, throw)
}

func (d *destructKeyedSource) hasInstance(v Value) bool {
	return d.w().hasInstance(v)
}

func (d *destructKeyedSource) isExtensible() bool {
	return d.w().isExtensible()
}

func (d *destructKeyedSource) preventExtensions(throw bool) bool {
	return d.w().preventExtensions(throw)
}

type destructKeyedSourceIter struct {
	d       *destructKeyedSource
	wrapped iterNextFunc
}

func (i *destructKeyedSourceIter) next() (propIterItem, iterNextFunc) {
	for {
		item, next := i.wrapped()
		if next == nil {
			return item, nil
		}
		i.wrapped = next
		if _, exists := i.d.usedKeys[item.name]; !exists {
			return item, i.next
		}
	}
}

func (d *destructKeyedSource) iterateStringKeys() iterNextFunc {
	return (&destructKeyedSourceIter{
		d:       d,
		wrapped: d.w().iterateStringKeys(),
	}).next
}

func (d *destructKeyedSource) iterateSymbols() iterNextFunc {
	return (&destructKeyedSourceIter{
		d:       d,
		wrapped: d.w().iterateSymbols(),
	}).next
}

func (d *destructKeyedSource) iterateKeys() iterNextFunc {
	return (&destructKeyedSourceIter{
		d:       d,
		wrapped: d.w().iterateKeys(),
	}).next
}

func (d *destructKeyedSource) export(ctx *objectExportCtx) interface{} {
	return d.w().export(ctx)
}

func (d *destructKeyedSource) exportType() reflect.Type {
	return d.w().exportType()
}

func (d *destructKeyedSource) exportToMap(dst reflect.Value, typ reflect.Type, ctx *objectExportCtx) error {
	return d.w().exportToMap(dst, typ, ctx)
}

func (d *destructKeyedSource) exportToArrayOrSlice(dst reflect.Value, typ reflect.Type, ctx *objectExportCtx) error {
	return d.w().exportToArrayOrSlice(dst, typ, ctx)
}

func (d *destructKeyedSource) equal(impl objectImpl) bool {
	return d.w().equal(impl)
}

func (d *destructKeyedSource) stringKeys(all bool, accum []Value) []Value {
	var next iterNextFunc
	if all {
		next = d.iterateStringKeys()
	} else {
		next = (&enumerableIter{
			o:       d.wrapped.ToObject(d.r),
			wrapped: d.iterateStringKeys(),
		}).next
	}
	for item, next := next(); next != nil; item, next = next() {
		accum = append(accum, item.name)
	}
	return accum
}

func (d *destructKeyedSource) filterUsedKeys(keys []Value) []Value {
	k := 0
	for i, key := range keys {
		if _, exists := d.usedKeys[key]; exists {
			continue
		}
		if k != i {
			keys[k] = key
		}
		k++
	}
	return keys[:k]
}

func (d *destructKeyedSource) symbols(all bool, accum []Value) []Value {
	return d.filterUsedKeys(d.w().symbols(all, accum))
}

func (d *destructKeyedSource) keys(all bool, accum []Value) []Value {
	return d.filterUsedKeys(d.w().keys(all, accum))
}

func (d *destructKeyedSource) _putProp(name unistring.String, value Value, writable, enumerable, configurable bool) Value {
	return d.w()._putProp(name, value, writable, enumerable, configurable)
}

func (d *destructKeyedSource) _putSym(s *Symbol, prop Value) {
	d.w()._putSym(s, prop)
}
