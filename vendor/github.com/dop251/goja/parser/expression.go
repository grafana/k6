package parser

import (
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
	"github.com/dop251/goja/token"
	"github.com/dop251/goja/unistring"
)

func (self *_parser) parseIdentifier() *ast.Identifier {
	literal := self.parsedLiteral
	idx := self.idx
	self.next()
	return &ast.Identifier{
		Name: literal,
		Idx:  idx,
	}
}

func (self *_parser) parsePrimaryExpression() ast.Expression {
	literal, parsedLiteral := self.literal, self.parsedLiteral
	idx := self.idx
	switch self.token {
	case token.IDENTIFIER:
		self.next()
		if len(literal) > 1 {
			tkn, strict := token.IsKeyword(literal)
			if tkn == token.KEYWORD {
				if !strict {
					self.error(idx, "Unexpected reserved word")
				}
			}
		}
		return &ast.Identifier{
			Name: parsedLiteral,
			Idx:  idx,
		}
	case token.NULL:
		self.next()
		return &ast.NullLiteral{
			Idx:     idx,
			Literal: literal,
		}
	case token.BOOLEAN:
		self.next()
		value := false
		switch parsedLiteral {
		case "true":
			value = true
		case "false":
			value = false
		default:
			self.error(idx, "Illegal boolean literal")
		}
		return &ast.BooleanLiteral{
			Idx:     idx,
			Literal: literal,
			Value:   value,
		}
	case token.STRING:
		self.next()
		return &ast.StringLiteral{
			Idx:     idx,
			Literal: literal,
			Value:   parsedLiteral,
		}
	case token.NUMBER:
		self.next()
		value, err := parseNumberLiteral(literal)
		if err != nil {
			self.error(idx, err.Error())
			value = 0
		}
		return &ast.NumberLiteral{
			Idx:     idx,
			Literal: literal,
			Value:   value,
		}
	case token.SLASH, token.QUOTIENT_ASSIGN:
		return self.parseRegExpLiteral()
	case token.LEFT_BRACE:
		return self.parseObjectLiteral()
	case token.LEFT_BRACKET:
		return self.parseArrayLiteral()
	case token.LEFT_PARENTHESIS:
		self.expect(token.LEFT_PARENTHESIS)
		expression := self.parseExpression()
		self.expect(token.RIGHT_PARENTHESIS)
		return expression
	case token.THIS:
		self.next()
		return &ast.ThisExpression{
			Idx: idx,
		}
	case token.FUNCTION:
		return self.parseFunction(false)
	}

	self.errorUnexpectedToken(self.token)
	self.nextStatement()
	return &ast.BadExpression{From: idx, To: self.idx}
}

func (self *_parser) parseRegExpLiteral() *ast.RegExpLiteral {

	offset := self.chrOffset - 1 // Opening slash already gotten
	if self.token == token.QUOTIENT_ASSIGN {
		offset -= 1 // =
	}
	idx := self.idxOf(offset)

	pattern, _, err := self.scanString(offset, false)
	endOffset := self.chrOffset

	if err == nil {
		pattern = pattern[1 : len(pattern)-1]
	}

	flags := ""
	if !isLineTerminator(self.chr) && !isLineWhiteSpace(self.chr) {
		self.next()

		if self.token == token.IDENTIFIER { // gim

			flags = self.literal
			self.next()
			endOffset = self.chrOffset - 1
		}
	} else {
		self.next()
	}

	literal := self.str[offset:endOffset]

	return &ast.RegExpLiteral{
		Idx:     idx,
		Literal: literal,
		Pattern: pattern,
		Flags:   flags,
	}
}

func (self *_parser) parseVariableDeclaration(declarationList *[]*ast.VariableExpression) ast.Expression {

	if self.token != token.IDENTIFIER {
		idx := self.expect(token.IDENTIFIER)
		self.nextStatement()
		return &ast.BadExpression{From: idx, To: self.idx}
	}

	name := self.parsedLiteral
	idx := self.idx
	self.next()
	node := &ast.VariableExpression{
		Name: name,
		Idx:  idx,
	}

	if declarationList != nil {
		*declarationList = append(*declarationList, node)
	}

	if self.token == token.ASSIGN {
		self.next()
		node.Initializer = self.parseAssignmentExpression()
	}

	return node
}

