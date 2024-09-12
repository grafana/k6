package sobek

import (
	"fmt"
	"hash/maphash"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"sync"

	"github.com/grafana/sobek/unistring"
)

type valueBigInt big.Int

func (v *valueBigInt) ToInteger() int64 {
	v.ToNumber()
	return 0
}

func (v *valueBigInt) toString() String {
	return asciiString((*big.Int)(v).String())
}

func (v *valueBigInt) string() unistring.String {
	return unistring.String(v.String())
}

func (v *valueBigInt) ToString() Value {
	return v
}

func (v *valueBigInt) String() string {
	return (*big.Int)(v).String()
}

func (v *valueBigInt) ToFloat() float64 {
	v.ToNumber()
	return 0
}

func (v *valueBigInt) ToNumber() Value {
	panic(typeError("Cannot convert a BigInt value to a number"))
}

func (v *valueBigInt) ToBoolean() bool {
	return (*big.Int)(v).Sign() != 0
}

func (v *valueBigInt) ToObject(r *Runtime) *Object {
	return r.newPrimitiveObject(v, r.getBigIntPrototype(), classObject)
}

func (v *valueBigInt) SameAs(other Value) bool {
	if o, ok := other.(*valueBigInt); ok {
		return (*big.Int)(v).Cmp((*big.Int)(o)) == 0
	}
	return false
}

func (v *valueBigInt) Equals(other Value) bool {
	switch o := other.(type) {
	case *valueBigInt:
		return (*big.Int)(v).Cmp((*big.Int)(o)) == 0
	case valueInt:
		return (*big.Int)(v).Cmp(big.NewInt(int64(o))) == 0
	case valueFloat:
		if IsInfinity(o) || math.IsNaN(float64(o)) {
			return false
		}
		if f := big.NewFloat(float64(o)); f.IsInt() {
			i, _ := f.Int(nil)
			return (*big.Int)(v).Cmp(i) == 0
		}
		return false
	case String:
		bigInt, err := stringToBigInt(o.toTrimmedUTF8())
		if err != nil {
			return false
		}
		return bigInt.Cmp((*big.Int)(v)) == 0
	case valueBool:
		return (*big.Int)(v).Int64() == o.ToInteger()
	case *Object:
		return v.Equals(o.toPrimitiveNumber())
	}
	return false
}

func (v *valueBigInt) StrictEquals(other Value) bool {
	o, ok := other.(*valueBigInt)
	if ok {
		return (*big.Int)(v).Cmp((*big.Int)(o)) == 0
	}
	return false
}

func (v *valueBigInt) Export() interface{} {
	return new(big.Int).Set((*big.Int)(v))
}

func (v *valueBigInt) ExportType() reflect.Type {
	return typeBigInt
}

func (v *valueBigInt) baseObject(rt *Runtime) *Object {
	return rt.getBigIntPrototype()
}

func (v *valueBigInt) hash(hash *maphash.Hash) uint64 {
	var sign byte
	if (*big.Int)(v).Sign() < 0 {
		sign = 0x01
	} else {
		sign = 0x00
	}
	_ = hash.WriteByte(sign)
	_, _ = hash.Write((*big.Int)(v).Bytes())
	h := hash.Sum64()
	hash.Reset()
	return h
}

func toBigInt(value Value) *valueBigInt {
	// Undefined	Throw a TypeError exception.
	// Null			Throw a TypeError exception.
	// Boolean		Return 1n if prim is true and 0n if prim is false.
	// BigInt		Return prim.
	// Number		Throw a TypeError exception.
	// String		1. Let n be StringToBigInt(prim).
	//				2. If n is undefined, throw a SyntaxError exception.
	//				3. Return n.
	// Symbol		Throw a TypeError exception.
	switch prim := value.(type) {
	case *valueBigInt:
		return prim
	case String:
		bigInt, err := stringToBigInt(prim.toTrimmedUTF8())
		if err != nil {
			panic(syntaxError(fmt.Sprintf("Cannot convert %s to a BigInt", prim)))
		}
		return (*valueBigInt)(bigInt)
	case valueBool:
		return (*valueBigInt)(big.NewInt(prim.ToInteger()))
	case *Symbol:
		panic(typeError("Cannot convert Symbol to a BigInt"))
	case *Object:
		return toBigInt(prim.toPrimitiveNumber())
	default:
		panic(typeError(fmt.Sprintf("Cannot convert %s to a BigInt", prim)))
	}
}

