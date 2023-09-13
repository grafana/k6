// Copyright 2020-2023 Buf Technologies, Inc.
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

import "fmt"

// Walk conducts a walk of the AST rooted at the given root using the
// given visitor. It performs a "pre-order traversal", visiting a
// given AST node before it visits that node's descendants.
//
// If a visitor returns an error while walking the tree, the entire
// operation is aborted and that error is returned.
func Walk(root Node, v Visitor, opts ...WalkOption) error {
	var wOpts walkOptions
	for _, opt := range opts {
		opt(&wOpts)
	}
	return walk(root, v, wOpts)
}

// WalkOption represents an option used with the Walk function. These
// allow optional before and after hooks to be invoked as each node in
// the tree is visited.
type WalkOption func(*walkOptions)

type walkOptions struct {
	before, after func(Node) error
}

// WithBefore returns a WalkOption that will cause the given function to be
// invoked before a node is visited during a walk operation. If this hook
// returns an error, the node is not visited and the walk operation is aborted.
func WithBefore(fn func(Node) error) WalkOption {
	return func(options *walkOptions) {
		options.before = fn
	}
}

// WithAfter returns a WalkOption that will cause the given function to be
// invoked after a node (as well as any descendants) is visited during a walk
// operation. If this hook returns an error, the node is not visited and the
// walk operation is aborted.
//
// If the walk is aborted due to some other visitor or before hook returning an
// error, the after hook is still called for all nodes that have been visited.
// However, the walk operation fails with the first error it encountered, so any
// error returned from an after hook is effectively ignored.
func WithAfter(fn func(Node) error) WalkOption {
	return func(options *walkOptions) {
		options.after = fn
	}
}

func walk(root Node, v Visitor, opts walkOptions) (err error) {
	if opts.before != nil {
		if err := opts.before(root); err != nil {
			return err
		}
	}
	if opts.after != nil {
		defer func() {
			if afterErr := opts.after(root); afterErr != nil {
				// if another call already returned an error then we
				// have to ignore the error from the after hook
				if err == nil {
					err = afterErr
				}
			}
		}()
	}

	if err := Visit(root, v); err != nil {
		return err
	}

	if comp, ok := root.(CompositeNode); ok {
		for _, child := range comp.Children() {
			if err := walk(child, v, opts); err != nil {
				return err
			}
		}
	}
	return nil
}

