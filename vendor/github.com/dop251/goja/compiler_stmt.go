package goja

import (
	"fmt"
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
	"github.com/dop251/goja/token"
	"github.com/dop251/goja/unistring"
)

func (c *compiler) compileStatement(v ast.Statement, needResult bool) {

	switch v := v.(type) {
	case *ast.BlockStatement:
		c.compileBlockStatement(v, needResult)
	case *ast.ExpressionStatement:
		c.compileExpressionStatement(v, needResult)
	case *ast.VariableStatement:
		c.compileVariableStatement(v)
	case *ast.LexicalDeclaration:
		c.compileLexicalDeclaration(v)
	case *ast.ReturnStatement:
		c.compileReturnStatement(v)
	case *ast.IfStatement:
		c.compileIfStatement(v, needResult)
	case *ast.DoWhileStatement:
		c.compileDoWhileStatement(v, needResult)
	case *ast.ForStatement:
		c.compileForStatement(v, needResult)
	case *ast.ForInStatement:
		c.compileForInStatement(v, needResult)
	case *ast.ForOfStatement:
		c.compileForOfStatement(v, needResult)
	case *ast.WhileStatement:
		c.compileWhileStatement(v, needResult)
	case *ast.BranchStatement:
		c.compileBranchStatement(v)
	case *ast.TryStatement:
		c.compileTryStatement(v, needResult)
	case *ast.ThrowStatement:
		c.compileThrowStatement(v)
	case *ast.SwitchStatement:
		c.compileSwitchStatement(v, needResult)
	case *ast.LabelledStatement:
		c.compileLabeledStatement(v, needResult)
	case *ast.EmptyStatement:
		c.compileEmptyStatement(needResult)
	case *ast.FunctionDeclaration:
		c.compileStandaloneFunctionDecl(v)
		// note functions inside blocks are hoisted to the top of the block and are compiled using compileFunctions()
	case *ast.WithStatement:
		c.compileWithStatement(v, needResult)
	case *ast.DebuggerStatement:
	default:
		panic(fmt.Errorf("Unknown statement type: %T", v))
	}
}

func (c *compiler) compileLabeledStatement(v *ast.LabelledStatement, needResult bool) {
	label := v.Label.Name
	if c.scope.strict {
		c.checkIdentifierName(label, int(v.Label.Idx)-1)
	}
	for b := c.block; b != nil; b = b.outer {
		if b.label == label {
			c.throwSyntaxError(int(v.Label.Idx-1), "Label '%s' has already been declared", label)
		}
	}
	switch s := v.Statement.(type) {
	case *ast.ForInStatement:
		c.compileLabeledForInStatement(s, needResult, label)
	case *ast.ForOfStatement:
		c.compileLabeledForOfStatement(s, needResult, label)
	case *ast.ForStatement:
		c.compileLabeledForStatement(s, needResult, label)
	case *ast.WhileStatement:
		c.compileLabeledWhileStatement(s, needResult, label)
	case *ast.DoWhileStatement:
		c.compileLabeledDoWhileStatement(s, needResult, label)
	default:
		c.compileGenericLabeledStatement(s, needResult, label)
	}
}

func (c *compiler) updateEnterBlock(enter *enterBlock) {
	scope := c.scope
	stashSize, stackSize := 0, 0
	if scope.dynLookup {
		stashSize = len(scope.bindings)
		enter.names = scope.makeNamesMap()
	} else {
		for _, b := range scope.bindings {
			if b.inStash {
				stashSize++
			} else {
				stackSize++
			}
		}
	}
	enter.stashSize, enter.stackSize = uint32(stashSize), uint32(stackSize)
}

