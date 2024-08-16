package sobek

import "github.com/grafana/sobek/unistring"

const propNameStack = "stack"

type errorObject struct {
	baseObject
	stack          []StackFrame
	stackPropAdded bool
}

func (e *errorObject) formatStack() String {
	var b StringBuilder
	val := writeErrorString(&b, e.val)
	if val != nil {
		b.WriteString(val)
	}
	b.WriteRune('\n')

	for _, frame := range e.stack {
		b.writeASCII("\tat ")
		frame.WriteToValueBuilder(&b)
		b.WriteRune('\n')
	}
	return b.String()
}

func (e *errorObject) addStackProp() Value {
	if !e.stackPropAdded {
		res := e._putProp(propNameStack, e.formatStack(), true, false, true)
		if len(e.propNames) > 1 {
			// reorder property names to ensure 'stack' is the first one
			copy(e.propNames[1:], e.propNames)
			e.propNames[0] = propNameStack
		}
		e.stackPropAdded = true
		return res
	}
	return nil
}

func (e *errorObject) getStr(p unistring.String, receiver Value) Value {
	return e.getStrWithOwnProp(e.getOwnPropStr(p), p, receiver)
}

func (e *errorObject) getOwnPropStr(name unistring.String) Value {
	res := e.baseObject.getOwnPropStr(name)
	if res == nil && name == propNameStack {
		return e.addStackProp()
	}

	return res
}

func (e *errorObject) setOwnStr(name unistring.String, val Value, throw bool) bool {
	if name == propNameStack {
		e.addStackProp()
	}
	return e.baseObject.setOwnStr(name, val, throw)
}

func (e *errorObject) setForeignStr(name unistring.String, val, receiver Value, throw bool) (bool, bool) {
	return e._setForeignStr(name, e.getOwnPropStr(name), val, receiver, throw)
}

func (e *errorObject) deleteStr(name unistring.String, throw bool) bool {
	if name == propNameStack {
		e.addStackProp()
	}
	return e.baseObject.deleteStr(name, throw)
}

func (e *errorObject) defineOwnPropertyStr(name unistring.String, desc PropertyDescriptor, throw bool) bool {
	if name == propNameStack {
		e.addStackProp()
	}
	return e.baseObject.defineOwnPropertyStr(name, desc, throw)
}

func (e *errorObject) hasOwnPropertyStr(name unistring.String) bool {
	if e.baseObject.hasOwnPropertyStr(name) {
		return true
	}

	return name == propNameStack && !e.stackPropAdded
}

func (e *errorObject) stringKeys(all bool, accum []Value) []Value {
	if all && !e.stackPropAdded {
		accum = append(accum, asciiString(propNameStack))
	}
	return e.baseObject.stringKeys(all, accum)
}

func (e *errorObject) iterateStringKeys() iterNextFunc {
	e.addStackProp()
	return e.baseObject.iterateStringKeys()
}

func (e *errorObject) init() {
	e.baseObject.init()
	vm := e.val.runtime.vm
	e.stack = vm.captureStack(make([]StackFrame, 0, len(vm.callStack)+1), 0)
}

func (r *Runtime) newErrorObject(proto *Object, class string) *errorObject {
	obj := &Object{runtime: r}
	o := &errorObject{
		baseObject: baseObject{
			class:      class,
			val:        obj,
			extensible: true,
			prototype:  proto,
		},
	}
	obj.self = o
	o.init()
	return o
}

func (r *Runtime) builtin_Error(args []Value, proto *Object) *Object {
	obj := r.newErrorObject(proto, classError)
	if len(args) > 0 && args[0] != _undefined {
		obj._putProp("message", args[0].ToString(), true, false, true)
	}
	if len(args) > 1 && args[1] != _undefined {
		if options, ok := args[1].(*Object); ok {
			if options.hasProperty(asciiString("cause")) {
				obj.defineOwnPropertyStr("cause", PropertyDescriptor{
					Writable:     FLAG_TRUE,
					Enumerable:   FLAG_FALSE,
					Configurable: FLAG_TRUE,
					Value:        options.Get("cause"),
				}, true)
			}
		}
	}
	return obj.val
}

func (r *Runtime) builtin_AggregateError(args []Value, proto *Object) *Object {
	obj := r.newErrorObject(proto, classError)
	if len(args) > 1 && args[1] != nil && args[1] != _undefined {
		obj._putProp("message", args[1].toString(), true, false, true)
	}
	var errors []Value
	if len(args) > 0 {
		errors = r.iterableToList(args[0], nil)
	}
	obj._putProp("errors", r.newArrayValues(errors), true, false, true)

	if len(args) > 2 && args[2] != _undefined {
		if options, ok := args[2].(*Object); ok {
			if options.hasProperty(asciiString("cause")) {
				obj.defineOwnPropertyStr("cause", PropertyDescriptor{
					Writable:     FLAG_TRUE,
					Enumerable:   FLAG_FALSE,
					Configurable: FLAG_TRUE,
					Value:        options.Get("cause"),
				}, true)
			}
		}
	}

	return obj.val
}

