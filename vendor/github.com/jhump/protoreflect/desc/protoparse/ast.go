package protoparse

import (
	"fmt"

	"github.com/bufbuild/protocompile/ast"

	ast2 "github.com/jhump/protoreflect/desc/protoparse/ast"
)

func convertAST(file *ast.FileNode) *ast2.FileNode {
	elements := make([]ast2.FileElement, len(file.Decls))
	for i := range file.Decls {
		elements[i] = convertASTFileElement(file, file.Decls[i])
	}
	root := ast2.NewFileNode(convertASTSyntax(file, file.Syntax), elements)
	eofInfo := file.NodeInfo(file.EOF)
	root.FinalComments = convertASTComments(eofInfo.LeadingComments())
	root.FinalWhitespace = eofInfo.LeadingWhitespace()
	return root
}

func convertASTSyntax(f *ast.FileNode, s *ast.SyntaxNode) *ast2.SyntaxNode {
	return ast2.NewSyntaxNode(
		convertASTKeyword(f, s.Keyword),
		convertASTRune(f, s.Equals),
		convertASTString(f, s.Syntax),
		convertASTRune(f, s.Semicolon),
	)
}

func convertASTFileElement(f *ast.FileNode, el ast.FileElement) ast2.FileElement {
	switch el := el.(type) {
	case *ast.ImportNode:
		return convertASTImport(f, el)
	case *ast.PackageNode:
		return convertASTPackage(f, el)
	case *ast.OptionNode:
		return convertASTOption(f, el)
	case *ast.MessageNode:
		return convertASTMessage(f, el)
	case *ast.EnumNode:
		return convertASTEnum(f, el)
	case *ast.ExtendNode:
		return convertASTExtend(f, el)
	case *ast.ServiceNode:
		return convertASTService(f, el)
	case *ast.EmptyDeclNode:
		return convertASTEmpty(f, el)
	default:
		panic(fmt.Sprintf("unrecognized type of ast.FileElement: %T", el))
	}
}

func convertASTImport(f *ast.FileNode, imp *ast.ImportNode) *ast2.ImportNode {
	var public, weak *ast2.KeywordNode
	if imp.Public != nil {
		public = convertASTKeyword(f, imp.Public)
	}
	if imp.Weak != nil {
		weak = convertASTKeyword(f, imp.Weak)
	}
	return ast2.NewImportNode(
		convertASTKeyword(f, imp.Keyword),
		public, weak,
		convertASTString(f, imp.Name),
		convertASTRune(f, imp.Semicolon),
	)
}

func convertASTPackage(f *ast.FileNode, p *ast.PackageNode) *ast2.PackageNode {
	return ast2.NewPackageNode(
		convertASTKeyword(f, p.Keyword),
		convertASTIdent(f, p.Name),
		convertASTRune(f, p.Semicolon),
	)
}

func convertASTOption(f *ast.FileNode, o *ast.OptionNode) *ast2.OptionNode {
	if o.Keyword == nil {
		return ast2.NewCompactOptionNode(
			convertASTOptionName(f, o.Name),
			convertASTRune(f, o.Equals),
			convertASTValue(f, o.Val),
		)
	}
	return ast2.NewOptionNode(
		convertASTKeyword(f, o.Keyword),
		convertASTOptionName(f, o.Name),
		convertASTRune(f, o.Equals),
		convertASTValue(f, o.Val),
		convertASTRune(f, o.Semicolon),
	)
}

func convertASTOptionName(f *ast.FileNode, n *ast.OptionNameNode) *ast2.OptionNameNode {
	parts := make([]*ast2.FieldReferenceNode, len(n.Parts))
	for i := range n.Parts {
		parts[i] = convertASTFieldReference(f, n.Parts[i])
	}
	dots := make([]*ast2.RuneNode, len(n.Dots))
	for i := range n.Dots {
		dots[i] = convertASTRune(f, n.Dots[i])
	}
	return ast2.NewOptionNameNode(parts, dots)
}

