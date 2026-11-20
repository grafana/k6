package sobek

import (
	"runtime"
	"sync"
	"weak"
)

type weakMapRefsValue struct {
	value Value

	// Keep these so that they can be stopped after an entry is removed from the weakMap to prevent memory leaks
	keyCleanup, wmCleanup runtime.Cleanup
}

type weakMapRefs struct {
	m map[weak.Pointer[weakMap]]*weakMapRefsValue
	sync.Mutex
}

func newWeakMapRefs() *weakMapRefs {
	return &weakMapRefs{
		m: make(map[weak.Pointer[weakMap]]*weakMapRefsValue),
	}
}

type weakMap struct {
	// Make sure it's not zero-sized and is not tiny-allocated so that runtime.Cleanup runs on it
	_ [16]byte
}

func (wm *weakMap) set(refs *weakMapRefs, value Value) {
	type wmCleanupArg struct {
		refPtr weak.Pointer[weakMapRefs]
		wmPtr  weak.Pointer[weakMap]
	}

	p := weak.Make(wm)
	refs.Lock()
	if v, exists := refs.m[p]; !exists {
		// This is to ensure keys do not keep values alive after weakMap becomes unreachable
		wmCleanup := runtime.AddCleanup(wm, func(arg wmCleanupArg) {
			refs := arg.refPtr.Value()
			if refs == nil {
				return
			}
			refs.Lock()
			if v, exists := refs.m[arg.wmPtr]; exists {
				v.keyCleanup.Stop()
				delete(refs.m, arg.wmPtr)
			}
			refs.Unlock()
		}, wmCleanupArg{weak.Make(refs), p})

		// This is to prevent wmCleanup from taking memory after key becomes unreachable (which makes wmCleanup a no-op)
		keyCleanup := runtime.AddCleanup(refs, func(c runtime.Cleanup) {
			c.Stop()
		}, wmCleanup)

		refs.m[p] = &weakMapRefsValue{
			value:      value,
			keyCleanup: keyCleanup,
			wmCleanup:  wmCleanup,
		}
	} else {
		v.value = value
	}
	refs.Unlock()
}

func (wm *weakMap) get(refs *weakMapRefs) (res Value) {
	if refs != nil {
		p := weak.Make(wm)
		refs.Lock()
		if v, exists := refs.m[p]; exists {
			res = v.value
		}
		refs.Unlock()
	}
	return
}

func (wm *weakMap) remove(refs *weakMapRefs) (removed bool) {
	if refs != nil {
		p := weak.Make(wm)
		refs.Lock()
		if v, exists := refs.m[p]; exists {
			delete(refs.m, p)
			v.wmCleanup.Stop()
			v.keyCleanup.Stop()
			removed = true
		}
		refs.Unlock()
	}
	return
}

func (wm *weakMap) has(refs *weakMapRefs) (exists bool) {
	if refs != nil {
		p := weak.Make(wm)
		refs.Lock()
		_, exists = refs.m[p]
		refs.Unlock()
	}
	return
}

type weakMapObject struct {
	baseObject
	m weakMap
}

func (wmo *weakMapObject) init() {
	wmo.baseObject.init()
}

