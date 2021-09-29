package goja

import (
	"math"

	"github.com/dop251/goja/ftoa"
)

func (r *Runtime) numberproto_valueOf(call FunctionCall) Value {
	this := call.This
	if !isNumber(this) {
		r.typeErrorResult(true, "Value is not a number")
	}
	switch t := this.(type) {
	case valueInt, valueFloat:
		return this
	case *Object:
		if v, ok := t.self.(*primitiveValueObject); ok {
			return v.pValue
		}
	}

	panic(r.NewTypeError("Number.prototype.valueOf is not generic"))
}

func isNumber(v Value) bool {
	switch t := v.(type) {
	case valueFloat, valueInt:
		return true
	case *Object:
		switch t := t.self.(type) {
		case *primitiveValueObject:
			return isNumber(t.pValue)
		}
	}
	return false
}

func (r *Runtime) numberproto_toString(call FunctionCall) Value {
	if !isNumber(call.This) {
		r.typeErrorResult(true, "Value is not a number")
	}
	var radix int
	if arg := call.Argument(0); arg != _undefined {
		radix = int(arg.ToInteger())
	} else {
		radix = 10
	}

	if radix < 2 || radix > 36 {
		panic(r.newError(r.global.RangeError, "toString() radix argument must be between 2 and 36"))
	}

	num := call.This.ToFloat()

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
		panic(r.newError(r.global.RangeError, "toFixed() precision must be between 0 and 100"))
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
		panic(r.newError(r.global.RangeError, "toExponential() precision must be between 0 and 100"))
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
		panic(r.newError(r.global.RangeError, "toPrecision() precision must be between 1 and 100"))
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

func (r *Runtime) initNumber() {
	r.global.NumberPrototype = r.newPrimitiveObject(valueInt(0), r.global.ObjectPrototype, classNumber)
	o := r.global.NumberPrototype.self
	o._putProp("toExponential", r.newNativeFunc(r.numberproto_toExponential, nil, "toExponential", nil, 1), true, false, true)
	o._putProp("toFixed", r.newNativeFunc(r.numberproto_toFixed, nil, "toFixed", nil, 1), true, false, true)
	o._putProp("toLocaleString", r.newNativeFunc(r.numberproto_toString, nil, "toLocaleString", nil, 0), true, false, true)
	o._putProp("toPrecision", r.newNativeFunc(r.numberproto_toPrecision, nil, "toPrecision", nil, 1), true, false, true)
	o._putProp("toString", r.newNativeFunc(r.numberproto_toString, nil, "toString", nil, 1), true, false, true)
	o._putProp("valueOf", r.newNativeFunc(r.numberproto_valueOf, nil, "valueOf", nil, 0), true, false, true)

	r.global.Number = r.newNativeFunc(r.builtin_Number, r.builtin_newNumber, "Number", r.global.NumberPrototype, 1)
	o = r.global.Number.self
	o._putProp("EPSILON", _epsilon, false, false, false)
	o._putProp("isFinite", r.newNativeFunc(r.number_isFinite, nil, "isFinite", nil, 1), true, false, true)
	o._putProp("isInteger", r.newNativeFunc(r.number_isInteger, nil, "isInteger", nil, 1), true, false, true)
	o._putProp("isNaN", r.newNativeFunc(r.number_isNaN, nil, "isNaN", nil, 1), true, false, true)
	o._putProp("isSafeInteger", r.newNativeFunc(r.number_isSafeInteger, nil, "isSafeInteger", nil, 1), true, false, true)
	o._putProp("MAX_SAFE_INTEGER", valueInt(maxInt-1), false, false, false)
	o._putProp("MIN_SAFE_INTEGER", valueInt(-(maxInt - 1)), false, false, false)
	o._putProp("MIN_VALUE", valueFloat(math.SmallestNonzeroFloat64), false, false, false)
	o._putProp("MAX_VALUE", valueFloat(math.MaxFloat64), false, false, false)
	o._putProp("NaN", _NaN, false, false, false)
	o._putProp("NEGATIVE_INFINITY", _negativeInf, false, false, false)
	o._putProp("parseFloat", r.Get("parseFloat"), true, false, true)
	o._putProp("parseInt", r.Get("parseInt"), true, false, true)
	o._putProp("POSITIVE_INFINITY", _positiveInf, false, false, false)
	r.addToGlobal("Number", r.global.Number)

}
