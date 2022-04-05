package goja

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/dop251/goja/file"
	"go/ast"
	"hash/maphash"
	"math"
	"math/bits"
	"math/rand"
	"reflect"
	"runtime"
	"strconv"
	"time"

	"golang.org/x/text/collate"

	js_ast "github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
	"github.com/dop251/goja/unistring"
)

const (
	sqrt1_2 float64 = math.Sqrt2 / 2

	deoptimiseRegexp = false
)

var (
	typeCallable = reflect.TypeOf(Callable(nil))
	typeValue    = reflect.TypeOf((*Value)(nil)).Elem()
	typeObject   = reflect.TypeOf((*Object)(nil))
	typeTime     = reflect.TypeOf(time.Time{})
	typeBytes    = reflect.TypeOf(([]byte)(nil))
)

type iterationKind int

const (
	iterationKindKey iterationKind = iota
	iterationKindValue
	iterationKindKeyValue
)

type global struct {
	stash    stash
	varNames map[unistring.String]struct{}

	Object   *Object
	Array    *Object
	Function *Object
	String   *Object
	Number   *Object
	Boolean  *Object
	RegExp   *Object
	Date     *Object
	Symbol   *Object
	Proxy    *Object
	Promise  *Object

	ArrayBuffer       *Object
	DataView          *Object
	TypedArray        *Object
	Uint8Array        *Object
	Uint8ClampedArray *Object
	Int8Array         *Object
	Uint16Array       *Object
	Int16Array        *Object
	Uint32Array       *Object
	Int32Array        *Object
	Float32Array      *Object
	Float64Array      *Object

	WeakSet *Object
	WeakMap *Object
	Map     *Object
	Set     *Object

	Error          *Object
	AggregateError *Object
	TypeError      *Object
	ReferenceError *Object
	SyntaxError    *Object
	RangeError     *Object
	EvalError      *Object
	URIError       *Object

	GoError *Object

	ObjectPrototype   *Object
	ArrayPrototype    *Object
	NumberPrototype   *Object
	StringPrototype   *Object
	BooleanPrototype  *Object
	FunctionPrototype *Object
	RegExpPrototype   *Object
	DatePrototype     *Object
	SymbolPrototype   *Object

	ArrayBufferPrototype *Object
	DataViewPrototype    *Object
	TypedArrayPrototype  *Object
	WeakSetPrototype     *Object
	WeakMapPrototype     *Object
	MapPrototype         *Object
	SetPrototype         *Object
	PromisePrototype     *Object

	IteratorPrototype             *Object
	ArrayIteratorPrototype        *Object
	MapIteratorPrototype          *Object
	SetIteratorPrototype          *Object
	StringIteratorPrototype       *Object
	RegExpStringIteratorPrototype *Object

	ErrorPrototype          *Object
	AggregateErrorPrototype *Object
	TypeErrorPrototype      *Object
	SyntaxErrorPrototype    *Object
	RangeErrorPrototype     *Object
	ReferenceErrorPrototype *Object
	EvalErrorPrototype      *Object
	URIErrorPrototype       *Object

	GoErrorPrototype *Object

	Eval *Object

	thrower         *Object
	throwerProperty Value

	stdRegexpProto *guardedObject

	weakSetAdder  *Object
	weakMapAdder  *Object
	mapAdder      *Object
	setAdder      *Object
	arrayValues   *Object
	arrayToString *Object
}

type Flag int

const (
	FLAG_NOT_SET Flag = iota
	FLAG_FALSE
	FLAG_TRUE
)

func (f Flag) Bool() bool {
	return f == FLAG_TRUE
}

func ToFlag(b bool) Flag {
	if b {
		return FLAG_TRUE
	}
	return FLAG_FALSE
}

type RandSource func() float64

type Now func() time.Time

type Runtime struct {
	global          global
	globalObject    *Object
	stringSingleton *stringObject
	rand            RandSource
	now             Now
	_collator       *collate.Collator
	parserOptions   []parser.Option

	symbolRegistry map[unistring.String]*Symbol

	typeInfoCache   map[reflect.Type]*reflectTypeInfo
	fieldNameMapper FieldNameMapper

	vm    *vm
	hash  *maphash.Hash
	idSeq uint64

	jobQueue []func()

	promiseRejectionTracker PromiseRejectionTracker
}

type StackFrame struct {
	prg      *Program
	funcName unistring.String
	pc       int
}

func (f *StackFrame) SrcName() string {
	if f.prg == nil {
		return "<native>"
	}
	return f.prg.src.Name()
}

func (f *StackFrame) FuncName() string {
	if f.funcName == "" && f.prg == nil {
		return "<native>"
	}
	if f.funcName == "" {
		return "<anonymous>"
	}
	return f.funcName.String()
}

func (f *StackFrame) Position() file.Position {
	if f.prg == nil || f.prg.src == nil {
		return file.Position{}
	}
	return f.prg.src.Position(f.prg.sourceOffset(f.pc))
}

func (f *StackFrame) WriteToValueBuilder(b *valueStringBuilder) {
	if f.prg != nil {
		if n := f.prg.funcName; n != "" {
			b.WriteString(stringValueFromRaw(n))
			b.WriteASCII(" (")
		}
		p := f.Position()
		if p.Filename != "" {
			b.WriteASCII(p.Filename)
		} else {
			b.WriteASCII("<eval>")
		}
		b.WriteRune(':')
		b.WriteASCII(strconv.Itoa(p.Line))
		b.WriteRune(':')
		b.WriteASCII(strconv.Itoa(p.Column))
		b.WriteRune('(')
		b.WriteASCII(strconv.Itoa(f.pc))
		b.WriteRune(')')
		if f.prg.funcName != "" {
			b.WriteRune(')')
		}
	} else {
		if f.funcName != "" {
			b.WriteString(stringValueFromRaw(f.funcName))
			b.WriteASCII(" (")
		}
		b.WriteASCII("native")
		if f.funcName != "" {
			b.WriteRune(')')
		}
	}
}

func (f *StackFrame) Write(b *bytes.Buffer) {
	if f.prg != nil {
		if n := f.prg.funcName; n != "" {
			b.WriteString(n.String())
			b.WriteString(" (")
		}
		p := f.Position()
		if p.Filename != "" {
			b.WriteString(p.Filename)
		} else {
			b.WriteString("<eval>")
		}
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(p.Line))
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(p.Column))
		b.WriteByte('(')
		b.WriteString(strconv.Itoa(f.pc))
		b.WriteByte(')')
		if f.prg.funcName != "" {
			b.WriteByte(')')
		}
	} else {
		if f.funcName != "" {
			b.WriteString(f.funcName.String())
			b.WriteString(" (")
		}
		b.WriteString("native")
		if f.funcName != "" {
			b.WriteByte(')')
		}
	}
}

type Exception struct {
	val   Value
	stack []StackFrame
}

type uncatchableException struct {
	err error
}

func (ue *uncatchableException) Unwrap() error {
	return ue.err
}

type InterruptedError struct {
	Exception
	iface interface{}
}

func (e *InterruptedError) Unwrap() error {
	if err, ok := e.iface.(error); ok {
		return err
	}
	return nil
}

type StackOverflowError struct {
	Exception
}

func (e *InterruptedError) Value() interface{} {
	return e.iface
}

func (e *InterruptedError) String() string {
	if e == nil {
		return "<nil>"
	}
	var b bytes.Buffer
	if e.iface != nil {
		b.WriteString(fmt.Sprint(e.iface))
		b.WriteByte('\n')
	}
	e.writeFullStack(&b)
	return b.String()
}

func (e *InterruptedError) Error() string {
	if e == nil || e.iface == nil {
		return "<nil>"
	}
	var b bytes.Buffer
	b.WriteString(fmt.Sprint(e.iface))
	e.writeShortStack(&b)
	return b.String()
}

func (e *Exception) writeFullStack(b *bytes.Buffer) {
	for _, frame := range e.stack {
		b.WriteString("\tat ")
		frame.Write(b)
		b.WriteByte('\n')
	}
}

func (e *Exception) writeShortStack(b *bytes.Buffer) {
	if len(e.stack) > 0 && (e.stack[0].prg != nil || e.stack[0].funcName != "") {
		b.WriteString(" at ")
		e.stack[0].Write(b)
	}
}

func (e *Exception) String() string {
	if e == nil {
		return "<nil>"
	}
	var b bytes.Buffer
	if e.val != nil {
		b.WriteString(e.val.String())
		b.WriteByte('\n')
	}
	e.writeFullStack(&b)
	return b.String()
}

func (e *Exception) Error() string {
	if e == nil || e.val == nil {
		return "<nil>"
	}
	var b bytes.Buffer
	b.WriteString(e.val.String())
	e.writeShortStack(&b)
	return b.String()
}

func (e *Exception) Value() Value {
	return e.val
}

func (r *Runtime) addToGlobal(name string, value Value) {
	r.globalObject.self._putProp(unistring.String(name), value, true, false, true)
}

func (r *Runtime) createIterProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.global.ObjectPrototype, classObject)

	o._putSym(SymIterator, valueProp(r.newNativeFunc(r.returnThis, nil, "[Symbol.iterator]", nil, 0), true, false, true))
	return o
}

