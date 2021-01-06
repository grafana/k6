package goja

import (
	"fmt"
	"math"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/dop251/goja/unistring"
)

const (
	maxInt = 1 << 53
)

type valueStack []Value

type stash struct {
	values    valueStack
	extraArgs valueStack
	names     map[unistring.String]uint32
	obj       *Object

	outer *stash
}

type context struct {
	prg       *Program
	funcName  unistring.String
	stash     *stash
	newTarget Value
	pc, sb    int
	args      int
}

type iterStackItem struct {
	val  Value
	f    iterNextFunc
	iter *Object
}

type ref interface {
	get() Value
	set(Value)
	refname() unistring.String
}

type stashRef struct {
	v *Value
	n unistring.String
}

func (r stashRef) get() Value {
	return *r.v
}

func (r *stashRef) set(v Value) {
	*r.v = v
}

func (r *stashRef) refname() unistring.String {
	return r.n
}

type objRef struct {
	base   objectImpl
	name   unistring.String
	strict bool
}

func (r *objRef) get() Value {
	return r.base.getStr(r.name, nil)
}

func (r *objRef) set(v Value) {
	r.base.setOwnStr(r.name, v, r.strict)
}

func (r *objRef) refname() unistring.String {
	return r.name
}

type unresolvedRef struct {
	runtime *Runtime
	name    unistring.String
}

func (r *unresolvedRef) get() Value {
	r.runtime.throwReferenceError(r.name)
	panic("Unreachable")
}

func (r *unresolvedRef) set(Value) {
	r.get()
}

func (r *unresolvedRef) refname() unistring.String {
	return r.name
}

type vm struct {
	r            *Runtime
	prg          *Program
	funcName     unistring.String
	pc           int
	stack        valueStack
	sp, sb, args int

	stash     *stash
	callStack []context
	iterStack []iterStackItem
	refStack  []ref
	newTarget Value

	stashAllocs int
	halt        bool

	interrupted   uint32
	interruptVal  interface{}
	interruptLock sync.Mutex
}

type instruction interface {
	exec(*vm)
}

func intToValue(i int64) Value {
	if i >= -maxInt && i <= maxInt {
		if i >= -128 && i <= 127 {
			return intCache[i+128]
		}
		return valueInt(i)
	}
	return valueFloat(i)
}

func floatToInt(f float64) (result int64, ok bool) {
	if (f != 0 || !math.Signbit(f)) && !math.IsInf(f, 0) && f == math.Trunc(f) && f >= -maxInt && f <= maxInt {
		return int64(f), true
	}
	return 0, false
}

func floatToValue(f float64) (result Value) {
	if i, ok := floatToInt(f); ok {
		return intToValue(i)
	}
	switch {
	case f == 0:
		return _negativeZero
	case math.IsNaN(f):
		return _NaN
	case math.IsInf(f, 1):
		return _positiveInf
	case math.IsInf(f, -1):
		return _negativeInf
	}
	return valueFloat(f)
}

func assertInt64(v Value) (int64, bool) {
	num := v.ToNumber()
	if i, ok := num.(valueInt); ok {
		return int64(i), true
	}
	if f, ok := num.(valueFloat); ok {
		if i, ok := floatToInt(float64(f)); ok {
			return i, true
		}
	}
	return 0, false
}

func toIntIgnoreNegZero(v Value) (int64, bool) {
	num := v.ToNumber()
	if i, ok := num.(valueInt); ok {
		return int64(i), true
	}
	if f, ok := num.(valueFloat); ok {
		if v == _negativeZero {
			return 0, true
		}
		if i, ok := floatToInt(float64(f)); ok {
			return i, true
		}
	}
	return 0, false
}

func (s *valueStack) expand(idx int) {
	if idx < len(*s) {
		return
	}

	if idx < cap(*s) {
		*s = (*s)[:idx+1]
	} else {
		n := make([]Value, idx+1, (idx+1)<<1)
		copy(n, *s)
		*s = n
	}
}

func stashObjHas(obj *Object, name unistring.String) bool {
	if obj.self.hasPropertyStr(name) {
		if unscopables, ok := obj.self.getSym(SymUnscopables, nil).(*Object); ok {
			if b := unscopables.self.getStr(name, nil); b != nil {
				return !b.ToBoolean()
			}
		}
		return true
	}
	return false
}

func (s *stash) put(name unistring.String, v Value) bool {
	if s.obj != nil {
		if stashObjHas(s.obj, name) {
			s.obj.self.setOwnStr(name, v, false)
			return true
		}
		return false
	} else {
		if idx, found := s.names[name]; found {
			s.values.expand(int(idx))
			s.values[idx] = v
			return true
		}
		return false
	}
}

func (s *stash) putByIdx(idx uint32, v Value) {
	if s.obj != nil {
		panic("Attempt to put by idx into an object scope")
	}
	s.values.expand(int(idx))
	s.values[idx] = v
}

func (s *stash) getByIdx(idx uint32) Value {
	if int(idx) < len(s.values) {
		return s.values[idx]
	}
	return _undefined
}

func (s *stash) getByName(name unistring.String, _ *vm) (v Value, exists bool) {
	if s.obj != nil {
		if stashObjHas(s.obj, name) {
			return nilSafe(s.obj.self.getStr(name, nil)), true
		}
		return nil, false
	}
	if idx, exists := s.names[name]; exists {
		return s.values[idx], true
	}
	return nil, false
	//return valueUnresolved{r: vm.r, ref: name}, false
}

func (s *stash) createBinding(name unistring.String) {
	if s.names == nil {
		s.names = make(map[unistring.String]uint32)
	}
	if _, exists := s.names[name]; !exists {
		s.names[name] = uint32(len(s.names))
		s.values = append(s.values, _undefined)
	}
}

func (s *stash) deleteBinding(name unistring.String) bool {
	if s.obj != nil {
		if stashObjHas(s.obj, name) {
			return s.obj.self.deleteStr(name, false)
		}
		return false
	}
	if idx, found := s.names[name]; found {
		s.values[idx] = nil
		delete(s.names, name)
		return true
	}
	return false
}

func (vm *vm) newStash() {
	vm.stash = &stash{
		outer: vm.stash,
	}
	vm.stashAllocs++
}

func (vm *vm) init() {
}

func (vm *vm) run() {
	vm.halt = false
	interrupted := false
	ticks := 0
	for !vm.halt {
		if interrupted = atomic.LoadUint32(&vm.interrupted) != 0; interrupted {
			break
		}
		vm.prg.code[vm.pc].exec(vm)
		ticks++
		if ticks > 10000 {
			runtime.Gosched()
			vm.r.removeDeadKeys()
			ticks = 0
		}
	}

	if interrupted {
		vm.interruptLock.Lock()
		v := &InterruptedError{
			iface: vm.interruptVal,
		}
		atomic.StoreUint32(&vm.interrupted, 0)
		vm.interruptVal = nil
		vm.interruptLock.Unlock()
		panic(v)
	}
}

func (vm *vm) Interrupt(v interface{}) {
	vm.interruptLock.Lock()
	vm.interruptVal = v
	atomic.StoreUint32(&vm.interrupted, 1)
	vm.interruptLock.Unlock()
}

func (vm *vm) ClearInterrupt() {
	atomic.StoreUint32(&vm.interrupted, 0)
}

func (vm *vm) captureStack(stack []StackFrame, ctxOffset int) []StackFrame {
	// Unroll the context stack
	stack = append(stack, StackFrame{prg: vm.prg, pc: vm.pc, funcName: vm.funcName})
	for i := len(vm.callStack) - 1; i > ctxOffset-1; i-- {
		if vm.callStack[i].pc != -1 {
			stack = append(stack, StackFrame{prg: vm.callStack[i].prg, pc: vm.callStack[i].pc - 1, funcName: vm.callStack[i].funcName})
		}
	}
	return stack
}

