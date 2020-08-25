package goja

import (
	"fmt"
)

func (r *Runtime) builtin_Object(args []Value, proto *Object) *Object {
	if len(args) > 0 {
		arg := args[0]
		if arg != _undefined && arg != _null {
			return arg.ToObject(r)
		}
	}
	return r.newBaseObject(proto, classObject).val
}

func (r *Runtime) object_getPrototypeOf(call FunctionCall) Value {
	o := call.Argument(0).ToObject(r)
	p := o.self.proto()
	if p == nil {
		return _null
	}
	return p
}

func (r *Runtime) valuePropToDescriptorObject(desc Value) Value {
	if desc == nil {
		return _undefined
	}
	var writable, configurable, enumerable, accessor bool
	var get, set *Object
	var value Value
	if v, ok := desc.(*valueProperty); ok {
		writable = v.writable
		configurable = v.configurable
		enumerable = v.enumerable
		accessor = v.accessor
		value = v.value
		get = v.getterFunc
		set = v.setterFunc
	} else {
		writable = true
		configurable = true
		enumerable = true
		value = desc
	}

	ret := r.NewObject()
	obj := ret.self
	if !accessor {
		obj.setOwnStr("value", value, false)
		obj.setOwnStr("writable", r.toBoolean(writable), false)
	} else {
		if get != nil {
			obj.setOwnStr("get", get, false)
		} else {
			obj.setOwnStr("get", _undefined, false)
		}
		if set != nil {
			obj.setOwnStr("set", set, false)
		} else {
			obj.setOwnStr("set", _undefined, false)
		}
	}
	obj.setOwnStr("enumerable", r.toBoolean(enumerable), false)
	obj.setOwnStr("configurable", r.toBoolean(configurable), false)

	return ret
}

func (r *Runtime) object_getOwnPropertyDescriptor(call FunctionCall) Value {
	o := call.Argument(0).ToObject(r)
	propName := toPropertyKey(call.Argument(1))
	return r.valuePropToDescriptorObject(o.getOwnProp(propName))
}

func (r *Runtime) object_getOwnPropertyNames(call FunctionCall) Value {
	obj := call.Argument(0).ToObject(r)

	return r.newArrayValues(obj.self.ownKeys(true, nil))
}

func (r *Runtime) object_getOwnPropertySymbols(call FunctionCall) Value {
	obj := call.Argument(0).ToObject(r)
	return r.newArrayValues(obj.self.ownSymbols(true, nil))
}

func (r *Runtime) toValueProp(v Value) *valueProperty {
	if v == nil || v == _undefined {
		return nil
	}
	obj := r.toObject(v)
	getter := obj.self.getStr("get", nil)
	setter := obj.self.getStr("set", nil)
	writable := obj.self.getStr("writable", nil)
	value := obj.self.getStr("value", nil)
	if (getter != nil || setter != nil) && (value != nil || writable != nil) {
		r.typeErrorResult(true, "Invalid property descriptor. Cannot both specify accessors and a value or writable attribute")
	}

	ret := &valueProperty{}
	if writable != nil && writable.ToBoolean() {
		ret.writable = true
	}
	if e := obj.self.getStr("enumerable", nil); e != nil && e.ToBoolean() {
		ret.enumerable = true
	}
	if c := obj.self.getStr("configurable", nil); c != nil && c.ToBoolean() {
		ret.configurable = true
	}
	ret.value = value

	if getter != nil && getter != _undefined {
		o := r.toObject(getter)
		if _, ok := o.self.assertCallable(); !ok {
			r.typeErrorResult(true, "getter must be a function")
		}
		ret.getterFunc = o
	}

	if setter != nil && setter != _undefined {
		o := r.toObject(v)
		if _, ok := o.self.assertCallable(); !ok {
			r.typeErrorResult(true, "setter must be a function")
		}
		ret.setterFunc = o
	}

	if ret.getterFunc != nil || ret.setterFunc != nil {
		ret.accessor = true
	}

	return ret
}