// Visit implements the double-dispatch idiom and visits the given node by
// calling the appropriate method of the given visitor.
func Visit(n Node, v Visitor) error {
	switch n := n.(type) {
	case *FileNode:
		return v.VisitFileNode(n)
	case *SyntaxNode:
		return v.VisitSyntaxNode(n)
	case *PackageNode:
		return v.VisitPackageNode(n)
	case *ImportNode:
		return v.VisitImportNode(n)
	case *OptionNode:
		return v.VisitOptionNode(n)
	case *OptionNameNode:
		return v.VisitOptionNameNode(n)
	case *FieldReferenceNode:
		return v.VisitFieldReferenceNode(n)
	case *CompactOptionsNode:
		return v.VisitCompactOptionsNode(n)
	case *MessageNode:
		return v.VisitMessageNode(n)
	case *ExtendNode:
		return v.VisitExtendNode(n)
	case *ExtensionRangeNode:
		return v.VisitExtensionRangeNode(n)
	case *ReservedNode:
		return v.VisitReservedNode(n)
	case *RangeNode:
		return v.VisitRangeNode(n)
	case *FieldNode:
		return v.VisitFieldNode(n)
	case *GroupNode:
		return v.VisitGroupNode(n)
	case *MapFieldNode:
		return v.VisitMapFieldNode(n)
	case *MapTypeNode:
		return v.VisitMapTypeNode(n)
	case *OneofNode:
		return v.VisitOneofNode(n)
	case *EnumNode:
		return v.VisitEnumNode(n)
	case *EnumValueNode:
		return v.VisitEnumValueNode(n)
	case *ServiceNode:
		return v.VisitServiceNode(n)
	case *RPCNode:
		return v.VisitRPCNode(n)
	case *RPCTypeNode:
		return v.VisitRPCTypeNode(n)
	case *IdentNode:
		return v.VisitIdentNode(n)
	case *CompoundIdentNode:
		return v.VisitCompoundIdentNode(n)
	case *StringLiteralNode:
		return v.VisitStringLiteralNode(n)
	case *CompoundStringLiteralNode:
		return v.VisitCompoundStringLiteralNode(n)
	case *UintLiteralNode:
		return v.VisitUintLiteralNode(n)
	case *PositiveUintLiteralNode:
		return v.VisitPositiveUintLiteralNode(n)
	case *NegativeIntLiteralNode:
		return v.VisitNegativeIntLiteralNode(n)
	case *FloatLiteralNode:
		return v.VisitFloatLiteralNode(n)
	case *SpecialFloatLiteralNode:
		return v.VisitSpecialFloatLiteralNode(n)
	case *SignedFloatLiteralNode:
		return v.VisitSignedFloatLiteralNode(n)
	case *ArrayLiteralNode:
		return v.VisitArrayLiteralNode(n)
	case *MessageLiteralNode:
		return v.VisitMessageLiteralNode(n)
	case *MessageFieldNode:
		return v.VisitMessageFieldNode(n)
	case *KeywordNode:
		return v.VisitKeywordNode(n)
	case *RuneNode:
		return v.VisitRuneNode(n)
	case *EmptyDeclNode:
		return v.VisitEmptyDeclNode(n)
	default:
		panic(fmt.Sprintf("unexpected type of node: %T", n))
	}
}

// AncestorTracker is used to track the path of nodes during a walk operation.
// By passing AsWalkOptions to a call to Walk, a visitor can inspect the path to
// the node being visited using this tracker.
type AncestorTracker struct {
	ancestors []Node
}

// AsWalkOptions returns WalkOption values that will cause this ancestor tracker
// to track the path through the AST during the walk operation.
func (t *AncestorTracker) AsWalkOptions() []WalkOption {
	return []WalkOption{
		WithBefore(func(n Node) error {
			t.ancestors = append(t.ancestors, n)
			return nil
		}),
		WithAfter(func(n Node) error {
			t.ancestors = t.ancestors[:len(t.ancestors)-1]
			return nil
		}),
	}
}

// Path returns a slice of nodes that represents the path from the root of the
// walk operaiton to the currently visited node. The first element in the path
// is the root supplied to Walk. The last element in the path is the currently
// visited node.
//
// The returned slice is not a defensive copy; so callers should NOT mutate it.
func (t *AncestorTracker) Path() []Node {
	return t.ancestors
}

// Parent returns the parent node of the currently visited node. If the node
// currently being visited is the root supplied to Walk then nil is returned.
func (t *AncestorTracker) Parent() Node {
	if len(t.ancestors) <= 1 {
		return nil
	}
	return t.ancestors[len(t.ancestors)-2]
}

// VisitChildren visits all direct children of the given node using the given
// visitor. If visiting a child returns an error, that error is immediately
// returned, and other children will not be visited.
func VisitChildren(n CompositeNode, v Visitor) error {
	for _, ch := range n.Children() {
		if err := Visit(ch, v); err != nil {
			return err
		}
	}
	return nil
}

