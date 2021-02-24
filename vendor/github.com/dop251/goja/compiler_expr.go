package goja

import (
	"fmt"
	"regexp"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
	"github.com/dop251/goja/token"
	"github.com/dop251/goja/unistring"
)

var (
	octalRegexp = regexp.MustCompile(`^0[0-7]`)
)

type compiledExpr interface {
	emitGetter(putOnStack bool)
	emitSetter(valueExpr compiledExpr, putOnStack bool)
	emitUnary(prepare, body func(), postfix, putOnStack bool)
	deleteExpr() compiledExpr
	constant() bool
	addSrcMap()
}

type compiledExprOrRef interface {
	compiledExpr
	emitGetterOrRef()
}

type compiledCallExpr struct {
	baseCompiledExpr
	args   []compiledExpr
	callee compiledExpr
}

type compiledObjectLiteral struct {
	baseCompiledExpr
	expr *ast.ObjectLiteral
}

type compiledArrayLiteral struct {
	baseCompiledExpr
	expr *ast.ArrayLiteral
}

type compiledRegexpLiteral struct {
	baseCompiledExpr
	expr *ast.RegExpLiteral
}

type compiledLiteral struct {
	baseCompiledExpr
	val Value
}

type compiledAssignExpr struct {
	baseCompiledExpr
	left, right compiledExpr
	operator    token.Token
}

type deleteGlobalExpr struct {
	baseCompiledExpr
	name unistring.String
}

type deleteVarExpr struct {
	baseCompiledExpr
	name unistring.String
}

type deletePropExpr struct {
	baseCompiledExpr
	left compiledExpr
	name unistring.String
}

type deleteElemExpr struct {
	baseCompiledExpr
	left, member compiledExpr
}

type constantExpr struct {
	baseCompiledExpr
	val Value
}

type baseCompiledExpr struct {
	c      *compiler
	offset int
}

type compiledIdentifierExpr struct {
	baseCompiledExpr
	name unistring.String
}

type compiledFunctionLiteral struct {
	baseCompiledExpr
	expr    *ast.FunctionLiteral
	lhsName unistring.String
	isExpr  bool
	strict  bool
}

type compiledBracketExpr struct {
	baseCompiledExpr
	left, member compiledExpr
}

type compiledThisExpr struct {
	baseCompiledExpr
}

type compiledNewExpr struct {
	baseCompiledExpr
	callee compiledExpr
	args   []compiledExpr
}

type compiledNewTarget struct {
	baseCompiledExpr
}

type compiledSequenceExpr struct {
	baseCompiledExpr
	sequence []compiledExpr
}

type compiledUnaryExpr struct {
	baseCompiledExpr
	operand  compiledExpr
	operator token.Token
	postfix  bool
}

type compiledConditionalExpr struct {
	baseCompiledExpr
	test, consequent, alternate compiledExpr
}

type compiledLogicalOr struct {
	baseCompiledExpr
	left, right compiledExpr
}

type compiledLogicalAnd struct {
	baseCompiledExpr
	left, right compiledExpr
}

type compiledBinaryExpr struct {
	baseCompiledExpr
	left, right compiledExpr
	operator    token.Token
}

type compiledVariableExpr struct {
	baseCompiledExpr
	name        unistring.String
	initializer compiledExpr
}

type compiledEnumGetExpr struct {
	baseCompiledExpr
}

type defaultDeleteExpr struct {
	baseCompiledExpr
	expr compiledExpr
}

func (e *defaultDeleteExpr) emitGetter(putOnStack bool) {
	e.expr.emitGetter(false)
	if putOnStack {
		e.c.emit(loadVal(e.c.p.defineLiteralValue(valueTrue)))
	}
}

func (c *compiler) compileExpression(v ast.Expression) compiledExpr {
	// log.Printf("compileExpression: %T", v)
	switch v := v.(type) {
	case nil:
		return nil
	case *ast.AssignExpression:
		return c.compileAssignExpression(v)
	case *ast.NumberLiteral:
		return c.compileNumberLiteral(v)
	case *ast.StringLiteral:
		return c.compileStringLiteral(v)
	case *ast.BooleanLiteral:
		return c.compileBooleanLiteral(v)
	case *ast.NullLiteral:
		r := &compiledLiteral{
			val: _null,
		}
		r.init(c, v.Idx0())
		return r
	case *ast.Identifier:
		return c.compileIdentifierExpression(v)
	case *ast.CallExpression:
		return c.compileCallExpression(v)
	case *ast.ObjectLiteral:
		return c.compileObjectLiteral(v)
	case *ast.ArrayLiteral:
		return c.compileArrayLiteral(v)
	case *ast.RegExpLiteral:
		return c.compileRegexpLiteral(v)
	case *ast.VariableExpression:
		return c.compileVariableExpression(v)
	case *ast.BinaryExpression:
		return c.compileBinaryExpression(v)
	case *ast.UnaryExpression:
		return c.compileUnaryExpression(v)
	case *ast.ConditionalExpression:
		return c.compileConditionalExpression(v)
	case *ast.FunctionLiteral:
		return c.compileFunctionLiteral(v, true)
	case *ast.DotExpression:
		r := &compiledDotExpr{
			left: c.compileExpression(v.Left),
			name: v.Identifier.Name,
		}
		r.init(c, v.Idx0())
		return r
	case *ast.BracketExpression:
		r := &compiledBracketExpr{
			left:   c.compileExpression(v.Left),
			member: c.compileExpression(v.Member),
		}
		r.init(c, v.Idx0())
		return r
	case *ast.ThisExpression:
		r := &compiledThisExpr{}
		r.init(c, v.Idx0())
		return r
	case *ast.SequenceExpression:
		return c.compileSequenceExpression(v)
	case *ast.NewExpression:
		return c.compileNewExpression(v)
	case *ast.MetaProperty:
		return c.compileMetaProperty(v)
	default:
		panic(fmt.Errorf("Unknown expression type: %T", v))
	}
}

