/*
Package ast declares types representing a JavaScript AST.

# Warning

The parser and AST interfaces are still works-in-progress (particularly where
node types are concerned) and may change in the future.
*/
package ast

import (
	"github.com/dop251/goja/file"
	"github.com/dop251/goja/token"
	"github.com/dop251/goja/unistring"
)

type PropertyKind string

const (
	PropertyKindValue  PropertyKind = "value"
	PropertyKindGet    PropertyKind = "get"
	PropertyKindSet    PropertyKind = "set"
	PropertyKindMethod PropertyKind = "method"
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

	BindingTarget interface {
		Expression
		_bindingTarget()
	}

	Binding struct {
		Target      BindingTarget
		Initializer Expression
	}

	Pattern interface {
		BindingTarget
		_pattern()
	}

	YieldExpression struct {
		Yield    file.Idx
		Argument Expression
		Delegate bool
	}

	AwaitExpression struct {
		Await    file.Idx
		Argument Expression
	}

	ArrayLiteral struct {
		LeftBracket  file.Idx
		RightBracket file.Idx
		Value        []Expression
	}

	ArrayPattern struct {
		LeftBracket  file.Idx
		RightBracket file.Idx
		Elements     []Expression
		Rest         Expression
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

	PrivateDotExpression struct {
		Left       Expression
		Identifier PrivateIdentifier
	}

	OptionalChain struct {
		Expression
	}

	Optional struct {
		Expression
	}

	FunctionLiteral struct {
		Function      file.Idx
		Name          *Identifier
		ParameterList *ParameterList
		Body          *BlockStatement
		Source        string

		DeclarationList []*VariableDeclaration

		Async, Generator bool
	}

	ClassLiteral struct {
		Class      file.Idx
		RightBrace file.Idx
		Name       *Identifier
		SuperClass Expression
		Body       []ClassElement
		Source     string
	}

	ConciseBody interface {
		Node
		_conciseBody()
	}

	ExpressionBody struct {
		Expression Expression
	}

	ArrowFunctionLiteral struct {
		Start           file.Idx
		ParameterList   *ParameterList
		Body            ConciseBody
		Source          string
		DeclarationList []*VariableDeclaration
		Async           bool
	}

	Identifier struct {
		Name unistring.String
		Idx  file.Idx
	}

	PrivateIdentifier struct {
		Identifier
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

	ObjectPattern struct {
		LeftBrace  file.Idx
		RightBrace file.Idx
		Properties []Property
		Rest       Expression
	}

	ParameterList struct {
		Opening file.Idx
		List    []*Binding
		Rest    Expression
		Closing file.Idx
	}

	Property interface {
		Expression
		_property()
	}

	PropertyShort struct {
		Name        Identifier
		Initializer Expression
	}

	PropertyKeyed struct {
		Key      Expression
		Kind     PropertyKind
		Value    Expression
		Computed bool
	}

	SpreadElement struct {
		Expression
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

	TemplateElement struct {
		Idx     file.Idx
		Literal string
		Parsed  unistring.String
		Valid   bool
	}

	TemplateLiteral struct {
		OpenQuote   file.Idx
		CloseQuote  file.Idx
		Tag         Expression
		Elements    []*TemplateElement
		Expressions []Expression
	}

	ThisExpression struct {
		Idx file.Idx
	}

	SuperExpression struct {
		Idx file.Idx
	}

	UnaryExpression struct {
		Operator token.Token
		Idx      file.Idx // If a prefix operation
		Operand  Expression
		Postfix  bool
	}

	MetaProperty struct {
		Meta, Property *Identifier
		Idx            file.Idx
	}
)

// _expressionNode

func (*ArrayLiteral) _expressionNode()          {}
func (*AssignExpression) _expressionNode()      {}
func (*YieldExpression) _expressionNode()       {}
func (*AwaitExpression) _expressionNode()       {}
func (*BadExpression) _expressionNode()         {}
func (*BinaryExpression) _expressionNode()      {}
func (*BooleanLiteral) _expressionNode()        {}
func (*BracketExpression) _expressionNode()     {}
func (*CallExpression) _expressionNode()        {}
func (*ConditionalExpression) _expressionNode() {}
func (*DotExpression) _expressionNode()         {}
func (*PrivateDotExpression) _expressionNode()  {}
func (*FunctionLiteral) _expressionNode()       {}
func (*ClassLiteral) _expressionNode()          {}
func (*ArrowFunctionLiteral) _expressionNode()  {}
func (*Identifier) _expressionNode()            {}
func (*NewExpression) _expressionNode()         {}
func (*NullLiteral) _expressionNode()           {}
func (*NumberLiteral) _expressionNode()         {}
func (*ObjectLiteral) _expressionNode()         {}
func (*RegExpLiteral) _expressionNode()         {}
func (*SequenceExpression) _expressionNode()    {}
func (*StringLiteral) _expressionNode()         {}
func (*TemplateLiteral) _expressionNode()       {}
func (*ThisExpression) _expressionNode()        {}
func (*SuperExpression) _expressionNode()       {}
func (*UnaryExpression) _expressionNode()       {}
func (*MetaProperty) _expressionNode()          {}
func (*ObjectPattern) _expressionNode()         {}
func (*ArrayPattern) _expressionNode()          {}
func (*Binding) _expressionNode()               {}

func (*PropertyShort) _expressionNode() {}
func (*PropertyKeyed) _expressionNode() {}

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
		Parameter BindingTarget
		Body      *BlockStatement
	}

	DebuggerStatement struct {
		Debugger file.Idx
	}

	DoWhileStatement struct {
		Do               file.Idx
		Test             Expression
		Body             Statement
		RightParenthesis file.Idx
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
		RightBrace   file.Idx
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
		List []*Binding
	}

	LexicalDeclaration struct {
		Idx   file.Idx
		Token token.Token
		List  []*Binding
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

	ClassDeclaration struct {
		Class *ClassLiteral
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
func (*ClassDeclaration) _statementNode()    {}

// =========== //
// Declaration //
// =========== //

type (
	VariableDeclaration struct {
		Var  file.Idx
		List []*Binding
	}

	ClassElement interface {
		Node
		_classElement()
	}

	FieldDefinition struct {
		Idx         file.Idx
		Key         Expression
		Initializer Expression
		Computed    bool
		Static      bool
	}

	MethodDefinition struct {
		Idx      file.Idx
		Key      Expression
		Kind     PropertyKind // "method", "get" or "set"
		Body     *FunctionLiteral
		Computed bool
		Static   bool
	}

	ClassStaticBlock struct {
		Static          file.Idx
		Block           *BlockStatement
		Source          string
		DeclarationList []*VariableDeclaration
	}
)

type (
	ForLoopInitializer interface {
		Node
		_forLoopInitializer()
	}

	ForLoopInitializerExpression struct {
		Expression Expression
	}

	ForLoopInitializerVarDeclList struct {
		Var  file.Idx
		List []*Binding
	}

	ForLoopInitializerLexicalDecl struct {
		LexicalDeclaration LexicalDeclaration
	}

	ForInto interface {
		Node
		_forInto()
	}

	ForIntoVar struct {
		Binding *Binding
	}

	ForDeclaration struct {
		Idx     file.Idx
		IsConst bool
		Target  BindingTarget
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

func (*ArrayPattern) _pattern()       {}
func (*ArrayPattern) _bindingTarget() {}

func (*ObjectPattern) _pattern()       {}
func (*ObjectPattern) _bindingTarget() {}

func (*BadExpression) _bindingTarget() {}

func (*PropertyShort) _property() {}
func (*PropertyKeyed) _property() {}
func (*SpreadElement) _property() {}

func (*Identifier) _bindingTarget() {}

func (*BlockStatement) _conciseBody() {}
func (*ExpressionBody) _conciseBody() {}

func (*FieldDefinition) _classElement()  {}
func (*MethodDefinition) _classElement() {}
func (*ClassStaticBlock) _classElement() {}

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
func (self *ArrayPattern) Idx0() file.Idx          { return self.LeftBracket }
func (self *YieldExpression) Idx0() file.Idx       { return self.Yield }
func (self *AwaitExpression) Idx0() file.Idx       { return self.Await }
func (self *ObjectPattern) Idx0() file.Idx         { return self.LeftBrace }
func (self *ParameterList) Idx0() file.Idx         { return self.Opening }
func (self *AssignExpression) Idx0() file.Idx      { return self.Left.Idx0() }
func (self *BadExpression) Idx0() file.Idx         { return self.From }
func (self *BinaryExpression) Idx0() file.Idx      { return self.Left.Idx0() }
func (self *BooleanLiteral) Idx0() file.Idx        { return self.Idx }
func (self *BracketExpression) Idx0() file.Idx     { return self.Left.Idx0() }
func (self *CallExpression) Idx0() file.Idx        { return self.Callee.Idx0() }
func (self *ConditionalExpression) Idx0() file.Idx { return self.Test.Idx0() }
func (self *DotExpression) Idx0() file.Idx         { return self.Left.Idx0() }
func (self *PrivateDotExpression) Idx0() file.Idx  { return self.Left.Idx0() }
func (self *FunctionLiteral) Idx0() file.Idx       { return self.Function }
func (self *ClassLiteral) Idx0() file.Idx          { return self.Class }
func (self *ArrowFunctionLiteral) Idx0() file.Idx  { return self.Start }
func (self *Identifier) Idx0() file.Idx            { return self.Idx }
func (self *NewExpression) Idx0() file.Idx         { return self.New }
func (self *NullLiteral) Idx0() file.Idx           { return self.Idx }
func (self *NumberLiteral) Idx0() file.Idx         { return self.Idx }
func (self *ObjectLiteral) Idx0() file.Idx         { return self.LeftBrace }
func (self *RegExpLiteral) Idx0() file.Idx         { return self.Idx }
func (self *SequenceExpression) Idx0() file.Idx    { return self.Sequence[0].Idx0() }
func (self *StringLiteral) Idx0() file.Idx         { return self.Idx }
func (self *TemplateElement) Idx0() file.Idx       { return self.Idx }
func (self *TemplateLiteral) Idx0() file.Idx       { return self.OpenQuote }
func (self *ThisExpression) Idx0() file.Idx        { return self.Idx }
func (self *SuperExpression) Idx0() file.Idx       { return self.Idx }
func (self *UnaryExpression) Idx0() file.Idx {
	if self.Postfix {
		return self.Operand.Idx0()
	}
	return self.Idx
}
func (self *MetaProperty) Idx0() file.Idx { return self.Idx }

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
func (self *ClassDeclaration) Idx0() file.Idx    { return self.Class.Idx0() }
func (self *Binding) Idx0() file.Idx             { return self.Target.Idx0() }

func (self *ForLoopInitializerExpression) Idx0() file.Idx  { return self.Expression.Idx0() }
func (self *ForLoopInitializerVarDeclList) Idx0() file.Idx { return self.List[0].Idx0() }
func (self *ForLoopInitializerLexicalDecl) Idx0() file.Idx { return self.LexicalDeclaration.Idx0() }
func (self *PropertyShort) Idx0() file.Idx                 { return self.Name.Idx }
func (self *PropertyKeyed) Idx0() file.Idx                 { return self.Key.Idx0() }
func (self *ExpressionBody) Idx0() file.Idx                { return self.Expression.Idx0() }

func (self *VariableDeclaration) Idx0() file.Idx { return self.Var }
func (self *FieldDefinition) Idx0() file.Idx     { return self.Idx }
func (self *MethodDefinition) Idx0() file.Idx    { return self.Idx }
func (self *ClassStaticBlock) Idx0() file.Idx    { return self.Static }

func (self *ForDeclaration) Idx0() file.Idx    { return self.Idx }
func (self *ForIntoVar) Idx0() file.Idx        { return self.Binding.Idx0() }
func (self *ForIntoExpression) Idx0() file.Idx { return self.Expression.Idx0() }

// ==== //
// Idx1 //
// ==== //

func (self *ArrayLiteral) Idx1() file.Idx          { return self.RightBracket + 1 }
func (self *ArrayPattern) Idx1() file.Idx          { return self.RightBracket + 1 }
func (self *AssignExpression) Idx1() file.Idx      { return self.Right.Idx1() }
func (self *AwaitExpression) Idx1() file.Idx       { return self.Argument.Idx1() }
func (self *BadExpression) Idx1() file.Idx         { return self.To }
func (self *BinaryExpression) Idx1() file.Idx      { return self.Right.Idx1() }
func (self *BooleanLiteral) Idx1() file.Idx        { return file.Idx(int(self.Idx) + len(self.Literal)) }
func (self *BracketExpression) Idx1() file.Idx     { return self.RightBracket + 1 }
func (self *CallExpression) Idx1() file.Idx        { return self.RightParenthesis + 1 }
func (self *ConditionalExpression) Idx1() file.Idx { return self.Alternate.Idx1() }
func (self *DotExpression) Idx1() file.Idx         { return self.Identifier.Idx1() }
func (self *PrivateDotExpression) Idx1() file.Idx  { return self.Identifier.Idx1() }
func (self *FunctionLiteral) Idx1() file.Idx       { return self.Body.Idx1() }
func (self *ClassLiteral) Idx1() file.Idx          { return self.RightBrace + 1 }
func (self *ArrowFunctionLiteral) Idx1() file.Idx  { return self.Body.Idx1() }
func (self *Identifier) Idx1() file.Idx            { return file.Idx(int(self.Idx) + len(self.Name)) }
func (self *NewExpression) Idx1() file.Idx {
	if self.ArgumentList != nil {
		return self.RightParenthesis + 1
	} else {
		return self.Callee.Idx1()
	}
}
func (self *NullLiteral) Idx1() file.Idx        { return file.Idx(int(self.Idx) + 4) } // "null"
func (self *NumberLiteral) Idx1() file.Idx      { return file.Idx(int(self.Idx) + len(self.Literal)) }
func (self *ObjectLiteral) Idx1() file.Idx      { return self.RightBrace + 1 }
func (self *ObjectPattern) Idx1() file.Idx      { return self.RightBrace + 1 }
func (self *ParameterList) Idx1() file.Idx      { return self.Closing + 1 }
func (self *RegExpLiteral) Idx1() file.Idx      { return file.Idx(int(self.Idx) + len(self.Literal)) }
func (self *SequenceExpression) Idx1() file.Idx { return self.Sequence[len(self.Sequence)-1].Idx1() }
func (self *StringLiteral) Idx1() file.Idx      { return file.Idx(int(self.Idx) + len(self.Literal)) }
func (self *TemplateElement) Idx1() file.Idx    { return file.Idx(int(self.Idx) + len(self.Literal)) }
func (self *TemplateLiteral) Idx1() file.Idx    { return self.CloseQuote + 1 }
func (self *ThisExpression) Idx1() file.Idx     { return self.Idx + 4 }
func (self *SuperExpression) Idx1() file.Idx    { return self.Idx + 5 }
func (self *UnaryExpression) Idx1() file.Idx {
	if self.Postfix {
		return self.Operand.Idx1() + 2 // ++ --
	}
	return self.Operand.Idx1()
}
func (self *MetaProperty) Idx1() file.Idx {
	return self.Property.Idx1()
}

func (self *BadStatement) Idx1() file.Idx   { return self.To }
func (self *BlockStatement) Idx1() file.Idx { return self.RightBrace + 1 }
func (self *BranchStatement) Idx1() file.Idx {
	if self.Label == nil {
		return file.Idx(int(self.Idx) + len(self.Token.String()))
	}
	return self.Label.Idx1()
}
func (self *CaseStatement) Idx1() file.Idx       { return self.Consequent[len(self.Consequent)-1].Idx1() }
func (self *CatchStatement) Idx1() file.Idx      { return self.Body.Idx1() }
func (self *DebuggerStatement) Idx1() file.Idx   { return self.Debugger + 8 }
func (self *DoWhileStatement) Idx1() file.Idx    { return self.RightParenthesis + 1 }
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
func (self *LabelledStatement) Idx1() file.Idx { return self.Statement.Idx1() }
func (self *Program) Idx1() file.Idx           { return self.Body[len(self.Body)-1].Idx1() }
func (self *ReturnStatement) Idx1() file.Idx {
	if self.Argument != nil {
		return self.Argument.Idx1()
	}
	return self.Return + 6
}
func (self *SwitchStatement) Idx1() file.Idx { return self.RightBrace + 1 }
func (self *ThrowStatement) Idx1() file.Idx  { return self.Argument.Idx1() }
func (self *TryStatement) Idx1() file.Idx {
	if self.Finally != nil {
		return self.Finally.Idx1()
	}
	if self.Catch != nil {
		return self.Catch.Idx1()
	}
	return self.Body.Idx1()
}
func (self *VariableStatement) Idx1() file.Idx   { return self.List[len(self.List)-1].Idx1() }
func (self *WhileStatement) Idx1() file.Idx      { return self.Body.Idx1() }
func (self *WithStatement) Idx1() file.Idx       { return self.Body.Idx1() }
func (self *LexicalDeclaration) Idx1() file.Idx  { return self.List[len(self.List)-1].Idx1() }
func (self *FunctionDeclaration) Idx1() file.Idx { return self.Function.Idx1() }
func (self *ClassDeclaration) Idx1() file.Idx    { return self.Class.Idx1() }
func (self *Binding) Idx1() file.Idx {
	if self.Initializer != nil {
		return self.Initializer.Idx1()
	}
	return self.Target.Idx1()
}

func (self *ForLoopInitializerExpression) Idx1() file.Idx  { return self.Expression.Idx1() }
func (self *ForLoopInitializerVarDeclList) Idx1() file.Idx { return self.List[len(self.List)-1].Idx1() }
func (self *ForLoopInitializerLexicalDecl) Idx1() file.Idx { return self.LexicalDeclaration.Idx1() }

func (self *PropertyShort) Idx1() file.Idx {
	if self.Initializer != nil {
		return self.Initializer.Idx1()
	}
	return self.Name.Idx1()
}

func (self *PropertyKeyed) Idx1() file.Idx { return self.Value.Idx1() }

func (self *ExpressionBody) Idx1() file.Idx { return self.Expression.Idx1() }

func (self *VariableDeclaration) Idx1() file.Idx {
	if len(self.List) > 0 {
		return self.List[len(self.List)-1].Idx1()
	}

	return self.Var + 3
}

func (self *FieldDefinition) Idx1() file.Idx {
	if self.Initializer != nil {
		return self.Initializer.Idx1()
	}
	return self.Key.Idx1()
}

func (self *MethodDefinition) Idx1() file.Idx {
	return self.Body.Idx1()
}

func (self *ClassStaticBlock) Idx1() file.Idx {
	return self.Block.Idx1()
}

func (self *YieldExpression) Idx1() file.Idx {
	if self.Argument != nil {
		return self.Argument.Idx1()
	}
	return self.Yield + 5
}

func (self *ForDeclaration) Idx1() file.Idx    { return self.Target.Idx1() }
func (self *ForIntoVar) Idx1() file.Idx        { return self.Binding.Idx1() }
func (self *ForIntoExpression) Idx1() file.Idx { return self.Expression.Idx1() }