func (r *Runtime) weakMapProto_delete(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wmo, ok := thisObj.self.(*weakMapObject)
	if !ok {
		panic(r.NewTypeError("Method WeakMap.prototype.delete called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	if wmo.m.remove(r.getWeakMapRefs(call.Argument(0), false)) {
		return valueTrue
	}
	return valueFalse
}

func (r *Runtime) weakMapProto_get(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wmo, ok := thisObj.self.(*weakMapObject)
	if !ok {
		panic(r.NewTypeError("Method WeakMap.prototype.get called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	res := wmo.m.get(r.getWeakMapRefs(call.Argument(0), false))
	if res == nil {
		return _undefined
	}
	return res
}

func (r *Runtime) weakMapProto_getOrInsert(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wmo, ok := thisObj.self.(*weakMapObject)
	if !ok {
		panic(r.NewTypeError("Method WeakMap.prototype.getOrInsert called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	refs := r.weakMapKeyRefs(call.Argument(0))
	if v := wmo.m.get(refs); v != nil {
		return v
	}
	value := call.Argument(1)
	wmo.m.set(refs, value)
	return value
}

func (r *Runtime) weakMapProto_getOrInsertComputed(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wmo, ok := thisObj.self.(*weakMapObject)
	if !ok {
		panic(r.NewTypeError("Method WeakMap.prototype.getOrInsertComputed called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	key := call.Argument(0)
	refs := r.weakMapKeyRefs(key)
	callbackFn := r.toCallable(call.Argument(1))
	if v := wmo.m.get(refs); v != nil {
		return v
	}
	value := callbackFn(FunctionCall{This: _undefined, Arguments: []Value{key}})
	wmo.m.set(refs, value)
	return value
}

func (r *Runtime) weakMapProto_has(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wmo, ok := thisObj.self.(*weakMapObject)
	if !ok {
		panic(r.NewTypeError("Method WeakMap.prototype.has called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	if wmo.m.has(r.getWeakMapRefs(call.Argument(0), false)) {
		return valueTrue
	}
	return valueFalse
}

func (r *Runtime) weakMapProto_set(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wmo, ok := thisObj.self.(*weakMapObject)
	if !ok {
		panic(r.NewTypeError("Method WeakMap.prototype.set called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
	}
	wmo.m.set(r.weakMapKeyRefs(call.Argument(0)), call.Argument(1))
	return call.This
}

// getWeakMapRefs returns the weakMapRefs of v, or nil if v cannot be held weakly
// (https://tc39.es/ecma262/#sec-canbeheldweakly) or, unless create is true, if v has never been used as a weak key.
func (r *Runtime) getWeakMapRefs(v Value, create bool) *weakMapRefs {
	switch v := v.(type) {
	case *Object:
		return v.getWeakMapRefs(create)
	case *Symbol:
		if !r.isRegisteredSymbol(v) {
			return v.getWeakMapRefs(create)
		}
	}
	return nil
}

func (r *Runtime) weakMapKeyRefs(key Value) *weakMapRefs {
	if refs := r.getWeakMapRefs(key, true); refs != nil {
		return refs
	}
	panic(r.NewTypeError("Invalid value used as weak map key: %s", nilSafe(key).String()))
}

func (r *Runtime) needNew(name string) *Object {
	return r.NewTypeError("Constructor %s requires 'new'", name)
}

func (r *Runtime) builtin_newWeakMap(args []Value, newTarget *Object) *Object {
	if newTarget == nil {
		panic(r.needNew("WeakMap"))
	}
	proto := r.getPrototypeFromCtor(newTarget, r.global.WeakMap, r.global.WeakMapPrototype)
	o := &Object{runtime: r}

	wmo := &weakMapObject{}
	wmo.class = classObject
	wmo.val = o
	wmo.extensible = true
	o.self = wmo
	wmo.prototype = proto
	wmo.init()
	if len(args) > 0 {
		if arg := args[0]; arg != nil && arg != _undefined && arg != _null {
			adder := wmo.getStr("set", nil)
			adderFn := toMethod(adder)
			if adderFn == nil {
				panic(r.NewTypeError("WeakMap.set in missing"))
			}
			iter := r.getIterator(arg, nil)
			i0 := valueInt(0)
			i1 := valueInt(1)
			if adder == r.global.weakMapAdder {
				iter.iterate(func(item Value) {
					itemObj := r.toObject(item)
					k := itemObj.self.getIdx(i0, nil)
					v := nilSafe(itemObj.self.getIdx(i1, nil))
					wmo.m.set(r.weakMapKeyRefs(k), v)
				})
			} else {
				iter.iterate(func(item Value) {
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

	o._putProp("constructor", r.getWeakMap(), true, false, true)
	r.global.weakMapAdder = r.newNativeFunc(r.weakMapProto_set, "set", 2)
	o._putProp("set", r.global.weakMapAdder, true, false, true)
	o._putProp("delete", r.newNativeFunc(r.weakMapProto_delete, "delete", 1), true, false, true)
	o._putProp("has", r.newNativeFunc(r.weakMapProto_has, "has", 1), true, false, true)
	o._putProp("get", r.newNativeFunc(r.weakMapProto_get, "get", 1), true, false, true)
	o._putProp("getOrInsert", r.newNativeFunc(r.weakMapProto_getOrInsert, "getOrInsert", 2), true, false, true)
	o._putProp("getOrInsertComputed", r.newNativeFunc(r.weakMapProto_getOrInsertComputed, "getOrInsertComputed", 2), true, false, true)

	o._putSym(SymToStringTag, valueProp(asciiString(classWeakMap), false, false, true))

	return o
}

func (r *Runtime) createWeakMap(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.builtin_newWeakMap, r.getWeakMapPrototype(), "WeakMap", 0)

	return o
}

func (r *Runtime) getWeakMapPrototype() *Object {
	ret := r.global.WeakMapPrototype
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.WeakMapPrototype = ret
		ret.self = r.createWeakMapProto(ret)
	}
	return ret
}

func (r *Runtime) getWeakMap() *Object {
	ret := r.global.WeakMap
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.WeakMap = ret
		ret.self = r.createWeakMap(ret)
	}
	return ret
}
