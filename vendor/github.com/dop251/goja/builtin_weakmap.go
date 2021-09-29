package goja

type weakMap uint64

type weakMapObject struct {
	baseObject
	m weakMap
}

func (wmo *weakMapObject) init() {
	wmo.baseObject.init()
	wmo.m = weakMap(wmo.val.runtime.genId())
}

func (wm weakMap) set(key *Object, value Value) {
	key.getWeakRefs()[wm] = value
}

func (wm weakMap) get(key *Object) Value {
	return key.weakRefs[wm]
}

func (wm weakMap) remove(key *Object) bool {
	if _, exists := key.weakRefs[wm]; exists {
		delete(key.weakRefs, wm)
		return true
	}
	return false
}

func (wm weakMap) has(key *Object) bool {
	_, exists := key.weakRefs[wm]
	return exists
}

func (r *Runtime) weakMapProto_delete(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wmo, ok := thisObj.self.(*weakMapObject)
	if !ok {
		panic(r.NewTypeError("Method WeakMap.prototype.delete called on incompatible receiver %s", thisObj.String()))
	}
	key, ok := call.Argument(0).(*Object)
	if ok && wmo.m.remove(key) {
		return valueTrue
	}
	return valueFalse
}

func (r *Runtime) weakMapProto_get(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wmo, ok := thisObj.self.(*weakMapObject)
	if !ok {
		panic(r.NewTypeError("Method WeakMap.prototype.get called on incompatible receiver %s", thisObj.String()))
	}
	var res Value
	if key, ok := call.Argument(0).(*Object); ok {
		res = wmo.m.get(key)
	}
	if res == nil {
		return _undefined
	}
	return res
}

func (r *Runtime) weakMapProto_has(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wmo, ok := thisObj.self.(*weakMapObject)
	if !ok {
		panic(r.NewTypeError("Method WeakMap.prototype.has called on incompatible receiver %s", thisObj.String()))
	}
	key, ok := call.Argument(0).(*Object)
	if ok && wmo.m.has(key) {
		return valueTrue
	}
	return valueFalse
}

func (r *Runtime) weakMapProto_set(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wmo, ok := thisObj.self.(*weakMapObject)
	if !ok {
		panic(r.NewTypeError("Method WeakMap.prototype.set called on incompatible receiver %s", thisObj.String()))
	}
	key := r.toObject(call.Argument(0))
	wmo.m.set(key, call.Argument(1))
	return call.This
}

func (r *Runtime) needNew(name string) *Object {
	return r.NewTypeError("Constructor %s requires 'new'", name)
}

func (r *Runtime) getPrototypeFromCtor(newTarget, defCtor, defProto *Object) *Object {
	if newTarget == defCtor {
		return defProto
	}
	proto := newTarget.self.getStr("prototype", nil)
	if obj, ok := proto.(*Object); ok {
		return obj
	}
	return defProto
}

func (r *Runtime) builtin_newWeakMap(args []Value, newTarget *Object) *Object {
	if newTarget == nil {
		panic(r.needNew("WeakMap"))
	}
	proto := r.getPrototypeFromCtor(newTarget, r.global.WeakMap, r.global.WeakMapPrototype)
	o := &Object{runtime: r}

	wmo := &weakMapObject{}
	wmo.class = classWeakMap
	wmo.val = o
	wmo.extensible = true
	o.self = wmo
	wmo.prototype = proto
	wmo.init()
	if len(args) > 0 {
		if arg := args[0]; arg != nil && arg != _undefined && arg != _null {
			adder := wmo.getStr("set", nil)
			iter := r.getIterator(arg, nil)
			i0 := valueInt(0)
			i1 := valueInt(1)
			if adder == r.global.weakMapAdder {
				r.iterate(iter, func(item Value) {
					itemObj := r.toObject(item)
					k := itemObj.self.getIdx(i0, nil)
					v := nilSafe(itemObj.self.getIdx(i1, nil))
					wmo.m.set(r.toObject(k), v)
				})
			} else {
				adderFn := toMethod(adder)
				if adderFn == nil {
					panic(r.NewTypeError("WeakMap.set in missing"))
				}
				r.iterate(iter, func(item Value) {
					itemObj := r.toObject(item)
					k := itemObj.self.getIdx(i0, nil)
					v := itemObj.self.getIdx(i1, nil)
					adderFn(FunctionCall{This: o, Arguments: []Value{k, v}})
				})
			}
		}
	}
	return o
}

func (r *Runtime) createWeakMapProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.global.ObjectPrototype, classObject)

	o._putProp("constructor", r.global.WeakMap, true, false, true)
	r.global.weakMapAdder = r.newNativeFunc(r.weakMapProto_set, nil, "set", nil, 2)
	o._putProp("set", r.global.weakMapAdder, true, false, true)
	o._putProp("delete", r.newNativeFunc(r.weakMapProto_delete, nil, "delete", nil, 1), true, false, true)
	o._putProp("has", r.newNativeFunc(r.weakMapProto_has, nil, "has", nil, 1), true, false, true)
	o._putProp("get", r.newNativeFunc(r.weakMapProto_get, nil, "get", nil, 1), true, false, true)

	o._putSym(SymToStringTag, valueProp(asciiString(classWeakMap), false, false, true))

	return o
}

func (r *Runtime) createWeakMap(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.builtin_newWeakMap, r.global.WeakMapPrototype, "WeakMap", 0)

	return o
}

func (r *Runtime) initWeakMap() {
	r.global.WeakMapPrototype = r.newLazyObject(r.createWeakMapProto)
	r.global.WeakMap = r.newLazyObject(r.createWeakMap)

	r.addToGlobal("WeakMap", r.global.WeakMap)
}
