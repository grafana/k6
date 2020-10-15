package goja

import (
	"hash/maphash"
	"math"
	"reflect"
	"strconv"
	"unsafe"

	"github.com/dop251/goja/ftoa"
	"github.com/dop251/goja/unistring"
)

var (
	// Not goroutine-safe, do not use for anything other than package level init
	pkgHasher maphash.Hash

	hashFalse = randomHash()
	hashTrue  = randomHash()
	hashNull  = randomHash()
	hashUndef = randomHash()
)

// Not goroutine-safe, do not use for anything other than package level init
func randomHash() uint64 {
	pkgHasher.WriteByte(0)
	return pkgHasher.Sum64()
}

var (
	valueFalse    Value = valueBool(false)
	valueTrue     Value = valueBool(true)
	_null         Value = valueNull{}
	_NaN          Value = valueFloat(math.NaN())
	_positiveInf  Value = valueFloat(math.Inf(+1))
	_negativeInf  Value = valueFloat(math.Inf(-1))
	_positiveZero Value = valueInt(0)
	negativeZero        = math.Float64frombits(0 | (1 << 63))
	_negativeZero Value = valueFloat(negativeZero)
	_epsilon            = valueFloat(2.2204460492503130808472633361816e-16)
	_undefined    Value = valueUndefined{}
)

var (
	reflectTypeInt    = reflect.TypeOf(int64(0))
	reflectTypeBool   = reflect.TypeOf(false)
	reflectTypeNil    = reflect.TypeOf(nil)
	reflectTypeFloat  = reflect.TypeOf(float64(0))
	reflectTypeMap    = reflect.TypeOf(map[string]interface{}{})
	reflectTypeArray  = reflect.TypeOf([]interface{}{})
	reflectTypeString = reflect.TypeOf("")
)

var intCache [256]Value

type Value interface {
	ToInteger() int64
	toString() valueString
	string() unistring.String
	ToString() Value
	String() string
	ToFloat() float64
	ToNumber() Value
	ToBoolean() bool
	ToObject(*Runtime) *Object
	SameAs(Value) bool
	Equals(Value) bool
	StrictEquals(Value) bool
	Export() interface{}
	ExportType() reflect.Type

	baseObject(r *Runtime) *Object

	hash(hasher *maphash.Hash) uint64
}

type valueContainer interface {
	toValue(*Runtime) Value
}

type typeError string
type rangeError string

type valueInt int64
type valueFloat float64
type valueBool bool
type valueNull struct{}
type valueUndefined struct {
	valueNull
}
type valueSymbol struct {
	h    uintptr
	desc valueString
}

type valueUnresolved struct {
	r   *Runtime
	ref unistring.String
}

type memberUnresolved struct {
	valueUnresolved
}

type valueProperty struct {
	value        Value
	writable     bool
	configurable bool
	enumerable   bool
	accessor     bool
	getterFunc   *Object
	setterFunc   *Object
}

func propGetter(o Value, v Value, r *Runtime) *Object {
	if v == _undefined {
		return nil
	}
	if obj, ok := v.(*Object); ok {
		if _, ok := obj.self.assertCallable(); ok {
			return obj
		}
	}
	r.typeErrorResult(true, "Getter must be a function: %s", v.toString())
	return nil
}

func propSetter(o Value, v Value, r *Runtime) *Object {
	if v == _undefined {
		return nil
	}
	if obj, ok := v.(*Object); ok {
		if _, ok := obj.self.assertCallable(); ok {
			return obj
		}
	}
	r.typeErrorResult(true, "Setter must be a function: %s", v.toString())
	return nil
}

func fToStr(num float64, mode ftoa.FToStrMode, prec int) string {
	var buf1 [128]byte
	return string(ftoa.FToStr(num, mode, prec, buf1[:0]))
}

func (i valueInt) ToInteger() int64 {
	return int64(i)
}

func (i valueInt) toString() valueString {
	return asciiString(i.String())
}

func (i valueInt) string() unistring.String {
	return unistring.String(i.String())
}

func (i valueInt) ToString() Value {
	return i
}

func (i valueInt) String() string {
	return strconv.FormatInt(int64(i), 10)
}

func (i valueInt) ToFloat() float64 {
	return float64(i)
}

func (i valueInt) ToBoolean() bool {
	return i != 0
}

func (i valueInt) ToObject(r *Runtime) *Object {
	return r.newPrimitiveObject(i, r.global.NumberPrototype, classNumber)
}

func (i valueInt) ToNumber() Value {
	return i
}

