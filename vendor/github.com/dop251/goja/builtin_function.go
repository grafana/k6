package goja

import (
	"fmt"
	"math"
)

func (r *Runtime) builtin_Function(args []Value, proto *Object) *Object {
	var sb valueStringBuilder
	sb.WriteString(asciiString("(function anonymous("))
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

func nativeFuncString(f *nativeFuncObject) Value {
	return newStringValue(fmt.Sprintf("function %s() { [native code] }", nilSafe(f.getStr("name", nil)).toString()))
}

func (r *Runtime) functionproto_toString(call FunctionCall) Value {
	obj := r.toObject(call.This)
repeat:
	switch f := obj.self.(type) {
	case *funcObject:
		return newStringValue(f.src)
	case *classFuncObject:
		return newStringValue(f.src)
	case *methodFuncObject:
		return newStringValue(f.src)
	case *arrowFuncObject:
		return newStringValue(f.src)
	case *nativeFuncObject:
		return nativeFuncString(f)
	case *boundFuncObject:
		return nativeFuncString(&f.nativeFuncObject)
	case *wrappedFuncObject:
		return nativeFuncString(&f.nativeFuncObject)
	case *lazyObject:
		obj.self = f.create(obj)
		goto repeat
	case *proxyObject:
	repeat2:
		switch c := f.target.self.(type) {
		case *classFuncObject, *methodFuncObject, *funcObject, *arrowFuncObject, *nativeFuncObject, *boundFuncObject:
			return asciiString("function () { [native code] }")
		case *lazyObject:
			f.target.self = c.create(obj)
			goto repeat2
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
	if s, ok := name.(valueString); ok {
		nameStr = nameStr.concat(s)
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

func (r *Runtime) initFunction() {
	o := r.global.FunctionPrototype.self.(*nativeFuncObject)
	o.prototype = r.global.ObjectPrototype
	o._putProp("name", stringEmpty, false, false, true)
	o._putProp("apply", r.newNativeFunc(r.functionproto_apply, nil, "apply", nil, 2), true, false, true)
	o._putProp("bind", r.newNativeFunc(r.functionproto_bind, nil, "bind", nil, 1), true, false, true)
	o._putProp("call", r.newNativeFunc(r.functionproto_call, nil, "call", nil, 1), true, false, true)
	o._putProp("toString", r.newNativeFunc(r.functionproto_toString, nil, "toString", nil, 0), true, false, true)
	o._putSym(SymHasInstance, valueProp(r.newNativeFunc(r.functionproto_hasInstance, nil, "[Symbol.hasInstance]", nil, 1), false, false, false))

	r.global.Function = r.newNativeFuncConstruct(r.builtin_Function, "Function", r.global.FunctionPrototype, 1)
	r.addToGlobal("Function", r.global.Function)
}