func (vm *vm) try(f func()) (ex *Exception) {
	var ctx context
	vm.saveCtx(&ctx)

	ctxOffset := len(vm.callStack)
	sp := vm.sp
	iterLen := len(vm.iterStack)
	refLen := len(vm.refStack)

	defer func() {
		if x := recover(); x != nil {
			defer func() {
				vm.callStack = vm.callStack[:ctxOffset]
				vm.restoreCtx(&ctx)
				vm.sp = sp

				// Restore other stacks
				iterTail := vm.iterStack[iterLen:]
				for i := range iterTail {
					if iter := iterTail[i].iter; iter != nil {
						returnIter(iter)
					}
					iterTail[i] = iterStackItem{}
				}
				vm.iterStack = vm.iterStack[:iterLen]
				refTail := vm.refStack[refLen:]
				for i := range refTail {
					refTail[i] = nil
				}
				vm.refStack = vm.refStack[:refLen]
			}()
			switch x1 := x.(type) {
			case Value:
				ex = &Exception{
					val: x1,
				}
			case *InterruptedError:
				x1.stack = vm.captureStack(x1.stack, ctxOffset)
				panic(x1)
			case *Exception:
				ex = x1
			case typeError:
				ex = &Exception{
					val: vm.r.NewTypeError(string(x1)),
				}
			case rangeError:
				ex = &Exception{
					val: vm.r.newError(vm.r.global.RangeError, string(x1)),
				}
			default:
				/*
					if vm.prg != nil {
						vm.prg.dumpCode(log.Printf)
					}
					log.Print("Stack: ", string(debug.Stack()))
					panic(fmt.Errorf("Panic at %d: %v", vm.pc, x))
				*/
				panic(x)
			}
			ex.stack = vm.captureStack(ex.stack, ctxOffset)
		}
	}()

	f()
	return
}

func (vm *vm) runTry() (ex *Exception) {
	return vm.try(vm.run)
}

func (vm *vm) push(v Value) {
	vm.stack.expand(vm.sp)
	vm.stack[vm.sp] = v
	vm.sp++
}

func (vm *vm) pop() Value {
	vm.sp--
	return vm.stack[vm.sp]
}

func (vm *vm) peek() Value {
	return vm.stack[vm.sp-1]
}

func (vm *vm) saveCtx(ctx *context) {
	ctx.prg = vm.prg
	if vm.funcName != "" {
		ctx.funcName = vm.funcName
	} else if ctx.prg != nil && ctx.prg.funcName != "" {
		ctx.funcName = ctx.prg.funcName
	}
	ctx.stash = vm.stash
	ctx.newTarget = vm.newTarget
	ctx.pc = vm.pc
	ctx.sb = vm.sb
	ctx.args = vm.args
}

func (vm *vm) pushCtx() {
	/*
		vm.ctxStack = append(vm.ctxStack, context{
			prg: vm.prg,
			stash: vm.stash,
			pc: vm.pc,
			sb: vm.sb,
			args: vm.args,
		})*/
	vm.callStack = append(vm.callStack, context{})
	vm.saveCtx(&vm.callStack[len(vm.callStack)-1])
}

func (vm *vm) restoreCtx(ctx *context) {
	vm.prg = ctx.prg
	vm.funcName = ctx.funcName
	vm.pc = ctx.pc
	vm.stash = ctx.stash
	vm.sb = ctx.sb
	vm.args = ctx.args
	vm.newTarget = ctx.newTarget
}

func (vm *vm) popCtx() {
	l := len(vm.callStack) - 1
	vm.prg = vm.callStack[l].prg
	vm.callStack[l].prg = nil
	vm.funcName = vm.callStack[l].funcName
	vm.pc = vm.callStack[l].pc
	vm.stash = vm.callStack[l].stash
	vm.callStack[l].stash = nil
	vm.sb = vm.callStack[l].sb
	vm.args = vm.callStack[l].args

	vm.callStack = vm.callStack[:l]
}

func (vm *vm) toCallee(v Value) *Object {
	if obj, ok := v.(*Object); ok {
		return obj
	}
	switch unresolved := v.(type) {
	case valueUnresolved:
		unresolved.throw()
		panic("Unreachable")
	case memberUnresolved:
		panic(vm.r.NewTypeError("Object has no member '%s'", unresolved.ref))
	}
	panic(vm.r.NewTypeError("Value is not an object: %s", v.toString()))
}

type _newStash struct{}

var newStash _newStash

func (_newStash) exec(vm *vm) {
	vm.newStash()
	vm.pc++
}

type loadVal uint32

func (l loadVal) exec(vm *vm) {
	vm.push(vm.prg.values[l])
	vm.pc++
}

type _loadUndef struct{}

var loadUndef _loadUndef

func (_loadUndef) exec(vm *vm) {
	vm.push(_undefined)
	vm.pc++
}

type _loadNil struct{}

var loadNil _loadNil

func (_loadNil) exec(vm *vm) {
	vm.push(nil)
	vm.pc++
}

type _loadGlobalObject struct{}

var loadGlobalObject _loadGlobalObject

func (_loadGlobalObject) exec(vm *vm) {
	vm.push(vm.r.globalObject)
	vm.pc++
}

type loadStack int

func (l loadStack) exec(vm *vm) {
	// l < 0 -- arg<-l-1>
	// l > 0 -- var<l-1>
	// l == 0 -- this

	if l < 0 {
		arg := int(-l)
		if arg > vm.args {
			vm.push(_undefined)
		} else {
			vm.push(vm.stack[vm.sb+arg])
		}
	} else if l > 0 {
		vm.push(vm.stack[vm.sb+vm.args+int(l)])
	} else {
		vm.push(vm.stack[vm.sb])
	}
	vm.pc++
}

type _loadCallee struct{}

var loadCallee _loadCallee

func (_loadCallee) exec(vm *vm) {
	vm.push(vm.stack[vm.sb-1])
	vm.pc++
}

func (vm *vm) storeStack(s int) {
	// l < 0 -- arg<-l-1>
	// l > 0 -- var<l-1>
	// l == 0 -- this

	if s < 0 {
		vm.stack[vm.sb-s] = vm.stack[vm.sp-1]
	} else if s > 0 {
		vm.stack[vm.sb+vm.args+s] = vm.stack[vm.sp-1]
	} else {
		panic("Attempt to modify this")
	}
	vm.pc++
}

type storeStack int

func (s storeStack) exec(vm *vm) {
	vm.storeStack(int(s))
}

type storeStackP int

func (s storeStackP) exec(vm *vm) {
	vm.storeStack(int(s))
	vm.sp--
}

type _toNumber struct{}

var toNumber _toNumber

func (_toNumber) exec(vm *vm) {
	vm.stack[vm.sp-1] = vm.stack[vm.sp-1].ToNumber()
	vm.pc++
}

type _add struct{}

var add _add

func (_add) exec(vm *vm) {
	right := vm.stack[vm.sp-1]
	left := vm.stack[vm.sp-2]

	if o, ok := left.(*Object); ok {
		left = o.toPrimitive()
	}

	if o, ok := right.(*Object); ok {
		right = o.toPrimitive()
	}

	var ret Value

	leftString, isLeftString := left.(valueString)
	rightString, isRightString := right.(valueString)

	if isLeftString || isRightString {
		if !isLeftString {
			leftString = left.toString()
		}
		if !isRightString {
			rightString = right.toString()
		}
		ret = leftString.concat(rightString)
	} else {
		if leftInt, ok := left.(valueInt); ok {
			if rightInt, ok := right.(valueInt); ok {
				ret = intToValue(int64(leftInt) + int64(rightInt))
			} else {
				ret = floatToValue(float64(leftInt) + right.ToFloat())
			}
		} else {
			ret = floatToValue(left.ToFloat() + right.ToFloat())
		}
	}

	vm.stack[vm.sp-2] = ret
	vm.sp--
	vm.pc++
}

