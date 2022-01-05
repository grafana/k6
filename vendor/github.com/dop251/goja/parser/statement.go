package parser

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
	"github.com/dop251/goja/token"
	"github.com/go-sourcemap/sourcemap"
)

func (self *_parser) parseBlockStatement() *ast.BlockStatement {
	node := &ast.BlockStatement{}
	node.LeftBrace = self.expect(token.LEFT_BRACE)
	node.List = self.parseStatementList()
	node.RightBrace = self.expect(token.RIGHT_BRACE)

	return node
}

func (self *_parser) parseEmptyStatement() ast.Statement {
	idx := self.expect(token.SEMICOLON)
	return &ast.EmptyStatement{Semicolon: idx}
}

func (self *_parser) parseStatementList() (list []ast.Statement) {
	for self.token != token.RIGHT_BRACE && self.token != token.EOF {
		self.scope.allowLet = true
		list = append(list, self.parseStatement())
	}

	return
}

func (self *_parser) parseStatement() ast.Statement {

	if self.token == token.EOF {
		self.errorUnexpectedToken(self.token)
		return &ast.BadStatement{From: self.idx, To: self.idx + 1}
	}

	switch self.token {
	case token.SEMICOLON:
		return self.parseEmptyStatement()
	case token.LEFT_BRACE:
		return self.parseBlockStatement()
	case token.IF:
		return self.parseIfStatement()
	case token.DO:
		return self.parseDoWhileStatement()
	case token.WHILE:
		return self.parseWhileStatement()
	case token.FOR:
		return self.parseForOrForInStatement()
	case token.BREAK:
		return self.parseBreakStatement()
	case token.CONTINUE:
		return self.parseContinueStatement()
	case token.DEBUGGER:
		return self.parseDebuggerStatement()
	case token.WITH:
		return self.parseWithStatement()
	case token.VAR:
		return self.parseVariableStatement()
	case token.LET:
		tok := self.peek()
		if tok == token.LEFT_BRACKET || self.scope.allowLet && (tok == token.IDENTIFIER || tok == token.LET || tok == token.LEFT_BRACE) {
			return self.parseLexicalDeclaration(self.token)
		}
		self.insertSemicolon = true
	case token.CONST:
		return self.parseLexicalDeclaration(self.token)
	case token.FUNCTION:
		return &ast.FunctionDeclaration{
			Function: self.parseFunction(true),
		}
	case token.SWITCH:
		return self.parseSwitchStatement()
	case token.RETURN:
		return self.parseReturnStatement()
	case token.THROW:
		return self.parseThrowStatement()
	case token.TRY:
		return self.parseTryStatement()
	}

	expression := self.parseExpression()

	if identifier, isIdentifier := expression.(*ast.Identifier); isIdentifier && self.token == token.COLON {
		// LabelledStatement
		colon := self.idx
		self.next() // :
		label := identifier.Name
		for _, value := range self.scope.labels {
			if label == value {
				self.error(identifier.Idx0(), "Label '%s' already exists", label)
			}
		}
		self.scope.labels = append(self.scope.labels, label) // Push the label
		self.scope.allowLet = false
		statement := self.parseStatement()
		self.scope.labels = self.scope.labels[:len(self.scope.labels)-1] // Pop the label
		return &ast.LabelledStatement{
			Label:     identifier,
			Colon:     colon,
			Statement: statement,
		}
	}

	self.optionalSemicolon()

	return &ast.ExpressionStatement{
		Expression: expression,
	}
}

