package goja

import (
	"fmt"
	"github.com/dop251/goja/token"
	"sort"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
	"github.com/dop251/goja/unistring"
)

type blockType int

const (
	blockLoop blockType = iota
	blockLoopEnum
	blockTry
	blockLabel
	blockSwitch
	blockWith
	blockScope
	blockIterScope
	blockOptChain
)

const (
	maskConst     = 1 << 31
	maskVar       = 1 << 30
	maskDeletable = 1 << 29
	maskStrict    = maskDeletable

	maskTyp = maskConst | maskVar | maskDeletable
)

type varType byte

const (
	varTypeVar varType = iota
	varTypeLet
	varTypeStrictConst
	varTypeConst
)

const thisBindingName = " this" // must not be a valid identifier

type CompilerError struct {
	Message string
	File    *file.File
	Offset  int
}

type CompilerSyntaxError struct {
	CompilerError
}

type CompilerReferenceError struct {
	CompilerError
}

type srcMapItem struct {
	pc     int
	srcPos int
}

type Program struct {
	code   []instruction
	values []Value

	funcName unistring.String
	src      *file.File
	srcMap   []srcMapItem
}

type compiler struct {
	p     *Program
	scope *scope
	block *block

	classScope *classScope

	enumGetExpr compiledEnumGetExpr

	evalVM *vm // VM used to evaluate constant expressions
	ctxVM  *vm // VM in which an eval() code is compiled

	codeScratchpad []instruction
}

type binding struct {
	scope        *scope
	name         unistring.String
	accessPoints map[*scope]*[]int
	isConst      bool
	isStrict     bool
	isArg        bool
	isVar        bool
	inStash      bool
}

func (b *binding) getAccessPointsForScope(s *scope) *[]int {
	m := b.accessPoints[s]
	if m == nil {
		a := make([]int, 0, 1)
		m = &a
		if b.accessPoints == nil {
			b.accessPoints = make(map[*scope]*[]int)
		}
		b.accessPoints[s] = m
	}
	return m
}

func (b *binding) markAccessPointAt(pos int) {
	scope := b.scope.c.scope
	m := b.getAccessPointsForScope(scope)
	*m = append(*m, pos-scope.base)
}

func (b *binding) markAccessPointAtScope(scope *scope, pos int) {
	m := b.getAccessPointsForScope(scope)
	*m = append(*m, pos-scope.base)
}

func (b *binding) markAccessPoint() {
	scope := b.scope.c.scope
	m := b.getAccessPointsForScope(scope)
	*m = append(*m, len(scope.prg.code)-scope.base)
}

func (b *binding) emitGet() {
	b.markAccessPoint()
	if b.isVar && !b.isArg {
		b.scope.c.emit(loadStack(0))
	} else {
		b.scope.c.emit(loadStackLex(0))
	}
}

func (b *binding) emitGetAt(pos int) {
	b.markAccessPointAt(pos)
	if b.isVar && !b.isArg {
		b.scope.c.p.code[pos] = loadStack(0)
	} else {
		b.scope.c.p.code[pos] = loadStackLex(0)
	}
}

func (b *binding) emitGetP() {
	if b.isVar && !b.isArg {
		// no-op
	} else {
		// make sure TDZ is checked
		b.markAccessPoint()
		b.scope.c.emit(loadStackLex(0), pop)
	}
}

func (b *binding) emitSet() {
	if b.isConst {
		if b.isStrict || b.scope.c.scope.strict {
			b.scope.c.emit(throwAssignToConst)
		}
		return
	}
	b.markAccessPoint()
	if b.isVar && !b.isArg {
		b.scope.c.emit(storeStack(0))
	} else {
		b.scope.c.emit(storeStackLex(0))
	}
}

func (b *binding) emitSetP() {
	if b.isConst {
		if b.isStrict || b.scope.c.scope.strict {
			b.scope.c.emit(throwAssignToConst)
		}
		return
	}
	b.markAccessPoint()
	if b.isVar && !b.isArg {
		b.scope.c.emit(storeStackP(0))
	} else {
		b.scope.c.emit(storeStackLexP(0))
	}
}

func (b *binding) emitInitP() {
	if !b.isVar && b.scope.outer == nil {
		b.scope.c.emit(initGlobalP(b.name))
	} else {
		b.markAccessPoint()
		b.scope.c.emit(initStackP(0))
	}
}

func (b *binding) emitInit() {
	if !b.isVar && b.scope.outer == nil {
		b.scope.c.emit(initGlobal(b.name))
	} else {
		b.markAccessPoint()
		b.scope.c.emit(initStack(0))
	}
}

func (b *binding) emitInitAt(pos int) {
	if !b.isVar && b.scope.outer == nil {
		b.scope.c.p.code[pos] = initGlobal(b.name)
	} else {
		b.markAccessPointAt(pos)
		b.scope.c.p.code[pos] = initStack(0)
	}
}

func (b *binding) emitInitAtScope(scope *scope, pos int) {
	if !b.isVar && scope.outer == nil {
		scope.c.p.code[pos] = initGlobal(b.name)
	} else {
		b.markAccessPointAtScope(scope, pos)
		scope.c.p.code[pos] = initStack(0)
	}
}