func numberToBigInt(v Value) *valueBigInt {
	switch v := toNumeric(v).(type) {
	case *valueBigInt:
		return v
	case valueInt:
		return (*valueBigInt)(big.NewInt(v.ToInteger()))
	case valueFloat:
		if IsInfinity(v) || math.IsNaN(float64(v)) {
			panic(rangeError(fmt.Sprintf("Cannot convert %s to a BigInt", v)))
		}
		if f := big.NewFloat(float64(v)); f.IsInt() {
			n, _ := f.Int(nil)
			return (*valueBigInt)(n)
		}
		panic(rangeError(fmt.Sprintf("Cannot convert %s to a BigInt", v)))
	case *Object:
		prim := v.toPrimitiveNumber()
		switch prim.(type) {
		case valueInt, valueFloat:
			return numberToBigInt(prim)
		default:
			return toBigInt(prim)
		}
	default:
		panic(newTypeError("Cannot convert %s to a BigInt", v))
	}
}

func stringToBigInt(str string) (*big.Int, error) {
	var bigint big.Int
	n, err := stringToInt(str)
	if err != nil {
		switch {
		case isRangeErr(err):
			bigint.SetString(str, 0)
		case err == strconv.ErrSyntax:
		default:
			return nil, strconv.ErrSyntax
		}
	} else {
		bigint.SetInt64(n)
	}
	return &bigint, nil
}

func (r *Runtime) thisBigIntValue(value Value) Value {
	switch t := value.(type) {
	case *valueBigInt:
		return t
	case *Object:
		switch t := t.self.(type) {
		case *primitiveValueObject:
			return r.thisBigIntValue(t.pValue)
		case *objectGoReflect:
			if t.exportType() == typeBigInt && t.valueOf != nil {
				return t.valueOf()
			}
		}
	}
	panic(r.NewTypeError("requires that 'this' be a BigInt"))
}

func (r *Runtime) bigintproto_valueOf(call FunctionCall) Value {
	return r.thisBigIntValue(call.This)
}

func (r *Runtime) bigintproto_toString(call FunctionCall) Value {
	x := (*big.Int)(r.thisBigIntValue(call.This).(*valueBigInt))
	radix := call.Argument(0)
	var radixMV int

	if radix == _undefined {
		radixMV = 10
	} else {
		radixMV = int(radix.ToInteger())
		if radixMV < 2 || radixMV > 36 {
			panic(r.newError(r.getRangeError(), "radix must be an integer between 2 and 36"))
		}
	}

	return asciiString(x.Text(radixMV))
}

func (r *Runtime) bigint_asIntN(call FunctionCall) Value {
	if len(call.Arguments) < 2 {
		panic(r.NewTypeError("Cannot convert undefined to a BigInt"))
	}
	bits := r.toIndex(call.Argument(0).ToNumber())
	if bits < 0 {
		panic(r.NewTypeError("Invalid value: not (convertible to) a safe integer"))
	}
	bigint := toBigInt(call.Argument(1))

	twoToBits := new(big.Int).Lsh(big.NewInt(1), uint(bits))
	mod := new(big.Int).Mod((*big.Int)(bigint), twoToBits)
	if bits > 0 && mod.Cmp(new(big.Int).Lsh(big.NewInt(1), uint(bits-1))) >= 0 {
		return (*valueBigInt)(mod.Sub(mod, twoToBits))
	} else {
		return (*valueBigInt)(mod)
	}
}

func (r *Runtime) bigint_asUintN(call FunctionCall) Value {
	if len(call.Arguments) < 2 {
		panic(r.NewTypeError("Cannot convert undefined to a BigInt"))
	}
	bits := r.toIndex(call.Argument(0).ToNumber())
	if bits < 0 {
		panic(r.NewTypeError("Invalid value: not (convertible to) a safe integer"))
	}
	bigint := (*big.Int)(toBigInt(call.Argument(1)))
	ret := new(big.Int).Mod(bigint, new(big.Int).Lsh(big.NewInt(1), uint(bits)))
	return (*valueBigInt)(ret)
}