func (i valueInt) SameAs(other Value) bool {
	return i == other
}

func (i valueInt) Equals(other Value) bool {
	switch o := other.(type) {
	case valueInt:
		return i == o
	case valueFloat:
		return float64(i) == float64(o)
	case valueString:
		return o.ToNumber().Equals(i)
	case valueBool:
		return int64(i) == o.ToInteger()
	case *Object:
		return i.Equals(o.toPrimitiveNumber())
	}

	return false
}

func (i valueInt) StrictEquals(other Value) bool {
	switch o := other.(type) {
	case valueInt:
		return i == o
	case valueFloat:
		return float64(i) == float64(o)
	}

	return false
}

func (i valueInt) baseObject(r *Runtime) *Object {
	return r.global.NumberPrototype
}

func (i valueInt) Export() interface{} {
	return int64(i)
}

func (i valueInt) ExportType() reflect.Type {
	return reflectTypeInt
}

func (i valueInt) hash(*maphash.Hash) uint64 {
	return uint64(i)
}

func (b valueBool) ToInteger() int64 {
	if b {
		return 1
	}
	return 0
}

func (b valueBool) toString() valueString {
	if b {
		return stringTrue
	}
	return stringFalse
}

func (b valueBool) ToString() Value {
	return b
}

func (b valueBool) String() string {
	if b {
		return "true"
	}
	return "false"
}

func (b valueBool) string() unistring.String {
	return unistring.String(b.String())
}

func (b valueBool) ToFloat() float64 {
	if b {
		return 1.0
	}
	return 0
}

func (b valueBool) ToBoolean() bool {
	return bool(b)
}

func (b valueBool) ToObject(r *Runtime) *Object {
	return r.newPrimitiveObject(b, r.global.BooleanPrototype, "Boolean")
}

func (b valueBool) ToNumber() Value {
	if b {
		return valueInt(1)
	}
	return valueInt(0)
}

func (b valueBool) SameAs(other Value) bool {
	if other, ok := other.(valueBool); ok {
		return b == other
	}
	return false
}

func (b valueBool) Equals(other Value) bool {
	if o, ok := other.(valueBool); ok {
		return b == o
	}

	if b {
		return other.Equals(intToValue(1))
	} else {
		return other.Equals(intToValue(0))
	}

}

func (b valueBool) StrictEquals(other Value) bool {
	if other, ok := other.(valueBool); ok {
		return b == other
	}
	return false
}

func (b valueBool) baseObject(r *Runtime) *Object {
	return r.global.BooleanPrototype
}

func (b valueBool) Export() interface{} {
	return bool(b)
}

func (b valueBool) ExportType() reflect.Type {
	return reflectTypeBool
}

func (b valueBool) hash(*maphash.Hash) uint64 {
	if b {
		return hashTrue
	}

	return hashFalse
}

func (n valueNull) ToInteger() int64 {
	return 0
}

func (n valueNull) toString() valueString {
	return stringNull
}

func (n valueNull) string() unistring.String {
	return stringNull.string()
}

func (n valueNull) ToString() Value {
	return n
}

func (n valueNull) String() string {
	return "null"
}

func (u valueUndefined) toString() valueString {
	return stringUndefined
}

func (u valueUndefined) ToString() Value {
	return u
}

func (u valueUndefined) String() string {
	return "undefined"
}

func (u valueUndefined) string() unistring.String {
	return "undefined"
}

func (u valueUndefined) ToNumber() Value {
	return _NaN
}

func (u valueUndefined) SameAs(other Value) bool {
	_, same := other.(valueUndefined)
	return same
}

func (u valueUndefined) StrictEquals(other Value) bool {
	_, same := other.(valueUndefined)
	return same
}

func (u valueUndefined) ToFloat() float64 {
	return math.NaN()
}

func (u valueUndefined) hash(*maphash.Hash) uint64 {
	return hashUndef
}

func (n valueNull) ToFloat() float64 {
	return 0
}

func (n valueNull) ToBoolean() bool {
	return false
}

func (n valueNull) ToObject(r *Runtime) *Object {
	r.typeErrorResult(true, "Cannot convert undefined or null to object")
	return nil
	//return r.newObject()
}

func (n valueNull) ToNumber() Value {
	return intToValue(0)
}

func (n valueNull) SameAs(other Value) bool {
	_, same := other.(valueNull)
	return same
}

func (n valueNull) Equals(other Value) bool {
	switch other.(type) {
	case valueUndefined, valueNull:
		return true
	}
	return false
}