func convertASTFieldReference(f *ast.FileNode, n *ast.FieldReferenceNode) *ast2.FieldReferenceNode {
	switch {
	case n.IsExtension():
		return ast2.NewExtensionFieldReferenceNode(
			convertASTRune(f, n.Open),
			convertASTIdent(f, n.Name),
			convertASTRune(f, n.Close),
		)
	case n.IsAnyTypeReference():
		return ast2.NewAnyTypeReferenceNode(
			convertASTRune(f, n.Open),
			convertASTIdent(f, n.URLPrefix),
			convertASTRune(f, n.Slash),
			convertASTIdent(f, n.Name),
			convertASTRune(f, n.Close),
		)
	default:
		return ast2.NewFieldReferenceNode(convertASTIdent(f, n.Name).(*ast2.IdentNode))
	}
}

func convertASTMessage(f *ast.FileNode, m *ast.MessageNode) *ast2.MessageNode {
	decls := make([]ast2.MessageElement, len(m.Decls))
	for i := range m.Decls {
		decls[i] = convertASTMessageElement(f, m.Decls[i])
	}
	return ast2.NewMessageNode(
		convertASTKeyword(f, m.Keyword),
		convertASTIdentToken(f, m.Name),
		convertASTRune(f, m.OpenBrace),
		decls,
		convertASTRune(f, m.CloseBrace),
	)
}

func convertASTMessageElement(f *ast.FileNode, el ast.MessageElement) ast2.MessageElement {
	switch el := el.(type) {
	case *ast.OptionNode:
		return convertASTOption(f, el)
	case *ast.FieldNode:
		return convertASTField(f, el)
	case *ast.MapFieldNode:
		return convertASTMapField(f, el)
	case *ast.OneofNode:
		return convertASTOneOf(f, el)
	case *ast.GroupNode:
		return convertASTGroup(f, el)
	case *ast.MessageNode:
		return convertASTMessage(f, el)
	case *ast.EnumNode:
		return convertASTEnum(f, el)
	case *ast.ExtendNode:
		return convertASTExtend(f, el)
	case *ast.ExtensionRangeNode:
		return convertASTExtensions(f, el)
	case *ast.ReservedNode:
		return convertASTReserved(f, el)
	case *ast.EmptyDeclNode:
		return convertASTEmpty(f, el)
	default:
		panic(fmt.Sprintf("unrecognized type of ast.MessageElement: %T", el))
	}
}

func convertASTField(f *ast.FileNode, fld *ast.FieldNode) *ast2.FieldNode {
	var lbl *ast2.KeywordNode
	if fld.Label.KeywordNode != nil {
		lbl = convertASTKeyword(f, fld.Label.KeywordNode)
	}
	var opts *ast2.CompactOptionsNode
	if fld.Options != nil {
		opts = convertASTCompactOptions(f, fld.Options)
	}
	return ast2.NewFieldNode(
		lbl,
		convertASTIdent(f, fld.FldType),
		convertASTIdentToken(f, fld.Name),
		convertASTRune(f, fld.Equals),
		convertASTUintLiteral(f, fld.Tag),
		opts,
		convertASTRune(f, fld.Semicolon),
	)
}

func convertASTMapField(f *ast.FileNode, fld *ast.MapFieldNode) *ast2.MapFieldNode {
	var opts *ast2.CompactOptionsNode
	if fld.Options != nil {
		opts = convertASTCompactOptions(f, fld.Options)
	}
	return ast2.NewMapFieldNode(
		convertASTMapFieldType(f, fld.MapType),
		convertASTIdentToken(f, fld.Name),
		convertASTRune(f, fld.Equals),
		convertASTUintLiteral(f, fld.Tag),
		opts,
		convertASTRune(f, fld.Semicolon),
	)
}

func convertASTMapFieldType(f *ast.FileNode, t *ast.MapTypeNode) *ast2.MapTypeNode {
	return ast2.NewMapTypeNode(
		convertASTKeyword(f, t.Keyword),
		convertASTRune(f, t.OpenAngle),
		convertASTIdentToken(f, t.KeyType),
		convertASTRune(f, t.Comma),
		convertASTIdent(f, t.ValueType),
		convertASTRune(f, t.CloseAngle),
	)
}

