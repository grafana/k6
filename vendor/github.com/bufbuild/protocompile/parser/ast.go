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

package parser

import (
	"github.com/bufbuild/protocompile/ast"
)

// the types below are accumulator types, just used in intermediate productions
// to accumulate slices that will get stored in AST nodes

type compactOptionSlices struct {
	options []*ast.OptionNode
	commas  []*ast.RuneNode
}

func toStringValueNode(strs []*ast.StringLiteralNode) ast.StringValueNode {
	if len(strs) == 1 {
		return strs[0]
	}
	return ast.NewCompoundLiteralStringNode(strs...)
}

type nameSlices struct {
	// only names or idents will be set, never both
	names  []ast.StringValueNode
	idents []*ast.IdentNode
	commas []*ast.RuneNode
}

type rangeSlices struct {
	ranges []*ast.RangeNode
	commas []*ast.RuneNode
}

type valueSlices struct {
	vals   []ast.ValueNode
	commas []*ast.RuneNode
}

type fieldRefSlices struct {
	refs []*ast.FieldReferenceNode
	dots []*ast.RuneNode
}

type identSlices struct {
	idents []*ast.IdentNode
	dots   []*ast.RuneNode
}

func (s *identSlices) toIdentValueNode(leadingDot *ast.RuneNode) ast.IdentValueNode {
	if len(s.idents) == 1 && leadingDot == nil {
		// single simple name
		return s.idents[0]
	}
	return ast.NewCompoundIdentNode(leadingDot, s.idents, s.dots)
}

type messageFieldList struct {
	field     *ast.MessageFieldNode
	delimiter *ast.RuneNode
	next      *messageFieldList
}

func (list *messageFieldList) toNodes() ([]*ast.MessageFieldNode, []*ast.RuneNode) {
	if list == nil {
		return nil, nil
	}
	l := 0
	for cur := list; cur != nil; cur = cur.next {
		l++
	}
	fields := make([]*ast.MessageFieldNode, l)
	delimiters := make([]*ast.RuneNode, l)
	for cur, i := list, 0; cur != nil; cur, i = cur.next, i+1 {
		fields[i] = cur.field
		if cur.delimiter != nil {
			delimiters[i] = cur.delimiter
		}
	}
	return fields, delimiters
}

func prependRunes[T ast.Node](convert func(*ast.RuneNode) T, runes []*ast.RuneNode, elements []T) []T {
	elems := make([]T, 0, len(runes)+len(elements))
	for _, rune := range runes {
		elems = append(elems, convert(rune))
	}
	elems = append(elems, elements...)
	return elems
}

func toServiceElement(semi *ast.RuneNode) ast.ServiceElement {
	return ast.NewEmptyDeclNode(semi)
}

func toMethodElement(semi *ast.RuneNode) ast.RPCElement {
	return ast.NewEmptyDeclNode(semi)
}

func toFileElement(semi *ast.RuneNode) ast.FileElement {
	return ast.NewEmptyDeclNode(semi)
}

func toEnumElement(semi *ast.RuneNode) ast.EnumElement {
	return ast.NewEmptyDeclNode(semi)
}

func toMessageElement(semi *ast.RuneNode) ast.MessageElement {
	return ast.NewEmptyDeclNode(semi)
}

type nodeWithRunes[T ast.Node] struct {
	Node  T
	Runes []*ast.RuneNode
}

func newNodeWithRunes[T ast.Node](node T, trailingRunes ...*ast.RuneNode) nodeWithRunes[T] {
	return nodeWithRunes[T]{
		Node:  node,
		Runes: trailingRunes,
	}
}

func toElements[T ast.Node](convert func(*ast.RuneNode) T, node T, runes []*ast.RuneNode) []T {
	elements := make([]T, 1+len(runes))
	elements[0] = node
	for i, rune := range runes {
		elements[i+1] = convert(rune)
	}
	return elements
}