func (r *Runtime) init() {
	r.rand = rand.Float64
	r.now = time.Now
	r.global.ObjectPrototype = r.newBaseObject(nil, classObject).val
	r.globalObject = r.NewObject()

	r.vm = &vm{
		r: r,
	}
	r.vm.init()

	funcProto := r.newNativeFunc(func(FunctionCall) Value {
		return _undefined
	}, nil, " ", nil, 0)
	r.global.FunctionPrototype = funcProto
	funcProtoObj := funcProto.self.(*nativeFuncObject)

	r.global.IteratorPrototype = r.newLazyObject(r.createIterProto)

	r.initObject()
	r.initFunction()
	r.initArray()
	r.initString()
	r.initGlobalObject()
	r.initNumber()
	r.initRegExp()
	r.initDate()
	r.initBoolean()
	r.initProxy()
	r.initReflect()

	r.initErrors()

	r.global.Eval = r.newNativeFunc(r.builtin_eval, nil, "eval", nil, 1)
	r.addToGlobal("eval", r.global.Eval)

	r.initMath()
	r.initJSON()

	r.initTypedArrays()
	r.initSymbol()
	r.initWeakSet()
	r.initWeakMap()
	r.initMap()
	r.initSet()
	r.initPromise()

	r.global.thrower = r.newNativeFunc(r.builtin_thrower, nil, "", nil, 0)
	r.global.throwerProperty = &valueProperty{
		getterFunc: r.global.thrower,
		setterFunc: r.global.thrower,
		accessor:   true,
	}
	r.object_freeze(FunctionCall{Arguments: []Value{r.global.thrower}})

	funcProtoObj._put("caller", &valueProperty{
		getterFunc:   r.global.thrower,
		setterFunc:   r.global.thrower,
		accessor:     true,
		configurable: true,
	})
	funcProtoObj._put("arguments", &valueProperty{
		getterFunc:   r.global.thrower,
		setterFunc:   r.global.thrower,
		accessor:     true,
		configurable: true,
	})
}

func (r *Runtime) typeErrorResult(throw bool, args ...interface{}) {
	if throw {
		panic(r.NewTypeError(args...))
	}
}

func (r *Runtime) newError(typ *Object, format string, args ...interface{}) Value {
	msg := fmt.Sprintf(format, args...)
	return r.builtin_new(typ, []Value{newStringValue(msg)})
}

func (r *Runtime) throwReferenceError(name unistring.String) {
	panic(r.newError(r.global.ReferenceError, "%s is not defined", name))
}

func (r *Runtime) newSyntaxError(msg string, offset int) Value {
	return r.builtin_new(r.global.SyntaxError, []Value{newStringValue(msg)})
}

func newBaseObjectObj(obj, proto *Object, class string) *baseObject {
	o := &baseObject{
		class:      class,
		val:        obj,
		extensible: true,
		prototype:  proto,
	}
	obj.self = o
	o.init()
	return o
}

func newGuardedObj(proto *Object, class string) *guardedObject {
	return &guardedObject{
		baseObject: baseObject{
			class:      class,
			extensible: true,
			prototype:  proto,
		},
	}
}

func (r *Runtime) newBaseObject(proto *Object, class string) (o *baseObject) {
	v := &Object{runtime: r}
	return newBaseObjectObj(v, proto, class)
}

func (r *Runtime) newGuardedObject(proto *Object, class string) (o *guardedObject) {
	v := &Object{runtime: r}
	o = newGuardedObj(proto, class)
	v.self = o
	o.val = v
	o.init()
	return
}

func (r *Runtime) NewObject() (v *Object) {
	return r.newBaseObject(r.global.ObjectPrototype, classObject).val
}

// CreateObject creates an object with given prototype. Equivalent of Object.create(proto).
func (r *Runtime) CreateObject(proto *Object) *Object {
	return r.newBaseObject(proto, classObject).val
}

func (r *Runtime) NewArray(items ...interface{}) *Object {
	values := make([]Value, len(items))
	for i, item := range items {
		values[i] = r.ToValue(item)
	}
	return r.newArrayValues(values)
}

func (r *Runtime) NewTypeError(args ...interface{}) *Object {
	msg := ""
	if len(args) > 0 {
		f, _ := args[0].(string)
		msg = fmt.Sprintf(f, args[1:]...)
	}
	return r.builtin_new(r.global.TypeError, []Value{newStringValue(msg)})
}

func (r *Runtime) NewGoError(err error) *Object {
	e := r.newError(r.global.GoError, err.Error()).(*Object)
	e.Set("value", err)
	return e
}

func (r *Runtime) newFunc(name unistring.String, length int, strict bool) (f *funcObject) {
	v := &Object{runtime: r}

	f = &funcObject{}
	f.class = classFunction
	f.val = v
	f.extensible = true
	f.strict = strict
	v.self = f
	f.prototype = r.global.FunctionPrototype
	f.init(name, intToValue(int64(length)))
	return
}

func (r *Runtime) newMethod(name unistring.String, length int, strict bool) (f *methodFuncObject) {
	v := &Object{runtime: r}

	f = &methodFuncObject{}
	f.class = classFunction
	f.val = v
	f.extensible = true
	f.strict = strict
	v.self = f
	f.prototype = r.global.FunctionPrototype
	f.init(name, intToValue(int64(length)))
	return
}

func (r *Runtime) newArrowFunc(name unistring.String, length int, strict bool) (f *arrowFuncObject) {
	v := &Object{runtime: r}

	f = &arrowFuncObject{}
	f.class = classFunction
	f.val = v
	f.extensible = true
	f.strict = strict

	vm := r.vm
	var this Value
	if vm.sb >= 0 {
		this = vm.stack[vm.sb]
	} else {
		this = vm.r.globalObject
	}

	f.this = this
	f.newTarget = vm.newTarget
	v.self = f
	f.prototype = r.global.FunctionPrototype
	f.init(name, intToValue(int64(length)))
	return
}

func (r *Runtime) newNativeFuncObj(v *Object, call func(FunctionCall) Value, construct func(args []Value, proto *Object) *Object, name unistring.String, proto *Object, length Value) *nativeFuncObject {
	f := &nativeFuncObject{
		baseFuncObject: baseFuncObject{
			baseObject: baseObject{
				class:      classFunction,
				val:        v,
				extensible: true,
				prototype:  r.global.FunctionPrototype,
			},
		},
		f:         call,
		construct: r.wrapNativeConstruct(construct, proto),
	}
	v.self = f
	f.init(name, length)
	if proto != nil {
		f._putProp("prototype", proto, false, false, false)
	}
	return f
}

func (r *Runtime) newNativeConstructor(call func(ConstructorCall) *Object, name unistring.String, length int64) *Object {
	v := &Object{runtime: r}

	f := &nativeFuncObject{
		baseFuncObject: baseFuncObject{
			baseObject: baseObject{
				class:      classFunction,
				val:        v,
				extensible: true,
				prototype:  r.global.FunctionPrototype,
			},
		},
	}

	f.f = func(c FunctionCall) Value {
		thisObj, _ := c.This.(*Object)
		if thisObj != nil {
			res := call(ConstructorCall{
				This:      thisObj,
				Arguments: c.Arguments,
			})
			if res == nil {
				return _undefined
			}
			return res
		}
		return f.defaultConstruct(call, c.Arguments, nil)
	}

	f.construct = func(args []Value, newTarget *Object) *Object {
		return f.defaultConstruct(call, args, newTarget)
	}

	v.self = f
	f.init(name, intToValue(length))

	proto := r.NewObject()
	proto.self._putProp("constructor", v, true, false, true)
	f._putProp("prototype", proto, true, false, false)

	return v
}

func (r *Runtime) newNativeConstructOnly(v *Object, ctor func(args []Value, newTarget *Object) *Object, defaultProto *Object, name unistring.String, length int64) *nativeFuncObject {
	if v == nil {
		v = &Object{runtime: r}
	}

	f := &nativeFuncObject{
		baseFuncObject: baseFuncObject{
			baseObject: baseObject{
				class:      classFunction,
				val:        v,
				extensible: true,
				prototype:  r.global.FunctionPrototype,
			},
		},
		f: func(call FunctionCall) Value {
			return ctor(call.Arguments, nil)
		},
		construct: func(args []Value, newTarget *Object) *Object {
			if newTarget == nil {
				newTarget = v
			}
			return ctor(args, newTarget)
		},
	}
	v.self = f
	f.init(name, intToValue(length))
	if defaultProto != nil {
		f._putProp("prototype", defaultProto, false, false, false)
	}

	return f
}

func (r *Runtime) newNativeFunc(call func(FunctionCall) Value, construct func(args []Value, proto *Object) *Object, name unistring.String, proto *Object, length int) *Object {
	v := &Object{runtime: r}

	f := &nativeFuncObject{
		baseFuncObject: baseFuncObject{
			baseObject: baseObject{
				class:      classFunction,
				val:        v,
				extensible: true,
				prototype:  r.global.FunctionPrototype,
			},
		},
		f:         call,
		construct: r.wrapNativeConstruct(construct, proto),
	}
	v.self = f
	f.init(name, intToValue(int64(length)))
	if proto != nil {
		f._putProp("prototype", proto, false, false, false)
		proto.self._putProp("constructor", v, true, false, true)
	}
	return v
}

func (r *Runtime) newNativeFuncConstructObj(v *Object, construct func(args []Value, proto *Object) *Object, name unistring.String, proto *Object, length int) *nativeFuncObject {
	f := &nativeFuncObject{
		baseFuncObject: baseFuncObject{
			baseObject: baseObject{
				class:      classFunction,
				val:        v,
				extensible: true,
				prototype:  r.global.FunctionPrototype,
			},
		},
		f:         r.constructToCall(construct, proto),
		construct: r.wrapNativeConstruct(construct, proto),
	}

	f.init(name, intToValue(int64(length)))
	if proto != nil {
		f._putProp("prototype", proto, false, false, false)
	}
	return f
}

func (r *Runtime) newNativeFuncConstruct(construct func(args []Value, proto *Object) *Object, name unistring.String, prototype *Object, length int64) *Object {
	return r.newNativeFuncConstructProto(construct, name, prototype, r.global.FunctionPrototype, length)
}

func (r *Runtime) newNativeFuncConstructProto(construct func(args []Value, proto *Object) *Object, name unistring.String, prototype, proto *Object, length int64) *Object {
	v := &Object{runtime: r}

	f := &nativeFuncObject{}
	f.class = classFunction
	f.val = v
	f.extensible = true
	v.self = f
	f.prototype = proto
	f.f = r.constructToCall(construct, prototype)
	f.construct = r.wrapNativeConstruct(construct, prototype)
	f.init(name, intToValue(length))
	if prototype != nil {
		f._putProp("prototype", prototype, false, false, false)
		prototype.self._putProp("constructor", v, true, false, true)
	}
	return v
}