// Visitor provides a technique for walking the AST that allows for
// dynamic dispatch, where a particular function is invoked based on
// the runtime type of the argument.
//
// It consists of a number of functions, each of which matches a
// concrete Node type.
//
// Most visitor implementations will either embed NoOpVisitor (so as
// not to have to implement *all* of the methods) or will be instances
// of SimpleVisitor.
//
// Visitors can be supplied to a Walk operation or passed to a call
// to Visit or VisitChildren.
type Visitor interface {
	// VisitFileNode is invoked when visiting a *FileNode in the AST.
	VisitFileNode(*FileNode) error
	// VisitSyntaxNode is invoked when visiting a *SyntaxNode in the AST.
	VisitSyntaxNode(*SyntaxNode) error
	// VisitPackageNode is invoked when visiting a *PackageNode in the AST.
	VisitPackageNode(*PackageNode) error
	// VisitImportNode is invoked when visiting an *ImportNode in the AST.
	VisitImportNode(*ImportNode) error
	// VisitOptionNode is invoked when visiting an *OptionNode in the AST.
	VisitOptionNode(*OptionNode) error
	// VisitOptionNameNode is invoked when visiting an *OptionNameNode in the AST.
	VisitOptionNameNode(*OptionNameNode) error
	// VisitFieldReferenceNode is invoked when visiting a *FieldReferenceNode in the AST.
	VisitFieldReferenceNode(*FieldReferenceNode) error
	// VisitCompactOptionsNode is invoked when visiting a *CompactOptionsNode in the AST.
	VisitCompactOptionsNode(*CompactOptionsNode) error
	// VisitMessageNode is invoked when visiting a *MessageNode in the AST.
	VisitMessageNode(*MessageNode) error
	// VisitExtendNode is invoked when visiting an *ExtendNode in the AST.
	VisitExtendNode(*ExtendNode) error
	// VisitExtensionRangeNode is invoked when visiting an *ExtensionRangeNode in the AST.
	VisitExtensionRangeNode(*ExtensionRangeNode) error
	// VisitReservedNode is invoked when visiting a *ReservedNode in the AST.
	VisitReservedNode(*ReservedNode) error
	// VisitRangeNode is invoked when visiting a *RangeNode in the AST.
	VisitRangeNode(*RangeNode) error
	// VisitFieldNode is invoked when visiting a *FieldNode in the AST.
	VisitFieldNode(*FieldNode) error
	// VisitGroupNode is invoked when visiting a *GroupNode in the AST.
	VisitGroupNode(*GroupNode) error
	// VisitMapFieldNode is invoked when visiting a *MapFieldNode in the AST.
	VisitMapFieldNode(*MapFieldNode) error
	// VisitMapTypeNode is invoked when visiting a *MapTypeNode in the AST.
	VisitMapTypeNode(*MapTypeNode) error
	// VisitOneofNode is invoked when visiting a *OneofNode in the AST.
	VisitOneofNode(*OneofNode) error
	// VisitEnumNode is invoked when visiting an *EnumNode in the AST.
	VisitEnumNode(*EnumNode) error
	// VisitEnumValueNode is invoked when visiting an *EnumValueNode in the AST.
	VisitEnumValueNode(*EnumValueNode) error
	// VisitServiceNode is invoked when visiting a *ServiceNode in the AST.
	VisitServiceNode(*ServiceNode) error
	// VisitRPCNode is invoked when visiting an *RPCNode in the AST.
	VisitRPCNode(*RPCNode) error
	// VisitRPCTypeNode is invoked when visiting an *RPCTypeNode in the AST.
	VisitRPCTypeNode(*RPCTypeNode) error
	// VisitIdentNode is invoked when visiting an *IdentNode in the AST.
	VisitIdentNode(*IdentNode) error
	// VisitCompoundIdentNode is invoked when visiting a *CompoundIdentNode in the AST.
	VisitCompoundIdentNode(*CompoundIdentNode) error
	// VisitStringLiteralNode is invoked when visiting a *StringLiteralNode in the AST.
	VisitStringLiteralNode(*StringLiteralNode) error
	// VisitCompoundStringLiteralNode is invoked when visiting a *CompoundStringLiteralNode in the AST.
	VisitCompoundStringLiteralNode(*CompoundStringLiteralNode) error
	// VisitUintLiteralNode is invoked when visiting a *UintLiteralNode in the AST.
	VisitUintLiteralNode(*UintLiteralNode) error
	// VisitPositiveUintLiteralNode is invoked when visiting a *PositiveUintLiteralNode in the AST.
	VisitPositiveUintLiteralNode(*PositiveUintLiteralNode) error
	// VisitNegativeIntLiteralNode is invoked when visiting a *NegativeIntLiteralNode in the AST.
	VisitNegativeIntLiteralNode(*NegativeIntLiteralNode) error
	// VisitFloatLiteralNode is invoked when visiting a *FloatLiteralNode in the AST.
	VisitFloatLiteralNode(*FloatLiteralNode) error
	// VisitSpecialFloatLiteralNode is invoked when visiting a *SpecialFloatLiteralNode in the AST.
	VisitSpecialFloatLiteralNode(*SpecialFloatLiteralNode) error
	// VisitSignedFloatLiteralNode is invoked when visiting a *SignedFloatLiteralNode in the AST.
	VisitSignedFloatLiteralNode(*SignedFloatLiteralNode) error
	// VisitArrayLiteralNode is invoked when visiting an *ArrayLiteralNode in the AST.
	VisitArrayLiteralNode(*ArrayLiteralNode) error
	// VisitMessageLiteralNode is invoked when visiting a *MessageLiteralNode in the AST.
	VisitMessageLiteralNode(*MessageLiteralNode) error
	// VisitMessageFieldNode is invoked when visiting a *MessageFieldNode in the AST.
	VisitMessageFieldNode(*MessageFieldNode) error
	// VisitKeywordNode is invoked when visiting a *KeywordNode in the AST.
	VisitKeywordNode(*KeywordNode) error
	// VisitRuneNode is invoked when visiting a *RuneNode in the AST.
	VisitRuneNode(*RuneNode) error
	// VisitEmptyDeclNode is invoked when visiting a *EmptyDeclNode in the AST.
	VisitEmptyDeclNode(*EmptyDeclNode) error
}

