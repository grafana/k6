package goja

type mapObject struct {
	baseObject
	m *orderedMap
}

type mapIterObject struct {
	baseObject
	iter *orderedMapIter
	kind iterationKind
}

func (o *mapIterObject) next() Value {
	if o.iter == nil {
		return o.val.runtime.createIterResultObject(_undefined, true)
	}

	entry := o.iter.next()
	if entry == nil {
		o.iter = nil
		return o.val.runtime.createIterResultObject(_undefined, true)
	}

	var result Value
	switch o.kind {
	case iterationKindKey:
		result = entry.key
	case iterationKindValue:
		result = entry.value
	default:
		result = o.val.runtime.newArrayValues([]Value{entry.key, entry.value})
	}

	return o.val.runtime.createIterResultObject(result, false)
}

func (mo *mapObject) init() {
	mo.baseObject.init()
	mo.m = newOrderedMap(mo.val.runtime.getHash())
}

func (r *Runtime) mapProto_clear(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	mo, ok := thisObj.self.(*mapObject)
	if !ok {
		panic(r.NewTypeError("Method Map.prototype.clear called on incompatible receiver %s", thisObj.String()))
	}

	mo.m.clear()

	return _undefined
}

func (r *Runtime) mapProto_delete(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	mo, ok := thisObj.self.(*mapObject)
	if !ok {
		panic(r.NewTypeError("Method Map.prototype.delete called on incompatible receiver %s", thisObj.String()))
	}

	return r.toBoolean(mo.m.remove(call.Argument(0)))
}

func (r *Runtime) mapProto_get(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	mo, ok := thisObj.self.(*mapObject)
	if !ok {
		panic(r.NewTypeError("Method Map.prototype.get called on incompatible receiver %s", thisObj.String()))
	}

	return nilSafe(mo.m.get(call.Argument(0)))
}

func (r *Runtime) mapProto_has(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	mo, ok := thisObj.self.(*mapObject)
	if !ok {
		panic(r.NewTypeError("Method Map.prototype.has called on incompatible receiver %s", thisObj.String()))
	}
	if mo.m.has(call.Argument(0)) {
		return valueTrue
	}
	return valueFalse
}

func (r *Runtime) mapProto_set(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	mo, ok := thisObj.self.(*mapObject)
	if !ok {
		panic(r.NewTypeError("Method Map.prototype.set called on incompatible receiver %s", thisObj.String()))
	}
	mo.m.set(call.Argument(0), call.Argument(1))
	return call.This
}

func (r *Runtime) mapProto_entries(call FunctionCall) Value {
	return r.createMapIterator(call.This, iterationKindKeyValue)
}

func (r *Runtime) mapProto_forEach(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	mo, ok := thisObj.self.(*mapObject)
	if !ok {
		panic(r.NewTypeError("Method Map.prototype.forEach called on incompatible receiver %s", thisObj.String()))
	}
	callbackFn, ok := r.toObject(call.Argument(0)).self.assertCallable()
	if !ok {
		panic(r.NewTypeError("object is not a function %s"))
	}
	t := call.Argument(1)
	iter := mo.m.newIter()
	for {
		entry := iter.next()
		if entry == nil {
			break
		}
		callbackFn(FunctionCall{This: t, Arguments: []Value{entry.value, entry.key, thisObj}})
	}

	return _undefined
}

func (r *Runtime) mapProto_keys(call FunctionCall) Value {
	return r.createMapIterator(call.This, iterationKindKey)
}

func (r *Runtime) mapProto_values(call FunctionCall) Value {
	return r.createMapIterator(call.This, iterationKindValue)
}

func (r *Runtime) mapProto_getSize(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	mo, ok := thisObj.self.(*mapObject)
	if !ok {
		panic(r.NewTypeError("Method get Map.prototype.size called on incompatible receiver %s", thisObj.String()))
	}
	return intToValue(int64(mo.m.size))
}