func (r *Runtime) newPrimitiveObject(value Value, proto *Object, class string) *Object {
	v := &Object{runtime: r}

	o := &primitiveValueObject{}
	o.class = class
	o.val = v
	o.extensible = true
	v.self = o
	o.prototype = proto
	o.pValue = value
	o.init()
	return v
}

func (r *Runtime) builtin_Number(call FunctionCall) Value {
	if len(call.Arguments) > 0 {
		return call.Arguments[0].ToNumber()
	} else {
		return valueInt(0)
	}
}

func (r *Runtime) builtin_newNumber(args []Value, proto *Object) *Object {
	var v Value
	if len(args) > 0 {
		v = args[0].ToNumber()
	} else {
		v = intToValue(0)
	}
	return r.newPrimitiveObject(v, proto, classNumber)
}

func (r *Runtime) builtin_Boolean(call FunctionCall) Value {
	if len(call.Arguments) > 0 {
		if call.Arguments[0].ToBoolean() {
			return valueTrue
		} else {
			return valueFalse
		}
	} else {
		return valueFalse
	}
}

func (r *Runtime) builtin_newBoolean(args []Value, proto *Object) *Object {
	var v Value
	if len(args) > 0 {
		if args[0].ToBoolean() {
			v = valueTrue
		} else {
			v = valueFalse
		}
	} else {
		v = valueFalse
	}
	return r.newPrimitiveObject(v, proto, classBoolean)
}

func (r *Runtime) error_toString(call FunctionCall) Value {
	var nameStr, msgStr valueString
	obj := r.toObject(call.This)
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
	if nameStr.length() == 0 {
		return msgStr
	}
	if msgStr.length() == 0 {
		return nameStr
	}
	var sb valueStringBuilder
	sb.WriteString(nameStr)
	sb.WriteString(asciiString(": "))
	sb.WriteString(msgStr)
	return sb.String()
}

func (r *Runtime) builtin_new(construct *Object, args []Value) *Object {
	return r.toConstructor(construct)(args, nil)
}

func (r *Runtime) builtin_thrower(call FunctionCall) Value {
	obj := r.toObject(call.This)
	strict := true
	switch fn := obj.self.(type) {
	case *funcObject:
		strict = fn.strict
	}
	r.typeErrorResult(strict, "'caller', 'callee', and 'arguments' properties may not be accessed on strict mode functions or the arguments objects for calls to them")
	return nil
}

func (r *Runtime) eval(srcVal valueString, direct, strict bool, this Value) Value {
	src := escapeInvalidUtf16(srcVal)
	vm := r.vm
	inGlobal := true
	if direct {
		for s := vm.stash; s != nil; s = s.outer {
			if s.variable {
				inGlobal = false
				break
			}
		}
	}
	p, err := r.compile("<eval>", src, strict, true, inGlobal)
	if err != nil {
		panic(err)
	}

	vm.pushCtx()
	vm.prg = p
	vm.pc = 0
	vm.args = 0
	vm.result = _undefined
	if !direct {
		vm.stash = &r.global.stash
	}
	vm.sb = vm.sp
	vm.push(this)
	vm.run()
	retval := vm.result
	vm.popCtx()
	vm.halt = false
	vm.sp -= 1
	return retval
}

func (r *Runtime) builtin_eval(call FunctionCall) Value {
	if len(call.Arguments) == 0 {
		return _undefined
	}
	if str, ok := call.Arguments[0].(valueString); ok {
		return r.eval(str, false, false, r.globalObject)
	}
	return call.Arguments[0]
}

func (r *Runtime) constructToCall(construct func(args []Value, proto *Object) *Object, proto *Object) func(call FunctionCall) Value {
	return func(call FunctionCall) Value {
		return construct(call.Arguments, proto)
	}
}

func (r *Runtime) wrapNativeConstruct(c func(args []Value, proto *Object) *Object, proto *Object) func(args []Value, newTarget *Object) *Object {
	if c == nil {
		return nil
	}
	return func(args []Value, newTarget *Object) *Object {
		var p *Object
		if newTarget != nil {
			if pp, ok := newTarget.self.getStr("prototype", nil).(*Object); ok {
				p = pp
			}
		}
		if p == nil {
			p = proto
		}
		return c(args, p)
	}
}

func (r *Runtime) toCallable(v Value) func(FunctionCall) Value {
	if call, ok := r.toObject(v).self.assertCallable(); ok {
		return call
	}
	r.typeErrorResult(true, "Value is not callable: %s", v.toString())
	return nil
}

func (r *Runtime) checkObjectCoercible(v Value) {
	switch v.(type) {
	case valueUndefined, valueNull:
		r.typeErrorResult(true, "Value is not object coercible")
	}
}

func toInt8(v Value) int8 {
	v = v.ToNumber()
	if i, ok := v.(valueInt); ok {
		return int8(i)
	}

	if f, ok := v.(valueFloat); ok {
		f := float64(f)
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			return int8(int64(f))
		}
	}
	return 0
}

func toUint8(v Value) uint8 {
	v = v.ToNumber()
	if i, ok := v.(valueInt); ok {
		return uint8(i)
	}

	if f, ok := v.(valueFloat); ok {
		f := float64(f)
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			return uint8(int64(f))
		}
	}
	return 0
}

func toUint8Clamp(v Value) uint8 {
	v = v.ToNumber()
	if i, ok := v.(valueInt); ok {
		if i < 0 {
			return 0
		}
		if i <= 255 {
			return uint8(i)
		}
		return 255
	}

	if num, ok := v.(valueFloat); ok {
		num := float64(num)
		if !math.IsNaN(num) {
			if num < 0 {
				return 0
			}
			if num > 255 {
				return 255
			}
			f := math.Floor(num)
			f1 := f + 0.5
			if f1 < num {
				return uint8(f + 1)
			}
			if f1 > num {
				return uint8(f)
			}
			r := uint8(f)
			if r&1 != 0 {
				return r + 1
			}
			return r
		}
	}
	return 0
}

func toInt16(v Value) int16 {
	v = v.ToNumber()
	if i, ok := v.(valueInt); ok {
		return int16(i)
	}

	if f, ok := v.(valueFloat); ok {
		f := float64(f)
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			return int16(int64(f))
		}
	}
	return 0
}

func toUint16(v Value) uint16 {
	v = v.ToNumber()
	if i, ok := v.(valueInt); ok {
		return uint16(i)
	}

	if f, ok := v.(valueFloat); ok {
		f := float64(f)
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			return uint16(int64(f))
		}
	}
	return 0
}

func toInt32(v Value) int32 {
	v = v.ToNumber()
	if i, ok := v.(valueInt); ok {
		return int32(i)
	}

	if f, ok := v.(valueFloat); ok {
		f := float64(f)
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			return int32(int64(f))
		}
	}
	return 0
}

func toUint32(v Value) uint32 {
	v = v.ToNumber()
	if i, ok := v.(valueInt); ok {
		return uint32(i)
	}

	if f, ok := v.(valueFloat); ok {
		f := float64(f)
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			return uint32(int64(f))
		}
	}
	return 0
}

func toInt64(v Value) int64 {
	v = v.ToNumber()
	if i, ok := v.(valueInt); ok {
		return int64(i)
	}

	if f, ok := v.(valueFloat); ok {
		f := float64(f)
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			return int64(f)
		}
	}
	return 0
}

func toUint64(v Value) uint64 {
	v = v.ToNumber()
	if i, ok := v.(valueInt); ok {
		return uint64(i)
	}

	if f, ok := v.(valueFloat); ok {
		f := float64(f)
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			return uint64(int64(f))
		}
	}
	return 0
}

func toInt(v Value) int {
	v = v.ToNumber()
	if i, ok := v.(valueInt); ok {
		return int(i)
	}

	if f, ok := v.(valueFloat); ok {
		f := float64(f)
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			return int(f)
		}
	}
	return 0
}

func toUint(v Value) uint {
	v = v.ToNumber()
	if i, ok := v.(valueInt); ok {
		return uint(i)
	}

	if f, ok := v.(valueFloat); ok {
		f := float64(f)
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			return uint(int64(f))
		}
	}
	return 0
}

func toFloat32(v Value) float32 {
	return float32(v.ToFloat())
}

func toLength(v Value) int64 {
	if v == nil {
		return 0
	}
	i := v.ToInteger()
	if i < 0 {
		return 0
	}
	if i >= maxInt {
		return maxInt - 1
	}
	return i
}

func (r *Runtime) toLengthUint32(v Value) uint32 {
	var intVal int64
repeat:
	switch num := v.(type) {
	case valueInt:
		intVal = int64(num)
	case valueFloat:
		if v != _negativeZero {
			if i, ok := floatToInt(float64(num)); ok {
				intVal = i
			} else {
				goto fail
			}
		}
	case valueString:
		v = num.ToNumber()
		goto repeat
	default:
		// Legacy behaviour as specified in https://tc39.es/ecma262/#sec-arraysetlength (see the note)
		n2 := toUint32(v)
		n1 := v.ToNumber()
		if f, ok := n1.(valueFloat); ok {
			f := float64(f)
			if f != 0 || !math.Signbit(f) {
				goto fail
			}
		}
		if n1.ToInteger() != int64(n2) {
			goto fail
		}
		return n2
	}
	if intVal >= 0 && intVal <= math.MaxUint32 {
		return uint32(intVal)
	}
fail:
	panic(r.newError(r.global.RangeError, "Invalid array length"))
}

func toIntStrict(i int64) int {
	if bits.UintSize == 32 {
		if i > math.MaxInt32 || i < math.MinInt32 {
			panic(rangeError("Integer value overflows 32-bit int"))
		}
	}
	return int(i)
}

func toIntClamp(i int64) int {
	if bits.UintSize == 32 {
		if i > math.MaxInt32 {
			return math.MaxInt32
		}
		if i < math.MinInt32 {
			return math.MinInt32
		}
	}
	return int(i)
}

