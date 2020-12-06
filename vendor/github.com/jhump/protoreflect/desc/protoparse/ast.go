package protoparse

import "fmt"

// This file defines all of the nodes in the proto AST.

// SourcePos identifies a location in a proto source file.
type SourcePos struct {
	Filename  string
	Line, Col int
	Offset    int
}

func (pos SourcePos) String() string {
	if pos.Line <= 0 || pos.Col <= 0 {
		return pos.Filename
	}
	return fmt.Sprintf("%s:%d:%d", pos.Filename, pos.Line, pos.Col)
}

func unknownPos(filename string) *SourcePos {
	return &SourcePos{Filename: filename}
}

// node is the interface implemented by all nodes in the AST
type node interface {
	start() *SourcePos
	end() *SourcePos
	leadingComments() []comment
	trailingComments() []comment
}

type terminalNode interface {
	node
	popLeadingComment() comment
	pushTrailingComment(comment)
}

var _ terminalNode = (*basicNode)(nil)
var _ terminalNode = (*stringLiteralNode)(nil)
var _ terminalNode = (*intLiteralNode)(nil)
var _ terminalNode = (*floatLiteralNode)(nil)
var _ terminalNode = (*identNode)(nil)

type fileDecl interface {
	node
	getSyntax() node
}

var _ fileDecl = (*fileNode)(nil)
var _ fileDecl = (*noSourceNode)(nil)

type optionDecl interface {
	node
	getName() node
	getValue() valueNode
}

var _ optionDecl = (*optionNode)(nil)
var _ optionDecl = (*noSourceNode)(nil)

type fieldDecl interface {
	node
	fieldLabel() node
	fieldName() node
	fieldType() node
	fieldTag() node
	fieldExtendee() node
	getGroupKeyword() node
}

var _ fieldDecl = (*fieldNode)(nil)
var _ fieldDecl = (*groupNode)(nil)
var _ fieldDecl = (*mapFieldNode)(nil)
var _ fieldDecl = (*syntheticMapField)(nil)
var _ fieldDecl = (*noSourceNode)(nil)

type rangeDecl interface {
	node
	rangeStart() node
	rangeEnd() node
}

var _ rangeDecl = (*rangeNode)(nil)
var _ rangeDecl = (*noSourceNode)(nil)

type enumValueDecl interface {
	node
	getName() node
	getNumber() node
}

var _ enumValueDecl = (*enumValueNode)(nil)
var _ enumValueDecl = (*noSourceNode)(nil)

type msgDecl interface {
	node
	messageName() node
}

var _ msgDecl = (*messageNode)(nil)
var _ msgDecl = (*groupNode)(nil)
var _ msgDecl = (*mapFieldNode)(nil)
var _ msgDecl = (*noSourceNode)(nil)

type methodDecl interface {
	node
	getInputType() node
	getOutputType() node
}

var _ methodDecl = (*methodNode)(nil)
var _ methodDecl = (*noSourceNode)(nil)

type posRange struct {
	start, end SourcePos
}

type basicNode struct {
	posRange
	leading  []comment
	trailing []comment
}

func (n *basicNode) start() *SourcePos {
	return &n.posRange.start
}

func (n *basicNode) end() *SourcePos {
	return &n.posRange.end
}

func (n *basicNode) leadingComments() []comment {
	return n.leading
}

func (n *basicNode) trailingComments() []comment {
	return n.trailing
}

func (n *basicNode) popLeadingComment() comment {
	c := n.leading[0]
	n.leading = n.leading[1:]
	return c
}

func (n *basicNode) pushTrailingComment(c comment) {
	n.trailing = append(n.trailing, c)
}

type comment struct {
	posRange
	text string
}

type basicCompositeNode struct {
	first node
	last  node
}

func (n *basicCompositeNode) start() *SourcePos {
	return n.first.start()
}

func (n *basicCompositeNode) end() *SourcePos {
	return n.last.end()
}