func (r *Runtime) toPropertyDescriptor(v Value) (ret PropertyDescriptor) {
	if o, ok := v.(*Object); ok {
		descr := o.self

		// Save the original descriptor for reference
		ret.jsDescriptor = o

		ret.Value = descr.getStr("value", nil)

		if p := descr.getStr("writable", nil); p != nil {
			ret.Writable = ToFlag(p.ToBoolean())
		}
		if p := descr.getStr("enumerable", nil); p != nil {
			ret.Enumerable = ToFlag(p.ToBoolean())
		}
		if p := descr.getStr("configurable", nil); p != nil {
			ret.Configurable = ToFlag(p.ToBoolean())
		}

		ret.Getter = descr.getStr("get", nil)
		ret.Setter = descr.getStr("set", nil)

		if ret.Getter != nil && ret.Getter != _undefined {
			if _, ok := r.toObject(ret.Getter).self.assertCallable(); !ok {
				r.typeErrorResult(true, "getter must be a function")
			}
		}

		if ret.Setter != nil && ret.Setter != _undefined {
			if _, ok := r.toObject(ret.Setter).self.assertCallable(); !ok {
				r.typeErrorResult(true, "setter must be a function")
			}
		}

		if (ret.Getter != nil || ret.Setter != nil) && (ret.Value != nil || ret.Writable != FLAG_NOT_SET) {
			r.typeErrorResult(true, "Invalid property descriptor. Cannot both specify accessors and a value or writable attribute")
		}
	} else {
		r.typeErrorResult(true, "Property description must be an object: %s", v.String())
	}

	return
}

func (r *Runtime) _defineProperties(o *Object, p Value) {
	type propItem struct {
		name Value
		prop PropertyDescriptor
	}
	props := p.ToObject(r)
	names := props.self.ownPropertyKeys(false, nil)
	list := make([]propItem, 0, len(names))
	for _, itemName := range names {
		list = append(list, propItem{
			name: itemName,
			prop: r.toPropertyDescriptor(props.get(itemName, nil)),
		})
	}
	for _, prop := range list {
		o.defineOwnProperty(prop.name, prop.prop, true)
	}
}

func (r *Runtime) object_create(call FunctionCall) Value {
	var proto *Object
	if arg := call.Argument(0); arg != _null {
		if o, ok := arg.(*Object); ok {
			proto = o
		} else {
			r.typeErrorResult(true, "Object prototype may only be an Object or null: %s", arg.String())
		}
	}
	o := r.newBaseObject(proto, classObject).val

	if props := call.Argument(1); props != _undefined {
		r._defineProperties(o, props)
	}

	return o
}

func (r *Runtime) object_defineProperty(call FunctionCall) (ret Value) {
	if obj, ok := call.Argument(0).(*Object); ok {
		descr := r.toPropertyDescriptor(call.Argument(2))
		obj.defineOwnProperty(toPropertyKey(call.Argument(1)), descr, true)
		ret = call.Argument(0)
	} else {
		r.typeErrorResult(true, "Object.defineProperty called on non-object")
	}
	return
}

func (r *Runtime) object_defineProperties(call FunctionCall) Value {
	obj := r.toObject(call.Argument(0))
	r._defineProperties(obj, call.Argument(1))
	return obj
}

func (r *Runtime) object_seal(call FunctionCall) Value {
	// ES6
	arg := call.Argument(0)
	if obj, ok := arg.(*Object); ok {
		descr := PropertyDescriptor{
			Writable:     FLAG_TRUE,
			Enumerable:   FLAG_TRUE,
			Configurable: FLAG_FALSE,
		}
		for _, key := range obj.self.ownPropertyKeys(true, nil) {
			v := obj.getOwnProp(key)
			if prop, ok := v.(*valueProperty); ok {
				if !prop.configurable {
					continue
				}
				prop.configurable = false
			} else {
				descr.Value = v
				obj.defineOwnProperty(key, descr, true)
			}
		}
		obj.self.preventExtensions(false)
		return obj
	}
	return arg
}

func (r *Runtime) object_freeze(call FunctionCall) Value {
	arg := call.Argument(0)
	if obj, ok := arg.(*Object); ok {
		descr := PropertyDescriptor{
			Writable:     FLAG_FALSE,
			Enumerable:   FLAG_TRUE,
			Configurable: FLAG_FALSE,
		}
		for _, key := range obj.self.ownPropertyKeys(true, nil) {
			v := obj.getOwnProp(key)
			if prop, ok := v.(*valueProperty); ok {
				prop.configurable = false
				if prop.value != nil {
					prop.writable = false
				}
			} else {
				descr.Value = v
				obj.defineOwnProperty(key, descr, true)
			}
		}
		obj.self.preventExtensions(false)
		return obj
	} else {
		// ES6 behavior
		return arg
	}
}

func (r *Runtime) object_preventExtensions(call FunctionCall) (ret Value) {
	arg := call.Argument(0)
	if obj, ok := arg.(*Object); ok {
		obj.self.preventExtensions(false)
		return obj
	}
	// ES6
	//r.typeErrorResult(true, "Object.preventExtensions called on non-object")
	//panic("Unreachable")
	return arg
}