func (e *baseCompiledExpr) constant() bool {
	return false
}

func (e *baseCompiledExpr) init(c *compiler, idx file.Idx) {
	e.c = c
	e.offset = int(idx) - 1
}

func (e *baseCompiledExpr) emitSetter(compiledExpr, bool) {
	e.c.throwSyntaxError(e.offset, "Not a valid left-value expression")
}

func (e *baseCompiledExpr) deleteExpr() compiledExpr {
	r := &constantExpr{
		val: valueTrue,
	}
	r.init(e.c, file.Idx(e.offset+1))
	return r
}

func (e *baseCompiledExpr) emitUnary(func(), func(), bool, bool) {
	e.c.throwSyntaxError(e.offset, "Not a valid left-value expression")
}

func (e *baseCompiledExpr) addSrcMap() {
	if e.offset > 0 {
		e.c.p.srcMap = append(e.c.p.srcMap, srcMapItem{pc: len(e.c.p.code), srcPos: e.offset})
	}
}

func (e *constantExpr) emitGetter(putOnStack bool) {
	if putOnStack {
		e.addSrcMap()
		e.c.emit(loadVal(e.c.p.defineLiteralValue(e.val)))
	}
}

func (e *compiledIdentifierExpr) emitGetter(putOnStack bool) {
	e.addSrcMap()
	if b, noDynamics := e.c.scope.lookupName(e.name); noDynamics {
		if b != nil {
			if putOnStack {
				b.emitGet()
			} else {
				b.emitGetP()
			}
		} else {
			panic("No dynamics and not found")
		}
	} else {
		if b != nil {
			b.emitGetVar(false)
		} else {
			e.c.emit(loadDynamic(e.name))
		}
		if !putOnStack {
			e.c.emit(pop)
		}
	}
}

func (e *compiledIdentifierExpr) emitGetterOrRef() {
	e.addSrcMap()
	if b, noDynamics := e.c.scope.lookupName(e.name); noDynamics {
		if b != nil {
			b.emitGet()
		} else {
			panic("No dynamics and not found")
		}
	} else {
		if b != nil {
			b.emitGetVar(false)
		} else {
			e.c.emit(loadDynamicRef(e.name))
		}
	}
}

func (e *compiledIdentifierExpr) emitGetterAndCallee() {
	e.addSrcMap()
	if b, noDynamics := e.c.scope.lookupName(e.name); noDynamics {
		if b != nil {
			e.c.emit(loadUndef)
			b.emitGet()
		} else {
			panic("No dynamics and not found")
		}
	} else {
		if b != nil {
			b.emitGetVar(true)
		} else {
			e.c.emit(loadDynamicCallee(e.name))
		}
	}
}

func (c *compiler) emitVarSetter1(name unistring.String, offset int, putOnStack bool, emitRight func(isRef bool)) {
	if c.scope.strict {
		c.checkIdentifierLName(name, offset)
	}

	if b, noDynamics := c.scope.lookupName(name); noDynamics {
		emitRight(false)
		if b != nil {
			if putOnStack {
				b.emitSet()
			} else {
				b.emitSetP()
			}
		} else {
			if c.scope.strict {
				c.emit(setGlobalStrict(name))
			} else {
				c.emit(setGlobal(name))
			}
			if !putOnStack {
				c.emit(pop)
			}
		}
	} else {
		if b != nil {
			b.emitResolveVar(c.scope.strict)
		} else {
			if c.scope.strict {
				c.emit(resolveVar1Strict(name))
			} else {
				c.emit(resolveVar1(name))
			}
		}
		emitRight(true)
		if putOnStack {
			c.emit(putValue)
		} else {
			c.emit(putValueP)
		}
	}
}

func (c *compiler) emitVarSetter(name unistring.String, offset int, valueExpr compiledExpr, putOnStack bool) {
	c.emitVarSetter1(name, offset, putOnStack, func(bool) {
		c.emitExpr(valueExpr, true)
	})
}

func (e *compiledVariableExpr) emitSetter(valueExpr compiledExpr, putOnStack bool) {
	e.c.emitVarSetter(e.name, e.offset, valueExpr, putOnStack)
}

func (e *compiledIdentifierExpr) emitSetter(valueExpr compiledExpr, putOnStack bool) {
	e.c.emitVarSetter(e.name, e.offset, valueExpr, putOnStack)
}

func (e *compiledIdentifierExpr) emitUnary(prepare, body func(), postfix, putOnStack bool) {
	if putOnStack {
		e.c.emitVarSetter1(e.name, e.offset, true, func(isRef bool) {
			e.c.emit(loadUndef)
			if isRef {
				e.c.emit(getValue)
			} else {
				e.emitGetter(true)
			}
			if prepare != nil {
				prepare()
			}
			if !postfix {
				body()
			}
			e.c.emit(rdupN(1))
			if postfix {
				body()
			}
		})
		e.c.emit(pop)
	} else {
		e.c.emitVarSetter1(e.name, e.offset, false, func(isRef bool) {
			if isRef {
				e.c.emit(getValue)
			} else {
				e.emitGetter(true)
			}
			body()
		})
	}
}

