package goja

import "sync"

type weakSet struct {
	// need to synchronise access to the data map because it may be accessed
	// from the finalizer goroutine
	sync.Mutex
	data map[uint64]struct{}
}

type weakSetObject struct {
	baseObject
	s *weakSet
}

func newWeakSet() *weakSet {
	return &weakSet{
		data: make(map[uint64]struct{}),
	}
}

func (ws *weakSetObject) init() {
	ws.baseObject.init()
	ws.s = newWeakSet()
}

func (ws *weakSet) removeId(id uint64) {
	ws.Lock()
	delete(ws.data, id)
	ws.Unlock()
}

func (ws *weakSet) add(o *Object) {
	refs := o.getWeakCollRefs()
	ws.Lock()
	ws.data[refs.id()] = struct{}{}
	ws.Unlock()
	refs.add(ws)
}

func (ws *weakSet) remove(o *Object) bool {
	if o.weakColls == nil {
		return false
	}
	id := o.weakColls.id()
	ws.Lock()
	_, exists := ws.data[id]
	if exists {
		delete(ws.data, id)
	}
	ws.Unlock()
	if exists {
		o.weakColls.remove(ws)
	}
	return exists
}

func (ws *weakSet) has(o *Object) bool {
	if o.weakColls == nil {
		return false
	}
	ws.Lock()
	_, exists := ws.data[o.weakColls.id()]
	ws.Unlock()
	return exists
}

func (r *Runtime) weakSetProto_add(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wso, ok := thisObj.self.(*weakSetObject)
	if !ok {
		panic(r.NewTypeError("Method WeakSet.prototype.add called on incompatible receiver %s", thisObj.String()))
	}
	wso.s.add(r.toObject(call.Argument(0)))
	return call.This
}

func (r *Runtime) weakSetProto_delete(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	wso, ok := thisObj.self.(*weakSetObject)
	if !ok {
		panic(r.NewTypeError("Method WeakSet.prototype.delete called on incompatible receiver %s", thisObj.String()))
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
		panic(r.NewTypeError("Method WeakSet.prototype.has called on incompatible receiver %s", thisObj.String()))
	}
	obj, ok := call.Argument(0).(*Object)
	if ok && wso.s.has(obj) {
		return valueTrue
	}
	return valueFalse
}

func (r *Runtime) populateWeakSetGeneric(s *Object, adderValue Value, iterable Value) {
	adder := toMethod(adderValue)
	if adder == nil {
		panic(r.NewTypeError("WeakSet.add is not set"))
	}
	iter := r.getIterator(iterable, nil)
	r.iterate(iter, func(val Value) {
		adder(FunctionCall{This: s, Arguments: []Value{val}})
	})
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
			if adder == r.global.weakSetAdder {
				if arr := r.checkStdArrayIter(arg); arr != nil {
					for _, v := range arr.values {
						wso.s.add(r.toObject(v))
					}
					return o
				}
			}
			r.populateWeakSetGeneric(o, adder, arg)
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

	o._putSym(symToStringTag, valueProp(asciiString(classWeakSet), false, false, true))

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