func (b *binding) emitInitPAtScope(scope *scope, pos int) {
	if !b.isVar && scope.outer == nil {
		scope.c.p.code[pos] = initGlobalP(b.name)
	} else {
		b.markAccessPointAtScope(scope, pos)
		scope.c.p.code[pos] = initStackP(0)
	}
}

func (b *binding) emitGetVar(callee bool) {
	b.markAccessPoint()
	if b.isVar && !b.isArg {
		b.scope.c.emit(&loadMixed{name: b.name, callee: callee})
	} else {
		b.scope.c.emit(&loadMixedLex{name: b.name, callee: callee})
	}
}

func (b *binding) emitResolveVar(strict bool) {
	b.markAccessPoint()
	if b.isVar && !b.isArg {
		b.scope.c.emit(&resolveMixed{name: b.name, strict: strict, typ: varTypeVar})
	} else {
		var typ varType
		if b.isConst {
			if b.isStrict {
				typ = varTypeStrictConst
			} else {
				typ = varTypeConst
			}
		} else {
			typ = varTypeLet
		}
		b.scope.c.emit(&resolveMixed{name: b.name, strict: strict, typ: typ})
	}
}

func (b *binding) moveToStash() {
	if b.isArg && !b.scope.argsInStash {
		b.scope.moveArgsToStash()
	} else {
		b.inStash = true
		b.scope.needStash = true
	}
}

func (b *binding) useCount() (count int) {
	for _, a := range b.accessPoints {
		count += len(*a)
	}
	return
}

type scope struct {
	c          *compiler
	prg        *Program
	outer      *scope
	nested     []*scope
	boundNames map[unistring.String]*binding
	bindings   []*binding
	base       int
	numArgs    int

	// function type. If not funcNone, this is a function or a top-level lexical environment
	funcType funcType

	// in strict mode
	strict bool
	// eval top-level scope
	eval bool
	// at least one inner scope has direct eval() which can lookup names dynamically (by name)
	dynLookup bool
	// at least one binding has been marked for placement in stash
	needStash bool

	// is a variable environment, i.e. the target for dynamically created var bindings
	variable bool
	// a function scope that has at least one direct eval() and non-strict, so the variables can be added dynamically
	dynamic bool
	// arguments have been marked for placement in stash (functions only)
	argsInStash bool
	// need 'arguments' object (functions only)
	argsNeeded bool
}

type block struct {
	typ        blockType
	label      unistring.String
	cont       int
	breaks     []int
	conts      []int
	outer      *block
	breaking   *block // set when the 'finally' block is an empty break statement sequence
	needResult bool
}

func (c *compiler) leaveScopeBlock(enter *enterBlock) {
	c.updateEnterBlock(enter)
	leave := &leaveBlock{
		stackSize: enter.stackSize,
		popStash:  enter.stashSize > 0,
	}
	c.emit(leave)
	for _, pc := range c.block.breaks {
		c.p.code[pc] = leave
	}
	c.block.breaks = nil
	c.leaveBlock()
}

func (c *compiler) leaveBlock() {
	lbl := len(c.p.code)
	for _, item := range c.block.breaks {
		c.p.code[item] = jump(lbl - item)
	}
	if t := c.block.typ; t == blockLoop || t == blockLoopEnum {
		for _, item := range c.block.conts {
			c.p.code[item] = jump(c.block.cont - item)
		}
	}
	c.block = c.block.outer
}

func (e *CompilerSyntaxError) Error() string {
	if e.File != nil {
		return fmt.Sprintf("SyntaxError: %s at %s", e.Message, e.File.Position(e.Offset))
	}
	return fmt.Sprintf("SyntaxError: %s", e.Message)
}

func (e *CompilerReferenceError) Error() string {
	return fmt.Sprintf("ReferenceError: %s", e.Message)
}

func (c *compiler) newScope() {
	strict := false
	if c.scope != nil {
		strict = c.scope.strict
	}
	c.scope = &scope{
		c:      c,
		prg:    c.p,
		outer:  c.scope,
		strict: strict,
	}
}

func (c *compiler) newBlockScope() {
	c.newScope()
	if outer := c.scope.outer; outer != nil {
		outer.nested = append(outer.nested, c.scope)
	}
	c.scope.base = len(c.p.code)
}

func (c *compiler) popScope() {
	c.scope = c.scope.outer
}

func newCompiler() *compiler {
	c := &compiler{
		p: &Program{},
	}

	c.enumGetExpr.init(c, file.Idx(0))

	return c
}

func (p *Program) defineLiteralValue(val Value) uint32 {
	for idx, v := range p.values {
		if v.SameAs(val) {
			return uint32(idx)
		}
	}
	idx := uint32(len(p.values))
	p.values = append(p.values, val)
	return idx
}

func (p *Program) dumpCode(logger func(format string, args ...interface{})) {
	p._dumpCode("", logger)
}

