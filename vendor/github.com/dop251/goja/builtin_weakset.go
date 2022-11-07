package goja

type weakSetObject struct {
	baseObject
	s weakMap
}

func (ws *weakSetObject) init() {
	ws.baseObject.init()
	ws.s = weakMap(ws.val.runtime.genId())
}

func (r *Runtime) weakSetProto_add(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wso, ok := thisObj.self.(*weakSetObject)
	if !ok {
		panic(r.NewTypeError("Method WeakSet.prototype.add called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	wso.s.set(r.toObject(call.Argument(0)), nil)
	return call.This
}

func (r *Runtime) weakSetProto_delete(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wso, ok := thisObj.self.(*weakSetObject)
	if !ok {
		panic(r.NewTypeError("Method WeakSet.prototype.delete called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	obj, ok := call.Argument(0).(*Object)
	if ok && wso.s.remove(obj) {
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
	obj, ok := call.Argument(0).(*Object)
	if ok && wso.s.has(obj) {
		return valueTrue
	}
	return valueFalse
}

func (r *Runtime) builtin_newWeakSet(args []Value, newTarget *Object) *Object {
	if newTarget == nil {
		panic(r.needNew("WeakSet"))
	}
	proto := r.getPrototypeFromCtor(newTarget, r.global.WeakSet, r.global.WeakSetPrototype)
	o := &Object{runtime: r}

	wso := &weakSetObject{}
	wso.class = classWeakSet
	wso.val = o
	wso.extensible = true
	o.self = wso
	wso.prototype = proto
	wso.init()
	if len(args) > 0 {
		if arg := args[0]; arg != nil && arg != _undefined && arg != _null {
			adder := wso.getStr("add", nil)
			stdArr := r.checkStdArrayIter(arg)
			if adder == r.global.weakSetAdder {
				if stdArr != nil {
					for _, v := range stdArr.values {
						wso.s.set(r.toObject(v), nil)
					}
				} else {
					r.getIterator(arg, nil).iterate(func(item Value) {
						wso.s.set(r.toObject(item), nil)
					})
				}
			} else {
				adderFn := toMethod(adder)
				if adderFn == nil {
					panic(r.NewTypeError("WeakSet.add in missing"))
				}
				if stdArr != nil {
					for _, item := range stdArr.values {
						adderFn(FunctionCall{This: o, Arguments: []Value{item}})
					}
				} else {
					r.getIterator(arg, nil).iterate(func(item Value) {
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
	r.global.weakSetAdder = r.newNativeFunc(r.weakSetProto_add, nil, "add", nil, 1)
	o._putProp("add", r.global.weakSetAdder, true, false, true)
	o._putProp("delete", r.newNativeFunc(r.weakSetProto_delete, nil, "delete", nil, 1), true, false, true)
	o._putProp("has", r.newNativeFunc(r.weakSetProto_has, nil, "has", nil, 1), true, false, true)

	o._putSym(SymToStringTag, valueProp(asciiString(classWeakSet), false, false, true))

	return o
}

func (r *Runtime) createWeakSet(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.builtin_newWeakSet, r.global.WeakSetPrototype, "WeakSet", 0)

	return o
}

func (r *Runtime) initWeakSet() {
	r.global.WeakSetPrototype = r.newLazyObject(r.createWeakSetProto)
	r.global.WeakSet = r.newLazyObject(r.createWeakSet)

	r.addToGlobal("WeakSet", r.global.WeakSet)
}