func (r *Runtime) toIndex(v Value) int {
	num := v.ToInteger()
	if num >= 0 && num < maxInt {
		if bits.UintSize == 32 && num >= math.MaxInt32 {
			panic(r.newError(r.global.RangeError, "Index %s overflows int", v.String()))
		}
		return int(num)
	}
	panic(r.newError(r.global.RangeError, "Invalid index %s", v.String()))
}

func (r *Runtime) toBoolean(b bool) Value {
	if b {
		return valueTrue
	} else {
		return valueFalse
	}
}

// New creates an instance of a Javascript runtime that can be used to run code. Multiple instances may be created and
// used simultaneously, however it is not possible to pass JS values across runtimes.
func New() *Runtime {
	r := &Runtime{}
	r.init()
	return r
}

// Compile creates an internal representation of the JavaScript code that can be later run using the Runtime.RunProgram()
// method. This representation is not linked to a runtime in any way and can be run in multiple runtimes (possibly
// at the same time).
func Compile(name, src string, strict bool) (*Program, error) {
	return compile(name, src, strict, false, true)
}

// CompileAST creates an internal representation of the JavaScript code that can be later run using the Runtime.RunProgram()
// method. This representation is not linked to a runtime in any way and can be run in multiple runtimes (possibly
// at the same time).
func CompileAST(prg *js_ast.Program, strict bool) (*Program, error) {
	return compileAST(prg, strict, false, true)
}

// MustCompile is like Compile but panics if the code cannot be compiled.
// It simplifies safe initialization of global variables holding compiled JavaScript code.
func MustCompile(name, src string, strict bool) *Program {
	prg, err := Compile(name, src, strict)
	if err != nil {
		panic(err)
	}

	return prg
}

// Parse takes a source string and produces a parsed AST. Use this function if you want to pass options
// to the parser, e.g.:
//
//  p, err := Parse("test.js", "var a = true", parser.WithDisableSourceMaps)
//  if err != nil { /* ... */ }
//  prg, err := CompileAST(p, true)
//  // ...
//
// Otherwise use Compile which combines both steps.
func Parse(name, src string, options ...parser.Option) (prg *js_ast.Program, err error) {
	prg, err1 := parser.ParseFile(nil, name, src, 0, options...)
	if err1 != nil {
		// FIXME offset
		err = &CompilerSyntaxError{
			CompilerError: CompilerError{
				Message: err1.Error(),
			},
		}
	}
	return
}

func compile(name, src string, strict, eval, inGlobal bool, parserOptions ...parser.Option) (p *Program, err error) {
	prg, err := Parse(name, src, parserOptions...)
	if err != nil {
		return
	}

	return compileAST(prg, strict, eval, inGlobal)
}

func compileAST(prg *js_ast.Program, strict, eval, inGlobal bool) (p *Program, err error) {
	c := newCompiler()

	defer func() {
		if x := recover(); x != nil {
			p = nil
			switch x1 := x.(type) {
			case *CompilerSyntaxError:
				err = x1
			default:
				panic(x)
			}
		}
	}()

	c.compile(prg, strict, eval, inGlobal)
	p = c.p
	return
}

func (r *Runtime) compile(name, src string, strict, eval, inGlobal bool) (p *Program, err error) {
	p, err = compile(name, src, strict, eval, inGlobal, r.parserOptions...)
	if err != nil {
		switch x1 := err.(type) {
		case *CompilerSyntaxError:
			err = &Exception{
				val: r.builtin_new(r.global.SyntaxError, []Value{newStringValue(x1.Error())}),
			}
		case *CompilerReferenceError:
			err = &Exception{
				val: r.newError(r.global.ReferenceError, x1.Message),
			} // TODO proper message
		}
	}
	return
}

// RunString executes the given string in the global context.
func (r *Runtime) RunString(str string) (Value, error) {
	return r.RunScript("", str)
}

// RunScript executes the given string in the global context.
func (r *Runtime) RunScript(name, src string) (Value, error) {
	p, err := r.compile(name, src, false, false, true)

	if err != nil {
		return nil, err
	}

	return r.RunProgram(p)
}

// RunProgram executes a pre-compiled (see Compile()) code in the global context.
func (r *Runtime) RunProgram(p *Program) (result Value, err error) {
	defer func() {
		if x := recover(); x != nil {
			if ex, ok := x.(*uncatchableException); ok {
				err = ex.err
			} else {
				panic(x)
			}
		}
	}()
	vm := r.vm
	recursive := false
	if len(vm.callStack) > 0 {
		recursive = true
		vm.pushCtx()
		vm.stash = &r.global.stash
		vm.sb = vm.sp - 1
	}
	vm.prg = p
	vm.pc = 0
	vm.result = _undefined
	ex := vm.runTry()
	if ex == nil {
		result = r.vm.result
	} else {
		err = ex
	}
	if recursive {
		vm.popCtx()
		vm.halt = false
		vm.clearStack()
	} else {
		vm.stack = nil
		vm.prg = nil
		vm.funcName = ""
		r.leave()
	}
	return
}

// CaptureCallStack appends the current call stack frames to the stack slice (which may be nil) up to the specified depth.
// The most recent frame will be the first one.
// If depth <= 0 or more than the number of available frames, returns the entire stack.
// This method is not safe for concurrent use and should only be called by a Go function that is
// called from a running script.
func (r *Runtime) CaptureCallStack(depth int, stack []StackFrame) []StackFrame {
	l := len(r.vm.callStack)
	var offset int
	if depth > 0 {
		offset = l - depth + 1
		if offset < 0 {
			offset = 0
		}
	}
	if stack == nil {
		stack = make([]StackFrame, 0, l-offset+1)
	}
	return r.vm.captureStack(stack, offset)
}

// Interrupt a running JavaScript. The corresponding Go call will return an *InterruptedError containing v.
// Note, it only works while in JavaScript code, it does not interrupt native Go functions (which includes all built-ins).
// If the runtime is currently not running, it will be immediately interrupted on the next Run*() call.
// To avoid that use ClearInterrupt()
func (r *Runtime) Interrupt(v interface{}) {
	r.vm.Interrupt(v)
}

// ClearInterrupt resets the interrupt flag. Typically this needs to be called before the runtime
// is made available for re-use if there is a chance it could have been interrupted with Interrupt().
// Otherwise if Interrupt() was called when runtime was not running (e.g. if it had already finished)
// so that Interrupt() didn't actually trigger, an attempt to use the runtime will immediately cause
// an interruption. It is up to the user to ensure proper synchronisation so that ClearInterrupt() is
// only called when the runtime has finished and there is no chance of a concurrent Interrupt() call.
func (r *Runtime) ClearInterrupt() {
	r.vm.ClearInterrupt()
}