func (p *Program) _dumpCode(indent string, logger func(format string, args ...interface{})) {
	logger("values: %+v", p.values)
	dumpInitFields := func(initFields *Program) {
		i := indent + ">"
		logger("%s ---- init_fields:", i)
		initFields._dumpCode(i, logger)
		logger("%s ----", i)
	}
	for pc, ins := range p.code {
		logger("%s %d: %T(%v)", indent, pc, ins, ins)
		var prg *Program
		switch f := ins.(type) {
		case *newFunc:
			prg = f.prg
		case *newArrowFunc:
			prg = f.prg
		case *newMethod:
			prg = f.prg
		case *newDerivedClass:
			if f.initFields != nil {
				dumpInitFields(f.initFields)
			}
			prg = f.ctor
		case *newClass:
			if f.initFields != nil {
				dumpInitFields(f.initFields)
			}
			prg = f.ctor
		case *newStaticFieldInit:
			if f.initFields != nil {
				dumpInitFields(f.initFields)
			}
		}
		if prg != nil {
			prg._dumpCode(indent+">", logger)
		}
	}
}

func (p *Program) sourceOffset(pc int) int {
	i := sort.Search(len(p.srcMap), func(idx int) bool {
		return p.srcMap[idx].pc > pc
	}) - 1
	if i >= 0 {
		return p.srcMap[i].srcPos
	}

	return 0
}

func (p *Program) addSrcMap(srcPos int) {
	if len(p.srcMap) > 0 && p.srcMap[len(p.srcMap)-1].srcPos == srcPos {
		return
	}
	p.srcMap = append(p.srcMap, srcMapItem{pc: len(p.code), srcPos: srcPos})
}

func (s *scope) lookupName(name unistring.String) (binding *binding, noDynamics bool) {
	noDynamics = true
	toStash := false
	for curScope := s; ; curScope = curScope.outer {
		if curScope.outer != nil {
			if b, exists := curScope.boundNames[name]; exists {
				if toStash && !b.inStash {
					b.moveToStash()
				}
				binding = b
				return
			}
		} else {
			noDynamics = false
			return
		}
		if curScope.dynamic {
			noDynamics = false
		}
		if name == "arguments" && curScope.funcType != funcNone && curScope.funcType != funcArrow {
			if curScope.funcType == funcClsInit {
				s.c.throwSyntaxError(0, "'arguments' is not allowed in class field initializer or static initialization block")
			}
			curScope.argsNeeded = true
			binding, _ = curScope.bindName(name)
			return
		}
		if curScope.isFunction() {
			toStash = true
		}
	}
}

func (s *scope) lookupThis() (*binding, bool) {
	toStash := false
	for curScope := s; curScope != nil; curScope = curScope.outer {
		if curScope.outer == nil {
			if curScope.eval {
				return nil, true
			}
		}
		if b, exists := curScope.boundNames[thisBindingName]; exists {
			if toStash && !b.inStash {
				b.moveToStash()
			}
			return b, false
		}
		if curScope.isFunction() {
			toStash = true
		}
	}
	return nil, false
}

func (s *scope) ensureBoundNamesCreated() {
	if s.boundNames == nil {
		s.boundNames = make(map[unistring.String]*binding)
	}
}

func (s *scope) addBinding(offset int) *binding {
	if len(s.bindings) >= (1<<24)-1 {
		s.c.throwSyntaxError(offset, "Too many variables")
	}
	b := &binding{
		scope: s,
	}
	s.bindings = append(s.bindings, b)
	return b
}

func (s *scope) bindNameLexical(name unistring.String, unique bool, offset int) (*binding, bool) {
	if b := s.boundNames[name]; b != nil {
		if unique {
			s.c.throwSyntaxError(offset, "Identifier '%s' has already been declared", name)
		}
		return b, false
	}
	b := s.addBinding(offset)
	b.name = name
	s.ensureBoundNamesCreated()
	s.boundNames[name] = b
	return b, true
}

func (s *scope) createThisBinding() *binding {
	thisBinding, _ := s.bindNameLexical(thisBindingName, false, 0)
	thisBinding.isVar = true // don't check on load
	return thisBinding
}

func (s *scope) bindName(name unistring.String) (*binding, bool) {
	if !s.isFunction() && !s.variable && s.outer != nil {
		return s.outer.bindName(name)
	}
	b, created := s.bindNameLexical(name, false, 0)
	if created {
		b.isVar = true
	}
	return b, created
}

func (s *scope) bindNameShadow(name unistring.String) (*binding, bool) {
	if !s.isFunction() && s.outer != nil {
		return s.outer.bindNameShadow(name)
	}

	_, exists := s.boundNames[name]
	b := &binding{
		scope: s,
		name:  name,
	}
	s.bindings = append(s.bindings, b)
	s.ensureBoundNamesCreated()
	s.boundNames[name] = b
	return b, !exists
}

func (s *scope) nearestFunction() *scope {
	for sc := s; sc != nil; sc = sc.outer {
		if sc.isFunction() {
			return sc
		}
	}
	return nil
}

func (s *scope) nearestThis() *scope {
	for sc := s; sc != nil; sc = sc.outer {
		if sc.eval || sc.isFunction() && sc.funcType != funcArrow {
			return sc
		}
	}
	return nil
}