func (self *_parser) parseTryStatement() ast.Statement {

	node := &ast.TryStatement{
		Try:  self.expect(token.TRY),
		Body: self.parseBlockStatement(),
	}

	if self.token == token.CATCH {
		catch := self.idx
		self.next()
		var parameter ast.BindingTarget
		if self.token == token.LEFT_PARENTHESIS {
			self.next()
			parameter = self.parseBindingTarget()
			self.expect(token.RIGHT_PARENTHESIS)
		}
		node.Catch = &ast.CatchStatement{
			Catch:     catch,
			Parameter: parameter,
			Body:      self.parseBlockStatement(),
		}
	}

	if self.token == token.FINALLY {
		self.next()
		node.Finally = self.parseBlockStatement()
	}

	if node.Catch == nil && node.Finally == nil {
		self.error(node.Try, "Missing catch or finally after try")
		return &ast.BadStatement{From: node.Try, To: node.Body.Idx1()}
	}

	return node
}

func (self *_parser) parseFunctionParameterList() *ast.ParameterList {
	opening := self.expect(token.LEFT_PARENTHESIS)
	var list []*ast.Binding
	var rest ast.Expression
	for self.token != token.RIGHT_PARENTHESIS && self.token != token.EOF {
		if self.token == token.ELLIPSIS {
			self.next()
			rest = self.reinterpretAsDestructBindingTarget(self.parseAssignmentExpression())
			break
		}
		self.parseVariableDeclaration(&list)
		if self.token != token.RIGHT_PARENTHESIS {
			self.expect(token.COMMA)
		}
	}
	closing := self.expect(token.RIGHT_PARENTHESIS)

	return &ast.ParameterList{
		Opening: opening,
		List:    list,
		Rest:    rest,
		Closing: closing,
	}
}

func (self *_parser) parseFunction(declaration bool) *ast.FunctionLiteral {

	node := &ast.FunctionLiteral{
		Function: self.expect(token.FUNCTION),
	}

	var name *ast.Identifier
	if self.token == token.IDENTIFIER {
		name = self.parseIdentifier()
	} else if declaration {
		// Use expect error handling
		self.expect(token.IDENTIFIER)
	}
	node.Name = name
	node.ParameterList = self.parseFunctionParameterList()
	node.Body, node.DeclarationList = self.parseFunctionBlock()
	node.Source = self.slice(node.Idx0(), node.Idx1())

	return node
}

func (self *_parser) parseFunctionBlock() (body *ast.BlockStatement, declarationList []*ast.VariableDeclaration) {
	self.openScope()
	inFunction := self.scope.inFunction
	self.scope.inFunction = true
	defer func() {
		self.scope.inFunction = inFunction
		self.closeScope()
	}()
	body = self.parseBlockStatement()
	declarationList = self.scope.declarationList
	return
}

func (self *_parser) parseArrowFunctionBody() (ast.ConciseBody, []*ast.VariableDeclaration) {
	if self.token == token.LEFT_BRACE {
		return self.parseFunctionBlock()
	}
	return &ast.ExpressionBody{
		Expression: self.parseAssignmentExpression(),
	}, nil
}

func (self *_parser) parseDebuggerStatement() ast.Statement {
	idx := self.expect(token.DEBUGGER)

	node := &ast.DebuggerStatement{
		Debugger: idx,
	}

	self.semicolon()

	return node
}

func (self *_parser) parseReturnStatement() ast.Statement {
	idx := self.expect(token.RETURN)

	if !self.scope.inFunction {
		self.error(idx, "Illegal return statement")
		self.nextStatement()
		return &ast.BadStatement{From: idx, To: self.idx}
	}

	node := &ast.ReturnStatement{
		Return: idx,
	}

	if !self.implicitSemicolon && self.token != token.SEMICOLON && self.token != token.RIGHT_BRACE && self.token != token.EOF {
		node.Argument = self.parseExpression()
	}

	self.semicolon()

	return node
}

func (self *_parser) parseThrowStatement() ast.Statement {
	idx := self.expect(token.THROW)

	if self.implicitSemicolon {
		if self.chr == -1 { // Hackish
			self.error(idx, "Unexpected end of input")
		} else {
			self.error(idx, "Illegal newline after throw")
		}
		self.nextStatement()
		return &ast.BadStatement{From: idx, To: self.idx}
	}

	node := &ast.ThrowStatement{
		Throw:    idx,
		Argument: self.parseExpression(),
	}

	self.semicolon()

	return node
}

