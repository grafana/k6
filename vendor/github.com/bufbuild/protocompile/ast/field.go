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

import "fmt"

// FieldDeclNode is a node in the AST that defines a field. This includes
// normal message fields as well as extensions. There are multiple types
// of AST nodes that declare fields:
//   - *FieldNode
//   - *GroupNode
//   - *MapFieldNode
//   - *SyntheticMapField
//
// This also allows NoSourceNode and SyntheticMapField to be used in place of
// one of the above for some usages.
type FieldDeclNode interface {
	NodeWithOptions
	FieldLabel() Node
	FieldName() Node
	FieldType() Node
	FieldTag() Node
	FieldExtendee() Node
	GetGroupKeyword() Node
	GetOptions() *CompactOptionsNode
}

var _ FieldDeclNode = (*FieldNode)(nil)
var _ FieldDeclNode = (*GroupNode)(nil)
var _ FieldDeclNode = (*MapFieldNode)(nil)
var _ FieldDeclNode = (*SyntheticMapField)(nil)
var _ FieldDeclNode = NoSourceNode{}

// FieldNode represents a normal field declaration (not groups or maps). It
// can represent extension fields as well as non-extension fields (both inside
// of messages and inside of one-ofs). Example:
//
//	optional string foo = 1;
type FieldNode struct {
	compositeNode
	Label     FieldLabel
	FldType   IdentValueNode
	Name      *IdentNode
	Equals    *RuneNode
	Tag       *UintLiteralNode
	Options   *CompactOptionsNode
	Semicolon *RuneNode

	// This is an up-link to the containing *ExtendNode for fields
	// that are defined inside of "extend" blocks.
	Extendee *ExtendNode
}

func (*FieldNode) msgElement()    {}
func (*FieldNode) oneofElement()  {}
func (*FieldNode) extendElement() {}

// NewFieldNode creates a new *FieldNode. The label and options arguments may be
// nil but the others must be non-nil.
//   - label: The token corresponding to the label keyword if present ("optional",
//     "required", or "repeated").
//   - fieldType: The token corresponding to the field's type.
//   - name: The token corresponding to the field's name.
//   - equals: The token corresponding to the '=' rune after the name.
//   - tag: The token corresponding to the field's tag number.
//   - opts: Optional set of field options.
//   - semicolon: The token corresponding to the ";" rune that ends the declaration.
func NewFieldNode(label *KeywordNode, fieldType IdentValueNode, name *IdentNode, equals *RuneNode, tag *UintLiteralNode, opts *CompactOptionsNode, semicolon *RuneNode) *FieldNode {
	if fieldType == nil {
		panic("fieldType is nil")
	}
	if name == nil {
		panic("name is nil")
	}
	numChildren := 2
	if equals != nil {
		numChildren++
	}
	if tag != nil {
		numChildren++
	}
	if semicolon != nil {
		numChildren++
	}
	if label != nil {
		numChildren++
	}
	if opts != nil {
		numChildren++
	}
	children := make([]Node, 0, numChildren)
	if label != nil {
		children = append(children, label)
	}
	children = append(children, fieldType, name)
	if equals != nil {
		children = append(children, equals)
	}
	if tag != nil {
		children = append(children, tag)
	}
	if opts != nil {
		children = append(children, opts)
	}
	if semicolon != nil {
		children = append(children, semicolon)
	}

	return &FieldNode{
		compositeNode: compositeNode{
			children: children,
		},
		Label:     newFieldLabel(label),
		FldType:   fieldType,
		Name:      name,
		Equals:    equals,
		Tag:       tag,
		Options:   opts,
		Semicolon: semicolon,
	}
}

func (n *FieldNode) FieldLabel() Node {
	// proto3 fields and fields inside one-ofs will not have a label and we need
	// this check in order to return a nil node -- otherwise we'd return a
	// non-nil node that has a nil pointer value in it :/
	if n.Label.KeywordNode == nil {
		return nil
	}
	return n.Label.KeywordNode
}

func (n *FieldNode) FieldName() Node {
	return n.Name
}

func (n *FieldNode) FieldType() Node {
	return n.FldType
}

func (n *FieldNode) FieldTag() Node {
	if n.Tag == nil {
		return n
	}
	return n.Tag
}

func (n *FieldNode) FieldExtendee() Node {
	if n.Extendee != nil {
		return n.Extendee.Extendee
	}
	return nil
}

func (n *FieldNode) GetGroupKeyword() Node {
	return nil
}