func (s *scope) finaliseVarAlloc(stackOffset int) (stashSize, stackSize int) {
	argsInStash := false
	if f := s.nearestFunction(); f != nil {
		argsInStash = f.argsInStash
	}
	stackIdx, stashIdx := 0, 0
	allInStash := s.isDynamic()
	var derivedCtor bool
	if fs := s.nearestThis(); fs != nil && fs.funcType == funcDerivedCtor {
		derivedCtor = true
	}
	for i, b := range s.bindings {
		var this bool
		if b.name == thisBindingName {
			this = true
		}
		if allInStash || b.inStash {
			for scope, aps := range b.accessPoints {
				var level uint32
				for sc := scope; sc != nil && sc != s; sc = sc.outer {
					if sc.needStash || sc.isDynamic() {
						level++
					}
				}
				if level > 255 {
					s.c.throwSyntaxError(0, "Maximum nesting level (256) exceeded")
				}
				idx := (level << 24) | uint32(stashIdx)
				base := scope.base
				code := scope.prg.code
				if this {
					if derivedCtor {
						for _, pc := range *aps {
							ap := &code[base+pc]
							switch (*ap).(type) {
							case loadStack:
								*ap = loadThisStash(idx)
							case initStack:
								*ap = initStash(idx)
							case resolveThisStack:
								*ap = resolveThisStash(idx)
							case _ret:
								*ap = cret(idx)
							default:
								s.c.assert(false, s.c.p.sourceOffset(pc), "Unsupported instruction for 'this'")
							}
						}
					} else {
						for _, pc := range *aps {
							ap := &code[base+pc]
							switch (*ap).(type) {
							case loadStack:
								*ap = loadStash(idx)
							case initStack:
								*ap = initStash(idx)
							default:
								s.c.assert(false, s.c.p.sourceOffset(pc), "Unsupported instruction for 'this'")
							}
						}
					}
				} else {
					for _, pc := range *aps {
						ap := &code[base+pc]
						switch i := (*ap).(type) {
						case loadStack:
							*ap = loadStash(idx)
						case storeStack:
							*ap = storeStash(idx)
						case storeStackP:
							*ap = storeStashP(idx)
						case loadStackLex:
							*ap = loadStashLex(idx)
						case storeStackLex:
							*ap = storeStashLex(idx)
						case storeStackLexP:
							*ap = storeStashLexP(idx)
						case initStackP:
							*ap = initStashP(idx)
						case initStack:
							*ap = initStash(idx)
						case *loadMixed:
							i.idx = idx
						case *loadMixedLex:
							i.idx = idx
						case *resolveMixed:
							i.idx = idx
						default:
							s.c.assert(false, s.c.p.sourceOffset(pc), "Unsupported instruction for binding: %T", i)
						}
					}
				}
			}
			stashIdx++
		} else {
			var idx int
			if !this {
				if i < s.numArgs {
					idx = -(i + 1)
				} else {
					stackIdx++
					idx = stackIdx + stackOffset
				}
			}
			for scope, aps := range b.accessPoints {
				var level int
				for sc := scope; sc != nil && sc != s; sc = sc.outer {
					if sc.needStash || sc.isDynamic() {
						level++
					}
				}
				if level > 255 {
					s.c.throwSyntaxError(0, "Maximum nesting level (256) exceeded")
				}
				code := scope.prg.code
				base := scope.base
				if this {
					if derivedCtor {
						for _, pc := range *aps {
							ap := &code[base+pc]
							switch (*ap).(type) {
							case loadStack:
								*ap = loadThisStack{}
							case initStack:
								// no-op
							case resolveThisStack:
								// no-op
							case _ret:
								// no-op, already in the right place
							default:
								s.c.assert(false, s.c.p.sourceOffset(pc), "Unsupported instruction for 'this'")
							}
						}
					} /*else {
						no-op
					}*/
				} else if argsInStash {
					for _, pc := range *aps {
						ap := &code[base+pc]
						switch i := (*ap).(type) {
						case loadStack:
							*ap = loadStack1(idx)
						case storeStack:
							*ap = storeStack1(idx)
						case storeStackP:
							*ap = storeStack1P(idx)
						case loadStackLex:
							*ap = loadStack1Lex(idx)
						case storeStackLex:
							*ap = storeStack1Lex(idx)
						case storeStackLexP:
							*ap = storeStack1LexP(idx)
						case initStackP:
							*ap = initStack1P(idx)
						case initStack:
							*ap = initStack1(idx)
						case *loadMixed:
							*ap = &loadMixedStack1{name: i.name, idx: idx, level: uint8(level), callee: i.callee}
						case *loadMixedLex:
							*ap = &loadMixedStack1Lex{name: i.name, idx: idx, level: uint8(level), callee: i.callee}
						case *resolveMixed:
							*ap = &resolveMixedStack1{typ: i.typ, name: i.name, idx: idx, level: uint8(level), strict: i.strict}
						default:
							s.c.assert(false, s.c.p.sourceOffset(pc), "Unsupported instruction for binding: %T", i)
						}
					}
				} else {
					for _, pc := range *aps {
						ap := &code[base+pc]
						switch i := (*ap).(type) {
						case loadStack:
							*ap = loadStack(idx)
						case storeStack:
							*ap = storeStack(idx)
						case storeStackP:
							*ap = storeStackP(idx)
						case loadStackLex:
							*ap = loadStackLex(idx)
						case storeStackLex:
							*ap = storeStackLex(idx)
						case storeStackLexP:
							*ap = storeStackLexP(idx)
						case initStack:
							*ap = initStack(idx)
						case initStackP:
							*ap = initStackP(idx)
						case *loadMixed:
							*ap = &loadMixedStack{name: i.name, idx: idx, level: uint8(level), callee: i.callee}
						case *loadMixedLex:
							*ap = &loadMixedStackLex{name: i.name, idx: idx, level: uint8(level), callee: i.callee}
						case *resolveMixed:
							*ap = &resolveMixedStack{typ: i.typ, name: i.name, idx: idx, level: uint8(level), strict: i.strict}
						default:
							s.c.assert(false, s.c.p.sourceOffset(pc), "Unsupported instruction for binding: %T", i)
						}
					}
				}
			}
		}
	}
	for _, nested := range s.nested {
		nested.finaliseVarAlloc(stackIdx + stackOffset)
	}
	return stashIdx, stackIdx
}