var (
	bigintTemplate     *objectTemplate
	bigintTemplateOnce sync.Once
)

func getBigIntTemplate() *objectTemplate {
	bigintTemplateOnce.Do(func() {
		bigintTemplate = createBigIntTemplate()
	})
	return bigintTemplate
}

func createBigIntTemplate() *objectTemplate {
	t := newObjectTemplate()
	t.protoFactory = func(r *Runtime) *Object {
		return r.getFunctionPrototype()
	}

	t.putStr("name", func(r *Runtime) Value { return valueProp(asciiString("BigInt"), false, false, true) })
	t.putStr("length", func(r *Runtime) Value { return valueProp(intToValue(1), false, false, true) })
	t.putStr("prototype", func(r *Runtime) Value { return valueProp(r.getBigIntPrototype(), false, false, false) })

	t.putStr("asIntN", func(r *Runtime) Value { return r.methodProp(r.bigint_asIntN, "asIntN", 2) })
	t.putStr("asUintN", func(r *Runtime) Value { return r.methodProp(r.bigint_asUintN, "asUintN", 2) })

	return t
}

func (r *Runtime) builtin_BigInt(call FunctionCall) Value {
	if len(call.Arguments) > 0 {
		switch v := call.Argument(0).(type) {
		case *valueBigInt, valueInt, valueFloat, *Object:
			return numberToBigInt(v)
		default:
			return toBigInt(v)
		}
	}
	return (*valueBigInt)(big.NewInt(0))
}

func (r *Runtime) builtin_newBigInt(args []Value, newTarget *Object) *Object {
	if newTarget != nil {
		panic(r.NewTypeError("BigInt is not a constructor"))
	}
	var v Value
	if len(args) > 0 {
		v = numberToBigInt(args[0])
	} else {
		v = (*valueBigInt)(big.NewInt(0))
	}
	return r.newPrimitiveObject(v, newTarget, classObject)
}

func (r *Runtime) getBigInt() *Object {
	ret := r.global.BigInt
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.BigInt = ret
		r.newTemplatedFuncObject(getBigIntTemplate(), ret, r.builtin_BigInt,
			r.wrapNativeConstruct(r.builtin_newBigInt, ret, r.getBigIntPrototype()))
	}
	return ret
}

func createBigIntProtoTemplate() *objectTemplate {
	t := newObjectTemplate()
	t.protoFactory = func(r *Runtime) *Object {
		return r.global.ObjectPrototype
	}

	t.putStr("length", func(r *Runtime) Value { return valueProp(intToValue(0), false, false, true) })
	t.putStr("name", func(r *Runtime) Value { return valueProp(asciiString("BigInt"), false, false, true) })
	t.putStr("constructor", func(r *Runtime) Value { return valueProp(r.getBigInt(), true, false, true) })

	t.putStr("toLocaleString", func(r *Runtime) Value { return r.methodProp(r.bigintproto_toString, "toLocaleString", 0) })
	t.putStr("toString", func(r *Runtime) Value { return r.methodProp(r.bigintproto_toString, "toString", 0) })
	t.putStr("valueOf", func(r *Runtime) Value { return r.methodProp(r.bigintproto_valueOf, "valueOf", 0) })
	t.putSym(SymToStringTag, func(r *Runtime) Value { return valueProp(asciiString("BigInt"), false, false, true) })

	return t
}

var (
	bigintProtoTemplate     *objectTemplate
	bigintProtoTemplateOnce sync.Once
)

func getBigIntProtoTemplate() *objectTemplate {
	bigintProtoTemplateOnce.Do(func() {
		bigintProtoTemplate = createBigIntProtoTemplate()
	})
	return bigintProtoTemplate
}

func (r *Runtime) getBigIntPrototype() *Object {
	ret := r.global.BigIntPrototype
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.BigIntPrototype = ret
		o := r.newTemplatedObject(getBigIntProtoTemplate(), ret)
		o.class = classObject
	}
	return ret
}