func convertASTGroup(f *ast.FileNode, g *ast.GroupNode) *ast2.GroupNode {
	var lbl *ast2.KeywordNode
	if g.Label.KeywordNode != nil {
		lbl = convertASTKeyword(f, g.Label.KeywordNode)
	}
	var opts *ast2.CompactOptionsNode
	if g.Options != nil {
		opts = convertASTCompactOptions(f, g.Options)
	}
	decls := make([]ast2.MessageElement, len(g.Decls))
	for i := range g.Decls {
		decls[i] = convertASTMessageElement(f, g.Decls[i])
	}
	return ast2.NewGroupNode(
		lbl,
		convertASTKeyword(f, g.Keyword),
		convertASTIdentToken(f, g.Name),
		convertASTRune(f, g.Equals),
		convertASTUintLiteral(f, g.Tag),
		opts,
		convertASTRune(f, g.OpenBrace),
		decls,
		convertASTRune(f, g.CloseBrace),
	)
}

func convertASTOneOf(f *ast.FileNode, oo *ast.OneofNode) *ast2.OneOfNode {
	decls := make([]ast2.OneOfElement, len(oo.Decls))
	for i := range oo.Decls {
		decls[i] = convertASTOneOfElement(f, oo.Decls[i])
	}
	return ast2.NewOneOfNode(
		convertASTKeyword(f, oo.Keyword),
		convertASTIdentToken(f, oo.Name),
		convertASTRune(f, oo.OpenBrace),
		decls,
		convertASTRune(f, oo.CloseBrace),
	)
}

func convertASTOneOfElement(f *ast.FileNode, el ast.OneofElement) ast2.OneOfElement {
	switch el := el.(type) {
	case *ast.OptionNode:
		return convertASTOption(f, el)
	case *ast.FieldNode:
		return convertASTField(f, el)
	case *ast.GroupNode:
		return convertASTGroup(f, el)
	case *ast.EmptyDeclNode:
		return convertASTEmpty(f, el)
	default:
		panic(fmt.Sprintf("unrecognized type of ast.OneOfElement: %T", el))
	}
}

func convertASTExtensions(f *ast.FileNode, e *ast.ExtensionRangeNode) *ast2.ExtensionRangeNode {
	var opts *ast2.CompactOptionsNode
	if e.Options != nil {
		opts = convertASTCompactOptions(f, e.Options)
	}
	ranges := make([]*ast2.RangeNode, len(e.Ranges))
	for i := range e.Ranges {
		ranges[i] = convertASTRange(f, e.Ranges[i])
	}
	commas := make([]*ast2.RuneNode, len(e.Commas))
	for i := range e.Commas {
		commas[i] = convertASTRune(f, e.Commas[i])
	}
	return ast2.NewExtensionRangeNode(
		convertASTKeyword(f, e.Keyword),
		ranges, commas, opts,
		convertASTRune(f, e.Semicolon),
	)
}

func convertASTReserved(f *ast.FileNode, r *ast.ReservedNode) *ast2.ReservedNode {
	ranges := make([]*ast2.RangeNode, len(r.Ranges))
	for i := range r.Ranges {
		ranges[i] = convertASTRange(f, r.Ranges[i])
	}
	commas := make([]*ast2.RuneNode, len(r.Commas))
	for i := range r.Commas {
		commas[i] = convertASTRune(f, r.Commas[i])
	}
	names := make([]ast2.StringValueNode, len(r.Names))
	for i := range r.Names {
		names[i] = convertASTString(f, r.Names[i])
	}
	if len(r.Ranges) > 0 {
		return ast2.NewReservedRangesNode(
			convertASTKeyword(f, r.Keyword),
			ranges, commas,
			convertASTRune(f, r.Semicolon),
		)
	}
	return ast2.NewReservedNamesNode(
		convertASTKeyword(f, r.Keyword),
		names, commas,
		convertASTRune(f, r.Semicolon),
	)
}