func (self *_parser) parseSwitchStatement() ast.Statement {
	self.expect(token.SWITCH)
	self.expect(token.LEFT_PARENTHESIS)
	node := &ast.SwitchStatement{
		Discriminant: self.parseExpression(),
		Default:      -1,
	}
	self.expect(token.RIGHT_PARENTHESIS)

	self.expect(token.LEFT_BRACE)

	inSwitch := self.scope.inSwitch
	self.scope.inSwitch = true
	defer func() {
		self.scope.inSwitch = inSwitch
	}()

	for index := 0; self.token != token.EOF; index++ {
		if self.token == token.RIGHT_BRACE {
			self.next()
			break
		}

		clause := self.parseCaseStatement()
		if clause.Test == nil {
			if node.Default != -1 {
				self.error(clause.Case, "Already saw a default in switch")
			}
			node.Default = index
		}
		node.Body = append(node.Body, clause)
	}

	return node
}

func (self *_parser) parseWithStatement() ast.Statement {
	self.expect(token.WITH)
	self.expect(token.LEFT_PARENTHESIS)
	node := &ast.WithStatement{
		Object: self.parseExpression(),
	}
	self.expect(token.RIGHT_PARENTHESIS)
	self.scope.allowLet = false
	node.Body = self.parseStatement()

	return node
}

func (self *_parser) parseCaseStatement() *ast.CaseStatement {

	node := &ast.CaseStatement{
		Case: self.idx,
	}
	if self.token == token.DEFAULT {
		self.next()
	} else {
		self.expect(token.CASE)
		node.Test = self.parseExpression()
	}
	self.expect(token.COLON)

	for {
		if self.token == token.EOF ||
			self.token == token.RIGHT_BRACE ||
			self.token == token.CASE ||
			self.token == token.DEFAULT {
			break
		}
		node.Consequent = append(node.Consequent, self.parseStatement())

	}

	return node
}

func (self *_parser) parseIterationStatement() ast.Statement {
	inIteration := self.scope.inIteration
	self.scope.inIteration = true
	defer func() {
		self.scope.inIteration = inIteration
	}()
	self.scope.allowLet = false
	return self.parseStatement()
}

func (self *_parser) parseForIn(idx file.Idx, into ast.ForInto) *ast.ForInStatement {

	// Already have consumed "<into> in"

	source := self.parseExpression()
	self.expect(token.RIGHT_PARENTHESIS)

	return &ast.ForInStatement{
		For:    idx,
		Into:   into,
		Source: source,
		Body:   self.parseIterationStatement(),
	}
}

func (self *_parser) parseForOf(idx file.Idx, into ast.ForInto) *ast.ForOfStatement {

	// Already have consumed "<into> of"

	source := self.parseAssignmentExpression()
	self.expect(token.RIGHT_PARENTHESIS)

	return &ast.ForOfStatement{
		For:    idx,
		Into:   into,
		Source: source,
		Body:   self.parseIterationStatement(),
	}
}

func (self *_parser) parseFor(idx file.Idx, initializer ast.ForLoopInitializer) *ast.ForStatement {

	// Already have consumed "<initializer> ;"

	var test, update ast.Expression

	if self.token != token.SEMICOLON {
		test = self.parseExpression()
	}
	self.expect(token.SEMICOLON)

	if self.token != token.RIGHT_PARENTHESIS {
		update = self.parseExpression()
	}
	self.expect(token.RIGHT_PARENTHESIS)

	return &ast.ForStatement{
		For:         idx,
		Initializer: initializer,
		Test:        test,
		Update:      update,
		Body:        self.parseIterationStatement(),
	}
}