// NoOpVisitor is a visitor implementation that does nothing. All methods
// unconditionally return nil. This can be embedded into a struct to make that
// struct implement the Visitor interface, and only the relevant visit methods
// then need to be implemented on the struct.
type NoOpVisitor struct{}

var _ Visitor = NoOpVisitor{}

func (n NoOpVisitor) VisitFileNode(_ *FileNode) error {
	return nil
}

func (n NoOpVisitor) VisitSyntaxNode(_ *SyntaxNode) error {
	return nil
}

func (n NoOpVisitor) VisitPackageNode(_ *PackageNode) error {
	return nil
}

func (n NoOpVisitor) VisitImportNode(_ *ImportNode) error {
	return nil
}

func (n NoOpVisitor) VisitOptionNode(_ *OptionNode) error {
	return nil
}

func (n NoOpVisitor) VisitOptionNameNode(_ *OptionNameNode) error {
	return nil
}

func (n NoOpVisitor) VisitFieldReferenceNode(_ *FieldReferenceNode) error {
	return nil
}

func (n NoOpVisitor) VisitCompactOptionsNode(_ *CompactOptionsNode) error {
	return nil
}

func (n NoOpVisitor) VisitMessageNode(_ *MessageNode) error {
	return nil
}

func (n NoOpVisitor) VisitExtendNode(_ *ExtendNode) error {
	return nil
}

func (n NoOpVisitor) VisitExtensionRangeNode(_ *ExtensionRangeNode) error {
	return nil
}

func (n NoOpVisitor) VisitReservedNode(_ *ReservedNode) error {
	return nil
}

func (n NoOpVisitor) VisitRangeNode(_ *RangeNode) error {
	return nil
}

func (n NoOpVisitor) VisitFieldNode(_ *FieldNode) error {
	return nil
}

func (n NoOpVisitor) VisitGroupNode(_ *GroupNode) error {
	return nil
}

func (n NoOpVisitor) VisitMapFieldNode(_ *MapFieldNode) error {
	return nil
}