func (n *FieldNode) GetOptions() *CompactOptionsNode {
	return n.Options
}

func (n *FieldNode) RangeOptions(fn func(*OptionNode) bool) {
	for _, opt := range n.Options.Options {
		if !fn(opt) {
			return
		}
	}
}

// FieldLabel represents the label of a field, which indicates its cardinality
// (i.e. whether it is optional, required, or repeated).
type FieldLabel struct {
	*KeywordNode
	Repeated bool
	Required bool
}

func newFieldLabel(lbl *KeywordNode) FieldLabel {
	repeated, required := false, false
	if lbl != nil {
		repeated = lbl.Val == "repeated"
		required = lbl.Val == "required"
	}
	return FieldLabel{
		KeywordNode: lbl,
		Repeated:    repeated,
		Required:    required,
	}
}

// IsPresent returns true if a label keyword was present in the declaration
// and false if it was absent.
func (f *FieldLabel) IsPresent() bool {
	return f.KeywordNode != nil
}

// GroupNode represents a group declaration, which doubles as a field and inline
// message declaration. It can represent extension fields as well as
// non-extension fields (both inside of messages and inside of one-ofs).
// Example:
//
//	optional group Key = 4 {
//	  optional uint64 id = 1;
//	  optional string name = 2;
//	}
type GroupNode struct {
	compositeNode
	Label   FieldLabel
	Keyword *KeywordNode
	Name    *IdentNode
	Equals  *RuneNode
	Tag     *UintLiteralNode
	Options *CompactOptionsNode
	MessageBody

	// This is an up-link to the containing *ExtendNode for groups
	// that are defined inside of "extend" blocks.
	Extendee *ExtendNode
}

func (*GroupNode) msgElement()    {}
func (*GroupNode) oneofElement()  {}
func (*GroupNode) extendElement() {}

// NewGroupNode creates a new *GroupNode. The label and options arguments may be
// nil but the others must be non-nil.
//   - label: The token corresponding to the label keyword if present ("optional",
//     "required", or "repeated").
//   - keyword: The token corresponding to the "group" keyword.
//   - name: The token corresponding to the field's name.
//   - equals: The token corresponding to the '=' rune after the name.
//   - tag: The token corresponding to the field's tag number.
//   - opts: Optional set of field options.
//   - openBrace: The token corresponding to the "{" rune that starts the body.
//   - decls: All declarations inside the group body.
//   - closeBrace: The token corresponding to the "}" rune that ends the body.
func NewGroupNode(label *KeywordNode, keyword *KeywordNode, name *IdentNode, equals *RuneNode, tag *UintLiteralNode, opts *CompactOptionsNode, openBrace *RuneNode, decls []MessageElement, closeBrace *RuneNode) *GroupNode {
	if keyword == nil {
		panic("fieldType is nil")
	}
	if name == nil {
		panic("name is nil")
	}
	if openBrace == nil {
		panic("openBrace is nil")
	}
	if closeBrace == nil {
		panic("closeBrace is nil")
	}
	numChildren := 4 + len(decls)
	if label != nil {
		numChildren++
	}
	if equals != nil {
		numChildren++
	}
	if tag != nil {
		numChildren++
	}
	if opts != nil {
		numChildren++
	}
	children := make([]Node, 0, numChildren)
	if label != nil {
		children = append(children, label)
	}
	children = append(children, keyword, name)
	if equals != nil {
		children = append(children, equals)
	}
	if tag != nil {
		children = append(children, tag)
	}
	if opts != nil {
		children = append(children, opts)
	}
	children = append(children, openBrace)
	for _, decl := range decls {
		children = append(children, decl)
	}
	children = append(children, closeBrace)

	ret := &GroupNode{
		compositeNode: compositeNode{
			children: children,
		},
		Label:   newFieldLabel(label),
		Keyword: keyword,
		Name:    name,
		Equals:  equals,
		Tag:     tag,
		Options: opts,
	}
	populateMessageBody(&ret.MessageBody, openBrace, decls, closeBrace)
	return ret
}

func (n *GroupNode) FieldLabel() Node {
	if n.Label.KeywordNode == nil {
		// return nil interface to indicate absence, not a typed nil
		return nil
	}
	return n.Label.KeywordNode
}

func (n *GroupNode) FieldName() Node {
	return n.Name
}

func (n *GroupNode) FieldType() Node {
	return n.Keyword
}

func (n *GroupNode) FieldTag() Node {
	if n.Tag == nil {
		return n
	}
	return n.Tag
}

