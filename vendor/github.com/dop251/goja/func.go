package goja

import (
	"reflect"

	"github.com/dop251/goja/unistring"
)

type baseFuncObject struct {
	baseObject

	lenProp valueProperty
}

type baseJsFuncObject struct {
	baseFuncObject

	stash  *stash
	prg    *Program
	src    string
	strict bool
}

type funcObject struct {
	baseJsFuncObject
}

type methodFuncObject struct {
	baseJsFuncObject
}

type arrowFuncObject struct {
	baseJsFuncObject
	this      Value
	newTarget Value
}

type nativeFuncObject struct {
	baseFuncObject

	f         func(FunctionCall) Value
	construct func(args []Value, newTarget *Object) *Object
}

type boundFuncObject struct {
	nativeFuncObject
	wrapped *Object
}

func (f *nativeFuncObject) export(*objectExportCtx) interface{} {
	return f.f
}

func (f *nativeFuncObject) exportType() reflect.Type {
	return reflect.TypeOf(f.f)
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

func (f *funcObject) construct(args []Value, newTarget *Object) *Object {
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

func (f *baseJsFuncObject) Call(call FunctionCall) Value {
	return f.call(call, nil)
}

func (f *arrowFuncObject) Call(call FunctionCall) Value {
	return f._call(call, f.newTarget, f.this)
}

func (f *baseJsFuncObject) _call(call FunctionCall, newTarget, this Value) Value {
	vm := f.val.runtime.vm

	vm.stack.expand(vm.sp + len(call.Arguments) + 1)
	vm.stack[vm.sp] = f.val
	vm.sp++
	vm.stack[vm.sp] = this
	vm.sp++
	for _, arg := range call.Arguments {
		if arg != nil {
			vm.stack[vm.sp] = arg
		} else {
			vm.stack[vm.sp] = _undefined
		}
		vm.sp++
	}

	pc := vm.pc
	if pc != -1 {
		vm.pc++ // fake "return address" so that captureStack() records the correct call location
		vm.pushCtx()
		vm.callStack = append(vm.callStack, context{pc: -1}) // extra frame so that run() halts after ret
	} else {
		vm.pushCtx()
	}
	vm.args = len(call.Arguments)
	vm.prg = f.prg
	vm.stash = f.stash
	vm.newTarget = newTarget
	vm.pc = 0
	vm.run()
	if pc != -1 {
		vm.popCtx()
	}
	vm.pc = pc
	vm.halt = false
	return vm.pop()
}

func (f *baseJsFuncObject) call(call FunctionCall, newTarget Value) Value {
	return f._call(call, newTarget, nilSafe(call.This))
}

func (f *funcObject) export(*objectExportCtx) interface{} {
	return f.Call
}

func (f *funcObject) exportType() reflect.Type {
	return reflect.TypeOf(f.Call)
}

func (f *baseJsFuncObject) assertCallable() (func(FunctionCall) Value, bool) {
	return f.Call, true
}

func (f *funcObject) assertConstructor() func(args []Value, newTarget *Object) *Object {
	return f.construct
}

func (f *arrowFuncObject) exportType() reflect.Type {
	return reflect.TypeOf(f.Call)
}

func (f *arrowFuncObject) assertCallable() (func(FunctionCall) Value, bool) {
	return f.Call, true
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
	proto := f.getStr("prototype", nil)
	var protoObj *Object
	if p, ok := proto.(*Object); ok {
		protoObj = p
	} else {
		protoObj = f.val.runtime.global.ObjectPrototype
	}
	obj := f.val.runtime.newBaseObject(protoObj, classObject).val
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

func (f *nativeFuncObject) assertConstructor() func(args []Value, newTarget *Object) *Object {
	return f.construct
}

/*func (f *boundFuncObject) getStr(p unistring.String, receiver Value) Value {
	return f.getStrWithOwnProp(f.getOwnPropStr(p), p, receiver)
}

func (f *boundFuncObject) getOwnPropStr(name unistring.String) Value {
	if name == "caller" || name == "arguments" {
		return f.val.runtime.global.throwerProperty
	}

	return f.nativeFuncObject.getOwnPropStr(name)
}

func (f *boundFuncObject) deleteStr(name unistring.String, throw bool) bool {
	if name == "caller" || name == "arguments" {
		return true
	}
	return f.nativeFuncObject.deleteStr(name, throw)
}

func (f *boundFuncObject) setOwnStr(name unistring.String, val Value, throw bool) bool {
	if name == "caller" || name == "arguments" {
		panic(f.val.runtime.NewTypeError("'caller' and 'arguments' are restricted function properties and cannot be accessed in this context."))
	}
	return f.nativeFuncObject.setOwnStr(name, val, throw)
}

func (f *boundFuncObject) setForeignStr(name unistring.String, val, receiver Value, throw bool) (bool, bool) {
	return f._setForeignStr(name, f.getOwnPropStr(name), val, receiver, throw)
}
*/

func (f *boundFuncObject) hasInstance(v Value) bool {
	return instanceOfOperator(v, f.wrapped)
}
