package goja

import (
	"math"
	"sync"
)

func (r *Runtime) functionCtor(args []Value, proto *Object, async, generator bool) *Object {
	var sb StringBuilder
	if async {
		if generator {
			sb.WriteString(asciiString("(async function* anonymous("))
		} else {
			sb.WriteString(asciiString("(async function anonymous("))
		}
	} else {
		if generator {
			sb.WriteString(asciiString("(function* anonymous("))
		} else {
			sb.WriteString(asciiString("(function anonymous("))
		}
	}
	if len(args) > 1 {
		ar := args[:len(args)-1]
		for i, arg := range ar {
			sb.WriteString(arg.toString())
			if i < len(ar)-1 {
				sb.WriteRune(',')
			}
		}
	}
	sb.WriteString(asciiString("\n) {\n"))
	if len(args) > 0 {
		sb.WriteString(args[len(args)-1].toString())
	}
	sb.WriteString(asciiString("\n})"))

	ret := r.toObject(r.eval(sb.String(), false, false))
	ret.self.setProto(proto, true)
	return ret
}

func (r *Runtime) builtin_Function(args []Value, proto *Object) *Object {
	return r.functionCtor(args, proto, false, false)
}

func (r *Runtime) builtin_asyncFunction(args []Value, proto *Object) *Object {
	return r.functionCtor(args, proto, true, false)
}

func (r *Runtime) builtin_generatorFunction(args []Value, proto *Object) *Object {
	return r.functionCtor(args, proto, false, true)
}

func (r *Runtime) functionproto_toString(call FunctionCall) Value {
	obj := r.toObject(call.This)
	switch f := obj.self.(type) {
	case funcObjectImpl:
		return f.source()
	case *proxyObject:
		if _, ok := f.target.self.(funcObjectImpl); ok {
			return asciiString("function () { [native code] }")
		}
	}
	panic(r.NewTypeError("Function.prototype.toString requires that 'this' be a Function"))
}

func (r *Runtime) functionproto_hasInstance(call FunctionCall) Value {
	if o, ok := call.This.(*Object); ok {
		if _, ok = o.self.assertCallable(); ok {
			return r.toBoolean(o.self.hasInstance(call.Argument(0)))
		}
	}

	return valueFalse
}

func (r *Runtime) createListFromArrayLike(a Value) []Value {
	o := r.toObject(a)
	if arr := r.checkStdArrayObj(o); arr != nil {
		return arr.values
	}
	l := toLength(o.self.getStr("length", nil))
	res := make([]Value, 0, l)
	for k := int64(0); k < l; k++ {
		res = append(res, nilSafe(o.self.getIdx(valueInt(k), nil)))
	}
	return res
}

func (r *Runtime) functionproto_apply(call FunctionCall) Value {
	var args []Value
	if len(call.Arguments) >= 2 {
		args = r.createListFromArrayLike(call.Arguments[1])
	}

	f := r.toCallable(call.This)
	return f(FunctionCall{
		This:      call.Argument(0),
		Arguments: args,
	})
}

func (r *Runtime) functionproto_call(call FunctionCall) Value {
	var args []Value
	if len(call.Arguments) > 0 {
		args = call.Arguments[1:]
	}

	f := r.toCallable(call.This)
	return f(FunctionCall{
		This:      call.Argument(0),
		Arguments: args,
	})
}

func (r *Runtime) boundCallable(target func(FunctionCall) Value, boundArgs []Value) func(FunctionCall) Value {
	var this Value
	var args []Value
	if len(boundArgs) > 0 {
		this = boundArgs[0]
		args = make([]Value, len(boundArgs)-1)
		copy(args, boundArgs[1:])
	} else {
		this = _undefined
	}
	return func(call FunctionCall) Value {
		a := append(args, call.Arguments...)
		return target(FunctionCall{
			This:      this,
			Arguments: a,
		})
	}
}

func (r *Runtime) boundConstruct(f *Object, target func([]Value, *Object) *Object, boundArgs []Value) func([]Value, *Object) *Object {
	if target == nil {
		return nil
	}
	var args []Value
	if len(boundArgs) > 1 {
		args = make([]Value, len(boundArgs)-1)
		copy(args, boundArgs[1:])
	}
	return func(fargs []Value, newTarget *Object) *Object {
		a := append(args, fargs...)
		if newTarget == f {
			newTarget = nil
		}
		return target(a, newTarget)
	}
}

