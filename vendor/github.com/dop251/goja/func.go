package goja

import (
	"fmt"
	"reflect"

	"github.com/dop251/goja/unistring"
)

type resultType uint8

const (
	resultNormal resultType = iota
	resultYield
	resultAwait
)

var (
	resultAwaitMarker = NewSymbol("await")
)

// AsyncContextTracker is a handler that allows to track async function's execution context. Every time an async
// function is suspended on 'await', Suspended() is called. The trackingObject it returns is remembered and
// the next time just before the context is resumed, Resumed is called with the same trackingObject as argument.
// Completed is called when an async function returns or throws.
// To register it call Runtime.SetAsyncContextTracker().
type AsyncContextTracker interface {
	Suspended() (trackingObject interface{})
	Resumed(trackingObject interface{})
	Completed()
}

type funcObjectImpl interface {
	source() valueString
}

type baseFuncObject struct {
	baseObject

	lenProp valueProperty
}

type baseJsFuncObject struct {
	baseFuncObject

	stash   *stash
	privEnv *privateEnv

	prg    *Program
	src    string
	strict bool
}

type funcObject struct {
	baseJsFuncObject
}

type asyncFuncObject struct {
	baseJsFuncObject
}

type classFuncObject struct {
	baseJsFuncObject
	initFields   *Program
	computedKeys []Value

	privateEnvType *privateEnvType
	privateMethods []Value

	derived bool
}

type methodFuncObject struct {
	baseJsFuncObject
	homeObject *Object
}

type asyncMethodFuncObject struct {
	methodFuncObject
}

type arrowFuncObject struct {
	baseJsFuncObject
	funcObj   *Object
	newTarget Value
}

type asyncArrowFuncObject struct {
	arrowFuncObject
}

type nativeFuncObject struct {
	baseFuncObject

	f         func(FunctionCall) Value
	construct func(args []Value, newTarget *Object) *Object
}

type wrappedFuncObject struct {
	nativeFuncObject
	wrapped reflect.Value
}

type boundFuncObject struct {
	nativeFuncObject
	wrapped *Object
}

func (f *nativeFuncObject) source() valueString {
	return newStringValue(fmt.Sprintf("function %s() { [native code] }", nilSafe(f.getStr("name", nil)).toString()))
}

func (f *nativeFuncObject) export(*objectExportCtx) interface{} {
	return f.f
}

func (f *wrappedFuncObject) exportType() reflect.Type {
	return f.wrapped.Type()
}

func (f *wrappedFuncObject) export(*objectExportCtx) interface{} {
	return f.wrapped.Interface()
}

func (f *funcObject) _addProto(n unistring.String) Value {
	if n == "prototype" {
		if _, exists := f.values[n]; !exists {
			return f.addPrototype()
		}
	}
	return nil
}

func (f *funcObject) getStr(p unistring.String, receiver Value) Value {
	return f.getStrWithOwnProp(f.getOwnPropStr(p), p, receiver)
}

func (f *funcObject) getOwnPropStr(name unistring.String) Value {
	if v := f._addProto(name); v != nil {
		return v
	}

	return f.baseObject.getOwnPropStr(name)
}

func (f *funcObject) setOwnStr(name unistring.String, val Value, throw bool) bool {
	f._addProto(name)
	return f.baseObject.setOwnStr(name, val, throw)
}

func (f *funcObject) setForeignStr(name unistring.String, val, receiver Value, throw bool) (bool, bool) {
	return f._setForeignStr(name, f.getOwnPropStr(name), val, receiver, throw)
}

func (f *funcObject) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	f._addProto(name)
	return f.baseObject.defineOwnPropertyStr(name, descr, throw)
}

func (f *funcObject) deleteStr(name unistring.String, throw bool) bool {
	f._addProto(name)
	return f.baseObject.deleteStr(name, throw)
}

func (f *funcObject) addPrototype() Value {
	proto := f.val.runtime.NewObject()
	proto.self._putProp("constructor", f.val, true, false, true)
	return f._putProp("prototype", proto, true, false, false)
}

func (f *funcObject) hasOwnPropertyStr(name unistring.String) bool {
	if f.baseObject.hasOwnPropertyStr(name) {
		return true
	}

	if name == "prototype" {
		return true
	}
	return false
}

func (f *funcObject) stringKeys(all bool, accum []Value) []Value {
	if all {
		if _, exists := f.values["prototype"]; !exists {
			accum = append(accum, asciiString("prototype"))
		}
	}
	return f.baseFuncObject.stringKeys(all, accum)
}

