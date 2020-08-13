package goja

import (
	"fmt"

	"github.com/dop251/goja/unistring"
)

func (r *Runtime) newNativeProxyHandler(nativeHandler *ProxyTrapConfig) *Object {
	handler := r.NewObject()
	r.proxyproto_nativehandler_gen_obj_obj(proxy_trap_getPrototypeOf, nativeHandler.GetPrototypeOf, handler)
	r.proxyproto_nativehandler_setPrototypeOf(nativeHandler.SetPrototypeOf, handler)
	r.proxyproto_nativehandler_gen_obj_bool(proxy_trap_isExtensible, nativeHandler.IsExtensible, handler)
	r.proxyproto_nativehandler_gen_obj_bool(proxy_trap_preventExtensions, nativeHandler.PreventExtensions, handler)
	r.proxyproto_nativehandler_getOwnPropertyDescriptor(nativeHandler.GetOwnPropertyDescriptor, handler)
	r.proxyproto_nativehandler_defineProperty(nativeHandler.DefineProperty, handler)
	r.proxyproto_nativehandler_gen_obj_string_bool(proxy_trap_has, nativeHandler.Has, handler)
	r.proxyproto_nativehandler_get(nativeHandler.Get, handler)
	r.proxyproto_nativehandler_set(nativeHandler.Set, handler)
	r.proxyproto_nativehandler_gen_obj_string_bool(proxy_trap_deleteProperty, nativeHandler.DeleteProperty, handler)
	r.proxyproto_nativehandler_gen_obj_obj(proxy_trap_ownKeys, nativeHandler.OwnKeys, handler)
	r.proxyproto_nativehandler_apply(nativeHandler.Apply, handler)
	r.proxyproto_nativehandler_construct(nativeHandler.Construct, handler)
	return handler
}

func (r *Runtime) proxyproto_nativehandler_gen_obj_obj(name proxyTrap, native func(*Object) *Object, handler *Object) {
	if native != nil {
		handler.self._putProp(unistring.String(name), r.newNativeFunc(func(call FunctionCall) Value {
			if len(call.Arguments) >= 1 {
				if t, ok := call.Argument(0).(*Object); ok {
					return native(t)
				}
			}
			panic(r.NewTypeError("%s needs to be called with target as Object", name))
		}, nil, unistring.String(fmt.Sprintf("[native %s]", name)), nil, 1), true, true, true)
	}
}

func (r *Runtime) proxyproto_nativehandler_setPrototypeOf(native func(*Object, *Object) bool, handler *Object) {
	if native != nil {
		handler.self._putProp("setPrototypeOf", r.newNativeFunc(func(call FunctionCall) Value {
			if len(call.Arguments) >= 2 {
				if t, ok := call.Argument(0).(*Object); ok {
					if p, ok := call.Argument(1).(*Object); ok {
						s := native(t, p)
						return r.ToValue(s)
					}
				}
			}
			panic(r.NewTypeError("setPrototypeOf needs to be called with target and prototype as Object"))
		}, nil, "[native setPrototypeOf]", nil, 2), true, true, true)
	}
}

func (r *Runtime) proxyproto_nativehandler_gen_obj_bool(name proxyTrap, native func(*Object) bool, handler *Object) {
	if native != nil {
		handler.self._putProp(unistring.String(name), r.newNativeFunc(func(call FunctionCall) Value {
			if len(call.Arguments) >= 1 {
				if t, ok := call.Argument(0).(*Object); ok {
					s := native(t)
					return r.ToValue(s)
				}
			}
			panic(r.NewTypeError("%s needs to be called with target as Object", name))
		}, nil, unistring.String(fmt.Sprintf("[native %s]", name)), nil, 1), true, true, true)
	}
}

