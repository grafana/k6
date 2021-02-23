/*
Package ast declares types representing a JavaScript AST.

Warning

The parser and AST interfaces are still works-in-progress (particularly where
node types are concerned) and may change in the future.

*/
package ast

import (
	"github.com/dop251/goja/file"
	"github.com/dop251/goja/token"
	"github.com/dop251/goja/unistring"
)

// All nodes implement the Node interface.
type Node interface {
	Idx0() file.Idx // The index of the first character belonging to the node
	Idx1() file.Idx // The index of the first character immediately after the node
}

// ========== //
// Expression //
// ========== //

type (
	// All expression nodes implement the Expression interface.
	Expression interface {
		Node
		_expressionNode()
	}

	ArrayLiteral struct {
		LeftBracket  file.Idx
		RightBracket file.Idx
		Value        []Expression
	}

	AssignExpression struct {
		Operator token.Token
		Left     Expression
		Right    Expression
	}

	BadExpression struct {
		From file.Idx
		To   file.Idx
	}

	BinaryExpression struct {
		Operator   token.Token
		Left       Expression
		Right      Expression
		Comparison bool
	}

	BooleanLiteral struct {
		Idx     file.Idx
		Literal string
		Value   bool
	}

	BracketExpression struct {
		Left         Expression
		Member       Expression
		LeftBracket  file.Idx
		RightBracket file.Idx
	}

	CallExpression struct {
		Callee           Expression
		LeftParenthesis  file.Idx
		ArgumentList     []Expression
		RightParenthesis file.Idx
	}

	ConditionalExpression struct {
		Test       Expression
		Consequent Expression
		Alternate  Expression
	}

	DotExpression struct {
		Left       Expression
		Identifier Identifier
	}

	FunctionLiteral struct {
		Function      file.Idx
		Name          *Identifier
		ParameterList *ParameterList
		Body          *BlockStatement
		Source        string

		DeclarationList []*VariableDeclaration
	}

	Identifier struct {
		Name unistring.String
		Idx  file.Idx
	}

	NewExpression struct {
		New              file.Idx
		Callee           Expression
		LeftParenthesis  file.Idx
		ArgumentList     []Expression
		RightParenthesis file.Idx
	}

	NullLiteral struct {
		Idx     file.Idx
		Literal string
	}

	NumberLiteral struct {
		Idx     file.Idx
		Literal string
		Value   interface{}
	}

	ObjectLiteral struct {
		LeftBrace  file.Idx
		RightBrace file.Idx
		Value      []Property
	}

	ParameterList struct {
		Opening file.Idx
		List    []*Identifier
		Closing file.Idx
	}

	Property struct {
		Key   Expression
		Kind  string
		Value Expression
	}

	RegExpLiteral struct {
		Idx     file.Idx
		Literal string
		Pattern string
		Flags   string
	}

	SequenceExpression struct {
		Sequence []Expression
	}

	StringLiteral struct {
		Idx     file.Idx
		Literal string
		Value   unistring.String
	}

	ThisExpression struct {
		Idx file.Idx
	}

	UnaryExpression struct {
		Operator token.Token
		Idx      file.Idx // If a prefix operation
		Operand  Expression
		Postfix  bool
	}

	VariableExpression struct {
		Name        unistring.String
		Idx         file.Idx
		Initializer Expression
	}

	MetaProperty struct {
		Meta, Property *Identifier
		Idx            file.Idx
	}
)

// _expressionNode

func (*ArrayLiteral) _expressionNode()          {}
func (*AssignExpression) _expressionNode()      {}
func (*BadExpression) _expressionNode()         {}
func (*BinaryExpression) _expressionNode()      {}
func (*BooleanLiteral) _expressionNode()        {}
func (*BracketExpression) _expressionNode()     {}
func (*CallExpression) _expressionNode()        {}
func (*ConditionalExpression) _expressionNode() {}
func (*DotExpression) _expressionNode()         {}
func (*FunctionLiteral) _expressionNode()       {}
func (*Identifier) _expressionNode()            {}
func (*NewExpression) _expressionNode()         {}
func (*NullLiteral) _expressionNode()           {}
func (*NumberLiteral) _expressionNode()         {}
func (*ObjectLiteral) _expressionNode()         {}
func (*RegExpLiteral) _expressionNode()         {}
func (*SequenceExpression) _expressionNode()    {}
func (*StringLiteral) _expressionNode()         {}
func (*ThisExpression) _expressionNode()        {}
func (*UnaryExpression) _expressionNode()       {}
func (*VariableExpression) _expressionNode()    {}
func (*MetaProperty) _expressionNode()          {}

// ========= //
// Statement //
// ========= //