func (f *funcObject) iterateStringKeys() iterNextFunc {
	if _, exists := f.values["prototype"]; !exists {
		f.addPrototype()
	}
	return f.baseFuncObject.iterateStringKeys()
}

func (f *baseFuncObject) createInstance(newTarget *Object) *Object {
	r := f.val.runtime
	if newTarget == nil {
		newTarget = f.val
	}
	proto := r.getPrototypeFromCtor(newTarget, nil, r.global.ObjectPrototype)

	return f.val.runtime.newBaseObject(proto, classObject).val
}

func (f *baseJsFuncObject) source() valueString {
	return newStringValue(f.src)
}

func (f *baseJsFuncObject) construct(args []Value, newTarget *Object) *Object {
	if newTarget == nil {
		newTarget = f.val
	}
	proto := newTarget.self.getStr("prototype", nil)
	var protoObj *Object
	if p, ok := proto.(*Object); ok {
		protoObj = p
	} else {
		protoObj = f.val.runtime.global.ObjectPrototype
	}

	obj := f.val.runtime.newBaseObject(protoObj, classObject).val
	ret := f.call(FunctionCall{
		This:      obj,
		Arguments: args,
	}, newTarget)

	if ret, ok := ret.(*Object); ok {
		return ret
	}
	return obj
}

func (f *classFuncObject) Call(FunctionCall) Value {
	panic(f.val.runtime.NewTypeError("Class constructor cannot be invoked without 'new'"))
}

func (f *classFuncObject) assertCallable() (func(FunctionCall) Value, bool) {
	return f.Call, true
}

func (f *classFuncObject) vmCall(vm *vm, n int) {
	f.Call(FunctionCall{})
}

func (f *classFuncObject) export(*objectExportCtx) interface{} {
	return f.Call
}

func (f *classFuncObject) createInstance(args []Value, newTarget *Object) (instance *Object) {
	if f.derived {
		if ctor := f.prototype.self.assertConstructor(); ctor != nil {
			instance = ctor(args, newTarget)
		} else {
			panic(f.val.runtime.NewTypeError("Super constructor is not a constructor"))
		}
	} else {
		instance = f.baseFuncObject.createInstance(newTarget)
	}
	return
}

func (f *classFuncObject) _initFields(instance *Object) {
	if f.privateEnvType != nil {
		penv := instance.self.getPrivateEnv(f.privateEnvType, true)
		penv.methods = f.privateMethods
	}
	if f.initFields != nil {
		vm := f.val.runtime.vm
		vm.pushCtx()
		vm.prg = f.initFields
		vm.stash = f.stash
		vm.privEnv = f.privEnv
		vm.newTarget = nil

		// so that 'super' base could be correctly resolved (including from direct eval())
		vm.push(f.val)

		vm.sb = vm.sp
		vm.push(instance)
		vm.pc = 0
		ex := vm.runTry()
		vm.popCtx()
		if ex != nil {
			panic(ex)
		}
		vm.sp -= 2
	}
}

func (f *classFuncObject) construct(args []Value, newTarget *Object) *Object {
	if newTarget == nil {
		newTarget = f.val
	}
	if f.prg == nil {
		instance := f.createInstance(args, newTarget)
		f._initFields(instance)
		return instance
	} else {
		var instance *Object
		var thisVal Value
		if !f.derived {
			instance = f.createInstance(args, newTarget)
			f._initFields(instance)
			thisVal = instance
		}
		ret := f._call(args, newTarget, thisVal)

		if ret, ok := ret.(*Object); ok {
			return ret
		}
		if f.derived {
			r := f.val.runtime
			if ret != _undefined {
				panic(r.NewTypeError("Derived constructors may only return object or undefined"))
			}
			if v := r.vm.stack[r.vm.sp+1]; v != nil { // using residual 'this' value (a bit hacky)
				instance = r.toObject(v)
			} else {
				panic(r.newError(r.global.ReferenceError, "Must call super constructor in derived class before returning from derived constructor"))
			}
		}
		return instance
	}
}

func (f *classFuncObject) assertConstructor() func(args []Value, newTarget *Object) *Object {
	return f.construct
}

func (f *baseJsFuncObject) Call(call FunctionCall) Value {
	return f.call(call, nil)
}

func (f *arrowFuncObject) Call(call FunctionCall) Value {
	return f._call(call.Arguments, f.newTarget, nil)
}