func (c *compiler) compileTryStatement(v *ast.TryStatement, needResult bool) {
	c.block = &block{
		typ:   blockTry,
		outer: c.block,
	}
	var lp int
	var bodyNeedResult bool
	var finallyBreaking *block
	if v.Finally != nil {
		lp, finallyBreaking = c.scanStatements(v.Finally.List)
	}
	if finallyBreaking != nil {
		c.block.breaking = finallyBreaking
		if lp == -1 {
			bodyNeedResult = finallyBreaking.needResult
		}
	} else {
		bodyNeedResult = needResult
	}
	lbl := len(c.p.code)
	c.emit(nil)
	if needResult {
		c.emit(clearResult)
	}
	c.compileBlockStatement(v.Body, bodyNeedResult)
	c.emit(halt)
	lbl2 := len(c.p.code)
	c.emit(nil)
	var catchOffset int
	if v.Catch != nil {
		catchOffset = len(c.p.code) - lbl
		if v.Catch.Parameter != nil {
			c.block = &block{
				typ:   blockScope,
				outer: c.block,
			}
			c.newBlockScope()
			list := v.Catch.Body.List
			funcs := c.extractFunctions(list)
			if _, ok := v.Catch.Parameter.(ast.Pattern); ok {
				// add anonymous binding for the catch parameter, note it must be first
				c.scope.addBinding(int(v.Catch.Idx0()) - 1)
			}
			c.createBindings(v.Catch.Parameter, func(name unistring.String, offset int) {
				if c.scope.strict {
					switch name {
					case "arguments", "eval":
						c.throwSyntaxError(offset, "Catch variable may not be eval or arguments in strict mode")
					}
				}
				c.scope.bindNameLexical(name, true, offset)
			})
			enter := &enterBlock{}
			c.emit(enter)
			if pattern, ok := v.Catch.Parameter.(ast.Pattern); ok {
				c.scope.bindings[0].emitGet()
				c.emitPattern(pattern, func(target, init compiledExpr) {
					c.emitPatternLexicalAssign(target, init)
				}, false)
			}
			for _, decl := range funcs {
				c.scope.bindNameLexical(decl.Function.Name.Name, true, int(decl.Function.Name.Idx1())-1)
			}
			c.compileLexicalDeclarations(list, true)
			c.compileFunctions(funcs)
			c.compileStatements(list, bodyNeedResult)
			c.leaveScopeBlock(enter)
			if c.scope.dynLookup || c.scope.bindings[0].inStash {
				c.p.code[lbl+catchOffset] = &enterCatchBlock{
					names:     enter.names,
					stashSize: enter.stashSize,
					stackSize: enter.stackSize,
				}
			} else {
				enter.stackSize--
			}
			c.popScope()
		} else {
			c.emit(pop)
			c.compileBlockStatement(v.Catch.Body, bodyNeedResult)
		}
		c.emit(halt)
	}
	var finallyOffset int
	if v.Finally != nil {
		lbl1 := len(c.p.code)
		c.emit(nil)
		finallyOffset = len(c.p.code) - lbl
		if bodyNeedResult && finallyBreaking != nil && lp == -1 {
			c.emit(clearResult)
		}
		c.compileBlockStatement(v.Finally, false)
		c.emit(halt, retFinally)

		c.p.code[lbl1] = jump(len(c.p.code) - lbl1)
	}
	c.p.code[lbl] = try{catchOffset: int32(catchOffset), finallyOffset: int32(finallyOffset)}
	c.p.code[lbl2] = jump(len(c.p.code) - lbl2)
	c.leaveBlock()
}

func (c *compiler) addSrcMap(node ast.Node) {
	c.p.addSrcMap(int(node.Idx0()) - 1)
}

func (c *compiler) compileThrowStatement(v *ast.ThrowStatement) {
	c.compileExpression(v.Argument).emitGetter(true)
	c.addSrcMap(v)
	c.emit(throw)
}

func (c *compiler) compileDoWhileStatement(v *ast.DoWhileStatement, needResult bool) {
	c.compileLabeledDoWhileStatement(v, needResult, "")
}

func (c *compiler) compileLabeledDoWhileStatement(v *ast.DoWhileStatement, needResult bool, label unistring.String) {
	c.block = &block{
		typ:        blockLoop,
		outer:      c.block,
		label:      label,
		needResult: needResult,
	}

	start := len(c.p.code)
	c.compileStatement(v.Body, needResult)
	c.block.cont = len(c.p.code)
	c.emitExpr(c.compileExpression(v.Test), true)
	c.emit(jeq(start - len(c.p.code)))
	c.leaveBlock()
}

func (c *compiler) compileForStatement(v *ast.ForStatement, needResult bool) {
	c.compileLabeledForStatement(v, needResult, "")
}

