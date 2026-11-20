package sobek

import (
	"fmt"
	"hash/maphash"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"sync"
	"unsafe"

	"github.com/grafana/sobek/unistring"
)

type valueBigInt big.Int

var (
	big0 = big.NewInt(0)
	big1 = big.NewInt(1)

	bigTwoTo64 = new(big.Int).Lsh(big1, 64)
	bigTwoTo63 = new(big.Int).Lsh(big1, 63)
)

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

// numberValue returns the Number value nearest to v, rounding ties to even.
func (v *valueBigInt) numberValue() Value {
	if i := (*big.Int)(v); i.IsInt64() {
		return intToValue(i.Int64())
	}
	f, _ := (*big.Int)(v).Float64()
	return floatToValue(f)
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
		return (*big.Int)(v).Cmp(big.NewInt(o.ToInteger())) == 0
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
	n := (*big.Int)(v)
	if n.Sign() < 0 {
		sign = 0x01
	} else {
		sign = 0x00
	}
	_ = hash.WriteByte(sign)
	if words := n.Bits(); len(words) > 0 {
		b := unsafe.Slice((*byte)(unsafe.Pointer(&words[0])),
			len(words)*int(unsafe.Sizeof(words[0])))
		_, _ = hash.Write(b)
	}
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
	switch v := v.(type) {
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
	if str == "" {
		return big0, nil
	}
	base := 10
	if len(str) > 2 && str[0] == '0' {
		switch str[1] {
		case 'x', 'X':
			str, base = str[2:], 16
		case 'b', 'B':
			str, base = str[2:], 2
		case 'o', 'O':
			str, base = str[2:], 8
		}
	}
	if base != 10 && len(str) > 0 && (str[0] == '-' || str[0] == '+') {
		return nil, strconv.ErrSyntax
	}
	var bigint big.Int
	_, ok := bigint.SetString(str, base)
	if !ok {
		return nil, strconv.ErrSyntax
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
	bits := r.toIndex(call.Argument(0).ToNumber())
	bigint := toBigInt(call.Argument(1))

	twoToBits := new(big.Int).Lsh(big1, uint(bits))
	mod := new(big.Int).Mod((*big.Int)(bigint), twoToBits)
	if bits > 0 && mod.Cmp(new(big.Int).Lsh(big1, uint(bits-1))) >= 0 {
		return (*valueBigInt)(mod.Sub(mod, twoToBits))
	} else {
		return (*valueBigInt)(mod)
	}
}

func (r *Runtime) bigint_asUintN(call FunctionCall) Value {
	bits := r.toIndex(call.Argument(0).ToNumber())
	bigint := (*big.Int)(toBigInt(call.Argument(1)))
	ret := new(big.Int).Mod(bigint, new(big.Int).Lsh(big1, uint(bits)))
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
	return (*valueBigInt)(big0)
}

func (r *Runtime) builtin_newBigInt(_ []Value, _ *Object) *Object {
	panic(r.NewTypeError("BigInt is not a constructor"))
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