func (n *GroupNode) FieldExtendee() Node {
	if n.Extendee != nil {
		return n.Extendee.Extendee
	}
	return nil
}

func (n *GroupNode) GetGroupKeyword() Node {
	return n.Keyword
}

func (n *GroupNode) GetOptions() *CompactOptionsNode {
	return n.Options
}

func (n *GroupNode) RangeOptions(fn func(*OptionNode) bool) {
	for _, opt := range n.Options.Options {
		if !fn(opt) {
			return
		}
	}
}

func (n *GroupNode) AsMessage() *SyntheticGroupMessageNode {
	return (*SyntheticGroupMessageNode)(n)
}

// SyntheticGroupMessageNode is a view of a GroupNode that implements MessageDeclNode.
// Since a group field implicitly defines a message type, this node represents
// that message type while the corresponding GroupNode represents the field.
//
// This type is considered synthetic since it never appears in a file's AST, but
// is only returned from other accessors (e.g. GroupNode.AsMessage).
type SyntheticGroupMessageNode GroupNode

func (n *SyntheticGroupMessageNode) MessageName() Node {
	return n.Name
}

func (n *SyntheticGroupMessageNode) RangeOptions(fn func(*OptionNode) bool) {
	for _, decl := range n.Decls {
		if opt, ok := decl.(*OptionNode); ok {
			if !fn(opt) {
				return
			}
		}
	}
}

// OneofDeclNode is a node in the AST that defines a oneof. There are
// multiple types of AST nodes that declare oneofs:
//   - *OneofNode
//   - *SyntheticOneof
//
// This also allows NoSourceNode to be used in place of one of the above
// for some usages.
type OneofDeclNode interface {
	NodeWithOptions
	OneofName() Node
}

var _ OneofDeclNode = (*OneofNode)(nil)
var _ OneofDeclNode = (*SyntheticOneof)(nil)
var _ OneofDeclNode = NoSourceNode{}

// OneofNode represents a one-of declaration. Example:
//
//	oneof query {
//	  string by_name = 2;
//	  Type by_type = 3;
//	  Address by_address = 4;
//	  Labels by_label = 5;
//	}
type OneofNode struct {
	compositeNode
	Keyword    *KeywordNode
	Name       *IdentNode
	OpenBrace  *RuneNode
	Decls      []OneofElement
	CloseBrace *RuneNode
}

func (*OneofNode) msgElement() {}

// NewOneofNode creates a new *OneofNode. All arguments must be non-nil. While
// it is technically allowed for decls to be nil or empty, the resulting node
// will not be a valid oneof, which must have at least one field.
//   - keyword: The token corresponding to the "oneof" keyword.
//   - name: The token corresponding to the oneof's name.
//   - openBrace: The token corresponding to the "{" rune that starts the body.
//   - decls: All declarations inside the oneof body.
//   - closeBrace: The token corresponding to the "}" rune that ends the body.
func NewOneofNode(keyword *KeywordNode, name *IdentNode, openBrace *RuneNode, decls []OneofElement, closeBrace *RuneNode) *OneofNode {
	if keyword == nil {
		panic("keyword is nil")
	}
	if name == nil {
		panic("name is nil")
	}
	if openBrace == nil {
		panic("openBrace is nil")
	}
	if closeBrace == nil {
		panic("closeBrace is nil")
	}
	children := make([]Node, 0, 4+len(decls))
	children = append(children, keyword, name, openBrace)
	for _, decl := range decls {
		children = append(children, decl)
	}
	children = append(children, closeBrace)

	for _, decl := range decls {
		switch decl := decl.(type) {
		case *OptionNode, *FieldNode, *GroupNode, *EmptyDeclNode:
		default:
			panic(fmt.Sprintf("invalid OneofElement type: %T", decl))
		}
	}

	return &OneofNode{
		compositeNode: compositeNode{
			children: children,
		},
		Keyword:    keyword,
		Name:       name,
		OpenBrace:  openBrace,
		Decls:      decls,
		CloseBrace: closeBrace,
	}
}

func (n *OneofNode) OneofName() Node {
	return n.Name
}

func (n *OneofNode) RangeOptions(fn func(*OptionNode) bool) {
	for _, decl := range n.Decls {
		if opt, ok := decl.(*OptionNode); ok {
			if !fn(opt) {
				return
			}
		}
	}
}

// OneofElement is an interface implemented by all AST nodes that can
// appear in the body of a oneof declaration.
type OneofElement interface {
	Node
	oneofElement()
}