func (n valueNull) StrictEquals(other Value) bool {
	_, same := other.(valueNull)
	return same
}

func (n valueNull) baseObject(*Runtime) *Object {
	return nil
}

func (n valueNull) Export() interface{} {
	return nil
}

func (n valueNull) ExportType() reflect.Type {
	return reflectTypeNil
}

func (n valueNull) hash(*maphash.Hash) uint64 {
	return hashNull
}

func (p *valueProperty) ToInteger() int64 {
	return 0
}

func (p *valueProperty) toString() valueString {
	return stringEmpty
}

func (p *valueProperty) string() unistring.String {
	return ""
}

func (p *valueProperty) ToString() Value {
	return _undefined
}

func (p *valueProperty) String() string {
	return ""
}

func (p *valueProperty) ToFloat() float64 {
	return math.NaN()
}

func (p *valueProperty) ToBoolean() bool {
	return false
}

func (p *valueProperty) ToObject(*Runtime) *Object {
	return nil
}

func (p *valueProperty) ToNumber() Value {
	return nil
}

func (p *valueProperty) isWritable() bool {
	return p.writable || p.setterFunc != nil
}

func (p *valueProperty) get(this Value) Value {
	if p.getterFunc == nil {
		if p.value != nil {
			return p.value
		}
		return _undefined
	}
	call, _ := p.getterFunc.self.assertCallable()
	return call(FunctionCall{
		This: this,
	})
}

func (p *valueProperty) set(this, v Value) {
	if p.setterFunc == nil {
		p.value = v
		return
	}
	call, _ := p.setterFunc.self.assertCallable()
	call(FunctionCall{
		This:      this,
		Arguments: []Value{v},
	})
}

func (p *valueProperty) SameAs(other Value) bool {
	if otherProp, ok := other.(*valueProperty); ok {
		return p == otherProp
	}
	return false
}

func (p *valueProperty) Equals(Value) bool {
	return false
}

func (p *valueProperty) StrictEquals(Value) bool {
	return false
}

func (p *valueProperty) baseObject(r *Runtime) *Object {
	r.typeErrorResult(true, "BUG: baseObject() is called on valueProperty") // TODO error message
	return nil
}

func (p *valueProperty) Export() interface{} {
	panic("Cannot export valueProperty")
}

func (p *valueProperty) ExportType() reflect.Type {
	panic("Cannot export valueProperty")
}

func (p *valueProperty) hash(*maphash.Hash) uint64 {
	panic("valueProperty should never be used in maps or sets")
}

func floatToIntClip(n float64) int64 {
	switch {
	case math.IsNaN(n):
		return 0
	case n >= math.MaxInt64:
		return math.MaxInt64
	case n <= math.MinInt64:
		return math.MinInt64
	}
	return int64(n)
}

func (f valueFloat) ToInteger() int64 {
	return floatToIntClip(float64(f))
}

func (f valueFloat) toString() valueString {
	return asciiString(f.String())
}

func (f valueFloat) string() unistring.String {
	return unistring.String(f.String())
}

func (f valueFloat) ToString() Value {
	return f
}

func (f valueFloat) String() string {
	return fToStr(float64(f), ftoa.ModeStandard, 0)
}

func (f valueFloat) ToFloat() float64 {
	return float64(f)
}

func (f valueFloat) ToBoolean() bool {
	return float64(f) != 0.0 && !math.IsNaN(float64(f))
}

func (f valueFloat) ToObject(r *Runtime) *Object {
	return r.newPrimitiveObject(f, r.global.NumberPrototype, "Number")
}

func (f valueFloat) ToNumber() Value {
	return f
}

func (f valueFloat) SameAs(other Value) bool {
	switch o := other.(type) {
	case valueFloat:
		this := float64(f)
		o1 := float64(o)
		if math.IsNaN(this) && math.IsNaN(o1) {
			return true
		} else {
			ret := this == o1
			if ret && this == 0 {
				ret = math.Signbit(this) == math.Signbit(o1)
			}
			return ret
		}
	case valueInt:
		this := float64(f)
		ret := this == float64(o)
		if ret && this == 0 {
			ret = !math.Signbit(this)
		}
		return ret
	}

	return false
}

func (f valueFloat) Equals(other Value) bool {
	switch o := other.(type) {
	case valueFloat:
		return f == o
	case valueInt:
		return float64(f) == float64(o)
	case valueString, valueBool:
		return float64(f) == o.ToFloat()
	case *Object:
		return f.Equals(o.toPrimitiveNumber())
	}

	return false
}