func (self *_parser) parseForOrForInStatement() ast.Statement {
	idx := self.expect(token.FOR)
	self.expect(token.LEFT_PARENTHESIS)

	var initializer ast.ForLoopInitializer

	forIn := false
	forOf := false
	var into ast.ForInto
	if self.token != token.SEMICOLON {

		allowIn := self.scope.allowIn
		self.scope.allowIn = false
		tok := self.token
		if tok == token.LET {
			switch self.peek() {
			case token.IDENTIFIER, token.LEFT_BRACKET, token.LEFT_BRACE:
			default:
				tok = token.IDENTIFIER
			}
		}
		if tok == token.VAR || tok == token.LET || tok == token.CONST {
			idx := self.idx
			self.next()
			var list []*ast.Binding
			if tok == token.VAR {
				list = self.parseVarDeclarationList(idx)
			} else {
				list = self.parseVariableDeclarationList()
			}
			if len(list) == 1 {
				if self.token == token.IN {
					self.next() // in
					forIn = true
				} else if self.token == token.IDENTIFIER && self.literal == "of" {
					self.next()
					forOf = true
				}
			}
			if forIn || forOf {
				if list[0].Initializer != nil {
					self.error(list[0].Initializer.Idx0(), "for-in loop variable declaration may not have an initializer")
				}
				if tok == token.VAR {
					into = &ast.ForIntoVar{
						Binding: list[0],
					}
				} else {
					into = &ast.ForDeclaration{
						Idx:     idx,
						IsConst: tok == token.CONST,
						Target:  list[0].Target,
					}
				}
			} else {
				self.ensurePatternInit(list)
				if tok == token.VAR {
					initializer = &ast.ForLoopInitializerVarDeclList{
						List: list,
					}
				} else {
					initializer = &ast.ForLoopInitializerLexicalDecl{
						LexicalDeclaration: ast.LexicalDeclaration{
							Idx:   idx,
							Token: tok,
							List:  list,
						},
					}
				}
			}
		} else {
			expr := self.parseExpression()
			if self.token == token.IN {
				self.next()
				forIn = true
			} else if self.token == token.IDENTIFIER && self.literal == "of" {
				self.next()
				forOf = true
			}
			if forIn || forOf {
				switch e := expr.(type) {
				case *ast.Identifier, *ast.DotExpression, *ast.BracketExpression, *ast.Binding:
					// These are all acceptable
				case *ast.ObjectLiteral:
					expr = self.reinterpretAsObjectAssignmentPattern(e)
				case *ast.ArrayLiteral:
					expr = self.reinterpretAsArrayAssignmentPattern(e)
				default:
					self.error(idx, "Invalid left-hand side in for-in or for-of")
					self.nextStatement()
					return &ast.BadStatement{From: idx, To: self.idx}
				}
				into = &ast.ForIntoExpression{
					Expression: expr,
				}
			} else {
				initializer = &ast.ForLoopInitializerExpression{
					Expression: expr,
				}
			}
		}
		self.scope.allowIn = allowIn
	}

	if forIn {
		return self.parseForIn(idx, into)
	}
	if forOf {
		return self.parseForOf(idx, into)
	}

	self.expect(token.SEMICOLON)
	return self.parseFor(idx, initializer)
}

func (self *_parser) ensurePatternInit(list []*ast.Binding) {
	for _, item := range list {
		if _, ok := item.Target.(ast.Pattern); ok {
			if item.Initializer == nil {
				self.error(item.Idx1(), "Missing initializer in destructuring declaration")
				break
			}
		}
	}
}

func (self *_parser) parseVariableStatement() *ast.VariableStatement {

	idx := self.expect(token.VAR)

	list := self.parseVarDeclarationList(idx)
	self.ensurePatternInit(list)
	self.semicolon()

	return &ast.VariableStatement{
		Var:  idx,
		List: list,
	}
}

