package goja

import (
	"github.com/dop251/goja/unistring"
)

type nativeProxyHandler struct {
	handler *ProxyTrapConfig
}

func (h *nativeProxyHandler) getPrototypeOf(target *Object) (Value, bool) {
	if trap := h.handler.GetPrototypeOf; trap != nil {
		return trap(target), true
	}
	return nil, false
}

func (h *nativeProxyHandler) setPrototypeOf(target *Object, proto *Object) (bool, bool) {
	if trap := h.handler.SetPrototypeOf; trap != nil {
		return trap(target, proto), true
	}
	return false, false
}

func (h *nativeProxyHandler) isExtensible(target *Object) (bool, bool) {
	if trap := h.handler.IsExtensible; trap != nil {
		return trap(target), true
	}
	return false, false
}

func (h *nativeProxyHandler) preventExtensions(target *Object) (bool, bool) {
	if trap := h.handler.PreventExtensions; trap != nil {
		return trap(target), true
	}
	return false, false
}

func (h *nativeProxyHandler) getOwnPropertyDescriptorStr(target *Object, prop unistring.String) (Value, bool) {
	if trap := h.handler.GetOwnPropertyDescriptorIdx; trap != nil {
		if idx, ok := strToInt(prop); ok {
			desc := trap(target, idx)
			return desc.toValue(target.runtime), true
		}
	}
	if trap := h.handler.GetOwnPropertyDescriptor; trap != nil {
		desc := trap(target, prop.String())
		return desc.toValue(target.runtime), true
	}
	return nil, false
}

func (h *nativeProxyHandler) getOwnPropertyDescriptorIdx(target *Object, prop valueInt) (Value, bool) {
	if trap := h.handler.GetOwnPropertyDescriptorIdx; trap != nil {
		desc := trap(target, toIntStrict(int64(prop)))
		return desc.toValue(target.runtime), true
	}
	if trap := h.handler.GetOwnPropertyDescriptor; trap != nil {
		desc := trap(target, prop.String())
		return desc.toValue(target.runtime), true
	}
	return nil, false
}

func (h *nativeProxyHandler) getOwnPropertyDescriptorSym(target *Object, prop *Symbol) (Value, bool) {
	if trap := h.handler.GetOwnPropertyDescriptorSym; trap != nil {
		desc := trap(target, prop)
		return desc.toValue(target.runtime), true
	}
	return nil, false
}

func (h *nativeProxyHandler) definePropertyStr(target *Object, prop unistring.String, desc PropertyDescriptor) (bool, bool) {
	if trap := h.handler.DefinePropertyIdx; trap != nil {
		if idx, ok := strToInt(prop); ok {
			return trap(target, idx, desc), true
		}
	}
	if trap := h.handler.DefineProperty; trap != nil {
		return trap(target, prop.String(), desc), true
	}
	return false, false
}

func (h *nativeProxyHandler) definePropertyIdx(target *Object, prop valueInt, desc PropertyDescriptor) (bool, bool) {
	if trap := h.handler.DefinePropertyIdx; trap != nil {
		return trap(target, toIntStrict(int64(prop)), desc), true
	}
	if trap := h.handler.DefineProperty; trap != nil {
		return trap(target, prop.String(), desc), true
	}
	return false, false
}

func (h *nativeProxyHandler) definePropertySym(target *Object, prop *Symbol, desc PropertyDescriptor) (bool, bool) {
	if trap := h.handler.DefinePropertySym; trap != nil {
		return trap(target, prop, desc), true
	}
	return false, false
}

func (h *nativeProxyHandler) hasStr(target *Object, prop unistring.String) (bool, bool) {
	if trap := h.handler.HasIdx; trap != nil {
		if idx, ok := strToInt(prop); ok {
			return trap(target, idx), true
		}
	}
	if trap := h.handler.Has; trap != nil {
		return trap(target, prop.String()), true
	}
	return false, false
}