type (
	// All statement nodes implement the Statement interface.
	Statement interface {
		Node
		_statementNode()
	}

	BadStatement struct {
		From file.Idx
		To   file.Idx
	}

	BlockStatement struct {
		LeftBrace  file.Idx
		List       []Statement
		RightBrace file.Idx
	}

	BranchStatement struct {
		Idx   file.Idx
		Token token.Token
		Label *Identifier
	}

	CaseStatement struct {
		Case       file.Idx
		Test       Expression
		Consequent []Statement
	}

	CatchStatement struct {
		Catch     file.Idx
		Parameter *Identifier
		Body      *BlockStatement
	}

	DebuggerStatement struct {
		Debugger file.Idx
	}

	DoWhileStatement struct {
		Do   file.Idx
		Test Expression
		Body Statement
	}

	EmptyStatement struct {
		Semicolon file.Idx
	}

	ExpressionStatement struct {
		Expression Expression
	}

	ForInStatement struct {
		For    file.Idx
		Into   ForInto
		Source Expression
		Body   Statement
	}

	ForOfStatement struct {
		For    file.Idx
		Into   ForInto
		Source Expression
		Body   Statement
	}

	ForStatement struct {
		For         file.Idx
		Initializer ForLoopInitializer
		Update      Expression
		Test        Expression
		Body        Statement
	}

	IfStatement struct {
		If         file.Idx
		Test       Expression
		Consequent Statement
		Alternate  Statement
	}

	LabelledStatement struct {
		Label     *Identifier
		Colon     file.Idx
		Statement Statement
	}

	ReturnStatement struct {
		Return   file.Idx
		Argument Expression
	}

	SwitchStatement struct {
		Switch       file.Idx
		Discriminant Expression
		Default      int
		Body         []*CaseStatement
	}

	ThrowStatement struct {
		Throw    file.Idx
		Argument Expression
	}

	TryStatement struct {
		Try     file.Idx
		Body    *BlockStatement
		Catch   *CatchStatement
		Finally *BlockStatement
	}

	VariableStatement struct {
		Var  file.Idx
		List []*VariableExpression
	}

	LexicalDeclaration struct {
		Idx   file.Idx
		Token token.Token
		List  []*VariableExpression
	}

	WhileStatement struct {
		While file.Idx
		Test  Expression
		Body  Statement
	}

	WithStatement struct {
		With   file.Idx
		Object Expression
		Body   Statement
	}

	FunctionDeclaration struct {
		Function *FunctionLiteral
	}
)

// _statementNode

func (*BadStatement) _statementNode()        {}
func (*BlockStatement) _statementNode()      {}
func (*BranchStatement) _statementNode()     {}
func (*CaseStatement) _statementNode()       {}
func (*CatchStatement) _statementNode()      {}
func (*DebuggerStatement) _statementNode()   {}
func (*DoWhileStatement) _statementNode()    {}
func (*EmptyStatement) _statementNode()      {}
func (*ExpressionStatement) _statementNode() {}
func (*ForInStatement) _statementNode()      {}
func (*ForOfStatement) _statementNode()      {}
func (*ForStatement) _statementNode()        {}
func (*IfStatement) _statementNode()         {}
func (*LabelledStatement) _statementNode()   {}
func (*ReturnStatement) _statementNode()     {}
func (*SwitchStatement) _statementNode()     {}
func (*ThrowStatement) _statementNode()      {}
func (*TryStatement) _statementNode()        {}
func (*VariableStatement) _statementNode()   {}
func (*WhileStatement) _statementNode()      {}
func (*WithStatement) _statementNode()       {}
func (*LexicalDeclaration) _statementNode()  {}
func (*FunctionDeclaration) _statementNode() {}

// =========== //
// Declaration //
// =========== //

type (
	VariableDeclaration struct {
		Var  file.Idx
		List []*VariableExpression
	}
)

type (
	ForLoopInitializer interface {
		_forLoopInitializer()
	}

	ForLoopInitializerExpression struct {
		Expression Expression
	}

	ForLoopInitializerVarDeclList struct {
		Var  file.Idx
		List []*VariableExpression
	}

	ForLoopInitializerLexicalDecl struct {
		LexicalDeclaration LexicalDeclaration
	}

	ForInto interface {
		_forInto()
	}

	ForIntoVar struct {
		Binding *VariableExpression
	}

	ForBinding interface {
		_forBinding()
	}

	BindingIdentifier struct {
		Idx  file.Idx
		Name unistring.String
	}

	ForDeclaration struct {
		Idx     file.Idx
		IsConst bool
		Binding ForBinding
	}

	ForIntoExpression struct {
		Expression Expression
	}
)