func (n *basicCompositeNode) leadingComments() []comment {
	return n.first.leadingComments()
}

func (n *basicCompositeNode) trailingComments() []comment {
	return n.last.trailingComments()
}

func (n *basicCompositeNode) setRange(first, last node) {
	n.first = first
	n.last = last
}

type fileNode struct {
	basicCompositeNode
	syntax *syntaxNode
	decls  []*fileElement

	// This field is populated after parsing, to make it easier to find
	// source locations by import name for constructing link errors.
	imports []*importNode
}

func (n *fileNode) getSyntax() node {
	return n.syntax
}

type fileElement struct {
	// a discriminated union: only one field will be set
	imp     *importNode
	pkg     *packageNode
	option  *optionNode
	message *messageNode
	enum    *enumNode
	extend  *extendNode
	service *serviceNode
	empty   *basicNode
}

func (n *fileElement) start() *SourcePos {
	return n.get().start()
}

func (n *fileElement) end() *SourcePos {
	return n.get().end()
}

func (n *fileElement) leadingComments() []comment {
	return n.get().leadingComments()
}

func (n *fileElement) trailingComments() []comment {
	return n.get().trailingComments()
}

func (n *fileElement) get() node {
	switch {
	case n.imp != nil:
		return n.imp
	case n.pkg != nil:
		return n.pkg
	case n.option != nil:
		return n.option
	case n.message != nil:
		return n.message
	case n.enum != nil:
		return n.enum
	case n.extend != nil:
		return n.extend
	case n.service != nil:
		return n.service
	default:
		return n.empty
	}
}

type syntaxNode struct {
	basicCompositeNode
	syntax *compoundStringNode
}

type importNode struct {
	basicCompositeNode
	name   *compoundStringNode
	public bool
	weak   bool
}

type packageNode struct {
	basicCompositeNode
	name *compoundIdentNode
}

type identifier string

type identNode struct {
	basicNode
	val string
}

func (n *identNode) value() interface{} {
	return identifier(n.val)
}

type compoundIdentNode struct {
	basicCompositeNode
	val string
}

func (n *compoundIdentNode) value() interface{} {
	return identifier(n.val)
}

type compactOptionsNode struct {
	basicCompositeNode
	decls []*optionNode
}

func (n *compactOptionsNode) Elements() []*optionNode {
	if n == nil {
		return nil
	}
	return n.decls
}

type optionNode struct {
	basicCompositeNode
	name *optionNameNode
	val  valueNode
}

func (n *optionNode) getName() node {
	return n.name
}

func (n *optionNode) getValue() valueNode {
	return n.val
}

type optionNameNode struct {
	basicCompositeNode
	parts []*optionNamePartNode
}

type optionNamePartNode struct {
	basicCompositeNode
	text        *compoundIdentNode
	offset      int
	length      int
	isExtension bool
	st, en      *SourcePos
}

func (n *optionNamePartNode) start() *SourcePos {
	if n.isExtension {
		return n.basicCompositeNode.start()
	}
	return n.st
}

func (n *optionNamePartNode) end() *SourcePos {
	if n.isExtension {
		return n.basicCompositeNode.end()
	}
	return n.en
}

func (n *optionNamePartNode) setRange(first, last node) {
	n.basicCompositeNode.setRange(first, last)
	if !n.isExtension {
		st := *first.start()
		st.Col += n.offset
		n.st = &st
		en := st
		en.Col += n.length
		n.en = &en
	}
}

type valueNode interface {
	node
	value() interface{}
}

var _ valueNode = (*identNode)(nil)
var _ valueNode = (*compoundIdentNode)(nil)
var _ valueNode = (*stringLiteralNode)(nil)
var _ valueNode = (*compoundStringNode)(nil)
var _ valueNode = (*intLiteralNode)(nil)
var _ valueNode = (*compoundIntNode)(nil)
var _ valueNode = (*compoundUintNode)(nil)
var _ valueNode = (*floatLiteralNode)(nil)
var _ valueNode = (*compoundFloatNode)(nil)
var _ valueNode = (*boolLiteralNode)(nil)
var _ valueNode = (*sliceLiteralNode)(nil)
var _ valueNode = (*aggregateLiteralNode)(nil)
var _ valueNode = (*noSourceNode)(nil)