func (n NoOpVisitor) VisitMapTypeNode(_ *MapTypeNode) error {
	return nil
}

func (n NoOpVisitor) VisitOneofNode(_ *OneofNode) error {
	return nil
}

func (n NoOpVisitor) VisitEnumNode(_ *EnumNode) error {
	return nil
}

func (n NoOpVisitor) VisitEnumValueNode(_ *EnumValueNode) error {
	return nil
}

func (n NoOpVisitor) VisitServiceNode(_ *ServiceNode) error {
	return nil
}

func (n NoOpVisitor) VisitRPCNode(_ *RPCNode) error {
	return nil
}

func (n NoOpVisitor) VisitRPCTypeNode(_ *RPCTypeNode) error {
	return nil
}

func (n NoOpVisitor) VisitIdentNode(_ *IdentNode) error {
	return nil
}

func (n NoOpVisitor) VisitCompoundIdentNode(_ *CompoundIdentNode) error {
	return nil
}

func (n NoOpVisitor) VisitStringLiteralNode(_ *StringLiteralNode) error {
	return nil
}

func (n NoOpVisitor) VisitCompoundStringLiteralNode(_ *CompoundStringLiteralNode) error {
	return nil
}

func (n NoOpVisitor) VisitUintLiteralNode(_ *UintLiteralNode) error {
	return nil
}

func (n NoOpVisitor) VisitPositiveUintLiteralNode(_ *PositiveUintLiteralNode) error {
	return nil
}

func (n NoOpVisitor) VisitNegativeIntLiteralNode(_ *NegativeIntLiteralNode) error {
	return nil
}

func (n NoOpVisitor) VisitFloatLiteralNode(_ *FloatLiteralNode) error {
	return nil
}

func (n NoOpVisitor) VisitSpecialFloatLiteralNode(_ *SpecialFloatLiteralNode) error {
	return nil
}

func (n NoOpVisitor) VisitSignedFloatLiteralNode(_ *SignedFloatLiteralNode) error {
	return nil
}

func (n NoOpVisitor) VisitArrayLiteralNode(_ *ArrayLiteralNode) error {
	return nil
}

func (n NoOpVisitor) VisitMessageLiteralNode(_ *MessageLiteralNode) error {
	return nil
}

func (n NoOpVisitor) VisitMessageFieldNode(_ *MessageFieldNode) error {
	return nil
}

func (n NoOpVisitor) VisitKeywordNode(_ *KeywordNode) error {
	return nil
}

func (n NoOpVisitor) VisitRuneNode(_ *RuneNode) error {
	return nil
}

func (n NoOpVisitor) VisitEmptyDeclNode(_ *EmptyDeclNode) error {
	return nil
}