func (*ForLoopInitializerExpression) _forLoopInitializer()  {}
func (*ForLoopInitializerVarDeclList) _forLoopInitializer() {}
func (*ForLoopInitializerLexicalDecl) _forLoopInitializer() {}

func (*ForIntoVar) _forInto()        {}
func (*ForDeclaration) _forInto()    {}
func (*ForIntoExpression) _forInto() {}

func (*BindingIdentifier) _forBinding() {}

// ==== //
// Node //
// ==== //

type Program struct {
	Body []Statement

	DeclarationList []*VariableDeclaration

	File *file.File
}

// ==== //
// Idx0 //
// ==== //

func (self *ArrayLiteral) Idx0() file.Idx          { return self.LeftBracket }
func (self *AssignExpression) Idx0() file.Idx      { return self.Left.Idx0() }
func (self *BadExpression) Idx0() file.Idx         { return self.From }
func (self *BinaryExpression) Idx0() file.Idx      { return self.Left.Idx0() }
func (self *BooleanLiteral) Idx0() file.Idx        { return self.Idx }
func (self *BracketExpression) Idx0() file.Idx     { return self.Left.Idx0() }
func (self *CallExpression) Idx0() file.Idx        { return self.Callee.Idx0() }
func (self *ConditionalExpression) Idx0() file.Idx { return self.Test.Idx0() }
func (self *DotExpression) Idx0() file.Idx         { return self.Left.Idx0() }
func (self *FunctionLiteral) Idx0() file.Idx       { return self.Function }
func (self *Identifier) Idx0() file.Idx            { return self.Idx }
func (self *NewExpression) Idx0() file.Idx         { return self.New }
func (self *NullLiteral) Idx0() file.Idx           { return self.Idx }
func (self *NumberLiteral) Idx0() file.Idx         { return self.Idx }
func (self *ObjectLiteral) Idx0() file.Idx         { return self.LeftBrace }
func (self *RegExpLiteral) Idx0() file.Idx         { return self.Idx }
func (self *SequenceExpression) Idx0() file.Idx    { return self.Sequence[0].Idx0() }
func (self *StringLiteral) Idx0() file.Idx         { return self.Idx }
func (self *ThisExpression) Idx0() file.Idx        { return self.Idx }
func (self *UnaryExpression) Idx0() file.Idx       { return self.Idx }
func (self *VariableExpression) Idx0() file.Idx    { return self.Idx }
func (self *MetaProperty) Idx0() file.Idx          { return self.Idx }

func (self *BadStatement) Idx0() file.Idx        { return self.From }
func (self *BlockStatement) Idx0() file.Idx      { return self.LeftBrace }
func (self *BranchStatement) Idx0() file.Idx     { return self.Idx }
func (self *CaseStatement) Idx0() file.Idx       { return self.Case }
func (self *CatchStatement) Idx0() file.Idx      { return self.Catch }
func (self *DebuggerStatement) Idx0() file.Idx   { return self.Debugger }
func (self *DoWhileStatement) Idx0() file.Idx    { return self.Do }
func (self *EmptyStatement) Idx0() file.Idx      { return self.Semicolon }
func (self *ExpressionStatement) Idx0() file.Idx { return self.Expression.Idx0() }
func (self *ForInStatement) Idx0() file.Idx      { return self.For }
func (self *ForOfStatement) Idx0() file.Idx      { return self.For }
func (self *ForStatement) Idx0() file.Idx        { return self.For }
func (self *IfStatement) Idx0() file.Idx         { return self.If }
func (self *LabelledStatement) Idx0() file.Idx   { return self.Label.Idx0() }
func (self *Program) Idx0() file.Idx             { return self.Body[0].Idx0() }
func (self *ReturnStatement) Idx0() file.Idx     { return self.Return }
func (self *SwitchStatement) Idx0() file.Idx     { return self.Switch }
func (self *ThrowStatement) Idx0() file.Idx      { return self.Throw }
func (self *TryStatement) Idx0() file.Idx        { return self.Try }
func (self *VariableStatement) Idx0() file.Idx   { return self.Var }
func (self *WhileStatement) Idx0() file.Idx      { return self.While }
func (self *WithStatement) Idx0() file.Idx       { return self.With }
func (self *LexicalDeclaration) Idx0() file.Idx  { return self.Idx }
func (self *FunctionDeclaration) Idx0() file.Idx { return self.Function.Idx0() }

func (self *ForLoopInitializerVarDeclList) Idx0() file.Idx { return self.List[0].Idx0() }

// ==== //
// Idx1 //
// ==== //

