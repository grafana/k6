package goja

import (
	"fmt"
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
)

const (
	maskConst     = 1 << 31
	maskVar       = 1 << 30
	maskDeletable = maskConst

	maskTyp = maskConst | maskVar
)

type varType byte

const (
	varTypeVar varType = iota
	varTypeLet
	varTypeConst
)

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

	enumGetExpr compiledEnumGetExpr

	evalVM *vm
}

type binding struct {
	scope        *scope
	name         unistring.String
	accessPoints map[*scope]*[]int
	isConst      bool
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

func (b *binding) markAccessPoint() {
	scope := b.scope.c.scope
	m := b.getAccessPointsForScope(scope)
	*m = append(*m, len(scope.prg.code)-scope.base)
}

func (b *binding) emitGet() {
	b.markAccessPoint()
	if b.isVar && !b.isArg {
		b.scope.c.emit(loadStash(0))
	} else {
		b.scope.c.emit(loadStashLex(0))
	}
}

func (b *binding) emitGetP() {
	if b.isVar && !b.isArg {
		// no-op
	} else {
		// make sure TDZ is checked
		b.markAccessPoint()
		b.scope.c.emit(loadStashLex(0), pop)
	}
}

func (b *binding) emitSet() {
	if b.isConst {
		b.scope.c.emit(throwAssignToConst)
		return
	}
	b.markAccessPoint()
	if b.isVar && !b.isArg {
		b.scope.c.emit(storeStash(0))
	} else {
		b.scope.c.emit(storeStashLex(0))
	}
}

func (b *binding) emitSetP() {
	if b.isConst {
		b.scope.c.emit(throwAssignToConst)
		return
	}
	b.markAccessPoint()
	if b.isVar && !b.isArg {
		b.scope.c.emit(storeStashP(0))
	} else {
		b.scope.c.emit(storeStashLexP(0))
	}
}

func (b *binding) emitInit() {
	b.markAccessPoint()
	b.scope.c.emit(initStash(0))
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
			typ = varTypeConst
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

	// in strict mode
	strict bool
	// eval top-level scope
	eval bool
	// at least one inner scope has direct eval() which can lookup names dynamically (by name)
	dynLookup bool
	// at least one binding has been marked for placement in stash
	needStash bool

	// is a function or a top-level lexical environment
	function bool
	// a function scope that has at least one direct eval() and non-strict, so the variables can be added dynamically
	dynamic bool
	// arguments have been marked for placement in stash (functions only)
	argsInStash bool
	// need 'arguments' object (functions only)
	argsNeeded bool
	// 'this' is used and non-strict, so need to box it (functions only)
	thisNeeded bool
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
	for pc, ins := range p.code {
		logger("%s %d: %T(%v)", indent, pc, ins, ins)
		if f, ok := ins.(*newFunc); ok {
			f.prg._dumpCode(indent+">", logger)
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

func (s *scope) lookupName(name unistring.String) (binding *binding, noDynamics bool) {
	noDynamics = true
	toStash := false
	for curScope := s; curScope != nil; curScope = curScope.outer {
		if curScope.dynamic {
			noDynamics = false
		} else {
			if b, exists := curScope.boundNames[name]; exists {
				if toStash && !b.inStash {
					b.moveToStash()
				}
				binding = b
				return
			}
		}
		if name == "arguments" && curScope.function {
			curScope.argsNeeded = true
			binding, _ = curScope.bindName(name)
			return
		}
		if curScope.function {
			toStash = true
		}
	}
	return
}

func (s *scope) ensureBoundNamesCreated() {
	if s.boundNames == nil {
		s.boundNames = make(map[unistring.String]*binding)
	}
}

func (s *scope) bindNameLexical(name unistring.String, unique bool, offset int) (*binding, bool) {
	if b := s.boundNames[name]; b != nil {
		if unique {
			s.c.throwSyntaxError(offset, "Identifier '%s' has already been declared", name)
		}
		return b, false
	}
	if len(s.bindings) >= (1<<24)-1 {
		s.c.throwSyntaxError(offset, "Too many variables")
	}
	b := &binding{
		scope: s,
		name:  name,
	}
	s.bindings = append(s.bindings, b)
	s.ensureBoundNamesCreated()
	s.boundNames[name] = b
	return b, true
}

func (s *scope) bindName(name unistring.String) (*binding, bool) {
	if !s.function && s.outer != nil {
		return s.outer.bindName(name)
	}
	b, created := s.bindNameLexical(name, false, 0)
	if created {
		b.isVar = true
	}
	return b, created
}

func (s *scope) bindNameShadow(name unistring.String) (*binding, bool) {
	if !s.function && s.outer != nil {
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
		if sc.function {
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
	for i, b := range s.bindings {
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
				for _, pc := range *aps {
					ap := &code[base+pc]
					switch i := (*ap).(type) {
					case loadStash:
						*ap = loadStash(idx)
					case storeStash:
						*ap = storeStash(idx)
					case storeStashP:
						*ap = storeStashP(idx)
					case loadStashLex:
						*ap = loadStashLex(idx)
					case storeStashLex:
						*ap = storeStashLex(idx)
					case storeStashLexP:
						*ap = storeStashLexP(idx)
					case initStash:
						*ap = initStash(idx)
					case *loadMixed:
						i.idx = idx
					case *resolveMixed:
						i.idx = idx
					}
				}
			}
			stashIdx++
		} else {
			var idx int
			if i < s.numArgs {
				idx = -(i + 1)
			} else {
				stackIdx++
				idx = stackIdx + stackOffset
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
				if argsInStash {
					for _, pc := range *aps {
						ap := &code[base+pc]
						switch i := (*ap).(type) {
						case loadStash:
							*ap = loadStack1(idx)
						case storeStash:
							*ap = storeStack1(idx)
						case storeStashP:
							*ap = storeStack1P(idx)
						case loadStashLex:
							*ap = loadStack1Lex(idx)
						case storeStashLex:
							*ap = storeStack1Lex(idx)
						case storeStashLexP:
							*ap = storeStack1LexP(idx)
						case initStash:
							*ap = initStack1(idx)
						case *loadMixed:
							*ap = &loadMixedStack1{name: i.name, idx: idx, level: uint8(level), callee: i.callee}
						case *loadMixedLex:
							*ap = &loadMixedStack1Lex{name: i.name, idx: idx, level: uint8(level), callee: i.callee}
						case *resolveMixed:
							*ap = &resolveMixedStack1{typ: i.typ, name: i.name, idx: idx, level: uint8(level), strict: i.strict}
						}
					}
				} else {
					for _, pc := range *aps {
						ap := &code[base+pc]
						switch i := (*ap).(type) {
						case loadStash:
							*ap = loadStack(idx)
						case storeStash:
							*ap = storeStack(idx)
						case storeStashP:
							*ap = storeStackP(idx)
						case loadStashLex:
							*ap = loadStackLex(idx)
						case storeStashLex:
							*ap = storeStackLex(idx)
						case storeStashLexP:
							*ap = storeStackLexP(idx)
						case initStash:
							*ap = initStack(idx)
						case *loadMixed:
							*ap = &loadMixedStack{name: i.name, idx: idx, level: uint8(level), callee: i.callee}
						case *loadMixedLex:
							*ap = &loadMixedStackLex{name: i.name, idx: idx, level: uint8(level), callee: i.callee}
						case *resolveMixed:
							*ap = &resolveMixedStack{typ: i.typ, name: i.name, idx: idx, level: uint8(level), strict: i.strict}
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

func (c *compiler) compile(in *ast.Program, strict, eval, inGlobal bool) {
	c.p.src = in.File
	c.newScope()
	scope := c.scope
	scope.dynamic = true
	scope.eval = eval
	if !strict && len(in.Body) > 0 {
		strict = c.isStrict(in.Body)
	}
	scope.strict = strict
	ownVarScope := eval && strict
	ownLexScope := !inGlobal || eval
	if ownVarScope {
		c.newBlockScope()
		scope = c.scope
		scope.function = true
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
		c.compileVarDecl(value, inFunc)
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
		unique := !s.function && s.strict
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
	for i, decl := range list {
		if m[decl.Function.Name.Name] == i {
			c.compileFunctionLiteral(decl.Function, false).emitGetter(true)
		} else {
			leave := c.enterDummyMode()
			c.compileFunctionLiteral(decl.Function, false).emitGetter(false)
			leave()
		}
	}
}

func (c *compiler) compileVarDecl(v *ast.VariableDeclaration, inFunc bool) {
	for _, item := range v.List {
		if c.scope.strict {
			c.checkIdentifierLName(item.Name, int(item.Idx)-1)
			c.checkIdentifierName(item.Name, int(item.Idx)-1)
		}
		if !inFunc || item.Name != "arguments" {
			c.scope.bindName(item.Name)
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
		b.emitInit()
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

func (c *compiler) isStrict(list []ast.Statement) bool {
	for _, st := range list {
		if st, ok := st.(*ast.ExpressionStatement); ok {
			if e, ok := st.Expression.(*ast.StringLiteral); ok {
				if e.Literal == `"use strict"` || e.Literal == `'use strict'` {
					return true
				}
			} else {
				break
			}
		} else {
			break
		}
	}
	return false
}

func (c *compiler) isStrictStatement(s ast.Statement) bool {
	if s, ok := s.(*ast.BlockStatement); ok {
		return c.isStrict(s.List)
	}
	return false
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