func (c *compiler) compileForHeadLexDecl(decl *ast.LexicalDeclaration, needResult bool) *enterBlock {
	c.block = &block{
		typ:        blockIterScope,
		outer:      c.block,
		needResult: needResult,
	}

	c.newBlockScope()
	enterIterBlock := &enterBlock{}
	c.emit(enterIterBlock)
	c.createLexicalBindings(decl)
	c.compileLexicalDeclaration(decl)
	return enterIterBlock
}

func (c *compiler) compileLabeledForStatement(v *ast.ForStatement, needResult bool, label unistring.String) {
	loopBlock := &block{
		typ:        blockLoop,
		outer:      c.block,
		label:      label,
		needResult: needResult,
	}
	c.block = loopBlock

	var enterIterBlock *enterBlock
	switch init := v.Initializer.(type) {
	case nil:
		// no-op
	case *ast.ForLoopInitializerLexicalDecl:
		enterIterBlock = c.compileForHeadLexDecl(&init.LexicalDeclaration, needResult)
	case *ast.ForLoopInitializerVarDeclList:
		for _, expr := range init.List {
			c.compileVarBinding(expr)
		}
	case *ast.ForLoopInitializerExpression:
		c.compileExpression(init.Expression).emitGetter(false)
	default:
		panic(fmt.Sprintf("Unsupported for loop initializer: %T", init))
	}

	if needResult {
		c.emit(clearResult) // initial result
	}

	if enterIterBlock != nil {
		c.emit(jump(1))
	}

	start := len(c.p.code)
	var j int
	testConst := false
	if v.Test != nil {
		expr := c.compileExpression(v.Test)
		if expr.constant() {
			r, ex := c.evalConst(expr)
			if ex == nil {
				if r.ToBoolean() {
					testConst = true
				} else {
					leave := c.enterDummyMode()
					c.compileStatement(v.Body, false)
					if v.Update != nil {
						c.compileExpression(v.Update).emitGetter(false)
					}
					leave()
					goto end
				}
			} else {
				expr.addSrcMap()
				c.emitThrow(ex.val)
				goto end
			}
		} else {
			expr.emitGetter(true)
			j = len(c.p.code)
			c.emit(nil)
		}
	}
	if needResult {
		c.emit(clearResult)
	}
	c.compileStatement(v.Body, needResult)
	loopBlock.cont = len(c.p.code)
	if enterIterBlock != nil {
		c.emit(jump(1))
	}
	if v.Update != nil {
		c.compileExpression(v.Update).emitGetter(false)
	}
	if enterIterBlock != nil {
		if c.scope.needStash || c.scope.isDynamic() {
			c.p.code[start-1] = copyStash{}
			c.p.code[loopBlock.cont] = copyStash{}
		} else {
			if l := len(c.p.code); l > loopBlock.cont {
				loopBlock.cont++
			} else {
				c.p.code = c.p.code[:l-1]
			}
		}
	}
	c.emit(jump(start - len(c.p.code)))
	if v.Test != nil {
		if !testConst {
			c.p.code[j] = jne(len(c.p.code) - j)
		}
	}
end:
	if enterIterBlock != nil {
		c.leaveScopeBlock(enterIterBlock)
		c.popScope()
	}
	c.leaveBlock()
}

func (c *compiler) compileForInStatement(v *ast.ForInStatement, needResult bool) {
	c.compileLabeledForInStatement(v, needResult, "")
}

func (c *compiler) compileForInto(into ast.ForInto, needResult bool) (enter *enterBlock) {
	switch into := into.(type) {
	case *ast.ForIntoExpression:
		c.compileExpression(into.Expression).emitSetter(&c.enumGetExpr, false)
	case *ast.ForIntoVar:
		if c.scope.strict && into.Binding.Initializer != nil {
			c.throwSyntaxError(int(into.Binding.Initializer.Idx0())-1, "for-in loop variable declaration may not have an initializer.")
		}
		switch target := into.Binding.Target.(type) {
		case *ast.Identifier:
			c.compileIdentifierExpression(target).emitSetter(&c.enumGetExpr, false)
		case ast.Pattern:
			c.emit(enumGet)
			c.emitPattern(target, c.emitPatternVarAssign, false)
		default:
			c.throwSyntaxError(int(target.Idx0()-1), "unsupported for-in var target: %T", target)
		}
	case *ast.ForDeclaration:

		c.block = &block{
			typ:        blockIterScope,
			outer:      c.block,
			needResult: needResult,
		}

		c.newBlockScope()
		enter = &enterBlock{}
		c.emit(enter)
		switch target := into.Target.(type) {
		case *ast.Identifier:
			b := c.createLexicalIdBinding(target.Name, into.IsConst, int(into.Idx)-1)
			c.emit(enumGet)
			b.emitInit()
		case ast.Pattern:
			c.createLexicalBinding(target, into.IsConst)
			c.emit(enumGet)
			c.emitPattern(target, func(target, init compiledExpr) {
				c.emitPatternLexicalAssign(target, init)
			}, false)
		default:
			c.throwSyntaxError(int(into.Idx)-1, "Unsupported ForBinding: %T", into.Target)
		}
	default:
		panic(fmt.Sprintf("Unsupported for-into: %T", into))
	}

	return
}