func (self *_parser) parseVariableDeclarationList(var_ file.Idx) []ast.Expression {

	var declarationList []*ast.VariableExpression // Avoid bad expressions
	var list []ast.Expression

	for {
		list = append(list, self.parseVariableDeclaration(&declarationList))
		if self.token != token.COMMA {
			break
		}
		self.next()
	}

	self.scope.declare(&ast.VariableDeclaration{
		Var:  var_,
		List: declarationList,
	})

	return list
}

func (self *_parser) parseObjectPropertyKey() (unistring.String, ast.Expression, token.Token) {
	idx, tkn, literal, parsedLiteral := self.idx, self.token, self.literal, self.parsedLiteral
	var value ast.Expression
	self.next()
	switch tkn {
	case token.IDENTIFIER:
		value = &ast.StringLiteral{
			Idx:     idx,
			Literal: literal,
			Value:   unistring.String(literal),
		}
	case token.NUMBER:
		num, err := parseNumberLiteral(literal)
		if err != nil {
			self.error(idx, err.Error())
		} else {
			value = &ast.NumberLiteral{
				Idx:     idx,
				Literal: literal,
				Value:   num,
			}
		}
	case token.STRING:
		value = &ast.StringLiteral{
			Idx:     idx,
			Literal: literal,
			Value:   parsedLiteral,
		}
	default:
		// null, false, class, etc.
		if isId(tkn) {
			value = &ast.StringLiteral{
				Idx:     idx,
				Literal: literal,
				Value:   unistring.String(literal),
			}
			tkn = token.STRING
		}
	}
	return parsedLiteral, value, tkn
}

func (self *_parser) parseObjectProperty() ast.Property {
	literal, value, tkn := self.parseObjectPropertyKey()
	if tkn == token.IDENTIFIER || tkn == token.STRING {
		switch {
		case self.token == token.LEFT_PARENTHESIS:
			idx := self.idx
			parameterList := self.parseFunctionParameterList()

			node := &ast.FunctionLiteral{
				Function:      idx,
				ParameterList: parameterList,
			}
			self.parseFunctionBlock(node)

			return ast.Property{
				Key:   value,
				Kind:  "method",
				Value: node,
			}
		case self.token == token.COMMA || self.token == token.RIGHT_BRACE: // shorthand property
			return ast.Property{
				Key:  value,
				Kind: "value",
				Value: &ast.Identifier{
					Name: literal,
					Idx:  self.idx,
				},
			}
		case literal == "get" && self.token != token.COLON:
			idx := self.idx
			_, value, _ := self.parseObjectPropertyKey()
			parameterList := self.parseFunctionParameterList()

			node := &ast.FunctionLiteral{
				Function:      idx,
				ParameterList: parameterList,
			}
			self.parseFunctionBlock(node)
			return ast.Property{
				Key:   value,
				Kind:  "get",
				Value: node,
			}
		case literal == "set" && self.token != token.COLON:
			idx := self.idx
			_, value, _ := self.parseObjectPropertyKey()
			parameterList := self.parseFunctionParameterList()

			node := &ast.FunctionLiteral{
				Function:      idx,
				ParameterList: parameterList,
			}

			self.parseFunctionBlock(node)

			return ast.Property{
				Key:   value,
				Kind:  "set",
				Value: node,
			}
		}
	}

	self.expect(token.COLON)

	return ast.Property{
		Key:   value,
		Kind:  "value",
		Value: self.parseAssignmentExpression(),
	}
}

func (self *_parser) parseObjectLiteral() ast.Expression {
	var value []ast.Property
	idx0 := self.expect(token.LEFT_BRACE)
	for self.token != token.RIGHT_BRACE && self.token != token.EOF {
		property := self.parseObjectProperty()
		value = append(value, property)
		if self.token != token.RIGHT_BRACE {
			self.expect(token.COMMA)
		} else {
			break
		}
	}
	idx1 := self.expect(token.RIGHT_BRACE)

	return &ast.ObjectLiteral{
		LeftBrace:  idx0,
		RightBrace: idx1,
		Value:      value,
	}
}