func (e *compiledIdentifierExpr) deleteExpr() compiledExpr {
	if e.c.scope.strict {
		e.c.throwSyntaxError(e.offset, "Delete of an unqualified identifier in strict mode")
		panic("Unreachable")
	}
	if b, noDynamics := e.c.scope.lookupName(e.name); noDynamics {
		if b == nil {
			r := &deleteGlobalExpr{
				name: e.name,
			}
			r.init(e.c, file.Idx(0))
			return r
		}
	} else {
		if b == nil {
			r := &deleteVarExpr{
				name: e.name,
			}
			r.init(e.c, file.Idx(e.offset+1))
			return r
		}
	}
	r := &compiledLiteral{
		val: valueFalse,
	}
	r.init(e.c, file.Idx(e.offset+1))
	return r
}

type compiledDotExpr struct {
	baseCompiledExpr
	left compiledExpr
	name unistring.String
}

func (e *compiledDotExpr) emitGetter(putOnStack bool) {
	e.left.emitGetter(true)
	e.addSrcMap()
	e.c.emit(getProp(e.name))
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *compiledDotExpr) emitSetter(valueExpr compiledExpr, putOnStack bool) {
	e.left.emitGetter(true)
	valueExpr.emitGetter(true)
	if e.c.scope.strict {
		if putOnStack {
			e.c.emit(setPropStrict(e.name))
		} else {
			e.c.emit(setPropStrictP(e.name))
		}
	} else {
		if putOnStack {
			e.c.emit(setProp(e.name))
		} else {
			e.c.emit(setPropP(e.name))
		}
	}
}

func (e *compiledDotExpr) emitUnary(prepare, body func(), postfix, putOnStack bool) {
	if !putOnStack {
		e.left.emitGetter(true)
		e.c.emit(dup)
		e.c.emit(getProp(e.name))
		body()
		if e.c.scope.strict {
			e.c.emit(setPropStrict(e.name), pop)
		} else {
			e.c.emit(setProp(e.name), pop)
		}
	} else {
		if !postfix {
			e.left.emitGetter(true)
			e.c.emit(dup)
			e.c.emit(getProp(e.name))
			if prepare != nil {
				prepare()
			}
			body()
			if e.c.scope.strict {
				e.c.emit(setPropStrict(e.name))
			} else {
				e.c.emit(setProp(e.name))
			}
		} else {
			e.c.emit(loadUndef)
			e.left.emitGetter(true)
			e.c.emit(dup)
			e.c.emit(getProp(e.name))
			if prepare != nil {
				prepare()
			}
			e.c.emit(rdupN(2))
			body()
			if e.c.scope.strict {
				e.c.emit(setPropStrict(e.name))
			} else {
				e.c.emit(setProp(e.name))
			}
			e.c.emit(pop)
		}
	}
}

func (e *compiledDotExpr) deleteExpr() compiledExpr {
	r := &deletePropExpr{
		left: e.left,
		name: e.name,
	}
	r.init(e.c, file.Idx(0))
	return r
}

func (e *compiledBracketExpr) emitGetter(putOnStack bool) {
	e.left.emitGetter(true)
	e.member.emitGetter(true)
	e.addSrcMap()
	e.c.emit(getElem)
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *compiledBracketExpr) emitSetter(valueExpr compiledExpr, putOnStack bool) {
	e.left.emitGetter(true)
	e.member.emitGetter(true)
	valueExpr.emitGetter(true)
	if e.c.scope.strict {
		if putOnStack {
			e.c.emit(setElemStrict)
		} else {
			e.c.emit(setElemStrictP)
		}
	} else {
		if putOnStack {
			e.c.emit(setElem)
		} else {
			e.c.emit(setElemP)
		}
	}
}

func (e *compiledBracketExpr) emitUnary(prepare, body func(), postfix, putOnStack bool) {
	if !putOnStack {
		e.left.emitGetter(true)
		e.member.emitGetter(true)
		e.c.emit(dupN(1), dupN(1))
		e.c.emit(getElem)
		body()
		if e.c.scope.strict {
			e.c.emit(setElemStrict, pop)
		} else {
			e.c.emit(setElem, pop)
		}
	} else {
		if !postfix {
			e.left.emitGetter(true)
			e.member.emitGetter(true)
			e.c.emit(dupN(1), dupN(1))
			e.c.emit(getElem)
			if prepare != nil {
				prepare()
			}
			body()
			if e.c.scope.strict {
				e.c.emit(setElemStrict)
			} else {
				e.c.emit(setElem)
			}
		} else {
			e.c.emit(loadUndef)
			e.left.emitGetter(true)
			e.member.emitGetter(true)
			e.c.emit(dupN(1), dupN(1))
			e.c.emit(getElem)
			if prepare != nil {
				prepare()
			}
			e.c.emit(rdupN(3))
			body()
			if e.c.scope.strict {
				e.c.emit(setElemStrict, pop)
			} else {
				e.c.emit(setElem, pop)
			}
		}
	}
}

func (e *compiledBracketExpr) deleteExpr() compiledExpr {
	r := &deleteElemExpr{
		left:   e.left,
		member: e.member,
	}
	r.init(e.c, file.Idx(0))
	return r
}