func (c *compiler) compileLabeledForInOfStatement(into ast.ForInto, source ast.Expression, body ast.Statement, iter, needResult bool, label unistring.String) {
	c.block = &block{
		typ:        blockLoopEnum,
		outer:      c.block,
		label:      label,
		needResult: needResult,
	}
	enterPos := -1
	if forDecl, ok := into.(*ast.ForDeclaration); ok {
		c.block = &block{
			typ:        blockScope,
			outer:      c.block,
			needResult: false,
		}
		c.newBlockScope()
		enterPos = len(c.p.code)
		c.emit(jump(1))
		c.createLexicalBinding(forDecl.Target, forDecl.IsConst)
	}
	c.compileExpression(source).emitGetter(true)
	if enterPos != -1 {
		s := c.scope
		used := len(c.block.breaks) > 0 || s.isDynamic()
		if !used {
			for _, b := range s.bindings {
				if b.useCount() > 0 {
					used = true
					break
				}
			}
		}
		if used {
			// We need the stack untouched because it contains the source.
			// This is not the most optimal way, but it's an edge case, hopefully quite rare.
			for _, b := range s.bindings {
				b.moveToStash()
			}
			enter := &enterBlock{}
			c.p.code[enterPos] = enter
			c.leaveScopeBlock(enter)
		} else {
			c.block = c.block.outer
		}
		c.popScope()
	}
	if iter {
		c.emit(iterateP)
	} else {
		c.emit(enumerate)
	}
	if needResult {
		c.emit(clearResult)
	}
	start := len(c.p.code)
	c.block.cont = start
	c.emit(nil)
	enterIterBlock := c.compileForInto(into, needResult)
	if needResult {
		c.emit(clearResult)
	}
	c.compileStatement(body, needResult)
	if enterIterBlock != nil {
		c.leaveScopeBlock(enterIterBlock)
		c.popScope()
	}
	c.emit(jump(start - len(c.p.code)))
	if iter {
		c.p.code[start] = iterNext(len(c.p.code) - start)
	} else {
		c.p.code[start] = enumNext(len(c.p.code) - start)
	}
	c.emit(enumPop, jump(2))
	c.leaveBlock()
	c.emit(enumPopClose)
}

func (c *compiler) compileLabeledForInStatement(v *ast.ForInStatement, needResult bool, label unistring.String) {
	c.compileLabeledForInOfStatement(v.Into, v.Source, v.Body, false, needResult, label)
}

func (c *compiler) compileForOfStatement(v *ast.ForOfStatement, needResult bool) {
	c.compileLabeledForOfStatement(v, needResult, "")
}

func (c *compiler) compileLabeledForOfStatement(v *ast.ForOfStatement, needResult bool, label unistring.String) {
	c.compileLabeledForInOfStatement(v.Into, v.Source, v.Body, true, needResult, label)
}

func (c *compiler) compileWhileStatement(v *ast.WhileStatement, needResult bool) {
	c.compileLabeledWhileStatement(v, needResult, "")
}

func (c *compiler) compileLabeledWhileStatement(v *ast.WhileStatement, needResult bool, label unistring.String) {
	c.block = &block{
		typ:        blockLoop,
		outer:      c.block,
		label:      label,
		needResult: needResult,
	}

	if needResult {
		c.emit(clearResult)
	}
	start := len(c.p.code)
	c.block.cont = start
	expr := c.compileExpression(v.Test)
	testTrue := false
	var j int
	if expr.constant() {
		if t, ex := c.evalConst(expr); ex == nil {
			if t.ToBoolean() {
				testTrue = true
			} else {
				c.compileStatementDummy(v.Body)
				goto end
			}
		} else {
			c.emitThrow(ex.val)
			goto end
		}
	} else {
		expr.emitGetter(true)
		j = len(c.p.code)
		c.emit(nil)
	}
	if needResult {
		c.emit(clearResult)
	}
	c.compileStatement(v.Body, needResult)
	c.emit(jump(start - len(c.p.code)))
	if !testTrue {
		c.p.code[j] = jne(len(c.p.code) - j)
	}
end:
	c.leaveBlock()
}