func (r *Runtime) object_isSealed(call FunctionCall) Value {
	if obj, ok := call.Argument(0).(*Object); ok {
		if obj.self.isExtensible() {
			return valueFalse
		}
		for _, key := range obj.self.ownPropertyKeys(true, nil) {
			prop := obj.getOwnProp(key)
			if prop, ok := prop.(*valueProperty); ok {
				if prop.configurable {
					return valueFalse
				}
			} else {
				return valueFalse
			}
		}
	}
	return valueTrue
}

func (r *Runtime) object_isFrozen(call FunctionCall) Value {
	if obj, ok := call.Argument(0).(*Object); ok {
		if obj.self.isExtensible() {
			return valueFalse
		}
		for _, key := range obj.self.ownPropertyKeys(true, nil) {
			prop := obj.getOwnProp(key)
			if prop, ok := prop.(*valueProperty); ok {
				if prop.configurable || prop.value != nil && prop.writable {
					return valueFalse
				}
			} else {
				return valueFalse
			}
		}
	}
	return valueTrue
}

func (r *Runtime) object_isExtensible(call FunctionCall) Value {
	if obj, ok := call.Argument(0).(*Object); ok {
		if obj.self.isExtensible() {
			return valueTrue
		}
		return valueFalse
	} else {
		// ES6
		//r.typeErrorResult(true, "Object.isExtensible called on non-object")
		return valueFalse
	}
}

func (r *Runtime) object_keys(call FunctionCall) Value {
	obj := call.Argument(0).ToObject(r)

	return r.newArrayValues(obj.self.ownKeys(false, nil))
}

func (r *Runtime) objectproto_hasOwnProperty(call FunctionCall) Value {
	p := toPropertyKey(call.Argument(0))
	o := call.This.ToObject(r)
	if o.hasOwnProperty(p) {
		return valueTrue
	} else {
		return valueFalse
	}
}

func (r *Runtime) objectproto_isPrototypeOf(call FunctionCall) Value {
	if v, ok := call.Argument(0).(*Object); ok {
		o := call.This.ToObject(r)
		for {
			v = v.self.proto()
			if v == nil {
				break
			}
			if v == o {
				return valueTrue
			}
		}
	}
	return valueFalse
}

func (r *Runtime) objectproto_propertyIsEnumerable(call FunctionCall) Value {
	p := toPropertyKey(call.Argument(0))
	o := call.This.ToObject(r)
	pv := o.getOwnProp(p)
	if pv == nil {
		return valueFalse
	}
	if prop, ok := pv.(*valueProperty); ok {
		if !prop.enumerable {
			return valueFalse
		}
	}
	return valueTrue
}

func (r *Runtime) objectproto_toString(call FunctionCall) Value {
	switch o := call.This.(type) {
	case valueNull:
		return stringObjectNull
	case valueUndefined:
		return stringObjectUndefined
	default:
		obj := o.ToObject(r)
		var clsName string
		if isArray(obj) {
			clsName = classArray
		} else {
			clsName = obj.self.className()
		}
		if tag := obj.self.getSym(symToStringTag, nil); tag != nil {
			if str, ok := tag.(valueString); ok {
				clsName = str.String()
			}
		}
		return newStringValue(fmt.Sprintf("[object %s]", clsName))
	}
}

func (r *Runtime) objectproto_toLocaleString(call FunctionCall) Value {
	toString := toMethod(r.getVStr(call.This, "toString"))
	return toString(FunctionCall{This: call.This})
}

func (r *Runtime) objectproto_getProto(call FunctionCall) Value {
	proto := call.This.ToObject(r).self.proto()
	if proto != nil {
		return proto
	}
	return _null
}

func (r *Runtime) objectproto_setProto(call FunctionCall) Value {
	o := call.This
	r.checkObjectCoercible(o)
	proto := r.toProto(call.Argument(0))
	if o, ok := o.(*Object); ok {
		o.self.setProto(proto, true)
	}

	return _undefined
}

func (r *Runtime) objectproto_valueOf(call FunctionCall) Value {
	return call.This.ToObject(r)
}

func (r *Runtime) object_assign(call FunctionCall) Value {
	to := call.Argument(0).ToObject(r)
	if len(call.Arguments) > 1 {
		for _, arg := range call.Arguments[1:] {
			if arg != _undefined && arg != _null {
				source := arg.ToObject(r)
				for _, key := range source.self.ownPropertyKeys(true, nil) {
					p := source.getOwnProp(key)
					if p == nil {
						continue
					}
					if v, ok := p.(*valueProperty); ok {
						if !v.enumerable {
							continue
						}
						p = v.get(source)
					}
					to.setOwn(key, p, true)
				}
			}
		}
	}

	return to
}

func (r *Runtime) object_is(call FunctionCall) Value {
	return r.toBoolean(call.Argument(0).SameAs(call.Argument(1)))
}

