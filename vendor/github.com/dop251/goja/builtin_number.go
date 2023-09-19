package goja

import (
	"math"
	"sync"

	"github.com/dop251/goja/ftoa"
)

func (r *Runtime) toNumber(v Value) Value {
	switch t := v.(type) {
	case valueFloat, valueInt:
		return v
	case *Object:
		switch t := t.self.(type) {
		case *primitiveValueObject:
			return r.toNumber(t.pValue)
		case *objectGoReflect:
			if t.class == classNumber && t.valueOf != nil {
				return t.valueOf()
			}
		}
		if t == r.global.NumberPrototype {
			return _positiveZero
		}
	}
	panic(r.NewTypeError("Value is not a number: %s", v))
}

func (r *Runtime) numberproto_valueOf(call FunctionCall) Value {
	return r.toNumber(call.This)
}

func (r *Runtime) numberproto_toString(call FunctionCall) Value {
	var numVal Value
	switch t := call.This.(type) {
	case valueFloat, valueInt:
		numVal = t
	case *Object:
		switch t := t.self.(type) {
		case *primitiveValueObject:
			numVal = r.toNumber(t.pValue)
		case *objectGoReflect:
			if t.class == classNumber {
				if t.toString != nil {
					return t.toString()
				}
				if t.valueOf != nil {
					numVal = t.valueOf()
				}
			}
		}
		if t == r.global.NumberPrototype {
			return asciiString("0")
		}
	}
	if numVal == nil {
		panic(r.NewTypeError("Value is not a number"))
	}
	var radix int
	if arg := call.Argument(0); arg != _undefined {
		radix = int(arg.ToInteger())
	} else {
		radix = 10
	}

	if radix < 2 || radix > 36 {
		panic(r.newError(r.getRangeError(), "toString() radix argument must be between 2 and 36"))
	}

	num := numVal.ToFloat()

	if math.IsNaN(num) {
		return stringNaN
	}

	if math.IsInf(num, 1) {
		return stringInfinity
	}

	if math.IsInf(num, -1) {
		return stringNegInfinity
	}

	if radix == 10 {
		return asciiString(fToStr(num, ftoa.ModeStandard, 0))
	}

	return asciiString(ftoa.FToBaseStr(num, radix))
}

func (r *Runtime) numberproto_toFixed(call FunctionCall) Value {
	num := r.toNumber(call.This).ToFloat()
	prec := call.Argument(0).ToInteger()

	if prec < 0 || prec > 100 {
		panic(r.newError(r.getRangeError(), "toFixed() precision must be between 0 and 100"))
	}
	if math.IsNaN(num) {
		return stringNaN
	}
	return asciiString(fToStr(num, ftoa.ModeFixed, int(prec)))
}

func (r *Runtime) numberproto_toExponential(call FunctionCall) Value {
	num := r.toNumber(call.This).ToFloat()
	precVal := call.Argument(0)
	var prec int64
	if precVal == _undefined {
		return asciiString(fToStr(num, ftoa.ModeStandardExponential, 0))
	} else {
		prec = precVal.ToInteger()
	}

	if math.IsNaN(num) {
		return stringNaN
	}
	if math.IsInf(num, 1) {
		return stringInfinity
	}
	if math.IsInf(num, -1) {
		return stringNegInfinity
	}

	if prec < 0 || prec > 100 {
		panic(r.newError(r.getRangeError(), "toExponential() precision must be between 0 and 100"))
	}

	return asciiString(fToStr(num, ftoa.ModeExponential, int(prec+1)))
}

func (r *Runtime) numberproto_toPrecision(call FunctionCall) Value {
	numVal := r.toNumber(call.This)
	precVal := call.Argument(0)
	if precVal == _undefined {
		return numVal.toString()
	}
	num := numVal.ToFloat()
	prec := precVal.ToInteger()

	if math.IsNaN(num) {
		return stringNaN
	}
	if math.IsInf(num, 1) {
		return stringInfinity
	}
	if math.IsInf(num, -1) {
		return stringNegInfinity
	}
	if prec < 1 || prec > 100 {
		panic(r.newError(r.getRangeError(), "toPrecision() precision must be between 1 and 100"))
	}

	return asciiString(fToStr(num, ftoa.ModePrecision, int(prec)))
}

func (r *Runtime) number_isFinite(call FunctionCall) Value {
	switch arg := call.Argument(0).(type) {
	case valueInt:
		return valueTrue
	case valueFloat:
		f := float64(arg)
		return r.toBoolean(!math.IsInf(f, 0) && !math.IsNaN(f))
	default:
		return valueFalse
	}
}

func (r *Runtime) number_isInteger(call FunctionCall) Value {
	switch arg := call.Argument(0).(type) {
	case valueInt:
		return valueTrue
	case valueFloat:
		f := float64(arg)
		return r.toBoolean(!math.IsNaN(f) && !math.IsInf(f, 0) && math.Floor(f) == f)
	default:
		return valueFalse
	}
}

func (r *Runtime) number_isNaN(call FunctionCall) Value {
	if f, ok := call.Argument(0).(valueFloat); ok && math.IsNaN(float64(f)) {
		return valueTrue
	}
	return valueFalse
}

func (r *Runtime) number_isSafeInteger(call FunctionCall) Value {
	arg := call.Argument(0)
	if i, ok := arg.(valueInt); ok && i >= -(maxInt-1) && i <= maxInt-1 {
		return valueTrue
	}
	if arg == _negativeZero {
		return valueTrue
	}
	return valueFalse
}

