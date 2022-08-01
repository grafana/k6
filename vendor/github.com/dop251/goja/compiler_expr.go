package goja

import (
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
	"github.com/dop251/goja/token"
	"github.com/dop251/goja/unistring"
)

type compiledExpr interface {
	emitGetter(putOnStack bool)
	emitSetter(valueExpr compiledExpr, putOnStack bool)
	emitRef()
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

	isVariadic bool
}

type compiledNewExpr struct {
	compiledCallExpr
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

type compiledTemplateLiteral struct {
	baseCompiledExpr
	tag         compiledExpr
	elements    []*ast.TemplateElement
	expressions []compiledExpr
}

type compiledAssignExpr struct {
	baseCompiledExpr
	left, right compiledExpr
	operator    token.Token
}

type compiledObjectAssignmentPattern struct {
	baseCompiledExpr
	expr *ast.ObjectPattern
}

type compiledArrayAssignmentPattern struct {
	baseCompiledExpr
	expr *ast.ArrayPattern
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

type funcType uint8

const (
	funcNone funcType = iota
	funcRegular
	funcArrow
	funcMethod
	funcClsInit
	funcCtor
	funcDerivedCtor
)

type compiledFunctionLiteral struct {
	baseCompiledExpr
	name            *ast.Identifier
	parameterList   *ast.ParameterList
	body            []ast.Statement
	source          string
	declarationList []*ast.VariableDeclaration
	lhsName         unistring.String
	strict          *ast.StringLiteral
	homeObjOffset   uint32
	typ             funcType
	isExpr          bool
}

type compiledBracketExpr struct {
	baseCompiledExpr
	left, member compiledExpr
}

type compiledThisExpr struct {
	baseCompiledExpr
}

type compiledSuperExpr struct {
	baseCompiledExpr
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

type compiledCoalesce struct {
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

type compiledEnumGetExpr struct {
	baseCompiledExpr
}

type defaultDeleteExpr struct {
	baseCompiledExpr
	expr compiledExpr
}

type compiledSpreadCallArgument struct {
	baseCompiledExpr
	expr compiledExpr
}

type compiledOptionalChain struct {
	baseCompiledExpr
	expr compiledExpr
}

type compiledOptional struct {
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
	case *ast.TemplateLiteral:
		return c.compileTemplateLiteral(v)
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
	case *ast.BinaryExpression:
		return c.compileBinaryExpression(v)
	case *ast.UnaryExpression:
		return c.compileUnaryExpression(v)
	case *ast.ConditionalExpression:
		return c.compileConditionalExpression(v)
	case *ast.FunctionLiteral:
		return c.compileFunctionLiteral(v, true)
	case *ast.ArrowFunctionLiteral:
		return c.compileArrowFunctionLiteral(v)
	case *ast.ClassLiteral:
		return c.compileClassLiteral(v, true)
	case *ast.DotExpression:
		return c.compileDotExpression(v)
	case *ast.PrivateDotExpression:
		return c.compilePrivateDotExpression(v)
	case *ast.BracketExpression:
		return c.compileBracketExpression(v)
	case *ast.ThisExpression:
		r := &compiledThisExpr{}
		r.init(c, v.Idx0())
		return r
	case *ast.SuperExpression:
		c.throwSyntaxError(int(v.Idx0())-1, "'super' keyword unexpected here")
		panic("unreachable")
	case *ast.SequenceExpression:
		return c.compileSequenceExpression(v)
	case *ast.NewExpression:
		return c.compileNewExpression(v)
	case *ast.MetaProperty:
		return c.compileMetaProperty(v)
	case *ast.ObjectPattern:
		return c.compileObjectAssignmentPattern(v)
	case *ast.ArrayPattern:
		return c.compileArrayAssignmentPattern(v)
	case *ast.OptionalChain:
		r := &compiledOptionalChain{
			expr: c.compileExpression(v.Expression),
		}
		r.init(c, v.Idx0())
		return r
	case *ast.Optional:
		r := &compiledOptional{
			expr: c.compileExpression(v.Expression),
		}
		r.init(c, v.Idx0())
		return r
	default:
		c.assert(false, int(v.Idx0())-1, "Unknown expression type: %T", v)
		panic("unreachable")
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

func (e *baseCompiledExpr) emitRef() {
	e.c.assert(false, e.offset, "Cannot emit reference for this type of expression")
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
	if e.offset >= 0 {
		e.c.p.addSrcMap(e.offset)
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
		e.c.assert(b != nil, e.offset, "No dynamics and not found")
		if putOnStack {
			b.emitGet()
		} else {
			b.emitGetP()
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
		e.c.assert(b != nil, e.offset, "No dynamics and not found")
		b.emitGet()
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
		e.c.assert(b != nil, e.offset, "No dynamics and not found")
		e.c.emit(loadUndef)
		b.emitGet()
	} else {
		if b != nil {
			b.emitGetVar(true)
		} else {
			e.c.emit(loadDynamicCallee(e.name))
		}
	}
}

func (e *compiledIdentifierExpr) emitVarSetter1(putOnStack bool, emitRight func(isRef bool)) {
	e.addSrcMap()
	c := e.c

	if b, noDynamics := c.scope.lookupName(e.name); noDynamics {
		if c.scope.strict {
			c.checkIdentifierLName(e.name, e.offset)
		}
		emitRight(false)
		if b != nil {
			if putOnStack {
				b.emitSet()
			} else {
				b.emitSetP()
			}
		} else {
			if c.scope.strict {
				c.emit(setGlobalStrict(e.name))
			} else {
				c.emit(setGlobal(e.name))
			}
			if !putOnStack {
				c.emit(pop)
			}
		}
	} else {
		c.emitVarRef(e.name, e.offset, b)
		emitRight(true)
		if putOnStack {
			c.emit(putValue)
		} else {
			c.emit(putValueP)
		}
	}
}

func (e *compiledIdentifierExpr) emitVarSetter(valueExpr compiledExpr, putOnStack bool) {
	e.emitVarSetter1(putOnStack, func(bool) {
		e.c.emitNamedOrConst(valueExpr, e.name)
	})
}

func (c *compiler) emitVarRef(name unistring.String, offset int, b *binding) {
	if c.scope.strict {
		c.checkIdentifierLName(name, offset)
	}

	if b != nil {
		b.emitResolveVar(c.scope.strict)
	} else {
		if c.scope.strict {
			c.emit(resolveVar1Strict(name))
		} else {
			c.emit(resolveVar1(name))
		}
	}
}

func (e *compiledIdentifierExpr) emitRef() {
	b, _ := e.c.scope.lookupName(e.name)
	e.c.emitVarRef(e.name, e.offset, b)
}

func (e *compiledIdentifierExpr) emitSetter(valueExpr compiledExpr, putOnStack bool) {
	e.emitVarSetter(valueExpr, putOnStack)
}

func (e *compiledIdentifierExpr) emitUnary(prepare, body func(), postfix, putOnStack bool) {
	if putOnStack {
		e.emitVarSetter1(true, func(isRef bool) {
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
		e.emitVarSetter1(false, func(isRef bool) {
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

type compiledSuperDotExpr struct {
	baseCompiledExpr
	name unistring.String
}

func (e *compiledSuperDotExpr) emitGetter(putOnStack bool) {
	e.c.emitLoadThis()
	e.c.emit(loadSuper)
	e.addSrcMap()
	e.c.emit(getPropRecv(e.name))
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *compiledSuperDotExpr) emitSetter(valueExpr compiledExpr, putOnStack bool) {
	e.c.emitLoadThis()
	e.c.emit(loadSuper)
	valueExpr.emitGetter(true)
	e.addSrcMap()
	if putOnStack {
		if e.c.scope.strict {
			e.c.emit(setPropRecvStrict(e.name))
		} else {
			e.c.emit(setPropRecv(e.name))
		}
	} else {
		if e.c.scope.strict {
			e.c.emit(setPropRecvStrictP(e.name))
		} else {
			e.c.emit(setPropRecvP(e.name))
		}
	}
}

func (e *compiledSuperDotExpr) emitUnary(prepare, body func(), postfix, putOnStack bool) {
	if !putOnStack {
		e.c.emitLoadThis()
		e.c.emit(loadSuper, dupLast(2), getPropRecv(e.name))
		body()
		e.addSrcMap()
		if e.c.scope.strict {
			e.c.emit(setPropRecvStrictP(e.name))
		} else {
			e.c.emit(setPropRecvP(e.name))
		}
	} else {
		if !postfix {
			e.c.emitLoadThis()
			e.c.emit(loadSuper, dupLast(2), getPropRecv(e.name))
			if prepare != nil {
				prepare()
			}
			body()
			e.addSrcMap()
			if e.c.scope.strict {
				e.c.emit(setPropRecvStrict(e.name))
			} else {
				e.c.emit(setPropRecv(e.name))
			}
		} else {
			e.c.emit(loadUndef)
			e.c.emitLoadThis()
			e.c.emit(loadSuper, dupLast(2), getPropRecv(e.name))
			if prepare != nil {
				prepare()
			}
			e.c.emit(rdupN(3))
			body()
			e.addSrcMap()
			if e.c.scope.strict {
				e.c.emit(setPropRecvStrictP(e.name))
			} else {
				e.c.emit(setPropRecvP(e.name))
			}
		}
	}
}

func (e *compiledSuperDotExpr) emitRef() {
	e.c.emitLoadThis()
	e.c.emit(loadSuper)
	if e.c.scope.strict {
		e.c.emit(getPropRefRecvStrict(e.name))
	} else {
		e.c.emit(getPropRefRecv(e.name))
	}
}

func (e *compiledSuperDotExpr) deleteExpr() compiledExpr {
	return e.c.superDeleteError(e.offset)
}

type compiledDotExpr struct {
	baseCompiledExpr
	left compiledExpr
	name unistring.String
}

type compiledPrivateDotExpr struct {
	baseCompiledExpr
	left compiledExpr
	name unistring.String
}

func (c *compiler) checkSuperBase(idx file.Idx) {
	if s := c.scope.nearestThis(); s != nil {
		switch s.funcType {
		case funcMethod, funcClsInit, funcCtor, funcDerivedCtor:
			return
		}
	}
	c.throwSyntaxError(int(idx)-1, "'super' keyword unexpected here")
	panic("unreachable")
}

func (c *compiler) compileDotExpression(v *ast.DotExpression) compiledExpr {
	if sup, ok := v.Left.(*ast.SuperExpression); ok {
		c.checkSuperBase(sup.Idx)
		r := &compiledSuperDotExpr{
			name: v.Identifier.Name,
		}
		r.init(c, v.Identifier.Idx)
		return r
	}

	r := &compiledDotExpr{
		left: c.compileExpression(v.Left),
		name: v.Identifier.Name,
	}
	r.init(c, v.Identifier.Idx)
	return r
}

func (c *compiler) compilePrivateDotExpression(v *ast.PrivateDotExpression) compiledExpr {
	r := &compiledPrivateDotExpr{
		left: c.compileExpression(v.Left),
		name: v.Identifier.Name,
	}
	r.init(c, v.Identifier.Idx)
	return r
}

func (e *compiledPrivateDotExpr) _emitGetter(rn *resolvedPrivateName, id *privateId) {
	if rn != nil {
		e.c.emit((*getPrivatePropRes)(rn))
	} else {
		e.c.emit((*getPrivatePropId)(id))
	}
}

func (e *compiledPrivateDotExpr) _emitSetter(rn *resolvedPrivateName, id *privateId) {
	if rn != nil {
		e.c.emit((*setPrivatePropRes)(rn))
	} else {
		e.c.emit((*setPrivatePropId)(id))
	}
}

func (e *compiledPrivateDotExpr) _emitSetterP(rn *resolvedPrivateName, id *privateId) {
	if rn != nil {
		e.c.emit((*setPrivatePropResP)(rn))
	} else {
		e.c.emit((*setPrivatePropIdP)(id))
	}
}

func (e *compiledPrivateDotExpr) emitGetter(putOnStack bool) {
	e.left.emitGetter(true)
	e.addSrcMap()
	rn, id := e.c.resolvePrivateName(e.name, e.offset)
	e._emitGetter(rn, id)
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *compiledPrivateDotExpr) emitSetter(v compiledExpr, putOnStack bool) {
	rn, id := e.c.resolvePrivateName(e.name, e.offset)
	e.left.emitGetter(true)
	v.emitGetter(true)
	e.addSrcMap()
	if putOnStack {
		e._emitSetter(rn, id)
	} else {
		e._emitSetterP(rn, id)
	}
}

func (e *compiledPrivateDotExpr) emitUnary(prepare, body func(), postfix, putOnStack bool) {
	rn, id := e.c.resolvePrivateName(e.name, e.offset)
	if !putOnStack {
		e.left.emitGetter(true)
		e.c.emit(dup)
		e._emitGetter(rn, id)
		body()
		e.addSrcMap()
		e._emitSetterP(rn, id)
	} else {
		if !postfix {
			e.left.emitGetter(true)
			e.c.emit(dup)
			e._emitGetter(rn, id)
			if prepare != nil {
				prepare()
			}
			body()
			e.addSrcMap()
			e._emitSetter(rn, id)
		} else {
			e.c.emit(loadUndef)
			e.left.emitGetter(true)
			e.c.emit(dup)
			e._emitGetter(rn, id)
			if prepare != nil {
				prepare()
			}
			e.c.emit(rdupN(2))
			body()
			e.addSrcMap()
			e._emitSetterP(rn, id)
		}
	}
}

func (e *compiledPrivateDotExpr) deleteExpr() compiledExpr {
	e.c.throwSyntaxError(e.offset, "Private fields can not be deleted")
	panic("unreachable")
}

func (e *compiledPrivateDotExpr) emitRef() {
	e.left.emitGetter(true)
	rn, id := e.c.resolvePrivateName(e.name, e.offset)
	if rn != nil {
		e.c.emit((*getPrivateRefRes)(rn))
	} else {
		e.c.emit((*getPrivateRefId)(id))
	}
}

type compiledSuperBracketExpr struct {
	baseCompiledExpr
	member compiledExpr
}

func (e *compiledSuperBracketExpr) emitGetter(putOnStack bool) {
	e.c.emitLoadThis()
	e.member.emitGetter(true)
	e.c.emit(loadSuper)
	e.addSrcMap()
	e.c.emit(getElemRecv)
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *compiledSuperBracketExpr) emitSetter(valueExpr compiledExpr, putOnStack bool) {
	e.c.emitLoadThis()
	e.member.emitGetter(true)
	e.c.emit(loadSuper)
	valueExpr.emitGetter(true)
	e.addSrcMap()
	if putOnStack {
		if e.c.scope.strict {
			e.c.emit(setElemRecvStrict)
		} else {
			e.c.emit(setElemRecv)
		}
	} else {
		if e.c.scope.strict {
			e.c.emit(setElemRecvStrictP)
		} else {
			e.c.emit(setElemRecvP)
		}
	}
}

func (e *compiledSuperBracketExpr) emitUnary(prepare, body func(), postfix, putOnStack bool) {
	if !putOnStack {
		e.c.emitLoadThis()
		e.member.emitGetter(true)
		e.c.emit(loadSuper, dupLast(3), getElemRecv)
		body()
		e.addSrcMap()
		if e.c.scope.strict {
			e.c.emit(setElemRecvStrictP)
		} else {
			e.c.emit(setElemRecvP)
		}
	} else {
		if !postfix {
			e.c.emitLoadThis()
			e.member.emitGetter(true)
			e.c.emit(loadSuper, dupLast(3), getElemRecv)
			if prepare != nil {
				prepare()
			}
			body()
			e.addSrcMap()
			if e.c.scope.strict {
				e.c.emit(setElemRecvStrict)
			} else {
				e.c.emit(setElemRecv)
			}
		} else {
			e.c.emit(loadUndef)
			e.c.emitLoadThis()
			e.member.emitGetter(true)
			e.c.emit(loadSuper, dupLast(3), getElemRecv)
			if prepare != nil {
				prepare()
			}
			e.c.emit(rdupN(4))
			body()
			e.addSrcMap()
			if e.c.scope.strict {
				e.c.emit(setElemRecvStrictP)
			} else {
				e.c.emit(setElemRecvP)
			}
		}
	}
}

func (e *compiledSuperBracketExpr) emitRef() {
	e.c.emitLoadThis()
	e.member.emitGetter(true)
	e.c.emit(loadSuper)
	if e.c.scope.strict {
		e.c.emit(getElemRefRecvStrict)
	} else {
		e.c.emit(getElemRefRecv)
	}
}

func (c *compiler) superDeleteError(offset int) compiledExpr {
	return c.compileEmitterExpr(func() {
		c.emit(throwConst{referenceError("Unsupported reference to 'super'")})
	}, file.Idx(offset+1))
}

func (e *compiledSuperBracketExpr) deleteExpr() compiledExpr {
	return e.c.superDeleteError(e.offset)
}

func (c *compiler) checkConstantString(expr compiledExpr) (unistring.String, bool) {
	if expr.constant() {
		if val, ex := c.evalConst(expr); ex == nil {
			if s, ok := val.(valueString); ok {
				return s.string(), true
			}
		}
	}
	return "", false
}

func (c *compiler) compileBracketExpression(v *ast.BracketExpression) compiledExpr {
	if sup, ok := v.Left.(*ast.SuperExpression); ok {
		c.checkSuperBase(sup.Idx)
		member := c.compileExpression(v.Member)
		if name, ok := c.checkConstantString(member); ok {
			r := &compiledSuperDotExpr{
				name: name,
			}
			r.init(c, v.LeftBracket)
			return r
		}

		r := &compiledSuperBracketExpr{
			member: member,
		}
		r.init(c, v.LeftBracket)
		return r
	}

	left := c.compileExpression(v.Left)
	member := c.compileExpression(v.Member)
	if name, ok := c.checkConstantString(member); ok {
		r := &compiledDotExpr{
			left: left,
			name: name,
		}
		r.init(c, v.LeftBracket)
		return r
	}

	r := &compiledBracketExpr{
		left:   left,
		member: member,
	}
	r.init(c, v.LeftBracket)
	return r
}

func (e *compiledDotExpr) emitGetter(putOnStack bool) {
	e.left.emitGetter(true)
	e.addSrcMap()
	e.c.emit(getProp(e.name))
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *compiledDotExpr) emitRef() {
	e.left.emitGetter(true)
	if e.c.scope.strict {
		e.c.emit(getPropRefStrict(e.name))
	} else {
		e.c.emit(getPropRef(e.name))
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
		e.addSrcMap()
		if e.c.scope.strict {
			e.c.emit(setPropStrictP(e.name))
		} else {
			e.c.emit(setPropP(e.name))
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
			e.addSrcMap()
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
			e.addSrcMap()
			if e.c.scope.strict {
				e.c.emit(setPropStrictP(e.name))
			} else {
				e.c.emit(setPropP(e.name))
			}
		}
	}
}

func (e *compiledDotExpr) deleteExpr() compiledExpr {
	r := &deletePropExpr{
		left: e.left,
		name: e.name,
	}
	r.init(e.c, file.Idx(e.offset)+1)
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

func (e *compiledBracketExpr) emitRef() {
	e.left.emitGetter(true)
	e.member.emitGetter(true)
	if e.c.scope.strict {
		e.c.emit(getElemRefStrict)
	} else {
		e.c.emit(getElemRef)
	}
}

func (e *compiledBracketExpr) emitSetter(valueExpr compiledExpr, putOnStack bool) {
	e.left.emitGetter(true)
	e.member.emitGetter(true)
	valueExpr.emitGetter(true)
	e.addSrcMap()
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
		e.c.emit(dupLast(2), getElem)
		body()
		e.addSrcMap()
		if e.c.scope.strict {
			e.c.emit(setElemStrict, pop)
		} else {
			e.c.emit(setElem, pop)
		}
	} else {
		if !postfix {
			e.left.emitGetter(true)
			e.member.emitGetter(true)
			e.c.emit(dupLast(2), getElem)
			if prepare != nil {
				prepare()
			}
			body()
			e.addSrcMap()
			if e.c.scope.strict {
				e.c.emit(setElemStrict)
			} else {
				e.c.emit(setElem)
			}
		} else {
			e.c.emit(loadUndef)
			e.left.emitGetter(true)
			e.member.emitGetter(true)
			e.c.emit(dupLast(2), getElem)
			if prepare != nil {
				prepare()
			}
			e.c.emit(rdupN(3))
			body()
			e.addSrcMap()
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
	r.init(e.c, file.Idx(e.offset)+1)
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
	switch e.operator {
	case token.ASSIGN:
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
	case token.EXPONENT:
		e.left.emitUnary(nil, func() {
			e.right.emitGetter(true)
			e.c.emit(exp)
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
		e.c.assert(false, e.offset, "Unknown assign operator: %s", e.operator.String())
		panic("unreachable")
	}
}

func (e *compiledLiteral) emitGetter(putOnStack bool) {
	if putOnStack {
		e.c.emit(loadVal(e.c.p.defineLiteralValue(e.val)))
	}
}

func (e *compiledLiteral) constant() bool {
	return true
}

func (e *compiledTemplateLiteral) emitGetter(putOnStack bool) {
	if e.tag == nil {
		if len(e.elements) == 0 {
			e.c.emit(loadVal(e.c.p.defineLiteralValue(stringEmpty)))
		} else {
			tail := e.elements[len(e.elements)-1].Parsed
			if len(e.elements) == 1 {
				e.c.emit(loadVal(e.c.p.defineLiteralValue(stringValueFromRaw(tail))))
			} else {
				stringCount := 0
				if head := e.elements[0].Parsed; head != "" {
					e.c.emit(loadVal(e.c.p.defineLiteralValue(stringValueFromRaw(head))))
					stringCount++
				}
				e.expressions[0].emitGetter(true)
				e.c.emit(_toString{})
				stringCount++
				for i := 1; i < len(e.elements)-1; i++ {
					if elt := e.elements[i].Parsed; elt != "" {
						e.c.emit(loadVal(e.c.p.defineLiteralValue(stringValueFromRaw(elt))))
						stringCount++
					}
					e.expressions[i].emitGetter(true)
					e.c.emit(_toString{})
					stringCount++
				}
				if tail != "" {
					e.c.emit(loadVal(e.c.p.defineLiteralValue(stringValueFromRaw(tail))))
					stringCount++
				}
				e.c.emit(concatStrings(stringCount))
			}
		}
	} else {
		cooked := make([]Value, len(e.elements))
		raw := make([]Value, len(e.elements))
		for i, elt := range e.elements {
			raw[i] = &valueProperty{
				enumerable: true,
				value:      newStringValue(elt.Literal),
			}
			var cookedVal Value
			if elt.Valid {
				cookedVal = stringValueFromRaw(elt.Parsed)
			} else {
				cookedVal = _undefined
			}
			cooked[i] = &valueProperty{
				enumerable: true,
				value:      cookedVal,
			}
		}
		e.c.emitCallee(e.tag)
		e.c.emit(&getTaggedTmplObject{
			raw:    raw,
			cooked: cooked,
		})
		for _, expr := range e.expressions {
			expr.emitGetter(true)
		}
		e.c.emit(call(len(e.expressions) + 1))
	}
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (c *compiler) compileParameterBindingIdentifier(name unistring.String, offset int) (*binding, bool) {
	if c.scope.strict {
		c.checkIdentifierName(name, offset)
		c.checkIdentifierLName(name, offset)
	}
	return c.scope.bindNameShadow(name)
}

func (c *compiler) compileParameterPatternIdBinding(name unistring.String, offset int) {
	if _, unique := c.compileParameterBindingIdentifier(name, offset); !unique {
		c.throwSyntaxError(offset, "Duplicate parameter name not allowed in this context")
	}
}

func (c *compiler) compileParameterPatternBinding(item ast.Expression) {
	c.createBindings(item, c.compileParameterPatternIdBinding)
}

func (c *compiler) newCode(length, minCap int) (buf []instruction) {
	if c.codeScratchpad != nil {
		buf = c.codeScratchpad
		c.codeScratchpad = nil
	}
	if cap(buf) < minCap {
		buf = make([]instruction, length, minCap)
	} else {
		buf = buf[:length]
	}
	return
}

func (e *compiledFunctionLiteral) compile() (prg *Program, name unistring.String, length int, strict bool) {
	e.c.assert(e.typ != funcNone, e.offset, "compiledFunctionLiteral.typ is not set")

	savedPrg := e.c.p
	preambleLen := 8 // enter, boxThis, loadStack(0), initThis, createArgs, set, loadCallee, init
	e.c.p = &Program{
		src:  e.c.p.src,
		code: e.c.newCode(preambleLen, 16),
	}
	e.c.newScope()
	s := e.c.scope
	s.funcType = e.typ

	if e.name != nil {
		name = e.name.Name
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

	if !s.strict {
		s.strict = e.strict != nil
	}

	hasPatterns := false
	hasInits := false
	firstDupIdx := -1

	if e.parameterList.Rest != nil {
		hasPatterns = true // strictly speaking not, but we need to activate all the checks
	}

	// First, make sure that the first bindings correspond to the formal parameters
	for _, item := range e.parameterList.List {
		switch tgt := item.Target.(type) {
		case *ast.Identifier:
			offset := int(tgt.Idx) - 1
			b, unique := e.c.compileParameterBindingIdentifier(tgt.Name, offset)
			if !unique {
				firstDupIdx = offset
			}
			b.isArg = true
		case ast.Pattern:
			b := s.addBinding(int(item.Idx0()) - 1)
			b.isArg = true
			hasPatterns = true
		default:
			e.c.throwSyntaxError(int(item.Idx0())-1, "Unsupported BindingElement type: %T", item)
			return
		}
		if item.Initializer != nil {
			hasInits = true
		}

		if firstDupIdx >= 0 && (hasPatterns || hasInits || s.strict || e.typ == funcArrow || e.typ == funcMethod) {
			e.c.throwSyntaxError(firstDupIdx, "Duplicate parameter name not allowed in this context")
			return
		}

		if (hasPatterns || hasInits) && e.strict != nil {
			e.c.throwSyntaxError(int(e.strict.Idx)-1, "Illegal 'use strict' directive in function with non-simple parameter list")
			return
		}

		if !hasInits {
			length++
		}
	}

	var thisBinding *binding
	if e.typ != funcArrow {
		thisBinding = s.createThisBinding()
	}

	// create pattern bindings
	if hasPatterns {
		for _, item := range e.parameterList.List {
			switch tgt := item.Target.(type) {
			case *ast.Identifier:
				// we already created those in the previous loop, skipping
			default:
				e.c.compileParameterPatternBinding(tgt)
			}
		}
		if rest := e.parameterList.Rest; rest != nil {
			e.c.compileParameterPatternBinding(rest)
		}
	}

	paramsCount := len(e.parameterList.List)

	s.numArgs = paramsCount
	body := e.body
	funcs := e.c.extractFunctions(body)
	var calleeBinding *binding

	emitArgsRestMark := -1
	firstForwardRef := -1
	enterFunc2Mark := -1

	if hasPatterns || hasInits {
		if e.isExpr && e.name != nil {
			if b, created := s.bindNameLexical(e.name.Name, false, 0); created {
				b.isConst = true
				calleeBinding = b
			}
		}
		for i, item := range e.parameterList.List {
			if pattern, ok := item.Target.(ast.Pattern); ok {
				i := i
				e.c.compilePatternInitExpr(func() {
					if firstForwardRef == -1 {
						s.bindings[i].emitGet()
					} else {
						e.c.emit(loadStackLex(-i - 1))
					}
				}, item.Initializer, item.Target.Idx0()).emitGetter(true)
				e.c.emitPattern(pattern, func(target, init compiledExpr) {
					e.c.emitPatternLexicalAssign(target, init)
				}, false)
			} else if item.Initializer != nil {
				markGet := len(e.c.p.code)
				e.c.emit(nil)
				mark := len(e.c.p.code)
				e.c.emit(nil)
				e.c.emitExpr(e.c.compileExpression(item.Initializer), true)
				if firstForwardRef == -1 && (s.isDynamic() || s.bindings[i].useCount() > 0) {
					firstForwardRef = i
				}
				if firstForwardRef == -1 {
					s.bindings[i].emitGetAt(markGet)
				} else {
					e.c.p.code[markGet] = loadStackLex(-i - 1)
				}
				s.bindings[i].emitInitP()
				e.c.p.code[mark] = jdefP(len(e.c.p.code) - mark)
			} else {
				if firstForwardRef == -1 && s.bindings[i].useCount() > 0 {
					firstForwardRef = i
				}
				if firstForwardRef != -1 {
					e.c.emit(loadStackLex(-i - 1))
					s.bindings[i].emitInitP()
				}
			}
		}
		if rest := e.parameterList.Rest; rest != nil {
			e.c.emitAssign(rest, e.c.compileEmitterExpr(
				func() {
					emitArgsRestMark = len(e.c.p.code)
					e.c.emit(createArgsRestStack(paramsCount))
				}, rest.Idx0()),
				func(target, init compiledExpr) {
					e.c.emitPatternLexicalAssign(target, init)
				})
		}
		if firstForwardRef != -1 {
			for _, b := range s.bindings {
				b.inStash = true
			}
			s.argsInStash = true
			s.needStash = true
		}

		e.c.newBlockScope()
		varScope := e.c.scope
		varScope.variable = true
		enterFunc2Mark = len(e.c.p.code)
		e.c.emit(nil)
		e.c.compileDeclList(e.declarationList, false)
		e.c.createFunctionBindings(funcs)
		e.c.compileLexicalDeclarationsFuncBody(body, calleeBinding)
		for _, b := range varScope.bindings {
			if b.isVar {
				if parentBinding := s.boundNames[b.name]; parentBinding != nil && parentBinding != calleeBinding {
					parentBinding.emitGet()
					b.emitSetP()
				}
			}
		}
	} else {
		// To avoid triggering variable conflict when binding from non-strict direct eval().
		// Parameters are supposed to be in a parent scope, hence no conflict.
		for _, b := range s.bindings[:paramsCount] {
			b.isVar = true
		}
		e.c.compileDeclList(e.declarationList, true)
		e.c.createFunctionBindings(funcs)
		e.c.compileLexicalDeclarations(body, true)
		if e.isExpr && e.name != nil {
			if b, created := s.bindNameLexical(e.name.Name, false, 0); created {
				b.isConst = true
				calleeBinding = b
			}
		}
		if calleeBinding != nil {
			e.c.emit(loadCallee)
			calleeBinding.emitInitP()
		}
	}

	e.c.compileFunctions(funcs)
	e.c.compileStatements(body, false)

	var last ast.Statement
	if l := len(body); l > 0 {
		last = body[l-1]
	}
	if _, ok := last.(*ast.ReturnStatement); !ok {
		if e.typ == funcDerivedCtor {
			e.c.emit(loadUndef)
			thisBinding.markAccessPoint()
			e.c.emit(ret)
		} else {
			e.c.emit(loadUndef, ret)
		}
	}

	delta := 0
	code := e.c.p.code

	if s.isDynamic() && !s.argsInStash {
		s.moveArgsToStash()
	}

	if s.argsNeeded || s.isDynamic() && e.typ != funcArrow && e.typ != funcClsInit {
		if e.typ == funcClsInit {
			e.c.throwSyntaxError(e.offset, "'arguments' is not allowed in class field initializer or static initialization block")
		}
		b, created := s.bindNameLexical("arguments", false, 0)
		if created || b.isVar {
			if !s.argsInStash {
				s.moveArgsToStash()
			}
			if s.strict {
				b.isConst = true
			} else {
				b.isVar = e.c.scope.isFunction()
			}
			pos := preambleLen - 2
			delta += 2
			if s.strict || hasPatterns || hasInits {
				code[pos] = createArgsUnmapped(paramsCount)
			} else {
				code[pos] = createArgsMapped(paramsCount)
			}
			pos++
			b.emitInitPAtScope(s, pos)
		}
	}

	if calleeBinding != nil {
		if !s.isDynamic() && calleeBinding.useCount() == 0 {
			s.deleteBinding(calleeBinding)
			calleeBinding = nil
		} else {
			delta++
			calleeBinding.emitInitPAtScope(s, preambleLen-delta)
			delta++
			code[preambleLen-delta] = loadCallee
		}
	}

	if thisBinding != nil {
		if !s.isDynamic() && thisBinding.useCount() == 0 {
			s.deleteBinding(thisBinding)
			thisBinding = nil
		} else {
			if thisBinding.inStash || s.isDynamic() {
				delta++
				thisBinding.emitInitAtScope(s, preambleLen-delta)
			}
		}
	}

	stashSize, stackSize := s.finaliseVarAlloc(0)

	if thisBinding != nil && thisBinding.inStash && (!s.argsInStash || stackSize > 0) {
		delta++
		code[preambleLen-delta] = loadStack(0)
	} // otherwise, 'this' will be at stack[sp-1], no need to load

	if !s.strict && thisBinding != nil {
		delta++
		code[preambleLen-delta] = boxThis
	}
	delta++
	delta = preambleLen - delta
	var enter instruction
	if stashSize > 0 || s.argsInStash {
		if firstForwardRef == -1 {
			enter1 := enterFunc{
				numArgs:     uint32(paramsCount),
				argsToStash: s.argsInStash,
				stashSize:   uint32(stashSize),
				stackSize:   uint32(stackSize),
				extensible:  s.dynamic,
				funcType:    e.typ,
			}
			if s.isDynamic() {
				enter1.names = s.makeNamesMap()
			}
			enter = &enter1
			if enterFunc2Mark != -1 {
				ef2 := &enterFuncBody{
					extensible: e.c.scope.dynamic,
					funcType:   e.typ,
				}
				e.c.updateEnterBlock(&ef2.enterBlock)
				e.c.p.code[enterFunc2Mark] = ef2
			}
		} else {
			enter1 := enterFunc1{
				stashSize:  uint32(stashSize),
				numArgs:    uint32(paramsCount),
				argsToCopy: uint32(firstForwardRef),
				extensible: s.dynamic,
				funcType:   e.typ,
			}
			if s.isDynamic() {
				enter1.names = s.makeNamesMap()
			}
			enter = &enter1
			if enterFunc2Mark != -1 {
				ef2 := &enterFuncBody{
					adjustStack: true,
					extensible:  e.c.scope.dynamic,
					funcType:    e.typ,
				}
				e.c.updateEnterBlock(&ef2.enterBlock)
				e.c.p.code[enterFunc2Mark] = ef2
			}
		}
		if emitArgsRestMark != -1 && s.argsInStash {
			e.c.p.code[emitArgsRestMark] = createArgsRestStash
		}
	} else {
		enter = &enterFuncStashless{
			stackSize: uint32(stackSize),
			args:      uint32(paramsCount),
		}
		if enterFunc2Mark != -1 {
			ef2 := &enterFuncBody{
				extensible: e.c.scope.dynamic,
				funcType:   e.typ,
			}
			e.c.updateEnterBlock(&ef2.enterBlock)
			e.c.p.code[enterFunc2Mark] = ef2
		}
	}
	code[delta] = enter
	s.trimCode(delta)

	strict = s.strict
	prg = e.c.p
	// e.c.p.dumpCode()
	if enterFunc2Mark != -1 {
		e.c.popScope()
	}
	e.c.popScope()
	e.c.p = savedPrg

	return
}

func (e *compiledFunctionLiteral) emitGetter(putOnStack bool) {
	p, name, length, strict := e.compile()
	switch e.typ {
	case funcArrow:
		e.c.emit(&newArrowFunc{newFunc: newFunc{prg: p, length: length, name: name, source: e.source, strict: strict}})
	case funcMethod, funcClsInit:
		e.c.emit(&newMethod{newFunc: newFunc{prg: p, length: length, name: name, source: e.source, strict: strict}, homeObjOffset: e.homeObjOffset})
	case funcRegular:
		e.c.emit(&newFunc{prg: p, length: length, name: name, source: e.source, strict: strict})
	default:
		e.c.throwSyntaxError(e.offset, "Unsupported func type: %v", e.typ)
	}
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (c *compiler) compileFunctionLiteral(v *ast.FunctionLiteral, isExpr bool) *compiledFunctionLiteral {
	strictBody := c.isStrictStatement(v.Body)
	if v.Name != nil && (c.scope.strict || strictBody != nil) {
		c.checkIdentifierLName(v.Name.Name, int(v.Name.Idx)-1)
	}
	r := &compiledFunctionLiteral{
		name:            v.Name,
		parameterList:   v.ParameterList,
		body:            v.Body.List,
		source:          v.Source,
		declarationList: v.DeclarationList,
		isExpr:          isExpr,
		typ:             funcRegular,
		strict:          strictBody,
	}
	r.init(c, v.Idx0())
	return r
}

type compiledClassLiteral struct {
	baseCompiledExpr
	name       *ast.Identifier
	superClass compiledExpr
	body       []ast.ClassElement
	lhsName    unistring.String
	source     string
	isExpr     bool
}

func (c *compiler) processKey(expr ast.Expression) (val unistring.String, computed bool) {
	keyExpr := c.compileExpression(expr)
	if keyExpr.constant() {
		v, ex := c.evalConst(keyExpr)
		if ex == nil {
			return v.string(), false
		}
	}
	keyExpr.emitGetter(true)
	computed = true
	return
}

func (e *compiledClassLiteral) processClassKey(expr ast.Expression) (privateName *privateName, key unistring.String, computed bool) {
	if p, ok := expr.(*ast.PrivateIdentifier); ok {
		privateName = e.c.classScope.getDeclaredPrivateId(p.Name)
		key = privateIdString(p.Name)
		return
	}
	key, computed = e.c.processKey(expr)
	return
}

type clsElement struct {
	key         unistring.String
	privateName *privateName
	initializer compiledExpr
	body        *compiledFunctionLiteral
	computed    bool
}

func (e *compiledClassLiteral) emitGetter(putOnStack bool) {
	e.c.newBlockScope()
	s := e.c.scope
	s.strict = true

	enter := &enterBlock{}
	mark0 := len(e.c.p.code)
	e.c.emit(enter)
	e.c.block = &block{
		typ:   blockScope,
		outer: e.c.block,
	}
	var clsBinding *binding
	var clsName unistring.String
	if name := e.name; name != nil {
		clsName = name.Name
		clsBinding = e.c.createLexicalIdBinding(clsName, true, int(name.Idx)-1)
	} else {
		clsName = e.lhsName
	}

	var ctorMethod *ast.MethodDefinition
	ctorMethodIdx := -1
	staticsCount := 0
	instanceFieldsCount := 0
	hasStaticPrivateMethods := false
	cs := &classScope{
		c:     e.c,
		outer: e.c.classScope,
	}

	for idx, elt := range e.body {
		switch elt := elt.(type) {
		case *ast.ClassStaticBlock:
			if len(elt.Block.List) > 0 {
				staticsCount++
			}
		case *ast.FieldDefinition:
			if id, ok := elt.Key.(*ast.PrivateIdentifier); ok {
				cs.declarePrivateId(id.Name, ast.PropertyKindValue, elt.Static, int(elt.Idx)-1)
			}
			if elt.Static {
				staticsCount++
			} else {
				instanceFieldsCount++
			}
		case *ast.MethodDefinition:
			if !elt.Static {
				if id, ok := elt.Key.(*ast.StringLiteral); ok {
					if !elt.Computed && id.Value == "constructor" {
						if ctorMethod != nil {
							e.c.throwSyntaxError(int(id.Idx)-1, "A class may only have one constructor")
						}
						ctorMethod = elt
						ctorMethodIdx = idx
						continue
					}
				}
			}
			if id, ok := elt.Key.(*ast.PrivateIdentifier); ok {
				cs.declarePrivateId(id.Name, elt.Kind, elt.Static, int(elt.Idx)-1)
				if elt.Static {
					hasStaticPrivateMethods = true
				}
			}
		default:
			e.c.assert(false, int(elt.Idx0())-1, "Unsupported static element: %T", elt)
		}
	}

	var staticInit *newStaticFieldInit
	if staticsCount > 0 || hasStaticPrivateMethods {
		staticInit = &newStaticFieldInit{}
		e.c.emit(staticInit)
	}

	var derived bool
	var newClassIns *newClass
	if superClass := e.superClass; superClass != nil {
		derived = true
		superClass.emitGetter(true)
		ndc := &newDerivedClass{
			newClass: newClass{
				name:   clsName,
				source: e.source,
			},
		}
		e.addSrcMap()
		e.c.emit(ndc)
		newClassIns = &ndc.newClass
	} else {
		newClassIns = &newClass{
			name:   clsName,
			source: e.source,
		}
		e.addSrcMap()
		e.c.emit(newClassIns)
	}

	e.c.classScope = cs

	if ctorMethod != nil {
		newClassIns.ctor, newClassIns.length = e.c.compileCtor(ctorMethod.Body, derived)
	}

	curIsPrototype := false

	instanceFields := make([]clsElement, 0, instanceFieldsCount)
	staticElements := make([]clsElement, 0, staticsCount)

	// stack at this point:
	//
	// staticFieldInit (if staticsCount > 0 || hasStaticPrivateMethods)
	// prototype
	// class function
	// <- sp

	for idx, elt := range e.body {
		if idx == ctorMethodIdx {
			continue
		}
		switch elt := elt.(type) {
		case *ast.ClassStaticBlock:
			if len(elt.Block.List) > 0 {
				f := e.c.compileFunctionLiteral(&ast.FunctionLiteral{
					Function:        elt.Idx0(),
					ParameterList:   &ast.ParameterList{},
					Body:            elt.Block,
					Source:          elt.Source,
					DeclarationList: elt.DeclarationList,
				}, true)
				f.typ = funcClsInit
				//f.lhsName = "<static_initializer>"
				f.homeObjOffset = 1
				staticElements = append(staticElements, clsElement{
					body: f,
				})
			}
		case *ast.FieldDefinition:
			privateName, key, computed := e.processClassKey(elt.Key)
			var el clsElement
			if elt.Initializer != nil {
				el.initializer = e.c.compileExpression(elt.Initializer)
			}
			el.computed = computed
			if computed {
				if elt.Static {
					if curIsPrototype {
						e.c.emit(defineComputedKey(5))
					} else {
						e.c.emit(defineComputedKey(4))
					}
				} else {
					if curIsPrototype {
						e.c.emit(defineComputedKey(3))
					} else {
						e.c.emit(defineComputedKey(2))
					}
				}
			} else {
				el.privateName = privateName
				el.key = key
			}
			if elt.Static {
				staticElements = append(staticElements, el)
			} else {
				instanceFields = append(instanceFields, el)
			}
		case *ast.MethodDefinition:
			if elt.Static {
				if curIsPrototype {
					e.c.emit(pop)
					curIsPrototype = false
				}
			} else {
				if !curIsPrototype {
					e.c.emit(dupN(1))
					curIsPrototype = true
				}
			}
			privateName, key, computed := e.processClassKey(elt.Key)
			lit := e.c.compileFunctionLiteral(elt.Body, true)
			lit.typ = funcMethod
			if computed {
				e.c.emit(_toPropertyKey{})
				lit.homeObjOffset = 2
			} else {
				lit.homeObjOffset = 1
				lit.lhsName = key
			}
			lit.emitGetter(true)
			if privateName != nil {
				var offset int
				if elt.Static {
					if curIsPrototype {
						/*
							staticInit
							proto
							cls
							proto
							method
							<- sp
						*/
						offset = 5
					} else {
						/*
							staticInit
							proto
							cls
							method
							<- sp
						*/
						offset = 4
					}
				} else {
					if curIsPrototype {
						offset = 3
					} else {
						offset = 2
					}
				}
				switch elt.Kind {
				case ast.PropertyKindGet:
					e.c.emit(&definePrivateGetter{
						definePrivateMethod: definePrivateMethod{
							idx:          privateName.idx,
							targetOffset: offset,
						},
					})
				case ast.PropertyKindSet:
					e.c.emit(&definePrivateSetter{
						definePrivateMethod: definePrivateMethod{
							idx:          privateName.idx,
							targetOffset: offset,
						},
					})
				default:
					e.c.emit(&definePrivateMethod{
						idx:          privateName.idx,
						targetOffset: offset,
					})
				}
			} else if computed {
				switch elt.Kind {
				case ast.PropertyKindGet:
					e.c.emit(&defineGetter{})
				case ast.PropertyKindSet:
					e.c.emit(&defineSetter{})
				default:
					e.c.emit(&defineMethod{})
				}
			} else {
				switch elt.Kind {
				case ast.PropertyKindGet:
					e.c.emit(&defineGetterKeyed{key: key})
				case ast.PropertyKindSet:
					e.c.emit(&defineSetterKeyed{key: key})
				default:
					e.c.emit(&defineMethodKeyed{key: key})
				}
			}
		}
	}
	if curIsPrototype {
		e.c.emit(pop)
	}

	if len(instanceFields) > 0 {
		newClassIns.initFields = e.compileFieldsAndStaticBlocks(instanceFields, "<instance_members_initializer>")
	}
	if staticInit != nil {
		if len(staticElements) > 0 {
			staticInit.initFields = e.compileFieldsAndStaticBlocks(staticElements, "<static_initializer>")
		}
	}

	env := e.c.classScope.instanceEnv
	if s.dynLookup {
		newClassIns.privateMethods, newClassIns.privateFields = env.methods, env.fields
	}
	newClassIns.numPrivateMethods = uint32(len(env.methods))
	newClassIns.numPrivateFields = uint32(len(env.fields))
	newClassIns.hasPrivateEnv = len(e.c.classScope.privateNames) > 0

	if (clsBinding != nil && clsBinding.useCount() > 0) || s.dynLookup {
		if clsBinding != nil {
			// Because this block may be in the middle of an expression, it's initial stack position
			// cannot be known, and therefore it may not have any stack variables.
			// Note, because clsBinding would be accessed through a function, it should already be in stash,
			// this is just to make sure.
			clsBinding.moveToStash()
			clsBinding.emitInit()
		}
	} else {
		if clsBinding != nil {
			s.deleteBinding(clsBinding)
			clsBinding = nil
		}
		e.c.p.code[mark0] = jump(1)
	}

	if staticsCount > 0 || hasStaticPrivateMethods {
		ise := &initStaticElements{}
		e.c.emit(ise)
		env := e.c.classScope.staticEnv
		staticInit.numPrivateFields = uint32(len(env.fields))
		staticInit.numPrivateMethods = uint32(len(env.methods))
		if s.dynLookup {
			// These cannot be set on staticInit, because it is executed before ClassHeritage, and therefore
			// the VM's PrivateEnvironment is still not set.
			ise.privateFields = env.fields
			ise.privateMethods = env.methods
		}
	} else {
		e.c.emit(endVariadic) // re-using as semantics match
	}

	if !putOnStack {
		e.c.emit(pop)
	}

	if clsBinding != nil || s.dynLookup {
		e.c.leaveScopeBlock(enter)
		e.c.assert(enter.stackSize == 0, e.offset, "enter.StackSize != 0 in compiledClassLiteral")
	} else {
		e.c.block = e.c.block.outer
	}
	if len(e.c.classScope.privateNames) > 0 {
		e.c.emit(popPrivateEnv{})
	}
	e.c.classScope = e.c.classScope.outer
	e.c.popScope()
}

func (e *compiledClassLiteral) compileFieldsAndStaticBlocks(elements []clsElement, funcName unistring.String) *Program {
	savedPrg := e.c.p
	savedBlock := e.c.block
	defer func() {
		e.c.p = savedPrg
		e.c.block = savedBlock
	}()

	e.c.block = &block{
		typ: blockScope,
	}

	e.c.p = &Program{
		src:      savedPrg.src,
		funcName: funcName,
		code:     e.c.newCode(2, 16),
	}

	e.c.newScope()
	s := e.c.scope
	s.funcType = funcClsInit
	thisBinding := s.createThisBinding()

	valIdx := 0
	for _, elt := range elements {
		if elt.body != nil {
			e.c.emit(dup) // this
			elt.body.emitGetter(true)
			elt.body.addSrcMap()
			e.c.emit(call(0), pop)
		} else {
			if elt.computed {
				e.c.emit(loadComputedKey(valIdx))
				valIdx++
			}
			if init := elt.initializer; init != nil {
				if !elt.computed {
					e.c.emitNamedOrConst(init, elt.key)
				} else {
					e.c.emitExpr(init, true)
				}
			} else {
				e.c.emit(loadUndef)
			}
			if elt.privateName != nil {
				e.c.emit(&definePrivateProp{
					idx: elt.privateName.idx,
				})
			} else if elt.computed {
				e.c.emit(defineProp{})
			} else {
				e.c.emit(definePropKeyed(elt.key))
			}
		}
	}
	e.c.emit(halt)
	if s.isDynamic() || thisBinding.useCount() > 0 {
		if s.isDynamic() || thisBinding.inStash {
			thisBinding.emitInitAt(1)
		}
	} else {
		s.deleteBinding(thisBinding)
	}
	stashSize, stackSize := s.finaliseVarAlloc(0)
	e.c.assert(stackSize == 0, e.offset, "stackSize != 0 in initFields")
	if stashSize > 0 {
		e.c.assert(stashSize == 1, e.offset, "stashSize != 1 in initFields")
		enter := &enterFunc{
			stashSize: 1,
			funcType:  funcClsInit,
		}
		if s.dynLookup {
			enter.names = s.makeNamesMap()
		}
		e.c.p.code[0] = enter
		s.trimCode(0)
	} else {
		s.trimCode(2)
	}
	res := e.c.p
	e.c.popScope()
	return res
}

func (c *compiler) compileClassLiteral(v *ast.ClassLiteral, isExpr bool) *compiledClassLiteral {
	if v.Name != nil {
		c.checkIdentifierLName(v.Name.Name, int(v.Name.Idx)-1)
	}
	r := &compiledClassLiteral{
		name:       v.Name,
		superClass: c.compileExpression(v.SuperClass),
		body:       v.Body,
		source:     v.Source,
		isExpr:     isExpr,
	}
	r.init(c, v.Idx0())
	return r
}

func (c *compiler) compileCtor(ctor *ast.FunctionLiteral, derived bool) (p *Program, length int) {
	f := c.compileFunctionLiteral(ctor, true)
	if derived {
		f.typ = funcDerivedCtor
	} else {
		f.typ = funcCtor
	}
	p, _, length, _ = f.compile()
	return
}

func (c *compiler) compileArrowFunctionLiteral(v *ast.ArrowFunctionLiteral) *compiledFunctionLiteral {
	var strictBody *ast.StringLiteral
	var body []ast.Statement
	switch b := v.Body.(type) {
	case *ast.BlockStatement:
		strictBody = c.isStrictStatement(b)
		body = b.List
	case *ast.ExpressionBody:
		body = []ast.Statement{
			&ast.ReturnStatement{
				Argument: b.Expression,
			},
		}
	default:
		c.throwSyntaxError(int(b.Idx0())-1, "Unsupported ConciseBody type: %T", b)
	}
	r := &compiledFunctionLiteral{
		parameterList:   v.ParameterList,
		body:            body,
		source:          v.Source,
		declarationList: v.DeclarationList,
		isExpr:          true,
		typ:             funcArrow,
		strict:          strictBody,
	}
	r.init(c, v.Idx0())
	return r
}

func (c *compiler) emitLoadThis() {
	b, eval := c.scope.lookupThis()
	if b != nil {
		b.emitGet()
	} else {
		if eval {
			c.emit(getThisDynamic{})
		} else {
			c.emit(loadGlobalObject)
		}
	}
}

func (e *compiledThisExpr) emitGetter(putOnStack bool) {
	e.addSrcMap()
	e.c.emitLoadThis()
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *compiledSuperExpr) emitGetter(putOnStack bool) {
	if putOnStack {
		e.c.emit(loadSuper)
	}
}

func (e *compiledNewExpr) emitGetter(putOnStack bool) {
	if e.isVariadic {
		e.c.emit(startVariadic)
	}
	e.callee.emitGetter(true)
	for _, expr := range e.args {
		expr.emitGetter(true)
	}
	e.addSrcMap()
	if e.isVariadic {
		e.c.emit(newVariadic, endVariadic)
	} else {
		e.c.emit(_new(len(e.args)))
	}
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (c *compiler) compileCallArgs(list []ast.Expression) (args []compiledExpr, isVariadic bool) {
	args = make([]compiledExpr, len(list))
	for i, argExpr := range list {
		if spread, ok := argExpr.(*ast.SpreadElement); ok {
			args[i] = c.compileSpreadCallArgument(spread)
			isVariadic = true
		} else {
			args[i] = c.compileExpression(argExpr)
		}
	}
	return
}

func (c *compiler) compileNewExpression(v *ast.NewExpression) compiledExpr {
	args, isVariadic := c.compileCallArgs(v.ArgumentList)
	r := &compiledNewExpr{
		compiledCallExpr: compiledCallExpr{
			callee:     c.compileExpression(v.Callee),
			args:       args,
			isVariadic: isVariadic,
		},
	}
	r.init(c, v.Idx0())
	return r
}

func (e *compiledNewTarget) emitGetter(putOnStack bool) {
	if s := e.c.scope.nearestThis(); s == nil || s.funcType == funcNone {
		e.c.throwSyntaxError(e.offset, "new.target expression is not allowed here")
	}
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
	c.assert(false, 0, "unknown exception type thrown while evaluating constant expression: %s", v.String())
	panic("unreachable")
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
		e.addSrcMap()
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
		e.c.assert(false, e.offset, "Unknown unary operator: %s", e.operator.String())
		panic("unreachable")
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
	e.c.emitExpr(e.right, true)
	e.c.p.code[j] = jeq1(len(e.c.p.code) - j)
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *compiledCoalesce) constant() bool {
	if e.left.constant() {
		if v, ex := e.c.evalConst(e.left); ex == nil {
			if v != _null && v != _undefined {
				return true
			}
			return e.right.constant()
		} else {
			return true
		}
	}

	return false
}

func (e *compiledCoalesce) emitGetter(putOnStack bool) {
	if e.left.constant() {
		if v, ex := e.c.evalConst(e.left); ex == nil {
			if v == _undefined || v == _null {
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
	e.c.emitExpr(e.right, true)
	e.c.p.code[j] = jcoalesc(len(e.c.p.code) - j)
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
	case token.EXPONENT:
		e.c.emit(exp)
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
		e.c.assert(false, e.offset, "Unknown operator: %s", e.operator.String())
		panic("unreachable")
	}

	if !putOnStack {
		e.c.emit(pop)
	}
}

func (c *compiler) compileBinaryExpression(v *ast.BinaryExpression) compiledExpr {

	switch v.Operator {
	case token.LOGICAL_OR:
		return c.compileLogicalOr(v.Left, v.Right, v.Idx0())
	case token.COALESCE:
		return c.compileCoalesce(v.Left, v.Right, v.Idx0())
	case token.LOGICAL_AND:
		return c.compileLogicalAnd(v.Left, v.Right, v.Idx0())
	}

	if id, ok := v.Left.(*ast.PrivateIdentifier); ok {
		return c.compilePrivateIn(id, v.Right, id.Idx)
	}

	r := &compiledBinaryExpr{
		left:     c.compileExpression(v.Left),
		right:    c.compileExpression(v.Right),
		operator: v.Operator,
	}
	r.init(c, v.Idx0())
	return r
}

type compiledPrivateIn struct {
	baseCompiledExpr
	id    unistring.String
	right compiledExpr
}

func (e *compiledPrivateIn) emitGetter(putOnStack bool) {
	e.right.emitGetter(true)
	rn, id := e.c.resolvePrivateName(e.id, e.offset)
	if rn != nil {
		e.c.emit((*privateInRes)(rn))
	} else {
		e.c.emit((*privateInId)(id))
	}
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (c *compiler) compilePrivateIn(id *ast.PrivateIdentifier, right ast.Expression, idx file.Idx) compiledExpr {
	r := &compiledPrivateIn{
		id:    id.Name,
		right: c.compileExpression(right),
	}
	r.init(c, idx)
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

func (c *compiler) compileCoalesce(left, right ast.Expression, idx file.Idx) compiledExpr {
	r := &compiledCoalesce{
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

func (e *compiledObjectLiteral) emitGetter(putOnStack bool) {
	e.addSrcMap()
	e.c.emit(newObject)
	hasProto := false
	for _, prop := range e.expr.Value {
		switch prop := prop.(type) {
		case *ast.PropertyKeyed:
			key, computed := e.c.processKey(prop.Key)
			valueExpr := e.c.compileExpression(prop.Value)
			var ne namedEmitter
			if fn, ok := valueExpr.(*compiledFunctionLiteral); ok {
				if fn.name == nil {
					ne = fn
				}
				switch prop.Kind {
				case ast.PropertyKindMethod, ast.PropertyKindGet, ast.PropertyKindSet:
					fn.typ = funcMethod
					if computed {
						fn.homeObjOffset = 2
					} else {
						fn.homeObjOffset = 1
					}
				}
			} else if v, ok := valueExpr.(namedEmitter); ok {
				ne = v
			}
			if computed {
				e.c.emit(_toPropertyKey{})
				e.c.emitExpr(valueExpr, true)
				switch prop.Kind {
				case ast.PropertyKindValue:
					if ne != nil {
						e.c.emit(setElem1Named)
					} else {
						e.c.emit(setElem1)
					}
				case ast.PropertyKindMethod:
					e.c.emit(&defineMethod{enumerable: true})
				case ast.PropertyKindGet:
					e.c.emit(&defineGetter{enumerable: true})
				case ast.PropertyKindSet:
					e.c.emit(&defineSetter{enumerable: true})
				default:
					e.c.assert(false, e.offset, "unknown property kind: %s", prop.Kind)
					panic("unreachable")
				}
			} else {
				isProto := key == __proto__ && !prop.Computed
				if isProto {
					if hasProto {
						e.c.throwSyntaxError(int(prop.Idx0())-1, "Duplicate __proto__ fields are not allowed in object literals")
					} else {
						hasProto = true
					}
				}
				if ne != nil && !isProto {
					ne.emitNamed(key)
				} else {
					e.c.emitExpr(valueExpr, true)
				}
				switch prop.Kind {
				case ast.PropertyKindValue:
					if isProto {
						e.c.emit(setProto)
					} else {
						e.c.emit(putProp(key))
					}
				case ast.PropertyKindMethod:
					e.c.emit(&defineMethodKeyed{key: key, enumerable: true})
				case ast.PropertyKindGet:
					e.c.emit(&defineGetterKeyed{key: key, enumerable: true})
				case ast.PropertyKindSet:
					e.c.emit(&defineSetterKeyed{key: key, enumerable: true})
				default:
					e.c.assert(false, e.offset, "unknown property kind: %s", prop.Kind)
					panic("unreachable")
				}
			}
		case *ast.PropertyShort:
			key := prop.Name.Name
			if prop.Initializer != nil {
				e.c.throwSyntaxError(int(prop.Initializer.Idx0())-1, "Invalid shorthand property initializer")
			}
			if e.c.scope.strict && key == "let" {
				e.c.throwSyntaxError(e.offset, "'let' cannot be used as a shorthand property in strict mode")
			}
			e.c.compileIdentifierExpression(&prop.Name).emitGetter(true)
			e.c.emit(putProp(key))
		case *ast.SpreadElement:
			e.c.compileExpression(prop.Expression).emitGetter(true)
			e.c.emit(copySpread)
		default:
			e.c.assert(false, e.offset, "unknown Property type: %T", prop)
			panic("unreachable")
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
	hasSpread := false
	mark := len(e.c.p.code)
	e.c.emit(nil)
	for _, v := range e.expr.Value {
		if spread, ok := v.(*ast.SpreadElement); ok {
			hasSpread = true
			e.c.compileExpression(spread.Expression).emitGetter(true)
			e.c.emit(pushArraySpread)
		} else {
			if v != nil {
				e.c.emitExpr(e.c.compileExpression(v), true)
			} else {
				e.c.emit(loadNil)
			}
			e.c.emit(pushArrayItem)
		}
	}
	var objCount uint32
	if !hasSpread {
		objCount = uint32(len(e.expr.Value))
	}
	e.c.p.code[mark] = newArray(objCount)
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

func (c *compiler) emitCallee(callee compiledExpr) (calleeName unistring.String) {
	switch callee := callee.(type) {
	case *compiledDotExpr:
		callee.left.emitGetter(true)
		c.emit(getPropCallee(callee.name))
	case *compiledPrivateDotExpr:
		callee.left.emitGetter(true)
		rn, id := c.resolvePrivateName(callee.name, callee.offset)
		if rn != nil {
			c.emit((*getPrivatePropResCallee)(rn))
		} else {
			c.emit((*getPrivatePropIdCallee)(id))
		}
	case *compiledSuperDotExpr:
		c.emitLoadThis()
		c.emit(loadSuper)
		c.emit(getPropRecvCallee(callee.name))
	case *compiledBracketExpr:
		callee.left.emitGetter(true)
		callee.member.emitGetter(true)
		c.emit(getElemCallee)
	case *compiledSuperBracketExpr:
		c.emitLoadThis()
		c.emit(loadSuper)
		callee.member.emitGetter(true)
		c.emit(getElemRecvCallee)
	case *compiledIdentifierExpr:
		calleeName = callee.name
		callee.emitGetterAndCallee()
	case *compiledOptionalChain:
		c.startOptChain()
		c.emitCallee(callee.expr)
		c.endOptChain()
	case *compiledOptional:
		c.emitCallee(callee.expr)
		c.block.conts = append(c.block.conts, len(c.p.code))
		c.emit(nil)
	case *compiledSuperExpr:
		// no-op
	default:
		c.emit(loadUndef)
		callee.emitGetter(true)
	}
	return
}

func (e *compiledCallExpr) emitGetter(putOnStack bool) {
	if e.isVariadic {
		e.c.emit(startVariadic)
	}
	calleeName := e.c.emitCallee(e.callee)

	for _, expr := range e.args {
		expr.emitGetter(true)
	}

	e.addSrcMap()
	if _, ok := e.callee.(*compiledSuperExpr); ok {
		b, eval := e.c.scope.lookupThis()
		e.c.assert(eval || b != nil, e.offset, "super call, but no 'this' binding")
		if eval {
			e.c.emit(resolveThisDynamic{})
		} else {
			b.markAccessPoint()
			e.c.emit(resolveThisStack{})
		}
		if e.isVariadic {
			e.c.emit(superCallVariadic)
		} else {
			e.c.emit(superCall(len(e.args)))
		}
	} else if calleeName == "eval" {
		foundVar := false
		for sc := e.c.scope; sc != nil; sc = sc.outer {
			if !foundVar && (sc.variable || sc.isFunction()) {
				foundVar = true
				if !sc.strict {
					sc.dynamic = true
				}
			}
			sc.dynLookup = true
		}

		if e.c.scope.strict {
			if e.isVariadic {
				e.c.emit(callEvalVariadicStrict)
			} else {
				e.c.emit(callEvalStrict(len(e.args)))
			}
		} else {
			if e.isVariadic {
				e.c.emit(callEvalVariadic)
			} else {
				e.c.emit(callEval(len(e.args)))
			}
		}
	} else {
		if e.isVariadic {
			e.c.emit(callVariadic)
		} else {
			e.c.emit(call(len(e.args)))
		}
	}
	if e.isVariadic {
		e.c.emit(endVariadic)
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

func (c *compiler) compileSpreadCallArgument(spread *ast.SpreadElement) compiledExpr {
	r := &compiledSpreadCallArgument{
		expr: c.compileExpression(spread.Expression),
	}
	r.init(c, spread.Idx0())
	return r
}

func (c *compiler) compileCallee(v ast.Expression) compiledExpr {
	if sup, ok := v.(*ast.SuperExpression); ok {
		if s := c.scope.nearestThis(); s != nil && s.funcType == funcDerivedCtor {
			e := &compiledSuperExpr{}
			e.init(c, sup.Idx)
			return e
		}
		c.throwSyntaxError(int(v.Idx0())-1, "'super' keyword unexpected here")
		panic("unreachable")
	}
	return c.compileExpression(v)
}

func (c *compiler) compileCallExpression(v *ast.CallExpression) compiledExpr {

	args := make([]compiledExpr, len(v.ArgumentList))
	isVariadic := false
	for i, argExpr := range v.ArgumentList {
		if spread, ok := argExpr.(*ast.SpreadElement); ok {
			args[i] = c.compileSpreadCallArgument(spread)
			isVariadic = true
		} else {
			args[i] = c.compileExpression(argExpr)
		}
	}

	r := &compiledCallExpr{
		args:       args,
		callee:     c.compileCallee(v.Callee),
		isVariadic: isVariadic,
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
	if c.scope.strict && len(v.Literal) > 1 && v.Literal[0] == '0' && v.Literal[1] <= '7' && v.Literal[1] >= '0' {
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
		c.assert(false, int(v.Idx)-1, "Unsupported number literal type: %T", v.Value)
		panic("unreachable")
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

func (c *compiler) compileTemplateLiteral(v *ast.TemplateLiteral) compiledExpr {
	r := &compiledTemplateLiteral{}
	if v.Tag != nil {
		r.tag = c.compileExpression(v.Tag)
	}
	ce := make([]compiledExpr, len(v.Expressions))
	for i, expr := range v.Expressions {
		ce[i] = c.compileExpression(expr)
	}
	r.expressions = ce
	r.elements = v.Elements
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

func (c *compiler) compileObjectAssignmentPattern(v *ast.ObjectPattern) compiledExpr {
	r := &compiledObjectAssignmentPattern{
		expr: v,
	}
	r.init(c, v.Idx0())
	return r
}

func (e *compiledObjectAssignmentPattern) emitGetter(putOnStack bool) {
	if putOnStack {
		e.c.emit(loadUndef)
	}
}

func (c *compiler) compileArrayAssignmentPattern(v *ast.ArrayPattern) compiledExpr {
	r := &compiledArrayAssignmentPattern{
		expr: v,
	}
	r.init(c, v.Idx0())
	return r
}

func (e *compiledArrayAssignmentPattern) emitGetter(putOnStack bool) {
	if putOnStack {
		e.c.emit(loadUndef)
	}
}

func (c *compiler) emitExpr(expr compiledExpr, putOnStack bool) {
	if expr.constant() {
		c.emitConst(expr, putOnStack)
	} else {
		expr.emitGetter(putOnStack)
	}
}

type namedEmitter interface {
	emitNamed(name unistring.String)
}

func (c *compiler) emitNamed(expr compiledExpr, name unistring.String) {
	if en, ok := expr.(namedEmitter); ok {
		en.emitNamed(name)
	} else {
		expr.emitGetter(true)
	}
}

func (c *compiler) emitNamedOrConst(expr compiledExpr, name unistring.String) {
	if expr.constant() {
		c.emitConst(expr, true)
	} else {
		c.emitNamed(expr, name)
	}
}

func (e *compiledFunctionLiteral) emitNamed(name unistring.String) {
	e.lhsName = name
	e.emitGetter(true)
}

func (e *compiledClassLiteral) emitNamed(name unistring.String) {
	e.lhsName = name
	e.emitGetter(true)
}

func (c *compiler) emitPattern(pattern ast.Pattern, emitter func(target, init compiledExpr), putOnStack bool) {
	switch pattern := pattern.(type) {
	case *ast.ObjectPattern:
		c.emitObjectPattern(pattern, emitter, putOnStack)
	case *ast.ArrayPattern:
		c.emitArrayPattern(pattern, emitter, putOnStack)
	default:
		c.assert(false, int(pattern.Idx0())-1, "unsupported Pattern: %T", pattern)
		panic("unreachable")
	}
}

func (c *compiler) emitAssign(target ast.Expression, init compiledExpr, emitAssignSimple func(target, init compiledExpr)) {
	pattern, isPattern := target.(ast.Pattern)
	if isPattern {
		init.emitGetter(true)
		c.emitPattern(pattern, emitAssignSimple, false)
	} else {
		emitAssignSimple(c.compileExpression(target), init)
	}
}

func (c *compiler) emitObjectPattern(pattern *ast.ObjectPattern, emitAssign func(target, init compiledExpr), putOnStack bool) {
	if pattern.Rest != nil {
		c.emit(createDestructSrc)
	} else {
		c.emit(checkObjectCoercible)
	}
	for _, prop := range pattern.Properties {
		switch prop := prop.(type) {
		case *ast.PropertyShort:
			c.emit(dup)
			emitAssign(c.compileIdentifierExpression(&prop.Name), c.compilePatternInitExpr(func() {
				c.emit(getProp(prop.Name.Name))
			}, prop.Initializer, prop.Idx0()))
		case *ast.PropertyKeyed:
			c.emit(dup)
			c.compileExpression(prop.Key).emitGetter(true)
			c.emit(_toPropertyKey{})
			var target ast.Expression
			var initializer ast.Expression
			if e, ok := prop.Value.(*ast.AssignExpression); ok {
				target = e.Left
				initializer = e.Right
			} else {
				target = prop.Value
			}
			c.emitAssign(target, c.compilePatternInitExpr(func() {
				c.emit(getKey)
			}, initializer, prop.Idx0()), emitAssign)
		default:
			c.throwSyntaxError(int(prop.Idx0()-1), "Unsupported AssignmentProperty type: %T", prop)
		}
	}
	if pattern.Rest != nil {
		emitAssign(c.compileExpression(pattern.Rest), c.compileEmitterExpr(func() {
			c.emit(copyRest)
		}, pattern.Rest.Idx0()))
		c.emit(pop)
	}
	if !putOnStack {
		c.emit(pop)
	}
}

func (c *compiler) emitArrayPattern(pattern *ast.ArrayPattern, emitAssign func(target, init compiledExpr), putOnStack bool) {
	c.emit(iterate)
	for _, elt := range pattern.Elements {
		switch elt := elt.(type) {
		case nil:
			c.emit(iterGetNextOrUndef{}, pop)
		case *ast.AssignExpression:
			c.emitAssign(elt.Left, c.compilePatternInitExpr(func() {
				c.emit(iterGetNextOrUndef{})
			}, elt.Right, elt.Idx0()), emitAssign)
		default:
			c.emitAssign(elt, c.compileEmitterExpr(func() {
				c.emit(iterGetNextOrUndef{})
			}, elt.Idx0()), emitAssign)
		}
	}
	if pattern.Rest != nil {
		c.emitAssign(pattern.Rest, c.compileEmitterExpr(func() {
			c.emit(newArrayFromIter)
		}, pattern.Rest.Idx0()), emitAssign)
	} else {
		c.emit(enumPopClose)
	}

	if !putOnStack {
		c.emit(pop)
	}
}

func (e *compiledObjectAssignmentPattern) emitSetter(valueExpr compiledExpr, putOnStack bool) {
	valueExpr.emitGetter(true)
	e.c.emitObjectPattern(e.expr, e.c.emitPatternAssign, putOnStack)
}

func (e *compiledArrayAssignmentPattern) emitSetter(valueExpr compiledExpr, putOnStack bool) {
	valueExpr.emitGetter(true)
	e.c.emitArrayPattern(e.expr, e.c.emitPatternAssign, putOnStack)
}

type compiledPatternInitExpr struct {
	baseCompiledExpr
	emitSrc func()
	def     compiledExpr
}

func (e *compiledPatternInitExpr) emitGetter(putOnStack bool) {
	if !putOnStack {
		return
	}
	e.emitSrc()
	if e.def != nil {
		mark := len(e.c.p.code)
		e.c.emit(nil)
		e.c.emitExpr(e.def, true)
		e.c.p.code[mark] = jdef(len(e.c.p.code) - mark)
	}
}

func (e *compiledPatternInitExpr) emitNamed(name unistring.String) {
	e.emitSrc()
	if e.def != nil {
		mark := len(e.c.p.code)
		e.c.emit(nil)
		e.c.emitNamedOrConst(e.def, name)
		e.c.p.code[mark] = jdef(len(e.c.p.code) - mark)
	}
}

func (c *compiler) compilePatternInitExpr(emitSrc func(), def ast.Expression, idx file.Idx) compiledExpr {
	r := &compiledPatternInitExpr{
		emitSrc: emitSrc,
		def:     c.compileExpression(def),
	}
	r.init(c, idx)
	return r
}

type compiledEmitterExpr struct {
	baseCompiledExpr
	emitter      func()
	namedEmitter func(name unistring.String)
}

func (e *compiledEmitterExpr) emitGetter(putOnStack bool) {
	if e.emitter != nil {
		e.emitter()
	} else {
		e.namedEmitter("")
	}
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *compiledEmitterExpr) emitNamed(name unistring.String) {
	if e.namedEmitter != nil {
		e.namedEmitter(name)
	} else {
		e.emitter()
	}
}

func (c *compiler) compileEmitterExpr(emitter func(), idx file.Idx) *compiledEmitterExpr {
	r := &compiledEmitterExpr{
		emitter: emitter,
	}
	r.init(c, idx)
	return r
}

func (e *compiledSpreadCallArgument) emitGetter(putOnStack bool) {
	e.expr.emitGetter(putOnStack)
	if putOnStack {
		e.c.emit(pushSpread)
	}
}

func (c *compiler) startOptChain() {
	c.block = &block{
		typ:   blockOptChain,
		outer: c.block,
	}
}

func (c *compiler) endOptChain() {
	lbl := len(c.p.code)
	for _, item := range c.block.breaks {
		c.p.code[item] = jopt(lbl - item)
	}
	for _, item := range c.block.conts {
		c.p.code[item] = joptc(lbl - item)
	}
	c.block = c.block.outer
}

func (e *compiledOptionalChain) emitGetter(putOnStack bool) {
	e.c.startOptChain()
	e.expr.emitGetter(true)
	e.c.endOptChain()
	if !putOnStack {
		e.c.emit(pop)
	}
}

func (e *compiledOptional) emitGetter(putOnStack bool) {
	e.expr.emitGetter(putOnStack)
	if putOnStack {
		e.c.block.breaks = append(e.c.block.breaks, len(e.c.p.code))
		e.c.emit(nil)
	}
}