// SimpleVisitor is a visitor implementation that uses numerous function fields.
// If a relevant function field is not nil, then it will be invoked when a node
// is visited.
//
// In addition to a function for each concrete node type (and thus for each
// Visit* method of the Visitor interface), it also has function fields that
// accept interface types. So a visitor can, for example, easily treat all
// ValueNodes uniformly by providing a non-nil value for DoVisitValueNode
// instead of having to supply values for the various DoVisit*Node methods
// corresponding to all types that implement ValueNode.
//
// The most specific function provided that matches a given node is the one that
// will be invoked. For example, DoVisitStringValueNode will be called if
// present and applicable before DoVisitValueNode. Similarly, DoVisitValueNode
// would be called before DoVisitTerminalNode or DoVisitCompositeNode. The
// DoVisitNode is the most generic function and is called only if no more
// specific function is present for a given node type.
//
// The *UintLiteralNode type implements both IntValueNode and FloatValueNode.
// In this case, the DoVisitIntValueNode function is considered more specific
// than DoVisitFloatValueNode, so will be preferred if present.
//
// Similarly, *MapFieldNode and *GroupNode implement both FieldDeclNode and
// MessageDeclNode. In this case, the DoVisitFieldDeclNode function is
// treated as more specific than DoVisitMessageDeclNode, so will be preferred
// if both are present.
type SimpleVisitor struct {
	DoVisitFileNode                  func(*FileNode) error
	DoVisitSyntaxNode                func(*SyntaxNode) error
	DoVisitPackageNode               func(*PackageNode) error
	DoVisitImportNode                func(*ImportNode) error
	DoVisitOptionNode                func(*OptionNode) error
	DoVisitOptionNameNode            func(*OptionNameNode) error
	DoVisitFieldReferenceNode        func(*FieldReferenceNode) error
	DoVisitCompactOptionsNode        func(*CompactOptionsNode) error
	DoVisitMessageNode               func(*MessageNode) error
	DoVisitExtendNode                func(*ExtendNode) error
	DoVisitExtensionRangeNode        func(*ExtensionRangeNode) error
	DoVisitReservedNode              func(*ReservedNode) error
	DoVisitRangeNode                 func(*RangeNode) error
	DoVisitFieldNode                 func(*FieldNode) error
	DoVisitGroupNode                 func(*GroupNode) error
	DoVisitMapFieldNode              func(*MapFieldNode) error
	DoVisitMapTypeNode               func(*MapTypeNode) error
	DoVisitOneofNode                 func(*OneofNode) error
	DoVisitEnumNode                  func(*EnumNode) error
	DoVisitEnumValueNode             func(*EnumValueNode) error
	DoVisitServiceNode               func(*ServiceNode) error
	DoVisitRPCNode                   func(*RPCNode) error
	DoVisitRPCTypeNode               func(*RPCTypeNode) error
	DoVisitIdentNode                 func(*IdentNode) error
	DoVisitCompoundIdentNode         func(*CompoundIdentNode) error
	DoVisitStringLiteralNode         func(*StringLiteralNode) error
	DoVisitCompoundStringLiteralNode func(*CompoundStringLiteralNode) error
	DoVisitUintLiteralNode           func(*UintLiteralNode) error
	DoVisitPositiveUintLiteralNode   func(*PositiveUintLiteralNode) error
	DoVisitNegativeIntLiteralNode    func(*NegativeIntLiteralNode) error
	DoVisitFloatLiteralNode          func(*FloatLiteralNode) error
	DoVisitSpecialFloatLiteralNode   func(*SpecialFloatLiteralNode) error
	DoVisitSignedFloatLiteralNode    func(*SignedFloatLiteralNode) error
	DoVisitArrayLiteralNode          func(*ArrayLiteralNode) error
	DoVisitMessageLiteralNode        func(*MessageLiteralNode) error
	DoVisitMessageFieldNode          func(*MessageFieldNode) error
	DoVisitKeywordNode               func(*KeywordNode) error
	DoVisitRuneNode                  func(*RuneNode) error
	DoVisitEmptyDeclNode             func(*EmptyDeclNode) error

	DoVisitFieldDeclNode   func(FieldDeclNode) error
	DoVisitMessageDeclNode func(MessageDeclNode) error

	DoVisitIdentValueNode  func(IdentValueNode) error
	DoVisitStringValueNode func(StringValueNode) error
	DoVisitIntValueNode    func(IntValueNode) error
	DoVisitFloatValueNode  func(FloatValueNode) error
	DoVisitValueNode       func(ValueNode) error

	DoVisitTerminalNode  func(TerminalNode) error
	DoVisitCompositeNode func(CompositeNode) error
	DoVisitNode          func(Node) error
}

var _ Visitor = (*SimpleVisitor)(nil)