func (e *deleteElemExpr) emitGetter(putOnStack bool) {
	e.left.emitGetter(true)
	e.member.emitGetter(true)
	e.addSrcMap()
	if e.c.scope.strict {
		e.c.emit(deleteElemStrict)
	} else {
		e.c.emit(deleteElem)
	}
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *deletePropExpr) emitGetter(putOnStack bool) {
	e.left.emitGetter(true)
	e.addSrcMap()
	if e.c.scope.strict {
		e.c.emit(deletePropStrict(e.name))
	} else {
		e.c.emit(deleteProp(e.name))
	}
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *deleteVarExpr) emitGetter(putOnStack bool) {
	/*if e.c.scope.strict {
		e.c.throwSyntaxError(e.offset, "Delete of an unqualified identifier in strict mode")
		return
	}*/
	e.c.emit(deleteVar(e.name))
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *deleteGlobalExpr) emitGetter(putOnStack bool) {
	/*if e.c.scope.strict {
		e.c.throwSyntaxError(e.offset, "Delete of an unqualified identifier in strict mode")
		return
	}*/

	e.c.emit(deleteGlobal(e.name))
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *compiledAssignExpr) emitGetter(putOnStack bool) {
	e.addSrcMap()
	switch e.operator {
	case token.ASSIGN:
		if fn, ok := e.right.(*compiledFunctionLiteral); ok {
			if fn.expr.Name == nil {
				if id, ok := e.left.(*compiledIdentifierExpr); ok {
					fn.lhsName = id.name
				}
			}
		}
		e.left.emitSetter(e.right, putOnStack)
	case token.PLUS:
		e.left.emitUnary(nil, func() {
			e.right.emitGetter(true)
			e.c.emit(add)
		}, false, putOnStack)
	case token.MINUS:
		e.left.emitUnary(nil, func() {
			e.right.emitGetter(true)
			e.c.emit(sub)
		}, false, putOnStack)
	case token.MULTIPLY:
		e.left.emitUnary(nil, func() {
			e.right.emitGetter(true)
			e.c.emit(mul)
		}, false, putOnStack)
	case token.SLASH:
		e.left.emitUnary(nil, func() {
			e.right.emitGetter(true)
			e.c.emit(div)
		}, false, putOnStack)
	case token.REMAINDER:
		e.left.emitUnary(nil, func() {
			e.right.emitGetter(true)
			e.c.emit(mod)
		}, false, putOnStack)
	case token.OR:
		e.left.emitUnary(nil, func() {
			e.right.emitGetter(true)
			e.c.emit(or)
		}, false, putOnStack)
	case token.AND:
		e.left.emitUnary(nil, func() {
			e.right.emitGetter(true)
			e.c.emit(and)
		}, false, putOnStack)
	case token.EXCLUSIVE_OR:
		e.left.emitUnary(nil, func() {
			e.right.emitGetter(true)
			e.c.emit(xor)
		}, false, putOnStack)
	case token.SHIFT_LEFT:
		e.left.emitUnary(nil, func() {
			e.right.emitGetter(true)
			e.c.emit(sal)
		}, false, putOnStack)
	case token.SHIFT_RIGHT:
		e.left.emitUnary(nil, func() {
			e.right.emitGetter(true)
			e.c.emit(sar)
		}, false, putOnStack)
	case token.UNSIGNED_SHIFT_RIGHT:
		e.left.emitUnary(nil, func() {
			e.right.emitGetter(true)
			e.c.emit(shr)
		}, false, putOnStack)
	default:
		panic(fmt.Errorf("Unknown assign operator: %s", e.operator.String()))
	}
}

func (e *compiledLiteral) emitGetter(putOnStack bool) {
	if putOnStack {
		e.addSrcMap()
		e.c.emit(loadVal(e.c.p.defineLiteralValue(e.val)))
	}
}

func (e *compiledLiteral) constant() bool {
	return true
}