func (s *scope) moveArgsToStash() {
	for _, b := range s.bindings {
		if !b.isArg {
			break
		}
		b.inStash = true
	}
	s.argsInStash = true
	s.needStash = true
}

func (c *compiler) trimCode(delta int) {
	src := c.p.code[delta:]
	newCode := make([]instruction, len(src))
	copy(newCode, src)
	if cap(c.codeScratchpad) < cap(c.p.code) {
		c.codeScratchpad = c.p.code[:0]
	}
	c.p.code = newCode
}

func (s *scope) trimCode(delta int) {
	s.c.trimCode(delta)
	if delta != 0 {
		srcMap := s.c.p.srcMap
		for i := range srcMap {
			srcMap[i].pc -= delta
		}
		s.adjustBase(-delta)
	}
}

func (s *scope) adjustBase(delta int) {
	s.base += delta
	for _, nested := range s.nested {
		nested.adjustBase(delta)
	}
}

func (s *scope) makeNamesMap() map[unistring.String]uint32 {
	l := len(s.bindings)
	if l == 0 {
		return nil
	}
	names := make(map[unistring.String]uint32, l)
	for i, b := range s.bindings {
		idx := uint32(i)
		if b.isConst {
			idx |= maskConst
			if b.isStrict {
				idx |= maskStrict
			}
		}
		if b.isVar {
			idx |= maskVar
		}
		names[b.name] = idx
	}
	return names
}

func (s *scope) isDynamic() bool {
	return s.dynLookup || s.dynamic
}

func (s *scope) isFunction() bool {
	return s.funcType != funcNone && !s.eval
}

func (s *scope) deleteBinding(b *binding) {
	idx := 0
	for i, bb := range s.bindings {
		if bb == b {
			idx = i
			goto found
		}
	}
	return
found:
	delete(s.boundNames, b.name)
	copy(s.bindings[idx:], s.bindings[idx+1:])
	l := len(s.bindings) - 1
	s.bindings[l] = nil
	s.bindings = s.bindings[:l]
}

func (c *compiler) compile(in *ast.Program, strict, inGlobal bool, evalVm *vm) {
	c.ctxVM = evalVm

	eval := evalVm != nil
	c.p.src = in.File
	c.newScope()
	scope := c.scope
	scope.dynamic = true
	scope.eval = eval
	if !strict && len(in.Body) > 0 {
		strict = c.isStrict(in.Body) != nil
	}
	scope.strict = strict
	ownVarScope := eval && strict
	ownLexScope := !inGlobal || eval
	if ownVarScope {
		c.newBlockScope()
		scope = c.scope
		scope.variable = true
	}
	if eval && !inGlobal {
		for s := evalVm.stash; s != nil; s = s.outer {
			if ft := s.funcType; ft != funcNone && ft != funcArrow {
				scope.funcType = ft
				break
			}
		}
	}
	funcs := c.extractFunctions(in.Body)
	c.createFunctionBindings(funcs)
	numFuncs := len(scope.bindings)
	if inGlobal && !ownVarScope {
		if numFuncs == len(funcs) {
			c.compileFunctionsGlobalAllUnique(funcs)
		} else {
			c.compileFunctionsGlobal(funcs)
		}
	}
	c.compileDeclList(in.DeclarationList, false)
	numVars := len(scope.bindings) - numFuncs
	vars := make([]unistring.String, len(scope.bindings))
	for i, b := range scope.bindings {
		vars[i] = b.name
	}
	if len(vars) > 0 && !ownVarScope && ownLexScope {
		if inGlobal {
			c.emit(&bindGlobal{
				vars:      vars[numFuncs:],
				funcs:     vars[:numFuncs],
				deletable: eval,
			})
		} else {
			c.emit(&bindVars{names: vars, deletable: eval})
		}
	}
	var enter *enterBlock
	if c.compileLexicalDeclarations(in.Body, ownVarScope || !ownLexScope) {
		if ownLexScope {
			c.block = &block{
				outer:      c.block,
				typ:        blockScope,
				needResult: true,
			}
			enter = &enterBlock{}
			c.emit(enter)
		}
	}
	if len(scope.bindings) > 0 && !ownLexScope {
		var lets, consts []unistring.String
		for _, b := range c.scope.bindings[numFuncs+numVars:] {
			if b.isConst {
				consts = append(consts, b.name)
			} else {
				lets = append(lets, b.name)
			}
		}
		c.emit(&bindGlobal{
			vars:   vars[numFuncs:],
			funcs:  vars[:numFuncs],
			lets:   lets,
			consts: consts,
		})
	}
	if !inGlobal || ownVarScope {
		c.compileFunctions(funcs)
	}
	c.compileStatements(in.Body, true)
	if enter != nil {
		c.leaveScopeBlock(enter)
		c.popScope()
	}

	c.p.code = append(c.p.code, halt)

	scope.finaliseVarAlloc(0)
}

