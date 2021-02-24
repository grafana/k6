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
	values    []Value
	extraArgs []Value
	names     map[unistring.String]uint32
	obj       *Object

	outer *stash

	function bool
}

type context struct {
	prg       *Program
	funcName  unistring.String
	stash     *stash
	newTarget Value
	result    Value
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
	n   unistring.String
	v   *[]Value
	idx int
}

func (r *stashRef) get() Value {
	return nilSafe((*r.v)[r.idx])
}

func (r *stashRef) set(v Value) {
	(*r.v)[r.idx] = v
}

func (r *stashRef) refname() unistring.String {
	return r.n
}

type stashRefLex struct {
	stashRef
}

func (r *stashRefLex) get() Value {
	v := (*r.v)[r.idx]
	if v == nil {
		panic(errAccessBeforeInit)
	}
	return v
}

func (r *stashRefLex) set(v Value) {
	p := &(*r.v)[r.idx]
	if *p == nil {
		panic(errAccessBeforeInit)
	}
	*p = v
}

type stashRefConst struct {
	stashRefLex
}

func (r *stashRefConst) set(v Value) {
	panic(errAssignToConst)
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
	result    Value

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
	idx++
	if idx < cap(*s) {
		*s = (*s)[:idx]
	} else {
		var newCap int
		if idx < 1024 {
			newCap = idx * 2
		} else {
			newCap = (idx + 1025) &^ 1023
		}
		n := make([]Value, idx, newCap)
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

func (s *stash) initByIdx(idx uint32, v Value) {
	if s.obj != nil {
		panic("Attempt to init by idx into an object scope")
	}
	s.values[idx] = v
}

func (s *stash) initByName(name unistring.String, v Value) {
	if idx, exists := s.names[name]; exists {
		s.values[idx&^maskTyp] = v
	} else {
		panic(referenceError(fmt.Sprintf("%s is not defined", name)))
	}
}

func (s *stash) getByIdx(idx uint32) Value {
	return s.values[idx]
}

func (s *stash) getByName(name unistring.String) (v Value, exists bool) {
	if s.obj != nil {
		if stashObjHas(s.obj, name) {
			return nilSafe(s.obj.self.getStr(name, nil)), true
		}
		return nil, false
	}
	if idx, exists := s.names[name]; exists {
		v := s.values[idx&^maskTyp]
		if v == nil {
			if idx&maskVar == 0 {
				panic(errAccessBeforeInit)
			} else {
				v = _undefined
			}
		}
		return v, true
	}
	return nil, false
}

func (s *stash) getRefByName(name unistring.String, strict bool) ref {
	if obj := s.obj; obj != nil {
		if stashObjHas(obj, name) {
			return &objRef{
				base:   obj.self,
				name:   name,
				strict: strict,
			}
		}
	} else {
		if idx, exists := s.names[name]; exists {
			if idx&maskVar == 0 {
				if idx&maskConst == 0 {
					return &stashRefLex{
						stashRef: stashRef{
							n:   name,
							v:   &s.values,
							idx: int(idx &^ maskTyp),
						},
					}
				} else {
					return &stashRefConst{
						stashRefLex: stashRefLex{
							stashRef: stashRef{
								n:   name,
								v:   &s.values,
								idx: int(idx &^ maskTyp),
							},
						},
					}
				}
			} else {
				return &stashRef{
					n:   name,
					v:   &s.values,
					idx: int(idx &^ maskTyp),
				}
			}
		}
	}
	return nil
}

func (s *stash) createBinding(name unistring.String, deletable bool) {
	if s.names == nil {
		s.names = make(map[unistring.String]uint32)
	}
	if _, exists := s.names[name]; !exists {
		idx := uint32(len(s.names)) | maskVar
		if deletable {
			idx |= maskDeletable
		}
		s.names[name] = idx
		s.values = append(s.values, _undefined)
	}
}

func (s *stash) createLexBinding(name unistring.String, isConst bool) {
	if s.names == nil {
		s.names = make(map[unistring.String]uint32)
	}
	if _, exists := s.names[name]; !exists {
		idx := uint32(len(s.names))
		if isConst {
			idx |= maskConst
		}
		s.names[name] = idx
		s.values = append(s.values, nil)
	}
}

func (s *stash) deleteBinding(name unistring.String) {
	delete(s.names, name)
}

func (vm *vm) newStash() {
	vm.stash = &stash{
		outer: vm.stash,
	}
	vm.stashAllocs++
}

func (vm *vm) init() {
	vm.sb = -1
	vm.stash = &vm.r.global.stash
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
						vm.try(func() {
							returnIter(iter)
						})
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
			case referenceError:
				ex = &Exception{
					val: vm.r.newError(vm.r.global.ReferenceError, string(x1)),
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
	ctx.prg, ctx.stash, ctx.newTarget, ctx.result, ctx.pc, ctx.sb, ctx.args =
		vm.prg, vm.stash, vm.newTarget, vm.result, vm.pc, vm.sb, vm.args
	if vm.funcName != "" {
		ctx.funcName = vm.funcName
	} else if ctx.prg != nil && ctx.prg.funcName != "" {
		ctx.funcName = ctx.prg.funcName
	}
}

func (vm *vm) pushCtx() {
	vm.callStack = append(vm.callStack, context{})
	ctx := &vm.callStack[len(vm.callStack)-1]
	vm.saveCtx(ctx)
}

func (vm *vm) restoreCtx(ctx *context) {
	vm.prg, vm.funcName, vm.stash, vm.newTarget, vm.result, vm.pc, vm.sb, vm.args =
		ctx.prg, ctx.funcName, ctx.stash, ctx.newTarget, ctx.result, ctx.pc, ctx.sb, ctx.args
}

func (vm *vm) popCtx() {
	l := len(vm.callStack) - 1
	ctx := &vm.callStack[l]
	vm.restoreCtx(ctx)

	ctx.prg = nil
	ctx.stash = nil
	ctx.result = nil
	ctx.newTarget = nil

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

type _saveResult struct{}

var saveResult _saveResult

func (_saveResult) exec(vm *vm) {
	vm.sp--
	vm.result = vm.stack[vm.sp]
	vm.pc++
}

type _clearResult struct{}

var clearResult _clearResult

func (_clearResult) exec(vm *vm) {
	vm.result = _undefined
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
	// l > 0 -- var<l-1>
	// l == 0 -- this

	if l > 0 {
		vm.push(nilSafe(vm.stack[vm.sb+vm.args+int(l)]))
	} else {
		vm.push(vm.stack[vm.sb])
	}
	vm.pc++
}

type loadStack1 int

func (l loadStack1) exec(vm *vm) {
	// args are in stash
	// l > 0 -- var<l-1>
	// l == 0 -- this

	if l > 0 {
		vm.push(nilSafe(vm.stack[vm.sb+int(l)]))
	} else {
		vm.push(vm.stack[vm.sb])
	}
	vm.pc++
}

type loadStackLex int

func (l loadStackLex) exec(vm *vm) {
	// l < 0 -- arg<-l-1>
	// l > 0 -- var<l-1>
	var p *Value
	if l < 0 {
		arg := int(-l)
		if arg > vm.args {
			vm.push(_undefined)
			vm.pc++
			return
		} else {
			p = &vm.stack[vm.sb+arg]
		}
	} else {
		p = &vm.stack[vm.sb+vm.args+int(l)]
	}
	if *p == nil {
		panic(errAccessBeforeInit)
	}
	vm.push(*p)
	vm.pc++
}

type loadStack1Lex int

func (l loadStack1Lex) exec(vm *vm) {
	p := &vm.stack[vm.sb+int(l)]
	if *p == nil {
		panic(errAccessBeforeInit)
	}
	vm.push(*p)
	vm.pc++
}

type _loadCallee struct{}

var loadCallee _loadCallee

func (_loadCallee) exec(vm *vm) {
	vm.push(vm.stack[vm.sb-1])
	vm.pc++
}

func (vm *vm) storeStack(s int) {
	// l > 0 -- var<l-1>

	if s > 0 {
		vm.stack[vm.sb+vm.args+s] = vm.stack[vm.sp-1]
	} else {
		panic("Illegal stack var index")
	}
	vm.pc++
}

func (vm *vm) storeStack1(s int) {
	// args are in stash
	// l > 0 -- var<l-1>

	if s > 0 {
		vm.stack[vm.sb+s] = vm.stack[vm.sp-1]
	} else {
		panic("Illegal stack var index")
	}
	vm.pc++
}

func (vm *vm) storeStackLex(s int) {
	// l < 0 -- arg<-l-1>
	// l > 0 -- var<l-1>
	var p *Value
	if s < 0 {
		p = &vm.stack[vm.sb-s]
	} else {
		p = &vm.stack[vm.sb+vm.args+s]
	}

	if *p != nil {
		*p = vm.stack[vm.sp-1]
	} else {
		panic(errAccessBeforeInit)
	}
	vm.pc++
}

func (vm *vm) storeStack1Lex(s int) {
	// args are in stash
	// s > 0 -- var<l-1>
	if s <= 0 {
		panic("Illegal stack var index")
	}
	p := &vm.stack[vm.sb+s]
	if *p != nil {
		*p = vm.stack[vm.sp-1]
	} else {
		panic(errAccessBeforeInit)
	}
	vm.pc++
}

func (vm *vm) initStack(s int) {
	if s <= 0 {
		panic("Illegal stack var index")
	}
	vm.stack[vm.sb+vm.args+s] = vm.stack[vm.sp-1]
	vm.pc++
}

func (vm *vm) initStack1(s int) {
	if s <= 0 {
		panic("Illegal stack var index")
	}
	vm.stack[vm.sb+s] = vm.stack[vm.sp-1]
	vm.pc++
}

type storeStack int

func (s storeStack) exec(vm *vm) {
	vm.storeStack(int(s))
}

type storeStack1 int

func (s storeStack1) exec(vm *vm) {
	vm.storeStack1(int(s))
}

type storeStackLex int

func (s storeStackLex) exec(vm *vm) {
	vm.storeStackLex(int(s))
}

type storeStack1Lex int

func (s storeStack1Lex) exec(vm *vm) {
	vm.storeStack1Lex(int(s))
}

type initStack int

func (s initStack) exec(vm *vm) {
	vm.initStack(int(s))
	vm.sp--
}

type initStack1 int

func (s initStack1) exec(vm *vm) {
	vm.initStack1(int(s))
	vm.sp--
}

type storeStackP int

func (s storeStackP) exec(vm *vm) {
	vm.storeStack(int(s))
	vm.sp--
}

type storeStack1P int

func (s storeStack1P) exec(vm *vm) {
	vm.storeStack1(int(s))
	vm.sp--
}

type storeStackLexP int

func (s storeStackLexP) exec(vm *vm) {
	vm.storeStackLex(int(s))
	vm.sp--
}

type storeStack1LexP int

func (s storeStack1LexP) exec(vm *vm) {
	vm.storeStack1Lex(int(s))
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

type _setElemP struct{}

var setElemP _setElemP

func (_setElemP) exec(vm *vm) {
	obj := vm.stack[vm.sp-3].ToObject(vm.r)
	propName := toPropertyKey(vm.stack[vm.sp-2])
	val := vm.stack[vm.sp-1]

	obj.setOwn(propName, val, false)

	vm.sp -= 3
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

type _setElemStrictP struct{}

var setElemStrictP _setElemStrictP

func (_setElemStrictP) exec(vm *vm) {
	obj := vm.r.toObject(vm.stack[vm.sp-3])
	propName := toPropertyKey(vm.stack[vm.sp-2])
	val := vm.stack[vm.sp-1]

	obj.setOwn(propName, val, true)

	vm.sp -= 3
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

type setPropP unistring.String

func (p setPropP) exec(vm *vm) {
	val := vm.stack[vm.sp-1]
	vm.stack[vm.sp-2].ToObject(vm.r).self.setOwnStr(unistring.String(p), val, false)
	vm.sp -= 2
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

type setPropStrictP unistring.String

func (p setPropStrictP) exec(vm *vm) {
	obj := vm.r.toObject(vm.stack[vm.sp-2])
	val := vm.stack[vm.sp-1]

	obj.self.setOwnStr(unistring.String(p), val, true)
	vm.sp -= 2
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
	vm.push(vm.r.newRegExpp(n.pattern.clone(), n.src, vm.r.global.RegExpPrototype).val)
	vm.pc++
}

func (vm *vm) setLocalLex(s int) {
	v := vm.stack[vm.sp-1]
	level := s >> 24
	idx := uint32(s & 0x00FFFFFF)
	stash := vm.stash
	for i := 0; i < level; i++ {
		stash = stash.outer
	}
	p := &stash.values[idx]
	if *p == nil {
		panic(errAccessBeforeInit)
	}
	*p = v
	vm.pc++
}

func (vm *vm) initLocal(s int) {
	v := vm.stack[vm.sp-1]
	level := s >> 24
	idx := uint32(s & 0x00FFFFFF)
	stash := vm.stash
	for i := 0; i < level; i++ {
		stash = stash.outer
	}
	stash.initByIdx(idx, v)
	vm.pc++
}

type storeStash uint32

func (s storeStash) exec(vm *vm) {
	vm.initLocal(int(s))
}

type storeStashP uint32

func (s storeStashP) exec(vm *vm) {
	vm.initLocal(int(s))
	vm.sp--
}

type storeStashLex uint32

func (s storeStashLex) exec(vm *vm) {
	vm.setLocalLex(int(s))
}

type storeStashLexP uint32

func (s storeStashLexP) exec(vm *vm) {
	vm.setLocalLex(int(s))
	vm.sp--
}

type initStash uint32

func (s initStash) exec(vm *vm) {
	vm.initLocal(int(s))
	vm.sp--
}

type initGlobal unistring.String

func (s initGlobal) exec(vm *vm) {
	vm.sp--
	vm.r.global.stash.initByName(unistring.String(s), vm.stack[vm.sp])
	vm.pc++
}

type resolveVar1 unistring.String

func (s resolveVar1) exec(vm *vm) {
	name := unistring.String(s)
	var ref ref
	for stash := vm.stash; stash != nil; stash = stash.outer {
		ref = stash.getRefByName(name, false)
		if ref != nil {
			goto end
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
			if idx, exists := stash.names[name]; exists {
				if idx&(maskVar|maskDeletable) == maskVar|maskDeletable {
					stash.deleteBinding(name)
				} else {
					ret = false
				}
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
		if ret {
			delete(vm.r.global.varNames, name)
		}
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
		ref = stash.getRefByName(name, true)
		if ref != nil {
			goto end
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
	vm.r.setGlobal(unistring.String(s), vm.peek(), false)
	vm.pc++
}

type setGlobalStrict unistring.String

func (s setGlobalStrict) exec(vm *vm) {
	vm.r.setGlobal(unistring.String(s), vm.peek(), true)
	vm.pc++
}

// Load a var from stash
type loadStash uint32

func (g loadStash) exec(vm *vm) {
	level := int(g >> 24)
	idx := uint32(g & 0x00FFFFFF)
	stash := vm.stash
	for i := 0; i < level; i++ {
		stash = stash.outer
	}

	vm.push(nilSafe(stash.getByIdx(idx)))
	vm.pc++
}

// Load a lexical binding from stash
type loadStashLex uint32

func (g loadStashLex) exec(vm *vm) {
	level := int(g >> 24)
	idx := uint32(g & 0x00FFFFFF)
	stash := vm.stash
	for i := 0; i < level; i++ {
		stash = stash.outer
	}

	v := stash.getByIdx(idx)
	if v == nil {
		panic(errAccessBeforeInit)
	}
	vm.push(v)
	vm.pc++
}

// scan dynamic stashes up to the given level (encoded as 8 most significant bits of idx), if not found
// return the indexed var binding value from stash
type loadMixed struct {
	name   unistring.String
	idx    uint32
	callee bool
}

func (g *loadMixed) exec(vm *vm) {
	level := int(g.idx >> 24)
	idx := g.idx & 0x00FFFFFF
	stash := vm.stash
	name := g.name
	for i := 0; i < level; i++ {
		if v, found := stash.getByName(name); found {
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
		vm.push(nilSafe(stash.getByIdx(idx)))
	}
end:
	vm.pc++
}

// scan dynamic stashes up to the given level (encoded as 8 most significant bits of idx), if not found
// return the indexed lexical binding value from stash
type loadMixedLex loadMixed

func (g *loadMixedLex) exec(vm *vm) {
	level := int(g.idx >> 24)
	idx := g.idx & 0x00FFFFFF
	stash := vm.stash
	name := g.name
	for i := 0; i < level; i++ {
		if v, found := stash.getByName(name); found {
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
		v := stash.getByIdx(idx)
		if v == nil {
			panic(errAccessBeforeInit)
		}
		vm.push(v)
	}
end:
	vm.pc++
}

// scan dynamic stashes up to the given level (encoded as 8 most significant bits of idx), if not found
// return the indexed var binding value from stack
type loadMixedStack struct {
	name   unistring.String
	idx    int
	level  uint8
	callee bool
}

// same as loadMixedStack, but the args have been moved to stash (therefore stack layout is different)
type loadMixedStack1 loadMixedStack

func (g *loadMixedStack) exec(vm *vm) {
	stash := vm.stash
	name := g.name
	level := int(g.level)
	for i := 0; i < level; i++ {
		if v, found := stash.getByName(name); found {
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
	loadStack(g.idx).exec(vm)
	return
end:
	vm.pc++
}

func (g *loadMixedStack1) exec(vm *vm) {
	stash := vm.stash
	name := g.name
	level := int(g.level)
	for i := 0; i < level; i++ {
		if v, found := stash.getByName(name); found {
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
	loadStack1(g.idx).exec(vm)
	return
end:
	vm.pc++
}

type loadMixedStackLex loadMixedStack

// same as loadMixedStackLex but when the arguments have been moved into stash
type loadMixedStack1Lex loadMixedStack

func (g *loadMixedStackLex) exec(vm *vm) {
	stash := vm.stash
	name := g.name
	level := int(g.level)
	for i := 0; i < level; i++ {
		if v, found := stash.getByName(name); found {
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
	loadStackLex(g.idx).exec(vm)
	return
end:
	vm.pc++
}

func (g *loadMixedStack1Lex) exec(vm *vm) {
	stash := vm.stash
	name := g.name
	level := int(g.level)
	for i := 0; i < level; i++ {
		if v, found := stash.getByName(name); found {
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
	loadStack1Lex(g.idx).exec(vm)
	return
end:
	vm.pc++
}

type resolveMixed struct {
	name   unistring.String
	idx    uint32
	typ    varType
	strict bool
}

func newStashRef(typ varType, name unistring.String, v *[]Value, idx int) ref {
	switch typ {
	case varTypeVar:
		return &stashRef{
			n:   name,
			v:   v,
			idx: idx,
		}
	case varTypeLet:
		return &stashRefLex{
			stashRef: stashRef{
				n:   name,
				v:   v,
				idx: idx,
			},
		}
	case varTypeConst:
		return &stashRefConst{
			stashRefLex: stashRefLex{
				stashRef: stashRef{
					n:   name,
					v:   v,
					idx: idx,
				},
			},
		}
	}
	panic("unsupported var type")
}

func (r *resolveMixed) exec(vm *vm) {
	level := int(r.idx >> 24)
	idx := r.idx & 0x00FFFFFF
	stash := vm.stash
	var ref ref
	for i := 0; i < level; i++ {
		ref = stash.getRefByName(r.name, r.strict)
		if ref != nil {
			goto end
		}
		stash = stash.outer
	}

	if stash != nil {
		ref = newStashRef(r.typ, r.name, &stash.values, int(idx))
		goto end
	}

	ref = &unresolvedRef{
		runtime: vm.r,
		name:    r.name,
	}

end:
	vm.refStack = append(vm.refStack, ref)
	vm.pc++
}

type resolveMixedStack struct {
	name   unistring.String
	idx    int
	typ    varType
	level  uint8
	strict bool
}

type resolveMixedStack1 resolveMixedStack

func (r *resolveMixedStack) exec(vm *vm) {
	level := int(r.level)
	stash := vm.stash
	var ref ref
	var idx int
	for i := 0; i < level; i++ {
		ref = stash.getRefByName(r.name, r.strict)
		if ref != nil {
			goto end
		}
		stash = stash.outer
	}

	if r.idx > 0 {
		idx = vm.sb + vm.args + r.idx
	} else {
		idx = vm.sb + r.idx
	}

	ref = newStashRef(r.typ, r.name, (*[]Value)(&vm.stack), idx)

end:
	vm.refStack = append(vm.refStack, ref)
	vm.pc++
}

func (r *resolveMixedStack1) exec(vm *vm) {
	level := int(r.level)
	stash := vm.stash
	var ref ref
	for i := 0; i < level; i++ {
		ref = stash.getRefByName(r.name, r.strict)
		if ref != nil {
			goto end
		}
		stash = stash.outer
	}

	ref = newStashRef(r.typ, r.name, (*[]Value)(&vm.stack), vm.sb+r.idx)

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

type _putValueP struct{}

var putValueP _putValueP

func (_putValueP) exec(vm *vm) {
	l := len(vm.refStack) - 1
	ref := vm.refStack[l]
	vm.refStack[l] = nil
	vm.refStack = vm.refStack[:l]
	ref.set(vm.stack[vm.sp-1])
	vm.sp--
	vm.pc++
}

type loadDynamic unistring.String

func (n loadDynamic) exec(vm *vm) {
	name := unistring.String(n)
	var val Value
	for stash := vm.stash; stash != nil; stash = stash.outer {
		if v, exists := stash.getByName(name); exists {
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

type loadDynamicRef unistring.String

func (n loadDynamicRef) exec(vm *vm) {
	name := unistring.String(n)
	var val Value
	for stash := vm.stash; stash != nil; stash = stash.outer {
		if v, exists := stash.getByName(name); exists {
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

type loadDynamicCallee unistring.String

func (n loadDynamicCallee) exec(vm *vm) {
	name := unistring.String(n)
	var val Value
	var callee *Object
	for stash := vm.stash; stash != nil; stash = stash.outer {
		if v, exists := stash.getByName(name); exists {
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
				if vm.sb >= 0 {
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
	sp := vm.sp
	stackTail := vm.stack[sp:]
	for i := range stackTail {
		stackTail[i] = nil
	}
	vm.stack = vm.stack[:sp]
}

type enterBlock struct {
	names     map[unistring.String]uint32
	stashSize uint32
	stackSize uint32
}

func (e *enterBlock) exec(vm *vm) {
	if e.stashSize > 0 {
		vm.newStash()
		vm.stash.values = make([]Value, e.stashSize)
		if len(e.names) > 0 {
			vm.stash.names = e.names
		}
	}
	ss := int(e.stackSize)
	vm.stack.expand(vm.sp + ss - 1)
	vv := vm.stack[vm.sp : vm.sp+ss]
	for i := range vv {
		vv[i] = nil
	}
	vm.sp += ss
	vm.pc++
}

type enterCatchBlock struct {
	names     map[unistring.String]uint32
	stashSize uint32
	stackSize uint32
}

func (e *enterCatchBlock) exec(vm *vm) {
	vm.newStash()
	vm.stash.values = make([]Value, e.stashSize)
	if len(e.names) > 0 {
		vm.stash.names = e.names
	}
	vm.sp--
	vm.stash.values[0] = vm.stack[vm.sp]
	ss := int(e.stackSize)
	vm.stack.expand(vm.sp + ss - 1)
	vv := vm.stack[vm.sp : vm.sp+ss]
	for i := range vv {
		vv[i] = nil
	}
	vm.sp += ss
	vm.pc++
}

type leaveBlock struct {
	stackSize uint32
	popStash  bool
}

func (l *leaveBlock) exec(vm *vm) {
	if l.popStash {
		vm.stash = vm.stash.outer
	}
	if ss := l.stackSize; ss > 0 {
		vm.sp -= int(ss)
	}
	vm.pc++
}

type enterFunc struct {
	names       map[unistring.String]uint32
	stashSize   uint32
	stackSize   uint32
	numArgs     uint32
	argsToStash bool
	extensible  bool
}

func (e *enterFunc) exec(vm *vm) {
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
	// <local stack vars...>
	// <- sp
	sp := vm.sp
	vm.sb = sp - vm.args - 1
	vm.newStash()
	stash := vm.stash
	stash.function = true
	stash.values = make([]Value, e.stashSize)
	if len(e.names) > 0 {
		if e.extensible {
			m := make(map[unistring.String]uint32, len(e.names))
			for name, idx := range e.names {
				m[name] = idx
			}
			stash.names = m
		} else {
			stash.names = e.names
		}
	}

	ss := int(e.stackSize)
	ea := 0
	if e.argsToStash {
		offset := vm.args - int(e.numArgs)
		copy(stash.values, vm.stack[sp-vm.args:sp])
		if offset > 0 {
			vm.stash.extraArgs = make([]Value, offset)
			copy(stash.extraArgs, vm.stack[sp-offset:])
		} else {
			vv := stash.values[vm.args:e.numArgs]
			for i := range vv {
				vv[i] = _undefined
			}
		}
		sp -= vm.args
	} else {
		d := int(e.numArgs) - vm.args
		if d > 0 {
			ss += d
			ea = d
			vm.args = int(e.numArgs)
		}
	}
	vm.stack.expand(sp + ss - 1)
	if ea > 0 {
		vv := vm.stack[sp : vm.sp+ea]
		for i := range vv {
			vv[i] = _undefined
		}
	}
	vv := vm.stack[sp+ea : sp+ss]
	for i := range vv {
		vv[i] = nil
	}
	vm.sp = sp + ss
	vm.pc++
}

type _ret struct{}

var ret _ret

func (_ret) exec(vm *vm) {
	// callee -3
	// this -2 <- sb
	// retval -1

	vm.stack[vm.sb-1] = vm.stack[vm.sp-1]
	vm.sp = vm.sb
	vm.popCtx()
	if vm.pc < 0 {
		vm.halt = true
	}
}

type enterFuncStashless struct {
	stackSize uint32
	args      uint32
}

func (e *enterFuncStashless) exec(vm *vm) {
	sp := vm.sp
	vm.sb = sp - vm.args - 1
	d := int(e.args) - vm.args
	if d > 0 {
		ss := sp + int(e.stackSize) + d
		vm.stack.expand(ss)
		vv := vm.stack[sp : sp+d]
		for i := range vv {
			vv[i] = _undefined
		}
		vv = vm.stack[sp+d : ss]
		for i := range vv {
			vv[i] = nil
		}
		vm.args = int(e.args)
		vm.sp = ss
	} else {
		if e.stackSize > 0 {
			ss := sp + int(e.stackSize)
			vm.stack.expand(ss)
			vv := vm.stack[sp:ss]
			for i := range vv {
				vv[i] = nil
			}
			vm.sp = ss
		}
	}
	vm.pc++
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

func (vm *vm) alreadyDeclared(name unistring.String) Value {
	return vm.r.newError(vm.r.global.SyntaxError, "Identifier '%s' has already been declared", name)
}

func (vm *vm) checkBindVarsGlobal(names []unistring.String) {
	o := vm.r.globalObject.self
	sn := vm.r.global.stash.names
	if o, ok := o.(*baseObject); ok {
		// shortcut
		for _, name := range names {
			if !o.hasOwnPropertyStr(name) && !o.extensible {
				panic(vm.r.NewTypeError("Cannot define global variable '%s', global object is not extensible", name))
			}
			if _, exists := sn[name]; exists {
				panic(vm.alreadyDeclared(name))
			}
		}
	} else {
		for _, name := range names {
			if !o.hasOwnPropertyStr(name) && !o.isExtensible() {
				panic(vm.r.NewTypeError("Cannot define global variable '%s', global object is not extensible", name))
			}
			if _, exists := sn[name]; exists {
				panic(vm.alreadyDeclared(name))
			}
		}
	}
}

func (vm *vm) createGlobalVarBindings(names []unistring.String, d bool) {
	globalVarNames := vm.r.global.varNames
	if globalVarNames == nil {
		globalVarNames = make(map[unistring.String]struct{})
		vm.r.global.varNames = globalVarNames
	}
	o := vm.r.globalObject.self
	if o, ok := o.(*baseObject); ok {
		for _, name := range names {
			if !o.hasOwnPropertyStr(name) && o.extensible {
				o._putProp(name, _undefined, true, true, d)
			}
			globalVarNames[name] = struct{}{}
		}
	} else {
		var cf Flag
		if d {
			cf = FLAG_TRUE
		} else {
			cf = FLAG_FALSE
		}
		for _, name := range names {
			if !o.hasOwnPropertyStr(name) && o.isExtensible() {
				o.defineOwnPropertyStr(name, PropertyDescriptor{
					Value:        _undefined,
					Writable:     FLAG_TRUE,
					Enumerable:   FLAG_TRUE,
					Configurable: cf,
				}, true)
				o.setOwnStr(name, _undefined, false)
			}
			globalVarNames[name] = struct{}{}
		}
	}
}

func (vm *vm) createGlobalFuncBindings(names []unistring.String, d bool) {
	globalVarNames := vm.r.global.varNames
	if globalVarNames == nil {
		globalVarNames = make(map[unistring.String]struct{})
		vm.r.global.varNames = globalVarNames
	}
	o := vm.r.globalObject.self
	b := vm.sp - len(names)
	var shortcutObj *baseObject
	if o, ok := o.(*baseObject); ok {
		shortcutObj = o
	}
	for i, name := range names {
		var desc PropertyDescriptor
		prop := o.getOwnPropStr(name)
		desc.Value = vm.stack[b+i]
		if shortcutObj != nil && prop == nil && shortcutObj.extensible {
			shortcutObj._putProp(name, desc.Value, true, true, d)
		} else {
			if prop, ok := prop.(*valueProperty); ok && !prop.configurable {
				// no-op
			} else {
				desc.Writable = FLAG_TRUE
				desc.Enumerable = FLAG_TRUE
				if d {
					desc.Configurable = FLAG_TRUE
				} else {
					desc.Configurable = FLAG_FALSE
				}
			}
			if shortcutObj != nil {
				shortcutObj.defineOwnPropertyStr(name, desc, true)
			} else {
				o.defineOwnPropertyStr(name, desc, true)
				o.setOwnStr(name, desc.Value, false) // not a bug, see https://262.ecma-international.org/#sec-createglobalfunctionbinding
			}
		}
		globalVarNames[name] = struct{}{}
	}
	vm.sp = b
}

func (vm *vm) checkBindFuncsGlobal(names []unistring.String) {
	o := vm.r.globalObject.self
	sn := vm.r.global.stash.names
	for _, name := range names {
		if _, exists := sn[name]; exists {
			panic(vm.alreadyDeclared(name))
		}
		prop := o.getOwnPropStr(name)
		allowed := true
		switch prop := prop.(type) {
		case nil:
			allowed = o.isExtensible()
		case *valueProperty:
			allowed = prop.configurable || prop.getterFunc == nil && prop.setterFunc == nil && prop.writable && prop.enumerable
		}
		if !allowed {
			panic(vm.r.NewTypeError("Cannot redefine global function '%s'", name))
		}
	}
}

func (vm *vm) checkBindLexGlobal(names []unistring.String) {
	o := vm.r.globalObject.self
	s := &vm.r.global.stash
	for _, name := range names {
		if _, exists := vm.r.global.varNames[name]; exists {
			goto fail
		}
		if _, exists := s.names[name]; exists {
			goto fail
		}
		if prop, ok := o.getOwnPropStr(name).(*valueProperty); ok && !prop.configurable {
			goto fail
		}
		continue
	fail:
		panic(vm.alreadyDeclared(name))
	}
}

type bindVars struct {
	names     []unistring.String
	deletable bool
}

func (d *bindVars) exec(vm *vm) {
	var target *stash
	for _, name := range d.names {
		for s := vm.stash; s != nil; s = s.outer {
			if idx, exists := s.names[name]; exists && idx&maskVar == 0 {
				panic(vm.alreadyDeclared(name))
			}
			if s.function {
				target = s
				break
			}
		}
	}
	if target == nil {
		target = vm.stash
	}
	deletable := d.deletable
	for _, name := range d.names {
		target.createBinding(name, deletable)
	}
	vm.pc++
}

type bindGlobal struct {
	vars, funcs, lets, consts []unistring.String

	deletable bool
}

func (b *bindGlobal) exec(vm *vm) {
	vm.checkBindFuncsGlobal(b.funcs)
	vm.checkBindLexGlobal(b.lets)
	vm.checkBindLexGlobal(b.consts)
	vm.checkBindVarsGlobal(b.vars)

	s := &vm.r.global.stash
	for _, name := range b.lets {
		s.createLexBinding(name, false)
	}
	for _, name := range b.consts {
		s.createLexBinding(name, true)
	}
	vm.createGlobalFuncBindings(b.funcs, b.deletable)
	vm.createGlobalVarBindings(b.vars, b.deletable)
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
}

func (t try) exec(vm *vm) {
	o := vm.pc
	vm.pc++
	ex := vm.runTry()
	if ex != nil && t.catchOffset > 0 {
		// run the catch block (in try)
		vm.pc = o + int(t.catchOffset)
		// TODO: if ex.val is an Error, set the stack property
		vm.push(ex.val)
		ex = vm.runTry()
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
	args._putSym(SymIterator, valueProp(vm.r.global.arrayValues, true, false, true))
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
	args._putSym(SymIterator, valueProp(vm.r.global.arrayValues, true, false, true))
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
		vm.iterStack = append(vm.iterStack, iterStackItem{f: enumerateRecursive(v.ToObject(vm.r))})
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

type _enumPopClose struct{}

var enumPopClose _enumPopClose

func (_enumPopClose) exec(vm *vm) {
	l := len(vm.iterStack) - 1
	if iter := vm.iterStack[l].iter; iter != nil {
		returnIter(iter)
	}
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
	var res *Object
	var done bool
	var value Value
	ex := vm.try(func() {
		res = vm.r.toObject(toMethod(iter.self.getStr("next", nil))(FunctionCall{This: iter}))
		done = nilSafe(res.self.getStr("done", nil)).ToBoolean()
		if !done {
			value = nilSafe(res.self.getStr("value", nil))
			vm.iterStack[l].val = value
		}
	})
	if ex == nil {
		if done {
			vm.pc += int(jmp)
		} else {
			vm.iterStack[l].val = value
			vm.pc++
		}
	} else {
		l := len(vm.iterStack) - 1
		vm.iterStack[l] = iterStackItem{}
		vm.iterStack = vm.iterStack[:l]
		panic(ex.val)
	}
}

type copyStash struct{}

func (copyStash) exec(vm *vm) {
	oldStash := vm.stash
	newStash := &stash{
		outer: oldStash.outer,
	}
	vm.stashAllocs++
	newStash.values = append([]Value(nil), oldStash.values...)
	vm.stash = newStash
	vm.pc++
}

type _throwAssignToConst struct{}

var throwAssignToConst _throwAssignToConst

func (_throwAssignToConst) exec(vm *vm) {
	panic(errAssignToConst)
}