type stringLiteralNode struct {
	basicNode
	val string
}

func (n *stringLiteralNode) value() interface{} {
	return n.val
}

type compoundStringNode struct {
	basicCompositeNode
	val string
}

func (n *compoundStringNode) value() interface{} {
	return n.val
}

type intLiteral interface {
	asInt32(min, max int32) (int32, bool)
	value() interface{}
}

type intLiteralNode struct {
	basicNode
	val uint64
}

var _ intLiteral = (*intLiteralNode)(nil)

func (n *intLiteralNode) value() interface{} {
	return n.val
}

func (n *intLiteralNode) asInt32(min, max int32) (int32, bool) {
	if (min >= 0 && n.val < uint64(min)) || n.val > uint64(max) {
		return 0, false
	}
	return int32(n.val), true
}

type compoundUintNode struct {
	basicCompositeNode
	val uint64
}

var _ intLiteral = (*compoundUintNode)(nil)

func (n *compoundUintNode) value() interface{} {
	return n.val
}

func (n *compoundUintNode) asInt32(min, max int32) (int32, bool) {
	if (min >= 0 && n.val < uint64(min)) || n.val > uint64(max) {
		return 0, false
	}
	return int32(n.val), true
}

type compoundIntNode struct {
	basicCompositeNode
	val int64
}

var _ intLiteral = (*compoundIntNode)(nil)

func (n *compoundIntNode) value() interface{} {
	return n.val
}

func (n *compoundIntNode) asInt32(min, max int32) (int32, bool) {
	if n.val < int64(min) || n.val > int64(max) {
		return 0, false
	}
	return int32(n.val), true
}

type floatLiteralNode struct {
	basicNode
	val float64
}

func (n *floatLiteralNode) value() interface{} {
	return n.val
}

type compoundFloatNode struct {
	basicCompositeNode
	val float64
}

func (n *compoundFloatNode) value() interface{} {
	return n.val
}

type boolLiteralNode struct {
	*identNode
	val bool
}

func (n *boolLiteralNode) value() interface{} {
	return n.val
}

type sliceLiteralNode struct {
	basicCompositeNode
	elements []valueNode
}

func (n *sliceLiteralNode) value() interface{} {
	return n.elements
}

type aggregateLiteralNode struct {
	basicCompositeNode
	elements []*aggregateEntryNode
}

func (n *aggregateLiteralNode) value() interface{} {
	return n.elements
}

type aggregateEntryNode struct {
	basicCompositeNode
	name *aggregateNameNode
	val  valueNode
}

type aggregateNameNode struct {
	basicCompositeNode
	name        *compoundIdentNode
	isExtension bool
}

func (a *aggregateNameNode) value() string {
	if a.isExtension {
		return "[" + a.name.val + "]"
	} else {
		return a.name.val
	}
}

type fieldNode struct {
	basicCompositeNode
	label   fieldLabel
	fldType *compoundIdentNode
	name    *identNode
	tag     *intLiteralNode
	options *compactOptionsNode

	// This field is populated after parsing, to allow lookup of extendee source
	// locations when field extendees cannot be linked. (Otherwise, this is just
	// stored as a string in the field descriptors defined inside the extend
	// block).
	extendee *extendNode
}

func (n *fieldNode) fieldLabel() node {
	// proto3 fields and fields inside one-ofs will not have a label and we need
	// this check in order to return a nil node -- otherwise we'd return a
	// non-nil node that has a nil pointer value in it :/
	if n.label.identNode == nil {
		return nil
	}
	return n.label.identNode
}

func (n *fieldNode) fieldName() node {
	return n.name
}