/*
ToValue converts a Go value into a JavaScript value of a most appropriate type. Structural types (such as structs, maps
and slices) are wrapped so that changes are reflected on the original value which can be retrieved using Value.Export().

WARNING! There are two very important caveats to bear in mind when modifying wrapped Go structs, maps and
slices.

1. If a slice is passed by value (not as a pointer), resizing the slice does not reflect on the original
value. Moreover, extending the slice may result in the underlying array being re-allocated and copied.
For example:

 a := []interface{}{1}
 vm.Set("a", a)
 vm.RunString(`a.push(2); a[0] = 0;`)
 fmt.Println(a[0]) // prints "1"

2. If a regular JavaScript Object is assigned as an element of a wrapped Go struct, map or array, it is
Export()'ed and therefore copied. This may result in an unexpected behaviour in JavaScript:

 m := map[string]interface{}{}
 vm.Set("m", m)
 vm.RunString(`
 var obj = {test: false};
 m.obj = obj; // obj gets Export()'ed, i.e. copied to a new map[string]interface{} and then this map is set as m["obj"]
 obj.test = true; // note, m.obj.test is still false
 `)
 fmt.Println(m["obj"].(map[string]interface{})["test"]) // prints "false"

Non-addressable structs, slices and arrays get copied (as if they were passed as a function parameter, by value).

Notes on individual types:

Primitive types

Primitive types (numbers, string, bool) are converted to the corresponding JavaScript primitives.

Strings

Because of the difference in internal string representation between ECMAScript (which uses UTF-16) and Go (which uses
UTF-8) conversion from JS to Go may be lossy. In particular, code points that can be part of UTF-16 surrogate pairs
(0xD800-0xDFFF) cannot be represented in UTF-8 unless they form a valid surrogate pair and are replaced with
utf8.RuneError.

Nil

Nil is converted to null.

Functions

func(FunctionCall) Value is treated as a native JavaScript function. This increases performance because there are no
automatic argument and return value type conversions (which involves reflect). Attempting to use
the function as a constructor will result in a TypeError.

func(FunctionCall, *Runtime) Value is treated as above, except the *Runtime is also passed as a parameter.

func(ConstructorCall) *Object is treated as a native constructor, allowing to use it with the new
operator:

 func MyObject(call goja.ConstructorCall) *goja.Object {
    // call.This contains the newly created object as per http://www.ecma-international.org/ecma-262/5.1/index.html#sec-13.2.2
    // call.Arguments contain arguments passed to the function

    call.This.Set("method", method)

    //...

    // If return value is a non-nil *Object, it will be used instead of call.This
    // This way it is possible to return a Go struct or a map converted
    // into goja.Value using ToValue(), however in this case
    // instanceof will not work as expected, unless you set the prototype:
    //
    // instance := &myCustomStruct{}
    // instanceValue := vm.ToValue(instance).(*Object)
    // instanceValue.SetPrototype(call.This.Prototype())
    // return instanceValue
    return nil
 }

 runtime.Set("MyObject", MyObject)

Then it can be used in JS as follows:

 var o = new MyObject(arg);
 var o1 = MyObject(arg); // same thing
 o instanceof MyObject && o1 instanceof MyObject; // true

When a native constructor is called directly (without the new operator) its behavior depends on
this value: if it's an Object, it is passed through, otherwise a new one is created exactly as
if it was called with the new operator. In either case call.NewTarget will be nil.

func(ConstructorCall, *Runtime) *Object is treated as above, except the *Runtime is also passed as a parameter.

Any other Go function is wrapped so that the arguments are automatically converted into the required Go types and the
return value is converted to a JavaScript value (using this method).  If conversion is not possible, a TypeError is
thrown.

Functions with multiple return values return an Array. If the last return value is an `error` it is not returned but
converted into a JS exception. If the error is *Exception, it is thrown as is, otherwise it's wrapped in a GoEerror.
Note that if there are exactly two return values and the last is an `error`, the function returns the first value as is,
not an Array.

Structs

Structs are converted to Object-like values. Fields and methods are available as properties, their values are
results of this method (ToValue()) applied to the corresponding Go value.

Field properties are writable and non-configurable. Method properties are non-writable and non-configurable.

Attempt to define a new property or delete an existing property will fail (throw in strict mode) unless it's a Symbol
property. Symbol properties only exist in the wrapper and do not affect the underlying Go value.
Note that because a wrapper is created every time a property is accessed it may lead to unexpected results such as this:

 type Field struct{
 }
 type S struct {
	Field *Field
 }
 var s = S{
	Field: &Field{},
 }
 vm := New()
 vm.Set("s", &s)
 res, err := vm.RunString(`
 var sym = Symbol(66);
 var field1 = s.Field;
 field1[sym] = true;
 var field2 = s.Field;
 field1 === field2; // true, because the equality operation compares the wrapped values, not the wrappers
 field1[sym] === true; // true
 field2[sym] === undefined; // also true
 `)

The same applies to values from maps and slices as well.

Handling of time.Time

time.Time does not get special treatment and therefore is converted just like any other `struct` providing access to
all its methods. This is done deliberately instead of converting it to a `Date` because these two types are not fully
compatible: `time.Time` includes zone, whereas JS `Date` doesn't. Doing the conversion implicitly therefore would
result in a loss of information.

If you need to convert it to a `Date`, it can be done either in JS:

 var d = new Date(goval.UnixNano()/1e6);

... or in Go:

 now := time.Now()
 vm := New()
 val, err := vm.New(vm.Get("Date").ToObject(vm), vm.ToValue(now.UnixNano()/1e6))
 if err != nil {
	...
 }
 vm.Set("d", val)

Note that Value.Export() for a `Date` value returns time.Time in local timezone.

Maps

Maps with string or integer key type are converted into host objects that largely behave like a JavaScript Object.

Maps with methods

If a map type has at least one method defined, the properties of the resulting Object represent methods, not map keys.
This is because in JavaScript there is no distinction between 'object.key` and `object[key]`, unlike Go.
If access to the map values is required, it can be achieved by defining another method or, if it's not possible, by
defining an external getter function.

Slices

Slices are converted into host objects that behave largely like JavaScript Array. It has the appropriate
prototype and all the usual methods should work. There is, however, a caveat: converted Arrays may not contain holes
(because Go slices cannot). This means that hasOwnProperty(n) always returns `true` if n < length. Deleting an item with
an index < length will set it to a zero value (but the property will remain). Nil slice elements are be converted to
`null`. Accessing an element beyond `length` returns `undefined`. Also see the warning above about passing slices as
values (as opposed to pointers).

Arrays

Arrays are converted similarly to slices, except the resulting Arrays are not resizable (and therefore the 'length'
property is non-writable).

Any other type is converted to a generic reflect based host object. Depending on the underlying type it behaves similar
to a Number, String, Boolean or Object.

Note that the underlying type is not lost, calling Export() returns the original Go value. This applies to all
reflect based types.
*/
func (r *Runtime) ToValue(i interface{}) Value {
	return r.toValue(i, reflect.Value{})
}

func (r *Runtime) toValue(i interface{}, origValue reflect.Value) Value {
	switch i := i.(type) {
	case nil:
		return _null
	case *Object:
		if i == nil || i.runtime == nil {
			return _null
		}
		if i.runtime != r {
			panic(r.NewTypeError("Illegal runtime transition of an Object"))
		}
		return i
	case valueContainer:
		return i.toValue(r)
	case Value:
		return i
	case string:
		return newStringValue(i)
	case bool:
		if i {
			return valueTrue
		} else {
			return valueFalse
		}
	case func(FunctionCall) Value:
		name := unistring.NewFromString(runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name())
		return r.newNativeFunc(i, nil, name, nil, 0)
	case func(FunctionCall, *Runtime) Value:
		name := unistring.NewFromString(runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name())
		return r.newNativeFunc(func(call FunctionCall) Value {
			return i(call, r)
		}, nil, name, nil, 0)
	case func(ConstructorCall) *Object:
		name := unistring.NewFromString(runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name())
		return r.newNativeConstructor(i, name, 0)
	case func(ConstructorCall, *Runtime) *Object:
		name := unistring.NewFromString(runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name())
		return r.newNativeConstructor(func(call ConstructorCall) *Object {
			return i(call, r)
		}, name, 0)
	case int:
		return intToValue(int64(i))
	case int8:
		return intToValue(int64(i))
	case int16:
		return intToValue(int64(i))
	case int32:
		return intToValue(int64(i))
	case int64:
		return intToValue(i)
	case uint:
		if uint64(i) <= math.MaxInt64 {
			return intToValue(int64(i))
		} else {
			return floatToValue(float64(i))
		}
	case uint8:
		return intToValue(int64(i))
	case uint16:
		return intToValue(int64(i))
	case uint32:
		return intToValue(int64(i))
	case uint64:
		if i <= math.MaxInt64 {
			return intToValue(int64(i))
		}
		return floatToValue(float64(i))
	case float32:
		return floatToValue(float64(i))
	case float64:
		return floatToValue(i)
	case map[string]interface{}:
		if i == nil {
			return _null
		}
		obj := &Object{runtime: r}
		m := &objectGoMapSimple{
			baseObject: baseObject{
				val:        obj,
				extensible: true,
			},
			data: i,
		}
		obj.self = m
		m.init()
		return obj
	case []interface{}:
		if i == nil {
			return _null
		}
		obj := &Object{runtime: r}
		a := &objectGoSlice{
			baseObject: baseObject{
				val: obj,
			},
			data: &i,
		}
		obj.self = a
		a.init()
		return obj
	case *[]interface{}:
		if i == nil {
			return _null
		}
		obj := &Object{runtime: r}
		a := &objectGoSlice{
			baseObject: baseObject{
				val: obj,
			},
			data: i,
		}
		obj.self = a
		a.init()
		return obj
	}

	if !origValue.IsValid() {
		origValue = reflect.ValueOf(i)
	} else {
		// If origValue was a result of an Index(), or Field(), or such, its Kind may be Interface:
		// 	a := []interface{}{(*S)(nil)}
		//	a0 := reflect.ValueOf(a).Index(0) // a0.Kind() is reflect.Interface
		//	a1 := reflect.ValueOf(a[0]) // a1.Kind() is reflect.Ptr
		// Need to "dereference" it to make it consistent with plain value being passed.
		for origValue.Kind() == reflect.Interface {
			origValue = origValue.Elem()
		}
	}
	value := origValue
	for value.Kind() == reflect.Ptr {
		value = reflect.Indirect(value)
	}

	if !value.IsValid() {
		return _null
	}

	switch value.Kind() {
	case reflect.Map:
		if value.Type().NumMethod() == 0 {
			switch value.Type().Key().Kind() {
			case reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				reflect.Float64, reflect.Float32:

				obj := &Object{runtime: r}
				m := &objectGoMapReflect{
					objectGoReflect: objectGoReflect{
						baseObject: baseObject{
							val:        obj,
							extensible: true,
						},
						origValue: origValue,
						value:     value,
					},
				}
				m.init()
				obj.self = m
				return obj
			}
		}
	case reflect.Array:
		obj := &Object{runtime: r}
		a := &objectGoArrayReflect{
			objectGoReflect: objectGoReflect{
				baseObject: baseObject{
					val: obj,
				},
				origValue: origValue,
				value:     value,
			},
		}
		a.init()
		obj.self = a
		return obj
	case reflect.Slice:
		obj := &Object{runtime: r}
		a := &objectGoSliceReflect{
			objectGoArrayReflect: objectGoArrayReflect{
				objectGoReflect: objectGoReflect{
					baseObject: baseObject{
						val: obj,
					},
					origValue: origValue,
					value:     value,
				},
			},
		}
		a.init()
		obj.self = a
		return obj
	case reflect.Func:
		name := unistring.NewFromString(runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name())
		return r.newNativeFunc(r.wrapReflectFunc(value), nil, name, nil, value.Type().NumIn())
	}

	obj := &Object{runtime: r}
	o := &objectGoReflect{
		baseObject: baseObject{
			val: obj,
		},
		origValue: origValue,
		value:     value,
	}
	obj.self = o
	o.init()
	return obj
}

func (r *Runtime) wrapReflectFunc(value reflect.Value) func(FunctionCall) Value {
	return func(call FunctionCall) Value {
		typ := value.Type()
		nargs := typ.NumIn()
		var in []reflect.Value

		if l := len(call.Arguments); l < nargs {
			// fill missing arguments with zero values
			n := nargs
			if typ.IsVariadic() {
				n--
			}
			in = make([]reflect.Value, n)
			for i := l; i < n; i++ {
				in[i] = reflect.Zero(typ.In(i))
			}
		} else {
			if l > nargs && !typ.IsVariadic() {
				l = nargs
			}
			in = make([]reflect.Value, l)
		}

		for i, a := range call.Arguments {
			var t reflect.Type

			n := i
			if n >= nargs-1 && typ.IsVariadic() {
				if n > nargs-1 {
					n = nargs - 1
				}

				t = typ.In(n).Elem()
			} else if n > nargs-1 { // ignore extra arguments
				break
			} else {
				t = typ.In(n)
			}

			v := reflect.New(t).Elem()
			err := r.toReflectValue(a, v, &objectExportCtx{})
			if err != nil {
				panic(r.NewTypeError("could not convert function call parameter %d: %v", i, err))
			}
			in[i] = v
		}

		out := value.Call(in)
		if len(out) == 0 {
			return _undefined
		}

		if last := out[len(out)-1]; last.Type().Name() == "error" {
			if !last.IsNil() {
				err := last.Interface()
				if _, ok := err.(*Exception); ok {
					panic(err)
				}
				panic(r.NewGoError(last.Interface().(error)))
			}
			out = out[:len(out)-1]
		}

		switch len(out) {
		case 0:
			return _undefined
		case 1:
			return r.ToValue(out[0].Interface())
		default:
			s := make([]interface{}, len(out))
			for i, v := range out {
				s[i] = v.Interface()
			}

			return r.ToValue(s)
		}
	}
}