func (b *SimpleVisitor) visitInterface(node Node) error {
	switch n := node.(type) {
	case FieldDeclNode:
		if b.DoVisitFieldDeclNode != nil {
			return b.DoVisitFieldDeclNode(n)
		}
		// *MapFieldNode and *GroupNode both implement both FieldDeclNode and
		// MessageDeclNode, so handle other case here
		if fn, ok := n.(MessageDeclNode); ok && b.DoVisitMessageDeclNode != nil {
			return b.DoVisitMessageDeclNode(fn)
		}
	case MessageDeclNode:
		if b.DoVisitMessageDeclNode != nil {
			return b.DoVisitMessageDeclNode(n)
		}
	case IdentValueNode:
		if b.DoVisitIdentValueNode != nil {
			return b.DoVisitIdentValueNode(n)
		}
	case StringValueNode:
		if b.DoVisitStringValueNode != nil {
			return b.DoVisitStringValueNode(n)
		}
	case IntValueNode:
		if b.DoVisitIntValueNode != nil {
			return b.DoVisitIntValueNode(n)
		}
		// *UintLiteralNode implements both IntValueNode and FloatValueNode,
		// so handle other case here
		if fn, ok := n.(FloatValueNode); ok && b.DoVisitFloatValueNode != nil {
			return b.DoVisitFloatValueNode(fn)
		}
	case FloatValueNode:
		if b.DoVisitFloatValueNode != nil {
			return b.DoVisitFloatValueNode(n)
		}
	}

	if n, ok := node.(ValueNode); ok && b.DoVisitValueNode != nil {
		return b.DoVisitValueNode(n)
	}

	switch n := node.(type) {
	case TerminalNode:
		if b.DoVisitTerminalNode != nil {
			return b.DoVisitTerminalNode(n)
		}
	case CompositeNode:
		if b.DoVisitCompositeNode != nil {
			return b.DoVisitCompositeNode(n)
		}
	}

	if b.DoVisitNode != nil {
		return b.DoVisitNode(node)
	}

	return nil
}

