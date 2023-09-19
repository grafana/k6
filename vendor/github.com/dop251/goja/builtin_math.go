package goja

import (
	"math"
	"math/bits"
	"sync"
)

func (r *Runtime) math_abs(call FunctionCall) Value {
	return floatToValue(math.Abs(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_acos(call FunctionCall) Value {
	return floatToValue(math.Acos(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_acosh(call FunctionCall) Value {
	return floatToValue(math.Acosh(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_asin(call FunctionCall) Value {
	return floatToValue(math.Asin(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_asinh(call FunctionCall) Value {
	return floatToValue(math.Asinh(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_atan(call FunctionCall) Value {
	return floatToValue(math.Atan(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_atanh(call FunctionCall) Value {
	return floatToValue(math.Atanh(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_atan2(call FunctionCall) Value {
	y := call.Argument(0).ToFloat()
	x := call.Argument(1).ToFloat()

	return floatToValue(math.Atan2(y, x))
}

func (r *Runtime) math_cbrt(call FunctionCall) Value {
	return floatToValue(math.Cbrt(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_ceil(call FunctionCall) Value {
	return floatToValue(math.Ceil(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_clz32(call FunctionCall) Value {
	return intToValue(int64(bits.LeadingZeros32(toUint32(call.Argument(0)))))
}

func (r *Runtime) math_cos(call FunctionCall) Value {
	return floatToValue(math.Cos(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_cosh(call FunctionCall) Value {
	return floatToValue(math.Cosh(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_exp(call FunctionCall) Value {
	return floatToValue(math.Exp(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_expm1(call FunctionCall) Value {
	return floatToValue(math.Expm1(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_floor(call FunctionCall) Value {
	return floatToValue(math.Floor(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_fround(call FunctionCall) Value {
	return floatToValue(float64(float32(call.Argument(0).ToFloat())))
}

func (r *Runtime) math_hypot(call FunctionCall) Value {
	var max float64
	var hasNaN bool
	absValues := make([]float64, 0, len(call.Arguments))
	for _, v := range call.Arguments {
		arg := nilSafe(v).ToFloat()
		if math.IsNaN(arg) {
			hasNaN = true
		} else {
			abs := math.Abs(arg)
			if abs > max {
				max = abs
			}
			absValues = append(absValues, abs)
		}
	}
	if math.IsInf(max, 1) {
		return _positiveInf
	}
	if hasNaN {
		return _NaN
	}
	if max == 0 {
		return _positiveZero
	}

	// Kahan summation to avoid rounding errors.
	// Normalize the numbers to the largest one to avoid overflow.
	var sum, compensation float64
	for _, n := range absValues {
		n /= max
		summand := n*n - compensation
		preliminary := sum + summand
		compensation = (preliminary - sum) - summand
		sum = preliminary
	}
	return floatToValue(math.Sqrt(sum) * max)
}

func (r *Runtime) math_imul(call FunctionCall) Value {
	x := toUint32(call.Argument(0))
	y := toUint32(call.Argument(1))
	return intToValue(int64(int32(x * y)))
}

func (r *Runtime) math_log(call FunctionCall) Value {
	return floatToValue(math.Log(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_log1p(call FunctionCall) Value {
	return floatToValue(math.Log1p(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_log10(call FunctionCall) Value {
	return floatToValue(math.Log10(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_log2(call FunctionCall) Value {
	return floatToValue(math.Log2(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_max(call FunctionCall) Value {
	result := math.Inf(-1)
	args := call.Arguments
	for i, arg := range args {
		n := nilSafe(arg).ToFloat()
		if math.IsNaN(n) {
			args = args[i+1:]
			goto NaNLoop
		}
		result = math.Max(result, n)
	}

	return floatToValue(result)

NaNLoop:
	// All arguments still need to be coerced to number according to the specs.
	for _, arg := range args {
		nilSafe(arg).ToFloat()
	}
	return _NaN
}

func (r *Runtime) math_min(call FunctionCall) Value {
	result := math.Inf(1)
	args := call.Arguments
	for i, arg := range args {
		n := nilSafe(arg).ToFloat()
		if math.IsNaN(n) {
			args = args[i+1:]
			goto NaNLoop
		}
		result = math.Min(result, n)
	}

	return floatToValue(result)

NaNLoop:
	// All arguments still need to be coerced to number according to the specs.
	for _, arg := range args {
		nilSafe(arg).ToFloat()
	}
	return _NaN
}

func pow(x, y Value) Value {
	if x, ok := x.(valueInt); ok {
		if y, ok := y.(valueInt); ok && y >= 0 {
			if y == 0 {
				return intToValue(1)
			}
			if x == 0 {
				return intToValue(0)
			}
			ip := ipow(int64(x), int64(y))
			if ip != 0 {
				return intToValue(ip)
			}
		}
	}
	xf := x.ToFloat()
	yf := y.ToFloat()
	if math.Abs(xf) == 1 && math.IsInf(yf, 0) {
		return _NaN
	}
	if xf == 1 && math.IsNaN(yf) {
		return _NaN
	}
	return floatToValue(math.Pow(xf, yf))
}

func (r *Runtime) math_pow(call FunctionCall) Value {
	return pow(call.Argument(0), call.Argument(1))
}

func (r *Runtime) math_random(call FunctionCall) Value {
	return floatToValue(r.rand())
}

func (r *Runtime) math_round(call FunctionCall) Value {
	f := call.Argument(0).ToFloat()
	if math.IsNaN(f) {
		return _NaN
	}

	if f == 0 && math.Signbit(f) {
		return _negativeZero
	}

	t := math.Trunc(f)

	if f >= 0 {
		if f-t >= 0.5 {
			return floatToValue(t + 1)
		}
	} else {
		if t-f > 0.5 {
			return floatToValue(t - 1)
		}
	}

	return floatToValue(t)
}

func (r *Runtime) math_sign(call FunctionCall) Value {
	arg := call.Argument(0)
	num := arg.ToFloat()
	if math.IsNaN(num) || num == 0 { // this will match -0 too
		return arg
	}
	if num > 0 {
		return intToValue(1)
	}
	return intToValue(-1)
}

func (r *Runtime) math_sin(call FunctionCall) Value {
	return floatToValue(math.Sin(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_sinh(call FunctionCall) Value {
	return floatToValue(math.Sinh(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_sqrt(call FunctionCall) Value {
	return floatToValue(math.Sqrt(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_tan(call FunctionCall) Value {
	return floatToValue(math.Tan(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_tanh(call FunctionCall) Value {
	return floatToValue(math.Tanh(call.Argument(0).ToFloat()))
}

func (r *Runtime) math_trunc(call FunctionCall) Value {
	arg := call.Argument(0)
	if i, ok := arg.(valueInt); ok {
		return i
	}
	return floatToValue(math.Trunc(arg.ToFloat()))
}

func createMathTemplate() *objectTemplate {
	t := newObjectTemplate()
	t.protoFactory = func(r *Runtime) *Object {
		return r.global.ObjectPrototype
	}

	t.putStr("E", func(r *Runtime) Value { return valueProp(valueFloat(math.E), false, false, false) })
	t.putStr("LN10", func(r *Runtime) Value { return valueProp(valueFloat(math.Ln10), false, false, false) })
	t.putStr("LN2", func(r *Runtime) Value { return valueProp(valueFloat(math.Ln2), false, false, false) })
	t.putStr("LOG10E", func(r *Runtime) Value { return valueProp(valueFloat(math.Log10E), false, false, false) })
	t.putStr("LOG2E", func(r *Runtime) Value { return valueProp(valueFloat(math.Log2E), false, false, false) })
	t.putStr("PI", func(r *Runtime) Value { return valueProp(valueFloat(math.Pi), false, false, false) })
	t.putStr("SQRT1_2", func(r *Runtime) Value { return valueProp(valueFloat(sqrt1_2), false, false, false) })
	t.putStr("SQRT2", func(r *Runtime) Value { return valueProp(valueFloat(math.Sqrt2), false, false, false) })

	t.putSym(SymToStringTag, func(r *Runtime) Value { return valueProp(asciiString(classMath), false, false, true) })

	t.putStr("abs", func(r *Runtime) Value { return r.methodProp(r.math_abs, "abs", 1) })
	t.putStr("acos", func(r *Runtime) Value { return r.methodProp(r.math_acos, "acos", 1) })
	t.putStr("acosh", func(r *Runtime) Value { return r.methodProp(r.math_acosh, "acosh", 1) })
	t.putStr("asin", func(r *Runtime) Value { return r.methodProp(r.math_asin, "asin", 1) })
	t.putStr("asinh", func(r *Runtime) Value { return r.methodProp(r.math_asinh, "asinh", 1) })
	t.putStr("atan", func(r *Runtime) Value { return r.methodProp(r.math_atan, "atan", 1) })
	t.putStr("atanh", func(r *Runtime) Value { return r.methodProp(r.math_atanh, "atanh", 1) })
	t.putStr("atan2", func(r *Runtime) Value { return r.methodProp(r.math_atan2, "atan2", 2) })
	t.putStr("cbrt", func(r *Runtime) Value { return r.methodProp(r.math_cbrt, "cbrt", 1) })
	t.putStr("ceil", func(r *Runtime) Value { return r.methodProp(r.math_ceil, "ceil", 1) })
	t.putStr("clz32", func(r *Runtime) Value { return r.methodProp(r.math_clz32, "clz32", 1) })
	t.putStr("cos", func(r *Runtime) Value { return r.methodProp(r.math_cos, "cos", 1) })
	t.putStr("cosh", func(r *Runtime) Value { return r.methodProp(r.math_cosh, "cosh", 1) })
	t.putStr("exp", func(r *Runtime) Value { return r.methodProp(r.math_exp, "exp", 1) })
	t.putStr("expm1", func(r *Runtime) Value { return r.methodProp(r.math_expm1, "expm1", 1) })
	t.putStr("floor", func(r *Runtime) Value { return r.methodProp(r.math_floor, "floor", 1) })
	t.putStr("fround", func(r *Runtime) Value { return r.methodProp(r.math_fround, "fround", 1) })
	t.putStr("hypot", func(r *Runtime) Value { return r.methodProp(r.math_hypot, "hypot", 2) })
	t.putStr("imul", func(r *Runtime) Value { return r.methodProp(r.math_imul, "imul", 2) })
	t.putStr("log", func(r *Runtime) Value { return r.methodProp(r.math_log, "log", 1) })
	t.putStr("log1p", func(r *Runtime) Value { return r.methodProp(r.math_log1p, "log1p", 1) })
	t.putStr("log10", func(r *Runtime) Value { return r.methodProp(r.math_log10, "log10", 1) })
	t.putStr("log2", func(r *Runtime) Value { return r.methodProp(r.math_log2, "log2", 1) })
	t.putStr("max", func(r *Runtime) Value { return r.methodProp(r.math_max, "max", 2) })
	t.putStr("min", func(r *Runtime) Value { return r.methodProp(r.math_min, "min", 2) })
	t.putStr("pow", func(r *Runtime) Value { return r.methodProp(r.math_pow, "pow", 2) })
	t.putStr("random", func(r *Runtime) Value { return r.methodProp(r.math_random, "random", 0) })
	t.putStr("round", func(r *Runtime) Value { return r.methodProp(r.math_round, "round", 1) })
	t.putStr("sign", func(r *Runtime) Value { return r.methodProp(r.math_sign, "sign", 1) })
	t.putStr("sin", func(r *Runtime) Value { return r.methodProp(r.math_sin, "sin", 1) })
	t.putStr("sinh", func(r *Runtime) Value { return r.methodProp(r.math_sinh, "sinh", 1) })
	t.putStr("sqrt", func(r *Runtime) Value { return r.methodProp(r.math_sqrt, "sqrt", 1) })
	t.putStr("tan", func(r *Runtime) Value { return r.methodProp(r.math_tan, "tan", 1) })
	t.putStr("tanh", func(r *Runtime) Value { return r.methodProp(r.math_tanh, "tanh", 1) })
	t.putStr("trunc", func(r *Runtime) Value { return r.methodProp(r.math_trunc, "trunc", 1) })

	return t
}

var mathTemplate *objectTemplate
var mathTemplateOnce sync.Once

func getMathTemplate() *objectTemplate {
	mathTemplateOnce.Do(func() {
		mathTemplate = createMathTemplate()
	})
	return mathTemplate
}

func (r *Runtime) getMath() *Object {
	ret := r.global.Math
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Math = ret
		r.newTemplatedObject(getMathTemplate(), ret)
	}
	return ret
}