func (f *baseJsFuncObject) __call(args []Value, newTarget, this Value) (Value, *Exception) {
	vm := f.val.runtime.vm

	vm.stack.expand(vm.sp + len(args) + 1)
	vm.stack[vm.sp] = f.val
	vm.sp++
	vm.stack[vm.sp] = this
	vm.sp++
	for _, arg := range args {
		if arg != nil {
			vm.stack[vm.sp] = arg
		} else {
			vm.stack[vm.sp] = _undefined
		}
		vm.sp++
	}

	vm.pushTryFrame(tryPanicMarker, -1)
	defer vm.popTryFrame()

	var needPop bool
	if vm.prg != nil {
		vm.pushCtx()
		vm.callStack = append(vm.callStack, context{pc: -2}) // extra frame so that run() halts after ret
		needPop = true
	} else {
		vm.pc = -2
		vm.pushCtx()
	}

	vm.args = len(args)
	vm.prg = f.prg
	vm.stash = f.stash
	vm.privEnv = f.privEnv
	vm.newTarget = newTarget
	vm.pc = 0
	for {
		ex := vm.runTryInner()
		if ex != nil {
			return nil, ex
		}
		if vm.halted() {
			break
		}
	}
	if needPop {
		vm.popCtx()
	}

	return vm.pop(), nil
}

func (f *baseJsFuncObject) _call(args []Value, newTarget, this Value) Value {
	res, ex := f.__call(args, newTarget, this)
	if ex != nil {
		panic(ex)
	}
	return res
}

func (f *baseJsFuncObject) call(call FunctionCall, newTarget Value) Value {
	return f._call(call.Arguments, newTarget, nilSafe(call.This))
}

func (f *baseJsFuncObject) export(*objectExportCtx) interface{} {
	return f.Call
}

func (f *baseFuncObject) exportType() reflect.Type {
	return reflectTypeFunc
}

func (f *baseFuncObject) typeOf() valueString {
	return stringFunction
}

func (f *baseJsFuncObject) assertCallable() (func(FunctionCall) Value, bool) {
	return f.Call, true
}

func (f *funcObject) assertConstructor() func(args []Value, newTarget *Object) *Object {
	return f.construct
}

func (f *baseJsFuncObject) vmCall(vm *vm, n int) {
	vm.pushCtx()
	vm.args = n
	vm.prg = f.prg
	vm.stash = f.stash
	vm.privEnv = f.privEnv
	vm.pc = 0
	vm.stack[vm.sp-n-1], vm.stack[vm.sp-n-2] = vm.stack[vm.sp-n-2], vm.stack[vm.sp-n-1]
}

func (f *arrowFuncObject) assertCallable() (func(FunctionCall) Value, bool) {
	return f.Call, true
}

func (f *arrowFuncObject) vmCall(vm *vm, n int) {
	vm.pushCtx()
	vm.args = n
	vm.prg = f.prg
	vm.stash = f.stash
	vm.privEnv = f.privEnv
	vm.pc = 0
	vm.stack[vm.sp-n-1], vm.stack[vm.sp-n-2] = nil, vm.stack[vm.sp-n-1]
	vm.newTarget = f.newTarget
}

func (f *arrowFuncObject) export(*objectExportCtx) interface{} {
	return f.Call
}

func (f *baseFuncObject) init(name unistring.String, length Value) {
	f.baseObject.init()

	f.lenProp.configurable = true
	f.lenProp.value = length
	f._put("length", &f.lenProp)

	f._putProp("name", stringValueFromRaw(name), false, false, true)
}

func (f *baseFuncObject) hasInstance(v Value) bool {
	if v, ok := v.(*Object); ok {
		o := f.val.self.getStr("prototype", nil)
		if o1, ok := o.(*Object); ok {
			for {
				v = v.self.proto()
				if v == nil {
					return false
				}
				if o1 == v {
					return true
				}
			}
		} else {
			f.val.runtime.typeErrorResult(true, "prototype is not an object")
		}
	}

	return false
}

func (f *nativeFuncObject) defaultConstruct(ccall func(ConstructorCall) *Object, args []Value, newTarget *Object) *Object {
	obj := f.createInstance(newTarget)
	ret := ccall(ConstructorCall{
		This:      obj,
		Arguments: args,
		NewTarget: newTarget,
	})

	if ret != nil {
		return ret
	}
	return obj
}