func convertASTRange(f *ast.FileNode, r *ast.RangeNode) *ast2.RangeNode {
	var to, max *ast2.KeywordNode
	var end ast2.IntValueNode
	if r.To != nil {
		to = convertASTKeyword(f, r.To)
	}
	if r.Max != nil {
		max = convertASTKeyword(f, r.Max)
	}
	if r.EndVal != nil {
		end = convertASTInt(f, r.EndVal)
	}
	return ast2.NewRangeNode(
		convertASTInt(f, r.StartVal),
		to, end, max,
	)
}

func convertASTEnum(f *ast.FileNode, e *ast.EnumNode) *ast2.EnumNode {
	decls := make([]ast2.EnumElement, len(e.Decls))
	for i := range e.Decls {
		decls[i] = convertASTEnumElement(f, e.Decls[i])
	}
	return ast2.NewEnumNode(
		convertASTKeyword(f, e.Keyword),
		convertASTIdentToken(f, e.Name),
		convertASTRune(f, e.OpenBrace),
		decls,
		convertASTRune(f, e.CloseBrace),
	)
}

func convertASTEnumElement(f *ast.FileNode, el ast.EnumElement) ast2.EnumElement {
	switch el := el.(type) {
	case *ast.OptionNode:
		return convertASTOption(f, el)
	case *ast.EnumValueNode:
		return convertASTEnumValue(f, el)
	case *ast.ReservedNode:
		return convertASTReserved(f, el)
	case *ast.EmptyDeclNode:
		return convertASTEmpty(f, el)
	default:
		panic(fmt.Sprintf("unrecognized type of ast.EnumElement: %T", el))
	}
}

func convertASTEnumValue(f *ast.FileNode, e *ast.EnumValueNode) *ast2.EnumValueNode {
	var opts *ast2.CompactOptionsNode
	if e.Options != nil {
		opts = convertASTCompactOptions(f, e.Options)
	}
	return ast2.NewEnumValueNode(
		convertASTIdentToken(f, e.Name),
		convertASTRune(f, e.Equals),
		convertASTInt(f, e.Number),
		opts,
		convertASTRune(f, e.Semicolon),
	)
}

func convertASTExtend(f *ast.FileNode, e *ast.ExtendNode) *ast2.ExtendNode {
	decls := make([]ast2.ExtendElement, len(e.Decls))
	for i := range e.Decls {
		decls[i] = convertASTExtendElement(f, e.Decls[i])
	}
	return ast2.NewExtendNode(
		convertASTKeyword(f, e.Keyword),
		convertASTIdent(f, e.Extendee),
		convertASTRune(f, e.OpenBrace),
		decls,
		convertASTRune(f, e.CloseBrace),
	)
}

func convertASTExtendElement(f *ast.FileNode, el ast.ExtendElement) ast2.ExtendElement {
	switch el := el.(type) {
	case *ast.FieldNode:
		return convertASTField(f, el)
	case *ast.GroupNode:
		return convertASTGroup(f, el)
	case *ast.EmptyDeclNode:
		return convertASTEmpty(f, el)
	default:
		panic(fmt.Sprintf("unrecognized type of ast.ExtendElement: %T", el))
	}
}

func convertASTService(f *ast.FileNode, s *ast.ServiceNode) *ast2.ServiceNode {
	decls := make([]ast2.ServiceElement, len(s.Decls))
	for i := range s.Decls {
		decls[i] = convertASTServiceElement(f, s.Decls[i])
	}
	return ast2.NewServiceNode(
		convertASTKeyword(f, s.Keyword),
		convertASTIdentToken(f, s.Name),
		convertASTRune(f, s.OpenBrace),
		decls,
		convertASTRune(f, s.CloseBrace),
	)
}

func convertASTServiceElement(f *ast.FileNode, el ast.ServiceElement) ast2.ServiceElement {
	switch el := el.(type) {
	case *ast.OptionNode:
		return convertASTOption(f, el)
	case *ast.RPCNode:
		return convertASTMethod(f, el)
	case *ast.EmptyDeclNode:
		return convertASTEmpty(f, el)
	default:
		panic(fmt.Sprintf("unrecognized type of ast.ServiceElement: %T", el))
	}
}