func (n *fieldNode) fieldType() node {
	return n.fldType
}

func (n *fieldNode) fieldTag() node {
	return n.tag
}

func (n *fieldNode) fieldExtendee() node {
	if n.extendee != nil {
		return n.extendee.extendee
	}
	return nil
}

func (n *fieldNode) getGroupKeyword() node {
	return nil
}

type fieldLabel struct {
	*identNode
	repeated bool
	required bool
}

type groupNode struct {
	basicCompositeNode
	groupKeyword *identNode
	label        fieldLabel
	name         *identNode
	tag          *intLiteralNode
	decls        []*messageElement
	options      *compactOptionsNode

	// This field is populated after parsing, to allow lookup of extendee source
	// locations when field extendees cannot be linked. (Otherwise, this is just
	// stored as a string in the field descriptors defined inside the extend
	// block).
	extendee *extendNode
}

func (n *groupNode) fieldLabel() node {
	if n.label.identNode == nil {
		// return nil interface to indicate absence, not a typed nil
		return nil
	}
	return n.label.identNode
}

func (n *groupNode) fieldName() node {
	return n.name
}

func (n *groupNode) fieldType() node {
	return n.groupKeyword
}

func (n *groupNode) fieldTag() node {
	return n.tag
}

func (n *groupNode) fieldExtendee() node {
	if n.extendee != nil {
		return n.extendee.extendee
	}
	return nil
}

func (n *groupNode) getGroupKeyword() node {
	return n.groupKeyword
}

func (n *groupNode) messageName() node {
	return n.name
}

type oneOfNode struct {
	basicCompositeNode
	name  *identNode
	decls []*oneOfElement
}

type oneOfElement struct {
	// a discriminated union: only one field will be set
	option *optionNode
	field  *fieldNode
	group  *groupNode
	empty  *basicNode
}

func (n *oneOfElement) start() *SourcePos {
	return n.get().start()
}

func (n *oneOfElement) end() *SourcePos {
	return n.get().end()
}

func (n *oneOfElement) leadingComments() []comment {
	return n.get().leadingComments()
}

func (n *oneOfElement) trailingComments() []comment {
	return n.get().trailingComments()
}

func (n *oneOfElement) get() node {
	switch {
	case n.option != nil:
		return n.option
	case n.field != nil:
		return n.field
	default:
		return n.empty
	}
}

type mapTypeNode struct {
	basicCompositeNode
	mapKeyword *identNode
	keyType    *identNode
	valueType  *compoundIdentNode
}

type mapFieldNode struct {
	basicCompositeNode
	mapType *mapTypeNode
	name    *identNode
	tag     *intLiteralNode
	options *compactOptionsNode
}

func (n *mapFieldNode) fieldLabel() node {
	return nil
}

func (n *mapFieldNode) fieldName() node {
	return n.name
}

func (n *mapFieldNode) fieldType() node {
	return n.mapType
}

func (n *mapFieldNode) fieldTag() node {
	return n.tag
}

func (n *mapFieldNode) fieldExtendee() node {
	return nil
}

func (n *mapFieldNode) getGroupKeyword() node {
	return nil
}

func (n *mapFieldNode) messageName() node {
	return n.name
}

func (n *mapFieldNode) keyField() *syntheticMapField {
	k := n.mapType.keyType
	t := &compoundIdentNode{val: k.val}
	t.setRange(k, k)
	return newSyntheticMapField(t, 1)
}

func (n *mapFieldNode) valueField() *syntheticMapField {
	return newSyntheticMapField(n.mapType.valueType, 2)
}

func newSyntheticMapField(ident *compoundIdentNode, tagNum uint64) *syntheticMapField {
	tag := &intLiteralNode{
		basicNode: basicNode{
			posRange: posRange{start: *ident.start(), end: *ident.end()},
		},
		val: tagNum,
	}
	return &syntheticMapField{ident: ident, tag: tag}
}

