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

// Package ast defines types for modeling the AST (Abstract Syntax
// Tree) for the Protocol Buffers interface definition language.
//
// # Nodes
//
// All nodes of the tree implement the [Node] interface. Leaf nodes in the
// tree implement [TerminalNode], and all others implement [CompositeNode].
// The root of the tree for a proto source file is a *[FileNode].
//
// A [TerminalNode] represents a single lexical element, or [Token]. A
// [CompositeNode] represents a sub-tree of the AST and range of tokens.
//
// Position information is tracked using a *[FileInfo]. The lexer invokes its
// various Add* methods to add details as the file is tokenized. Storing
// the position information in the *[FileInfo], instead of in each AST node,
// allows the AST to have a much more compact representation. To extract
// detailed position information, you must use the NodeInfo method, available
// on either the *[FileInfo] which produced the node's items or the *[FileNode]
// root of the tree that contains the node.
//
// # Items, Tokens, and Comments
//
// An [Item] represents a lexical item, excluding whitespace. This can be
// either a [Token] or a [Comment].
//
// Comments are not represented as nodes in the tree. Instead, they are
// attributed to terminal nodes in the tree. So, when lexing, comments
// are accumulated until the next non-comment token is found. The AST
// model in this package thus provides access to all comments in the
// file, regardless of location (unlike the SourceCodeInfo present in
// descriptor protos, which is lossy). The comments associated with a
// non-leaf/non-token node (i.e. a CompositeNode) come from the first
// and last nodes in its sub-tree, for leading and trailing comments
// respectively.
//
// A [Comment] value corresponds to a line ("//") or block ("/*") style
// comment in the source. These have no bearing on the grammar and are
// effectively ignored as the parser is determining the shape of the
// syntax tree.
//
// A [Token] value corresponds to a component of the grammar, that is
// used to produce an AST. They correspond to leaves in the AST (i.e.
// [TerminalNode]).
//
// The *[FileInfo] and *[FileNode] types provide methods for querying
// and iterating through all the items or tokens in the file. They also
// include a method for resolving an [Item] into a [Token] or [Comment].
//
// # Factory Functions
//
// Creation of AST nodes should use the factory functions in this
// package instead of struct literals. Some factory functions accept
// optional arguments, which means the arguments can be nil. If nil
// values are provided for other (non-optional) arguments, the resulting
// node may be invalid and cause panics later in the program.
//
// This package defines numerous interfaces. However, user code should
// not attempt to implement any of them. Most consumers of an AST will
// not work correctly if they encounter concrete implementations other
// than the ones defined in this package.
package ast