func (self *_parser) parseArrayLiteral() ast.Expression {

	idx0 := self.expect(token.LEFT_BRACKET)
	var value []ast.Expression
	for self.token != token.RIGHT_BRACKET && self.token != token.EOF {
		if self.token == token.COMMA {
			self.next()
			value = append(value, nil)
			continue
		}
		value = append(value, self.parseAssignmentExpression())
		if self.token != token.RIGHT_BRACKET {
			self.expect(token.COMMA)
		}
	}
	idx1 := self.expect(token.RIGHT_BRACKET)

	return &ast.ArrayLiteral{
		LeftBracket:  idx0,
		RightBracket: idx1,
		Value:        value,
	}
}

func (self *_parser) parseArgumentList() (argumentList []ast.Expression, idx0, idx1 file.Idx) {
	idx0 = self.expect(token.LEFT_PARENTHESIS)
	if self.token != token.RIGHT_PARENTHESIS {
		for {
			argumentList = append(argumentList, self.parseAssignmentExpression())
			if self.token != token.COMMA {
				break
			}
			self.next()
		}
	}
	idx1 = self.expect(token.RIGHT_PARENTHESIS)
	return
}

func (self *_parser) parseCallExpression(left ast.Expression) ast.Expression {
	argumentList, idx0, idx1 := self.parseArgumentList()
	return &ast.CallExpression{
		Callee:           left,
		LeftParenthesis:  idx0,
		ArgumentList:     argumentList,
		RightParenthesis: idx1,
	}
}

func (self *_parser) parseDotMember(left ast.Expression) ast.Expression {
	period := self.expect(token.PERIOD)

	literal := self.parsedLiteral
	idx := self.idx

	if self.token != token.IDENTIFIER && !isId(self.token) {
		self.expect(token.IDENTIFIER)
		self.nextStatement()
		return &ast.BadExpression{From: period, To: self.idx}
	}

	self.next()

	return &ast.DotExpression{
		Left: left,
		Identifier: ast.Identifier{
			Idx:  idx,
			Name: literal,
		},
	}
}

func (self *_parser) parseBracketMember(left ast.Expression) ast.Expression {
	idx0 := self.expect(token.LEFT_BRACKET)
	member := self.parseExpression()
	idx1 := self.expect(token.RIGHT_BRACKET)
	return &ast.BracketExpression{
		LeftBracket:  idx0,
		Left:         left,
		Member:       member,
		RightBracket: idx1,
	}
}

func (self *_parser) parseNewExpression() ast.Expression {
	idx := self.expect(token.NEW)
	if self.token == token.PERIOD {
		self.next()
		prop := self.parseIdentifier()
		if prop.Name == "target" {
			if !self.scope.inFunction {
				self.error(idx, "new.target expression is not allowed here")
			}
			return &ast.MetaProperty{
				Meta: &ast.Identifier{
					Name: unistring.String(token.NEW.String()),
					Idx:  idx,
				},
				Property: prop,
			}
		}
		self.errorUnexpectedToken(token.IDENTIFIER)
	}
	callee := self.parseLeftHandSideExpression()
	node := &ast.NewExpression{
		New:    idx,
		Callee: callee,
	}
	if self.token == token.LEFT_PARENTHESIS {
		argumentList, idx0, idx1 := self.parseArgumentList()
		node.ArgumentList = argumentList
		node.LeftParenthesis = idx0
		node.RightParenthesis = idx1
	}
	return node
}

func (self *_parser) parseLeftHandSideExpression() ast.Expression {

	var left ast.Expression
	if self.token == token.NEW {
		left = self.parseNewExpression()
	} else {
		left = self.parsePrimaryExpression()
	}

	for {
		if self.token == token.PERIOD {
			left = self.parseDotMember(left)
		} else if self.token == token.LEFT_BRACKET {
			left = self.parseBracketMember(left)
		} else {
			break
		}
	}

	return left
}