func (r *Runtime) toProto(proto Value) *Object {
	if proto != _null {
		if obj, ok := proto.(*Object); ok {
			return obj
		} else {
			panic(r.NewTypeError("Object prototype may only be an Object or null: %s", proto))
		}
	}
	return nil
}

func (r *Runtime) object_setPrototypeOf(call FunctionCall) Value {
	o := call.Argument(0)
	r.checkObjectCoercible(o)
	proto := r.toProto(call.Argument(1))
	if o, ok := o.(*Object); ok {
		o.self.setProto(proto, true)
	}

	return o
}

func (r *Runtime) initObject() {
	o := r.global.ObjectPrototype.self
	o._putProp("toString", r.newNativeFunc(r.objectproto_toString, nil, "toString", nil, 0), true, false, true)
	o._putProp("toLocaleString", r.newNativeFunc(r.objectproto_toLocaleString, nil, "toLocaleString", nil, 0), true, false, true)
	o._putProp("valueOf", r.newNativeFunc(r.objectproto_valueOf, nil, "valueOf", nil, 0), true, false, true)
	o._putProp("hasOwnProperty", r.newNativeFunc(r.objectproto_hasOwnProperty, nil, "hasOwnProperty", nil, 1), true, false, true)
	o._putProp("isPrototypeOf", r.newNativeFunc(r.objectproto_isPrototypeOf, nil, "isPrototypeOf", nil, 1), true, false, true)
	o._putProp("propertyIsEnumerable", r.newNativeFunc(r.objectproto_propertyIsEnumerable, nil, "propertyIsEnumerable", nil, 1), true, false, true)
	o.defineOwnPropertyStr(__proto__, PropertyDescriptor{
		Getter:       r.newNativeFunc(r.objectproto_getProto, nil, "get __proto__", nil, 0),
		Setter:       r.newNativeFunc(r.objectproto_setProto, nil, "set __proto__", nil, 1),
		Configurable: FLAG_TRUE,
	}, true)

	r.global.Object = r.newNativeFuncConstruct(r.builtin_Object, classObject, r.global.ObjectPrototype, 1)
	o = r.global.Object.self
	o._putProp("assign", r.newNativeFunc(r.object_assign, nil, "assign", nil, 2), true, false, true)
	o._putProp("defineProperty", r.newNativeFunc(r.object_defineProperty, nil, "defineProperty", nil, 3), true, false, true)
	o._putProp("defineProperties", r.newNativeFunc(r.object_defineProperties, nil, "defineProperties", nil, 2), true, false, true)
	o._putProp("getOwnPropertyDescriptor", r.newNativeFunc(r.object_getOwnPropertyDescriptor, nil, "getOwnPropertyDescriptor", nil, 2), true, false, true)
	o._putProp("getPrototypeOf", r.newNativeFunc(r.object_getPrototypeOf, nil, "getPrototypeOf", nil, 1), true, false, true)
	o._putProp("is", r.newNativeFunc(r.object_is, nil, "is", nil, 2), true, false, true)
	o._putProp("getOwnPropertyNames", r.newNativeFunc(r.object_getOwnPropertyNames, nil, "getOwnPropertyNames", nil, 1), true, false, true)
	o._putProp("getOwnPropertySymbols", r.newNativeFunc(r.object_getOwnPropertySymbols, nil, "getOwnPropertySymbols", nil, 1), true, false, true)
	o._putProp("create", r.newNativeFunc(r.object_create, nil, "create", nil, 2), true, false, true)
	o._putProp("seal", r.newNativeFunc(r.object_seal, nil, "seal", nil, 1), true, false, true)
	o._putProp("freeze", r.newNativeFunc(r.object_freeze, nil, "freeze", nil, 1), true, false, true)
	o._putProp("preventExtensions", r.newNativeFunc(r.object_preventExtensions, nil, "preventExtensions", nil, 1), true, false, true)
	o._putProp("isSealed", r.newNativeFunc(r.object_isSealed, nil, "isSealed", nil, 1), true, false, true)
	o._putProp("isFrozen", r.newNativeFunc(r.object_isFrozen, nil, "isFrozen", nil, 1), true, false, true)
	o._putProp("isExtensible", r.newNativeFunc(r.object_isExtensible, nil, "isExtensible", nil, 1), true, false, true)
	o._putProp("keys", r.newNativeFunc(r.object_keys, nil, "keys", nil, 1), true, false, true)
	o._putProp("setPrototypeOf", r.newNativeFunc(r.object_setPrototypeOf, nil, "setPrototypeOf", nil, 2), true, false, true)

	r.addToGlobal("Object", r.global.Object)
}