type syntheticMapField struct {
	ident *compoundIdentNode
	tag   *intLiteralNode
}

func (n *syntheticMapField) start() *SourcePos {
	return n.ident.start()
}

func (n *syntheticMapField) end() *SourcePos {
	return n.ident.end()
}

func (n *syntheticMapField) leadingComments() []comment {
	return nil
}

func (n *syntheticMapField) trailingComments() []comment {
	return nil
}

func (n *syntheticMapField) fieldLabel() node {
	return n.ident
}

func (n *syntheticMapField) fieldName() node {
	return n.ident
}

func (n *syntheticMapField) fieldType() node {
	return n.ident
}

func (n *syntheticMapField) fieldTag() node {
	return n.tag
}

func (n *syntheticMapField) fieldExtendee() node {
	return nil
}

func (n *syntheticMapField) getGroupKeyword() node {
	return nil
}

type extensionRangeNode struct {
	basicCompositeNode
	ranges  []*rangeNode
	options *compactOptionsNode
}

type rangeNode struct {
	basicCompositeNode
	startNode, endNode node
	endMax             bool
}

func (n *rangeNode) rangeStart() node {
	return n.startNode
}

func (n *rangeNode) rangeEnd() node {
	if n.endNode == nil {
		return n.startNode
	}
	return n.endNode
}

func (n *rangeNode) startValue() interface{} {
	return n.startNode.(intLiteral).value()
}

func (n *rangeNode) startValueAsInt32(min, max int32) (int32, bool) {
	return n.startNode.(intLiteral).asInt32(min, max)
}

func (n *rangeNode) endValue() interface{} {
	l, ok := n.endNode.(intLiteral)
	if !ok {
		return nil
	}
	return l.value()
}

func (n *rangeNode) endValueAsInt32(min, max int32) (int32, bool) {
	if n.endMax {
		return max, true
	}
	if n.endNode == nil {
		return n.startValueAsInt32(min, max)
	}
	return n.endNode.(intLiteral).asInt32(min, max)
}

type reservedNode struct {
	basicCompositeNode
	ranges []*rangeNode
	names  []*compoundStringNode
}

type enumNode struct {
	basicCompositeNode
	name  *identNode
	decls []*enumElement
}

type enumElement struct {
	// a discriminated union: only one field will be set
	option   *optionNode
	value    *enumValueNode
	reserved *reservedNode
	empty    *basicNode
}

func (n *enumElement) start() *SourcePos {
	return n.get().start()
}

func (n *enumElement) end() *SourcePos {
	return n.get().end()
}

func (n *enumElement) leadingComments() []comment {
	return n.get().leadingComments()
}

func (n *enumElement) trailingComments() []comment {
	return n.get().trailingComments()
}

func (n *enumElement) get() node {
	switch {
	case n.option != nil:
		return n.option
	case n.value != nil:
		return n.value
	default:
		return n.empty
	}
}

type enumValueNode struct {
	basicCompositeNode
	name    *identNode
	options *compactOptionsNode
	number  *compoundIntNode
}

func (n *enumValueNode) getName() node {
	return n.name
}

func (n *enumValueNode) getNumber() node {
	return n.number
}

type messageNode struct {
	basicCompositeNode
	name  *identNode
	decls []*messageElement
}

func (n *messageNode) messageName() node {
	return n.name
}

type messageElement struct {
	// a discriminated union: only one field will be set
	option         *optionNode
	field          *fieldNode
	mapField       *mapFieldNode
	oneOf          *oneOfNode
	group          *groupNode
	nested         *messageNode
	enum           *enumNode
	extend         *extendNode
	extensionRange *extensionRangeNode
	reserved       *reservedNode
	empty          *basicNode
}

func (n *messageElement) start() *SourcePos {
	return n.get().start()
}

func (n *messageElement) end() *SourcePos {
	return n.get().end()
}

func (n *messageElement) leadingComments() []comment {
	return n.get().leadingComments()
}