func (b *SimpleVisitor) VisitFileNode(node *FileNode) error {
	if b.DoVisitFileNode != nil {
		return b.DoVisitFileNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitSyntaxNode(node *SyntaxNode) error {
	if b.DoVisitSyntaxNode != nil {
		return b.DoVisitSyntaxNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitPackageNode(node *PackageNode) error {
	if b.DoVisitPackageNode != nil {
		return b.DoVisitPackageNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitImportNode(node *ImportNode) error {
	if b.DoVisitImportNode != nil {
		return b.DoVisitImportNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitOptionNode(node *OptionNode) error {
	if b.DoVisitOptionNode != nil {
		return b.DoVisitOptionNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitOptionNameNode(node *OptionNameNode) error {
	if b.DoVisitOptionNameNode != nil {
		return b.DoVisitOptionNameNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitFieldReferenceNode(node *FieldReferenceNode) error {
	if b.DoVisitFieldReferenceNode != nil {
		return b.DoVisitFieldReferenceNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitCompactOptionsNode(node *CompactOptionsNode) error {
	if b.DoVisitCompactOptionsNode != nil {
		return b.DoVisitCompactOptionsNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitMessageNode(node *MessageNode) error {
	if b.DoVisitMessageNode != nil {
		return b.DoVisitMessageNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitExtendNode(node *ExtendNode) error {
	if b.DoVisitExtendNode != nil {
		return b.DoVisitExtendNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitExtensionRangeNode(node *ExtensionRangeNode) error {
	if b.DoVisitExtensionRangeNode != nil {
		return b.DoVisitExtensionRangeNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitReservedNode(node *ReservedNode) error {
	if b.DoVisitReservedNode != nil {
		return b.DoVisitReservedNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitRangeNode(node *RangeNode) error {
	if b.DoVisitRangeNode != nil {
		return b.DoVisitRangeNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitFieldNode(node *FieldNode) error {
	if b.DoVisitFieldNode != nil {
		return b.DoVisitFieldNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitGroupNode(node *GroupNode) error {
	if b.DoVisitGroupNode != nil {
		return b.DoVisitGroupNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitMapFieldNode(node *MapFieldNode) error {
	if b.DoVisitMapFieldNode != nil {
		return b.DoVisitMapFieldNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitMapTypeNode(node *MapTypeNode) error {
	if b.DoVisitMapTypeNode != nil {
		return b.DoVisitMapTypeNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitOneofNode(node *OneofNode) error {
	if b.DoVisitOneofNode != nil {
		return b.DoVisitOneofNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitEnumNode(node *EnumNode) error {
	if b.DoVisitEnumNode != nil {
		return b.DoVisitEnumNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitEnumValueNode(node *EnumValueNode) error {
	if b.DoVisitEnumValueNode != nil {
		return b.DoVisitEnumValueNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitServiceNode(node *ServiceNode) error {
	if b.DoVisitServiceNode != nil {
		return b.DoVisitServiceNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitRPCNode(node *RPCNode) error {
	if b.DoVisitRPCNode != nil {
		return b.DoVisitRPCNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitRPCTypeNode(node *RPCTypeNode) error {
	if b.DoVisitRPCTypeNode != nil {
		return b.DoVisitRPCTypeNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitIdentNode(node *IdentNode) error {
	if b.DoVisitIdentNode != nil {
		return b.DoVisitIdentNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitCompoundIdentNode(node *CompoundIdentNode) error {
	if b.DoVisitCompoundIdentNode != nil {
		return b.DoVisitCompoundIdentNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitStringLiteralNode(node *StringLiteralNode) error {
	if b.DoVisitStringLiteralNode != nil {
		return b.DoVisitStringLiteralNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitCompoundStringLiteralNode(node *CompoundStringLiteralNode) error {
	if b.DoVisitCompoundStringLiteralNode != nil {
		return b.DoVisitCompoundStringLiteralNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitUintLiteralNode(node *UintLiteralNode) error {
	if b.DoVisitUintLiteralNode != nil {
		return b.DoVisitUintLiteralNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitPositiveUintLiteralNode(node *PositiveUintLiteralNode) error {
	if b.DoVisitPositiveUintLiteralNode != nil {
		return b.DoVisitPositiveUintLiteralNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitNegativeIntLiteralNode(node *NegativeIntLiteralNode) error {
	if b.DoVisitNegativeIntLiteralNode != nil {
		return b.DoVisitNegativeIntLiteralNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitFloatLiteralNode(node *FloatLiteralNode) error {
	if b.DoVisitFloatLiteralNode != nil {
		return b.DoVisitFloatLiteralNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitSpecialFloatLiteralNode(node *SpecialFloatLiteralNode) error {
	if b.DoVisitSpecialFloatLiteralNode != nil {
		return b.DoVisitSpecialFloatLiteralNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitSignedFloatLiteralNode(node *SignedFloatLiteralNode) error {
	if b.DoVisitSignedFloatLiteralNode != nil {
		return b.DoVisitSignedFloatLiteralNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitArrayLiteralNode(node *ArrayLiteralNode) error {
	if b.DoVisitArrayLiteralNode != nil {
		return b.DoVisitArrayLiteralNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitMessageLiteralNode(node *MessageLiteralNode) error {
	if b.DoVisitMessageLiteralNode != nil {
		return b.DoVisitMessageLiteralNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitMessageFieldNode(node *MessageFieldNode) error {
	if b.DoVisitMessageFieldNode != nil {
		return b.DoVisitMessageFieldNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitKeywordNode(node *KeywordNode) error {
	if b.DoVisitKeywordNode != nil {
		return b.DoVisitKeywordNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitRuneNode(node *RuneNode) error {
	if b.DoVisitRuneNode != nil {
		return b.DoVisitRuneNode(node)
	}
	return b.visitInterface(node)
}

func (b *SimpleVisitor) VisitEmptyDeclNode(node *EmptyDeclNode) error {
	if b.DoVisitEmptyDeclNode != nil {
		return b.DoVisitEmptyDeclNode(node)
	}
	return b.visitInterface(node)
}