func (c *compiler) compileDeclList(v []*ast.VariableDeclaration, inFunc bool) {
	for _, value := range v {
		c.createVarBindings(value, inFunc)
	}
}

func (c *compiler) extractLabelled(st ast.Statement) ast.Statement {
	if st, ok := st.(*ast.LabelledStatement); ok {
		return c.extractLabelled(st.Statement)
	}
	return st
}

func (c *compiler) extractFunctions(list []ast.Statement) (funcs []*ast.FunctionDeclaration) {
	for _, st := range list {
		var decl *ast.FunctionDeclaration
		switch st := c.extractLabelled(st).(type) {
		case *ast.FunctionDeclaration:
			decl = st
		case *ast.LabelledStatement:
			if st1, ok := st.Statement.(*ast.FunctionDeclaration); ok {
				decl = st1
			} else {
				continue
			}
		default:
			continue
		}
		funcs = append(funcs, decl)
	}
	return
}

func (c *compiler) createFunctionBindings(funcs []*ast.FunctionDeclaration) {
	s := c.scope
	if s.outer != nil {
		unique := !s.isFunction() && !s.variable && s.strict
		for _, decl := range funcs {
			s.bindNameLexical(decl.Function.Name.Name, unique, int(decl.Function.Name.Idx1())-1)
		}
	} else {
		for _, decl := range funcs {
			s.bindName(decl.Function.Name.Name)
		}
	}
}

func (c *compiler) compileFunctions(list []*ast.FunctionDeclaration) {
	for _, decl := range list {
		c.compileFunction(decl)
	}
}

func (c *compiler) compileFunctionsGlobalAllUnique(list []*ast.FunctionDeclaration) {
	for _, decl := range list {
		c.compileFunctionLiteral(decl.Function, false).emitGetter(true)
	}
}

func (c *compiler) compileFunctionsGlobal(list []*ast.FunctionDeclaration) {
	m := make(map[unistring.String]int, len(list))
	for i := len(list) - 1; i >= 0; i-- {
		name := list[i].Function.Name.Name
		if _, exists := m[name]; !exists {
			m[name] = i
		}
	}
	idx := 0
	for i, decl := range list {
		name := decl.Function.Name.Name
		if m[name] == i {
			c.compileFunctionLiteral(decl.Function, false).emitGetter(true)
			c.scope.bindings[idx] = c.scope.boundNames[name]
			idx++
		} else {
			leave := c.enterDummyMode()
			c.compileFunctionLiteral(decl.Function, false).emitGetter(false)
			leave()
		}
	}
}

func (c *compiler) createVarIdBinding(name unistring.String, offset int, inFunc bool) {
	if c.scope.strict {
		c.checkIdentifierLName(name, offset)
		c.checkIdentifierName(name, offset)
	}
	if !inFunc || name != "arguments" {
		c.scope.bindName(name)
	}
}

func (c *compiler) createBindings(target ast.Expression, createIdBinding func(name unistring.String, offset int)) {
	switch target := target.(type) {
	case *ast.Identifier:
		createIdBinding(target.Name, int(target.Idx)-1)
	case *ast.ObjectPattern:
		for _, prop := range target.Properties {
			switch prop := prop.(type) {
			case *ast.PropertyShort:
				createIdBinding(prop.Name.Name, int(prop.Name.Idx)-1)
			case *ast.PropertyKeyed:
				c.createBindings(prop.Value, createIdBinding)
			default:
				c.throwSyntaxError(int(target.Idx0()-1), "unsupported property type in ObjectPattern: %T", prop)
			}
		}
		if target.Rest != nil {
			c.createBindings(target.Rest, createIdBinding)
		}
	case *ast.ArrayPattern:
		for _, elt := range target.Elements {
			if elt != nil {
				c.createBindings(elt, createIdBinding)
			}
		}
		if target.Rest != nil {
			c.createBindings(target.Rest, createIdBinding)
		}
	case *ast.AssignExpression:
		c.createBindings(target.Left, createIdBinding)
	default:
		c.throwSyntaxError(int(target.Idx0()-1), "unsupported binding target: %T", target)
	}
}

func (c *compiler) createVarBinding(target ast.Expression, inFunc bool) {
	c.createBindings(target, func(name unistring.String, offset int) {
		c.createVarIdBinding(name, offset, inFunc)
	})
}