func (r *Runtime) builtin_newMap(args []Value, newTarget *Object) *Object {
	if newTarget == nil {
		panic(r.needNew("Map"))
	}
	proto := r.getPrototypeFromCtor(newTarget, r.global.Map, r.global.MapPrototype)
	o := &Object{runtime: r}

	mo := &mapObject{}
	mo.class = classMap
	mo.val = o
	mo.extensible = true
	o.self = mo
	mo.prototype = proto
	mo.init()
	if len(args) > 0 {
		if arg := args[0]; arg != nil && arg != _undefined && arg != _null {
			adder := mo.getStr("set", nil)
			iter := r.getIterator(arg, nil)
			i0 := valueInt(0)
			i1 := valueInt(1)
			if adder == r.global.mapAdder {
				r.iterate(iter, func(item Value) {
					itemObj := r.toObject(item)
					k := nilSafe(itemObj.self.getIdx(i0, nil))
					v := nilSafe(itemObj.self.getIdx(i1, nil))
					mo.m.set(k, v)
				})
			} else {
				adderFn := toMethod(adder)
				if adderFn == nil {
					panic(r.NewTypeError("Map.set in missing"))
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

func (r *Runtime) createMapIterator(mapValue Value, kind iterationKind) Value {
	obj := r.toObject(mapValue)
	mapObj, ok := obj.self.(*mapObject)
	if !ok {
		panic(r.NewTypeError("Object is not a Map"))
	}

	o := &Object{runtime: r}

	mi := &mapIterObject{
		iter: mapObj.m.newIter(),
		kind: kind,
	}
	mi.class = classMapIterator
	mi.val = o
	mi.extensible = true
	o.self = mi
	mi.prototype = r.global.MapIteratorPrototype
	mi.init()

	return o
}

func (r *Runtime) mapIterProto_next(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	if iter, ok := thisObj.self.(*mapIterObject); ok {
		return iter.next()
	}
	panic(r.NewTypeError("Method Map Iterator.prototype.next called on incompatible receiver %s", thisObj.String()))
}

func (r *Runtime) createMapProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.global.ObjectPrototype, classObject)

	o._putProp("constructor", r.global.Map, true, false, true)
	o._putProp("clear", r.newNativeFunc(r.mapProto_clear, nil, "clear", nil, 0), true, false, true)
	r.global.mapAdder = r.newNativeFunc(r.mapProto_set, nil, "set", nil, 2)
	o._putProp("set", r.global.mapAdder, true, false, true)
	o._putProp("delete", r.newNativeFunc(r.mapProto_delete, nil, "delete", nil, 1), true, false, true)
	o._putProp("forEach", r.newNativeFunc(r.mapProto_forEach, nil, "forEach", nil, 1), true, false, true)
	o._putProp("has", r.newNativeFunc(r.mapProto_has, nil, "has", nil, 1), true, false, true)
	o._putProp("get", r.newNativeFunc(r.mapProto_get, nil, "get", nil, 1), true, false, true)
	o.setOwnStr("size", &valueProperty{
		getterFunc:   r.newNativeFunc(r.mapProto_getSize, nil, "get size", nil, 0),
		accessor:     true,
		writable:     true,
		configurable: true,
	}, true)
	o._putProp("keys", r.newNativeFunc(r.mapProto_keys, nil, "keys", nil, 0), true, false, true)
	o._putProp("values", r.newNativeFunc(r.mapProto_values, nil, "values", nil, 0), true, false, true)

	entriesFunc := r.newNativeFunc(r.mapProto_entries, nil, "entries", nil, 0)
	o._putProp("entries", entriesFunc, true, false, true)
	o._putSym(SymIterator, valueProp(entriesFunc, true, false, true))
	o._putSym(SymToStringTag, valueProp(asciiString(classMap), false, false, true))

	return o
}

func (r *Runtime) createMap(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.builtin_newMap, r.global.MapPrototype, "Map", 0)
	o._putSym(SymSpecies, &valueProperty{
		getterFunc:   r.newNativeFunc(r.returnThis, nil, "get [Symbol.species]", nil, 0),
		accessor:     true,
		configurable: true,
	})

	return o
}

func (r *Runtime) createMapIterProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.global.IteratorPrototype, classObject)

	o._putProp("next", r.newNativeFunc(r.mapIterProto_next, nil, "next", nil, 0), true, false, true)
	o._putSym(SymToStringTag, valueProp(asciiString(classMapIterator), false, false, true))

	return o
}

func (r *Runtime) initMap() {
	r.global.MapIteratorPrototype = r.newLazyObject(r.createMapIterProto)

	r.global.MapPrototype = r.newLazyObject(r.createMapProto)
	r.global.Map = r.newLazyObject(r.createMap)

	r.addToGlobal("Map", r.global.Map)
}