func (self *ArrayLiteral) Idx1() file.Idx          { return self.RightBracket }
func (self *AssignExpression) Idx1() file.Idx      { return self.Right.Idx1() }
func (self *BadExpression) Idx1() file.Idx         { return self.To }
func (self *BinaryExpression) Idx1() file.Idx      { return self.Right.Idx1() }
func (self *BooleanLiteral) Idx1() file.Idx        { return file.Idx(int(self.Idx) + len(self.Literal)) }
func (self *BracketExpression) Idx1() file.Idx     { return self.RightBracket + 1 }
func (self *CallExpression) Idx1() file.Idx        { return self.RightParenthesis + 1 }
func (self *ConditionalExpression) Idx1() file.Idx { return self.Test.Idx1() }
func (self *DotExpression) Idx1() file.Idx         { return self.Identifier.Idx1() }
func (self *FunctionLiteral) Idx1() file.Idx       { return self.Body.Idx1() }
func (self *Identifier) Idx1() file.Idx            { return file.Idx(int(self.Idx) + len(self.Name)) }
func (self *NewExpression) Idx1() file.Idx         { return self.RightParenthesis + 1 }
func (self *NullLiteral) Idx1() file.Idx           { return file.Idx(int(self.Idx) + 4) } // "null"
func (self *NumberLiteral) Idx1() file.Idx         { return file.Idx(int(self.Idx) + len(self.Literal)) }
func (self *ObjectLiteral) Idx1() file.Idx         { return self.RightBrace }
func (self *RegExpLiteral) Idx1() file.Idx         { return file.Idx(int(self.Idx) + len(self.Literal)) }
func (self *SequenceExpression) Idx1() file.Idx    { return self.Sequence[0].Idx1() }
func (self *StringLiteral) Idx1() file.Idx         { return file.Idx(int(self.Idx) + len(self.Literal)) }
func (self *ThisExpression) Idx1() file.Idx        { return self.Idx }
func (self *UnaryExpression) Idx1() file.Idx {
	if self.Postfix {
		return self.Operand.Idx1() + 2 // ++ --
	}
	return self.Operand.Idx1()
}
func (self *VariableExpression) Idx1() file.Idx {
	if self.Initializer == nil {
		return file.Idx(int(self.Idx) + len(self.Name) + 1)
	}
	return self.Initializer.Idx1()
}
func (self *MetaProperty) Idx1() file.Idx {
	return self.Property.Idx1()
}

func (self *BadStatement) Idx1() file.Idx        { return self.To }
func (self *BlockStatement) Idx1() file.Idx      { return self.RightBrace + 1 }
func (self *BranchStatement) Idx1() file.Idx     { return self.Idx }
func (self *CaseStatement) Idx1() file.Idx       { return self.Consequent[len(self.Consequent)-1].Idx1() }
func (self *CatchStatement) Idx1() file.Idx      { return self.Body.Idx1() }
func (self *DebuggerStatement) Idx1() file.Idx   { return self.Debugger + 8 }
func (self *DoWhileStatement) Idx1() file.Idx    { return self.Test.Idx1() }
func (self *EmptyStatement) Idx1() file.Idx      { return self.Semicolon + 1 }
func (self *ExpressionStatement) Idx1() file.Idx { return self.Expression.Idx1() }
func (self *ForInStatement) Idx1() file.Idx      { return self.Body.Idx1() }
func (self *ForOfStatement) Idx1() file.Idx      { return self.Body.Idx1() }
func (self *ForStatement) Idx1() file.Idx        { return self.Body.Idx1() }
func (self *IfStatement) Idx1() file.Idx {
	if self.Alternate != nil {
		return self.Alternate.Idx1()
	}
	return self.Consequent.Idx1()
}
func (self *LabelledStatement) Idx1() file.Idx   { return self.Colon + 1 }
func (self *Program) Idx1() file.Idx             { return self.Body[len(self.Body)-1].Idx1() }
func (self *ReturnStatement) Idx1() file.Idx     { return self.Return }
func (self *SwitchStatement) Idx1() file.Idx     { return self.Body[len(self.Body)-1].Idx1() }
func (self *ThrowStatement) Idx1() file.Idx      { return self.Throw }
func (self *TryStatement) Idx1() file.Idx        { return self.Try }
func (self *VariableStatement) Idx1() file.Idx   { return self.List[len(self.List)-1].Idx1() }
func (self *WhileStatement) Idx1() file.Idx      { return self.Body.Idx1() }
func (self *WithStatement) Idx1() file.Idx       { return self.Body.Idx1() }
func (self *LexicalDeclaration) Idx1() file.Idx  { return self.List[len(self.List)-1].Idx1() }
func (self *FunctionDeclaration) Idx1() file.Idx { return self.Function.Idx1() }

func (self *ForLoopInitializerVarDeclList) Idx1() file.Idx { return self.List[len(self.List)-1].Idx1() }