func (f *nativeFuncObject) assertCallable() (func(FunctionCall) Value, bool) {
	if f.f != nil {
		return f.f, true
	}
	return nil, false
}

func (f *nativeFuncObject) vmCall(vm *vm, n int) {
	if f.f != nil {
		vm.pushCtx()
		vm.prg = nil
		vm.sb = vm.sp - n // so that [sb-1] points to the callee
		ret := f.f(FunctionCall{
			Arguments: vm.stack[vm.sp-n : vm.sp],
			This:      vm.stack[vm.sp-n-2],
		})
		if ret == nil {
			ret = _undefined
		}
		vm.stack[vm.sp-n-2] = ret
		vm.popCtx()
	} else {
		vm.stack[vm.sp-n-2] = _undefined
	}
	vm.sp -= n + 1
	vm.pc++
}

func (f *nativeFuncObject) assertConstructor() func(args []Value, newTarget *Object) *Object {
	return f.construct
}

func (f *boundFuncObject) hasInstance(v Value) bool {
	return instanceOfOperator(v, f.wrapped)
}

func (f *baseJsFuncObject) asyncCall(call FunctionCall, vmCall func(*vm, int)) Value {
	vm := f.val.runtime.vm
	args := call.Arguments
	vm.stack.expand(vm.sp + len(args) + 1)
	vm.stack[vm.sp] = call.This
	vm.sp++
	vm.stack[vm.sp] = f.val
	vm.sp++
	for _, arg := range args {
		if arg != nil {
			vm.stack[vm.sp] = arg
		} else {
			vm.stack[vm.sp] = _undefined
		}
		vm.sp++
	}
	ar := &asyncRunner{
		f:      f.val,
		vmCall: vmCall,
	}
	ar.start(len(args))
	return ar.promiseCap.promise
}

func (f *asyncFuncObject) Call(call FunctionCall) Value {
	return f.asyncCall(call, f.baseJsFuncObject.vmCall)
}

func (f *asyncFuncObject) assertCallable() (func(FunctionCall) Value, bool) {
	return f.Call, true
}

func (f *asyncFuncObject) export(*objectExportCtx) interface{} {
	return f.Call
}

func (f *asyncArrowFuncObject) Call(call FunctionCall) Value {
	return f.asyncCall(call, f.arrowFuncObject.vmCall)
}

func (f *asyncArrowFuncObject) assertCallable() (func(FunctionCall) Value, bool) {
	return f.Call, true
}

func (f *asyncArrowFuncObject) export(*objectExportCtx) interface{} {
	return f.Call
}

func (f *asyncArrowFuncObject) vmCall(vm *vm, n int) {
	f.asyncVmCall(vm, n, f.arrowFuncObject.vmCall)
}

func (f *asyncMethodFuncObject) Call(call FunctionCall) Value {
	return f.asyncCall(call, f.methodFuncObject.vmCall)
}

func (f *asyncMethodFuncObject) assertCallable() (func(FunctionCall) Value, bool) {
	return f.Call, true
}

func (f *asyncMethodFuncObject) export(ctx *objectExportCtx) interface{} {
	return f.Call
}

func (f *asyncMethodFuncObject) vmCall(vm *vm, n int) {
	f.asyncVmCall(vm, n, f.methodFuncObject.vmCall)
}

func (f *baseJsFuncObject) asyncVmCall(vm *vm, n int, vmCall func(*vm, int)) {
	ar := &asyncRunner{
		f:      f.val,
		vmCall: vmCall,
	}
	ar.start(n)
	vm.push(ar.promiseCap.promise)
	vm.pc++
}

func (f *asyncFuncObject) vmCall(vm *vm, n int) {
	f.asyncVmCall(vm, n, f.baseJsFuncObject.vmCall)
}

type asyncRunner struct {
	gen        generator
	promiseCap *promiseCapability
	f          *Object
	vmCall     func(*vm, int)

	trackingObj interface{}
}

func (ar *asyncRunner) onFulfilled(call FunctionCall) Value {
	if tracker := ar.f.runtime.asyncContextTracker; tracker != nil {
		tracker.Resumed(ar.trackingObj)
		ar.trackingObj = nil
	}
	ar.gen.vm.curAsyncRunner = ar
	defer func() {
		ar.gen.vm.curAsyncRunner = nil
	}()
	arg := call.Argument(0)
	res, resType, ex := ar.gen.next(arg)
	ar.step(res, resType == resultNormal, ex)
	return _undefined
}