func convertASTMethod(f *ast.FileNode, m *ast.RPCNode) *ast2.RPCNode {
	if m.OpenBrace == nil {
		return ast2.NewRPCNode(
			convertASTKeyword(f, m.Keyword),
			convertASTIdentToken(f, m.Name),
			convertASTMethodType(f, m.Input),
			convertASTKeyword(f, m.Returns),
			convertASTMethodType(f, m.Output),
			convertASTRune(f, m.Semicolon),
		)
	}
	decls := make([]ast2.RPCElement, len(m.Decls))
	for i := range m.Decls {
		decls[i] = convertASTMethodElement(f, m.Decls[i])
	}
	return ast2.NewRPCNodeWithBody(
		convertASTKeyword(f, m.Keyword),
		convertASTIdentToken(f, m.Name),
		convertASTMethodType(f, m.Input),
		convertASTKeyword(f, m.Returns),
		convertASTMethodType(f, m.Output),
		convertASTRune(f, m.OpenBrace),
		decls,
		convertASTRune(f, m.CloseBrace),
	)
}

func convertASTMethodElement(f *ast.FileNode, el ast.RPCElement) ast2.RPCElement {
	switch el := el.(type) {
	case *ast.OptionNode:
		return convertASTOption(f, el)
	case *ast.EmptyDeclNode:
		return convertASTEmpty(f, el)
	default:
		panic(fmt.Sprintf("unrecognized type of ast.RPCElement: %T", el))
	}
}

func convertASTMethodType(f *ast.FileNode, t *ast.RPCTypeNode) *ast2.RPCTypeNode {
	var stream *ast2.KeywordNode
	if t.Stream != nil {
		stream = convertASTKeyword(f, t.Stream)
	}
	return ast2.NewRPCTypeNode(
		convertASTRune(f, t.OpenParen),
		stream,
		convertASTIdent(f, t.MessageType),
		convertASTRune(f, t.CloseParen),
	)
}

func convertASTCompactOptions(f *ast.FileNode, opts *ast.CompactOptionsNode) *ast2.CompactOptionsNode {
	elems := make([]*ast2.OptionNode, len(opts.Options))
	for i := range opts.Options {
		elems[i] = convertASTOption(f, opts.Options[i])
	}
	commas := make([]*ast2.RuneNode, len(opts.Commas))
	for i := range opts.Commas {
		commas[i] = convertASTRune(f, opts.Commas[i])
	}
	return ast2.NewCompactOptionsNode(
		convertASTRune(f, opts.OpenBracket),
		elems, commas,
		convertASTRune(f, opts.CloseBracket),
	)
}

func convertASTEmpty(f *ast.FileNode, e *ast.EmptyDeclNode) *ast2.EmptyDeclNode {
	return ast2.NewEmptyDeclNode(convertASTRune(f, e.Semicolon))
}

func convertASTValue(f *ast.FileNode, v ast.ValueNode) ast2.ValueNode {
	switch v := v.(type) {
	case *ast.IdentNode:
		return convertASTIdentToken(f, v)
	case *ast.CompoundIdentNode:
		return convertASTCompoundIdent(f, v)
	case *ast.StringLiteralNode:
		return convertASTStringLiteral(f, v)
	case *ast.CompoundStringLiteralNode:
		return convertASTCompoundStringLiteral(f, v)
	case *ast.UintLiteralNode:
		return convertASTUintLiteral(f, v)
	case *ast.NegativeIntLiteralNode:
		return convertASTNegativeIntLiteral(f, v)
	case *ast.FloatLiteralNode:
		return convertASTFloatLiteral(f, v)
	case *ast.SpecialFloatLiteralNode:
		return convertASTSpecialFloatLiteral(f, v)
	case *ast.SignedFloatLiteralNode:
		return convertASTSignedFloatLiteral(f, v)
	case *ast.ArrayLiteralNode:
		return convertASTArrayLiteral(f, v)
	case *ast.MessageLiteralNode:
		return convertASTMessageLiteral(f, v)
	default:
		panic(fmt.Sprintf("unrecognized type of ast.ValueNode: %T", v))
	}
}