func (self *_parser) parseLexicalDeclaration(tok token.Token) *ast.LexicalDeclaration {
	idx := self.expect(tok)
	if !self.scope.allowLet {
		self.error(idx, "Lexical declaration cannot appear in a single-statement context")
	}

	list := self.parseVariableDeclarationList()
	self.ensurePatternInit(list)
	self.semicolon()

	return &ast.LexicalDeclaration{
		Idx:   idx,
		Token: tok,
		List:  list,
	}
}

func (self *_parser) parseDoWhileStatement() ast.Statement {
	inIteration := self.scope.inIteration
	self.scope.inIteration = true
	defer func() {
		self.scope.inIteration = inIteration
	}()

	self.expect(token.DO)
	node := &ast.DoWhileStatement{}
	if self.token == token.LEFT_BRACE {
		node.Body = self.parseBlockStatement()
	} else {
		self.scope.allowLet = false
		node.Body = self.parseStatement()
	}

	self.expect(token.WHILE)
	self.expect(token.LEFT_PARENTHESIS)
	node.Test = self.parseExpression()
	self.expect(token.RIGHT_PARENTHESIS)
	if self.token == token.SEMICOLON {
		self.next()
	}

	return node
}

func (self *_parser) parseWhileStatement() ast.Statement {
	self.expect(token.WHILE)
	self.expect(token.LEFT_PARENTHESIS)
	node := &ast.WhileStatement{
		Test: self.parseExpression(),
	}
	self.expect(token.RIGHT_PARENTHESIS)
	node.Body = self.parseIterationStatement()

	return node
}

func (self *_parser) parseIfStatement() ast.Statement {
	self.expect(token.IF)
	self.expect(token.LEFT_PARENTHESIS)
	node := &ast.IfStatement{
		Test: self.parseExpression(),
	}
	self.expect(token.RIGHT_PARENTHESIS)

	if self.token == token.LEFT_BRACE {
		node.Consequent = self.parseBlockStatement()
	} else {
		self.scope.allowLet = false
		node.Consequent = self.parseStatement()
	}

	if self.token == token.ELSE {
		self.next()
		self.scope.allowLet = false
		node.Alternate = self.parseStatement()
	}

	return node
}

func (self *_parser) parseSourceElements() (body []ast.Statement) {
	for self.token != token.EOF {
		self.scope.allowLet = true
		body = append(body, self.parseStatement())
	}

	return body
}

func (self *_parser) parseProgram() *ast.Program {
	self.openScope()
	defer self.closeScope()
	prg := &ast.Program{
		Body:            self.parseSourceElements(),
		DeclarationList: self.scope.declarationList,
		File:            self.file,
	}
	self.file.SetSourceMap(self.parseSourceMap())
	return prg
}

func extractSourceMapLine(str string) string {
	for {
		p := strings.LastIndexByte(str, '\n')
		line := str[p+1:]
		if line != "" && line != "})" {
			if strings.HasPrefix(line, "//# sourceMappingURL=") {
				return line
			}
			break
		}
		if p >= 0 {
			str = str[:p]
		} else {
			break
		}
	}
	return ""
}

func (self *_parser) parseSourceMap() *sourcemap.Consumer {
	if self.opts.disableSourceMaps {
		return nil
	}
	if smLine := extractSourceMapLine(self.str); smLine != "" {
		urlIndex := strings.Index(smLine, "=")
		urlStr := smLine[urlIndex+1:]

		var data []byte
		var err error
		if strings.HasPrefix(urlStr, "data:application/json") {
			b64Index := strings.Index(urlStr, ",")
			b64 := urlStr[b64Index+1:]
			data, err = base64.StdEncoding.DecodeString(b64)
		} else {
			if sourceURL := file.ResolveSourcemapURL(self.file.Name(), urlStr); sourceURL != nil {
				if self.opts.sourceMapLoader != nil {
					data, err = self.opts.sourceMapLoader(sourceURL.String())
				} else {
					if sourceURL.Scheme == "" || sourceURL.Scheme == "file" {
						data, err = ioutil.ReadFile(sourceURL.Path)
					} else {
						err = fmt.Errorf("unsupported source map URL scheme: %s", sourceURL.Scheme)
					}
				}
			}
		}

		if err != nil {
			self.error(file.Idx(0), "Could not load source map: %v", err)
			return nil
		}
		if data == nil {
			return nil
		}

		if sm, err := sourcemap.Parse(self.file.Name(), data); err == nil {
			return sm
		} else {
			self.error(file.Idx(0), "Could not parse source map: %v", err)
		}
	}
	return nil
}

