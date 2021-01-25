package goja

import (
	"fmt"
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
	return r._newProxyObject(target, &jsProxyHandler{handler: handler}, proto)
}

func (r *Runtime) _newProxyObject(target *Object, handler proxyHandler, proto *Object) *proxyObject {
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

func (p Proxy) Handler() *Object {
	if handler := p.proxy.handler; handler != nil {
		return handler.toObject(p.proxy.val.runtime)
	}
	return nil
}

func (p Proxy) Target() *Object {
	return p.proxy.target
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

type proxyHandler interface {
	getPrototypeOf(target *Object) (Value, bool)
	setPrototypeOf(target *Object, proto *Object) (bool, bool)
	isExtensible(target *Object) (bool, bool)
	preventExtensions(target *Object) (bool, bool)

	getOwnPropertyDescriptorStr(target *Object, prop unistring.String) (Value, bool)
	getOwnPropertyDescriptorIdx(target *Object, prop valueInt) (Value, bool)
	getOwnPropertyDescriptorSym(target *Object, prop *Symbol) (Value, bool)

	definePropertyStr(target *Object, prop unistring.String, desc PropertyDescriptor) (bool, bool)
	definePropertyIdx(target *Object, prop valueInt, desc PropertyDescriptor) (bool, bool)
	definePropertySym(target *Object, prop *Symbol, desc PropertyDescriptor) (bool, bool)

	hasStr(target *Object, prop unistring.String) (bool, bool)
	hasIdx(target *Object, prop valueInt) (bool, bool)
	hasSym(target *Object, prop *Symbol) (bool, bool)

	getStr(target *Object, prop unistring.String, receiver Value) (Value, bool)
	getIdx(target *Object, prop valueInt, receiver Value) (Value, bool)
	getSym(target *Object, prop *Symbol, receiver Value) (Value, bool)

	setStr(target *Object, prop unistring.String, value Value, receiver Value) (bool, bool)
	setIdx(target *Object, prop valueInt, value Value, receiver Value) (bool, bool)
	setSym(target *Object, prop *Symbol, value Value, receiver Value) (bool, bool)

	deleteStr(target *Object, prop unistring.String) (bool, bool)
	deleteIdx(target *Object, prop valueInt) (bool, bool)
	deleteSym(target *Object, prop *Symbol) (bool, bool)

	ownKeys(target *Object) (*Object, bool)
	apply(target *Object, this Value, args []Value) (Value, bool)
	construct(target *Object, args []Value, newTarget *Object) (Value, bool)

	toObject(*Runtime) *Object
}

type jsProxyHandler struct {
	handler *Object
}

func (h *jsProxyHandler) toObject(*Runtime) *Object {
	return h.handler
}

func (h *jsProxyHandler) proxyCall(trap proxyTrap, args ...Value) (Value, bool) {
	r := h.handler.runtime

	if m := toMethod(r.getVStr(h.handler, unistring.String(trap.String()))); m != nil {
		return m(FunctionCall{
			This:      h.handler,
			Arguments: args,
		}), true
	}

	return nil, false
}

func (h *jsProxyHandler) boolProxyCall(trap proxyTrap, args ...Value) (bool, bool) {
	if v, ok := h.proxyCall(trap, args...); ok {
		return v.ToBoolean(), true
	}
	return false, false
}

func (h *jsProxyHandler) getPrototypeOf(target *Object) (Value, bool) {
	return h.proxyCall(proxy_trap_getPrototypeOf, target)
}

func (h *jsProxyHandler) setPrototypeOf(target *Object, proto *Object) (bool, bool) {
	var protoVal Value
	if proto != nil {
		protoVal = proto
	} else {
		protoVal = _null
	}
	return h.boolProxyCall(proxy_trap_setPrototypeOf, target, protoVal)
}

func (h *jsProxyHandler) isExtensible(target *Object) (bool, bool) {
	return h.boolProxyCall(proxy_trap_isExtensible, target)
}

func (h *jsProxyHandler) preventExtensions(target *Object) (bool, bool) {
	return h.boolProxyCall(proxy_trap_preventExtensions, target)
}

func (h *jsProxyHandler) getOwnPropertyDescriptorStr(target *Object, prop unistring.String) (Value, bool) {
	return h.proxyCall(proxy_trap_getOwnPropertyDescriptor, target, stringValueFromRaw(prop))
}

func (h *jsProxyHandler) getOwnPropertyDescriptorIdx(target *Object, prop valueInt) (Value, bool) {
	return h.proxyCall(proxy_trap_getOwnPropertyDescriptor, target, prop.toString())
}

func (h *jsProxyHandler) getOwnPropertyDescriptorSym(target *Object, prop *Symbol) (Value, bool) {
	return h.proxyCall(proxy_trap_getOwnPropertyDescriptor, target, prop)
}

func (h *jsProxyHandler) definePropertyStr(target *Object, prop unistring.String, desc PropertyDescriptor) (bool, bool) {
	return h.boolProxyCall(proxy_trap_defineProperty, target, stringValueFromRaw(prop), desc.toValue(h.handler.runtime))
}

func (h *jsProxyHandler) definePropertyIdx(target *Object, prop valueInt, desc PropertyDescriptor) (bool, bool) {
	return h.boolProxyCall(proxy_trap_defineProperty, target, prop.toString(), desc.toValue(h.handler.runtime))
}

func (h *jsProxyHandler) definePropertySym(target *Object, prop *Symbol, desc PropertyDescriptor) (bool, bool) {
	return h.boolProxyCall(proxy_trap_defineProperty, target, prop, desc.toValue(h.handler.runtime))
}

func (h *jsProxyHandler) hasStr(target *Object, prop unistring.String) (bool, bool) {
	return h.boolProxyCall(proxy_trap_has, target, stringValueFromRaw(prop))
}

func (h *jsProxyHandler) hasIdx(target *Object, prop valueInt) (bool, bool) {
	return h.boolProxyCall(proxy_trap_has, target, prop.toString())
}

func (h *jsProxyHandler) hasSym(target *Object, prop *Symbol) (bool, bool) {
	return h.boolProxyCall(proxy_trap_has, target, prop)
}

func (h *jsProxyHandler) getStr(target *Object, prop unistring.String, receiver Value) (Value, bool) {
	return h.proxyCall(proxy_trap_get, target, stringValueFromRaw(prop), receiver)
}

func (h *jsProxyHandler) getIdx(target *Object, prop valueInt, receiver Value) (Value, bool) {
	return h.proxyCall(proxy_trap_get, target, prop.toString(), receiver)
}

func (h *jsProxyHandler) getSym(target *Object, prop *Symbol, receiver Value) (Value, bool) {
	return h.proxyCall(proxy_trap_get, target, prop, receiver)
}

func (h *jsProxyHandler) setStr(target *Object, prop unistring.String, value Value, receiver Value) (bool, bool) {
	return h.boolProxyCall(proxy_trap_set, target, stringValueFromRaw(prop), value, receiver)
}

func (h *jsProxyHandler) setIdx(target *Object, prop valueInt, value Value, receiver Value) (bool, bool) {
	return h.boolProxyCall(proxy_trap_set, target, prop.toString(), value, receiver)
}

func (h *jsProxyHandler) setSym(target *Object, prop *Symbol, value Value, receiver Value) (bool, bool) {
	return h.boolProxyCall(proxy_trap_set, target, prop, value, receiver)
}

func (h *jsProxyHandler) deleteStr(target *Object, prop unistring.String) (bool, bool) {
	return h.boolProxyCall(proxy_trap_deleteProperty, target, stringValueFromRaw(prop))
}

func (h *jsProxyHandler) deleteIdx(target *Object, prop valueInt) (bool, bool) {
	return h.boolProxyCall(proxy_trap_deleteProperty, target, prop.toString())
}

func (h *jsProxyHandler) deleteSym(target *Object, prop *Symbol) (bool, bool) {
	return h.boolProxyCall(proxy_trap_deleteProperty, target, prop)
}

func (h *jsProxyHandler) ownKeys(target *Object) (*Object, bool) {
	if v, ok := h.proxyCall(proxy_trap_ownKeys, target); ok {
		return h.handler.runtime.toObject(v), true
	}
	return nil, false
}

func (h *jsProxyHandler) apply(target *Object, this Value, args []Value) (Value, bool) {
	return h.proxyCall(proxy_trap_apply, target, this, h.handler.runtime.newArrayValues(args))
}

func (h *jsProxyHandler) construct(target *Object, args []Value, newTarget *Object) (Value, bool) {
	return h.proxyCall(proxy_trap_construct, target, h.handler.runtime.newArrayValues(args), newTarget)
}

type proxyObject struct {
	baseObject
	target  *Object
	handler proxyHandler
	call    func(FunctionCall) Value
	ctor    func(args []Value, newTarget *Object) *Object
}

func (p *proxyObject) checkHandler() proxyHandler {
	r := p.val.runtime
	if handler := p.handler; handler != nil {
		return handler
	}
	panic(r.NewTypeError("Proxy already revoked"))
}

func (p *proxyObject) proto() *Object {
	target := p.target
	if v, ok := p.checkHandler().getPrototypeOf(target); ok {
		var handlerProto *Object
		if v != _null {
			handlerProto = p.val.runtime.toObject(v)
		}
		if !target.self.isExtensible() && !p.__sameValue(handlerProto, target.self.proto()) {
			panic(p.val.runtime.NewTypeError("'getPrototypeOf' on proxy: proxy target is non-extensible but the trap did not return its actual prototype"))
		}
		return handlerProto
	}

	return target.self.proto()
}

func (p *proxyObject) setProto(proto *Object, throw bool) bool {
	target := p.target
	if v, ok := p.checkHandler().setPrototypeOf(target, proto); ok {
		if v {
			if !target.self.isExtensible() && !p.__sameValue(proto, target.self.proto()) {
				panic(p.val.runtime.NewTypeError("'setPrototypeOf' on proxy: trap returned truish for setting a new prototype on the non-extensible proxy target"))
			}
			return true
		} else {
			p.val.runtime.typeErrorResult(throw, "'setPrototypeOf' on proxy: trap returned falsish")
			return false
		}
	}

	return target.self.setProto(proto, throw)
}

func (p *proxyObject) isExtensible() bool {
	target := p.target
	if booleanTrapResult, ok := p.checkHandler().isExtensible(p.target); ok {
		if te := target.self.isExtensible(); booleanTrapResult != te {
			panic(p.val.runtime.NewTypeError("'isExtensible' on proxy: trap result does not reflect extensibility of proxy target (which is '%v')", te))
		}
		return booleanTrapResult
	}

	return target.self.isExtensible()
}

func (p *proxyObject) preventExtensions(throw bool) bool {
	target := p.target
	if booleanTrapResult, ok := p.checkHandler().preventExtensions(target); ok {
		if !booleanTrapResult {
			p.val.runtime.typeErrorResult(throw, "'preventExtensions' on proxy: trap returned falsish")
			return false
		}
		if te := target.self.isExtensible(); booleanTrapResult && te {
			panic(p.val.runtime.NewTypeError("'preventExtensions' on proxy: trap returned truish but the proxy target is extensible"))
		}
	}

	return target.self.preventExtensions(throw)
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

func (p *proxyObject) proxyDefineOwnPropertyPreCheck(trapResult, throw bool) bool {
	if !trapResult {
		p.val.runtime.typeErrorResult(throw, "'defineProperty' on proxy: trap returned falsish")
		return false
	}
	return true
}

func (p *proxyObject) proxyDefineOwnPropertyPostCheck(prop Value, target *Object, descr PropertyDescriptor) {
	targetDesc := propToValueProp(prop)
	extensibleTarget := target.self.isExtensible()
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
		if targetDesc.value != nil && !targetDesc.configurable && targetDesc.writable {
			if descr.Writable == FLAG_FALSE {
				panic(p.val.runtime.NewTypeError())
			}
		}
	}
}

func (p *proxyObject) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	target := p.target
	if booleanTrapResult, ok := p.checkHandler().definePropertyStr(target, name, descr); ok {
		if !p.proxyDefineOwnPropertyPreCheck(booleanTrapResult, throw) {
			return false
		}
		p.proxyDefineOwnPropertyPostCheck(target.self.getOwnPropStr(name), target, descr)
		return true
	}
	return target.self.defineOwnPropertyStr(name, descr, throw)
}

func (p *proxyObject) defineOwnPropertyIdx(idx valueInt, descr PropertyDescriptor, throw bool) bool {
	target := p.target
	if booleanTrapResult, ok := p.checkHandler().definePropertyIdx(target, idx, descr); ok {
		if !p.proxyDefineOwnPropertyPreCheck(booleanTrapResult, throw) {
			return false
		}
		p.proxyDefineOwnPropertyPostCheck(target.self.getOwnPropIdx(idx), target, descr)
		return true
	}

	return target.self.defineOwnPropertyIdx(idx, descr, throw)
}

func (p *proxyObject) defineOwnPropertySym(s *Symbol, descr PropertyDescriptor, throw bool) bool {
	target := p.target
	if booleanTrapResult, ok := p.checkHandler().definePropertySym(target, s, descr); ok {
		if !p.proxyDefineOwnPropertyPreCheck(booleanTrapResult, throw) {
			return false
		}
		p.proxyDefineOwnPropertyPostCheck(target.self.getOwnPropSym(s), target, descr)
		return true
	}

	return target.self.defineOwnPropertySym(s, descr, throw)
}

func (p *proxyObject) proxyHasChecks(targetProp Value, target *Object, name fmt.Stringer) {
	targetDesc := propToValueProp(targetProp)
	if targetDesc != nil {
		if !targetDesc.configurable {
			panic(p.val.runtime.NewTypeError("'has' on proxy: trap returned falsish for property '%s' which exists in the proxy target as non-configurable", name.String()))
		}
		if !target.self.isExtensible() {
			panic(p.val.runtime.NewTypeError("'has' on proxy: trap returned falsish for property '%s' but the proxy target is not extensible", name.String()))
		}
	}
}

func (p *proxyObject) hasPropertyStr(name unistring.String) bool {
	target := p.target
	if b, ok := p.checkHandler().hasStr(target, name); ok {
		if !b {
			p.proxyHasChecks(target.self.getOwnPropStr(name), target, name)
		}
		return b
	}

	return target.self.hasPropertyStr(name)
}

func (p *proxyObject) hasPropertyIdx(idx valueInt) bool {
	target := p.target
	if b, ok := p.checkHandler().hasIdx(target, idx); ok {
		if !b {
			p.proxyHasChecks(target.self.getOwnPropIdx(idx), target, idx)
		}
		return b
	}

	return target.self.hasPropertyIdx(idx)
}

func (p *proxyObject) hasPropertySym(s *Symbol) bool {
	target := p.target
	if b, ok := p.checkHandler().hasSym(target, s); ok {
		if !b {
			p.proxyHasChecks(target.self.getOwnPropSym(s), target, s)
		}
		return b
	}

	return target.self.hasPropertySym(s)
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

func (p *proxyObject) proxyGetOwnPropertyDescriptor(targetProp Value, target *Object, trapResult Value, name fmt.Stringer) Value {
	r := p.val.runtime
	targetDesc := propToValueProp(targetProp)
	var trapResultObj *Object
	if trapResult != nil && trapResult != _undefined {
		if obj, ok := trapResult.(*Object); ok {
			trapResultObj = obj
		} else {
			panic(r.NewTypeError("'getOwnPropertyDescriptor' on proxy: trap returned neither object nor undefined for property '%s'", name.String()))
		}
	}
	if trapResultObj == nil {
		if targetDesc == nil {
			return nil
		}
		if !targetDesc.configurable {
			panic(r.NewTypeError())
		}
		if !target.self.isExtensible() {
			panic(r.NewTypeError())
		}
		return nil
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

		if resultDesc.Writable == FLAG_FALSE && targetDesc.writable {
			panic(r.NewTypeError("'getOwnPropertyDescriptor' on proxy: trap reported non-configurable and writable for property '%s' which is non-configurable, non-writable in the proxy target", name.String()))
		}
	}

	if resultDesc.Writable == FLAG_TRUE && resultDesc.Configurable == FLAG_TRUE &&
		resultDesc.Enumerable == FLAG_TRUE {
		return resultDesc.Value
	}
	return r.toValueProp(trapResultObj)
}

func (p *proxyObject) getOwnPropStr(name unistring.String) Value {
	target := p.target
	if v, ok := p.checkHandler().getOwnPropertyDescriptorStr(target, name); ok {
		return p.proxyGetOwnPropertyDescriptor(target.self.getOwnPropStr(name), target, v, name)
	}

	return target.self.getOwnPropStr(name)
}

func (p *proxyObject) getOwnPropIdx(idx valueInt) Value {
	target := p.target
	if v, ok := p.checkHandler().getOwnPropertyDescriptorIdx(target, idx); ok {
		return p.proxyGetOwnPropertyDescriptor(target.self.getOwnPropIdx(idx), target, v, idx)
	}

	return target.self.getOwnPropIdx(idx)
}

func (p *proxyObject) getOwnPropSym(s *Symbol) Value {
	target := p.target
	if v, ok := p.checkHandler().getOwnPropertyDescriptorSym(target, s); ok {
		return p.proxyGetOwnPropertyDescriptor(target.self.getOwnPropSym(s), target, v, s)
	}

	return target.self.getOwnPropSym(s)
}

func (p *proxyObject) proxyGetChecks(targetProp, trapResult Value, name fmt.Stringer) {
	if targetDesc, ok := targetProp.(*valueProperty); ok {
		if !targetDesc.accessor {
			if !targetDesc.writable && !targetDesc.configurable && !trapResult.SameAs(targetDesc.value) {
				panic(p.val.runtime.NewTypeError("'get' on proxy: property '%s' is a read-only and non-configurable data property on the proxy target but the proxy did not return its actual value (expected '%s' but got '%s')", name.String(), nilSafe(targetDesc.value), ret))
			}
		} else {
			if !targetDesc.configurable && targetDesc.getterFunc == nil && trapResult != _undefined {
				panic(p.val.runtime.NewTypeError("'get' on proxy: property '%s' is a non-configurable accessor property on the proxy target and does not have a getter function, but the trap did not return 'undefined' (got '%s')", name.String(), ret))
			}
		}
	}
}

func (p *proxyObject) getStr(name unistring.String, receiver Value) Value {
	target := p.target
	if receiver == nil {
		receiver = p.val
	}
	if v, ok := p.checkHandler().getStr(target, name, receiver); ok {
		p.proxyGetChecks(target.self.getOwnPropStr(name), v, name)
		return v
	}
	return target.self.getStr(name, receiver)
}

func (p *proxyObject) getIdx(idx valueInt, receiver Value) Value {
	target := p.target
	if receiver == nil {
		receiver = p.val
	}
	if v, ok := p.checkHandler().getIdx(target, idx, receiver); ok {
		p.proxyGetChecks(target.self.getOwnPropIdx(idx), v, idx)
		return v
	}
	return target.self.getIdx(idx, receiver)
}

func (p *proxyObject) getSym(s *Symbol, receiver Value) Value {
	target := p.target
	if receiver == nil {
		receiver = p.val
	}
	if v, ok := p.checkHandler().getSym(target, s, receiver); ok {
		p.proxyGetChecks(target.self.getOwnPropSym(s), v, s)
		return v
	}

	return target.self.getSym(s, receiver)
}

func (p *proxyObject) proxySetPreCheck(trapResult, throw bool, name fmt.Stringer) bool {
	if !trapResult {
		p.val.runtime.typeErrorResult(throw, "'set' on proxy: trap returned falsish for property '%s'", name.String())
	}
	return trapResult
}

func (p *proxyObject) proxySetPostCheck(targetProp, value Value, name fmt.Stringer) {
	if prop, ok := targetProp.(*valueProperty); ok {
		if prop.accessor {
			if !prop.configurable && prop.setterFunc == nil {
				panic(p.val.runtime.NewTypeError("'set' on proxy: trap returned truish for property '%s' which exists in the proxy target as a non-configurable and non-writable accessor property without a setter", name.String()))
			}
		} else if !prop.configurable && !prop.writable && !p.__sameValue(prop.value, value) {
			panic(p.val.runtime.NewTypeError("'set' on proxy: trap returned truish for property '%s' which exists in the proxy target as a non-configurable and non-writable data property with a different value", name.String()))
		}
	}
}

func (p *proxyObject) proxySetStr(name unistring.String, value, receiver Value, throw bool) bool {
	target := p.target
	if v, ok := p.checkHandler().setStr(target, name, value, receiver); ok {
		if p.proxySetPreCheck(v, throw, name) {
			p.proxySetPostCheck(target.self.getOwnPropStr(name), value, name)
			return true
		}
		return false
	}
	return target.setStr(name, value, receiver, throw)
}

func (p *proxyObject) proxySetIdx(idx valueInt, value, receiver Value, throw bool) bool {
	target := p.target
	if v, ok := p.checkHandler().setIdx(target, idx, value, receiver); ok {
		if p.proxySetPreCheck(v, throw, idx) {
			p.proxySetPostCheck(target.self.getOwnPropIdx(idx), value, idx)
			return true
		}
		return false
	}
	return target.setIdx(idx, value, receiver, throw)
}

func (p *proxyObject) proxySetSym(s *Symbol, value, receiver Value, throw bool) bool {
	target := p.target
	if v, ok := p.checkHandler().setSym(target, s, value, receiver); ok {
		if p.proxySetPreCheck(v, throw, s) {
			p.proxySetPostCheck(target.self.getOwnPropSym(s), value, s)
			return true
		}
		return false
	}
	return target.setSym(s, value, receiver, throw)
}

func (p *proxyObject) setOwnStr(name unistring.String, v Value, throw bool) bool {
	return p.proxySetStr(name, v, p.val, throw)
}

func (p *proxyObject) setOwnIdx(idx valueInt, v Value, throw bool) bool {
	return p.proxySetIdx(idx, v, p.val, throw)
}

func (p *proxyObject) setOwnSym(s *Symbol, v Value, throw bool) bool {
	return p.proxySetSym(s, v, p.val, throw)
}

func (p *proxyObject) setForeignStr(name unistring.String, v, receiver Value, throw bool) (bool, bool) {
	return p.proxySetStr(name, v, receiver, throw), true
}

func (p *proxyObject) setForeignIdx(idx valueInt, v, receiver Value, throw bool) (bool, bool) {
	return p.proxySetIdx(idx, v, receiver, throw), true
}

func (p *proxyObject) setForeignSym(s *Symbol, v, receiver Value, throw bool) (bool, bool) {
	return p.proxySetSym(s, v, receiver, throw), true
}

func (p *proxyObject) proxyDeleteCheck(trapResult bool, targetProp Value, name fmt.Stringer, target *Object) {
	if trapResult {
		if targetProp == nil {
			return
		}
		if targetDesc, ok := targetProp.(*valueProperty); ok {
			if !targetDesc.configurable {
				panic(p.val.runtime.NewTypeError("'deleteProperty' on proxy: property '%s' is a non-configurable property but the trap returned truish", name.String()))
			}
		}
		if !target.self.isExtensible() {
			panic(p.val.runtime.NewTypeError("'deleteProperty' on proxy: trap returned truish for property '%s' but the proxy target is non-extensible", name.String()))
		}
	}
}

func (p *proxyObject) deleteStr(name unistring.String, throw bool) bool {
	target := p.target
	if v, ok := p.checkHandler().deleteStr(target, name); ok {
		p.proxyDeleteCheck(v, target.self.getOwnPropStr(name), name, target)
		return v
	}

	return target.self.deleteStr(name, throw)
}

func (p *proxyObject) deleteIdx(idx valueInt, throw bool) bool {
	target := p.target
	if v, ok := p.checkHandler().deleteIdx(target, idx); ok {
		p.proxyDeleteCheck(v, target.self.getOwnPropIdx(idx), idx, target)
		return v
	}

	return target.self.deleteIdx(idx, throw)
}

func (p *proxyObject) deleteSym(s *Symbol, throw bool) bool {
	target := p.target
	if v, ok := p.checkHandler().deleteSym(target, s); ok {
		p.proxyDeleteCheck(v, target.self.getOwnPropSym(s), s, target)
		return v
	}

	return target.self.deleteSym(s, throw)
}

func (p *proxyObject) ownPropertyKeys(all bool, _ []Value) []Value {
	if v, ok := p.proxyOwnKeys(); ok {
		return v
	}
	return p.target.self.ownPropertyKeys(all, nil)
}

func (p *proxyObject) proxyOwnKeys() ([]Value, bool) {
	target := p.target
	if v, ok := p.checkHandler().ownKeys(target); ok {
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
			if _, exists := keySet[item]; exists {
				panic(p.val.runtime.NewTypeError("'ownKeys' on proxy: trap returned duplicate entries"))
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

func (p *proxyObject) enumerateOwnKeys() iterNextFunc {
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
	if v, ok := p.checkHandler().apply(p.target, nilSafe(call.This), call.Arguments); ok {
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
	if v, ok := p.checkHandler().construct(p.target, args, newTarget); ok {
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