func convertASTIdent(f *ast.FileNode, ident ast.IdentValueNode) ast2.IdentValueNode {
	switch ident := ident.(type) {
	case *ast.IdentNode:
		return convertASTIdentToken(f, ident)
	case *ast.CompoundIdentNode:
		return convertASTCompoundIdent(f, ident)
	default:
		panic(fmt.Sprintf("unrecognized type of ast.IdentValueNode: %T", ident))
	}
}

func convertASTIdentToken(f *ast.FileNode, ident *ast.IdentNode) *ast2.IdentNode {
	return ast2.NewIdentNode(ident.Val, convertASTTokenInfo(f, ident.Token()))
}

func convertASTCompoundIdent(f *ast.FileNode, ident *ast.CompoundIdentNode) *ast2.CompoundIdentNode {
	var leadingDot *ast2.RuneNode
	if ident.LeadingDot != nil {
		leadingDot = convertASTRune(f, ident.LeadingDot)
	}
	components := make([]*ast2.IdentNode, len(ident.Components))
	for i := range ident.Components {
		components[i] = convertASTIdentToken(f, ident.Components[i])
	}
	dots := make([]*ast2.RuneNode, len(ident.Dots))
	for i := range ident.Dots {
		dots[i] = convertASTRune(f, ident.Dots[i])
	}
	return ast2.NewCompoundIdentNode(leadingDot, components, dots)
}

func convertASTString(f *ast.FileNode, str ast.StringValueNode) ast2.StringValueNode {
	switch str := str.(type) {
	case *ast.StringLiteralNode:
		return convertASTStringLiteral(f, str)
	case *ast.CompoundStringLiteralNode:
		return convertASTCompoundStringLiteral(f, str)
	default:
		panic(fmt.Sprintf("unrecognized type of ast.StringValueNode: %T", str))
	}
}

func convertASTStringLiteral(f *ast.FileNode, str *ast.StringLiteralNode) *ast2.StringLiteralNode {
	return ast2.NewStringLiteralNode(str.Val, convertASTTokenInfo(f, str.Token()))
}

func convertASTCompoundStringLiteral(f *ast.FileNode, str *ast.CompoundStringLiteralNode) *ast2.CompoundStringLiteralNode {
	children := str.Children()
	components := make([]*ast2.StringLiteralNode, len(children))
	for i := range children {
		components[i] = convertASTStringLiteral(f, children[i].(*ast.StringLiteralNode))
	}
	return ast2.NewCompoundLiteralStringNode(components...)
}

func convertASTInt(f *ast.FileNode, n ast.IntValueNode) ast2.IntValueNode {
	switch n := n.(type) {
	case *ast.UintLiteralNode:
		return convertASTUintLiteral(f, n)
	case *ast.NegativeIntLiteralNode:
		return convertASTNegativeIntLiteral(f, n)
	default:
		panic(fmt.Sprintf("unrecognized type of ast.IntValueNode: %T", n))
	}
}

func convertASTUintLiteral(f *ast.FileNode, n *ast.UintLiteralNode) *ast2.UintLiteralNode {
	return ast2.NewUintLiteralNode(n.Val, convertASTTokenInfo(f, n.Token()))
}

func convertASTNegativeIntLiteral(f *ast.FileNode, n *ast.NegativeIntLiteralNode) *ast2.NegativeIntLiteralNode {
	return ast2.NewNegativeIntLiteralNode(convertASTRune(f, n.Minus), convertASTUintLiteral(f, n.Uint))
}

func convertASTFloat(f *ast.FileNode, n ast.FloatValueNode) ast2.FloatValueNode {
	switch n := n.(type) {
	case *ast.FloatLiteralNode:
		return convertASTFloatLiteral(f, n)
	case *ast.SpecialFloatLiteralNode:
		return convertASTSpecialFloatLiteral(f, n)
	case *ast.UintLiteralNode:
		return convertASTUintLiteral(f, n)
	default:
		panic(fmt.Sprintf("unrecognized type of ast.FloatValueNode: %T", n))
	}
}