func (c *compiler) compileEmptyStatement(needResult bool) {
	if needResult {
		c.emit(clearResult)
	}
}

func (c *compiler) compileBranchStatement(v *ast.BranchStatement) {
	switch v.Token {
	case token.BREAK:
		c.compileBreak(v.Label, v.Idx)
	case token.CONTINUE:
		c.compileContinue(v.Label, v.Idx)
	default:
		panic(fmt.Errorf("Unknown branch statement token: %s", v.Token.String()))
	}
}

func (c *compiler) findBranchBlock(st *ast.BranchStatement) *block {
	switch st.Token {
	case token.BREAK:
		return c.findBreakBlock(st.Label, true)
	case token.CONTINUE:
		return c.findBreakBlock(st.Label, false)
	}
	return nil
}

func (c *compiler) findBreakBlock(label *ast.Identifier, isBreak bool) (res *block) {
	if label != nil {
		var found *block
		for b := c.block; b != nil; b = b.outer {
			if res == nil {
				if bb := b.breaking; bb != nil {
					res = bb
					if isBreak {
						return
					}
				}
			}
			if b.label == label.Name {
				found = b
				break
			}
		}
		if !isBreak && found != nil && found.typ != blockLoop && found.typ != blockLoopEnum {
			c.throwSyntaxError(int(label.Idx)-1, "Illegal continue statement: '%s' does not denote an iteration statement", label.Name)
		}
		if res == nil {
			res = found
		}
	} else {
		// find the nearest loop or switch (if break)
	L:
		for b := c.block; b != nil; b = b.outer {
			if bb := b.breaking; bb != nil {
				return bb
			}
			switch b.typ {
			case blockLoop, blockLoopEnum:
				res = b
				break L
			case blockSwitch:
				if isBreak {
					res = b
					break L
				}
			}
		}
	}

	return
}

func (c *compiler) emitBlockExitCode(label *ast.Identifier, idx file.Idx, isBreak bool) *block {
	block := c.findBreakBlock(label, isBreak)
	if block == nil {
		c.throwSyntaxError(int(idx)-1, "Could not find block")
		panic("unreachable")
	}
L:
	for b := c.block; b != block; b = b.outer {
		switch b.typ {
		case blockIterScope:
			if !isBreak && b.outer == block {
				break L
			}
			fallthrough
		case blockScope:
			b.breaks = append(b.breaks, len(c.p.code))
			c.emit(nil)
		case blockTry:
			c.emit(halt)
		case blockWith:
			c.emit(leaveWith)
		case blockLoopEnum:
			c.emit(enumPopClose)
		}
	}
	return block
}

func (c *compiler) compileBreak(label *ast.Identifier, idx file.Idx) {
	block := c.emitBlockExitCode(label, idx, true)
	block.breaks = append(block.breaks, len(c.p.code))
	c.emit(nil)
}

func (c *compiler) compileContinue(label *ast.Identifier, idx file.Idx) {
	block := c.emitBlockExitCode(label, idx, false)
	block.conts = append(block.conts, len(c.p.code))
	c.emit(nil)
}

func (c *compiler) compileIfBody(s ast.Statement, needResult bool) {
	if !c.scope.strict {
		if s, ok := s.(*ast.FunctionDeclaration); ok {
			c.compileFunction(s)
			if needResult {
				c.emit(clearResult)
			}
			return
		}
	}
	c.compileStatement(s, needResult)
}

func (c *compiler) compileIfBodyDummy(s ast.Statement) {
	leave := c.enterDummyMode()
	defer leave()
	c.compileIfBody(s, false)
}