func (self *_parser) parseLeftHandSideExpressionAllowCall() ast.Expression {

	allowIn := self.scope.allowIn
	self.scope.allowIn = true
	defer func() {
		self.scope.allowIn = allowIn
	}()

	var left ast.Expression
	if self.token == token.NEW {
		left = self.parseNewExpression()
	} else {
		left = self.parsePrimaryExpression()
	}

	for {
		if self.token == token.PERIOD {
			left = self.parseDotMember(left)
		} else if self.token == token.LEFT_BRACKET {
			left = self.parseBracketMember(left)
		} else if self.token == token.LEFT_PARENTHESIS {
			left = self.parseCallExpression(left)
		} else {
			break
		}
	}

	return left
}

func (self *_parser) parsePostfixExpression() ast.Expression {
	operand := self.parseLeftHandSideExpressionAllowCall()

	switch self.token {
	case token.INCREMENT, token.DECREMENT:
		// Make sure there is no line terminator here
		if self.implicitSemicolon {
			break
		}
		tkn := self.token
		idx := self.idx
		self.next()
		switch operand.(type) {
		case *ast.Identifier, *ast.DotExpression, *ast.BracketExpression:
		default:
			self.error(idx, "Invalid left-hand side in assignment")
			self.nextStatement()
			return &ast.BadExpression{From: idx, To: self.idx}
		}
		return &ast.UnaryExpression{
			Operator: tkn,
			Idx:      idx,
			Operand:  operand,
			Postfix:  true,
		}
	}

	return operand
}

func (self *_parser) parseUnaryExpression() ast.Expression {

	switch self.token {
	case token.PLUS, token.MINUS, token.NOT, token.BITWISE_NOT:
		fallthrough
	case token.DELETE, token.VOID, token.TYPEOF:
		tkn := self.token
		idx := self.idx
		self.next()
		return &ast.UnaryExpression{
			Operator: tkn,
			Idx:      idx,
			Operand:  self.parseUnaryExpression(),
		}
	case token.INCREMENT, token.DECREMENT:
		tkn := self.token
		idx := self.idx
		self.next()
		operand := self.parseUnaryExpression()
		switch operand.(type) {
		case *ast.Identifier, *ast.DotExpression, *ast.BracketExpression:
		default:
			self.error(idx, "Invalid left-hand side in assignment")
			self.nextStatement()
			return &ast.BadExpression{From: idx, To: self.idx}
		}
		return &ast.UnaryExpression{
			Operator: tkn,
			Idx:      idx,
			Operand:  operand,
		}
	}

	return self.parsePostfixExpression()
}