func (r *Runtime) proxyproto_nativehandler_getOwnPropertyDescriptor(native func(*Object, string) PropertyDescriptor, handler *Object) {
	if native != nil {
		handler.self._putProp("getOwnPropertyDescriptor", r.newNativeFunc(func(call FunctionCall) Value {
			if len(call.Arguments) >= 2 {
				if t, ok := call.Argument(0).(*Object); ok {
					switch p := call.Argument(1).(type) {
					case *valueSymbol:
						return _undefined
					default:
						desc := native(t, p.String())
						return desc.toValue(r)
					}
				}
			}
			panic(r.NewTypeError("getOwnPropertyDescriptor needs to be called with target as Object and prop as string"))
		}, nil, "[native getOwnPropertyDescriptor]", nil, 2), true, true, true)
	}
}

func (r *Runtime) proxyproto_nativehandler_defineProperty(native func(*Object, string, PropertyDescriptor) bool, handler *Object) {
	if native != nil {
		handler.self._putProp("defineProperty", r.newNativeFunc(func(call FunctionCall) Value {
			if len(call.Arguments) >= 3 {
				if t, ok := call.Argument(0).(*Object); ok {
					if k, ok := call.Argument(1).(valueString); ok {
						propertyDescriptor := r.toPropertyDescriptor(call.Argument(2))
						s := native(t, k.String(), propertyDescriptor)
						return r.ToValue(s)
					}
				}
			}
			panic(r.NewTypeError("defineProperty needs to be called with target as Object and propertyDescriptor as string and key as string"))
		}, nil, "[native defineProperty]", nil, 3), true, true, true)
	}
}

func (r *Runtime) proxyproto_nativehandler_gen_obj_string_bool(name proxyTrap, native func(*Object, string) bool, handler *Object) {
	if native != nil {
		handler.self._putProp(unistring.String(name), r.newNativeFunc(func(call FunctionCall) Value {
			if len(call.Arguments) >= 2 {
				if t, ok := call.Argument(0).(*Object); ok {
					switch p := call.Argument(1).(type) {
					case *valueSymbol:
						return valueFalse
					default:
						o := native(t, p.String())
						return r.ToValue(o)
					}
				}
			}
			panic(r.NewTypeError("%s needs to be called with target as Object and property as string", name))
		}, nil, unistring.String(fmt.Sprintf("[native %s]", name)), nil, 2), true, true, true)
	}
}

func (r *Runtime) proxyproto_nativehandler_get(native func(*Object, string, *Object) Value, handler *Object) {
	if native != nil {
		handler.self._putProp("get", r.newNativeFunc(func(call FunctionCall) Value {
			if len(call.Arguments) >= 3 {
				if t, ok := call.Argument(0).(*Object); ok {
					if r, ok := call.Argument(2).(*Object); ok {
						switch p := call.Argument(1).(type) {
						case *valueSymbol:
							return _undefined
						default:
							return native(t, p.String(), r)
						}
					}
				}
			}
			panic(r.NewTypeError("get needs to be called with target and receiver as Object and property as string"))
		}, nil, "[native get]", nil, 3), true, true, true)
	}
}

func (r *Runtime) proxyproto_nativehandler_set(native func(*Object, string, Value, *Object) bool, handler *Object) {
	if native != nil {
		handler.self._putProp("set", r.newNativeFunc(func(call FunctionCall) Value {
			if len(call.Arguments) >= 4 {
				if t, ok := call.Argument(0).(*Object); ok {
					if p, ok := call.Argument(1).(valueString); ok {
						v := call.Argument(2)
						if re, ok := call.Argument(3).(*Object); ok {
							s := native(t, p.String(), v, re)
							return r.ToValue(s)
						}
					}
				}
			}
			panic(r.NewTypeError("set needs to be called with target and receiver as Object, property as string and value as a legal javascript value"))
		}, nil, "[native set]", nil, 4), true, true, true)
	}
}

func (r *Runtime) proxyproto_nativehandler_apply(native func(*Object, *Object, []Value) Value, handler *Object) {
	if native != nil {
		handler.self._putProp("apply", r.newNativeFunc(func(call FunctionCall) Value {
			if len(call.Arguments) >= 3 {
				if t, ok := call.Argument(0).(*Object); ok {
					if this, ok := call.Argument(1).(*Object); ok {
						if v, ok := call.Argument(2).(*Object); ok {
							if a, ok := v.self.(*arrayObject); ok {
								v := native(t, this, a.values)
								return r.ToValue(v)
							}
						}
					}
				}
			}
			panic(r.NewTypeError("apply needs to be called with target and this as Object and argumentsList as an array of legal javascript values"))
		}, nil, "[native apply]", nil, 3), true, true, true)
	}
}