func (h *nativeProxyHandler) hasIdx(target *Object, prop valueInt) (bool, bool) {
	if trap := h.handler.HasIdx; trap != nil {
		return trap(target, toIntStrict(int64(prop))), true
	}
	if trap := h.handler.Has; trap != nil {
		return trap(target, prop.String()), true
	}
	return false, false
}

func (h *nativeProxyHandler) hasSym(target *Object, prop *Symbol) (bool, bool) {
	if trap := h.handler.HasSym; trap != nil {
		return trap(target, prop), true
	}
	return false, false
}

func (h *nativeProxyHandler) getStr(target *Object, prop unistring.String, receiver Value) (Value, bool) {
	if trap := h.handler.GetIdx; trap != nil {
		if idx, ok := strToInt(prop); ok {
			return trap(target, idx, receiver), true
		}
	}
	if trap := h.handler.Get; trap != nil {
		return trap(target, prop.String(), receiver), true
	}
	return nil, false
}

func (h *nativeProxyHandler) getIdx(target *Object, prop valueInt, receiver Value) (Value, bool) {
	if trap := h.handler.GetIdx; trap != nil {
		return trap(target, toIntStrict(int64(prop)), receiver), true
	}
	if trap := h.handler.Get; trap != nil {
		return trap(target, prop.String(), receiver), true
	}
	return nil, false
}

func (h *nativeProxyHandler) getSym(target *Object, prop *Symbol, receiver Value) (Value, bool) {
	if trap := h.handler.GetSym; trap != nil {
		return trap(target, prop, receiver), true
	}
	return nil, false
}

func (h *nativeProxyHandler) setStr(target *Object, prop unistring.String, value Value, receiver Value) (bool, bool) {
	if trap := h.handler.SetIdx; trap != nil {
		if idx, ok := strToInt(prop); ok {
			return trap(target, idx, value, receiver), true
		}
	}
	if trap := h.handler.Set; trap != nil {
		return trap(target, prop.String(), value, receiver), true
	}
	return false, false
}

func (h *nativeProxyHandler) setIdx(target *Object, prop valueInt, value Value, receiver Value) (bool, bool) {
	if trap := h.handler.SetIdx; trap != nil {
		return trap(target, toIntStrict(int64(prop)), value, receiver), true
	}
	if trap := h.handler.Set; trap != nil {
		return trap(target, prop.String(), value, receiver), true
	}
	return false, false
}

func (h *nativeProxyHandler) setSym(target *Object, prop *Symbol, value Value, receiver Value) (bool, bool) {
	if trap := h.handler.SetSym; trap != nil {
		return trap(target, prop, value, receiver), true
	}
	return false, false
}

func (h *nativeProxyHandler) deleteStr(target *Object, prop unistring.String) (bool, bool) {
	if trap := h.handler.DeletePropertyIdx; trap != nil {
		if idx, ok := strToInt(prop); ok {
			return trap(target, idx), true
		}
	}
	if trap := h.handler.DeleteProperty; trap != nil {
		return trap(target, prop.String()), true
	}
	return false, false
}

func (h *nativeProxyHandler) deleteIdx(target *Object, prop valueInt) (bool, bool) {
	if trap := h.handler.DeletePropertyIdx; trap != nil {
		return trap(target, toIntStrict(int64(prop))), true
	}
	if trap := h.handler.DeleteProperty; trap != nil {
		return trap(target, prop.String()), true
	}
	return false, false
}

func (h *nativeProxyHandler) deleteSym(target *Object, prop *Symbol) (bool, bool) {
	if trap := h.handler.DeletePropertySym; trap != nil {
		return trap(target, prop), true
	}
	return false, false
}

func (h *nativeProxyHandler) ownKeys(target *Object) (*Object, bool) {
	if trap := h.handler.OwnKeys; trap != nil {
		return trap(target), true
	}
	return nil, false
}