func (c *compiler) compileIfStatement(v *ast.IfStatement, needResult bool) {
	test := c.compileExpression(v.Test)
	if needResult {
		c.emit(clearResult)
	}
	if test.constant() {
		r, ex := c.evalConst(test)
		if ex != nil {
			test.addSrcMap()
			c.emitThrow(ex.val)
			return
		}
		if r.ToBoolean() {
			c.compileIfBody(v.Consequent, needResult)
			if v.Alternate != nil {
				c.compileIfBodyDummy(v.Alternate)
			}
		} else {
			c.compileIfBodyDummy(v.Consequent)
			if v.Alternate != nil {
				c.compileIfBody(v.Alternate, needResult)
			} else {
				if needResult {
					c.emit(clearResult)
				}
			}
		}
		return
	}
	test.emitGetter(true)
	jmp := len(c.p.code)
	c.emit(nil)
	c.compileIfBody(v.Consequent, needResult)
	if v.Alternate != nil {
		jmp1 := len(c.p.code)
		c.emit(nil)
		c.p.code[jmp] = jne(len(c.p.code) - jmp)
		c.compileIfBody(v.Alternate, needResult)
		c.p.code[jmp1] = jump(len(c.p.code) - jmp1)
	} else {
		if needResult {
			c.emit(jump(2))
			c.p.code[jmp] = jne(len(c.p.code) - jmp)
			c.emit(clearResult)
		} else {
			c.p.code[jmp] = jne(len(c.p.code) - jmp)
		}
	}
}

func (c *compiler) compileReturnStatement(v *ast.ReturnStatement) {
	if v.Argument != nil {
		c.emitExpr(c.compileExpression(v.Argument), true)
	} else {
		c.emit(loadUndef)
	}
	for b := c.block; b != nil; b = b.outer {
		switch b.typ {
		case blockTry:
			c.emit(halt)
		case blockLoopEnum:
			c.emit(enumPopClose)
		}
	}
	c.emit(ret)
}

func (c *compiler) checkVarConflict(name unistring.String, offset int) {
	for sc := c.scope; sc != nil; sc = sc.outer {
		if b, exists := sc.boundNames[name]; exists && !b.isVar && !(b.isArg && sc != c.scope) {
			c.throwSyntaxError(offset, "Identifier '%s' has already been declared", name)
		}
		if sc.function {
			break
		}
	}
}

func (c *compiler) emitVarAssign(name unistring.String, offset int, init compiledExpr) {
	c.checkVarConflict(name, offset)
	if init != nil {
		b, noDyn := c.scope.lookupName(name)
		if noDyn {
			c.emitNamedOrConst(init, name)
			c.p.addSrcMap(offset)
			b.emitInit()
		} else {
			c.emitVarRef(name, offset, b)
			c.emitNamedOrConst(init, name)
			c.p.addSrcMap(offset)
			c.emit(initValueP)
		}
	}
}

func (c *compiler) compileVarBinding(expr *ast.Binding) {
	switch target := expr.Target.(type) {
	case *ast.Identifier:
		c.emitVarAssign(target.Name, int(target.Idx)-1, c.compileExpression(expr.Initializer))
	case ast.Pattern:
		c.compileExpression(expr.Initializer).emitGetter(true)
		c.emitPattern(target, c.emitPatternVarAssign, false)
	default:
		c.throwSyntaxError(int(target.Idx0()-1), "unsupported variable binding target: %T", target)
	}
}

func (c *compiler) emitLexicalAssign(name unistring.String, offset int, init compiledExpr) {
	b := c.scope.boundNames[name]
	if b == nil {
		panic("Lexical declaration for an unbound name")
	}
	if init != nil {
		c.emitNamedOrConst(init, name)
		c.p.addSrcMap(offset)
	} else {
		if b.isConst {
			c.throwSyntaxError(offset, "Missing initializer in const declaration")
		}
		c.emit(loadUndef)
	}
	if c.scope.outer != nil {
		b.emitInit()
	} else {
		c.emit(initGlobal(name))
	}
}

func (c *compiler) emitPatternVarAssign(target, init compiledExpr) {
	id := target.(*compiledIdentifierExpr)
	c.emitVarAssign(id.name, id.offset, init)
}

func (c *compiler) emitPatternLexicalAssign(target, init compiledExpr) {
	id := target.(*compiledIdentifierExpr)
	c.emitLexicalAssign(id.name, id.offset, init)
}

