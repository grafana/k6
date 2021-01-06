package goja

import (
	"reflect"

	"github.com/dop251/goja/unistring"
)

// Proxy is a Go wrapper around ECMAScript Proxy. Calling Runtime.ToValue() on it
// returns the underlying Proxy. Calling Export() on an ECMAScript Proxy returns a wrapper.
// Use Runtime.NewProxy() to create one.
type Proxy struct {
	proxy *proxyObject
}

var (
	proxyType = reflect.TypeOf(Proxy{})
)

type proxyPropIter struct {
	p     *proxyObject
	names []Value
	idx   int
}

func (i *proxyPropIter) next() (propIterItem, iterNextFunc) {
	for i.idx < len(i.names) {
		name := i.names[i.idx]
		i.idx++
		if prop := i.p.val.getOwnProp(name); prop != nil {
			return propIterItem{name: name.string(), value: prop}, i.next
		}
	}
	if proto := i.p.proto(); proto != nil {
		return proto.self.enumerateUnfiltered()()
	}
	return propIterItem{}, nil
}

func (r *Runtime) newProxyObject(target, handler, proto *Object) *proxyObject {
	if p, ok := target.self.(*proxyObject); ok {
		if p.handler == nil {
			panic(r.NewTypeError("Cannot create proxy with a revoked proxy as target"))
		}
	}
	if p, ok := handler.self.(*proxyObject); ok {
		if p.handler == nil {
			panic(r.NewTypeError("Cannot create proxy with a revoked proxy as handler"))
		}
	}
	v := &Object{runtime: r}
	p := &proxyObject{}
	v.self = p
	p.val = v
	p.class = classObject
	if proto == nil {
		p.prototype = r.global.ObjectPrototype
	} else {
		p.prototype = proto
	}
	p.extensible = false
	p.init()
	p.target = target
	p.handler = handler
	if call, ok := target.self.assertCallable(); ok {
		p.call = call
	}
	if ctor := target.self.assertConstructor(); ctor != nil {
		p.ctor = ctor
	}
	return p
}

func (p Proxy) Revoke() {
	p.proxy.revoke()
}

func (p Proxy) toValue(r *Runtime) Value {
	if p.proxy == nil {
		return _null
	}
	proxy := p.proxy.val
	if proxy.runtime != r {
		panic(r.NewTypeError("Illegal runtime transition of a Proxy"))
	}
	return proxy
}

type proxyTrap string

const (
	proxy_trap_getPrototypeOf           = "getPrototypeOf"
	proxy_trap_setPrototypeOf           = "setPrototypeOf"
	proxy_trap_isExtensible             = "isExtensible"
	proxy_trap_preventExtensions        = "preventExtensions"
	proxy_trap_getOwnPropertyDescriptor = "getOwnPropertyDescriptor"
	proxy_trap_defineProperty           = "defineProperty"
	proxy_trap_has                      = "has"
	proxy_trap_get                      = "get"
	proxy_trap_set                      = "set"
	proxy_trap_deleteProperty           = "deleteProperty"
	proxy_trap_ownKeys                  = "ownKeys"
	proxy_trap_apply                    = "apply"
	proxy_trap_construct                = "construct"
)

func (p proxyTrap) String() (name string) {
	return string(p)
}

type proxyObject struct {
	baseObject
	target  *Object
	handler *Object
	call    func(FunctionCall) Value
	ctor    func(args []Value, newTarget *Object) *Object
}

func (p *proxyObject) proxyCall(trap proxyTrap, args ...Value) (Value, bool) {
	r := p.val.runtime
	if p.handler == nil {
		panic(r.NewTypeError("Proxy already revoked"))
	}

	if m := toMethod(r.getVStr(p.handler, unistring.String(trap.String()))); m != nil {
		return m(FunctionCall{
			This:      p.handler,
			Arguments: args,
		}), true
	}

	return nil, false
}

func (p *proxyObject) proto() *Object {
	if v, ok := p.proxyCall(proxy_trap_getPrototypeOf, p.target); ok {
		var handlerProto *Object
		if v != _null {
			handlerProto = p.val.runtime.toObject(v)
		}
		if !p.target.self.isExtensible() && !p.__sameValue(handlerProto, p.target.self.proto()) {
			panic(p.val.runtime.NewTypeError("'getPrototypeOf' on proxy: proxy target is non-extensible but the trap did not return its actual prototype"))
		}
		return handlerProto
	}

	return p.target.self.proto()
}