func (h *nativeProxyHandler) apply(target *Object, this Value, args []Value) (Value, bool) {
	if trap := h.handler.Apply; trap != nil {
		return trap(target, this, args), true
	}
	return nil, false
}

func (h *nativeProxyHandler) construct(target *Object, args []Value, newTarget *Object) (Value, bool) {
	if trap := h.handler.Construct; trap != nil {
		return trap(target, args, newTarget), true
	}
	return nil, false
}

func (h *nativeProxyHandler) toObject(runtime *Runtime) *Object {
	return runtime.ToValue(h.handler).ToObject(runtime)
}

func (r *Runtime) newNativeProxyHandler(nativeHandler *ProxyTrapConfig) proxyHandler {
	return &nativeProxyHandler{handler: nativeHandler}
}

// ProxyTrapConfig provides a simplified Go-friendly API for implementing Proxy traps.
// If an *Idx trap is defined it gets called for integer property keys, including negative ones. Note that
// this only includes string property keys that represent a canonical integer
// (i.e. "0", "123", but not "00", "01", " 1" or "-0").
// For efficiency strings representing integers exceeding 2^53 are not checked to see if they are canonical,
// i.e. the *Idx traps will receive "9007199254740993" as well as "9007199254740994", even though the former is not
// a canonical representation in ECMAScript (Number("9007199254740993") === 9007199254740992).
// See https://262.ecma-international.org/#sec-canonicalnumericindexstring
// If an *Idx trap is not set, the corresponding string one is used.
type ProxyTrapConfig struct {
	// A trap for Object.getPrototypeOf, Reflect.getPrototypeOf, __proto__, Object.prototype.isPrototypeOf, instanceof
	GetPrototypeOf func(target *Object) (prototype *Object)

	// A trap for Object.setPrototypeOf, Reflect.setPrototypeOf
	SetPrototypeOf func(target *Object, prototype *Object) (success bool)

	// A trap for Object.isExtensible, Reflect.isExtensible
	IsExtensible func(target *Object) (success bool)

	// A trap for Object.preventExtensions, Reflect.preventExtensions
	PreventExtensions func(target *Object) (success bool)

	// A trap for Object.getOwnPropertyDescriptor, Reflect.getOwnPropertyDescriptor (string properties)
	GetOwnPropertyDescriptor func(target *Object, prop string) (propertyDescriptor PropertyDescriptor)

	// A trap for Object.getOwnPropertyDescriptor, Reflect.getOwnPropertyDescriptor (integer properties)
	GetOwnPropertyDescriptorIdx func(target *Object, prop int) (propertyDescriptor PropertyDescriptor)

	// A trap for Object.getOwnPropertyDescriptor, Reflect.getOwnPropertyDescriptor (Symbol properties)
	GetOwnPropertyDescriptorSym func(target *Object, prop *Symbol) (propertyDescriptor PropertyDescriptor)

	// A trap for Object.defineProperty, Reflect.defineProperty (string properties)
	DefineProperty func(target *Object, key string, propertyDescriptor PropertyDescriptor) (success bool)

	// A trap for Object.defineProperty, Reflect.defineProperty (integer properties)
	DefinePropertyIdx func(target *Object, key int, propertyDescriptor PropertyDescriptor) (success bool)

	// A trap for Object.defineProperty, Reflect.defineProperty (Symbol properties)
	DefinePropertySym func(target *Object, key *Symbol, propertyDescriptor PropertyDescriptor) (success bool)

	// A trap for the in operator, with operator, Reflect.has (string properties)
	Has func(target *Object, property string) (available bool)

	// A trap for the in operator, with operator, Reflect.has (integer properties)
	HasIdx func(target *Object, property int) (available bool)

	// A trap for the in operator, with operator, Reflect.has (Symbol properties)
	HasSym func(target *Object, property *Symbol) (available bool)

	// A trap for getting property values, Reflect.get (string properties)
	Get func(target *Object, property string, receiver Value) (value Value)

	// A trap for getting property values, Reflect.get (integer properties)
	GetIdx func(target *Object, property int, receiver Value) (value Value)

	// A trap for getting property values, Reflect.get (Symbol properties)
	GetSym func(target *Object, property *Symbol, receiver Value) (value Value)

	// A trap for setting property values, Reflect.set (string properties)
	Set func(target *Object, property string, value Value, receiver Value) (success bool)

	// A trap for setting property values, Reflect.set (integer properties)
	SetIdx func(target *Object, property int, value Value, receiver Value) (success bool)

	// A trap for setting property values, Reflect.set (Symbol properties)
	SetSym func(target *Object, property *Symbol, value Value, receiver Value) (success bool)

	// A trap for the delete operator, Reflect.deleteProperty (string properties)
	DeleteProperty func(target *Object, property string) (success bool)

	// A trap for the delete operator, Reflect.deleteProperty (integer properties)
	DeletePropertyIdx func(target *Object, property int) (success bool)

	// A trap for the delete operator, Reflect.deleteProperty (Symbol properties)
	DeletePropertySym func(target *Object, property *Symbol) (success bool)

	// A trap for Object.getOwnPropertyNames, Object.getOwnPropertySymbols, Object.keys, Reflect.ownKeys
	OwnKeys func(target *Object) (object *Object)

	// A trap for a function call, Function.prototype.apply, Function.prototype.call, Reflect.apply
	Apply func(target *Object, this Value, argumentsList []Value) (value Value)

	// A trap for the new operator, Reflect.construct
	Construct func(target *Object, argumentsList []Value, newTarget *Object) (value *Object)
}