func (r *Runtime) functionproto_bind(call FunctionCall) Value {
	obj := r.toObject(call.This)

	fcall := r.toCallable(call.This)
	construct := obj.self.assertConstructor()

	var l = _positiveZero
	if obj.self.hasOwnPropertyStr("length") {
		var li int64
		switch lenProp := nilSafe(obj.self.getStr("length", nil)).(type) {
		case valueInt:
			li = lenProp.ToInteger()
		case valueFloat:
			switch lenProp {
			case _positiveInf:
				l = lenProp
				goto lenNotInt
			case _negativeInf:
				goto lenNotInt
			case _negativeZero:
				// no-op, li == 0
			default:
				if !math.IsNaN(float64(lenProp)) {
					li = int64(math.Abs(float64(lenProp)))
				} // else li = 0
			}
		}
		if len(call.Arguments) > 1 {
			li -= int64(len(call.Arguments)) - 1
		}
		if li < 0 {
			li = 0
		}
		l = intToValue(li)
	}
lenNotInt:
	name := obj.self.getStr("name", nil)
	nameStr := stringBound_
	if s, ok := name.(String); ok {
		nameStr = nameStr.Concat(s)
	}

	v := &Object{runtime: r}
	ff := r.newNativeFuncAndConstruct(v, r.boundCallable(fcall, call.Arguments), r.boundConstruct(v, construct, call.Arguments), nil, nameStr.string(), l)
	bf := &boundFuncObject{
		nativeFuncObject: *ff,
		wrapped:          obj,
	}
	bf.prototype = obj.self.proto()
	v.self = bf

	return v
}

func (r *Runtime) getThrower() *Object {
	ret := r.global.thrower
	if ret == nil {
		ret = r.newNativeFunc(r.builtin_thrower, "", 0)
		r.global.thrower = ret
		r.object_freeze(FunctionCall{Arguments: []Value{ret}})
	}
	return ret
}

func (r *Runtime) newThrowerProperty(configurable bool) Value {
	thrower := r.getThrower()
	return &valueProperty{
		getterFunc:   thrower,
		setterFunc:   thrower,
		accessor:     true,
		configurable: configurable,
	}
}

func createFunctionProtoTemplate() *objectTemplate {
	t := newObjectTemplate()
	t.protoFactory = func(r *Runtime) *Object {
		return r.global.ObjectPrototype
	}

	t.putStr("constructor", func(r *Runtime) Value { return valueProp(r.getFunction(), true, false, true) })

	t.putStr("length", func(r *Runtime) Value { return valueProp(_positiveZero, false, false, true) })
	t.putStr("name", func(r *Runtime) Value { return valueProp(stringEmpty, false, false, true) })

	t.putStr("apply", func(r *Runtime) Value { return r.methodProp(r.functionproto_apply, "apply", 2) })
	t.putStr("bind", func(r *Runtime) Value { return r.methodProp(r.functionproto_bind, "bind", 1) })
	t.putStr("call", func(r *Runtime) Value { return r.methodProp(r.functionproto_call, "call", 1) })
	t.putStr("toString", func(r *Runtime) Value { return r.methodProp(r.functionproto_toString, "toString", 0) })

	t.putStr("caller", func(r *Runtime) Value { return r.newThrowerProperty(true) })
	t.putStr("arguments", func(r *Runtime) Value { return r.newThrowerProperty(true) })

	t.putSym(SymHasInstance, func(r *Runtime) Value {
		return valueProp(r.newNativeFunc(r.functionproto_hasInstance, "[Symbol.hasInstance]", 1), false, false, false)
	})

	return t
}

var functionProtoTemplate *objectTemplate
var functionProtoTemplateOnce sync.Once

func getFunctionProtoTemplate() *objectTemplate {
	functionProtoTemplateOnce.Do(func() {
		functionProtoTemplate = createFunctionProtoTemplate()
	})
	return functionProtoTemplate
}

func (r *Runtime) getFunctionPrototype() *Object {
	ret := r.global.FunctionPrototype
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.FunctionPrototype = ret
		r.newTemplatedFuncObject(getFunctionProtoTemplate(), ret, func(FunctionCall) Value {
			return _undefined
		}, nil)
	}
	return ret
}

func (r *Runtime) createFunction(v *Object) objectImpl {
	return r.newNativeFuncConstructObj(v, r.builtin_Function, "Function", r.getFunctionPrototype(), 1)
}