type _sub struct{}

var sub _sub

func (_sub) exec(vm *vm) {
	right := vm.stack[vm.sp-1]
	left := vm.stack[vm.sp-2]

	var result Value

	if left, ok := left.(valueInt); ok {
		if right, ok := right.(valueInt); ok {
			result = intToValue(int64(left) - int64(right))
			goto end
		}
	}

	result = floatToValue(left.ToFloat() - right.ToFloat())
end:
	vm.sp--
	vm.stack[vm.sp-1] = result
	vm.pc++
}

type _mul struct{}

var mul _mul

func (_mul) exec(vm *vm) {
	left := vm.stack[vm.sp-2]
	right := vm.stack[vm.sp-1]

	var result Value

	if left, ok := assertInt64(left); ok {
		if right, ok := assertInt64(right); ok {
			if left == 0 && right == -1 || left == -1 && right == 0 {
				result = _negativeZero
				goto end
			}
			res := left * right
			// check for overflow
			if left == 0 || right == 0 || res/left == right {
				result = intToValue(res)
				goto end
			}

		}
	}

	result = floatToValue(left.ToFloat() * right.ToFloat())

end:
	vm.sp--
	vm.stack[vm.sp-1] = result
	vm.pc++
}

type _div struct{}

var div _div

func (_div) exec(vm *vm) {
	left := vm.stack[vm.sp-2].ToFloat()
	right := vm.stack[vm.sp-1].ToFloat()

	var result Value

	if math.IsNaN(left) || math.IsNaN(right) {
		result = _NaN
		goto end
	}
	if math.IsInf(left, 0) && math.IsInf(right, 0) {
		result = _NaN
		goto end
	}
	if left == 0 && right == 0 {
		result = _NaN
		goto end
	}

	if math.IsInf(left, 0) {
		if math.Signbit(left) == math.Signbit(right) {
			result = _positiveInf
			goto end
		} else {
			result = _negativeInf
			goto end
		}
	}
	if math.IsInf(right, 0) {
		if math.Signbit(left) == math.Signbit(right) {
			result = _positiveZero
			goto end
		} else {
			result = _negativeZero
			goto end
		}
	}
	if right == 0 {
		if math.Signbit(left) == math.Signbit(right) {
			result = _positiveInf
			goto end
		} else {
			result = _negativeInf
			goto end
		}
	}

	result = floatToValue(left / right)

end:
	vm.sp--
	vm.stack[vm.sp-1] = result
	vm.pc++
}

type _mod struct{}

var mod _mod

func (_mod) exec(vm *vm) {
	left := vm.stack[vm.sp-2]
	right := vm.stack[vm.sp-1]

	var result Value

	if leftInt, ok := assertInt64(left); ok {
		if rightInt, ok := assertInt64(right); ok {
			if rightInt == 0 {
				result = _NaN
				goto end
			}
			r := leftInt % rightInt
			if r == 0 && leftInt < 0 {
				result = _negativeZero
			} else {
				result = intToValue(leftInt % rightInt)
			}
			goto end
		}
	}

	result = floatToValue(math.Mod(left.ToFloat(), right.ToFloat()))
end:
	vm.sp--
	vm.stack[vm.sp-1] = result
	vm.pc++
}

type _neg struct{}

var neg _neg

func (_neg) exec(vm *vm) {
	operand := vm.stack[vm.sp-1]

	var result Value

	if i, ok := assertInt64(operand); ok {
		if i == 0 {
			result = _negativeZero
		} else {
			result = valueInt(-i)
		}
	} else {
		f := operand.ToFloat()
		if !math.IsNaN(f) {
			f = -f
		}
		result = valueFloat(f)
	}

	vm.stack[vm.sp-1] = result
	vm.pc++
}

type _plus struct{}

var plus _plus

func (_plus) exec(vm *vm) {
	vm.stack[vm.sp-1] = vm.stack[vm.sp-1].ToNumber()
	vm.pc++
}

type _inc struct{}

var inc _inc

func (_inc) exec(vm *vm) {
	v := vm.stack[vm.sp-1]

	if i, ok := assertInt64(v); ok {
		v = intToValue(i + 1)
		goto end
	}

	v = valueFloat(v.ToFloat() + 1)

end:
	vm.stack[vm.sp-1] = v
	vm.pc++
}

type _dec struct{}

var dec _dec

func (_dec) exec(vm *vm) {
	v := vm.stack[vm.sp-1]

	if i, ok := assertInt64(v); ok {
		v = intToValue(i - 1)
		goto end
	}

	v = valueFloat(v.ToFloat() - 1)

end:
	vm.stack[vm.sp-1] = v
	vm.pc++
}

type _and struct{}

var and _and

func (_and) exec(vm *vm) {
	left := toInt32(vm.stack[vm.sp-2])
	right := toInt32(vm.stack[vm.sp-1])
	vm.stack[vm.sp-2] = intToValue(int64(left & right))
	vm.sp--
	vm.pc++
}

type _or struct{}

var or _or

func (_or) exec(vm *vm) {
	left := toInt32(vm.stack[vm.sp-2])
	right := toInt32(vm.stack[vm.sp-1])
	vm.stack[vm.sp-2] = intToValue(int64(left | right))
	vm.sp--
	vm.pc++
}

type _xor struct{}

var xor _xor

func (_xor) exec(vm *vm) {
	left := toInt32(vm.stack[vm.sp-2])
	right := toInt32(vm.stack[vm.sp-1])
	vm.stack[vm.sp-2] = intToValue(int64(left ^ right))
	vm.sp--
	vm.pc++
}

type _bnot struct{}

var bnot _bnot

func (_bnot) exec(vm *vm) {
	op := toInt32(vm.stack[vm.sp-1])
	vm.stack[vm.sp-1] = intToValue(int64(^op))
	vm.pc++
}

type _sal struct{}

var sal _sal

func (_sal) exec(vm *vm) {
	left := toInt32(vm.stack[vm.sp-2])
	right := toUint32(vm.stack[vm.sp-1])
	vm.stack[vm.sp-2] = intToValue(int64(left << (right & 0x1F)))
	vm.sp--
	vm.pc++
}

type _sar struct{}

var sar _sar

func (_sar) exec(vm *vm) {
	left := toInt32(vm.stack[vm.sp-2])
	right := toUint32(vm.stack[vm.sp-1])
	vm.stack[vm.sp-2] = intToValue(int64(left >> (right & 0x1F)))
	vm.sp--
	vm.pc++
}

type _shr struct{}

var shr _shr

func (_shr) exec(vm *vm) {
	left := toUint32(vm.stack[vm.sp-2])
	right := toUint32(vm.stack[vm.sp-1])
	vm.stack[vm.sp-2] = intToValue(int64(left >> (right & 0x1F)))
	vm.sp--
	vm.pc++
}

type _halt struct{}

var halt _halt

func (_halt) exec(vm *vm) {
	vm.halt = true
	vm.pc++
}

type jump int32

func (j jump) exec(vm *vm) {
	vm.pc += int(j)
}

type _setElem struct{}

var setElem _setElem