func (self *_parser) parseMultiplicativeExpression() ast.Expression {
	next := self.parseUnaryExpression
	left := next()

	for self.token == token.MULTIPLY || self.token == token.SLASH ||
		self.token == token.REMAINDER {
		tkn := self.token
		self.next()
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseAdditiveExpression() ast.Expression {
	next := self.parseMultiplicativeExpression
	left := next()

	for self.token == token.PLUS || self.token == token.MINUS {
		tkn := self.token
		self.next()
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseShiftExpression() ast.Expression {
	next := self.parseAdditiveExpression
	left := next()

	for self.token == token.SHIFT_LEFT || self.token == token.SHIFT_RIGHT ||
		self.token == token.UNSIGNED_SHIFT_RIGHT {
		tkn := self.token
		self.next()
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseRelationalExpression() ast.Expression {
	next := self.parseShiftExpression
	left := next()

	allowIn := self.scope.allowIn
	self.scope.allowIn = true
	defer func() {
		self.scope.allowIn = allowIn
	}()

	switch self.token {
	case token.LESS, token.LESS_OR_EQUAL, token.GREATER, token.GREATER_OR_EQUAL:
		tkn := self.token
		self.next()
		return &ast.BinaryExpression{
			Operator:   tkn,
			Left:       left,
			Right:      self.parseRelationalExpression(),
			Comparison: true,
		}
	case token.INSTANCEOF:
		tkn := self.token
		self.next()
		return &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    self.parseRelationalExpression(),
		}
	case token.IN:
		if !allowIn {
			return left
		}
		tkn := self.token
		self.next()
		return &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    self.parseRelationalExpression(),
		}
	}

	return left
}

func (self *_parser) parseEqualityExpression() ast.Expression {
	next := self.parseRelationalExpression
	left := next()

	for self.token == token.EQUAL || self.token == token.NOT_EQUAL ||
		self.token == token.STRICT_EQUAL || self.token == token.STRICT_NOT_EQUAL {
		tkn := self.token
		self.next()
		left = &ast.BinaryExpression{
			Operator:   tkn,
			Left:       left,
			Right:      next(),
			Comparison: true,
		}
	}

	return left
}

func (self *_parser) parseBitwiseAndExpression() ast.Expression {
	next := self.parseEqualityExpression
	left := next()

	for self.token == token.AND {
		tkn := self.token
		self.next()
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseBitwiseExclusiveOrExpression() ast.Expression {
	next := self.parseBitwiseAndExpression
	left := next()

	for self.token == token.EXCLUSIVE_OR {
		tkn := self.token
		self.next()
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseBitwiseOrExpression() ast.Expression {
	next := self.parseBitwiseExclusiveOrExpression
	left := next()

	for self.token == token.OR {
		tkn := self.token
		self.next()
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseLogicalAndExpression() ast.Expression {
	next := self.parseBitwiseOrExpression
	left := next()

	for self.token == token.LOGICAL_AND {
		tkn := self.token
		self.next()
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseLogicalOrExpression() ast.Expression {
	next := self.parseLogicalAndExpression
	left := next()

	for self.token == token.LOGICAL_OR {
		tkn := self.token
		self.next()
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseConditionlExpression() ast.Expression {
	left := self.parseLogicalOrExpression()

	if self.token == token.QUESTION_MARK {
		self.next()
		consequent := self.parseAssignmentExpression()
		self.expect(token.COLON)
		return &ast.ConditionalExpression{
			Test:       left,
			Consequent: consequent,
			Alternate:  self.parseAssignmentExpression(),
		}
	}

	return left
}

func (self *_parser) parseAssignmentExpression() ast.Expression {
	left := self.parseConditionlExpression()
	var operator token.Token
	switch self.token {
	case token.ASSIGN:
		operator = self.token
	case token.ADD_ASSIGN:
		operator = token.PLUS
	case token.SUBTRACT_ASSIGN:
		operator = token.MINUS
	case token.MULTIPLY_ASSIGN:
		operator = token.MULTIPLY
	case token.QUOTIENT_ASSIGN:
		operator = token.SLASH
	case token.REMAINDER_ASSIGN:
		operator = token.REMAINDER
	case token.AND_ASSIGN:
		operator = token.AND
	case token.OR_ASSIGN:
		operator = token.OR
	case token.EXCLUSIVE_OR_ASSIGN:
		operator = token.EXCLUSIVE_OR
	case token.SHIFT_LEFT_ASSIGN:
		operator = token.SHIFT_LEFT
	case token.SHIFT_RIGHT_ASSIGN:
		operator = token.SHIFT_RIGHT
	case token.UNSIGNED_SHIFT_RIGHT_ASSIGN:
		operator = token.UNSIGNED_SHIFT_RIGHT
	}

	if operator != 0 {
		idx := self.idx
		self.next()
		switch left.(type) {
		case *ast.Identifier, *ast.DotExpression, *ast.BracketExpression:
		default:
			self.error(left.Idx0(), "Invalid left-hand side in assignment")
			self.nextStatement()
			return &ast.BadExpression{From: idx, To: self.idx}
		}
		return &ast.AssignExpression{
			Left:     left,
			Operator: operator,
			Right:    self.parseAssignmentExpression(),
		}
	}

	return left
}

func (self *_parser) parseExpression() ast.Expression {
	next := self.parseAssignmentExpression
	left := next()

	if self.token == token.COMMA {
		sequence := []ast.Expression{left}
		for {
			if self.token != token.COMMA {
				break
			}
			self.next()
			sequence = append(sequence, next())
		}
		return &ast.SequenceExpression{
			Sequence: sequence,
		}
	}

	return left
}