func (r *Runtime) toReflectValue(v Value, dst reflect.Value, ctx *objectExportCtx) error {
	typ := dst.Type()

	if typ == typeValue {
		dst.Set(reflect.ValueOf(v))
		return nil
	}

	if typ == typeObject {
		if obj, ok := v.(*Object); ok {
			dst.Set(reflect.ValueOf(obj))
			return nil
		}
	}

	if typ == typeCallable {
		if fn, ok := AssertFunction(v); ok {
			dst.Set(reflect.ValueOf(fn))
			return nil
		}
	}

	et := v.ExportType()
	if et == nil || et == reflectTypeNil {
		dst.Set(reflect.Zero(typ))
		return nil
	}

	kind := typ.Kind()
	for i := 0; ; i++ {
		if et.AssignableTo(typ) {
			ev := reflect.ValueOf(exportValue(v, ctx))
			for ; i > 0; i-- {
				ev = ev.Elem()
			}
			dst.Set(ev)
			return nil
		}
		expKind := et.Kind()
		if expKind == kind && et.ConvertibleTo(typ) || expKind == reflect.String && typ == typeBytes {
			ev := reflect.ValueOf(exportValue(v, ctx))
			for ; i > 0; i-- {
				ev = ev.Elem()
			}
			dst.Set(ev.Convert(typ))
			return nil
		}
		if expKind == reflect.Ptr {
			et = et.Elem()
		} else {
			break
		}
	}

	if typ == typeTime {
		if obj, ok := v.(*Object); ok {
			if d, ok := obj.self.(*dateObject); ok {
				dst.Set(reflect.ValueOf(d.time()))
				return nil
			}
		}
		if et.Kind() == reflect.String {
			tme, ok := dateParse(v.String())
			if !ok {
				return fmt.Errorf("could not convert string %v to %v", v, typ)
			}
			dst.Set(reflect.ValueOf(tme))
			return nil
		}
	}

	switch kind {
	case reflect.String:
		dst.Set(reflect.ValueOf(v.String()).Convert(typ))
		return nil
	case reflect.Bool:
		dst.Set(reflect.ValueOf(v.ToBoolean()).Convert(typ))
		return nil
	case reflect.Int:
		dst.Set(reflect.ValueOf(toInt(v)).Convert(typ))
		return nil
	case reflect.Int64:
		dst.Set(reflect.ValueOf(toInt64(v)).Convert(typ))
		return nil
	case reflect.Int32:
		dst.Set(reflect.ValueOf(toInt32(v)).Convert(typ))
		return nil
	case reflect.Int16:
		dst.Set(reflect.ValueOf(toInt16(v)).Convert(typ))
		return nil
	case reflect.Int8:
		dst.Set(reflect.ValueOf(toInt8(v)).Convert(typ))
		return nil
	case reflect.Uint:
		dst.Set(reflect.ValueOf(toUint(v)).Convert(typ))
		return nil
	case reflect.Uint64:
		dst.Set(reflect.ValueOf(toUint64(v)).Convert(typ))
		return nil
	case reflect.Uint32:
		dst.Set(reflect.ValueOf(toUint32(v)).Convert(typ))
		return nil
	case reflect.Uint16:
		dst.Set(reflect.ValueOf(toUint16(v)).Convert(typ))
		return nil
	case reflect.Uint8:
		dst.Set(reflect.ValueOf(toUint8(v)).Convert(typ))
		return nil
	case reflect.Float64:
		dst.Set(reflect.ValueOf(v.ToFloat()).Convert(typ))
		return nil
	case reflect.Float32:
		dst.Set(reflect.ValueOf(toFloat32(v)).Convert(typ))
		return nil
	case reflect.Slice, reflect.Array:
		if o, ok := v.(*Object); ok {
			if v, exists := ctx.getTyped(o, typ); exists {
				dst.Set(reflect.ValueOf(v))
				return nil
			}
			return o.self.exportToArrayOrSlice(dst, typ, ctx)
		}
	case reflect.Map:
		if o, ok := v.(*Object); ok {
			if v, exists := ctx.getTyped(o, typ); exists {
				dst.Set(reflect.ValueOf(v))
				return nil
			}
			return o.self.exportToMap(dst, typ, ctx)
		}
	case reflect.Struct:
		if o, ok := v.(*Object); ok {
			t := reflect.PtrTo(typ)
			if v, exists := ctx.getTyped(o, t); exists {
				dst.Set(reflect.ValueOf(v).Elem())
				return nil
			}
			s := dst
			ctx.putTyped(o, t, s.Addr().Interface())
			for i := 0; i < typ.NumField(); i++ {
				field := typ.Field(i)
				if ast.IsExported(field.Name) {
					name := field.Name
					if r.fieldNameMapper != nil {
						name = r.fieldNameMapper.FieldName(typ, field)
					}
					var v Value
					if field.Anonymous {
						v = o
					} else {
						v = o.self.getStr(unistring.NewFromString(name), nil)
					}

					if v != nil {
						err := r.toReflectValue(v, s.Field(i), ctx)
						if err != nil {
							return fmt.Errorf("could not convert struct value %v to %v for field %s: %w", v, field.Type, field.Name, err)
						}
					}
				}
			}
			return nil
		}
	case reflect.Func:
		if fn, ok := AssertFunction(v); ok {
			dst.Set(reflect.MakeFunc(typ, r.wrapJSFunc(fn, typ)))
			return nil
		}
	case reflect.Ptr:
		if o, ok := v.(*Object); ok {
			if v, exists := ctx.getTyped(o, typ); exists {
				dst.Set(reflect.ValueOf(v))
				return nil
			}
		}
		if dst.IsNil() {
			dst.Set(reflect.New(typ.Elem()))
		}
		return r.toReflectValue(v, dst.Elem(), ctx)
	}

	return fmt.Errorf("could not convert %v to %v", v, typ)
}

func (r *Runtime) wrapJSFunc(fn Callable, typ reflect.Type) func(args []reflect.Value) (results []reflect.Value) {
	return func(args []reflect.Value) (results []reflect.Value) {
		jsArgs := make([]Value, len(args))
		for i, arg := range args {
			jsArgs[i] = r.ToValue(arg.Interface())
		}

		results = make([]reflect.Value, typ.NumOut())
		res, err := fn(_undefined, jsArgs...)
		if err == nil {
			if typ.NumOut() > 0 {
				v := reflect.New(typ.Out(0)).Elem()
				err = r.toReflectValue(res, v, &objectExportCtx{})
				if err == nil {
					results[0] = v
				}
			}
		}

		if err != nil {
			if typ.NumOut() == 2 && typ.Out(1).Name() == "error" {
				results[1] = reflect.ValueOf(err).Convert(typ.Out(1))
			} else {
				panic(err)
			}
		}

		for i, v := range results {
			if !v.IsValid() {
				results[i] = reflect.Zero(typ.Out(i))
			}
		}

		return
	}
}

// ExportTo converts a JavaScript value into the specified Go value. The second parameter must be a non-nil pointer.
// Returns error if conversion is not possible.
//
// Notes on specific cases:
//
// Empty interface
//
// Exporting to an interface{} results in a value of the same type as Value.Export() would produce.
//
// Numeric types
//
// Exporting to numeric types uses the standard ECMAScript conversion operations, same as used when assigning
// values to non-clamped typed array items, e.g. https://262.ecma-international.org/#sec-toint32.
//
// Functions
//
// Exporting to a 'func' creates a strictly typed 'gateway' into an ES function which can be called from Go.
// The arguments are converted into ES values using Runtime.ToValue(). If the func has no return values,
// the return value is ignored. If the func has exactly one return value, it is converted to the appropriate
// type using ExportTo(). If the func has exactly 2 return values and the second value is 'error', exceptions
// are caught and returned as *Exception. In all other cases exceptions result in a panic. Any extra return values
// are zeroed.
//
// Note, if you want to catch and return exceptions as an `error` and you don't need the return value,
// 'func(...) error' will not work as expected. The 'error' in this case is mapped to the function return value, not
// the exception which will still result in a panic. Use 'func(...) (Value, error)' instead, and ignore the Value.
//
// 'this' value will always be set to 'undefined'.
//
// For a more low-level mechanism see AssertFunction().
//
// Map types
//
// An ES Map can be exported into a Go map type. If any exported key value is non-hashable, the operation panics
// (as reflect.Value.SetMapIndex() would). Symbol.iterator is ignored.
//
// Exporting an ES Set into a map type results in the map being populated with (element) -> (zero value) key/value
// pairs. If any value is non-hashable, the operation panics (as reflect.Value.SetMapIndex() would).
// Symbol.iterator is ignored.
//
// Any other Object populates the map with own enumerable non-symbol properties.
//
// Slice types
//
// Exporting an ES Set into a slice type results in its elements being exported.
//
// Exporting any Object that implements the iterable protocol (https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Iteration_protocols#the_iterable_protocol)
// into a slice type results in the slice being populated with the results of the iteration.
//
// Array is treated as iterable (i.e. overwriting Symbol.iterator affects the result).
//
// If an object has a 'length' property and is not a function it is treated as array-like. The resulting slice
// will contain obj[0], ... obj[length-1].
//
// For any other Object an error is returned.
//
// Array types
//
// Anything that can be exported to a slice type can also be exported to an array type, as long as the lengths
// match. If they do not, an error is returned.
//
// Proxy
//
// Proxy objects are treated the same way as if they were accessed from ES code in regard to their properties
// (such as 'length' or [Symbol.iterator]). This means exporting them to slice types works, however
// exporting a proxied Map into a map type does not produce its contents, because the Proxy is not recognised
// as a Map. Same applies to a proxied Set.
//
func (r *Runtime) ExportTo(v Value, target interface{}) error {
	tval := reflect.ValueOf(target)
	if tval.Kind() != reflect.Ptr || tval.IsNil() {
		return errors.New("target must be a non-nil pointer")
	}
	return r.toReflectValue(v, tval.Elem(), &objectExportCtx{})
}

