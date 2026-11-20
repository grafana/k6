package sobek

type weakSetObject struct {
	baseObject
	s weakMap
}

func (ws *weakSetObject) init() {
	ws.baseObject.init()
}

func (r *Runtime) weakSetProto_add(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wso, ok := thisObj.self.(*weakSetObject)
	if !ok {
		panic(r.NewTypeError("Method WeakSet.prototype.add called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	wso.s.set(r.weakSetValueRefs(call.Argument(0)), nil)
	return call.This
}

func (r *Runtime) weakSetProto_delete(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wso, ok := thisObj.self.(*weakSetObject)
	if !ok {
		panic(r.NewTypeError("Method WeakSet.prototype.delete called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	if wso.s.remove(r.getWeakMapRefs(call.Argument(0), false)) {
		return valueTrue
	}
	return valueFalse
}

func (r *Runtime) weakSetProto_has(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wso, ok := thisObj.self.(*weakSetObject)
	if !ok {
		panic(r.NewTypeError("Method WeakSet.prototype.has called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	if wso.s.has(r.getWeakMapRefs(call.Argument(0), false)) {
		return valueTrue
	}
	return valueFalse
}

func (r *Runtime) weakSetValueRefs(v Value) *weakMapRefs {
	if refs := r.getWeakMapRefs(v, true); refs != nil {
		return refs
	}
	panic(r.NewTypeError("Invalid value used in weak set: %s", nilSafe(v).String()))
}

func (r *Runtime) builtin_newWeakSet(args []Value, newTarget *Object) *Object {
	if newTarget == nil {
		panic(r.needNew("WeakSet"))
	}
	proto := r.getPrototypeFromCtor(newTarget, r.global.WeakSet, r.global.WeakSetPrototype)
	o := &Object{runtime: r}

	wso := &weakSetObject{}
	wso.class = classObject
	wso.val = o
	wso.extensible = true
	o.self = wso
	wso.prototype = proto
	wso.init()
	if len(args) > 0 {
		if arg := args[0]; arg != nil && arg != _undefined && arg != _null {
			adder := wso.getStr("add", nil)
			stdArr := r.checkStdArray(arg)
			if adder == r.global.weakSetAdder {
				ir := r.getIterator(arg, nil)
				if stdArr != nil && ir.nextRes != nil {
					for _, v := range stdArr.values {
						wso.s.set(r.weakSetValueRefs(v), nil)
					}
				} else {
					ir.iterate(func(item Value) {
						wso.s.set(r.weakSetValueRefs(item), nil)
					})
				}
			} else {
				adderFn := toMethod(adder)
				if adderFn == nil {
					panic(r.NewTypeError("WeakSet.add in missing"))
				}
				ir := r.getIterator(arg, nil)
				if stdArr != nil && ir.nextRes != nil {
					for _, item := range stdArr.values {
						adderFn(FunctionCall{This: o, Arguments: []Value{item}})
					}
				} else {
					ir.iterate(func(item Value) {
						adderFn(FunctionCall{This: o, Arguments: []Value{item}})
					})
				}
			}
		}
	}
	return o
}

func (r *Runtime) createWeakSetProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.global.ObjectPrototype, classObject)

	o._putProp("constructor", r.global.WeakSet, true, false, true)
	r.global.weakSetAdder = r.newNativeFunc(r.weakSetProto_add, "add", 1)
	o._putProp("add", r.global.weakSetAdder, true, false, true)
	o._putProp("delete", r.newNativeFunc(r.weakSetProto_delete, "delete", 1), true, false, true)
	o._putProp("has", r.newNativeFunc(r.weakSetProto_has, "has", 1), true, false, true)

	o._putSym(SymToStringTag, valueProp(asciiString(classWeakSet), false, false, true))

	return o
}

func (r *Runtime) createWeakSet(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.builtin_newWeakSet, r.getWeakSetPrototype(), "WeakSet", 0)

	return o
}

func (r *Runtime) getWeakSetPrototype() *Object {
	ret := r.global.WeakSetPrototype
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.WeakSetPrototype = ret
		ret.self = r.createWeakSetProto(ret)
	}
	return ret
}

func (r *Runtime) getWeakSet() *Object {
	ret := r.global.WeakSet
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.WeakSet = ret
		ret.self = r.createWeakSet(ret)
	}
	return ret
}