func (e *compiledFunctionLiteral) emitGetter(putOnStack bool) {
	savedPrg := e.c.p
	e.c.p = &Program{
		src: e.c.p.src,
	}
	e.c.newScope()
	e.c.scope.function = true

	var name unistring.String
	if e.expr.Name != nil {
		name = e.expr.Name.Name
	} else {
		name = e.lhsName
	}

	if name != "" {
		e.c.p.funcName = name
	}
	savedBlock := e.c.block
	defer func() {
		e.c.block = savedBlock
	}()

	e.c.block = &block{
		typ: blockScope,
	}

	if !e.c.scope.strict {
		e.c.scope.strict = e.strict
	}

	if e.c.scope.strict {
		for _, item := range e.expr.ParameterList.List {
			e.c.checkIdentifierName(item.Name, int(item.Idx)-1)
			e.c.checkIdentifierLName(item.Name, int(item.Idx)-1)
		}
	}

	length := len(e.expr.ParameterList.List)

	for _, item := range e.expr.ParameterList.List {
		b, unique := e.c.scope.bindNameShadow(item.Name)
		if !unique && e.c.scope.strict {
			e.c.throwSyntaxError(int(item.Idx)-1, "Strict mode function may not have duplicate parameter names (%s)", item.Name)
			return
		}
		b.isArg = true
		b.isVar = true
	}
	paramsCount := len(e.c.scope.bindings)
	e.c.scope.numArgs = paramsCount
	e.c.compileDeclList(e.expr.DeclarationList, true)
	body := e.expr.Body.List
	funcs := e.c.extractFunctions(body)
	e.c.createFunctionBindings(funcs)
	s := e.c.scope
	e.c.compileLexicalDeclarations(body, true)
	var calleeBinding *binding
	if e.isExpr && e.expr.Name != nil {
		if b, created := s.bindName(e.expr.Name.Name); created {
			calleeBinding = b
		}
	}
	preambleLen := 4 // enter, boxThis, createArgs, set
	e.c.p.code = make([]instruction, preambleLen, 8)

	if calleeBinding != nil {
		e.c.emit(loadCallee)
		calleeBinding.emitSetP()
	}

	e.c.compileFunctions(funcs)
	e.c.compileStatements(body, false)

	var last ast.Statement
	if l := len(body); l > 0 {
		last = body[l-1]
	}
	if _, ok := last.(*ast.ReturnStatement); !ok {
		e.c.emit(loadUndef, ret)
	}

	delta := 0
	code := e.c.p.code

	if calleeBinding != nil && !s.isDynamic() && calleeBinding.useCount() == 1 {
		s.deleteBinding(calleeBinding)
		preambleLen += 2
	}

	if s.argsNeeded {
		s.moveArgsToStash()
		pos := preambleLen - 2
		delta += 2
		if s.strict {
			code[pos] = createArgsStrict(length)
		} else {
			code[pos] = createArgs(length)
		}
		pos++
		b, _ := s.bindName("arguments")
		e.c.p.code = code[:pos]
		b.emitSetP()
		e.c.p.code = code
	}

	stashSize, stackSize := s.finaliseVarAlloc(0)

	if !s.strict && s.thisNeeded {
		delta++
		code[preambleLen-delta] = boxThis
	}
	delta++
	delta = preambleLen - delta
	var enter instruction
	if stashSize > 0 || s.argsInStash || s.isDynamic() {
		enter1 := enterFunc{
			numArgs:     uint32(paramsCount),
			argsToStash: s.argsInStash,
			stashSize:   uint32(stashSize),
			stackSize:   uint32(stackSize),
			extensible:  s.dynamic,
		}
		if s.isDynamic() {
			enter1.names = s.makeNamesMap()
		}
		enter = &enter1
	} else {
		enter = &enterFuncStashless{
			stackSize: uint32(stackSize),
			args:      uint32(paramsCount),
		}
	}
	code[delta] = enter
	if delta != 0 {
		e.c.p.code = code[delta:]
		for i := range e.c.p.srcMap {
			e.c.p.srcMap[i].pc -= delta
		}
		s.adjustBase(-delta)
	}

	strict := s.strict
	p := e.c.p
	// e.c.p.dumpCode()
	e.c.popScope()
	e.c.p = savedPrg
	e.c.emit(&newFunc{prg: p, length: uint32(length), name: name, srcStart: uint32(e.expr.Idx0() - 1), srcEnd: uint32(e.expr.Idx1() - 1), strict: strict})
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (c *compiler) compileFunctionLiteral(v *ast.FunctionLiteral, isExpr bool) *compiledFunctionLiteral {
	strict := c.scope.strict || c.isStrictStatement(v.Body)
	if v.Name != nil && strict {
		c.checkIdentifierLName(v.Name.Name, int(v.Name.Idx)-1)
	}
	r := &compiledFunctionLiteral{
		expr:   v,
		isExpr: isExpr,
		strict: strict,
	}
	r.init(c, v.Idx0())
	return r
}

func (e *compiledThisExpr) emitGetter(putOnStack bool) {
	if putOnStack {
		e.addSrcMap()
		scope := e.c.scope
		for ; scope != nil && !scope.function && !scope.eval; scope = scope.outer {
		}

		if scope != nil {
			scope.thisNeeded = true
			e.c.emit(loadStack(0))
		} else {
			e.c.emit(loadGlobalObject)
		}
	}
}

func (e *compiledNewExpr) emitGetter(putOnStack bool) {
	e.callee.emitGetter(true)
	for _, expr := range e.args {
		expr.emitGetter(true)
	}
	e.addSrcMap()
	e.c.emit(_new(len(e.args)))
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (c *compiler) compileNewExpression(v *ast.NewExpression) compiledExpr {
	args := make([]compiledExpr, len(v.ArgumentList))
	for i, expr := range v.ArgumentList {
		args[i] = c.compileExpression(expr)
	}
	r := &compiledNewExpr{
		callee: c.compileExpression(v.Callee),
		args:   args,
	}
	r.init(c, v.Idx0())
	return r
}

func (e *compiledNewTarget) emitGetter(putOnStack bool) {
	if putOnStack {
		e.addSrcMap()
		e.c.emit(loadNewTarget)
	}
}

func (c *compiler) compileMetaProperty(v *ast.MetaProperty) compiledExpr {
	if v.Meta.Name == "new" || v.Property.Name != "target" {
		r := &compiledNewTarget{}
		r.init(c, v.Idx0())
		return r
	}
	c.throwSyntaxError(int(v.Idx)-1, "Unsupported meta property: %s.%s", v.Meta.Name, v.Property.Name)
	return nil
}

func (e *compiledSequenceExpr) emitGetter(putOnStack bool) {
	if len(e.sequence) > 0 {
		for i := 0; i < len(e.sequence)-1; i++ {
			e.sequence[i].emitGetter(false)
		}
		e.sequence[len(e.sequence)-1].emitGetter(putOnStack)
	}
}

func (c *compiler) compileSequenceExpression(v *ast.SequenceExpression) compiledExpr {
	s := make([]compiledExpr, len(v.Sequence))
	for i, expr := range v.Sequence {
		s[i] = c.compileExpression(expr)
	}
	r := &compiledSequenceExpr{
		sequence: s,
	}
	var idx file.Idx
	if len(v.Sequence) > 0 {
		idx = v.Idx0()
	}
	r.init(c, idx)
	return r
}

func (c *compiler) emitThrow(v Value) {
	if o, ok := v.(*Object); ok {
		t := nilSafe(o.self.getStr("name", nil)).toString().String()
		switch t {
		case "TypeError":
			c.emit(loadDynamic(t))
			msg := o.self.getStr("message", nil)
			if msg != nil {
				c.emit(loadVal(c.p.defineLiteralValue(msg)))
				c.emit(_new(1))
			} else {
				c.emit(_new(0))
			}
			c.emit(throw)
			return
		}
	}
	panic(fmt.Errorf("unknown exception type thrown while evaliating constant expression: %s", v.String()))
}

func (c *compiler) emitConst(expr compiledExpr, putOnStack bool) {
	v, ex := c.evalConst(expr)
	if ex == nil {
		if putOnStack {
			c.emit(loadVal(c.p.defineLiteralValue(v)))
		}
	} else {
		c.emitThrow(ex.val)
	}
}

func (c *compiler) emitExpr(expr compiledExpr, putOnStack bool) {
	if expr.constant() {
		c.emitConst(expr, putOnStack)
	} else {
		expr.emitGetter(putOnStack)
	}
}

func (c *compiler) evalConst(expr compiledExpr) (Value, *Exception) {
	if expr, ok := expr.(*compiledLiteral); ok {
		return expr.val, nil
	}
	if c.evalVM == nil {
		c.evalVM = New().vm
	}
	var savedPrg *Program
	createdPrg := false
	if c.evalVM.prg == nil {
		c.evalVM.prg = &Program{}
		savedPrg = c.p
		c.p = c.evalVM.prg
		createdPrg = true
	}
	savedPc := len(c.p.code)
	expr.emitGetter(true)
	c.emit(halt)
	c.evalVM.pc = savedPc
	ex := c.evalVM.runTry()
	if createdPrg {
		c.evalVM.prg = nil
		c.evalVM.pc = 0
		c.p = savedPrg
	} else {
		c.evalVM.prg.code = c.evalVM.prg.code[:savedPc]
		c.p.code = c.evalVM.prg.code
	}
	if ex == nil {
		return c.evalVM.pop(), nil
	}
	return nil, ex
}

func (e *compiledUnaryExpr) constant() bool {
	return e.operand.constant()
}

func (e *compiledUnaryExpr) emitGetter(putOnStack bool) {
	var prepare, body func()

	toNumber := func() {
		e.c.emit(toNumber)
	}

	switch e.operator {
	case token.NOT:
		e.operand.emitGetter(true)
		e.c.emit(not)
		goto end
	case token.BITWISE_NOT:
		e.operand.emitGetter(true)
		e.c.emit(bnot)
		goto end
	case token.TYPEOF:
		if o, ok := e.operand.(compiledExprOrRef); ok {
			o.emitGetterOrRef()
		} else {
			e.operand.emitGetter(true)
		}
		e.c.emit(typeof)
		goto end
	case token.DELETE:
		e.operand.deleteExpr().emitGetter(putOnStack)
		return
	case token.MINUS:
		e.c.emitExpr(e.operand, true)
		e.c.emit(neg)
		goto end
	case token.PLUS:
		e.c.emitExpr(e.operand, true)
		e.c.emit(plus)
		goto end
	case token.INCREMENT:
		prepare = toNumber
		body = func() {
			e.c.emit(inc)
		}
	case token.DECREMENT:
		prepare = toNumber
		body = func() {
			e.c.emit(dec)
		}
	case token.VOID:
		e.c.emitExpr(e.operand, false)
		if putOnStack {
			e.c.emit(loadUndef)
		}
		return
	default:
		panic(fmt.Errorf("Unknown unary operator: %s", e.operator.String()))
	}

	e.operand.emitUnary(prepare, body, e.postfix, putOnStack)
	return

end:
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (c *compiler) compileUnaryExpression(v *ast.UnaryExpression) compiledExpr {
	r := &compiledUnaryExpr{
		operand:  c.compileExpression(v.Operand),
		operator: v.Operator,
		postfix:  v.Postfix,
	}
	r.init(c, v.Idx0())
	return r
}

func (e *compiledConditionalExpr) emitGetter(putOnStack bool) {
	e.test.emitGetter(true)
	j := len(e.c.p.code)
	e.c.emit(nil)
	e.consequent.emitGetter(putOnStack)
	j1 := len(e.c.p.code)
	e.c.emit(nil)
	e.c.p.code[j] = jne(len(e.c.p.code) - j)
	e.alternate.emitGetter(putOnStack)
	e.c.p.code[j1] = jump(len(e.c.p.code) - j1)
}

func (c *compiler) compileConditionalExpression(v *ast.ConditionalExpression) compiledExpr {
	r := &compiledConditionalExpr{
		test:       c.compileExpression(v.Test),
		consequent: c.compileExpression(v.Consequent),
		alternate:  c.compileExpression(v.Alternate),
	}
	r.init(c, v.Idx0())
	return r
}

func (e *compiledLogicalOr) constant() bool {
	if e.left.constant() {
		if v, ex := e.c.evalConst(e.left); ex == nil {
			if v.ToBoolean() {
				return true
			}
			return e.right.constant()
		} else {
			return true
		}
	}

	return false
}

func (e *compiledLogicalOr) emitGetter(putOnStack bool) {
	if e.left.constant() {
		if v, ex := e.c.evalConst(e.left); ex == nil {
			if !v.ToBoolean() {
				e.c.emitExpr(e.right, putOnStack)
			} else {
				if putOnStack {
					e.c.emit(loadVal(e.c.p.defineLiteralValue(v)))
				}
			}
		} else {
			e.c.emitThrow(ex.val)
		}
		return
	}
	e.c.emitExpr(e.left, true)
	j := len(e.c.p.code)
	e.addSrcMap()
	e.c.emit(nil)
	e.c.emit(pop)
	e.c.emitExpr(e.right, true)
	e.c.p.code[j] = jeq1(len(e.c.p.code) - j)
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *compiledLogicalAnd) constant() bool {
	if e.left.constant() {
		if v, ex := e.c.evalConst(e.left); ex == nil {
			if !v.ToBoolean() {
				return true
			} else {
				return e.right.constant()
			}
		} else {
			return true
		}
	}

	return false
}

func (e *compiledLogicalAnd) emitGetter(putOnStack bool) {
	var j int
	if e.left.constant() {
		if v, ex := e.c.evalConst(e.left); ex == nil {
			if !v.ToBoolean() {
				e.c.emit(loadVal(e.c.p.defineLiteralValue(v)))
			} else {
				e.c.emitExpr(e.right, putOnStack)
			}
		} else {
			e.c.emitThrow(ex.val)
		}
		return
	}
	e.left.emitGetter(true)
	j = len(e.c.p.code)
	e.addSrcMap()
	e.c.emit(nil)
	e.c.emit(pop)
	e.c.emitExpr(e.right, true)
	e.c.p.code[j] = jneq1(len(e.c.p.code) - j)
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *compiledBinaryExpr) constant() bool {
	return e.left.constant() && e.right.constant()
}

func (e *compiledBinaryExpr) emitGetter(putOnStack bool) {
	e.c.emitExpr(e.left, true)
	e.c.emitExpr(e.right, true)
	e.addSrcMap()

	switch e.operator {
	case token.LESS:
		e.c.emit(op_lt)
	case token.GREATER:
		e.c.emit(op_gt)
	case token.LESS_OR_EQUAL:
		e.c.emit(op_lte)
	case token.GREATER_OR_EQUAL:
		e.c.emit(op_gte)
	case token.EQUAL:
		e.c.emit(op_eq)
	case token.NOT_EQUAL:
		e.c.emit(op_neq)
	case token.STRICT_EQUAL:
		e.c.emit(op_strict_eq)
	case token.STRICT_NOT_EQUAL:
		e.c.emit(op_strict_neq)
	case token.PLUS:
		e.c.emit(add)
	case token.MINUS:
		e.c.emit(sub)
	case token.MULTIPLY:
		e.c.emit(mul)
	case token.SLASH:
		e.c.emit(div)
	case token.REMAINDER:
		e.c.emit(mod)
	case token.AND:
		e.c.emit(and)
	case token.OR:
		e.c.emit(or)
	case token.EXCLUSIVE_OR:
		e.c.emit(xor)
	case token.INSTANCEOF:
		e.c.emit(op_instanceof)
	case token.IN:
		e.c.emit(op_in)
	case token.SHIFT_LEFT:
		e.c.emit(sal)
	case token.SHIFT_RIGHT:
		e.c.emit(sar)
	case token.UNSIGNED_SHIFT_RIGHT:
		e.c.emit(shr)
	default:
		panic(fmt.Errorf("Unknown operator: %s", e.operator.String()))
	}

	if !putOnStack {
		e.c.emit(pop)
	}
}

func (c *compiler) compileBinaryExpression(v *ast.BinaryExpression) compiledExpr {

	switch v.Operator {
	case token.LOGICAL_OR:
		return c.compileLogicalOr(v.Left, v.Right, v.Idx0())
	case token.LOGICAL_AND:
		return c.compileLogicalAnd(v.Left, v.Right, v.Idx0())
	}

	r := &compiledBinaryExpr{
		left:     c.compileExpression(v.Left),
		right:    c.compileExpression(v.Right),
		operator: v.Operator,
	}
	r.init(c, v.Idx0())
	return r
}

func (c *compiler) compileLogicalOr(left, right ast.Expression, idx file.Idx) compiledExpr {
	r := &compiledLogicalOr{
		left:  c.compileExpression(left),
		right: c.compileExpression(right),
	}
	r.init(c, idx)
	return r
}

func (c *compiler) compileLogicalAnd(left, right ast.Expression, idx file.Idx) compiledExpr {
	r := &compiledLogicalAnd{
		left:  c.compileExpression(left),
		right: c.compileExpression(right),
	}
	r.init(c, idx)
	return r
}

func (e *compiledVariableExpr) emitGetter(putOnStack bool) {
	if e.initializer != nil {
		idExpr := &compiledIdentifierExpr{
			name: e.name,
		}
		idExpr.init(e.c, file.Idx(0))
		idExpr.emitSetter(e.initializer, putOnStack)
	} else {
		if putOnStack {
			e.c.emit(loadUndef)
		}
	}
}

func (c *compiler) compileVariableExpression(v *ast.VariableExpression) compiledExpr {
	r := &compiledVariableExpr{
		name:        v.Name,
		initializer: c.compileExpression(v.Initializer),
	}
	if fn, ok := r.initializer.(*compiledFunctionLiteral); ok {
		fn.lhsName = v.Name
	}
	r.init(c, v.Idx0())
	return r
}

func (e *compiledObjectLiteral) emitGetter(putOnStack bool) {
	e.addSrcMap()
	e.c.emit(newObject)
	for _, prop := range e.expr.Value {
		keyExpr := e.c.compileExpression(prop.Key)
		cl, ok := keyExpr.(*compiledLiteral)
		if !ok {
			e.c.throwSyntaxError(e.offset, "non-literal properties in object literal are not supported yet")
		}
		key := cl.val.string()
		valueExpr := e.c.compileExpression(prop.Value)
		if fn, ok := valueExpr.(*compiledFunctionLiteral); ok {
			if fn.expr.Name == nil {
				fn.lhsName = key
			}
		}
		valueExpr.emitGetter(true)
		switch prop.Kind {
		case "value":
			if key == __proto__ {
				e.c.emit(setProto)
			} else {
				e.c.emit(setProp1(key))
			}
		case "method":
			e.c.emit(setProp1(key))
		case "get":
			e.c.emit(setPropGetter(key))
		case "set":
			e.c.emit(setPropSetter(key))
		default:
			panic(fmt.Errorf("unknown property kind: %s", prop.Kind))
		}
	}
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (c *compiler) compileObjectLiteral(v *ast.ObjectLiteral) compiledExpr {
	r := &compiledObjectLiteral{
		expr: v,
	}
	r.init(c, v.Idx0())
	return r
}

func (e *compiledArrayLiteral) emitGetter(putOnStack bool) {
	e.addSrcMap()
	objCount := 0
	for _, v := range e.expr.Value {
		if v != nil {
			e.c.compileExpression(v).emitGetter(true)
			objCount++
		} else {
			e.c.emit(loadNil)
		}
	}
	if objCount == len(e.expr.Value) {
		e.c.emit(newArray(objCount))
	} else {
		e.c.emit(&newArraySparse{
			l:        len(e.expr.Value),
			objCount: objCount,
		})
	}
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (c *compiler) compileArrayLiteral(v *ast.ArrayLiteral) compiledExpr {
	r := &compiledArrayLiteral{
		expr: v,
	}
	r.init(c, v.Idx0())
	return r
}

func (e *compiledRegexpLiteral) emitGetter(putOnStack bool) {
	if putOnStack {
		pattern, err := compileRegexp(e.expr.Pattern, e.expr.Flags)
		if err != nil {
			e.c.throwSyntaxError(e.offset, err.Error())
		}

		e.c.emit(&newRegexp{pattern: pattern, src: newStringValue(e.expr.Pattern)})
	}
}

func (c *compiler) compileRegexpLiteral(v *ast.RegExpLiteral) compiledExpr {
	r := &compiledRegexpLiteral{
		expr: v,
	}
	r.init(c, v.Idx0())
	return r
}

func (e *compiledCallExpr) emitGetter(putOnStack bool) {
	var calleeName unistring.String
	switch callee := e.callee.(type) {
	case *compiledDotExpr:
		callee.left.emitGetter(true)
		e.c.emit(dup)
		e.c.emit(getPropCallee(callee.name))
	case *compiledBracketExpr:
		callee.left.emitGetter(true)
		e.c.emit(dup)
		callee.member.emitGetter(true)
		e.c.emit(getElemCallee)
	case *compiledIdentifierExpr:
		calleeName = callee.name
		callee.emitGetterAndCallee()
	default:
		e.c.emit(loadUndef)
		callee.emitGetter(true)
	}

	for _, expr := range e.args {
		expr.emitGetter(true)
	}

	e.addSrcMap()
	if calleeName == "eval" {
		foundFunc := false
		for sc := e.c.scope; sc != nil; sc = sc.outer {
			if !foundFunc && sc.function {
				foundFunc = true
				sc.thisNeeded, sc.argsNeeded = true, true
				if !sc.strict {
					sc.dynamic = true
				}
			}
			sc.dynLookup = true
		}

		if e.c.scope.strict {
			e.c.emit(callEvalStrict(len(e.args)))
		} else {
			e.c.emit(callEval(len(e.args)))
		}
	} else {
		e.c.emit(call(len(e.args)))
	}

	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *compiledCallExpr) deleteExpr() compiledExpr {
	r := &defaultDeleteExpr{
		expr: e,
	}
	r.init(e.c, file.Idx(e.offset+1))
	return r
}

func (c *compiler) compileCallExpression(v *ast.CallExpression) compiledExpr {

	args := make([]compiledExpr, len(v.ArgumentList))
	for i, argExpr := range v.ArgumentList {
		args[i] = c.compileExpression(argExpr)
	}

	r := &compiledCallExpr{
		args:   args,
		callee: c.compileExpression(v.Callee),
	}
	r.init(c, v.LeftParenthesis)
	return r
}

func (c *compiler) compileIdentifierExpression(v *ast.Identifier) compiledExpr {
	if c.scope.strict {
		c.checkIdentifierName(v.Name, int(v.Idx)-1)
	}

	r := &compiledIdentifierExpr{
		name: v.Name,
	}
	r.offset = int(v.Idx) - 1
	r.init(c, v.Idx0())
	return r
}

func (c *compiler) compileNumberLiteral(v *ast.NumberLiteral) compiledExpr {
	if c.scope.strict && octalRegexp.MatchString(v.Literal) {
		c.throwSyntaxError(int(v.Idx)-1, "Octal literals are not allowed in strict mode")
		panic("Unreachable")
	}
	var val Value
	switch num := v.Value.(type) {
	case int64:
		val = intToValue(num)
	case float64:
		val = floatToValue(num)
	default:
		panic(fmt.Errorf("Unsupported number literal type: %T", v.Value))
	}
	r := &compiledLiteral{
		val: val,
	}
	r.init(c, v.Idx0())
	return r
}

func (c *compiler) compileStringLiteral(v *ast.StringLiteral) compiledExpr {
	r := &compiledLiteral{
		val: stringValueFromRaw(v.Value),
	}
	r.init(c, v.Idx0())
	return r
}

func (c *compiler) compileBooleanLiteral(v *ast.BooleanLiteral) compiledExpr {
	var val Value
	if v.Value {
		val = valueTrue
	} else {
		val = valueFalse
	}

	r := &compiledLiteral{
		val: val,
	}
	r.init(c, v.Idx0())
	return r
}

func (c *compiler) compileAssignExpression(v *ast.AssignExpression) compiledExpr {
	// log.Printf("compileAssignExpression(): %+v", v)

	r := &compiledAssignExpr{
		left:     c.compileExpression(v.Left),
		right:    c.compileExpression(v.Right),
		operator: v.Operator,
	}
	r.init(c, v.Idx0())
	return r
}

func (e *compiledEnumGetExpr) emitGetter(putOnStack bool) {
	e.c.emit(enumGet)
	if !putOnStack {
		e.c.emit(pop)
	}
}