// GlobalObject returns the global object.
func (r *Runtime) GlobalObject() *Object {
	return r.globalObject
}

// Set the specified variable in the global context.
// Equivalent to running "name = value" in non-strict mode.
// The value is first converted using ToValue().
// Note, this is not the same as GlobalObject().Set(name, value),
// because if a global lexical binding (let or const) exists, it is set instead.
func (r *Runtime) Set(name string, value interface{}) error {
	return r.try(func() {
		name := unistring.NewFromString(name)
		v := r.ToValue(value)
		if ref := r.global.stash.getRefByName(name, false); ref != nil {
			ref.set(v)
		} else {
			r.globalObject.self.setOwnStr(name, v, true)
		}
	})
}

// Get the specified variable in the global context.
// Equivalent to dereferencing a variable by name in non-strict mode. If variable is not defined returns nil.
// Note, this is not the same as GlobalObject().Get(name),
// because if a global lexical binding (let or const) exists, it is used instead.
// This method will panic with an *Exception if a JavaScript exception is thrown in the process.
func (r *Runtime) Get(name string) (ret Value) {
	r.tryPanic(func() {
		n := unistring.NewFromString(name)
		if v, exists := r.global.stash.getByName(n); exists {
			ret = v
		} else {
			ret = r.globalObject.self.getStr(n, nil)
		}
	})
	return
}

// SetRandSource sets random source for this Runtime. If not called, the default math/rand is used.
func (r *Runtime) SetRandSource(source RandSource) {
	r.rand = source
}

// SetTimeSource sets the current time source for this Runtime.
// If not called, the default time.Now() is used.
func (r *Runtime) SetTimeSource(now Now) {
	r.now = now
}

// SetParserOptions sets parser options to be used by RunString, RunScript and eval() within the code.
func (r *Runtime) SetParserOptions(opts ...parser.Option) {
	r.parserOptions = opts
}

// SetMaxCallStackSize sets the maximum function call depth. When exceeded, a *StackOverflowError is thrown and
// returned by RunProgram or by a Callable call. This is useful to prevent memory exhaustion caused by an
// infinite recursion. The default value is math.MaxInt32.
// This method (as the rest of the Set* methods) is not safe for concurrent use and may only be called
// from the vm goroutine or when the vm is not running.
func (r *Runtime) SetMaxCallStackSize(size int) {
	r.vm.maxCallStackSize = size
}

// New is an equivalent of the 'new' operator allowing to call it directly from Go.
func (r *Runtime) New(construct Value, args ...Value) (o *Object, err error) {
	err = r.try(func() {
		o = r.builtin_new(r.toObject(construct), args)
	})
	return
}

// Callable represents a JavaScript function that can be called from Go.
type Callable func(this Value, args ...Value) (Value, error)

// AssertFunction checks if the Value is a function and returns a Callable.
func AssertFunction(v Value) (Callable, bool) {
	if obj, ok := v.(*Object); ok {
		if f, ok := obj.self.assertCallable(); ok {
			return func(this Value, args ...Value) (ret Value, err error) {
				defer func() {
					if x := recover(); x != nil {
						if ex, ok := x.(*uncatchableException); ok {
							err = ex.err
						} else {
							panic(x)
						}
					}
				}()
				ex := obj.runtime.vm.try(func() {
					ret = f(FunctionCall{
						This:      this,
						Arguments: args,
					})
				})
				if ex != nil {
					err = ex
				}
				vm := obj.runtime.vm
				vm.clearStack()
				if len(vm.callStack) == 0 {
					obj.runtime.leave()
				}
				return
			}, true
		}
	}
	return nil, false
}

// IsUndefined returns true if the supplied Value is undefined. Note, it checks against the real undefined, not
// against the global object's 'undefined' property.
func IsUndefined(v Value) bool {
	return v == _undefined
}

// IsNull returns true if the supplied Value is null.
func IsNull(v Value) bool {
	return v == _null
}

// IsNaN returns true if the supplied value is NaN.
func IsNaN(v Value) bool {
	f, ok := v.(valueFloat)
	return ok && math.IsNaN(float64(f))
}

// IsInfinity returns true if the supplied is (+/-)Infinity
func IsInfinity(v Value) bool {
	return v == _positiveInf || v == _negativeInf
}

// Undefined returns JS undefined value. Note if global 'undefined' property is changed this still returns the original value.
func Undefined() Value {
	return _undefined
}

// Null returns JS null value.
func Null() Value {
	return _null
}

// NaN returns a JS NaN value.
func NaN() Value {
	return _NaN
}

// PositiveInf returns a JS +Inf value.
func PositiveInf() Value {
	return _positiveInf
}

// NegativeInf returns a JS -Inf value.
func NegativeInf() Value {
	return _negativeInf
}

func tryFunc(f func()) (ret interface{}) {
	defer func() {
		ret = recover()
	}()

	f()
	return
}

func (r *Runtime) try(f func()) error {
	if ex := r.vm.try(f); ex != nil {
		return ex
	}
	return nil
}

func (r *Runtime) tryPanic(f func()) {
	if ex := r.vm.try(f); ex != nil {
		panic(ex)
	}
}

func (r *Runtime) toObject(v Value, args ...interface{}) *Object {
	if obj, ok := v.(*Object); ok {
		return obj
	}
	if len(args) > 0 {
		panic(r.NewTypeError(args...))
	} else {
		var s string
		if v == nil {
			s = "undefined"
		} else {
			s = v.String()
		}
		panic(r.NewTypeError("Value is not an object: %s", s))
	}
}

func (r *Runtime) toNumber(v Value) Value {
	switch o := v.(type) {
	case valueInt, valueFloat:
		return v
	case *Object:
		if pvo, ok := o.self.(*primitiveValueObject); ok {
			return r.toNumber(pvo.pValue)
		}
	}
	panic(r.NewTypeError("Value is not a number: %s", v))
}

func (r *Runtime) speciesConstructor(o, defaultConstructor *Object) func(args []Value, newTarget *Object) *Object {
	c := o.self.getStr("constructor", nil)
	if c != nil && c != _undefined {
		c = r.toObject(c).self.getSym(SymSpecies, nil)
	}
	if c == nil || c == _undefined || c == _null {
		c = defaultConstructor
	}
	return r.toConstructor(c)
}

func (r *Runtime) speciesConstructorObj(o, defaultConstructor *Object) *Object {
	c := o.self.getStr("constructor", nil)
	if c != nil && c != _undefined {
		c = r.toObject(c).self.getSym(SymSpecies, nil)
	}
	if c == nil || c == _undefined || c == _null {
		return defaultConstructor
	}
	obj := r.toObject(c)
	if obj.self.assertConstructor() == nil {
		panic(r.NewTypeError("Value is not a constructor"))
	}
	return obj
}

func (r *Runtime) returnThis(call FunctionCall) Value {
	return call.This
}

func createDataProperty(o *Object, p Value, v Value) {
	o.defineOwnProperty(p, PropertyDescriptor{
		Writable:     FLAG_TRUE,
		Enumerable:   FLAG_TRUE,
		Configurable: FLAG_TRUE,
		Value:        v,
	}, false)
}

func createDataPropertyOrThrow(o *Object, p Value, v Value) {
	o.defineOwnProperty(p, PropertyDescriptor{
		Writable:     FLAG_TRUE,
		Enumerable:   FLAG_TRUE,
		Configurable: FLAG_TRUE,
		Value:        v,
	}, true)
}

func toPropertyKey(key Value) Value {
	return key.ToString()
}

func (r *Runtime) getVStr(v Value, p unistring.String) Value {
	o := v.ToObject(r)
	return o.self.getStr(p, v)
}

func (r *Runtime) getV(v Value, p Value) Value {
	o := v.ToObject(r)
	return o.get(p, v)
}

type iteratorRecord struct {
	iterator *Object
	next     func(FunctionCall) Value
}

func (r *Runtime) getIterator(obj Value, method func(FunctionCall) Value) *iteratorRecord {
	if method == nil {
		method = toMethod(r.getV(obj, SymIterator))
		if method == nil {
			panic(r.NewTypeError("object is not iterable"))
		}
	}

	iter := r.toObject(method(FunctionCall{
		This: obj,
	}))
	next := toMethod(iter.self.getStr("next", nil))
	return &iteratorRecord{
		iterator: iter,
		next:     next,
	}
}

func (ir *iteratorRecord) iterate(step func(Value)) {
	r := ir.iterator.runtime
	for {
		res := r.toObject(ir.next(FunctionCall{This: ir.iterator}))
		if nilSafe(res.self.getStr("done", nil)).ToBoolean() {
			break
		}
		value := nilSafe(res.self.getStr("value", nil))
		ret := tryFunc(func() {
			step(value)
		})
		if ret != nil {
			_ = tryFunc(func() {
				ir.returnIter()
			})
			panic(ret)
		}
	}
}

func (ir *iteratorRecord) step() (value Value, ex *Exception) {
	r := ir.iterator.runtime
	ex = r.vm.try(func() {
		res := r.toObject(ir.next(FunctionCall{This: ir.iterator}))
		done := nilSafe(res.self.getStr("done", nil)).ToBoolean()
		if !done {
			value = nilSafe(res.self.getStr("value", nil))
		} else {
			ir.close()
		}
	})
	return
}