func (f valueFloat) StrictEquals(other Value) bool {
	switch o := other.(type) {
	case valueFloat:
		return f == o
	case valueInt:
		return float64(f) == float64(o)
	}

	return false
}

func (f valueFloat) baseObject(r *Runtime) *Object {
	return r.global.NumberPrototype
}

func (f valueFloat) Export() interface{} {
	return float64(f)
}

func (f valueFloat) ExportType() reflect.Type {
	return reflectTypeFloat
}

func (f valueFloat) hash(*maphash.Hash) uint64 {
	if f == _negativeZero {
		return 0
	}
	return math.Float64bits(float64(f))
}

func (o *Object) ToInteger() int64 {
	return o.toPrimitiveNumber().ToNumber().ToInteger()
}

func (o *Object) toString() valueString {
	return o.toPrimitiveString().toString()
}

func (o *Object) string() unistring.String {
	return o.toPrimitiveString().string()
}

func (o *Object) ToString() Value {
	return o.toPrimitiveString().ToString()
}

func (o *Object) String() string {
	return o.toPrimitiveString().String()
}

func (o *Object) ToFloat() float64 {
	return o.toPrimitiveNumber().ToFloat()
}

func (o *Object) ToBoolean() bool {
	return true
}

func (o *Object) ToObject(*Runtime) *Object {
	return o
}

func (o *Object) ToNumber() Value {
	return o.toPrimitiveNumber().ToNumber()
}

func (o *Object) SameAs(other Value) bool {
	if other, ok := other.(*Object); ok {
		return o == other
	}
	return false
}

func (o *Object) Equals(other Value) bool {
	if other, ok := other.(*Object); ok {
		return o == other || o.self.equal(other.self)
	}

	switch o1 := other.(type) {
	case valueInt, valueFloat, valueString:
		return o.toPrimitive().Equals(other)
	case valueBool:
		return o.Equals(o1.ToNumber())
	}

	return false
}

func (o *Object) StrictEquals(other Value) bool {
	if other, ok := other.(*Object); ok {
		return o == other || o.self.equal(other.self)
	}
	return false
}

func (o *Object) baseObject(*Runtime) *Object {
	return o
}

func (o *Object) Export() interface{} {
	return o.self.export(&objectExportCtx{})
}

func (o *Object) ExportType() reflect.Type {
	return o.self.exportType()
}

func (o *Object) hash(*maphash.Hash) uint64 {
	return o.getId()
}

func (o *Object) Get(name string) Value {
	return o.self.getStr(unistring.NewFromString(name), nil)
}

func (o *Object) Keys() (keys []string) {
	names := o.self.ownKeys(false, nil)
	keys = make([]string, 0, len(names))
	for _, name := range names {
		keys = append(keys, name.String())
	}

	return
}

// DefineDataProperty is a Go equivalent of Object.defineProperty(o, name, {value: value, writable: writable,
// configurable: configurable, enumerable: enumerable})
func (o *Object) DefineDataProperty(name string, value Value, writable, configurable, enumerable Flag) error {
	return tryFunc(func() {
		o.self.defineOwnPropertyStr(unistring.NewFromString(name), PropertyDescriptor{
			Value:        value,
			Writable:     writable,
			Configurable: configurable,
			Enumerable:   enumerable,
		}, true)
	})
}

// DefineAccessorProperty is a Go equivalent of Object.defineProperty(o, name, {get: getter, set: setter,
// configurable: configurable, enumerable: enumerable})
func (o *Object) DefineAccessorProperty(name string, getter, setter Value, configurable, enumerable Flag) error {
	return tryFunc(func() {
		o.self.defineOwnPropertyStr(unistring.NewFromString(name), PropertyDescriptor{
			Getter:       getter,
			Setter:       setter,
			Configurable: configurable,
			Enumerable:   enumerable,
		}, true)
	})
}

func (o *Object) Set(name string, value interface{}) error {
	return tryFunc(func() {
		o.self.setOwnStr(unistring.NewFromString(name), o.runtime.ToValue(value), true)
	})
}

func (o *Object) Delete(name string) error {
	return tryFunc(func() {
		o.self.deleteStr(unistring.NewFromString(name), true)
	})
}