var _ OneofElement = (*OptionNode)(nil)
var _ OneofElement = (*FieldNode)(nil)
var _ OneofElement = (*GroupNode)(nil)
var _ OneofElement = (*EmptyDeclNode)(nil)

// SyntheticOneof is not an actual node in the AST but a synthetic node
// that represents the oneof implied by a proto3 optional field.
//
// This type is considered synthetic since it never appears in a file's AST,
// but is only returned from other functions (e.g. NewSyntheticOneof).
type SyntheticOneof struct {
	// The proto3 optional field that implies the presence of this oneof.
	Field *FieldNode
}

var _ Node = (*SyntheticOneof)(nil)

// NewSyntheticOneof creates a new *SyntheticOneof that corresponds to the
// given proto3 optional field.
func NewSyntheticOneof(field *FieldNode) *SyntheticOneof {
	return &SyntheticOneof{Field: field}
}

func (n *SyntheticOneof) Start() Token {
	return n.Field.Start()
}

func (n *SyntheticOneof) End() Token {
	return n.Field.End()
}

func (n *SyntheticOneof) LeadingComments() []Comment {
	return nil
}

func (n *SyntheticOneof) TrailingComments() []Comment {
	return nil
}

func (n *SyntheticOneof) OneofName() Node {
	return n.Field.FieldName()
}

func (n *SyntheticOneof) RangeOptions(_ func(*OptionNode) bool) {
}

// MapTypeNode represents the type declaration for a map field. It defines
// both the key and value types for the map. Example:
//
//	map<string, Values>
type MapTypeNode struct {
	compositeNode
	Keyword    *KeywordNode
	OpenAngle  *RuneNode
	KeyType    *IdentNode
	Comma      *RuneNode
	ValueType  IdentValueNode
	CloseAngle *RuneNode
}

// NewMapTypeNode creates a new *MapTypeNode. All arguments must be non-nil.
//   - keyword: The token corresponding to the "map" keyword.
//   - openAngle: The token corresponding to the "<" rune after the keyword.
//   - keyType: The token corresponding to the key type for the map.
//   - comma: The token corresponding to the "," rune between key and value types.
//   - valType: The token corresponding to the value type for the map.
//   - closeAngle: The token corresponding to the ">" rune that ends the declaration.
func NewMapTypeNode(keyword *KeywordNode, openAngle *RuneNode, keyType *IdentNode, comma *RuneNode, valType IdentValueNode, closeAngle *RuneNode) *MapTypeNode {
	if keyword == nil {
		panic("keyword is nil")
	}
	if openAngle == nil {
		panic("openAngle is nil")
	}
	if keyType == nil {
		panic("keyType is nil")
	}
	if comma == nil {
		panic("comma is nil")
	}
	if valType == nil {
		panic("valType is nil")
	}
	if closeAngle == nil {
		panic("closeAngle is nil")
	}
	children := []Node{keyword, openAngle, keyType, comma, valType, closeAngle}
	return &MapTypeNode{
		compositeNode: compositeNode{
			children: children,
		},
		Keyword:    keyword,
		OpenAngle:  openAngle,
		KeyType:    keyType,
		Comma:      comma,
		ValueType:  valType,
		CloseAngle: closeAngle,
	}
}

// MapFieldNode represents a map field declaration. Example:
//
//	map<string,string> replacements = 3 [deprecated = true];
type MapFieldNode struct {
	compositeNode
	MapType   *MapTypeNode
	Name      *IdentNode
	Equals    *RuneNode
	Tag       *UintLiteralNode
	Options   *CompactOptionsNode
	Semicolon *RuneNode
}

func (*MapFieldNode) msgElement() {}

// NewMapFieldNode creates a new *MapFieldNode. All arguments must be non-nil
// except opts, which may be nil.
//   - mapType: The token corresponding to the map type.
//   - name: The token corresponding to the field's name.
//   - equals: The token corresponding to the '=' rune after the name.
//   - tag: The token corresponding to the field's tag number.
//   - opts: Optional set of field options.
//   - semicolon: The token corresponding to the ";" rune that ends the declaration.
func NewMapFieldNode(mapType *MapTypeNode, name *IdentNode, equals *RuneNode, tag *UintLiteralNode, opts *CompactOptionsNode, semicolon *RuneNode) *MapFieldNode {
	if mapType == nil {
		panic("mapType is nil")
	}
	if name == nil {
		panic("name is nil")
	}
	numChildren := 2
	if equals != nil {
		numChildren++
	}
	if tag != nil {
		numChildren++
	}
	if opts != nil {
		numChildren++
	}
	if semicolon != nil {
		numChildren++
	}
	children := make([]Node, 0, numChildren)
	children = append(children, mapType, name)
	if equals != nil {
		children = append(children, equals)
	}
	if tag != nil {
		children = append(children, tag)
	}
	if opts != nil {
		children = append(children, opts)
	}
	if semicolon != nil {
		children = append(children, semicolon)
	}

	return &MapFieldNode{
		compositeNode: compositeNode{
			children: children,
		},
		MapType:   mapType,
		Name:      name,
		Equals:    equals,
		Tag:       tag,
		Options:   opts,
		Semicolon: semicolon,
	}
}