func (n *messageElement) trailingComments() []comment {
	return n.get().trailingComments()
}

func (n *messageElement) get() node {
	switch {
	case n.option != nil:
		return n.option
	case n.field != nil:
		return n.field
	case n.mapField != nil:
		return n.mapField
	case n.oneOf != nil:
		return n.oneOf
	case n.group != nil:
		return n.group
	case n.nested != nil:
		return n.nested
	case n.enum != nil:
		return n.enum
	case n.extend != nil:
		return n.extend
	case n.extensionRange != nil:
		return n.extensionRange
	case n.reserved != nil:
		return n.reserved
	default:
		return n.empty
	}
}

type extendNode struct {
	basicCompositeNode
	extendee *compoundIdentNode
	decls    []*extendElement
}

type extendElement struct {
	// a discriminated union: only one field will be set
	field *fieldNode
	group *groupNode
	empty *basicNode
}

func (n *extendElement) start() *SourcePos {
	return n.get().start()
}

func (n *extendElement) end() *SourcePos {
	return n.get().end()
}

func (n *extendElement) leadingComments() []comment {
	return n.get().leadingComments()
}

func (n *extendElement) trailingComments() []comment {
	return n.get().trailingComments()
}

func (n *extendElement) get() node {
	switch {
	case n.field != nil:
		return n.field
	case n.group != nil:
		return n.group
	default:
		return n.empty
	}
}

type serviceNode struct {
	basicCompositeNode
	name  *identNode
	decls []*serviceElement
}

type serviceElement struct {
	// a discriminated union: only one field will be set
	option *optionNode
	rpc    *methodNode
	empty  *basicNode
}

func (n *serviceElement) start() *SourcePos {
	return n.get().start()
}

func (n *serviceElement) end() *SourcePos {
	return n.get().end()
}

func (n *serviceElement) leadingComments() []comment {
	return n.get().leadingComments()
}

func (n *serviceElement) trailingComments() []comment {
	return n.get().trailingComments()
}

func (n *serviceElement) get() node {
	switch {
	case n.option != nil:
		return n.option
	case n.rpc != nil:
		return n.rpc
	default:
		return n.empty
	}
}

type methodNode struct {
	basicCompositeNode
	name    *identNode
	input   *rpcTypeNode
	output  *rpcTypeNode
	options []*optionNode
}

func (n *methodNode) getInputType() node {
	return n.input.msgType
}

func (n *methodNode) getOutputType() node {
	return n.output.msgType
}

type rpcTypeNode struct {
	basicCompositeNode
	msgType       *compoundIdentNode
	streamKeyword node
}

type noSourceNode struct {
	pos *SourcePos
}

func (n noSourceNode) start() *SourcePos {
	return n.pos
}

func (n noSourceNode) end() *SourcePos {
	return n.pos
}

func (n noSourceNode) leadingComments() []comment {
	return nil
}

func (n noSourceNode) trailingComments() []comment {
	return nil
}

func (n noSourceNode) getSyntax() node {
	return n
}

func (n noSourceNode) getName() node {
	return n
}

func (n noSourceNode) getValue() valueNode {
	return n
}

func (n noSourceNode) fieldLabel() node {
	return n
}

func (n noSourceNode) fieldName() node {
	return n
}

func (n noSourceNode) fieldType() node {
	return n
}

func (n noSourceNode) fieldTag() node {
	return n
}

func (n noSourceNode) fieldExtendee() node {
	return n
}

func (n noSourceNode) getGroupKeyword() node {
	return n
}

func (n noSourceNode) rangeStart() node {
	return n
}

func (n noSourceNode) rangeEnd() node {
	return n
}

func (n noSourceNode) getNumber() node {
	return n
}

func (n noSourceNode) messageName() node {
	return n
}

func (n noSourceNode) getInputType() node {
	return n
}

func (n noSourceNode) getOutputType() node {
	return n
}

func (n noSourceNode) value() interface{} {
	return nil
}