// MarshalJSON returns JSON representation of the Object. It is equivalent to JSON.stringify(o).
// Note, this implements json.Marshaler so that json.Marshal() can be used without the need to Export().
func (o *Object) MarshalJSON() ([]byte, error) {
	ctx := _builtinJSON_stringifyContext{
		r: o.runtime,
	}
	ex := o.runtime.vm.try(func() {
		if !ctx.do(o) {
			ctx.buf.WriteString("null")
		}
	})
	if ex != nil {
		return nil, ex
	}
	return ctx.buf.Bytes(), nil
}

// ClassName returns the class name
func (o *Object) ClassName() string {
	return o.self.className()
}

func (o valueUnresolved) throw() {
	o.r.throwReferenceError(o.ref)
}

func (o valueUnresolved) ToInteger() int64 {
	o.throw()
	return 0
}

func (o valueUnresolved) toString() valueString {
	o.throw()
	return nil
}

func (o valueUnresolved) string() unistring.String {
	o.throw()
	return ""
}

func (o valueUnresolved) ToString() Value {
	o.throw()
	return nil
}

func (o valueUnresolved) String() string {
	o.throw()
	return ""
}

func (o valueUnresolved) ToFloat() float64 {
	o.throw()
	return 0
}

func (o valueUnresolved) ToBoolean() bool {
	o.throw()
	return false
}

func (o valueUnresolved) ToObject(*Runtime) *Object {
	o.throw()
	return nil
}

func (o valueUnresolved) ToNumber() Value {
	o.throw()
	return nil
}

func (o valueUnresolved) SameAs(Value) bool {
	o.throw()
	return false
}

func (o valueUnresolved) Equals(Value) bool {
	o.throw()
	return false
}

func (o valueUnresolved) StrictEquals(Value) bool {
	o.throw()
	return false
}

func (o valueUnresolved) baseObject(*Runtime) *Object {
	o.throw()
	return nil
}

func (o valueUnresolved) Export() interface{} {
	o.throw()
	return nil
}

func (o valueUnresolved) ExportType() reflect.Type {
	o.throw()
	return nil
}

func (o valueUnresolved) hash(*maphash.Hash) uint64 {
	o.throw()
	return 0
}

func (s *valueSymbol) ToInteger() int64 {
	panic(typeError("Cannot convert a Symbol value to a number"))
}

func (s *valueSymbol) toString() valueString {
	panic(typeError("Cannot convert a Symbol value to a string"))
}

func (s *valueSymbol) ToString() Value {
	return s
}

func (s *valueSymbol) String() string {
	return s.desc.String()
}

func (s *valueSymbol) string() unistring.String {
	return s.desc.string()
}

func (s *valueSymbol) ToFloat() float64 {
	panic(typeError("Cannot convert a Symbol value to a number"))
}

func (s *valueSymbol) ToNumber() Value {
	panic(typeError("Cannot convert a Symbol value to a number"))
}

func (s *valueSymbol) ToBoolean() bool {
	return true
}

func (s *valueSymbol) ToObject(r *Runtime) *Object {
	return s.baseObject(r)
}

func (s *valueSymbol) SameAs(other Value) bool {
	if s1, ok := other.(*valueSymbol); ok {
		return s == s1
	}
	return false
}

func (s *valueSymbol) Equals(o Value) bool {
	return s.SameAs(o)
}

func (s *valueSymbol) StrictEquals(o Value) bool {
	return s.SameAs(o)
}

func (s *valueSymbol) Export() interface{} {
	return s.String()
}

func (s *valueSymbol) ExportType() reflect.Type {
	return reflectTypeString
}

func (s *valueSymbol) baseObject(r *Runtime) *Object {
	return r.newPrimitiveObject(s, r.global.SymbolPrototype, "Symbol")
}

func (s *valueSymbol) hash(*maphash.Hash) uint64 {
	return uint64(s.h)
}

func exportValue(v Value, ctx *objectExportCtx) interface{} {
	if obj, ok := v.(*Object); ok {
		return obj.self.export(ctx)
	}
	return v.Export()
}

func newSymbol(s valueString) *valueSymbol {
	r := &valueSymbol{
		desc: asciiString("Symbol(").concat(s).concat(asciiString(")")),
	}
	// This may need to be reconsidered in the future.
	// Depending on changes in Go's allocation policy and/or introduction of a compacting GC
	// this may no longer provide sufficient dispersion. The alternative, however, is a globally
	// synchronised random generator/hasher/sequencer and I don't want to go down that route just yet.
	r.h = uintptr(unsafe.Pointer(r))
	return r
}

func init() {
	for i := 0; i < 256; i++ {
		intCache[i] = valueInt(i - 128)
	}
	_positiveZero = intToValue(0)
}