func (r *Runtime) newProxy(args []Value, proto *Object) *Object {
	if len(args) >= 2 {
		if target, ok := args[0].(*Object); ok {
			if proxyHandler, ok := args[1].(*Object); ok {
				return r.newProxyObject(target, proxyHandler, proto).val
			}
		}
	}
	panic(r.NewTypeError("Cannot create proxy with a non-object as target or handler"))
}

func (r *Runtime) builtin_newProxy(args []Value, newTarget *Object) *Object {
	if newTarget == nil {
		panic(r.needNew("Proxy"))
	}
	return r.newProxy(args, r.getPrototypeFromCtor(newTarget, r.getProxy(), r.global.ObjectPrototype))
}

func (r *Runtime) NewProxy(target *Object, nativeHandler *ProxyTrapConfig) Proxy {
	if p, ok := target.self.(*proxyObject); ok {
		if p.handler == nil {
			panic(r.NewTypeError("Cannot create proxy with a revoked proxy as target"))
		}
	}
	handler := r.newNativeProxyHandler(nativeHandler)
	proxy := r._newProxyObject(target, handler, nil)
	return Proxy{proxy: proxy}
}

func (r *Runtime) builtin_proxy_revocable(call FunctionCall) Value {
	if len(call.Arguments) >= 2 {
		if target, ok := call.Argument(0).(*Object); ok {
			if proxyHandler, ok := call.Argument(1).(*Object); ok {
				proxy := r.newProxyObject(target, proxyHandler, nil)
				revoke := r.newNativeFunc(func(FunctionCall) Value {
					proxy.revoke()
					return _undefined
				}, "", 0)
				ret := r.NewObject()
				ret.self._putProp("proxy", proxy.val, true, true, true)
				ret.self._putProp("revoke", revoke, true, true, true)
				return ret
			}
		}
	}
	panic(r.NewTypeError("Cannot create proxy with a non-object as target or handler"))
}

func (r *Runtime) createProxy(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.builtin_newProxy, nil, "Proxy", 2)

	o._putProp("revocable", r.newNativeFunc(r.builtin_proxy_revocable, "revocable", 2), true, false, true)
	return o
}

func (r *Runtime) getProxy() *Object {
	ret := r.global.Proxy
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Proxy = ret
		r.createProxy(ret)
	}
	return ret
}
