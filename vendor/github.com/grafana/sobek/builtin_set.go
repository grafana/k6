package sobek

import (
	"fmt"
	"reflect"
)

var setExportType = reflectTypeArray

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

func (so *setObject) exportType() reflect.Type {
	return setExportType
}

func (so *setObject) export(ctx *objectExportCtx) interface{} {
	a := make([]interface{}, so.m.size)
	ctx.put(so.val, a)
	iter := so.m.newIter()
	for i := 0; i < len(a); i++ {
		entry := iter.next()
		if entry == nil {
			break
		}
		a[i] = exportValue(entry.key, ctx)
	}
	return a
}

func (so *setObject) exportToArrayOrSlice(dst reflect.Value, typ reflect.Type, ctx *objectExportCtx) error {
	l := so.m.size
	if typ.Kind() == reflect.Array {
		if dst.Len() != l {
			return fmt.Errorf("cannot convert a Set into an array, lengths mismatch: have %d, need %d)", l, dst.Len())
		}
	} else {
		dst.Set(reflect.MakeSlice(typ, l, l))
	}
	ctx.putTyped(so.val, typ, dst.Interface())
	iter := so.m.newIter()
	r := so.val.runtime
	for i := 0; i < l; i++ {
		entry := iter.next()
		if entry == nil {
			break
		}
		err := r.toReflectValue(entry.key, dst.Index(i), ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (so *setObject) exportToMap(dst reflect.Value, typ reflect.Type, ctx *objectExportCtx) error {
	dst.Set(reflect.MakeMap(typ))
	keyTyp := typ.Key()
	elemTyp := typ.Elem()
	iter := so.m.newIter()
	r := so.val.runtime
	for {
		entry := iter.next()
		if entry == nil {
			break
		}
		keyVal := reflect.New(keyTyp).Elem()
		err := r.toReflectValue(entry.key, keyVal, ctx)
		if err != nil {
			return err
		}
		dst.SetMapIndex(keyVal, reflect.Zero(elemTyp))
	}
	return nil
}

func (r *Runtime) newSet(prototype *Object) *setObject {
	o := &Object{runtime: r}
	so := &setObject{}
	so.class = classObject
	so.val = o
	so.extensible = true
	o.self = so
	so.prototype = prototype
	so.init()
	return so
}

func (r *Runtime) newSetObject() *setObject {
	return r.newSet(r.getSetPrototype())
}

func (r *Runtime) setProto_add(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method Set.prototype.add called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}

	so.m.set(call.Argument(0), nil)
	return call.This
}

func (r *Runtime) setProto_clear(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method Set.prototype.clear called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}

	so.m.clear()
	return _undefined
}

func (r *Runtime) setProto_delete(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method Set.prototype.delete called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
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
		panic(r.NewTypeError("Method Set.prototype.forEach called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
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
		panic(r.NewTypeError("Method Set.prototype.has called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}

	return r.toBoolean(so.m.has(call.Argument(0)))
}

func (r *Runtime) setProto_getSize(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method get Set.prototype.size called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}

	return intToValue(int64(so.m.size))
}

func (r *Runtime) setProto_difference(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method Set.prototype.difference called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	otherRecord := r.getSetRecord(call.Argument(0))
	result := r.newSetObject()
	// 4. Let resultSetData be a copy of set.[[SetData]].

	// 5. If SetDataSize(set.[[SetData]]) ≤ otherRecord.[[Size]], then
	if so.m.size <= otherRecord.size {
		// We can skip the copy if we are certain that the other set is a native Set object
		if otherRecord.isStd() {
			otherSet := otherRecord.stdObj
			iter := so.m.newIter()
			for {
				entry := iter.next()
				if entry == nil {
					break
				}
				if !otherSet.m.has(entry.key) {
					result.m.set(entry.key, nil)
				}
			}
		} else {
			so.m.copyTo(result.m)
			iter := so.m.newIter()
			for {
				entry := iter.next()
				if entry == nil {
					break
				}
				if otherRecord.has(entry.key) {
					result.m.remove(entry.key)
				}
			}
		}
	} else {
		so.m.copyTo(result.m)
		otherRecord.iterateKeys(func(key Value) bool {
			result.m.remove(key)
			return true
		})
	}
	return result.val
}

func (r *Runtime) setProto_intersection(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method Set.prototype.intersection called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	otherRecord := r.getSetRecord(call.Argument(0))
	result := r.newSetObject()

	if so.m.size <= otherRecord.size {
		iter := so.m.newIter()
		for {
			entry := iter.next()
			if entry == nil {
				break
			}
			if otherRecord.has(entry.key) {
				result.m.set(entry.key, nil)
			}
		}
	} else {
		otherRecord.iterateKeys(func(key Value) bool {
			if so.m.has(key) {
				result.m.set(key, nil)
			}
			return true
		})
	}
	return result.val
}

func (r *Runtime) setProto_isDisjointFrom(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method Set.prototype.isDisjointFrom called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	otherRecord := r.getSetRecord(call.Argument(0))

	if so.m.size <= otherRecord.size {
		iter := so.m.newIter()
		for {
			entry := iter.next()
			if entry == nil {
				break
			}
			if otherRecord.has(entry.key) {
				return valueFalse
			}
		}
		return valueTrue
	}

	result := valueTrue
	otherRecord.iterateKeys(func(key Value) bool {
		if so.m.has(key) {
			result = valueFalse
			return false
		}
		return true
	})
	return result
}

func (r *Runtime) setProto_isSubsetOf(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method Set.prototype.isSubsetOf called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	otherRecord := r.getSetRecord(call.Argument(0))

	if so.m.size > otherRecord.size {
		return valueFalse
	}

	iter := so.m.newIter()
	for {
		entry := iter.next()
		if entry == nil {
			break
		}
		if !otherRecord.has(entry.key) {
			return valueFalse
		}
	}
	return valueTrue
}

func (r *Runtime) setProto_isSupersetOf(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method Set.prototype.isSupersetOf called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	otherRecord := r.getSetRecord(call.Argument(0))

	if so.m.size < otherRecord.size {
		return valueFalse
	}

	result := valueTrue
	otherRecord.iterateKeys(func(key Value) bool {
		if !so.m.has(key) {
			result = valueFalse
			return false
		}
		return true
	})
	return result
}

func (r *Runtime) setProto_symmetricDifference(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method Set.prototype.symmetricDifference called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	otherRecord := r.getSetRecord(call.Argument(0))
	result := r.newSetObject()
	so.m.copyTo(result.m)

	otherRecord.iterateKeys(func(key Value) bool {
		// ii. Let resultIndex be SetDataIndex(resultSetData, next).
		// iii. If resultIndex is not-found, let alreadyInResult be false; else let alreadyInResult be true.
		alreadyInResult := result.m.has(key)
		// iv. If SetDataHas(set.[[SetData]], next) is true, then
		if so.m.has(key) {
			// 1. If alreadyInResult is true, set resultSetData[resultIndex] to empty.
			if alreadyInResult {
				result.m.remove(key)
			}
			// v. Else,
		} else {
			// 1. If alreadyInResult is false, append next to resultSetData.
			if !alreadyInResult {
				result.m.set(key, nil)
			}
		}
		return true
	})
	return result.val
}

func (r *Runtime) setProto_union(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	so, ok := thisObj.self.(*setObject)
	if !ok {
		panic(r.NewTypeError("Method Set.prototype.union called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	otherRecord := r.getSetRecord(call.Argument(0))
	result := r.newSetObject()
	so.m.copyTo(result.m)

	otherRecord.iterateKeys(func(key Value) bool {
		result.m.set(key, nil)
		return true
	})
	return result.val
}

func (r *Runtime) setProto_values(call FunctionCall) Value {
	return r.createSetIterator(call.This, iterationKindValue)
}

func (r *Runtime) builtin_newSet(args []Value, newTarget *Object) *Object {
	if newTarget == nil {
		panic(r.needNew("Set"))
	}
	proto := r.getPrototypeFromCtor(newTarget, r.global.Set, r.global.SetPrototype)
	so := r.newSet(proto)
	o := so.val
	if len(args) > 0 {
		if arg := args[0]; arg != nil && arg != _undefined && arg != _null {
			adder := so.getStr("add", nil)
			stdArr := r.checkStdArrayIter(arg)
			if adder == r.global.setAdder {
				if stdArr != nil {
					for _, v := range stdArr.values {
						so.m.set(v, nil)
					}
				} else {
					r.getIterator(arg, nil).iterate(func(item Value) {
						so.m.set(item, nil)
					})
				}
			} else {
				adderFn := toMethod(adder)
				if adderFn == nil {
					panic(r.NewTypeError("Set.add in missing"))
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
	si.class = classObject
	si.val = o
	si.extensible = true
	o.self = si
	si.prototype = r.getSetIteratorPrototype()
	si.init()

	return o
}

func (r *Runtime) setIterProto_next(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	if iter, ok := thisObj.self.(*setIterObject); ok {
		return iter.next()
	}
	panic(r.NewTypeError("Method Set Iterator.prototype.next called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
}

type setRecord struct {
	o           *Object
	size        int
	has         func(Value) bool
	iterateKeys func(func(Value) bool)
	stdObj      *setObject
}

func (sr *setRecord) isStd() bool {
	return sr.stdObj != nil
}

func (r *Runtime) getSetRecord(value Value) *setRecord {
	// 1. If obj is not an Object, throw a TypeError exception.
	o, ok := value.(*Object)
	if !ok {
		panic(r.NewTypeError("value is not an Object"))
	}

	// 2. Let rawSize be ? Get(obj, "size").
	rawSize := nilSafe(o.self.getStr("size", nil))
	// 3. Let numberSize be ? ToNumber(rawSize).
	// 4. NOTE: If rawSize is undefined, then numberSize will be NaN.
	numberSize := rawSize.ToNumber()
	// 5. If numberSize is NaN, throw a TypeError exception.
	if IsNaN(numberSize) {
		panic(r.NewTypeError("size is NaN"))
	}
	// 6. Let intSize be ! ToIntegerOrInfinity(numberSize).
	intSize := toInt(numberSize)
	// 7. If intSize < 0, throw a RangeError exception.
	if intSize < 0 {
		panic(r.newErrorf(r.getRangeError(), "Invalid size: %d", intSize))
	}

	// 8. Let has be ? Get(obj, "has").
	has := o.self.getStr("has", nil)
	// 9. If IsCallable(has) is false, throw a TypeError exception.
	hasFn := toMethod(has)
	if hasFn == nil {
		panic(r.NewTypeError("Object does not have a valid 'has' method"))
	}

	// 10. Let keys be ? Get(obj, "keys").
	keys := o.self.getStr("keys", nil)
	// 11. If IsCallable(keys) is false, throw a TypeError exception.
	keysFn := toMethod(keys)
	if keysFn == nil {
		panic(r.NewTypeError("Object does not have a valid 'keys' method"))
	}

	// 12. Return a new Set Record { [[SetObject]]: obj, [[Size]]: intSize, [[Has]]: has, [[Keys]]: keys }.

	// common case for native Set objects
	if setObj, ok := o.self.(*setObject); ok && has == r.global.setHas && keys == r.global.setValues {
		return &setRecord{
			o:      o,
			size:   intSize,
			stdObj: setObj,
			has:    setObj.m.has,
			iterateKeys: func(f func(Value) bool) {
				iter := setObj.m.newIter()
				for {
					entry := iter.next()
					if entry == nil {
						break
					}
					if !f(entry.key) {
						break
					}
				}
			},
		}
	}

	return &setRecord{
		o:    o,
		size: intSize,
		has: func(v Value) bool {
			return nilSafe(hasFn(FunctionCall{This: o, Arguments: []Value{v}})).ToBoolean()
		},
		iterateKeys: func(f func(Value) bool) {
			r.forOfMethod(o, keysFn, f)
		},
	}
}

func (r *Runtime) createSetProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.global.ObjectPrototype, classObject)

	o._putProp("constructor", r.getSet(), true, false, true)
	r.global.setAdder = r.newNativeFunc(r.setProto_add, "add", 1)
	o._putProp("add", r.global.setAdder, true, false, true)

	o._putProp("clear", r.newNativeFunc(r.setProto_clear, "clear", 0), true, false, true)
	o._putProp("delete", r.newNativeFunc(r.setProto_delete, "delete", 1), true, false, true)
	o._putProp("forEach", r.newNativeFunc(r.setProto_forEach, "forEach", 1), true, false, true)
	r.global.setHas = r.newNativeFunc(r.setProto_has, "has", 1)
	o._putProp("has", r.global.setHas, true, false, true)
	o.setOwnStr("size", &valueProperty{
		getterFunc:   r.newNativeFunc(r.setProto_getSize, "get size", 0),
		accessor:     true,
		writable:     true,
		configurable: true,
	}, true)

	o._putProp("difference", r.newNativeFunc(r.setProto_difference, "difference", 1), true, false, true)
	o._putProp("intersection", r.newNativeFunc(r.setProto_intersection, "intersection", 1), true, false, true)
	o._putProp("isDisjointFrom", r.newNativeFunc(r.setProto_isDisjointFrom, "isDisjointFrom", 1), true, false, true)
	o._putProp("isSubsetOf", r.newNativeFunc(r.setProto_isSubsetOf, "isSubsetOf", 1), true, false, true)
	o._putProp("isSupersetOf", r.newNativeFunc(r.setProto_isSupersetOf, "isSupersetOf", 1), true, false, true)
	o._putProp("symmetricDifference", r.newNativeFunc(r.setProto_symmetricDifference, "symmetricDifference", 1), true, false, true)
	o._putProp("union", r.newNativeFunc(r.setProto_union, "union", 1), true, false, true)

	r.global.setValues = r.newNativeFunc(r.setProto_values, "values", 0)
	o._putProp("values", r.global.setValues, true, false, true)
	o._putProp("keys", r.global.setValues, true, false, true)
	o._putProp("entries", r.newNativeFunc(r.setProto_entries, "entries", 0), true, false, true)
	o._putSym(SymIterator, valueProp(r.global.setValues, true, false, true))
	o._putSym(SymToStringTag, valueProp(asciiString(classSet), false, false, true))

	return o
}

func (r *Runtime) createSet(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.builtin_newSet, r.getSetPrototype(), "Set", 0)
	r.putSpeciesReturnThis(o)

	return o
}

func (r *Runtime) createSetIterProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.getIteratorPrototype(), classObject)

	o._putProp("next", r.newNativeFunc(r.setIterProto_next, "next", 0), true, false, true)
	o._putSym(SymToStringTag, valueProp(asciiString(classSetIterator), false, false, true))

	return o
}

func (r *Runtime) getSetIteratorPrototype() *Object {
	var o *Object
	if o = r.global.SetIteratorPrototype; o == nil {
		o = &Object{runtime: r}
		r.global.SetIteratorPrototype = o
		o.self = r.createSetIterProto(o)
	}
	return o
}

func (r *Runtime) getSetPrototype() *Object {
	ret := r.global.SetPrototype
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.SetPrototype = ret
		ret.self = r.createSetProto(ret)
	}
	return ret
}

func (r *Runtime) getSet() *Object {
	ret := r.global.Set
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Set = ret
		ret.self = r.createSet(ret)
	}
	return ret
}