func (p *proxyObject) setProto(proto *Object, throw bool) bool {
	if v, ok := p.proxyCall(proxy_trap_setPrototypeOf, p.target, proto); ok {
		if v.ToBoolean() {
			if !p.target.self.isExtensible() && !p.__sameValue(proto, p.target.self.proto()) {
				panic(p.val.runtime.NewTypeError("'setPrototypeOf' on proxy: trap returned truish for setting a new prototype on the non-extensible proxy target"))
			}
			return true
		} else {
			p.val.runtime.typeErrorResult(throw, "'setPrototypeOf' on proxy: trap returned falsish")
		}
	}

	return p.target.self.setProto(proto, throw)
}

func (p *proxyObject) isExtensible() bool {
	if v, ok := p.proxyCall(proxy_trap_isExtensible, p.target); ok {
		booleanTrapResult := v.ToBoolean()
		if te := p.target.self.isExtensible(); booleanTrapResult != te {
			panic(p.val.runtime.NewTypeError("'isExtensible' on proxy: trap result does not reflect extensibility of proxy target (which is '%v')", te))
		}
		return booleanTrapResult
	}

	return p.target.self.isExtensible()
}

func (p *proxyObject) preventExtensions(throw bool) bool {
	if v, ok := p.proxyCall(proxy_trap_preventExtensions, p.target); ok {
		booleanTrapResult := v.ToBoolean()
		if !booleanTrapResult {
			p.val.runtime.typeErrorResult(throw, "'preventExtensions' on proxy: trap returned falsish")
			return false
		}
		if te := p.target.self.isExtensible(); booleanTrapResult && te {
			panic(p.val.runtime.NewTypeError("'preventExtensions' on proxy: trap returned truish but the proxy target is extensible"))
		}
	}

	return p.target.self.preventExtensions(throw)
}

func propToValueProp(v Value) *valueProperty {
	if v == nil {
		return nil
	}
	if v, ok := v.(*valueProperty); ok {
		return v
	}
	return &valueProperty{
		value:        v,
		writable:     true,
		configurable: true,
		enumerable:   true,
	}
}

func (p *proxyObject) proxyDefineOwnProperty(name Value, descr PropertyDescriptor, throw bool) (bool, bool) {
	if v, ok := p.proxyCall(proxy_trap_defineProperty, p.target, name, descr.toValue(p.val.runtime)); ok {
		booleanTrapResult := v.ToBoolean()
		if !booleanTrapResult {
			p.val.runtime.typeErrorResult(throw, "'defineProperty' on proxy: trap returned falsish")
			return false, true
		}
		targetDesc := propToValueProp(p.target.getOwnProp(name))
		extensibleTarget := p.target.self.isExtensible()
		settingConfigFalse := descr.Configurable == FLAG_FALSE
		if targetDesc == nil {
			if !extensibleTarget {
				panic(p.val.runtime.NewTypeError())
			}
			if settingConfigFalse {
				panic(p.val.runtime.NewTypeError())
			}
		} else {
			if !p.__isCompatibleDescriptor(extensibleTarget, &descr, targetDesc) {
				panic(p.val.runtime.NewTypeError())
			}
			if settingConfigFalse && targetDesc.configurable {
				panic(p.val.runtime.NewTypeError())
			}
		}
		return booleanTrapResult, true
	}
	return false, false
}

func (p *proxyObject) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	if v, ok := p.proxyDefineOwnProperty(stringValueFromRaw(name), descr, throw); ok {
		return v
	}
	return p.target.self.defineOwnPropertyStr(name, descr, throw)
}

func (p *proxyObject) defineOwnPropertyIdx(idx valueInt, descr PropertyDescriptor, throw bool) bool {
	if v, ok := p.proxyDefineOwnProperty(idx, descr, throw); ok {
		return v
	}
	return p.target.self.defineOwnPropertyIdx(idx, descr, throw)
}

func (p *proxyObject) defineOwnPropertySym(s *Symbol, descr PropertyDescriptor, throw bool) bool {
	if v, ok := p.proxyDefineOwnProperty(s, descr, throw); ok {
		return v
	}
	return p.target.self.defineOwnPropertySym(s, descr, throw)
}

