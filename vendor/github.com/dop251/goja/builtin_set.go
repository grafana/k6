package goja

type setObject struct {
	baseObject
	m *orderedMap
}

type setIterObject struct {
	baseObject
	iter *orderedMapIter
	kind iterationKind
}

func (o *setIterObject) next() Value {
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
	case iterationKindValue:
		result = entry.key
	default:
		result = o.val.runtime.newArrayValues([]Value{entry.key, entry.key})
	}

	return o.val.runtime.createIterResultObject(result, false)
}

func (so *setObject) init() {
	so.baseObject.init()
	so.m = newOrderedMap(so.val.runtime.getHash())
}

func (r *Runtime) setProto_add(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method Set.prototype.add called on incompatible receiver %s", thisObj.String()))
	}

	so.m.set(call.Argument(0), nil)
	return call.This
}

func (r *Runtime) setProto_clear(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method Set.prototype.clear called on incompatible receiver %s", thisObj.String()))
	}

	so.m.clear()
	return _undefined
}

func (r *Runtime) setProto_delete(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method Set.prototype.delete called on incompatible receiver %s", thisObj.String()))
	}

	return r.toBoolean(so.m.remove(call.Argument(0)))
}

func (r *Runtime) setProto_entries(call FunctionCall) Value {
	return r.createSetIterator(call.This, iterationKindKeyValue)
}

func (r *Runtime) setProto_forEach(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method Set.prototype.forEach called on incompatible receiver %s", thisObj.String()))
	}
	callbackFn, ok := r.toObject(call.Argument(0)).self.assertCallable()
	if !ok {
		panic(r.NewTypeError("object is not a function %s"))
	}
	t := call.Argument(1)
	iter := so.m.newIter()
	for {
		entry := iter.next()
		if entry == nil {
			break
		}
		callbackFn(FunctionCall{This: t, Arguments: []Value{entry.key, entry.key, thisObj}})
	}

	return _undefined
}

func (r *Runtime) setProto_has(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method Set.prototype.has called on incompatible receiver %s", thisObj.String()))
	}

	return r.toBoolean(so.m.has(call.Argument(0)))
}

func (r *Runtime) setProto_getSize(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method get Set.prototype.size called on incompatible receiver %s", thisObj.String()))
	}

	return intToValue(int64(so.m.size))
}

func (r *Runtime) setProto_values(call FunctionCall) Value {
	return r.createSetIterator(call.This, iterationKindValue)
}

func (r *Runtime) builtin_newSet(args []Value, newTarget *Object) *Object {
	if newTarget == nil {
		panic(r.needNew("Set"))
	}
	proto := r.getPrototypeFromCtor(newTarget, r.global.Set, r.global.SetPrototype)
	o := &Object{runtime: r}

	so := &setObject{}
	so.class = classSet
	so.val = o
	so.extensible = true
	o.self = so
	so.prototype = proto
	so.init()
	if len(args) > 0 {
		if arg := args[0]; arg != nil && arg != _undefined && arg != _null {
			adder := so.getStr("add", nil)
			iter := r.getIterator(arg, nil)
			if adder == r.global.setAdder {
				r.iterate(iter, func(item Value) {
					so.m.set(item, nil)
				})
			} else {
				adderFn := toMethod(adder)
				if adderFn == nil {
					panic(r.NewTypeError("Set.add in missing"))
				}
				r.iterate(iter, func(item Value) {
					adderFn(FunctionCall{This: o, Arguments: []Value{item}})
				})
			}
		}
	}
	return o
}

func (r *Runtime) createSetIterator(setValue Value, kind iterationKind) Value {
	obj := r.toObject(setValue)
	setObj, ok := obj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Object is not a Set"))
	}

	o := &Object{runtime: r}

	si := &setIterObject{
		iter: setObj.m.newIter(),
		kind: kind,
	}
	si.class = classSetIterator
	si.val = o
	si.extensible = true
	o.self = si
	si.prototype = r.global.SetIteratorPrototype
	si.init()

	return o
}

func (r *Runtime) setIterProto_next(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	if iter, ok := thisObj.self.(*setIterObject); ok {
		return iter.next()
	}
	panic(r.NewTypeError("Method Set Iterator.prototype.next called on incompatible receiver %s", thisObj.String()))
}

func (r *Runtime) createSetProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.global.ObjectPrototype, classObject)

	o._putProp("constructor", r.global.Set, true, false, true)
	r.global.setAdder = r.newNativeFunc(r.setProto_add, nil, "add", nil, 1)
	o._putProp("add", r.global.setAdder, true, false, true)

	o._putProp("clear", r.newNativeFunc(r.setProto_clear, nil, "clear", nil, 0), true, false, true)
	o._putProp("delete", r.newNativeFunc(r.setProto_delete, nil, "delete", nil, 1), true, false, true)
	o._putProp("forEach", r.newNativeFunc(r.setProto_forEach, nil, "forEach", nil, 1), true, false, true)
	o._putProp("has", r.newNativeFunc(r.setProto_has, nil, "has", nil, 1), true, false, true)
	o.setOwnStr("size", &valueProperty{
		getterFunc:   r.newNativeFunc(r.setProto_getSize, nil, "get size", nil, 0),
		accessor:     true,
		writable:     true,
		configurable: true,
	}, true)

	valuesFunc := r.newNativeFunc(r.setProto_values, nil, "values", nil, 0)
	o._putProp("values", valuesFunc, true, false, true)
	o._putProp("keys", valuesFunc, true, false, true)
	o._putProp("entries", r.newNativeFunc(r.setProto_entries, nil, "entries", nil, 0), true, false, true)
	o._putSym(symIterator, valueProp(valuesFunc, true, false, true))
	o._putSym(symToStringTag, valueProp(asciiString(classSet), false, false, true))

	return o
}

func (r *Runtime) createSet(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.builtin_newSet, r.global.SetPrototype, "Set", 0)
	o._putSym(symSpecies, &valueProperty{
		getterFunc:   r.newNativeFunc(r.returnThis, nil, "get [Symbol.species]", nil, 0),
		accessor:     true,
		configurable: true,
	})

	return o
}

func (r *Runtime) createSetIterProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.global.IteratorPrototype, classObject)

	o._putProp("next", r.newNativeFunc(r.setIterProto_next, nil, "next", nil, 0), true, false, true)
	o._putSym(symToStringTag, valueProp(asciiString(classSetIterator), false, false, true))

	return o
}

func (r *Runtime) initSet() {
	r.global.SetIteratorPrototype = r.newLazyObject(r.createSetIterProto)

	r.global.SetPrototype = r.newLazyObject(r.createSetProto)
	r.global.Set = r.newLazyObject(r.createSet)

	r.addToGlobal("Set", r.global.Set)
}