func (n *MapFieldNode) FieldLabel() Node {
	return nil
}

func (n *MapFieldNode) FieldName() Node {
	return n.Name
}

func (n *MapFieldNode) FieldType() Node {
	return n.MapType
}

func (n *MapFieldNode) FieldTag() Node {
	if n.Tag == nil {
		return n
	}
	return n.Tag
}

func (n *MapFieldNode) FieldExtendee() Node {
	return nil
}

func (n *MapFieldNode) GetGroupKeyword() Node {
	return nil
}

func (n *MapFieldNode) GetOptions() *CompactOptionsNode {
	return n.Options
}

func (n *MapFieldNode) RangeOptions(fn func(*OptionNode) bool) {
	for _, opt := range n.Options.Options {
		if !fn(opt) {
			return
		}
	}
}

func (n *MapFieldNode) AsMessage() *SyntheticMapEntryNode {
	return (*SyntheticMapEntryNode)(n)
}

func (n *MapFieldNode) KeyField() *SyntheticMapField {
	return NewSyntheticMapField(n.MapType.KeyType, 1)
}

func (n *MapFieldNode) ValueField() *SyntheticMapField {
	return NewSyntheticMapField(n.MapType.ValueType, 2)
}

// SyntheticMapEntryNode is a view of a MapFieldNode that implements MessageDeclNode.
// Since a map field implicitly defines a message type for the map entry,
// this node represents that message type.
//
// This type is considered synthetic since it never appears in a file's AST, but
// is only returned from other accessors (e.g. MapFieldNode.AsMessage).
type SyntheticMapEntryNode MapFieldNode

func (n *SyntheticMapEntryNode) MessageName() Node {
	return n.Name
}

func (n *SyntheticMapEntryNode) RangeOptions(_ func(*OptionNode) bool) {
}

// SyntheticMapField is not an actual node in the AST but a synthetic node
// that implements FieldDeclNode. These are used to represent the implicit
// field declarations of the "key" and "value" fields in a map entry.
//
// This type is considered synthetic since it never appears in a file's AST,
// but is only returned from other accessors and functions (e.g.
// MapFieldNode.KeyField, MapFieldNode.ValueField, and NewSyntheticMapField).
type SyntheticMapField struct {
	Ident IdentValueNode
	Tag   *UintLiteralNode
}

// NewSyntheticMapField creates a new *SyntheticMapField for the given
// identifier (either a key or value type in a map declaration) and tag
// number (1 for key, 2 for value).
func NewSyntheticMapField(ident IdentValueNode, tagNum uint64) *SyntheticMapField {
	tag := &UintLiteralNode{
		terminalNode: ident.Start().asTerminalNode(),
		Val:          tagNum,
	}
	return &SyntheticMapField{Ident: ident, Tag: tag}
}

func (n *SyntheticMapField) Start() Token {
	return n.Ident.Start()
}

func (n *SyntheticMapField) End() Token {
	return n.Ident.End()
}

func (n *SyntheticMapField) LeadingComments() []Comment {
	return nil
}

func (n *SyntheticMapField) TrailingComments() []Comment {
	return nil
}

func (n *SyntheticMapField) FieldLabel() Node {
	return n.Ident
}

func (n *SyntheticMapField) FieldName() Node {
	return n.Ident
}

func (n *SyntheticMapField) FieldType() Node {
	return n.Ident
}

func (n *SyntheticMapField) FieldTag() Node {
	if n.Tag == nil {
		return n
	}
	return n.Tag
}

func (n *SyntheticMapField) FieldExtendee() Node {
	return nil
}

func (n *SyntheticMapField) GetGroupKeyword() Node {
	return nil
}

func (n *SyntheticMapField) GetOptions() *CompactOptionsNode {
	return nil
}

func (n *SyntheticMapField) RangeOptions(_ func(*OptionNode) bool) {
}