func (c *compiler) emitPatternAssign(target, init compiledExpr) {
	if id, ok := target.(*compiledIdentifierExpr); ok {
		b, noDyn := c.scope.lookupName(id.name)
		if noDyn {
			c.emitNamedOrConst(init, id.name)
			b.emitSetP()
		} else {
			c.emitVarRef(id.name, id.offset, b)
			c.emitNamedOrConst(init, id.name)
			c.emit(putValueP)
		}
	} else {
		target.emitRef()
		c.emitExpr(init, true)
		c.emit(putValueP)
	}
}

func (c *compiler) compileLexicalBinding(expr *ast.Binding) {
	switch target := expr.Target.(type) {
	case *ast.Identifier:
		c.emitLexicalAssign(target.Name, int(target.Idx)-1, c.compileExpression(expr.Initializer))
	case ast.Pattern:
		c.compileExpression(expr.Initializer).emitGetter(true)
		c.emitPattern(target, func(target, init compiledExpr) {
			c.emitPatternLexicalAssign(target, init)
		}, false)
	default:
		c.throwSyntaxError(int(target.Idx0()-1), "unsupported lexical binding target: %T", target)
	}
}

func (c *compiler) compileVariableStatement(v *ast.VariableStatement) {
	for _, expr := range v.List {
		c.compileVarBinding(expr)
	}
}

func (c *compiler) compileLexicalDeclaration(v *ast.LexicalDeclaration) {
	for _, e := range v.List {
		c.compileLexicalBinding(e)
	}
}

func (c *compiler) isEmptyResult(st ast.Statement) bool {
	switch st := st.(type) {
	case *ast.EmptyStatement, *ast.VariableStatement, *ast.LexicalDeclaration, *ast.FunctionDeclaration,
		*ast.BranchStatement, *ast.DebuggerStatement:
		return true
	case *ast.LabelledStatement:
		return c.isEmptyResult(st.Statement)
	case *ast.BlockStatement:
		for _, s := range st.List {
			if _, ok := s.(*ast.BranchStatement); ok {
				return true
			}
			if !c.isEmptyResult(s) {
				return false
			}
		}
		return true
	}
	return false
}

func (c *compiler) scanStatements(list []ast.Statement) (lastProducingIdx int, breakingBlock *block) {
	lastProducingIdx = -1
	for i, st := range list {
		if bs, ok := st.(*ast.BranchStatement); ok {
			if blk := c.findBranchBlock(bs); blk != nil {
				breakingBlock = blk
			}
			break
		}
		if !c.isEmptyResult(st) {
			lastProducingIdx = i
		}
	}
	return
}

func (c *compiler) compileStatementsNeedResult(list []ast.Statement, lastProducingIdx int) {
	if lastProducingIdx >= 0 {
		for _, st := range list[:lastProducingIdx] {
			if _, ok := st.(*ast.FunctionDeclaration); ok {
				continue
			}
			c.compileStatement(st, false)
		}
		c.compileStatement(list[lastProducingIdx], true)
	}
	var leave func()
	defer func() {
		if leave != nil {
			leave()
		}
	}()
	for _, st := range list[lastProducingIdx+1:] {
		if _, ok := st.(*ast.FunctionDeclaration); ok {
			continue
		}
		c.compileStatement(st, false)
		if leave == nil {
			if _, ok := st.(*ast.BranchStatement); ok {
				leave = c.enterDummyMode()
			}
		}
	}
}

func (c *compiler) compileStatements(list []ast.Statement, needResult bool) {
	lastProducingIdx, blk := c.scanStatements(list)
	if blk != nil {
		needResult = blk.needResult
	}
	if needResult {
		c.compileStatementsNeedResult(list, lastProducingIdx)
		return
	}
	for _, st := range list {
		if _, ok := st.(*ast.FunctionDeclaration); ok {
			continue
		}
		c.compileStatement(st, false)
	}
}

func (c *compiler) compileGenericLabeledStatement(v ast.Statement, needResult bool, label unistring.String) {
	c.block = &block{
		typ:        blockLabel,
		outer:      c.block,
		label:      label,
		needResult: needResult,
	}
	c.compileStatement(v, needResult)
	c.leaveBlock()
}

