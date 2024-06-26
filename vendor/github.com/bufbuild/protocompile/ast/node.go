// Copyright 2020-2024 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ast

// Node is the interface implemented by all nodes in the AST. It
// provides information about the span of this AST node in terms
// of location in the source file. It also provides information
// about all prior comments (attached as leading comments) and
// optional subsequent comments (attached as trailing comments).
type Node interface {
	Start() Token
	End() Token
}

// TerminalNode represents a leaf in the AST. These represent
// the items/lexemes in the protobuf language. Comments and
// whitespace are accumulated by the lexer and associated with
// the following lexed token.
type TerminalNode interface {
	Node
	Token() Token
}

var _ TerminalNode = (*StringLiteralNode)(nil)
var _ TerminalNode = (*UintLiteralNode)(nil)
var _ TerminalNode = (*FloatLiteralNode)(nil)
var _ TerminalNode = (*IdentNode)(nil)
var _ TerminalNode = (*SpecialFloatLiteralNode)(nil)
var _ TerminalNode = (*KeywordNode)(nil)
var _ TerminalNode = (*RuneNode)(nil)

// CompositeNode represents any non-terminal node in the tree. These
// are interior or root nodes and have child nodes.
type CompositeNode interface {
	Node
	// Children contains all AST nodes that are immediate children of this one.
	Children() []Node
}

// terminalNode contains bookkeeping shared by all TerminalNode
// implementations. It is embedded in all such node types in this
// package. It provides the implementation of the TerminalNode
// interface.
type terminalNode Token

func (n terminalNode) Start() Token {
	return Token(n)
}

func (n terminalNode) End() Token {
	return Token(n)
}

func (n terminalNode) Token() Token {
	return Token(n)
}

// compositeNode contains bookkeeping shared by all CompositeNode
// implementations. It is embedded in all such node types in this
// package. It provides the implementation of the CompositeNode
// interface.
type compositeNode struct {
	children []Node
}

func (n *compositeNode) Children() []Node {
	return n.children
}

func (n *compositeNode) Start() Token {
	return n.children[0].Start()
}

func (n *compositeNode) End() Token {
	return n.children[len(n.children)-1].End()
}

// RuneNode represents a single rune in protobuf source. Runes
// are typically collected into items, but some runes stand on
// their own, such as punctuation/symbols like commas, semicolons,
// equals signs, open and close symbols (braces, brackets, angles,
// and parentheses), and periods/dots.
// TODO: make this more compact; if runes don't have attributed comments
// then we don't need a Token to represent them and only need an offset
// into the file's contents.
type RuneNode struct {
	terminalNode
	Rune rune
}

// NewRuneNode creates a new *RuneNode with the given properties.
func NewRuneNode(r rune, tok Token) *RuneNode {
	return &RuneNode{
		terminalNode: tok.asTerminalNode(),
		Rune:         r,
	}
}

// EmptyDeclNode represents an empty declaration in protobuf source.
// These amount to extra semicolons, with no actual content preceding
// the semicolon.
type EmptyDeclNode struct {
	compositeNode
	Semicolon *RuneNode
}

// NewEmptyDeclNode creates a new *EmptyDeclNode. The one argument must
// be non-nil.
func NewEmptyDeclNode(semicolon *RuneNode) *EmptyDeclNode {
	if semicolon == nil {
		panic("semicolon is nil")
	}
	return &EmptyDeclNode{
		compositeNode: compositeNode{
			children: []Node{semicolon},
		},
		Semicolon: semicolon,
	}
}

func (e *EmptyDeclNode) fileElement()    {}
func (e *EmptyDeclNode) msgElement()     {}
func (e *EmptyDeclNode) extendElement()  {}
func (e *EmptyDeclNode) oneofElement()   {}
func (e *EmptyDeclNode) enumElement()    {}
func (e *EmptyDeclNode) serviceElement() {}
func (e *EmptyDeclNode) methodElement()  {}