func writeErrorString(sb *StringBuilder, obj *Object) String {
	var nameStr, msgStr String
	name := obj.self.getStr("name", nil)
	if name == nil || name == _undefined {
		nameStr = asciiString("Error")
	} else {
		nameStr = name.toString()
	}
	msg := obj.self.getStr("message", nil)
	if msg == nil || msg == _undefined {
		msgStr = stringEmpty
	} else {
		msgStr = msg.toString()
	}
	if nameStr.Length() == 0 {
		return msgStr
	}
	if msgStr.Length() == 0 {
		return nameStr
	}
	sb.WriteString(nameStr)
	sb.WriteString(asciiString(": "))
	sb.WriteString(msgStr)
	return nil
}

func (r *Runtime) error_toString(call FunctionCall) Value {
	var sb StringBuilder
	val := writeErrorString(&sb, r.toObject(call.This))
	if val != nil {
		return val
	}
	return sb.String()
}

func (r *Runtime) createErrorPrototype(name String, ctor *Object) *Object {
	o := r.newBaseObject(r.getErrorPrototype(), classObject)
	o._putProp("message", stringEmpty, true, false, true)
	o._putProp("name", name, true, false, true)
	o._putProp("constructor", ctor, true, false, true)
	return o.val
}

func (r *Runtime) getErrorPrototype() *Object {
	ret := r.global.ErrorPrototype
	if ret == nil {
		ret = r.NewObject()
		r.global.ErrorPrototype = ret
		o := ret.self
		o._putProp("message", stringEmpty, true, false, true)
		o._putProp("name", stringError, true, false, true)
		o._putProp("toString", r.newNativeFunc(r.error_toString, "toString", 0), true, false, true)
		o._putProp("constructor", r.getError(), true, false, true)
	}
	return ret
}

func (r *Runtime) getError() *Object {
	ret := r.global.Error
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Error = ret
		r.newNativeFuncConstruct(ret, r.builtin_Error, "Error", r.getErrorPrototype(), 1)
	}
	return ret
}

func (r *Runtime) getAggregateError() *Object {
	ret := r.global.AggregateError
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.AggregateError = ret
		r.newNativeFuncConstructProto(ret, r.builtin_AggregateError, "AggregateError", r.createErrorPrototype(stringAggregateError, ret), r.getError(), 2)
	}
	return ret
}

func (r *Runtime) getTypeError() *Object {
	ret := r.global.TypeError
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.TypeError = ret
		r.newNativeFuncConstructProto(ret, r.builtin_Error, "TypeError", r.createErrorPrototype(stringTypeError, ret), r.getError(), 1)
	}
	return ret
}

func (r *Runtime) getReferenceError() *Object {
	ret := r.global.ReferenceError
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.ReferenceError = ret
		r.newNativeFuncConstructProto(ret, r.builtin_Error, "ReferenceError", r.createErrorPrototype(stringReferenceError, ret), r.getError(), 1)
	}
	return ret
}

func (r *Runtime) getSyntaxError() *Object {
	ret := r.global.SyntaxError
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.SyntaxError = ret
		r.newNativeFuncConstructProto(ret, r.builtin_Error, "SyntaxError", r.createErrorPrototype(stringSyntaxError, ret), r.getError(), 1)
	}
	return ret
}

func (r *Runtime) getRangeError() *Object {
	ret := r.global.RangeError
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.RangeError = ret
		r.newNativeFuncConstructProto(ret, r.builtin_Error, "RangeError", r.createErrorPrototype(stringRangeError, ret), r.getError(), 1)
	}
	return ret
}

func (r *Runtime) getEvalError() *Object {
	ret := r.global.EvalError
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.EvalError = ret
		r.newNativeFuncConstructProto(ret, r.builtin_Error, "EvalError", r.createErrorPrototype(stringEvalError, ret), r.getError(), 1)
	}
	return ret
}

func (r *Runtime) getURIError() *Object {
	ret := r.global.URIError
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.URIError = ret
		r.newNativeFuncConstructProto(ret, r.builtin_Error, "URIError", r.createErrorPrototype(stringURIError, ret), r.getError(), 1)
	}
	return ret
}

func (r *Runtime) getGoError() *Object {
	ret := r.global.GoError
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.GoError = ret
		r.newNativeFuncConstructProto(ret, r.builtin_Error, "GoError", r.createErrorPrototype(stringGoError, ret), r.getError(), 1)
	}
	return ret
}