func (c *compiler) compileBlockStatement(v *ast.BlockStatement, needResult bool) {
	var scopeDeclared bool
	funcs := c.extractFunctions(v.List)
	if len(funcs) > 0 {
		c.newBlockScope()
		scopeDeclared = true
	}
	c.createFunctionBindings(funcs)
	scopeDeclared = c.compileLexicalDeclarations(v.List, scopeDeclared)

	var enter *enterBlock
	if scopeDeclared {
		c.block = &block{
			outer:      c.block,
			typ:        blockScope,
			needResult: needResult,
		}
		enter = &enterBlock{}
		c.emit(enter)
	}
	c.compileFunctions(funcs)
	c.compileStatements(v.List, needResult)
	if scopeDeclared {
		c.leaveScopeBlock(enter)
		c.popScope()
	}
}

func (c *compiler) compileExpressionStatement(v *ast.ExpressionStatement, needResult bool) {
	c.emitExpr(c.compileExpression(v.Expression), needResult)
	if needResult {
		c.emit(saveResult)
	}
}

func (c *compiler) compileWithStatement(v *ast.WithStatement, needResult bool) {
	if c.scope.strict {
		c.throwSyntaxError(int(v.With)-1, "Strict mode code may not include a with statement")
		return
	}
	c.compileExpression(v.Object).emitGetter(true)
	c.emit(enterWith)
	c.block = &block{
		outer:      c.block,
		typ:        blockWith,
		needResult: needResult,
	}
	c.newBlockScope()
	c.scope.dynamic = true
	c.compileStatement(v.Body, needResult)
	c.emit(leaveWith)
	c.leaveBlock()
	c.popScope()
}

func (c *compiler) compileSwitchStatement(v *ast.SwitchStatement, needResult bool) {
	c.block = &block{
		typ:        blockSwitch,
		outer:      c.block,
		needResult: needResult,
	}

	c.compileExpression(v.Discriminant).emitGetter(true)

	var funcs []*ast.FunctionDeclaration
	for _, s := range v.Body {
		f := c.extractFunctions(s.Consequent)
		funcs = append(funcs, f...)
	}
	var scopeDeclared bool
	if len(funcs) > 0 {
		c.newBlockScope()
		scopeDeclared = true
		c.createFunctionBindings(funcs)
	}

	for _, s := range v.Body {
		scopeDeclared = c.compileLexicalDeclarations(s.Consequent, scopeDeclared)
	}

	var enter *enterBlock
	var db *binding
	if scopeDeclared {
		c.block = &block{
			typ:        blockScope,
			outer:      c.block,
			needResult: needResult,
		}
		enter = &enterBlock{}
		c.emit(enter)
		// create anonymous variable for the discriminant
		bindings := c.scope.bindings
		var bb []*binding
		if cap(bindings) == len(bindings) {
			bb = make([]*binding, len(bindings)+1)
		} else {
			bb = bindings[:len(bindings)+1]
		}
		copy(bb[1:], bindings)
		db = &binding{
			scope:    c.scope,
			isConst:  true,
			isStrict: true,
		}
		bb[0] = db
		c.scope.bindings = bb
	}

	c.compileFunctions(funcs)

	if needResult {
		c.emit(clearResult)
	}

	jumps := make([]int, len(v.Body))

	for i, s := range v.Body {
		if s.Test != nil {
			if db != nil {
				db.emitGet()
			} else {
				c.emit(dup)
			}
			c.compileExpression(s.Test).emitGetter(true)
			c.emit(op_strict_eq)
			if db != nil {
				c.emit(jne(2))
			} else {
				c.emit(jne(3), pop)
			}
			jumps[i] = len(c.p.code)
			c.emit(nil)
		}
	}

	if db == nil {
		c.emit(pop)
	}
	jumpNoMatch := -1
	if v.Default != -1 {
		if v.Default != 0 {
			jumps[v.Default] = len(c.p.code)
			c.emit(nil)
		}
	} else {
		jumpNoMatch = len(c.p.code)
		c.emit(nil)
	}

	for i, s := range v.Body {
		if s.Test != nil || i != 0 {
			c.p.code[jumps[i]] = jump(len(c.p.code) - jumps[i])
		}
		c.compileStatements(s.Consequent, needResult)
	}

	if jumpNoMatch != -1 {
		c.p.code[jumpNoMatch] = jump(len(c.p.code) - jumpNoMatch)
	}
	if enter != nil {
		c.leaveScopeBlock(enter)
		enter.stackSize--
		c.popScope()
	}
	c.leaveBlock()
}