func createNumberProtoTemplate() *objectTemplate {
	t := newObjectTemplate()
	t.protoFactory = func(r *Runtime) *Object {
		return r.global.ObjectPrototype
	}

	t.putStr("constructor", func(r *Runtime) Value { return valueProp(r.getNumber(), true, false, true) })

	t.putStr("toExponential", func(r *Runtime) Value { return r.methodProp(r.numberproto_toExponential, "toExponential", 1) })
	t.putStr("toFixed", func(r *Runtime) Value { return r.methodProp(r.numberproto_toFixed, "toFixed", 1) })
	t.putStr("toLocaleString", func(r *Runtime) Value { return r.methodProp(r.numberproto_toString, "toLocaleString", 0) })
	t.putStr("toPrecision", func(r *Runtime) Value { return r.methodProp(r.numberproto_toPrecision, "toPrecision", 1) })
	t.putStr("toString", func(r *Runtime) Value { return r.methodProp(r.numberproto_toString, "toString", 1) })
	t.putStr("valueOf", func(r *Runtime) Value { return r.methodProp(r.numberproto_valueOf, "valueOf", 0) })

	return t
}

var numberProtoTemplate *objectTemplate
var numberProtoTemplateOnce sync.Once

func getNumberProtoTemplate() *objectTemplate {
	numberProtoTemplateOnce.Do(func() {
		numberProtoTemplate = createNumberProtoTemplate()
	})
	return numberProtoTemplate
}

func (r *Runtime) getNumberPrototype() *Object {
	ret := r.global.NumberPrototype
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.NumberPrototype = ret
		o := r.newTemplatedObject(getNumberProtoTemplate(), ret)
		o.class = classNumber
	}
	return ret
}

func (r *Runtime) getParseFloat() *Object {
	ret := r.global.parseFloat
	if ret == nil {
		ret = r.newNativeFunc(r.builtin_parseFloat, "parseFloat", 1)
		r.global.parseFloat = ret
	}
	return ret
}

func (r *Runtime) getParseInt() *Object {
	ret := r.global.parseInt
	if ret == nil {
		ret = r.newNativeFunc(r.builtin_parseInt, "parseInt", 2)
		r.global.parseInt = ret
	}
	return ret
}

func createNumberTemplate() *objectTemplate {
	t := newObjectTemplate()
	t.protoFactory = func(r *Runtime) *Object {
		return r.getFunctionPrototype()
	}
	t.putStr("length", func(r *Runtime) Value { return valueProp(intToValue(1), false, false, true) })
	t.putStr("name", func(r *Runtime) Value { return valueProp(asciiString("Number"), false, false, true) })

	t.putStr("prototype", func(r *Runtime) Value { return valueProp(r.getNumberPrototype(), false, false, false) })

	t.putStr("EPSILON", func(r *Runtime) Value { return valueProp(_epsilon, false, false, false) })
	t.putStr("isFinite", func(r *Runtime) Value { return r.methodProp(r.number_isFinite, "isFinite", 1) })
	t.putStr("isInteger", func(r *Runtime) Value { return r.methodProp(r.number_isInteger, "isInteger", 1) })
	t.putStr("isNaN", func(r *Runtime) Value { return r.methodProp(r.number_isNaN, "isNaN", 1) })
	t.putStr("isSafeInteger", func(r *Runtime) Value { return r.methodProp(r.number_isSafeInteger, "isSafeInteger", 1) })
	t.putStr("MAX_SAFE_INTEGER", func(r *Runtime) Value { return valueProp(valueInt(maxInt-1), false, false, false) })
	t.putStr("MIN_SAFE_INTEGER", func(r *Runtime) Value { return valueProp(valueInt(-(maxInt - 1)), false, false, false) })
	t.putStr("MIN_VALUE", func(r *Runtime) Value { return valueProp(valueFloat(math.SmallestNonzeroFloat64), false, false, false) })
	t.putStr("MAX_VALUE", func(r *Runtime) Value { return valueProp(valueFloat(math.MaxFloat64), false, false, false) })
	t.putStr("NaN", func(r *Runtime) Value { return valueProp(_NaN, false, false, false) })
	t.putStr("NEGATIVE_INFINITY", func(r *Runtime) Value { return valueProp(_negativeInf, false, false, false) })
	t.putStr("parseFloat", func(r *Runtime) Value { return valueProp(r.getParseFloat(), true, false, true) })
	t.putStr("parseInt", func(r *Runtime) Value { return valueProp(r.getParseInt(), true, false, true) })
	t.putStr("POSITIVE_INFINITY", func(r *Runtime) Value { return valueProp(_positiveInf, false, false, false) })

	return t
}

var numberTemplate *objectTemplate
var numberTemplateOnce sync.Once

func getNumberTemplate() *objectTemplate {
	numberTemplateOnce.Do(func() {
		numberTemplate = createNumberTemplate()
	})
	return numberTemplate
}

func (r *Runtime) getNumber() *Object {
	ret := r.global.Number
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Number = ret
		r.newTemplatedFuncObject(getNumberTemplate(), ret, r.builtin_Number,
			r.wrapNativeConstruct(r.builtin_newNumber, ret, r.getNumberPrototype()))
	}
	return ret
}