func (_setElem) exec(vm *vm) {
	obj := vm.stack[vm.sp-3].ToObject(vm.r)
	propName := toPropertyKey(vm.stack[vm.sp-2])
	val := vm.stack[vm.sp-1]

	obj.setOwn(propName, val, false)

	vm.sp -= 2
	vm.stack[vm.sp-1] = val
	vm.pc++
}

type _setElemStrict struct{}

var setElemStrict _setElemStrict

func (_setElemStrict) exec(vm *vm) {
	obj := vm.r.toObject(vm.stack[vm.sp-3])
	propName := toPropertyKey(vm.stack[vm.sp-2])
	val := vm.stack[vm.sp-1]

	obj.setOwn(propName, val, true)

	vm.sp -= 2
	vm.stack[vm.sp-1] = val
	vm.pc++
}

type _deleteElem struct{}

var deleteElem _deleteElem

func (_deleteElem) exec(vm *vm) {
	obj := vm.r.toObject(vm.stack[vm.sp-2])
	propName := toPropertyKey(vm.stack[vm.sp-1])
	if obj.delete(propName, false) {
		vm.stack[vm.sp-2] = valueTrue
	} else {
		vm.stack[vm.sp-2] = valueFalse
	}
	vm.sp--
	vm.pc++
}

type _deleteElemStrict struct{}

var deleteElemStrict _deleteElemStrict

func (_deleteElemStrict) exec(vm *vm) {
	obj := vm.r.toObject(vm.stack[vm.sp-2])
	propName := toPropertyKey(vm.stack[vm.sp-1])
	obj.delete(propName, true)
	vm.stack[vm.sp-2] = valueTrue
	vm.sp--
	vm.pc++
}

type deleteProp unistring.String

func (d deleteProp) exec(vm *vm) {
	obj := vm.r.toObject(vm.stack[vm.sp-1])
	if obj.self.deleteStr(unistring.String(d), false) {
		vm.stack[vm.sp-1] = valueTrue
	} else {
		vm.stack[vm.sp-1] = valueFalse
	}
	vm.pc++
}

type deletePropStrict unistring.String

func (d deletePropStrict) exec(vm *vm) {
	obj := vm.r.toObject(vm.stack[vm.sp-1])
	obj.self.deleteStr(unistring.String(d), true)
	vm.stack[vm.sp-1] = valueTrue
	vm.pc++
}

type setProp unistring.String

func (p setProp) exec(vm *vm) {
	val := vm.stack[vm.sp-1]
	vm.stack[vm.sp-2].ToObject(vm.r).self.setOwnStr(unistring.String(p), val, false)
	vm.stack[vm.sp-2] = val
	vm.sp--
	vm.pc++
}

type setPropStrict unistring.String

func (p setPropStrict) exec(vm *vm) {
	obj := vm.stack[vm.sp-2]
	val := vm.stack[vm.sp-1]

	obj1 := vm.r.toObject(obj)
	obj1.self.setOwnStr(unistring.String(p), val, true)
	vm.stack[vm.sp-2] = val
	vm.sp--
	vm.pc++
}

type setProp1 unistring.String

func (p setProp1) exec(vm *vm) {
	vm.r.toObject(vm.stack[vm.sp-2]).self._putProp(unistring.String(p), vm.stack[vm.sp-1], true, true, true)

	vm.sp--
	vm.pc++
}

type _setProto struct{}

var setProto _setProto

func (_setProto) exec(vm *vm) {
	vm.r.toObject(vm.stack[vm.sp-2]).self.setProto(vm.r.toProto(vm.stack[vm.sp-1]), true)

	vm.sp--
	vm.pc++
}

type setPropGetter unistring.String

func (s setPropGetter) exec(vm *vm) {
	obj := vm.r.toObject(vm.stack[vm.sp-2])
	val := vm.stack[vm.sp-1]

	descr := PropertyDescriptor{
		Getter:       val,
		Configurable: FLAG_TRUE,
		Enumerable:   FLAG_TRUE,
	}

	obj.self.defineOwnPropertyStr(unistring.String(s), descr, false)

	vm.sp--
	vm.pc++
}

type setPropSetter unistring.String

func (s setPropSetter) exec(vm *vm) {
	obj := vm.r.toObject(vm.stack[vm.sp-2])
	val := vm.stack[vm.sp-1]

	descr := PropertyDescriptor{
		Setter:       val,
		Configurable: FLAG_TRUE,
		Enumerable:   FLAG_TRUE,
	}

	obj.self.defineOwnPropertyStr(unistring.String(s), descr, false)

	vm.sp--
	vm.pc++
}

type getProp unistring.String

func (g getProp) exec(vm *vm) {
	v := vm.stack[vm.sp-1]
	obj := v.baseObject(vm.r)
	if obj == nil {
		panic(vm.r.NewTypeError("Cannot read property '%s' of undefined", g))
	}
	vm.stack[vm.sp-1] = nilSafe(obj.self.getStr(unistring.String(g), v))

	vm.pc++
}

type getPropCallee unistring.String

func (g getPropCallee) exec(vm *vm) {
	v := vm.stack[vm.sp-1]
	obj := v.baseObject(vm.r)
	n := unistring.String(g)
	if obj == nil {
		panic(vm.r.NewTypeError("Cannot read property '%s' of undefined or null", n))
	}
	prop := obj.self.getStr(n, v)
	if prop == nil {
		prop = memberUnresolved{valueUnresolved{r: vm.r, ref: n}}
	}
	vm.stack[vm.sp-1] = prop

	vm.pc++
}

type _getElem struct{}

var getElem _getElem

func (_getElem) exec(vm *vm) {
	v := vm.stack[vm.sp-2]
	obj := v.baseObject(vm.r)
	propName := toPropertyKey(vm.stack[vm.sp-1])
	if obj == nil {
		panic(vm.r.NewTypeError("Cannot read property '%s' of undefined", propName.String()))
	}

	vm.stack[vm.sp-2] = nilSafe(obj.get(propName, v))

	vm.sp--
	vm.pc++
}

type _getElemCallee struct{}

var getElemCallee _getElemCallee

func (_getElemCallee) exec(vm *vm) {
	v := vm.stack[vm.sp-2]
	obj := v.baseObject(vm.r)
	propName := toPropertyKey(vm.stack[vm.sp-1])
	if obj == nil {
		panic(vm.r.NewTypeError("Cannot read property '%s' of undefined", propName.String()))
	}

	prop := obj.get(propName, v)
	if prop == nil {
		prop = memberUnresolved{valueUnresolved{r: vm.r, ref: propName.string()}}
	}
	vm.stack[vm.sp-2] = prop

	vm.sp--
	vm.pc++
}

type _dup struct{}

var dup _dup

func (_dup) exec(vm *vm) {
	vm.push(vm.stack[vm.sp-1])
	vm.pc++
}

type dupN uint32

func (d dupN) exec(vm *vm) {
	vm.push(vm.stack[vm.sp-1-int(d)])
	vm.pc++
}

type rdupN uint32

func (d rdupN) exec(vm *vm) {
	vm.stack[vm.sp-1-int(d)] = vm.stack[vm.sp-1]
	vm.pc++
}

type _newObject struct{}

var newObject _newObject

func (_newObject) exec(vm *vm) {
	vm.push(vm.r.NewObject())
	vm.pc++
}

type newArray uint32

func (l newArray) exec(vm *vm) {
	values := make([]Value, l)
	if l > 0 {
		copy(values, vm.stack[vm.sp-int(l):vm.sp])
	}
	obj := vm.r.newArrayValues(values)
	if l > 0 {
		vm.sp -= int(l) - 1
		vm.stack[vm.sp-1] = obj
	} else {
		vm.push(obj)
	}
	vm.pc++
}

type newArraySparse struct {
	l, objCount int
}