func (p *proxyObject) proxyHas(name Value) (bool, bool) {
	if v, ok := p.proxyCall(proxy_trap_has, p.target, name); ok {
		booleanTrapResult := v.ToBoolean()
		if !booleanTrapResult {
			targetDesc := propToValueProp(p.target.getOwnProp(name))
			if targetDesc != nil {
				if !targetDesc.configurable {
					panic(p.val.runtime.NewTypeError("'has' on proxy: trap returned falsish for property '%s' which exists in the proxy target as non-configurable", name.String()))
				}
				if !p.target.self.isExtensible() {
					panic(p.val.runtime.NewTypeError("'has' on proxy: trap returned falsish for property '%s' but the proxy target is not extensible", name.String()))
				}
			}
		}
		return booleanTrapResult, true
	}

	return false, false
}

func (p *proxyObject) hasPropertyStr(name unistring.String) bool {
	if b, ok := p.proxyHas(stringValueFromRaw(name)); ok {
		return b
	}

	return p.target.self.hasPropertyStr(name)
}

func (p *proxyObject) hasPropertyIdx(idx valueInt) bool {
	if b, ok := p.proxyHas(idx); ok {
		return b
	}

	return p.target.self.hasPropertyIdx(idx)
}

func (p *proxyObject) hasPropertySym(s *Symbol) bool {
	if b, ok := p.proxyHas(s); ok {
		return b
	}

	return p.target.self.hasPropertySym(s)
}

func (p *proxyObject) hasOwnPropertyStr(name unistring.String) bool {
	return p.getOwnPropStr(name) != nil
}

func (p *proxyObject) hasOwnPropertyIdx(idx valueInt) bool {
	return p.getOwnPropIdx(idx) != nil
}

func (p *proxyObject) hasOwnPropertySym(s *Symbol) bool {
	return p.getOwnPropSym(s) != nil
}

func (p *proxyObject) proxyGetOwnPropertyDescriptor(name Value) (Value, bool) {
	target := p.target
	if v, ok := p.proxyCall(proxy_trap_getOwnPropertyDescriptor, target, name); ok {
		r := p.val.runtime

		targetDesc := propToValueProp(target.getOwnProp(name))

		var trapResultObj *Object
		if v != nil && v != _undefined {
			if obj, ok := v.(*Object); ok {
				trapResultObj = obj
			} else {
				panic(r.NewTypeError("'getOwnPropertyDescriptor' on proxy: trap returned neither object nor undefined for property '%s'", name.String()))
			}
		}
		if trapResultObj == nil {
			if targetDesc == nil {
				return nil, true
			}
			if !targetDesc.configurable {
				panic(r.NewTypeError())
			}
			if !target.self.isExtensible() {
				panic(r.NewTypeError())
			}
			return nil, true
		}
		extensibleTarget := target.self.isExtensible()
		resultDesc := r.toPropertyDescriptor(trapResultObj)
		resultDesc.complete()
		if !p.__isCompatibleDescriptor(extensibleTarget, &resultDesc, targetDesc) {
			panic(r.NewTypeError("'getOwnPropertyDescriptor' on proxy: trap returned descriptor for property '%s' that is incompatible with the existing property in the proxy target", name.String()))
		}

		if resultDesc.Configurable == FLAG_FALSE {
			if targetDesc == nil {
				panic(r.NewTypeError("'getOwnPropertyDescriptor' on proxy: trap reported non-configurability for property '%s' which is non-existent in the proxy target", name.String()))
			}

			if targetDesc.configurable {
				panic(r.NewTypeError("'getOwnPropertyDescriptor' on proxy: trap reported non-configurability for property '%s' which is configurable in the proxy target", name.String()))
			}
		}

		if resultDesc.Writable == FLAG_TRUE && resultDesc.Configurable == FLAG_TRUE &&
			resultDesc.Enumerable == FLAG_TRUE {
			return resultDesc.Value, true
		}
		return r.toValueProp(trapResultObj), true
	}

	return nil, false
}

func (p *proxyObject) getOwnPropStr(name unistring.String) Value {
	if v, ok := p.proxyGetOwnPropertyDescriptor(stringValueFromRaw(name)); ok {
		return v
	}

	return p.target.self.getOwnPropStr(name)
}

func (p *proxyObject) getOwnPropIdx(idx valueInt) Value {
	if v, ok := p.proxyGetOwnPropertyDescriptor(idx.toString()); ok {
		return v
	}

	return p.target.self.getOwnPropIdx(idx)
}

func (p *proxyObject) getOwnPropSym(s *Symbol) Value {
	if v, ok := p.proxyGetOwnPropertyDescriptor(s); ok {
		return v
	}

	return p.target.self.getOwnPropSym(s)
}