func (self *_parser) parseBreakStatement() ast.Statement {
	idx := self.expect(token.BREAK)
	semicolon := self.implicitSemicolon
	if self.token == token.SEMICOLON {
		semicolon = true
		self.next()
	}

	if semicolon || self.token == token.RIGHT_BRACE {
		self.implicitSemicolon = false
		if !self.scope.inIteration && !self.scope.inSwitch {
			goto illegal
		}
		return &ast.BranchStatement{
			Idx:   idx,
			Token: token.BREAK,
		}
	}

	if self.token == token.IDENTIFIER {
		identifier := self.parseIdentifier()
		if !self.scope.hasLabel(identifier.Name) {
			self.error(idx, "Undefined label '%s'", identifier.Name)
			return &ast.BadStatement{From: idx, To: identifier.Idx1()}
		}
		self.semicolon()
		return &ast.BranchStatement{
			Idx:   idx,
			Token: token.BREAK,
			Label: identifier,
		}
	}

	self.expect(token.IDENTIFIER)

illegal:
	self.error(idx, "Illegal break statement")
	self.nextStatement()
	return &ast.BadStatement{From: idx, To: self.idx}
}

func (self *_parser) parseContinueStatement() ast.Statement {
	idx := self.expect(token.CONTINUE)
	semicolon := self.implicitSemicolon
	if self.token == token.SEMICOLON {
		semicolon = true
		self.next()
	}

	if semicolon || self.token == token.RIGHT_BRACE {
		self.implicitSemicolon = false
		if !self.scope.inIteration {
			goto illegal
		}
		return &ast.BranchStatement{
			Idx:   idx,
			Token: token.CONTINUE,
		}
	}

	if self.token == token.IDENTIFIER {
		identifier := self.parseIdentifier()
		if !self.scope.hasLabel(identifier.Name) {
			self.error(idx, "Undefined label '%s'", identifier.Name)
			return &ast.BadStatement{From: idx, To: identifier.Idx1()}
		}
		if !self.scope.inIteration {
			goto illegal
		}
		self.semicolon()
		return &ast.BranchStatement{
			Idx:   idx,
			Token: token.CONTINUE,
			Label: identifier,
		}
	}

	self.expect(token.IDENTIFIER)

illegal:
	self.error(idx, "Illegal continue statement")
	self.nextStatement()
	return &ast.BadStatement{From: idx, To: self.idx}
}

// Find the next statement after an error (recover)
func (self *_parser) nextStatement() {
	for {
		switch self.token {
		case token.BREAK, token.CONTINUE,
			token.FOR, token.IF, token.RETURN, token.SWITCH,
			token.VAR, token.DO, token.TRY, token.WITH,
			token.WHILE, token.THROW, token.CATCH, token.FINALLY:
			// Return only if parser made some progress since last
			// sync or if it has not reached 10 next calls without
			// progress. Otherwise consume at least one token to
			// avoid an endless parser loop
			if self.idx == self.recover.idx && self.recover.count < 10 {
				self.recover.count++
				return
			}
			if self.idx > self.recover.idx {
				self.recover.idx = self.idx
				self.recover.count = 0
				return
			}
			// Reaching here indicates a parser bug, likely an
			// incorrect token list in this function, but it only
			// leads to skipping of possibly correct code if a
			// previous error is present, and thus is preferred
			// over a non-terminating parse.
		case token.EOF:
			return
		}
		self.next()
	}
}