func (n *newArraySparse) exec(vm *vm) {
	values := make([]Value, n.l)
	copy(values, vm.stack[vm.sp-int(n.l):vm.sp])
	arr := vm.r.newArrayObject()
	setArrayValues(arr, values)
	arr.objCount = n.objCount
	vm.sp -= int(n.l) - 1
	vm.stack[vm.sp-1] = arr.val
	vm.pc++
}

type newRegexp struct {
	pattern *regexpPattern
	src     valueString
}

func (n *newRegexp) exec(vm *vm) {
	vm.push(vm.r.newRegExpp(n.pattern.clone(), n.src, vm.r.global.RegExpPrototype))
	vm.pc++
}

func (vm *vm) setLocal(s int) {
	v := vm.stack[vm.sp-1]
	level := s >> 24
	idx := uint32(s & 0x00FFFFFF)
	stash := vm.stash
	for i := 0; i < level; i++ {
		stash = stash.outer
	}
	stash.putByIdx(idx, v)
	vm.pc++
}

type setLocal uint32

func (s setLocal) exec(vm *vm) {
	vm.setLocal(int(s))
}

type setLocalP uint32

func (s setLocalP) exec(vm *vm) {
	vm.setLocal(int(s))
	vm.sp--
}

type setVar struct {
	name unistring.String
	idx  uint32
}

func (s setVar) exec(vm *vm) {
	v := vm.peek()

	level := int(s.idx >> 24)
	idx := s.idx & 0x00FFFFFF
	stash := vm.stash
	name := s.name
	for i := 0; i < level; i++ {
		if stash.put(name, v) {
			goto end
		}
		stash = stash.outer
	}

	if stash != nil {
		stash.putByIdx(idx, v)
	} else {
		vm.r.globalObject.self.setOwnStr(name, v, false)
	}

end:
	vm.pc++
}

type resolveVar1 unistring.String

func (s resolveVar1) exec(vm *vm) {
	name := unistring.String(s)
	var ref ref
	for stash := vm.stash; stash != nil; stash = stash.outer {
		if stash.obj != nil {
			if stashObjHas(stash.obj, name) {
				ref = &objRef{
					base: stash.obj.self,
					name: name,
				}
				goto end
			}
		} else {
			if idx, exists := stash.names[name]; exists {
				ref = &stashRef{
					v: &stash.values[idx],
				}
				goto end
			}
		}
	}

	ref = &objRef{
		base: vm.r.globalObject.self,
		name: name,
	}

end:
	vm.refStack = append(vm.refStack, ref)
	vm.pc++
}

type deleteVar unistring.String

func (d deleteVar) exec(vm *vm) {
	name := unistring.String(d)
	ret := true
	for stash := vm.stash; stash != nil; stash = stash.outer {
		if stash.obj != nil {
			if stashObjHas(stash.obj, name) {
				ret = stash.obj.self.deleteStr(name, false)
				goto end
			}
		} else {
			if _, exists := stash.names[name]; exists {
				ret = false
				goto end
			}
		}
	}

	if vm.r.globalObject.self.hasPropertyStr(name) {
		ret = vm.r.globalObject.self.deleteStr(name, false)
	}

end:
	if ret {
		vm.push(valueTrue)
	} else {
		vm.push(valueFalse)
	}
	vm.pc++
}

type deleteGlobal unistring.String

func (d deleteGlobal) exec(vm *vm) {
	name := unistring.String(d)
	var ret bool
	if vm.r.globalObject.self.hasPropertyStr(name) {
		ret = vm.r.globalObject.self.deleteStr(name, false)
	} else {
		ret = true
	}
	if ret {
		vm.push(valueTrue)
	} else {
		vm.push(valueFalse)
	}
	vm.pc++
}

type resolveVar1Strict unistring.String

func (s resolveVar1Strict) exec(vm *vm) {
	name := unistring.String(s)
	var ref ref
	for stash := vm.stash; stash != nil; stash = stash.outer {
		if stash.obj != nil {
			if stashObjHas(stash.obj, name) {
				ref = &objRef{
					base:   stash.obj.self,
					name:   name,
					strict: true,
				}
				goto end
			}
		} else {
			if idx, exists := stash.names[name]; exists {
				ref = &stashRef{
					v: &stash.values[idx],
				}
				goto end
			}
		}
	}

	if vm.r.globalObject.self.hasPropertyStr(name) {
		ref = &objRef{
			base:   vm.r.globalObject.self,
			name:   name,
			strict: true,
		}
		goto end
	}

	ref = &unresolvedRef{
		runtime: vm.r,
		name:    name,
	}

end:
	vm.refStack = append(vm.refStack, ref)
	vm.pc++
}

type setGlobal unistring.String

func (s setGlobal) exec(vm *vm) {
	v := vm.peek()

	vm.r.globalObject.self.setOwnStr(unistring.String(s), v, false)
	vm.pc++
}

type setGlobalStrict unistring.String

func (s setGlobalStrict) exec(vm *vm) {
	v := vm.peek()

	name := unistring.String(s)
	o := vm.r.globalObject.self
	if o.hasOwnPropertyStr(name) {
		o.setOwnStr(name, v, true)
	} else {
		vm.r.throwReferenceError(name)
	}
	vm.pc++
}

type getLocal uint32

func (g getLocal) exec(vm *vm) {
	level := int(g >> 24)
	idx := uint32(g & 0x00FFFFFF)
	stash := vm.stash
	for i := 0; i < level; i++ {
		stash = stash.outer
	}

	vm.push(stash.getByIdx(idx))
	vm.pc++
}

type getVar struct {
	name        unistring.String
	idx         uint32
	ref, callee bool
}

func (g getVar) exec(vm *vm) {
	level := int(g.idx >> 24)
	idx := g.idx & 0x00FFFFFF
	stash := vm.stash
	name := g.name
	for i := 0; i < level; i++ {
		if v, found := stash.getByName(name, vm); found {
			if g.callee {
				if stash.obj != nil {
					vm.push(stash.obj)
				} else {
					vm.push(_undefined)
				}
			}
			vm.push(v)
			goto end
		}
		stash = stash.outer
	}
	if g.callee {
		vm.push(_undefined)
	}
	if stash != nil {
		vm.push(stash.getByIdx(idx))
	} else {
		v := vm.r.globalObject.self.getStr(name, nil)
		if v == nil {
			if g.ref {
				v = valueUnresolved{r: vm.r, ref: name}
			} else {
				vm.r.throwReferenceError(name)
			}
		}
		vm.push(v)
	}
end:
	vm.pc++
}

type resolveVar struct {
	name   unistring.String
	idx    uint32
	strict bool
}

func (r resolveVar) exec(vm *vm) {
	level := int(r.idx >> 24)
	idx := r.idx & 0x00FFFFFF
	stash := vm.stash
	var ref ref
	for i := 0; i < level; i++ {
		if obj := stash.obj; obj != nil {
			if stashObjHas(obj, r.name) {
				ref = &objRef{
					base:   stash.obj.self,
					name:   r.name,
					strict: r.strict,
				}
				goto end
			}
		} else {
			if idx, exists := stash.names[r.name]; exists {
				ref = &stashRef{
					v: &stash.values[idx],
				}
				goto end
			}
		}
		stash = stash.outer
	}

	if stash != nil {
		ref = &stashRef{
			v: &stash.values[idx],
		}
		goto end
	} /*else {
		if vm.r.globalObject.self.hasProperty(nameVal) {
			ref = &objRef{
				base: vm.r.globalObject.self,
				name: r.name,
			}
			goto end
		}
	} */

	ref = &unresolvedRef{
		runtime: vm.r,
		name:    r.name,
	}

end:
	vm.refStack = append(vm.refStack, ref)
	vm.pc++
}