func (ir *iteratorRecord) returnIter() {
	if ir.iterator == nil {
		return
	}
	retMethod := toMethod(ir.iterator.self.getStr("return", nil))
	if retMethod != nil {
		ir.iterator.runtime.toObject(retMethod(FunctionCall{This: ir.iterator}))
	}
	ir.iterator = nil
	ir.next = nil
}

func (ir *iteratorRecord) close() {
	ir.iterator = nil
	ir.next = nil
}

func (r *Runtime) createIterResultObject(value Value, done bool) Value {
	o := r.NewObject()
	o.self.setOwnStr("value", value, false)
	o.self.setOwnStr("done", r.toBoolean(done), false)
	return o
}

func (r *Runtime) newLazyObject(create func(*Object) objectImpl) *Object {
	val := &Object{runtime: r}
	o := &lazyObject{
		val:    val,
		create: create,
	}
	val.self = o
	return val
}

func (r *Runtime) getHash() *maphash.Hash {
	if r.hash == nil {
		r.hash = &maphash.Hash{}
	}
	return r.hash
}

// called when the top level function returns (i.e. control is passed outside the Runtime).
func (r *Runtime) leave() {
	for {
		jobs := r.jobQueue
		r.jobQueue = nil
		if len(jobs) == 0 {
			break
		}
		for _, job := range jobs {
			job()
		}
	}
}

func nilSafe(v Value) Value {
	if v != nil {
		return v
	}
	return _undefined
}

func isArray(object *Object) bool {
	self := object.self
	if proxy, ok := self.(*proxyObject); ok {
		if proxy.target == nil {
			panic(typeError("Cannot perform 'IsArray' on a proxy that has been revoked"))
		}
		return isArray(proxy.target)
	}
	switch self.className() {
	case classArray:
		return true
	default:
		return false
	}
}

func isRegexp(v Value) bool {
	if o, ok := v.(*Object); ok {
		matcher := o.self.getSym(SymMatch, nil)
		if matcher != nil && matcher != _undefined {
			return matcher.ToBoolean()
		}
		_, reg := o.self.(*regexpObject)
		return reg
	}
	return false
}

func limitCallArgs(call FunctionCall, n int) FunctionCall {
	if len(call.Arguments) > n {
		return FunctionCall{This: call.This, Arguments: call.Arguments[:n]}
	} else {
		return call
	}
}

func shrinkCap(newSize, oldCap int) int {
	if oldCap > 8 {
		if cap := oldCap / 2; cap >= newSize {
			return cap
		}
	}
	return oldCap
}

func growCap(newSize, oldSize, oldCap int) int {
	// Use the same algorithm as in runtime.growSlice
	doublecap := oldCap + oldCap
	if newSize > doublecap {
		return newSize
	} else {
		if oldSize < 1024 {
			return doublecap
		} else {
			cap := oldCap
			// Check 0 < cap to detect overflow
			// and prevent an infinite loop.
			for 0 < cap && cap < newSize {
				cap += cap / 4
			}
			// Return the requested cap when
			// the calculation overflowed.
			if cap <= 0 {
				return newSize
			}
			return cap
		}
	}
}

func (r *Runtime) genId() (ret uint64) {
	if r.hash == nil {
		h := r.getHash()
		r.idSeq = h.Sum64()
	}
	if r.idSeq == 0 {
		r.idSeq = 1
	}
	ret = r.idSeq
	r.idSeq++
	return
}

func (r *Runtime) setGlobal(name unistring.String, v Value, strict bool) {
	if ref := r.global.stash.getRefByName(name, strict); ref != nil {
		ref.set(v)
	} else {
		o := r.globalObject.self
		if strict {
			if o.hasOwnPropertyStr(name) {
				o.setOwnStr(name, v, true)
			} else {
				r.throwReferenceError(name)
			}
		} else {
			o.setOwnStr(name, v, false)
		}
	}
}

func (r *Runtime) trackPromiseRejection(p *Promise, operation PromiseRejectionOperation) {
	if r.promiseRejectionTracker != nil {
		r.promiseRejectionTracker(p, operation)
	}
}

func (r *Runtime) callJobCallback(job *jobCallback, this Value, args ...Value) Value {
	return job.callback(FunctionCall{This: this, Arguments: args})
}

func (r *Runtime) invoke(v Value, p unistring.String, args ...Value) Value {
	o := v.ToObject(r)
	return r.toCallable(o.self.getStr(p, nil))(FunctionCall{This: v, Arguments: args})
}

func (r *Runtime) iterableToList(iterable Value, method func(FunctionCall) Value) []Value {
	iter := r.getIterator(iterable, method)
	var values []Value
	iter.iterate(func(item Value) {
		values = append(values, item)
	})
	return values
}

func (r *Runtime) putSpeciesReturnThis(o objectImpl) {
	o._putSym(SymSpecies, &valueProperty{
		getterFunc:   r.newNativeFunc(r.returnThis, nil, "get [Symbol.species]", nil, 0),
		accessor:     true,
		configurable: true,
	})
}

func strToArrayIdx(s unistring.String) uint32 {
	if s == "" {
		return math.MaxUint32
	}
	l := len(s)
	if s[0] == '0' {
		if l == 1 {
			return 0
		}
		return math.MaxUint32
	}
	var n uint32
	if l < 10 {
		// guaranteed not to overflow
		for i := 0; i < len(s); i++ {
			c := s[i]
			if c < '0' || c > '9' {
				return math.MaxUint32
			}
			n = n*10 + uint32(c-'0')
		}
		return n
	}
	if l > 10 {
		// guaranteed to overflow
		return math.MaxUint32
	}
	c9 := s[9]
	if c9 < '0' || c9 > '9' {
		return math.MaxUint32
	}
	for i := 0; i < 9; i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return math.MaxUint32
		}
		n = n*10 + uint32(c-'0')
	}
	if n >= math.MaxUint32/10+1 {
		return math.MaxUint32
	}
	n *= 10
	n1 := n + uint32(c9-'0')
	if n1 < n {
		return math.MaxUint32
	}

	return n1
}

func strToInt32(s unistring.String) (int32, bool) {
	if s == "" {
		return -1, false
	}
	neg := s[0] == '-'
	if neg {
		s = s[1:]
	}
	l := len(s)
	if s[0] == '0' {
		if l == 1 {
			return 0, !neg
		}
		return -1, false
	}
	var n uint32
	if l < 10 {
		// guaranteed not to overflow
		for i := 0; i < len(s); i++ {
			c := s[i]
			if c < '0' || c > '9' {
				return -1, false
			}
			n = n*10 + uint32(c-'0')
		}
	} else if l > 10 {
		// guaranteed to overflow
		return -1, false
	} else {
		c9 := s[9]
		if c9 >= '0' {
			if !neg && c9 > '7' || c9 > '8' {
				// guaranteed to overflow
				return -1, false
			}
			for i := 0; i < 9; i++ {
				c := s[i]
				if c < '0' || c > '9' {
					return -1, false
				}
				n = n*10 + uint32(c-'0')
			}
			if n >= math.MaxInt32/10+1 {
				// valid number, but it overflows integer
				return 0, false
			}
			n = n*10 + uint32(c9-'0')
		} else {
			return -1, false
		}
	}
	if neg {
		return int32(-n), true
	}
	return int32(n), true
}

func strToInt64(s unistring.String) (int64, bool) {
	if s == "" {
		return -1, false
	}
	neg := s[0] == '-'
	if neg {
		s = s[1:]
	}
	l := len(s)
	if s[0] == '0' {
		if l == 1 {
			return 0, !neg
		}
		return -1, false
	}
	var n uint64
	if l < 19 {
		// guaranteed not to overflow
		for i := 0; i < len(s); i++ {
			c := s[i]
			if c < '0' || c > '9' {
				return -1, false
			}
			n = n*10 + uint64(c-'0')
		}
	} else if l > 19 {
		// guaranteed to overflow
		return -1, false
	} else {
		c18 := s[18]
		if c18 >= '0' {
			if !neg && c18 > '7' || c18 > '8' {
				// guaranteed to overflow
				return -1, false
			}
			for i := 0; i < 18; i++ {
				c := s[i]
				if c < '0' || c > '9' {
					return -1, false
				}
				n = n*10 + uint64(c-'0')
			}
			if n >= math.MaxInt64/10+1 {
				// valid number, but it overflows integer
				return 0, false
			}
			n = n*10 + uint64(c18-'0')
		} else {
			return -1, false
		}
	}
	if neg {
		return int64(-n), true
	}
	return int64(n), true
}

func strToInt(s unistring.String) (int, bool) {
	if bits.UintSize == 32 {
		n, ok := strToInt32(s)
		return int(n), ok
	}
	n, ok := strToInt64(s)
	return int(n), ok
}

// Attempts to convert a string into a canonical integer.
// On success returns (number, true).
// If it was a canonical number, but not an integer returns (0, false). This includes -0 and overflows.
// In all other cases returns (-1, false).
// See https://262.ecma-international.org/#sec-canonicalnumericindexstring
func strToIntNum(s unistring.String) (int, bool) {
	n, ok := strToInt64(s)
	if n == 0 {
		return 0, ok
	}
	if ok && n >= -maxInt && n <= maxInt {
		if bits.UintSize == 32 {
			if n > math.MaxInt32 || n < math.MinInt32 {
				return 0, false
			}
		}
		return int(n), true
	}
	str := stringValueFromRaw(s)
	if str.ToNumber().toString().SameAs(str) {
		return 0, false
	}
	return -1, false
}

func strToGoIdx(s unistring.String) int {
	if n, ok := strToInt(s); ok {
		return n
	}
	return -1
}

func strToIdx64(s unistring.String) int64 {
	if n, ok := strToInt64(s); ok {
		return n
	}
	return -1
}

func assertCallable(v Value) (func(FunctionCall) Value, bool) {
	if obj, ok := v.(*Object); ok {
		return obj.self.assertCallable()
	}
	return nil, false
}
