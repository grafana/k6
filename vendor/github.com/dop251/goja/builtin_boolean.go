package goja

func (r *Runtime) booleanproto_toString(call FunctionCall) Value {
	var b bool
	switch o := call.This.(type) {
	case valueBool:
		b = bool(o)
		goto success
	case *Object:
		if p, ok := o.self.(*primitiveValueObject); ok {
			if b1, ok := p.pValue.(valueBool); ok {
				b = bool(b1)
				goto success
			}
		}
		if o, ok := o.self.(*objectGoReflect); ok {
			if o.class == classBoolean && o.toString != nil {
				return o.toString()
			}
		}
	}
	r.typeErrorResult(true, "Method Boolean.prototype.toString is called on incompatible receiver")

success:
	if b {
		return stringTrue
	}
	return stringFalse
}

func (r *Runtime) booleanproto_valueOf(call FunctionCall) Value {
	switch o := call.This.(type) {
	case valueBool:
		return o
	case *Object:
		if p, ok := o.self.(*primitiveValueObject); ok {
			if b, ok := p.pValue.(valueBool); ok {
				return b
			}
		}
		if o, ok := o.self.(*objectGoReflect); ok {
			if o.class == classBoolean && o.valueOf != nil {
				return o.valueOf()
			}
		}
	}

	r.typeErrorResult(true, "Method Boolean.prototype.valueOf is called on incompatible receiver")
	return nil
}

func (r *Runtime) getBooleanPrototype() *Object {
	ret := r.global.BooleanPrototype
	if ret == nil {
		ret = r.newPrimitiveObject(valueFalse, r.global.ObjectPrototype, classBoolean)
		r.global.BooleanPrototype = ret
		o := ret.self
		o._putProp("toString", r.newNativeFunc(r.booleanproto_toString, "toString", 0), true, false, true)
		o._putProp("valueOf", r.newNativeFunc(r.booleanproto_valueOf, "valueOf", 0), true, false, true)
		o._putProp("constructor", r.getBoolean(), true, false, true)
	}
	return ret
}

func (r *Runtime) getBoolean() *Object {
	ret := r.global.Boolean
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Boolean = ret
		proto := r.getBooleanPrototype()
		r.newNativeFuncAndConstruct(ret, r.builtin_Boolean,
			r.wrapNativeConstruct(r.builtin_newBoolean, ret, proto), proto, "Boolean", intToValue(1))
	}
	return ret
}