type _getValue struct{}

var getValue _getValue

func (_getValue) exec(vm *vm) {
	ref := vm.refStack[len(vm.refStack)-1]
	if v := ref.get(); v != nil {
		vm.push(v)
	} else {
		vm.r.throwReferenceError(ref.refname())
		panic("Unreachable")
	}
	vm.pc++
}

type _putValue struct{}

var putValue _putValue

func (_putValue) exec(vm *vm) {
	l := len(vm.refStack) - 1
	ref := vm.refStack[l]
	vm.refStack[l] = nil
	vm.refStack = vm.refStack[:l]
	ref.set(vm.stack[vm.sp-1])
	vm.pc++
}

type getVar1 unistring.String

func (n getVar1) exec(vm *vm) {
	name := unistring.String(n)
	var val Value
	for stash := vm.stash; stash != nil; stash = stash.outer {
		if v, exists := stash.getByName(name, vm); exists {
			val = v
			break
		}
	}
	if val == nil {
		val = vm.r.globalObject.self.getStr(name, nil)
		if val == nil {
			vm.r.throwReferenceError(name)
		}
	}
	vm.push(val)
	vm.pc++
}

type getVar1Ref string

func (n getVar1Ref) exec(vm *vm) {
	name := unistring.String(n)
	var val Value
	for stash := vm.stash; stash != nil; stash = stash.outer {
		if v, exists := stash.getByName(name, vm); exists {
			val = v
			break
		}
	}
	if val == nil {
		val = vm.r.globalObject.self.getStr(name, nil)
		if val == nil {
			val = valueUnresolved{r: vm.r, ref: name}
		}
	}
	vm.push(val)
	vm.pc++
}

type getVar1Callee unistring.String

func (n getVar1Callee) exec(vm *vm) {
	name := unistring.String(n)
	var val Value
	var callee *Object
	for stash := vm.stash; stash != nil; stash = stash.outer {
		if v, exists := stash.getByName(name, vm); exists {
			callee = stash.obj
			val = v
			break
		}
	}
	if val == nil {
		val = vm.r.globalObject.self.getStr(name, nil)
		if val == nil {
			val = valueUnresolved{r: vm.r, ref: name}
		}
	}
	if callee != nil {
		vm.push(callee)
	} else {
		vm.push(_undefined)
	}
	vm.push(val)
	vm.pc++
}

type _pop struct{}

var pop _pop

func (_pop) exec(vm *vm) {
	vm.sp--
	vm.pc++
}

func (vm *vm) callEval(n int, strict bool) {
	if vm.r.toObject(vm.stack[vm.sp-n-1]) == vm.r.global.Eval {
		if n > 0 {
			srcVal := vm.stack[vm.sp-n]
			if src, ok := srcVal.(valueString); ok {
				var this Value
				if vm.sb != 0 {
					this = vm.stack[vm.sb]
				} else {
					this = vm.r.globalObject
				}
				ret := vm.r.eval(src, true, strict, this)
				vm.stack[vm.sp-n-2] = ret
			} else {
				vm.stack[vm.sp-n-2] = srcVal
			}
		} else {
			vm.stack[vm.sp-n-2] = _undefined
		}

		vm.sp -= n + 1
		vm.pc++
	} else {
		call(n).exec(vm)
	}
}

type callEval uint32

func (numargs callEval) exec(vm *vm) {
	vm.callEval(int(numargs), false)
}

type callEvalStrict uint32

func (numargs callEvalStrict) exec(vm *vm) {
	vm.callEval(int(numargs), true)
}

type _boxThis struct{}

var boxThis _boxThis

func (_boxThis) exec(vm *vm) {
	v := vm.stack[vm.sb]
	if v == _undefined || v == _null {
		vm.stack[vm.sb] = vm.r.globalObject
	} else {
		vm.stack[vm.sb] = v.ToObject(vm.r)
	}
	vm.pc++
}

type call uint32

func (numargs call) exec(vm *vm) {
	// this
	// callee
	// arg0
	// ...
	// arg<numargs-1>
	n := int(numargs)
	v := vm.stack[vm.sp-n-1] // callee
	obj := vm.toCallee(v)
repeat:
	switch f := obj.self.(type) {
	case *funcObject:
		vm.pc++
		vm.pushCtx()
		vm.args = n
		vm.prg = f.prg
		vm.stash = f.stash
		vm.pc = 0
		vm.stack[vm.sp-n-1], vm.stack[vm.sp-n-2] = vm.stack[vm.sp-n-2], vm.stack[vm.sp-n-1]
		return
	case *nativeFuncObject:
		vm._nativeCall(f, n)
	case *boundFuncObject:
		vm._nativeCall(&f.nativeFuncObject, n)
	case *proxyObject:
		vm.pushCtx()
		vm.prg = nil
		vm.funcName = "proxy"
		ret := f.apply(FunctionCall{This: vm.stack[vm.sp-n-2], Arguments: vm.stack[vm.sp-n : vm.sp]})
		if ret == nil {
			ret = _undefined
		}
		vm.stack[vm.sp-n-2] = ret
		vm.popCtx()
		vm.sp -= n + 1
		vm.pc++
	case *lazyObject:
		obj.self = f.create(obj)
		goto repeat
	default:
		vm.r.typeErrorResult(true, "Not a function: %s", obj.toString())
	}
}

func (vm *vm) _nativeCall(f *nativeFuncObject, n int) {
	if f.f != nil {
		vm.pushCtx()
		vm.prg = nil
		vm.funcName = f.nameProp.get(nil).string()
		ret := f.f(FunctionCall{
			Arguments: vm.stack[vm.sp-n : vm.sp],
			This:      vm.stack[vm.sp-n-2],
		})
		if ret == nil {
			ret = _undefined
		}
		vm.stack[vm.sp-n-2] = ret
		vm.popCtx()
	} else {
		vm.stack[vm.sp-n-2] = _undefined
	}
	vm.sp -= n + 1
	vm.pc++
}

func (vm *vm) clearStack() {
	stackTail := vm.stack[vm.sp:]
	for i := range stackTail {
		stackTail[i] = nil
	}
	vm.stack = vm.stack[:vm.sp]
}

type enterFunc uint32

func (e enterFunc) exec(vm *vm) {
	// Input stack:
	//
	// callee
	// this
	// arg0
	// ...
	// argN
	// <- sp

	// Output stack:
	//
	// this <- sb
	// <- sp

	vm.newStash()
	offset := vm.args - int(e)
	vm.stash.values = make([]Value, e)
	if offset > 0 {
		copy(vm.stash.values, vm.stack[vm.sp-vm.args:])
		vm.stash.extraArgs = make([]Value, offset)
		copy(vm.stash.extraArgs, vm.stack[vm.sp-offset:])
	} else {
		copy(vm.stash.values, vm.stack[vm.sp-vm.args:])
		vv := vm.stash.values[vm.args:]
		for i := range vv {
			vv[i] = _undefined
		}
	}
	vm.sp -= vm.args
	vm.sb = vm.sp - 1
	vm.pc++
}

type _ret struct{}

var ret _ret

func (_ret) exec(vm *vm) {
	// callee -3
	// this -2
	// retval -1

	vm.stack[vm.sp-3] = vm.stack[vm.sp-1]
	vm.sp -= 2
	vm.popCtx()
	if vm.pc < 0 {
		vm.halt = true
	}
}

type enterFuncStashless struct {
	stackSize uint32
	args      uint32
}