func (c *compiler) createVarBindings(v *ast.VariableDeclaration, inFunc bool) {
	for _, item := range v.List {
		c.createVarBinding(item.Target, inFunc)
	}
}

func (c *compiler) createLexicalIdBinding(name unistring.String, isConst bool, offset int) *binding {
	if name == "let" {
		c.throwSyntaxError(offset, "let is disallowed as a lexically bound name")
	}
	if c.scope.strict {
		c.checkIdentifierLName(name, offset)
		c.checkIdentifierName(name, offset)
	}
	b, _ := c.scope.bindNameLexical(name, true, offset)
	if isConst {
		b.isConst, b.isStrict = true, true
	}
	return b
}

func (c *compiler) createLexicalIdBindingFuncBody(name unistring.String, isConst bool, offset int, calleeBinding *binding) *binding {
	if name == "let" {
		c.throwSyntaxError(offset, "let is disallowed as a lexically bound name")
	}
	if c.scope.strict {
		c.checkIdentifierLName(name, offset)
		c.checkIdentifierName(name, offset)
	}
	paramScope := c.scope.outer
	parentBinding := paramScope.boundNames[name]
	if parentBinding != nil {
		if parentBinding != calleeBinding && (name != "arguments" || !paramScope.argsNeeded) {
			c.throwSyntaxError(offset, "Identifier '%s' has already been declared", name)
		}
	}
	b, _ := c.scope.bindNameLexical(name, true, offset)
	if isConst {
		b.isConst, b.isStrict = true, true
	}
	return b
}

func (c *compiler) createLexicalBinding(target ast.Expression, isConst bool) {
	c.createBindings(target, func(name unistring.String, offset int) {
		c.createLexicalIdBinding(name, isConst, offset)
	})
}

func (c *compiler) createLexicalBindings(lex *ast.LexicalDeclaration) {
	for _, d := range lex.List {
		c.createLexicalBinding(d.Target, lex.Token == token.CONST)
	}
}

func (c *compiler) compileLexicalDeclarations(list []ast.Statement, scopeDeclared bool) bool {
	for _, st := range list {
		if lex, ok := st.(*ast.LexicalDeclaration); ok {
			if !scopeDeclared {
				c.newBlockScope()
				scopeDeclared = true
			}
			c.createLexicalBindings(lex)
		} else if cls, ok := st.(*ast.ClassDeclaration); ok {
			if !scopeDeclared {
				c.newBlockScope()
				scopeDeclared = true
			}
			c.createLexicalIdBinding(cls.Class.Name.Name, false, int(cls.Class.Name.Idx)-1)
		}
	}
	return scopeDeclared
}

func (c *compiler) compileLexicalDeclarationsFuncBody(list []ast.Statement, calleeBinding *binding) {
	for _, st := range list {
		if lex, ok := st.(*ast.LexicalDeclaration); ok {
			isConst := lex.Token == token.CONST
			for _, d := range lex.List {
				c.createBindings(d.Target, func(name unistring.String, offset int) {
					c.createLexicalIdBindingFuncBody(name, isConst, offset, calleeBinding)
				})
			}
		}
	}
}

func (c *compiler) compileFunction(v *ast.FunctionDeclaration) {
	name := v.Function.Name.Name
	b := c.scope.boundNames[name]
	if b == nil || b.isVar {
		e := &compiledIdentifierExpr{
			name: v.Function.Name.Name,
		}
		e.init(c, v.Function.Idx0())
		e.emitSetter(c.compileFunctionLiteral(v.Function, false), false)
	} else {
		c.compileFunctionLiteral(v.Function, false).emitGetter(true)
		b.emitInitP()
	}
}

func (c *compiler) compileStandaloneFunctionDecl(v *ast.FunctionDeclaration) {
	if c.scope.strict {
		c.throwSyntaxError(int(v.Idx0())-1, "In strict mode code, functions can only be declared at top level or inside a block.")
	}
	c.throwSyntaxError(int(v.Idx0())-1, "In non-strict mode code, functions can only be declared at top level, inside a block, or as the body of an if statement.")
}

func (c *compiler) emit(instructions ...instruction) {
	c.p.code = append(c.p.code, instructions...)
}

func (c *compiler) throwSyntaxError(offset int, format string, args ...interface{}) {
	panic(&CompilerSyntaxError{
		CompilerError: CompilerError{
			File:    c.p.src,
			Offset:  offset,
			Message: fmt.Sprintf(format, args...),
		},
	})
}

func (c *compiler) isStrict(list []ast.Statement) *ast.StringLiteral {
	for _, st := range list {
		if st, ok := st.(*ast.ExpressionStatement); ok {
			if e, ok := st.Expression.(*ast.StringLiteral); ok {
				if e.Literal == `"use strict"` || e.Literal == `'use strict'` {
					return e
				}
			} else {
				break
			}
		} else {
			break
		}
	}
	return nil
}

func (c *compiler) isStrictStatement(s ast.Statement) *ast.StringLiteral {
	if s, ok := s.(*ast.BlockStatement); ok {
		return c.isStrict(s.List)
	}
	return nil
}