func (p *proxyObject) getStr(name unistring.String, receiver Value) Value {
	if v, ok := p.proxyGet(stringValueFromRaw(name), receiver); ok {
		return v
	}
	return p.target.self.getStr(name, receiver)
}

func (p *proxyObject) getIdx(idx valueInt, receiver Value) Value {
	if v, ok := p.proxyGet(idx.toString(), receiver); ok {
		return v
	}
	return p.target.self.getIdx(idx, receiver)
}

func (p *proxyObject) getSym(s *Symbol, receiver Value) Value {
	if v, ok := p.proxyGet(s, receiver); ok {
		return v
	}
	return p.target.self.getSym(s, receiver)

}

func (p *proxyObject) proxyGet(name, receiver Value) (Value, bool) {
	target := p.target
	if receiver == nil {
		receiver = p.val
	}
	if v, ok := p.proxyCall(proxy_trap_get, target, name, receiver); ok {
		if targetDesc, ok := target.getOwnProp(name).(*valueProperty); ok {
			if !targetDesc.accessor {
				if !targetDesc.writable && !targetDesc.configurable && !v.SameAs(targetDesc.value) {
					panic(p.val.runtime.NewTypeError("'get' on proxy: property '%s' is a read-only and non-configurable data property on the proxy target but the proxy did not return its actual value (expected '%s' but got '%s')", name.String(), nilSafe(targetDesc.value), ret))
				}
			} else {
				if !targetDesc.configurable && targetDesc.getterFunc == nil && v != _undefined {
					panic(p.val.runtime.NewTypeError("'get' on proxy: property '%s' is a non-configurable accessor property on the proxy target and does not have a getter function, but the trap did not return 'undefined' (got '%s')", name.String(), ret))
				}
			}
		}
		return v, true
	}

	return nil, false
}

func (p *proxyObject) proxySet(name, value, receiver Value, throw bool) (bool, bool) {
	target := p.target
	if v, ok := p.proxyCall(proxy_trap_set, target, name, value, receiver); ok {
		if v.ToBoolean() {
			if prop, ok := target.getOwnProp(name).(*valueProperty); ok {
				if prop.accessor {
					if !prop.configurable && prop.setterFunc == nil {
						panic(p.val.runtime.NewTypeError("'set' on proxy: trap returned truish for property '%s' which exists in the proxy target as a non-configurable and non-writable accessor property without a setter", name.String()))
					}
				} else if !prop.configurable && !prop.writable && !p.__sameValue(prop.value, value) {
					panic(p.val.runtime.NewTypeError("'set' on proxy: trap returned truish for property '%s' which exists in the proxy target as a non-configurable and non-writable data property with a different value", name.String()))
				}
			}
			return true, true
		}
		if throw {
			panic(p.val.runtime.NewTypeError("'set' on proxy: trap returned falsish for property '%s'", name.String()))
		}
		return false, true
	}

	return false, false
}

func (p *proxyObject) setOwnStr(name unistring.String, v Value, throw bool) bool {
	if res, ok := p.proxySet(stringValueFromRaw(name), v, p.val, throw); ok {
		return res
	}
	return p.target.setStr(name, v, p.val, throw)
}

func (p *proxyObject) setOwnIdx(idx valueInt, v Value, throw bool) bool {
	if res, ok := p.proxySet(idx.toString(), v, p.val, throw); ok {
		return res
	}
	return p.target.setIdx(idx, v, p.val, throw)
}

func (p *proxyObject) setOwnSym(s *Symbol, v Value, throw bool) bool {
	if res, ok := p.proxySet(s, v, p.val, throw); ok {
		return res
	}
	return p.target.setSym(s, v, p.val, throw)
}

func (p *proxyObject) setForeignStr(name unistring.String, v, receiver Value, throw bool) (bool, bool) {
	if res, ok := p.proxySet(stringValueFromRaw(name), v, receiver, throw); ok {
		return res, true
	}
	return p.target.setStr(name, v, receiver, throw), true
}

func (p *proxyObject) setForeignIdx(idx valueInt, v, receiver Value, throw bool) (bool, bool) {
	if res, ok := p.proxySet(idx.toString(), v, receiver, throw); ok {
		return res, true
	}
	return p.target.setIdx(idx, v, receiver, throw), true
}

func (p *proxyObject) setForeignSym(s *Symbol, v, receiver Value, throw bool) (bool, bool) {
	if res, ok := p.proxySet(s, v, receiver, throw); ok {
		return res, true
	}
	return p.target.setSym(s, v, receiver, throw), true
}