func (e enterFuncStashless) exec(vm *vm) {
	vm.sb = vm.sp - vm.args - 1
	var ss int
	d := int(e.args) - vm.args
	if d > 0 {
		ss = int(e.stackSize) + d
		vm.args = int(e.args)
	} else {
		ss = int(e.stackSize)
	}
	sp := vm.sp
	if ss > 0 {
		vm.sp += int(ss)
		vm.stack.expand(vm.sp)
		s := vm.stack[sp:vm.sp]
		for i := range s {
			s[i] = _undefined
		}
	}
	vm.pc++
}

type _retStashless struct{}

var retStashless _retStashless

func (_retStashless) exec(vm *vm) {
	retval := vm.stack[vm.sp-1]
	vm.sp = vm.sb
	vm.stack[vm.sp-1] = retval
	vm.popCtx()
	if vm.pc < 0 {
		vm.halt = true
	}
}

type newFunc struct {
	prg    *Program
	name   unistring.String
	length uint32
	strict bool

	srcStart, srcEnd uint32
}

func (n *newFunc) exec(vm *vm) {
	obj := vm.r.newFunc(n.name, int(n.length), n.strict)
	obj.prg = n.prg
	obj.stash = vm.stash
	obj.src = n.prg.src.Source()[n.srcStart:n.srcEnd]
	vm.push(obj.val)
	vm.pc++
}

type bindName unistring.String

func (d bindName) exec(vm *vm) {
	name := unistring.String(d)
	if vm.stash != nil {
		vm.stash.createBinding(name)
	} else {
		vm.r.globalObject.self._putProp(name, _undefined, true, true, false)
	}
	vm.pc++
}

type jne int32

func (j jne) exec(vm *vm) {
	vm.sp--
	if !vm.stack[vm.sp].ToBoolean() {
		vm.pc += int(j)
	} else {
		vm.pc++
	}
}

type jeq int32

func (j jeq) exec(vm *vm) {
	vm.sp--
	if vm.stack[vm.sp].ToBoolean() {
		vm.pc += int(j)
	} else {
		vm.pc++
	}
}

type jeq1 int32

func (j jeq1) exec(vm *vm) {
	if vm.stack[vm.sp-1].ToBoolean() {
		vm.pc += int(j)
	} else {
		vm.pc++
	}
}

type jneq1 int32

func (j jneq1) exec(vm *vm) {
	if !vm.stack[vm.sp-1].ToBoolean() {
		vm.pc += int(j)
	} else {
		vm.pc++
	}
}

type _not struct{}

var not _not

func (_not) exec(vm *vm) {
	if vm.stack[vm.sp-1].ToBoolean() {
		vm.stack[vm.sp-1] = valueFalse
	} else {
		vm.stack[vm.sp-1] = valueTrue
	}
	vm.pc++
}

func toPrimitiveNumber(v Value) Value {
	if o, ok := v.(*Object); ok {
		return o.toPrimitiveNumber()
	}
	return v
}

func toPrimitive(v Value) Value {
	if o, ok := v.(*Object); ok {
		return o.toPrimitive()
	}
	return v
}

func cmp(px, py Value) Value {
	var ret bool
	var nx, ny float64

	if xs, ok := px.(valueString); ok {
		if ys, ok := py.(valueString); ok {
			ret = xs.compareTo(ys) < 0
			goto end
		}
	}

	if xi, ok := px.(valueInt); ok {
		if yi, ok := py.(valueInt); ok {
			ret = xi < yi
			goto end
		}
	}

	nx = px.ToFloat()
	ny = py.ToFloat()

	if math.IsNaN(nx) || math.IsNaN(ny) {
		return _undefined
	}

	ret = nx < ny

end:
	if ret {
		return valueTrue
	}
	return valueFalse

}

type _op_lt struct{}

var op_lt _op_lt

func (_op_lt) exec(vm *vm) {
	left := toPrimitiveNumber(vm.stack[vm.sp-2])
	right := toPrimitiveNumber(vm.stack[vm.sp-1])

	r := cmp(left, right)
	if r == _undefined {
		vm.stack[vm.sp-2] = valueFalse
	} else {
		vm.stack[vm.sp-2] = r
	}
	vm.sp--
	vm.pc++
}

type _op_lte struct{}

var op_lte _op_lte

func (_op_lte) exec(vm *vm) {
	left := toPrimitiveNumber(vm.stack[vm.sp-2])
	right := toPrimitiveNumber(vm.stack[vm.sp-1])

	r := cmp(right, left)
	if r == _undefined || r == valueTrue {
		vm.stack[vm.sp-2] = valueFalse
	} else {
		vm.stack[vm.sp-2] = valueTrue
	}

	vm.sp--
	vm.pc++
}

type _op_gt struct{}

var op_gt _op_gt

func (_op_gt) exec(vm *vm) {
	left := toPrimitiveNumber(vm.stack[vm.sp-2])
	right := toPrimitiveNumber(vm.stack[vm.sp-1])

	r := cmp(right, left)
	if r == _undefined {
		vm.stack[vm.sp-2] = valueFalse
	} else {
		vm.stack[vm.sp-2] = r
	}
	vm.sp--
	vm.pc++
}

type _op_gte struct{}

var op_gte _op_gte

func (_op_gte) exec(vm *vm) {
	left := toPrimitiveNumber(vm.stack[vm.sp-2])
	right := toPrimitiveNumber(vm.stack[vm.sp-1])

	r := cmp(left, right)
	if r == _undefined || r == valueTrue {
		vm.stack[vm.sp-2] = valueFalse
	} else {
		vm.stack[vm.sp-2] = valueTrue
	}

	vm.sp--
	vm.pc++
}

type _op_eq struct{}

var op_eq _op_eq

func (_op_eq) exec(vm *vm) {
	if vm.stack[vm.sp-2].Equals(vm.stack[vm.sp-1]) {
		vm.stack[vm.sp-2] = valueTrue
	} else {
		vm.stack[vm.sp-2] = valueFalse
	}
	vm.sp--
	vm.pc++
}

type _op_neq struct{}

var op_neq _op_neq

func (_op_neq) exec(vm *vm) {
	if vm.stack[vm.sp-2].Equals(vm.stack[vm.sp-1]) {
		vm.stack[vm.sp-2] = valueFalse
	} else {
		vm.stack[vm.sp-2] = valueTrue
	}
	vm.sp--
	vm.pc++
}

type _op_strict_eq struct{}

var op_strict_eq _op_strict_eq

func (_op_strict_eq) exec(vm *vm) {
	if vm.stack[vm.sp-2].StrictEquals(vm.stack[vm.sp-1]) {
		vm.stack[vm.sp-2] = valueTrue
	} else {
		vm.stack[vm.sp-2] = valueFalse
	}
	vm.sp--
	vm.pc++
}

type _op_strict_neq struct{}

var op_strict_neq _op_strict_neq

func (_op_strict_neq) exec(vm *vm) {
	if vm.stack[vm.sp-2].StrictEquals(vm.stack[vm.sp-1]) {
		vm.stack[vm.sp-2] = valueFalse
	} else {
		vm.stack[vm.sp-2] = valueTrue
	}
	vm.sp--
	vm.pc++
}

type _op_instanceof struct{}

var op_instanceof _op_instanceof

func (_op_instanceof) exec(vm *vm) {
	left := vm.stack[vm.sp-2]
	right := vm.r.toObject(vm.stack[vm.sp-1])

	if instanceOfOperator(left, right) {
		vm.stack[vm.sp-2] = valueTrue
	} else {
		vm.stack[vm.sp-2] = valueFalse
	}

	vm.sp--
	vm.pc++
}

type _op_in struct{}

var op_in _op_in