func (ar *asyncRunner) onRejected(call FunctionCall) Value {
	if tracker := ar.f.runtime.asyncContextTracker; tracker != nil {
		tracker.Resumed(ar.trackingObj)
		ar.trackingObj = nil
	}
	ar.gen.vm.curAsyncRunner = ar
	defer func() {
		ar.gen.vm.curAsyncRunner = nil
	}()
	reason := call.Argument(0)
	res, resType, ex := ar.gen.nextThrow(reason)
	ar.step(res, resType == resultNormal, ex)
	return _undefined
}

func (ar *asyncRunner) step(res Value, done bool, ex *Exception) {
	r := ar.f.runtime
	if done || ex != nil {
		if tracker := r.asyncContextTracker; tracker != nil {
			tracker.Completed()
		}
		if ex == nil {
			ar.promiseCap.resolve(res)
		} else {
			ar.promiseCap.reject(ex.val)
		}
		return
	}

	// await
	if tracker := r.asyncContextTracker; tracker != nil {
		ar.trackingObj = tracker.Suspended()
	}
	promise := r.promiseResolve(r.global.Promise, res)
	promise.self.(*Promise).addReactions(&promiseReaction{
		typ:         promiseReactionFulfill,
		handler:     &jobCallback{callback: ar.onFulfilled},
		asyncRunner: ar,
	}, &promiseReaction{
		typ:         promiseReactionReject,
		handler:     &jobCallback{callback: ar.onRejected},
		asyncRunner: ar,
	})
}

func (ar *asyncRunner) start(nArgs int) {
	r := ar.f.runtime
	ar.gen.vm = r.vm
	ar.promiseCap = r.newPromiseCapability(r.global.Promise)
	sp := r.vm.sp
	ar.gen.enter()
	ar.vmCall(r.vm, nArgs)
	res, resType, ex := ar.gen.step()
	ar.step(res, resType == resultNormal, ex)
	if ex != nil {
		r.vm.sp = sp - nArgs - 2
	}
	r.vm.popTryFrame()
	r.vm.popCtx()
}

type generator struct {
	ctx execCtx
	vm  *vm

	tryStackLen, iterStackLen, refStackLen uint32
}

func (g *generator) storeLengths() {
	g.tryStackLen, g.iterStackLen, g.refStackLen = uint32(len(g.vm.tryStack)), uint32(len(g.vm.iterStack)), uint32(len(g.vm.refStack))
}

func (g *generator) enter() {
	g.vm.pushCtx()
	g.vm.pushTryFrame(tryPanicMarker, -1)
	g.vm.prg, g.vm.sb, g.vm.pc = nil, -1, -2 // so that vm.run() halts after ret
	g.storeLengths()
}

func (g *generator) step() (res Value, resultType resultType, ex *Exception) {
	for {
		ex = g.vm.runTryInner()
		if ex != nil {
			return
		}
		if g.vm.halted() {
			break
		}
	}
	res = g.vm.pop()
	if res == resultAwaitMarker {
		resultType = resultAwait
		g.ctx = execCtx{}
		g.vm.pc = -g.vm.pc + 1
		res = g.vm.pop()
		g.vm.suspend(&g.ctx, g.tryStackLen, g.iterStackLen, g.refStackLen)
		g.vm.sp = g.vm.sb - 1
		g.vm.callStack = g.vm.callStack[:len(g.vm.callStack)-1] // remove the frame with pc == -2, as ret would do
	}
	return
}

func (g *generator) enterNext() {
	g.vm.pushCtx()
	g.vm.pushTryFrame(tryPanicMarker, -1)
	g.vm.callStack = append(g.vm.callStack, context{pc: -2}) // extra frame so that vm.run() halts after ret
	g.storeLengths()
	g.vm.resume(&g.ctx)
}

func (g *generator) next(v Value) (Value, resultType, *Exception) {
	g.enterNext()
	if v != nil {
		g.vm.push(v)
	}
	res, done, ex := g.step()
	g.vm.popTryFrame()
	g.vm.popCtx()
	return res, done, ex
}

func (g *generator) nextThrow(v Value) (Value, resultType, *Exception) {
	g.enterNext()
	ex := g.vm.handleThrow(v)
	if ex != nil {
		g.vm.popTryFrame()
		g.vm.popCtx()
		return nil, resultNormal, ex
	}

	res, resType, ex := g.step()
	g.vm.popTryFrame()
	g.vm.popCtx()
	return res, resType, ex
}