func convertASTFloatLiteral(f *ast.FileNode, n *ast.FloatLiteralNode) *ast2.FloatLiteralNode {
	return ast2.NewFloatLiteralNode(n.Val, convertASTTokenInfo(f, n.Token()))
}

func convertASTSpecialFloatLiteral(f *ast.FileNode, n *ast.SpecialFloatLiteralNode) *ast2.SpecialFloatLiteralNode {
	return ast2.NewSpecialFloatLiteralNode(convertASTKeyword(f, n.KeywordNode))
}

func convertASTSignedFloatLiteral(f *ast.FileNode, n *ast.SignedFloatLiteralNode) *ast2.SignedFloatLiteralNode {
	return ast2.NewSignedFloatLiteralNode(convertASTRune(f, n.Sign), convertASTFloat(f, n.Float))
}

func convertASTArrayLiteral(f *ast.FileNode, ar *ast.ArrayLiteralNode) *ast2.ArrayLiteralNode {
	vals := make([]ast2.ValueNode, len(ar.Elements))
	for i := range ar.Elements {
		vals[i] = convertASTValue(f, ar.Elements[i])
	}
	commas := make([]*ast2.RuneNode, len(ar.Commas))
	for i := range ar.Commas {
		commas[i] = convertASTRune(f, ar.Commas[i])
	}
	return ast2.NewArrayLiteralNode(
		convertASTRune(f, ar.OpenBracket),
		vals, commas,
		convertASTRune(f, ar.CloseBracket),
	)
}

func convertASTMessageLiteral(f *ast.FileNode, m *ast.MessageLiteralNode) *ast2.MessageLiteralNode {
	fields := make([]*ast2.MessageFieldNode, len(m.Elements))
	for i := range m.Elements {
		fields[i] = convertASTMessageLiteralField(f, m.Elements[i])
	}
	seps := make([]*ast2.RuneNode, len(m.Seps))
	for i := range m.Seps {
		if m.Seps[i] != nil {
			seps[i] = convertASTRune(f, m.Seps[i])
		}
	}
	return ast2.NewMessageLiteralNode(
		convertASTRune(f, m.Open),
		fields, seps,
		convertASTRune(f, m.Close),
	)
}

func convertASTMessageLiteralField(f *ast.FileNode, fld *ast.MessageFieldNode) *ast2.MessageFieldNode {
	var sep *ast2.RuneNode
	if fld.Sep != nil {
		sep = convertASTRune(f, fld.Sep)
	}
	return ast2.NewMessageFieldNode(
		convertASTFieldReference(f, fld.Name),
		sep,
		convertASTValue(f, fld.Val),
	)
}

func convertASTKeyword(f *ast.FileNode, k *ast.KeywordNode) *ast2.KeywordNode {
	return ast2.NewKeywordNode(k.Val, convertASTTokenInfo(f, k.Token()))
}

func convertASTRune(f *ast.FileNode, r *ast.RuneNode) *ast2.RuneNode {
	return ast2.NewRuneNode(r.Rune, convertASTTokenInfo(f, r.Token()))
}

func convertASTTokenInfo(f *ast.FileNode, tok ast.Token) ast2.TokenInfo {
	info := f.TokenInfo(tok)
	return ast2.TokenInfo{
		PosRange: ast2.PosRange{
			Start: info.Start(),
			End:   info.End(),
		},
		RawText:           info.RawText(),
		LeadingWhitespace: info.LeadingWhitespace(),
		LeadingComments:   convertASTComments(info.LeadingComments()),
		TrailingComments:  convertASTComments(info.TrailingComments()),
	}
}

func convertASTComments(comments ast.Comments) []ast2.Comment {
	results := make([]ast2.Comment, comments.Len())
	for i := 0; i < comments.Len(); i++ {
		cmt := comments.Index(i)
		results[i] = ast2.Comment{
			PosRange: ast2.PosRange{
				Start: cmt.Start(),
				End:   cmt.End(),
			},
			LeadingWhitespace: cmt.LeadingWhitespace(),
			Text:              cmt.RawText(),
		}
	}
	return results
}