func (_op_in) exec(vm *vm) {
	left := vm.stack[vm.sp-2]
	right := vm.r.toObject(vm.stack[vm.sp-1])

	if right.hasProperty(left) {
		vm.stack[vm.sp-2] = valueTrue
	} else {
		vm.stack[vm.sp-2] = valueFalse
	}

	vm.sp--
	vm.pc++
}

type try struct {
	catchOffset   int32
	finallyOffset int32
	dynamic       bool
}

func (t try) exec(vm *vm) {
	o := vm.pc
	vm.pc++
	ex := vm.runTry()
	if ex != nil && t.catchOffset > 0 {
		// run the catch block (in try)
		vm.pc = o + int(t.catchOffset)
		// TODO: if ex.val is an Error, set the stack property
		if t.dynamic {
			vm.newStash()
			vm.stash.putByIdx(0, ex.val)
		} else {
			vm.push(ex.val)
		}
		ex = vm.runTry()
		if t.dynamic {
			vm.stash = vm.stash.outer
		}
	}

	if t.finallyOffset > 0 {
		pc := vm.pc
		// Run finally
		vm.pc = o + int(t.finallyOffset)
		vm.run()
		if vm.prg.code[vm.pc] == retFinally {
			vm.pc = pc
		} else {
			// break or continue out of finally, dropping exception
			ex = nil
		}
	}

	vm.halt = false

	if ex != nil {
		panic(ex)
	}
}

type _retFinally struct{}

var retFinally _retFinally

func (_retFinally) exec(vm *vm) {
	vm.pc++
}

type enterCatch unistring.String

func (varName enterCatch) exec(vm *vm) {
	vm.stash.names = map[unistring.String]uint32{
		unistring.String(varName): 0,
	}
	vm.pc++
}

type _throw struct{}

var throw _throw

func (_throw) exec(vm *vm) {
	panic(vm.stack[vm.sp-1])
}

type _new uint32

func (n _new) exec(vm *vm) {
	sp := vm.sp - int(n)
	obj := vm.stack[sp-1]
	ctor := vm.r.toConstructor(obj)
	vm.stack[sp-1] = ctor(vm.stack[sp:vm.sp], nil)
	vm.sp = sp
	vm.pc++
}

type _loadNewTarget struct{}

var loadNewTarget _loadNewTarget

func (_loadNewTarget) exec(vm *vm) {
	if t := vm.newTarget; t != nil {
		vm.push(t)
	} else {
		vm.push(_undefined)
	}
	vm.pc++
}

type _typeof struct{}

var typeof _typeof

func (_typeof) exec(vm *vm) {
	var r Value
	switch v := vm.stack[vm.sp-1].(type) {
	case valueUndefined, valueUnresolved:
		r = stringUndefined
	case valueNull:
		r = stringObjectC
	case *Object:
	repeat:
		switch s := v.self.(type) {
		case *funcObject, *nativeFuncObject, *boundFuncObject:
			r = stringFunction
		case *lazyObject:
			v.self = s.create(v)
			goto repeat
		default:
			r = stringObjectC
		}
	case valueBool:
		r = stringBoolean
	case valueString:
		r = stringString
	case valueInt, valueFloat:
		r = stringNumber
	case *Symbol:
		r = stringSymbol
	default:
		panic(fmt.Errorf("Unknown type: %T", v))
	}
	vm.stack[vm.sp-1] = r
	vm.pc++
}

type createArgs uint32

func (formalArgs createArgs) exec(vm *vm) {
	v := &Object{runtime: vm.r}
	args := &argumentsObject{}
	args.extensible = true
	args.prototype = vm.r.global.ObjectPrototype
	args.class = "Arguments"
	v.self = args
	args.val = v
	args.length = vm.args
	args.init()
	i := 0
	c := int(formalArgs)
	if vm.args < c {
		c = vm.args
	}
	for ; i < c; i++ {
		args._put(unistring.String(strconv.Itoa(i)), &mappedProperty{
			valueProperty: valueProperty{
				writable:     true,
				configurable: true,
				enumerable:   true,
			},
			v: &vm.stash.values[i],
		})
	}

	for _, v := range vm.stash.extraArgs {
		args._put(unistring.String(strconv.Itoa(i)), v)
		i++
	}

	args._putProp("callee", vm.stack[vm.sb-1], true, false, true)
	vm.push(v)
	vm.pc++
}

type createArgsStrict uint32

func (formalArgs createArgsStrict) exec(vm *vm) {
	args := vm.r.newBaseObject(vm.r.global.ObjectPrototype, "Arguments")
	i := 0
	c := int(formalArgs)
	if vm.args < c {
		c = vm.args
	}
	for _, v := range vm.stash.values[:c] {
		args._put(unistring.String(strconv.Itoa(i)), v)
		i++
	}

	for _, v := range vm.stash.extraArgs {
		args._put(unistring.String(strconv.Itoa(i)), v)
		i++
	}

	args._putProp("length", intToValue(int64(vm.args)), true, false, true)
	args._put("callee", vm.r.global.throwerProperty)
	args._put("caller", vm.r.global.throwerProperty)
	vm.push(args.val)
	vm.pc++
}

type _enterWith struct{}

var enterWith _enterWith

func (_enterWith) exec(vm *vm) {
	vm.newStash()
	vm.stash.obj = vm.stack[vm.sp-1].ToObject(vm.r)
	vm.sp--
	vm.pc++
}

type _leaveWith struct{}

var leaveWith _leaveWith

func (_leaveWith) exec(vm *vm) {
	vm.stash = vm.stash.outer
	vm.pc++
}

func emptyIter() (propIterItem, iterNextFunc) {
	return propIterItem{}, nil
}

type _enumerate struct{}

var enumerate _enumerate

func (_enumerate) exec(vm *vm) {
	v := vm.stack[vm.sp-1]
	if v == _undefined || v == _null {
		vm.iterStack = append(vm.iterStack, iterStackItem{f: emptyIter})
	} else {
		vm.iterStack = append(vm.iterStack, iterStackItem{f: v.ToObject(vm.r).self.enumerate()})
	}
	vm.sp--
	vm.pc++
}

type enumNext int32

func (jmp enumNext) exec(vm *vm) {
	l := len(vm.iterStack) - 1
	item, n := vm.iterStack[l].f()
	if n != nil {
		vm.iterStack[l].val = stringValueFromRaw(item.name)
		vm.iterStack[l].f = n
		vm.pc++
	} else {
		vm.pc += int(jmp)
	}
}

type _enumGet struct{}

var enumGet _enumGet

func (_enumGet) exec(vm *vm) {
	l := len(vm.iterStack) - 1
	vm.push(vm.iterStack[l].val)
	vm.pc++
}

type _enumPop struct{}

var enumPop _enumPop

func (_enumPop) exec(vm *vm) {
	l := len(vm.iterStack) - 1
	vm.iterStack[l] = iterStackItem{}
	vm.iterStack = vm.iterStack[:l]
	vm.pc++
}

type _iterate struct{}

var iterate _iterate

func (_iterate) exec(vm *vm) {
	iter := vm.r.getIterator(vm.stack[vm.sp-1], nil)
	vm.iterStack = append(vm.iterStack, iterStackItem{iter: iter})
	vm.sp--
	vm.pc++
}

type iterNext int32

func (jmp iterNext) exec(vm *vm) {
	l := len(vm.iterStack) - 1
	iter := vm.iterStack[l].iter
	res := vm.r.toObject(toMethod(iter.self.getStr("next", nil))(FunctionCall{This: iter}))
	if nilSafe(res.self.getStr("done", nil)).ToBoolean() {
		vm.pc += int(jmp)
	} else {
		vm.iterStack[l].val = nilSafe(res.self.getStr("value", nil))
		vm.pc++
	}
}