func (c *compiler) checkIdentifierName(name unistring.String, offset int) {
	switch name {
	case "implements", "interface", "let", "package", "private", "protected", "public", "static", "yield":
		c.throwSyntaxError(offset, "Unexpected strict mode reserved word")
	}
}

func (c *compiler) checkIdentifierLName(name unistring.String, offset int) {
	switch name {
	case "eval", "arguments":
		c.throwSyntaxError(offset, "Assignment to eval or arguments is not allowed in strict mode")
	}
}

// Enter a 'dummy' compilation mode. Any code produced after this method is called will be discarded after
// leaveFunc is called with no additional side effects. This is useful for compiling code inside a
// constant falsy condition 'if' branch or a loop (i.e 'if (false) { ... } or while (false) { ... }).
// Such code should not be included in the final compilation result as it's never called, but it must
// still produce compilation errors if there are any.
// TODO: make sure variable lookups do not de-optimise parent scopes
func (c *compiler) enterDummyMode() (leaveFunc func()) {
	savedBlock, savedProgram := c.block, c.p
	if savedBlock != nil {
		c.block = &block{
			typ:      savedBlock.typ,
			label:    savedBlock.label,
			outer:    savedBlock.outer,
			breaking: savedBlock.breaking,
		}
	}
	c.p = &Program{}
	c.newScope()
	return func() {
		c.block, c.p = savedBlock, savedProgram
		c.popScope()
	}
}

func (c *compiler) compileStatementDummy(statement ast.Statement) {
	leave := c.enterDummyMode()
	c.compileStatement(statement, false)
	leave()
}

func (c *compiler) assert(cond bool, offset int, msg string, args ...interface{}) {
	if !cond {
		c.throwSyntaxError(offset, "Compiler bug: "+msg, args...)
	}
}

func privateIdString(desc unistring.String) unistring.String {
	return asciiString("#").concat(stringValueFromRaw(desc)).string()
}

type privateName struct {
	idx                  int
	isStatic             bool
	isMethod             bool
	hasGetter, hasSetter bool
}

type resolvedPrivateName struct {
	name     unistring.String
	idx      uint32
	level    uint8
	isStatic bool
	isMethod bool
}

func (r *resolvedPrivateName) string() unistring.String {
	return privateIdString(r.name)
}

type privateEnvRegistry struct {
	fields, methods []unistring.String
}

type classScope struct {
	c            *compiler
	privateNames map[unistring.String]*privateName

	instanceEnv, staticEnv privateEnvRegistry

	outer *classScope
}

func (r *privateEnvRegistry) createPrivateMethodId(name unistring.String) int {
	r.methods = append(r.methods, name)
	return len(r.methods) - 1
}

func (r *privateEnvRegistry) createPrivateFieldId(name unistring.String) int {
	r.fields = append(r.fields, name)
	return len(r.fields) - 1
}

func (s *classScope) declarePrivateId(name unistring.String, kind ast.PropertyKind, isStatic bool, offset int) {
	pn := s.privateNames[name]
	if pn != nil {
		if pn.isStatic == isStatic {
			switch kind {
			case ast.PropertyKindGet:
				if pn.hasSetter && !pn.hasGetter {
					pn.hasGetter = true
					return
				}
			case ast.PropertyKindSet:
				if pn.hasGetter && !pn.hasSetter {
					pn.hasSetter = true
					return
				}
			}
		}
		s.c.throwSyntaxError(offset, "Identifier '#%s' has already been declared", name)
		panic("unreachable")
	}
	var env *privateEnvRegistry
	if isStatic {
		env = &s.staticEnv
	} else {
		env = &s.instanceEnv
	}

	pn = &privateName{
		isStatic:  isStatic,
		hasGetter: kind == ast.PropertyKindGet,
		hasSetter: kind == ast.PropertyKindSet,
	}
	if kind != ast.PropertyKindValue {
		pn.idx = env.createPrivateMethodId(name)
		pn.isMethod = true
	} else {
		pn.idx = env.createPrivateFieldId(name)
	}

	if s.privateNames == nil {
		s.privateNames = make(map[unistring.String]*privateName)
	}
	s.privateNames[name] = pn
}

func (s *classScope) getDeclaredPrivateId(name unistring.String) *privateName {
	if n := s.privateNames[name]; n != nil {
		return n
	}
	s.c.assert(false, 0, "getDeclaredPrivateId() for undeclared id")
	panic("unreachable")
}

func (c *compiler) resolvePrivateName(name unistring.String, offset int) (*resolvedPrivateName, *privateId) {
	level := 0
	for s := c.classScope; s != nil; s = s.outer {
		if len(s.privateNames) > 0 {
			if pn := s.privateNames[name]; pn != nil {
				return &resolvedPrivateName{
					name:     name,
					idx:      uint32(pn.idx),
					level:    uint8(level),
					isStatic: pn.isStatic,
					isMethod: pn.isMethod,
				}, nil
			}
			level++
		}
	}
	if c.ctxVM != nil {
		for s := c.ctxVM.privEnv; s != nil; s = s.outer {
			if id := s.names[name]; id != nil {
				return nil, id
			}
		}
	}
	c.throwSyntaxError(offset, "Private field '#%s' must be declared in an enclosing class", name)
	panic("unreachable")
}