func (r *Runtime) proxyproto_nativehandler_construct(native func(*Object, []Value, *Object) *Object, handler *Object) {
	if native != nil {
		handler.self._putProp("construct", r.newNativeFunc(func(call FunctionCall) Value {
			if len(call.Arguments) >= 3 {
				if t, ok := call.Argument(0).(*Object); ok {
					if v, ok := call.Argument(1).(*Object); ok {
						if newTarget, ok := call.Argument(2).(*Object); ok {
							if a, ok := v.self.(*arrayObject); ok {
								return native(t, a.values, newTarget)
							}
						}
					}
				}
			}
			panic(r.NewTypeError("construct needs to be called with target and newTarget as Object and argumentsList as an array of legal javascript values"))
		}, nil, "[native construct]", nil, 3), true, true, true)
	}
}

// ProxyTrapConfig provides a simplified Go-friendly API for implementing Proxy traps.
// Note that the Proxy may not have Symbol properties when using this as a handler because property keys are
// passed as strings.
// get() and getOwnPropertyDescriptor() for Symbol properties will always return undefined;
// has() and deleteProperty() for Symbol properties will always return false;
// set() and defineProperty() for Symbol properties will throw a TypeError.
// If you need Symbol properties implement the handler in JavaScript.
type ProxyTrapConfig struct {
	// A trap for Object.getPrototypeOf, Reflect.getPrototypeOf, __proto__, Object.prototype.isPrototypeOf, instanceof
	GetPrototypeOf func(target *Object) (prototype *Object)

	// A trap for Object.setPrototypeOf, Reflect.setPrototypeOf
	SetPrototypeOf func(target *Object, prototype *Object) (success bool)

	// A trap for Object.isExtensible, Reflect.isExtensible
	IsExtensible func(target *Object) (success bool)

	// A trap for Object.preventExtensions, Reflect.preventExtensions
	PreventExtensions func(target *Object) (success bool)

	// A trap for Object.getOwnPropertyDescriptor, Reflect.getOwnPropertyDescriptor
	GetOwnPropertyDescriptor func(target *Object, prop string) (propertyDescriptor PropertyDescriptor)

	// A trap for Object.defineProperty, Reflect.defineProperty
	DefineProperty func(target *Object, key string, propertyDescriptor PropertyDescriptor) (success bool)

	// A trap for the in operator, with operator, Reflect.has
	Has func(target *Object, property string) (available bool)

	// A trap for getting property values, Reflect.get
	Get func(target *Object, property string, receiver *Object) (value Value)

	// A trap for setting property values, Reflect.set
	Set func(target *Object, property string, value Value, receiver *Object) (success bool)

	// A trap for the delete operator, Reflect.deleteProperty
	DeleteProperty func(target *Object, property string) (success bool)

	// A trap for Object.getOwnPropertyNames, Object.getOwnPropertySymbols, Object.keys, Reflect.ownKeys
	OwnKeys func(target *Object) (object *Object)

	// A trap for a function call, Function.prototype.apply, Function.prototype.call, Reflect.apply
	Apply func(target *Object, this *Object, argumentsList []Value) (value Value)

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
	return r.newProxy(args, r.getPrototypeFromCtor(newTarget, r.global.Proxy, r.global.ObjectPrototype))
}

func (r *Runtime) NewProxy(target *Object, nativeHandler *ProxyTrapConfig) Proxy {
	handler := r.newNativeProxyHandler(nativeHandler)
	proxy := r.newProxyObject(target, handler, nil)
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
				}, nil, "", nil, 0)
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

	o._putProp("revocable", r.newNativeFunc(r.builtin_proxy_revocable, nil, "revocable", nil, 2), true, false, true)
	return o
}

func (r *Runtime) initProxy() {
	r.global.Proxy = r.newLazyObject(r.createProxy)
	r.addToGlobal("Proxy", r.global.Proxy)
}