func (p *proxyObject) proxyDelete(n Value) (bool, bool) {
	target := p.target
	if v, ok := p.proxyCall(proxy_trap_deleteProperty, target, n); ok {
		if v.ToBoolean() {
			if targetDesc, ok := target.getOwnProp(n).(*valueProperty); ok {
				if !targetDesc.configurable {
					panic(p.val.runtime.NewTypeError("'deleteProperty' on proxy: property '%s' is a non-configurable property but the trap returned truish", n.String()))
				}
			}
			return true, true
		}
		return false, true
	}
	return false, false
}

func (p *proxyObject) deleteStr(name unistring.String, throw bool) bool {
	if ret, ok := p.proxyDelete(stringValueFromRaw(name)); ok {
		return ret
	}

	return p.target.self.deleteStr(name, throw)
}

func (p *proxyObject) deleteIdx(idx valueInt, throw bool) bool {
	if ret, ok := p.proxyDelete(idx.toString()); ok {
		return ret
	}

	return p.target.self.deleteIdx(idx, throw)
}

func (p *proxyObject) deleteSym(s *Symbol, throw bool) bool {
	if ret, ok := p.proxyDelete(s); ok {
		return ret
	}

	return p.target.self.deleteSym(s, throw)
}

func (p *proxyObject) ownPropertyKeys(all bool, _ []Value) []Value {
	if v, ok := p.proxyOwnKeys(); ok {
		return v
	}
	return p.target.self.ownPropertyKeys(all, nil)
}

func (p *proxyObject) proxyOwnKeys() ([]Value, bool) {
	target := p.target
	if v, ok := p.proxyCall(proxy_trap_ownKeys, p.target); ok {
		keys := p.val.runtime.toObject(v)
		var keyList []Value
		keySet := make(map[Value]struct{})
		l := toLength(keys.self.getStr("length", nil))
		for k := int64(0); k < l; k++ {
			item := keys.self.getIdx(valueInt(k), nil)
			if _, ok := item.(valueString); !ok {
				if _, ok := item.(*Symbol); !ok {
					panic(p.val.runtime.NewTypeError("%s is not a valid property name", item.String()))
				}
			}
			keyList = append(keyList, item)
			keySet[item] = struct{}{}
		}
		ext := target.self.isExtensible()
		for _, itemName := range target.self.ownPropertyKeys(true, nil) {
			if _, exists := keySet[itemName]; exists {
				delete(keySet, itemName)
			} else {
				if !ext {
					panic(p.val.runtime.NewTypeError("'ownKeys' on proxy: trap result did not include '%s'", itemName.String()))
				}
				prop := target.getOwnProp(itemName)
				if prop, ok := prop.(*valueProperty); ok && !prop.configurable {
					panic(p.val.runtime.NewTypeError("'ownKeys' on proxy: trap result did not include non-configurable '%s'", itemName.String()))
				}
			}
		}
		if !ext && len(keyList) > 0 && len(keySet) > 0 {
			panic(p.val.runtime.NewTypeError("'ownKeys' on proxy: trap returned extra keys but proxy target is non-extensible"))
		}

		return keyList, true
	}

	return nil, false
}

func (p *proxyObject) enumerateUnfiltered() iterNextFunc {
	return (&proxyPropIter{
		p:     p,
		names: p.ownKeys(true, nil),
	}).next
}

func (p *proxyObject) assertCallable() (call func(FunctionCall) Value, ok bool) {
	if p.call != nil {
		return func(call FunctionCall) Value {
			return p.apply(call)
		}, true
	}
	return nil, false
}

func (p *proxyObject) assertConstructor() func(args []Value, newTarget *Object) *Object {
	if p.ctor != nil {
		return p.construct
	}
	return nil
}

func (p *proxyObject) apply(call FunctionCall) Value {
	if p.call == nil {
		p.val.runtime.NewTypeError("proxy target is not a function")
	}
	if v, ok := p.proxyCall(proxy_trap_apply, p.target, nilSafe(call.This), p.val.runtime.newArrayValues(call.Arguments)); ok {
		return v
	}
	return p.call(call)
}

func (p *proxyObject) construct(args []Value, newTarget *Object) *Object {
	if p.ctor == nil {
		panic(p.val.runtime.NewTypeError("proxy target is not a constructor"))
	}
	if newTarget == nil {
		newTarget = p.val
	}
	if v, ok := p.proxyCall(proxy_trap_construct, p.target, p.val.runtime.newArrayValues(args), newTarget); ok {
		return p.val.runtime.toObject(v)
	}
	return p.ctor(args, newTarget)
}