func (r *Runtime) createAsyncFunctionProto(val *Object) objectImpl {
	o := &baseObject{
		class:      classObject,
		val:        val,
		extensible: true,
		prototype:  r.getFunctionPrototype(),
	}
	o.init()

	o._putProp("constructor", r.getAsyncFunction(), true, false, true)

	o._putSym(SymToStringTag, valueProp(asciiString(classAsyncFunction), false, false, true))

	return o
}

func (r *Runtime) getAsyncFunctionPrototype() *Object {
	var o *Object
	if o = r.global.AsyncFunctionPrototype; o == nil {
		o = &Object{runtime: r}
		r.global.AsyncFunctionPrototype = o
		o.self = r.createAsyncFunctionProto(o)
	}
	return o
}

func (r *Runtime) createAsyncFunction(val *Object) objectImpl {
	o := r.newNativeFuncConstructObj(val, r.builtin_asyncFunction, "AsyncFunction", r.getAsyncFunctionPrototype(), 1)

	return o
}

func (r *Runtime) getAsyncFunction() *Object {
	var o *Object
	if o = r.global.AsyncFunction; o == nil {
		o = &Object{runtime: r}
		r.global.AsyncFunction = o
		o.self = r.createAsyncFunction(o)
	}
	return o
}

func (r *Runtime) builtin_genproto_next(call FunctionCall) Value {
	if o, ok := call.This.(*Object); ok {
		if gen, ok := o.self.(*generatorObject); ok {
			return gen.next(call.Argument(0))
		}
	}
	panic(r.NewTypeError("Method [Generator].prototype.next called on incompatible receiver"))
}

func (r *Runtime) builtin_genproto_return(call FunctionCall) Value {
	if o, ok := call.This.(*Object); ok {
		if gen, ok := o.self.(*generatorObject); ok {
			return gen._return(call.Argument(0))
		}
	}
	panic(r.NewTypeError("Method [Generator].prototype.return called on incompatible receiver"))
}

func (r *Runtime) builtin_genproto_throw(call FunctionCall) Value {
	if o, ok := call.This.(*Object); ok {
		if gen, ok := o.self.(*generatorObject); ok {
			return gen.throw(call.Argument(0))
		}
	}
	panic(r.NewTypeError("Method [Generator].prototype.throw called on incompatible receiver"))
}

func (r *Runtime) createGeneratorFunctionProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.getFunctionPrototype(), classObject)

	o._putProp("constructor", r.getGeneratorFunction(), false, false, true)
	o._putProp("prototype", r.getGeneratorPrototype(), false, false, true)
	o._putSym(SymToStringTag, valueProp(asciiString(classGeneratorFunction), false, false, true))

	return o
}

func (r *Runtime) getGeneratorFunctionPrototype() *Object {
	var o *Object
	if o = r.global.GeneratorFunctionPrototype; o == nil {
		o = &Object{runtime: r}
		r.global.GeneratorFunctionPrototype = o
		o.self = r.createGeneratorFunctionProto(o)
	}
	return o
}

func (r *Runtime) createGeneratorFunction(val *Object) objectImpl {
	o := r.newNativeFuncConstructObj(val, r.builtin_generatorFunction, "GeneratorFunction", r.getGeneratorFunctionPrototype(), 1)
	return o
}

func (r *Runtime) getGeneratorFunction() *Object {
	var o *Object
	if o = r.global.GeneratorFunction; o == nil {
		o = &Object{runtime: r}
		r.global.GeneratorFunction = o
		o.self = r.createGeneratorFunction(o)
	}
	return o
}

func (r *Runtime) createGeneratorProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.getIteratorPrototype(), classObject)

	o._putProp("constructor", r.getGeneratorFunctionPrototype(), false, false, true)
	o._putProp("next", r.newNativeFunc(r.builtin_genproto_next, "next", 1), true, false, true)
	o._putProp("return", r.newNativeFunc(r.builtin_genproto_return, "return", 1), true, false, true)
	o._putProp("throw", r.newNativeFunc(r.builtin_genproto_throw, "throw", 1), true, false, true)

	o._putSym(SymToStringTag, valueProp(asciiString(classGenerator), false, false, true))

	return o
}

func (r *Runtime) getGeneratorPrototype() *Object {
	var o *Object
	if o = r.global.GeneratorPrototype; o == nil {
		o = &Object{runtime: r}
		r.global.GeneratorPrototype = o
		o.self = r.createGeneratorProto(o)
	}
	return o
}

func (r *Runtime) getFunction() *Object {
	ret := r.global.Function
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Function = ret
		ret.self = r.createFunction(ret)
	}

	return ret
}