func (p *proxyObject) __isCompatibleDescriptor(extensible bool, desc *PropertyDescriptor, current *valueProperty) bool {
	if current == nil {
		return extensible
	}

	/*if desc.Empty() {
		return true
	}*/

	/*if p.__isEquivalentDescriptor(desc, current) {
		return true
	}*/

	if !current.configurable {
		if desc.Configurable == FLAG_TRUE {
			return false
		}

		if desc.Enumerable != FLAG_NOT_SET && desc.Enumerable.Bool() != current.enumerable {
			return false
		}

		if p.__isGenericDescriptor(desc) {
			return true
		}

		if p.__isDataDescriptor(desc) != !current.accessor {
			return desc.Configurable != FLAG_FALSE
		}

		if p.__isDataDescriptor(desc) && !current.accessor {
			if !current.configurable {
				if desc.Writable == FLAG_TRUE && !current.writable {
					return false
				}
				if !current.writable {
					if desc.Value != nil && !desc.Value.SameAs(current.value) {
						return false
					}
				}
			}
			return true
		}
		if p.__isAccessorDescriptor(desc) && current.accessor {
			if !current.configurable {
				if desc.Setter != nil && desc.Setter.SameAs(current.setterFunc) {
					return false
				}
				if desc.Getter != nil && desc.Getter.SameAs(current.getterFunc) {
					return false
				}
			}
		}
	}
	return true
}

func (p *proxyObject) __isAccessorDescriptor(desc *PropertyDescriptor) bool {
	return desc.Setter != nil || desc.Getter != nil
}

func (p *proxyObject) __isDataDescriptor(desc *PropertyDescriptor) bool {
	return desc.Value != nil || desc.Writable != FLAG_NOT_SET
}

func (p *proxyObject) __isGenericDescriptor(desc *PropertyDescriptor) bool {
	return !p.__isAccessorDescriptor(desc) && !p.__isDataDescriptor(desc)
}

func (p *proxyObject) __sameValue(val1, val2 Value) bool {
	if val1 == nil && val2 == nil {
		return true
	}
	if val1 != nil {
		return val1.SameAs(val2)
	}
	return false
}

func (p *proxyObject) filterKeys(vals []Value, all, symbols bool) []Value {
	if !all {
		k := 0
		for i, val := range vals {
			var prop Value
			if symbols {
				if s, ok := val.(*Symbol); ok {
					prop = p.getOwnPropSym(s)
				} else {
					continue
				}
			} else {
				if _, ok := val.(*Symbol); !ok {
					prop = p.getOwnPropStr(val.string())
				} else {
					continue
				}
			}
			if prop == nil {
				continue
			}
			if prop, ok := prop.(*valueProperty); ok && !prop.enumerable {
				continue
			}
			if k != i {
				vals[k] = vals[i]
			}
			k++
		}
		vals = vals[:k]
	} else {
		k := 0
		for i, val := range vals {
			if _, ok := val.(*Symbol); ok != symbols {
				continue
			}
			if k != i {
				vals[k] = vals[i]
			}
			k++
		}
		vals = vals[:k]
	}
	return vals
}

func (p *proxyObject) ownKeys(all bool, _ []Value) []Value { // we can assume accum is empty
	if vals, ok := p.proxyOwnKeys(); ok {
		return p.filterKeys(vals, all, false)
	}

	return p.target.self.ownKeys(all, nil)
}

func (p *proxyObject) ownSymbols(all bool, accum []Value) []Value {
	if vals, ok := p.proxyOwnKeys(); ok {
		res := p.filterKeys(vals, true, true)
		if accum == nil {
			return res
		}
		accum = append(accum, res...)
		return accum
	}

	return p.target.self.ownSymbols(all, accum)
}

func (p *proxyObject) className() string {
	if p.target == nil {
		panic(p.val.runtime.NewTypeError("proxy has been revoked"))
	}
	if p.call != nil || p.ctor != nil {
		return classFunction
	}
	return classObject
}

func (p *proxyObject) exportType() reflect.Type {
	return proxyType
}

func (p *proxyObject) export(*objectExportCtx) interface{} {
	return Proxy{
		proxy: p,
	}
}

func (p *proxyObject) revoke() {
	p.handler = nil
	p.target = nil
}
