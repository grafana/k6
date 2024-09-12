%{
package parser

//lint:file-ignore SA4006 generated parser has unused values

import (
	"math"
	"strings"

	"github.com/bufbuild/protocompile/ast"
)

%}

// fields inside this union end up as the fields in a structure known
// as ${PREFIX}SymType, of which a reference is passed to the lexer.
%union{
	file         *ast.FileNode
	syn          *ast.SyntaxNode
	ed           *ast.EditionNode
	fileElements []ast.FileElement
	pkg          nodeWithRunes[*ast.PackageNode]
	imprt        nodeWithRunes[*ast.ImportNode]
	msg          nodeWithRunes[*ast.MessageNode]
	msgElements  []ast.MessageElement
	fld          *ast.FieldNode
	msgFld       nodeWithRunes[*ast.FieldNode]
	mapFld       nodeWithRunes[*ast.MapFieldNode]
	mapType      *ast.MapTypeNode
	grp          *ast.GroupNode
	msgGrp       nodeWithRunes[*ast.GroupNode]
	oo           nodeWithRunes[*ast.OneofNode]
	ooElement    ast.OneofElement
	ooElements   []ast.OneofElement
	ext          nodeWithRunes[*ast.ExtensionRangeNode]
	resvd        nodeWithRunes[*ast.ReservedNode]
	en           nodeWithRunes[*ast.EnumNode]
	enElements   []ast.EnumElement
	env          nodeWithRunes[*ast.EnumValueNode]
	extend       nodeWithRunes[*ast.ExtendNode]
	extElement   ast.ExtendElement
	extElements  []ast.ExtendElement
	svc          nodeWithRunes[*ast.ServiceNode]
	svcElements  []ast.ServiceElement
	mtd          nodeWithRunes[*ast.RPCNode]
	mtdMsgType   *ast.RPCTypeNode
	mtdElements  []ast.RPCElement
	optRaw       *ast.OptionNode
	opt          nodeWithRunes[*ast.OptionNode]
	opts         *compactOptionSlices
	refRaw       *ast.FieldReferenceNode
	ref			     nodeWithRunes[*ast.FieldReferenceNode]
	optNms       *fieldRefSlices
	cmpctOpts    *ast.CompactOptionsNode
	rng          *ast.RangeNode
	rngs         *rangeSlices
	names        *nameSlices
	cidPart      nodeWithRunes[*ast.IdentNode]
	cid          *identSlices
	tid          ast.IdentValueNode
	sl           *valueSlices
	msgLitFlds   *messageFieldList
	msgLitFld    *ast.MessageFieldNode
	v            ast.ValueNode
	il           ast.IntValueNode
	str          []*ast.StringLiteralNode
	s            *ast.StringLiteralNode
	i            *ast.UintLiteralNode
	f            *ast.FloatLiteralNode
	id           *ast.IdentNode
	b            *ast.RuneNode
	bs           []*ast.RuneNode
	err          error
}

// any non-terminal which returns a value needs a type, which is
// really a field name in the above union struct
%type <file>         file
%type <syn>          syntaxDecl
%type <ed>           editionDecl
%type <fileElements> fileBody fileElement fileElements
%type <imprt>        importDecl
%type <pkg>          packageDecl
%type <optRaw>       compactOption oneofOptionDecl
%type <opt>          optionDecl compactOptionEntry compactOptionFinal
%type <opts>         compactOptionDecls compactOptionLeadingDecls
%type <refRaw>       extensionName messageLiteralFieldName optionNamePart
%type <ref>          optionNameEntry optionNameFinal
%type <optNms>       optionName optionNameLeading
%type <cmpctOpts>    compactOptions
%type <v>            fieldValue optionValue scalarValue fieldScalarValue messageLiteralWithBraces messageLiteral numLit specialFloatLit listLiteral listElement listOfMessagesLiteral messageValue
%type <il>           enumValueNumber
%type <id>           identifier mapKeyType msgElementName extElementName oneofElementName notGroupElementName mtdElementName enumValueName fieldCardinality
%type <cidPart>      qualifiedIdentifierEntry qualifiedIdentifierFinal mtdElementIdentEntry mtdElementIdentFinal
%type <cid>          qualifiedIdentifier msgElementIdent extElementIdent oneofElementIdent notGroupElementIdent mtdElementIdent qualifiedIdentifierDot qualifiedIdentifierLeading mtdElementIdentLeading
%type <tid>          typeName msgElementTypeIdent extElementTypeIdent oneofElementTypeIdent notGroupElementTypeIdent mtdElementTypeIdent
%type <sl>           listElements messageLiterals
%type <msgLitFlds>   messageLiteralFieldEntry messageLiteralFields messageTextFormat
%type <msgLitFld>    messageLiteralField
%type <msgFld>       messageFieldDecl
%type <fld>          oneofFieldDecl extensionFieldDecl
%type <oo>           oneofDecl
%type <grp>          groupDecl oneofGroupDecl
%type <msgGrp>       messageGroupDecl
%type <mapFld>       mapFieldDecl
%type <mapType>      mapType
%type <msg>          messageDecl
%type <msgElements>  messageElement messageElements messageBody
%type <ooElement>    oneofElement
%type <ooElements>   oneofElements oneofBody
%type <names>        fieldNameStrings fieldNameIdents
%type <resvd>        msgReserved enumReserved reservedNames
%type <rng>          tagRange enumValueRange
%type <rngs>         tagRanges enumValueRanges
%type <ext>          extensionRangeDecl
%type <en>           enumDecl
%type <enElements>   enumElement enumElements enumBody
%type <env>          enumValueDecl
%type <extend>       extensionDecl
%type <extElement>   extensionElement
%type <extElements>  extensionElements extensionBody
%type <str>          stringLit
%type <svc>          serviceDecl
%type <svcElements>  serviceElement serviceElements serviceBody
%type <mtd>          methodDecl
%type <mtdElements>  methodElement methodElements methodBody
%type <mtdMsgType>   methodMessageType
%type <b>            semicolon
%type <bs>           semicolons semicolonList

// same for terminals
%token <s>   _STRING_LIT
%token <i>   _INT_LIT
%token <f>   _FLOAT_LIT
%token <id>  _NAME
%token <id>  _SYNTAX _EDITION _IMPORT _WEAK _PUBLIC _PACKAGE _OPTION _TRUE _FALSE _INF _NAN _REPEATED _OPTIONAL _REQUIRED
%token <id>  _DOUBLE _FLOAT _INT32 _INT64 _UINT32 _UINT64 _SINT32 _SINT64 _FIXED32 _FIXED64 _SFIXED32 _SFIXED64
%token <id>  _BOOL _STRING _BYTES _GROUP _ONEOF _MAP _EXTENSIONS _TO _MAX _RESERVED _ENUM _MESSAGE _EXTEND
%token <id>  _SERVICE _RPC _STREAM _RETURNS
%token <err> _ERROR
// we define all of these, even ones that aren't used, to improve error messages
// so it shows the unexpected symbol instead of showing "$unk"
%token <b>   '=' ';' ':' '{' '}' '\\' '/' '?' '.' ',' '>' '<' '+' '-' '(' ')' '[' ']' '*' '&' '^' '%' '$' '#' '@' '!' '~' '`'

%%

file : syntaxDecl {
		lex := protolex.(*protoLex)
		$$ = ast.NewFileNode(lex.info, $1, nil, lex.eof)
		lex.res = $$
	}
	| editionDecl {
		lex := protolex.(*protoLex)
		$$ = ast.NewFileNodeWithEdition(lex.info, $1, nil, lex.eof)
		lex.res = $$
	}
	| fileBody  {
		lex := protolex.(*protoLex)
		$$ = ast.NewFileNode(lex.info, nil, $1, lex.eof)
		lex.res = $$
	}
	| syntaxDecl fileBody {
		lex := protolex.(*protoLex)
		$$ = ast.NewFileNode(lex.info, $1, $2, lex.eof)
		lex.res = $$
	}
	| editionDecl fileBody {
		lex := protolex.(*protoLex)
		$$ = ast.NewFileNodeWithEdition(lex.info, $1, $2, lex.eof)
		lex.res = $$
	}
	| {
		lex := protolex.(*protoLex)
		$$ = ast.NewFileNode(lex.info, nil, nil, lex.eof)
		lex.res = $$
	}

fileBody : semicolons fileElements {
		$$ = prependRunes(toFileElement, $1, $2)
	}

fileElements : fileElements fileElement {
		$$ = append($1, $2...)
	}
	| fileElement {
		$$ = $1
	}

fileElement : importDecl {
	  $$ = toElements[ast.FileElement](toFileElement, $1.Node, $1.Runes)
	}
	| packageDecl {
	  $$ = toElements[ast.FileElement](toFileElement, $1.Node, $1.Runes)
	}
	| optionDecl {
	  $$ = toElements[ast.FileElement](toFileElement, $1.Node, $1.Runes)
	}
	| messageDecl {
	  $$ = toElements[ast.FileElement](toFileElement, $1.Node, $1.Runes)
	}
	| enumDecl {
	  $$ = toElements[ast.FileElement](toFileElement, $1.Node, $1.Runes)
	}
	| extensionDecl {
	  $$ = toElements[ast.FileElement](toFileElement, $1.Node, $1.Runes)
	}
	| serviceDecl {
	  $$ = toElements[ast.FileElement](toFileElement, $1.Node, $1.Runes)
	}
	| error {
		$$ = nil
	}

semicolonList : ';' {
		$$ = []*ast.RuneNode{$1}
	}
	| semicolonList ';' {
		$$ = append($1, $2)
	}

semicolons : semicolonList {
		$$ = $1
	}
	| {
		$$ = nil
	}

semicolon : ';' {
		$$ = $1
	} |
	{
		protolex.(*protoLex).Error("syntax error: expecting ';'")
		$$ = nil
	}

syntaxDecl : _SYNTAX '=' stringLit ';' {
		$$ = ast.NewSyntaxNode($1.ToKeyword(), $2, toStringValueNode($3), $4)
	}

editionDecl : _EDITION '=' stringLit ';' {
		$$ = ast.NewEditionNode($1.ToKeyword(), $2, toStringValueNode($3), $4)
	}

importDecl : _IMPORT stringLit semicolons {
	  semi, extra := protolex.(*protoLex).requireSemicolon($3)
		$$ = newNodeWithRunes(ast.NewImportNode($1.ToKeyword(), nil, nil, toStringValueNode($2), semi), extra...)
	}
	| _IMPORT _WEAK stringLit semicolons {
	  semi, extra := protolex.(*protoLex).requireSemicolon($4)
		$$ = newNodeWithRunes(ast.NewImportNode($1.ToKeyword(), nil, $2.ToKeyword(), toStringValueNode($3), semi), extra...)
	}
	| _IMPORT _PUBLIC stringLit semicolons {
	  semi, extra := protolex.(*protoLex).requireSemicolon($4)
		$$ = newNodeWithRunes(ast.NewImportNode($1.ToKeyword(), $2.ToKeyword(), nil, toStringValueNode($3), semi), extra...)
	}

packageDecl : _PACKAGE qualifiedIdentifier semicolons {
		semi, extra := protolex.(*protoLex).requireSemicolon($3)
		$$ = newNodeWithRunes(ast.NewPackageNode($1.ToKeyword(), $2.toIdentValueNode(nil), semi), extra...)
	}

qualifiedIdentifier : identifier {
		$$ = &identSlices{idents: []*ast.IdentNode{$1}}
	}
	| qualifiedIdentifier '.' identifier {
		$1.idents = append($1.idents, $3)
		$1.dots = append($1.dots, $2)
		$$ = $1
	}

qualifiedIdentifierDot : qualifiedIdentifierFinal {
		$$ = &identSlices{idents: []*ast.IdentNode{$1.Node}, dots: $1.Runes}
	}
	| qualifiedIdentifierLeading qualifiedIdentifierFinal {
		$1.idents = append($1.idents, $2.Node)
		$1.dots = append($1.dots, $2.Runes...)
		$$ = $1
	}

qualifiedIdentifierLeading : qualifiedIdentifierEntry {
	  $$ = &identSlices{idents: []*ast.IdentNode{$1.Node}, dots: $1.Runes}
	}
	| qualifiedIdentifierLeading qualifiedIdentifierEntry {
		$1.idents = append($1.idents, $2.Node)
		$1.dots = append($1.dots, $2.Runes...)
		$$ = $1
	}

qualifiedIdentifierFinal : identifier {
		$$ = newNodeWithRunes($1)
	}
	| qualifiedIdentifierEntry {
		protolex.(*protoLex).Error("syntax error: unexpected '.'")
		$$ = $1
	}

qualifiedIdentifierEntry : identifier '.' {
		$$ = newNodeWithRunes($1, $2)
	}

// to mimic limitations of protoc recursive-descent parser,
// we don't allowed message statement keywords as identifiers
// (or oneof statement keywords [e.g. "option"] below)

msgElementIdent : msgElementName {
		$$ = &identSlices{idents: []*ast.IdentNode{$1}}
	}
	| msgElementIdent '.' identifier {
		$1.idents = append($1.idents, $3)
		$1.dots = append($1.dots, $2)
		$$ = $1
	}

extElementIdent : extElementName {
		$$ = &identSlices{idents: []*ast.IdentNode{$1}}
	}
	| extElementIdent '.' identifier {
		$1.idents = append($1.idents, $3)
		$1.dots = append($1.dots, $2)
		$$ = $1
	}

oneofElementIdent : oneofElementName {
		$$ = &identSlices{idents: []*ast.IdentNode{$1}}
	}
	| oneofElementIdent '.' identifier {
		$1.idents = append($1.idents, $3)
		$1.dots = append($1.dots, $2)
		$$ = $1
	}

notGroupElementIdent : notGroupElementName {
		$$ = &identSlices{idents: []*ast.IdentNode{$1}}
	}
	| notGroupElementIdent '.' identifier {
		$1.idents = append($1.idents, $3)
		$1.dots = append($1.dots, $2)
		$$ = $1
	}

mtdElementIdent : mtdElementIdentFinal {
		$$ = &identSlices{idents: []*ast.IdentNode{$1.Node}, dots: $1.Runes}
	}
	| mtdElementIdentLeading mtdElementIdentFinal {
		$1.idents = append($1.idents, $2.Node)
		$1.dots = append($1.dots, $2.Runes...)
		$$ = $1
	}

mtdElementIdentLeading : mtdElementIdentEntry {
	  $$ = &identSlices{idents: []*ast.IdentNode{$1.Node}, dots: $1.Runes}
	}
	| mtdElementIdentLeading mtdElementIdentEntry {
		$1.idents = append($1.idents, $2.Node)
		$1.dots = append($1.dots, $2.Runes...)
		$$ = $1
	}

mtdElementIdentFinal : mtdElementName {
	  $$ = newNodeWithRunes($1)
  }
  | mtdElementIdentEntry {
		protolex.(*protoLex).Error("syntax error: unexpected '.'")
		$$ = $1
	}

mtdElementIdentEntry : mtdElementName '.' {
	  $$ = newNodeWithRunes($1, $2)
	}

oneofOptionDecl : _OPTION optionName '=' optionValue semicolon {
		optName := ast.NewOptionNameNode($2.refs, $2.dots)
		$$ = ast.NewOptionNode($1.ToKeyword(), optName, $3, $4, $5)
	}

optionDecl : _OPTION optionName '=' optionValue semicolons {
		optName := ast.NewOptionNameNode($2.refs, $2.dots)
		semi, extra := protolex.(*protoLex).requireSemicolon($5)
		$$ = newNodeWithRunes(ast.NewOptionNode($1.ToKeyword(), optName, $3, $4, semi), extra...)
	}

optionNamePart : identifier {
		$$ = ast.NewFieldReferenceNode($1)
	}
	| extensionName {
		$$ = $1
	}

optionNameEntry : optionNamePart '.' {
		$$ = newNodeWithRunes($1, $2)
	}

optionNameFinal : optionNamePart {
		$$ = newNodeWithRunes($1)
	}
	| optionNameEntry {
		protolex.(*protoLex).Error("syntax error: unexpected '.'")
		$$ = $1
	}

optionNameLeading : optionNameEntry {
	  $$ = &fieldRefSlices{refs: []*ast.FieldReferenceNode{$1.Node}, dots: $1.Runes}
  }
	| optionNameLeading optionNameEntry {
		$1.refs = append($1.refs, $2.Node)
		$1.dots = append($1.dots, $2.Runes...)
		$$ = $1
	}

optionName : optionNameFinal {
	  $$ = &fieldRefSlices{refs: []*ast.FieldReferenceNode{$1.Node}, dots: $1.Runes}
	}
	| optionNameLeading optionNameFinal {
		$1.refs = append($1.refs, $2.Node)
		$1.dots = append($1.dots, $2.Runes...)
		$$ = $1
	}

extensionName : '(' typeName ')' {
		$$ = ast.NewExtensionFieldReferenceNode($1, $2, $3)
	}

optionValue : scalarValue
	| messageLiteralWithBraces

scalarValue : stringLit {
		$$ = toStringValueNode($1)
	}
	| numLit
	| specialFloatLit
	| identifier {
		$$ = $1
	}

numLit : _FLOAT_LIT {
		$$ = $1
	}
	| '-' _FLOAT_LIT {
		$$ = ast.NewSignedFloatLiteralNode($1, $2)
	}
	| _INT_LIT {
		$$ = $1
	}
	| '-' _INT_LIT {
		if $2.Val > math.MaxInt64 + 1 {
			// can't represent as int so treat as float literal
			$$ = ast.NewSignedFloatLiteralNode($1, $2)
		} else {
			$$ = ast.NewNegativeIntLiteralNode($1, $2)
		}
	}

specialFloatLit : '-' _INF {
		f := ast.NewSpecialFloatLiteralNode($2.ToKeyword())
		$$ = ast.NewSignedFloatLiteralNode($1, f)
	}
	| '-' _NAN {
		f := ast.NewSpecialFloatLiteralNode($2.ToKeyword())
		$$ = ast.NewSignedFloatLiteralNode($1, f)
	}

stringLit : _STRING_LIT {
		$$ = []*ast.StringLiteralNode{$1}
	}
	| stringLit _STRING_LIT {
		$$ = append($1, $2)
	}

messageLiteralWithBraces : '{' messageTextFormat '}' {
		if $2 == nil {
			$$ = ast.NewMessageLiteralNode($1, nil, nil, $3)
		} else {
			fields, delimiters := $2.toNodes()
			$$ = ast.NewMessageLiteralNode($1, fields, delimiters, $3)
		}
	}
	| '{' '}' {
		$$ = ast.NewMessageLiteralNode($1, nil, nil, $2)
	}

messageTextFormat : messageLiteralFields

messageLiteralFields : messageLiteralFieldEntry
	| messageLiteralFieldEntry messageLiteralFields {
		if $1 != nil {
			$1.next = $2
			$$ = $1
		} else {
			$$ = $2
		}
	}

messageLiteralFieldEntry : messageLiteralField {
		if $1 != nil {
			$$ = &messageFieldList{field: $1}
		} else {
			$$ = nil
		}
	}
	| messageLiteralField ',' {
		if $1 != nil {
			$$ = &messageFieldList{field: $1, delimiter: $2}
		} else {
			$$ = nil
		}
	}
	| messageLiteralField ';' {
		if $1 != nil {
			$$ = &messageFieldList{field: $1, delimiter: $2}
		} else {
			$$ = nil
		}
	}
	| error ',' {
		$$ = nil
	}
	| error ';' {
		$$ = nil
	}
	| error {
		$$ = nil
	}

messageLiteralField : messageLiteralFieldName ':' fieldValue {
		if $1 != nil && $2 != nil {
			$$ = ast.NewMessageFieldNode($1, $2, $3)
		} else {
			$$ = nil
		}
	}
	| messageLiteralFieldName messageValue {
		if $1 != nil && $2 != nil {
			$$ = ast.NewMessageFieldNode($1, nil, $2)
		} else {
			$$ = nil
		}
	}
	| error ':' fieldValue {
		$$ = nil
	}

messageLiteralFieldName : identifier {
		$$ = ast.NewFieldReferenceNode($1)
	}
	| '[' qualifiedIdentifierDot ']' {
		$$ = ast.NewExtensionFieldReferenceNode($1, $2.toIdentValueNode(nil), $3)
	}
	| '[' qualifiedIdentifierDot '/' qualifiedIdentifierDot ']' {
		$$ = ast.NewAnyTypeReferenceNode($1, $2.toIdentValueNode(nil), $3, $4.toIdentValueNode(nil), $5)
	}
	| '[' error ']' {
		$$ = nil
	}

fieldValue : fieldScalarValue
	| messageLiteral
	| listLiteral

fieldScalarValue : stringLit {
		$$ = toStringValueNode($1)
	}
	| numLit
	| '-' identifier {
		kw := $2.ToKeyword()
		switch strings.ToLower(kw.Val) {
		case "inf", "infinity", "nan":
			// these are acceptable
		default:
			// anything else is not
			protolex.(*protoLex).Error(`only identifiers "inf", "infinity", or "nan" may appear after negative sign`)
		}
		// we'll validate the identifier later
		f := ast.NewSpecialFloatLiteralNode(kw)
		$$ = ast.NewSignedFloatLiteralNode($1, f)
	}
	| identifier {
		$$ = $1
	}

messageValue : messageLiteral
	| listOfMessagesLiteral

messageLiteral : messageLiteralWithBraces
	| '<' messageTextFormat '>' {
		if $2 == nil {
			$$ = ast.NewMessageLiteralNode($1, nil, nil, $3)
		} else {
			fields, delimiters := $2.toNodes()
			$$ = ast.NewMessageLiteralNode($1, fields, delimiters, $3)
		}
	}
	| '<' '>' {
		$$ = ast.NewMessageLiteralNode($1, nil, nil, $2)
	}

listLiteral : '[' listElements ']' {
		if $2 == nil {
			$$ = ast.NewArrayLiteralNode($1, nil, nil, $3)
		} else {
			$$ = ast.NewArrayLiteralNode($1, $2.vals, $2.commas, $3)
		}
	}
	| '[' ']' {
		$$ = ast.NewArrayLiteralNode($1, nil, nil, $2)
	}
	| '[' error ']' {
		$$ = ast.NewArrayLiteralNode($1, nil, nil, $3)
	}

listElements : listElement {
		$$ = &valueSlices{vals: []ast.ValueNode{$1}}
	}
	| listElements ',' listElement {
		$1.vals = append($1.vals, $3)
		$1.commas = append($1.commas, $2)
		$$ = $1
	}

listElement : fieldScalarValue
	| messageLiteral

listOfMessagesLiteral : '[' messageLiterals ']' {
		if $2 == nil {
			$$ = ast.NewArrayLiteralNode($1, nil, nil, $3)
		} else {
			$$ = ast.NewArrayLiteralNode($1, $2.vals, $2.commas, $3)
		}
	}
	| '[' ']' {
		$$ = ast.NewArrayLiteralNode($1, nil, nil, $2)
	}
	| '[' error ']' {
		$$ = ast.NewArrayLiteralNode($1, nil, nil, $3)
	}

messageLiterals : messageLiteral {
		$$ = &valueSlices{vals: []ast.ValueNode{$1}}
	}
	| messageLiterals ',' messageLiteral {
		$1.vals = append($1.vals, $3)
		$1.commas = append($1.commas, $2)
		$$ = $1
	}

typeName : qualifiedIdentifierDot {
		$$ = $1.toIdentValueNode(nil)
	}
	| '.' qualifiedIdentifierDot {
		$$ = $2.toIdentValueNode($1)
	}

msgElementTypeIdent : msgElementIdent {
		$$ = $1.toIdentValueNode(nil)
	}
	| '.' qualifiedIdentifier {
		$$ = $2.toIdentValueNode($1)
	}

extElementTypeIdent : extElementIdent {
		$$ = $1.toIdentValueNode(nil)
	}
	| '.' qualifiedIdentifier {
		$$ = $2.toIdentValueNode($1)
	}

oneofElementTypeIdent : oneofElementIdent {
		$$ = $1.toIdentValueNode(nil)
	}
	| '.' qualifiedIdentifier {
		$$ = $2.toIdentValueNode($1)
	}

notGroupElementTypeIdent : notGroupElementIdent {
		$$ = $1.toIdentValueNode(nil)
	}
	| '.' qualifiedIdentifier {
		$$ = $2.toIdentValueNode($1)
	}

mtdElementTypeIdent : mtdElementIdent {
		$$ = $1.toIdentValueNode(nil)
	}
	| '.' qualifiedIdentifierDot {
		$$ = $2.toIdentValueNode($1)
	}

fieldCardinality : _REQUIRED
	| _OPTIONAL
	| _REPEATED

compactOptions : '[' compactOptionDecls ']' {
		$$ = ast.NewCompactOptionsNode($1, $2.options, $2.commas, $3)
	}
	| '[' ']' {
		protolex.(*protoLex).Error("compact options must have at least one option")
		$$ = ast.NewCompactOptionsNode($1, nil, nil, $2)
	}

compactOptionDecls : compactOptionFinal {
		$$ = &compactOptionSlices{options: []*ast.OptionNode{$1.Node}, commas: $1.Runes}
	}
	| compactOptionLeadingDecls compactOptionFinal {
		$1.options = append($1.options, $2.Node)
		$1.commas = append($1.commas, $2.Runes...)
		$$ = $1
	}

compactOptionLeadingDecls : compactOptionEntry {
	  $$ = &compactOptionSlices{options: []*ast.OptionNode{$1.Node}, commas: $1.Runes}
	}
	| compactOptionLeadingDecls compactOptionEntry {
		$1.options = append($1.options, $2.Node)
		$1.commas = append($1.commas, $2.Runes...)
		$$ = $1
	}

compactOptionFinal : compactOption {
		$$ = newNodeWithRunes($1)
	}
	| compactOptionEntry {
		protolex.(*protoLex).Error("syntax error: unexpected ','")
		$$ = $1
	}

compactOptionEntry : compactOption ',' {
	  $$ = newNodeWithRunes($1, $2)
  }

compactOption : optionName '=' optionValue {
		optName := ast.NewOptionNameNode($1.refs, $1.dots)
		$$ = ast.NewCompactOptionNode(optName, $2, $3)
	}
	| optionName {
		optName := ast.NewOptionNameNode($1.refs, $1.dots)
		protolex.(*protoLex).Error("compact option must have a value")
		$$ = ast.NewCompactOptionNode(optName, nil, nil)
	}


groupDecl : fieldCardinality _GROUP identifier '=' _INT_LIT '{' messageBody '}' {
		$$ = ast.NewGroupNode($1.ToKeyword(), $2.ToKeyword(), $3, $4, $5, nil, $6, $7, $8)
	}
	| fieldCardinality _GROUP identifier '=' _INT_LIT compactOptions '{' messageBody '}' {
		$$ = ast.NewGroupNode($1.ToKeyword(), $2.ToKeyword(), $3, $4, $5, $6, $7, $8, $9)
	}

messageGroupDecl : fieldCardinality _GROUP identifier '=' _INT_LIT '{' messageBody '}' semicolons {
		$$ = newNodeWithRunes(ast.NewGroupNode($1.ToKeyword(), $2.ToKeyword(), $3, $4, $5, nil, $6, $7, $8), $9...)
	}
	| fieldCardinality _GROUP identifier '=' _INT_LIT compactOptions '{' messageBody '}' semicolons {
		$$ = newNodeWithRunes(ast.NewGroupNode($1.ToKeyword(), $2.ToKeyword(), $3, $4, $5, $6, $7, $8, $9), $10...)
	}
	| fieldCardinality _GROUP identifier '{' messageBody '}' semicolons {
		$$ = newNodeWithRunes(ast.NewGroupNode($1.ToKeyword(), $2.ToKeyword(), $3, nil, nil, nil, $4, $5, $6), $7...)
	}
	| fieldCardinality _GROUP identifier compactOptions '{' messageBody '}' semicolons {
		$$ = newNodeWithRunes(ast.NewGroupNode($1.ToKeyword(), $2.ToKeyword(), $3, nil, nil, $4, $5, $6, $7), $8...)
	}

oneofDecl : _ONEOF identifier '{' oneofBody '}' semicolons {
		$$ = newNodeWithRunes(ast.NewOneofNode($1.ToKeyword(), $2, $3, $4, $5), $6...)
	}

oneofBody : {
		$$ = nil
	}
	| oneofElements

oneofElements : oneofElements oneofElement {
		if $2 != nil {
			$$ = append($1, $2)
		} else {
			$$ = $1
		}
	}
	| oneofElement {
		if $1 != nil {
			$$ = []ast.OneofElement{$1}
		} else {
			$$ = nil
		}
	}

oneofElement : oneofOptionDecl {
		$$ = $1
	}
	| oneofFieldDecl {
		$$ = $1
	}
	| oneofGroupDecl {
		$$ = $1
	}
	| error ';' {
		$$ = nil
	}
	| error {
		$$ = nil
	}

oneofFieldDecl : oneofElementTypeIdent identifier '=' _INT_LIT semicolon {
		$$ = ast.NewFieldNode(nil, $1, $2, $3, $4, nil, $5)
	}
	| oneofElementTypeIdent identifier '=' _INT_LIT compactOptions semicolon {
		$$ = ast.NewFieldNode(nil, $1, $2, $3, $4, $5, $6)
	}
	| oneofElementTypeIdent identifier semicolon {
		$$ = ast.NewFieldNode(nil, $1, $2, nil, nil, nil, $3)
	}
	| oneofElementTypeIdent identifier compactOptions semicolon {
		$$ = ast.NewFieldNode(nil, $1, $2, nil, nil, $3, $4)
	}

oneofGroupDecl : _GROUP identifier '=' _INT_LIT '{' messageBody '}' {
		$$ = ast.NewGroupNode(nil, $1.ToKeyword(), $2, $3, $4, nil, $5, $6, $7)
	}
	| _GROUP identifier '=' _INT_LIT compactOptions '{' messageBody '}' {
		$$ = ast.NewGroupNode(nil, $1.ToKeyword(), $2, $3, $4, $5, $6, $7, $8)
	}
	| _GROUP identifier '{' messageBody '}' {
		$$ = ast.NewGroupNode(nil, $1.ToKeyword(), $2, nil, nil, nil, $3, $4, $5)
	}
	| _GROUP identifier compactOptions '{' messageBody '}' {
		$$ = ast.NewGroupNode(nil, $1.ToKeyword(), $2, nil, nil, $3, $4, $5, $6)
	}


mapFieldDecl : mapType identifier '=' _INT_LIT semicolons {
	  semi, extra := protolex.(*protoLex).requireSemicolon($5)
		$$ = newNodeWithRunes(ast.NewMapFieldNode($1, $2, $3, $4, nil, semi), extra...)
	}
	| mapType identifier '=' _INT_LIT compactOptions semicolons {
		semi, extra := protolex.(*protoLex).requireSemicolon($6)
		$$ = newNodeWithRunes(ast.NewMapFieldNode($1, $2, $3, $4, $5, semi), extra...)
	}
	| mapType identifier semicolons {
	  semi, extra := protolex.(*protoLex).requireSemicolon($3)
		$$ = newNodeWithRunes(ast.NewMapFieldNode($1, $2, nil, nil, nil, semi), extra...)
	}
	| mapType identifier compactOptions semicolons {
		semi, extra := protolex.(*protoLex).requireSemicolon($4)
		$$ = newNodeWithRunes(ast.NewMapFieldNode($1, $2, nil, nil, $3, semi), extra...)
	}

mapType : _MAP '<' mapKeyType ',' typeName '>' {
		$$ = ast.NewMapTypeNode($1.ToKeyword(), $2, $3, $4, $5, $6)
	}

mapKeyType : _INT32
	| _INT64
	| _UINT32
	| _UINT64
	| _SINT32
	| _SINT64
	| _FIXED32
	| _FIXED64
	| _SFIXED32
	| _SFIXED64
	| _BOOL
	| _STRING

extensionRangeDecl : _EXTENSIONS tagRanges ';' semicolons {
	  // TODO: Tolerate a missing semicolon here. This currnelty creates a shift/reduce conflict
		// between `extensions 1 to 10` and `extensions 1` followed by `to = 10`.
		$$ = newNodeWithRunes(ast.NewExtensionRangeNode($1.ToKeyword(), $2.ranges, $2.commas, nil, $3), $4...)
	}
	| _EXTENSIONS tagRanges compactOptions semicolons {
		semi, extra := protolex.(*protoLex).requireSemicolon($4)
		$$ = newNodeWithRunes(ast.NewExtensionRangeNode($1.ToKeyword(), $2.ranges, $2.commas, $3, semi), extra...)
	}

tagRanges : tagRange {
		$$ = &rangeSlices{ranges: []*ast.RangeNode{$1}}
	}
	| tagRanges ',' tagRange {
		$1.ranges = append($1.ranges, $3)
		$1.commas = append($1.commas, $2)
		$$ = $1
	}

tagRange : _INT_LIT {
		$$ = ast.NewRangeNode($1, nil, nil, nil)
	}
	| _INT_LIT _TO _INT_LIT {
		$$ = ast.NewRangeNode($1, $2.ToKeyword(), $3, nil)
	}
	| _INT_LIT _TO _MAX {
		$$ = ast.NewRangeNode($1, $2.ToKeyword(), nil, $3.ToKeyword())
	}

enumValueRanges : enumValueRange {
		$$ = &rangeSlices{ranges: []*ast.RangeNode{$1}}
	}
	| enumValueRanges ',' enumValueRange {
		$1.ranges = append($1.ranges, $3)
		$1.commas = append($1.commas, $2)
		$$ = $1
	}

enumValueRange : enumValueNumber {
		$$ = ast.NewRangeNode($1, nil, nil, nil)
	}
	| enumValueNumber _TO enumValueNumber {
		$$ = ast.NewRangeNode($1, $2.ToKeyword(), $3, nil)
	}
	| enumValueNumber _TO _MAX {
		$$ = ast.NewRangeNode($1, $2.ToKeyword(), nil, $3.ToKeyword())
	}

enumValueNumber : _INT_LIT {
		$$ = $1
	}
	| '-' _INT_LIT {
		$$ = ast.NewNegativeIntLiteralNode($1, $2)
	}

msgReserved : _RESERVED tagRanges ';' semicolons {
	  // TODO: Tolerate a missing semicolon here. This currnelty creates a shift/reduce conflict
		// between `reserved 1 to 10` and `reserved 1` followed by `to = 10`.
		$$ = newNodeWithRunes(ast.NewReservedRangesNode($1.ToKeyword(), $2.ranges, $2.commas, $3), $4...)
	}
	| reservedNames

enumReserved : _RESERVED enumValueRanges ';' semicolons {
	  // TODO: Tolerate a missing semicolon here. This currnelty creates a shift/reduce conflict
		// between `reserved 1 to 10` and `reserved 1` followed by `to = 10`.
		$$ = newNodeWithRunes(ast.NewReservedRangesNode($1.ToKeyword(), $2.ranges, $2.commas, $3), $4...)
	}
	| reservedNames

reservedNames : _RESERVED fieldNameStrings semicolons {
	  semi, extra := protolex.(*protoLex).requireSemicolon($3)
		$$ = newNodeWithRunes(ast.NewReservedNamesNode($1.ToKeyword(), $2.names, $2.commas, semi), extra...)
	}
	| _RESERVED fieldNameIdents semicolons {
		semi, extra := protolex.(*protoLex).requireSemicolon($3)
		$$ = newNodeWithRunes(ast.NewReservedIdentifiersNode($1.ToKeyword(), $2.idents, $2.commas, semi), extra...)
	}

fieldNameStrings : stringLit {
		$$ = &nameSlices{names: []ast.StringValueNode{toStringValueNode($1)}}
	}
	| fieldNameStrings ',' stringLit {
		$1.names = append($1.names, toStringValueNode($3))
		$1.commas = append($1.commas, $2)
		$$ = $1
	}

fieldNameIdents : identifier {
		$$ = &nameSlices{idents: []*ast.IdentNode{$1}}
	}
	| fieldNameIdents ',' identifier {
		$1.idents = append($1.idents, $3)
		$1.commas = append($1.commas, $2)
		$$ = $1
	}

enumDecl : _ENUM identifier '{' enumBody '}' semicolons {
		$$ = newNodeWithRunes(ast.NewEnumNode($1.ToKeyword(), $2, $3, $4, $5), $6...)
	}

enumBody : semicolons {
		$$ = prependRunes(toEnumElement, $1, nil)
	}
	| semicolons enumElements {
		$$ = prependRunes(toEnumElement, $1, $2)
	}

enumElements : enumElements enumElement {
		$$ = append($1, $2...)
	}
	| enumElement {
		$$ = $1
	}

enumElement : optionDecl {
		$$ = toElements[ast.EnumElement](toEnumElement, $1.Node, $1.Runes)
	}
	| enumValueDecl {
		$$ = toElements[ast.EnumElement](toEnumElement, $1.Node, $1.Runes)
	}
	| enumReserved {
		$$ = toElements[ast.EnumElement](toEnumElement, $1.Node, $1.Runes)
	}
	| error {
		$$ = nil
	}

enumValueDecl : enumValueName '=' enumValueNumber semicolons {
		semi, extra := protolex.(*protoLex).requireSemicolon($4)
		$$ = newNodeWithRunes(ast.NewEnumValueNode($1, $2, $3, nil, semi), extra...)
	}
	|  enumValueName '=' enumValueNumber compactOptions semicolons {
		semi, extra := protolex.(*protoLex).requireSemicolon($5)
		$$ = newNodeWithRunes(ast.NewEnumValueNode($1, $2, $3, $4, semi), extra...)
	}

messageDecl : _MESSAGE identifier '{' messageBody '}' semicolons {
		$$ = newNodeWithRunes(ast.NewMessageNode($1.ToKeyword(), $2, $3, $4, $5), $6...)
	}

messageBody : semicolons {
		$$ = prependRunes(toMessageElement, $1, nil)
	}
	| semicolons messageElements {
		$$ = prependRunes(toMessageElement, $1, $2)
	}

messageElements : messageElements messageElement {
		$$ = append($1, $2...)
	}
	| messageElement {
		$$ = $1
	}

messageElement : messageFieldDecl {
		$$ = toElements[ast.MessageElement](toMessageElement, $1.Node, $1.Runes)
	}
	| enumDecl {
		$$ = toElements[ast.MessageElement](toMessageElement, $1.Node, $1.Runes)
	}
	| messageDecl {
		$$ = toElements[ast.MessageElement](toMessageElement, $1.Node, $1.Runes)
	}
	| extensionDecl {
		$$ = toElements[ast.MessageElement](toMessageElement, $1.Node, $1.Runes)
	}
	| extensionRangeDecl {
		$$ = toElements[ast.MessageElement](toMessageElement, $1.Node, $1.Runes)
	}
	| messageGroupDecl {
		$$ = toElements[ast.MessageElement](toMessageElement, $1.Node, $1.Runes)
	}
	| optionDecl {
		$$ = toElements[ast.MessageElement](toMessageElement, $1.Node, $1.Runes)
	}
	| oneofDecl {
		$$ = toElements[ast.MessageElement](toMessageElement, $1.Node, $1.Runes)
	}
	| mapFieldDecl {
		$$ = toElements[ast.MessageElement](toMessageElement, $1.Node, $1.Runes)
	}
	| msgReserved {
		$$ = toElements[ast.MessageElement](toMessageElement, $1.Node, $1.Runes)
	}
	| error {
		$$ = nil
	}

messageFieldDecl : fieldCardinality notGroupElementTypeIdent identifier '=' _INT_LIT semicolons {
		semis, extra := protolex.(*protoLex).requireSemicolon($6)
		$$ = newNodeWithRunes(ast.NewFieldNode($1.ToKeyword(), $2, $3, $4, $5, nil, semis), extra...)
	}
	| fieldCardinality notGroupElementTypeIdent identifier '=' _INT_LIT compactOptions semicolons {
		semis, extra := protolex.(*protoLex).requireSemicolon($7)
		$$ = newNodeWithRunes(ast.NewFieldNode($1.ToKeyword(), $2, $3, $4, $5, $6, semis), extra...)
	}
	| msgElementTypeIdent identifier '=' _INT_LIT semicolons {
		semis, extra := protolex.(*protoLex).requireSemicolon($5)
		$$ = newNodeWithRunes(ast.NewFieldNode(nil, $1, $2, $3, $4, nil, semis), extra...)
	}
	| msgElementTypeIdent identifier '=' _INT_LIT compactOptions semicolons {
		semis, extra := protolex.(*protoLex).requireSemicolon($6)
		$$ = newNodeWithRunes(ast.NewFieldNode(nil, $1, $2, $3, $4, $5, semis), extra...)
	}
	| fieldCardinality notGroupElementTypeIdent identifier semicolons {
		semis, extra := protolex.(*protoLex).requireSemicolon($4)
		$$ = newNodeWithRunes(ast.NewFieldNode($1.ToKeyword(), $2, $3, nil, nil, nil, semis), extra...)
	}
	| fieldCardinality notGroupElementTypeIdent identifier compactOptions semicolons {
		semis, extra := protolex.(*protoLex).requireSemicolon($5)
		$$ = newNodeWithRunes(ast.NewFieldNode($1.ToKeyword(), $2, $3, nil, nil, $4, semis), extra...)
	}
	| msgElementTypeIdent identifier semicolons {
		semis, extra := protolex.(*protoLex).requireSemicolon($3)
		$$ = newNodeWithRunes(ast.NewFieldNode(nil, $1, $2, nil, nil, nil, semis), extra...)
	}
	| msgElementTypeIdent identifier compactOptions semicolons {
		semis, extra := protolex.(*protoLex).requireSemicolon($4)
		$$ = newNodeWithRunes(ast.NewFieldNode(nil, $1, $2, nil, nil, $3, semis), extra...)
	}


extensionDecl : _EXTEND typeName '{' extensionBody '}' semicolons {
		$$ = newNodeWithRunes(ast.NewExtendNode($1.ToKeyword(), $2, $3, $4, $5), $6...)
	}

extensionBody : {
		$$ = nil
	}
	| extensionElements

extensionElements : extensionElements extensionElement {
		if $2 != nil {
			$$ = append($1, $2)
		} else {
			$$ = $1
		}
	}
	| extensionElement {
		if $1 != nil {
			$$ = []ast.ExtendElement{$1}
		} else {
			$$ = nil
		}
	}

extensionElement : extensionFieldDecl {
		$$ = $1
	}
	| groupDecl {
		$$ = $1
	}
	| error ';' {
		$$ = nil
	}
	| error {
		$$ = nil
	}

extensionFieldDecl : fieldCardinality notGroupElementTypeIdent identifier '=' _INT_LIT semicolon {
		$$ = ast.NewFieldNode($1.ToKeyword(), $2, $3, $4, $5, nil, $6)
	}
	| fieldCardinality notGroupElementTypeIdent identifier '=' _INT_LIT compactOptions semicolon {
		$$ = ast.NewFieldNode($1.ToKeyword(), $2, $3, $4, $5, $6, $7)
	}
	| extElementTypeIdent identifier '=' _INT_LIT semicolon {
		$$ = ast.NewFieldNode(nil, $1, $2, $3, $4, nil, $5)
	}
	| extElementTypeIdent identifier '=' _INT_LIT compactOptions semicolon {
		$$ = ast.NewFieldNode(nil, $1, $2, $3, $4, $5, $6)
	}

serviceDecl : _SERVICE identifier '{' serviceBody '}' semicolons {
		$$ = newNodeWithRunes(ast.NewServiceNode($1.ToKeyword(), $2, $3, $4, $5), $6...)
	}

serviceBody : semicolons {
	  $$ = prependRunes(toServiceElement, $1, nil)
	}
	| semicolons serviceElements {
		$$ = prependRunes(toServiceElement, $1, $2)
	}

serviceElements : serviceElements serviceElement {
		$$ = append($1, $2...)
	}
	| serviceElement {
		$$ = $1
	}

// NB: doc suggests support for "stream" declaration, separate from "rpc", but
// it does not appear to be supported in protoc (doc is likely from grammar for
// Google-internal version of protoc, with support for streaming stubby)
serviceElement : optionDecl {
		$$ = toElements[ast.ServiceElement](toServiceElement, $1.Node, $1.Runes)
	}
	| methodDecl {
		$$ = toElements[ast.ServiceElement](toServiceElement, $1.Node, $1.Runes)
	}
	| error {
		$$ = nil
	}

methodDecl : _RPC identifier methodMessageType _RETURNS methodMessageType semicolons {
	  semi, extra := protolex.(*protoLex).requireSemicolon($6)
		$$ = newNodeWithRunes(ast.NewRPCNode($1.ToKeyword(), $2, $3, $4.ToKeyword(), $5, semi), extra...)
	}
	| _RPC identifier methodMessageType _RETURNS methodMessageType '{' methodBody '}' semicolons {
		$$ = newNodeWithRunes(ast.NewRPCNodeWithBody($1.ToKeyword(), $2, $3, $4.ToKeyword(), $5, $6, $7, $8), $9...)
	}

methodMessageType : '(' _STREAM typeName ')' {
		$$ = ast.NewRPCTypeNode($1, $2.ToKeyword(), $3, $4)
	}
	| '(' mtdElementTypeIdent ')' {
		$$ = ast.NewRPCTypeNode($1, nil, $2, $3)
	}

methodBody : semicolons {
		$$ = prependRunes(toMethodElement, $1, nil)
	}
	| semicolons methodElements {
		$$ = prependRunes(toMethodElement, $1, $2)
	}

methodElements : methodElements methodElement {
		$$ = append($1, $2...)
	}
	| methodElement {
		$$ = $1
	}

methodElement : optionDecl {
		$$ = toElements[ast.RPCElement](toMethodElement, $1.Node, $1.Runes)
	}
	| error {
		$$ = nil
	}

// excludes message, enum, oneof, extensions, reserved, extend,
//   option, group, optional, required, and repeated
msgElementName : _NAME
	| _SYNTAX
	| _EDITION
	| _IMPORT
	| _WEAK
	| _PUBLIC
	| _PACKAGE
	| _TRUE
	| _FALSE
	| _INF
	| _NAN
	| _DOUBLE
	| _FLOAT
	| _INT32
	| _INT64
	| _UINT32
	| _UINT64
	| _SINT32
	| _SINT64
	| _FIXED32
	| _FIXED64
	| _SFIXED32
	| _SFIXED64
	| _BOOL
	| _STRING
	| _BYTES
	| _MAP
	| _TO
	| _MAX
	| _SERVICE
	| _RPC
	| _STREAM
	| _RETURNS

// excludes group, optional, required, and repeated
extElementName : _NAME
	| _SYNTAX
	| _EDITION
	| _IMPORT
	| _WEAK
	| _PUBLIC
	| _PACKAGE
	| _OPTION
	| _TRUE
	| _FALSE
	| _INF
	| _NAN
	| _DOUBLE
	| _FLOAT
	| _INT32
	| _INT64
	| _UINT32
	| _UINT64
	| _SINT32
	| _SINT64
	| _FIXED32
	| _FIXED64
	| _SFIXED32
	| _SFIXED64
	| _BOOL
	| _STRING
	| _BYTES
	| _ONEOF
	| _MAP
	| _EXTENSIONS
	| _TO
	| _MAX
	| _RESERVED
	| _ENUM
	| _MESSAGE
	| _EXTEND
	| _SERVICE
	| _RPC
	| _STREAM
	| _RETURNS

// excludes reserved, option
enumValueName : _NAME
	| _SYNTAX
	| _EDITION
	| _IMPORT
	| _WEAK
	| _PUBLIC
	| _PACKAGE
	| _TRUE
	| _FALSE
	| _INF
	| _NAN
	| _REPEATED
	| _OPTIONAL
	| _REQUIRED
	| _DOUBLE
	| _FLOAT
	| _INT32
	| _INT64
	| _UINT32
	| _UINT64
	| _SINT32
	| _SINT64
	| _FIXED32
	| _FIXED64
	| _SFIXED32
	| _SFIXED64
	| _BOOL
	| _STRING
	| _BYTES
	| _GROUP
	| _ONEOF
	| _MAP
	| _EXTENSIONS
	| _TO
	| _MAX
	| _ENUM
	| _MESSAGE
	| _EXTEND
	| _SERVICE
	| _RPC
	| _STREAM
	| _RETURNS

// excludes group, option, optional, required, and repeated
oneofElementName : _NAME
	| _SYNTAX
	| _EDITION
	| _IMPORT
	| _WEAK
	| _PUBLIC
	| _PACKAGE
	| _TRUE
	| _FALSE
	| _INF
	| _NAN
	| _DOUBLE
	| _FLOAT
	| _INT32
	| _INT64
	| _UINT32
	| _UINT64
	| _SINT32
	| _SINT64
	| _FIXED32
	| _FIXED64
	| _SFIXED32
	| _SFIXED64
	| _BOOL
	| _STRING
	| _BYTES
	| _ONEOF
	| _MAP
	| _EXTENSIONS
	| _TO
	| _MAX
	| _RESERVED
	| _ENUM
	| _MESSAGE
	| _EXTEND
	| _SERVICE
	| _RPC
	| _STREAM
	| _RETURNS

// excludes group
notGroupElementName : _NAME
	| _SYNTAX
	| _EDITION
	| _IMPORT
	| _WEAK
	| _PUBLIC
	| _PACKAGE
	| _OPTION
	| _TRUE
	| _FALSE
	| _INF
	| _NAN
	| _REPEATED
	| _OPTIONAL
	| _REQUIRED
	| _DOUBLE
	| _FLOAT
	| _INT32
	| _INT64
	| _UINT32
	| _UINT64
	| _SINT32
	| _SINT64
	| _FIXED32
	| _FIXED64
	| _SFIXED32
	| _SFIXED64
	| _BOOL
	| _STRING
	| _BYTES
	| _ONEOF
	| _MAP
	| _EXTENSIONS
	| _TO
	| _MAX
	| _RESERVED
	| _ENUM
	| _MESSAGE
	| _EXTEND
	| _SERVICE
	| _RPC
	| _STREAM
	| _RETURNS

// excludes stream
mtdElementName : _NAME
	| _SYNTAX
	| _EDITION
	| _IMPORT
	| _WEAK
	| _PUBLIC
	| _PACKAGE
	| _OPTION
	| _TRUE
	| _FALSE
	| _INF
	| _NAN
	| _REPEATED
	| _OPTIONAL
	| _REQUIRED
	| _DOUBLE
	| _FLOAT
	| _INT32
	| _INT64
	| _UINT32
	| _UINT64
	| _SINT32
	| _SINT64
	| _FIXED32
	| _FIXED64
	| _SFIXED32
	| _SFIXED64
	| _BOOL
	| _STRING
	| _BYTES
	| _GROUP
	| _ONEOF
	| _MAP
	| _EXTENSIONS
	| _TO
	| _MAX
	| _RESERVED
	| _ENUM
	| _MESSAGE
	| _EXTEND
	| _SERVICE
	| _RPC
	| _RETURNS

identifier : _NAME
	| _SYNTAX
	| _EDITION
	| _IMPORT
	| _WEAK
	| _PUBLIC
	| _PACKAGE
	| _OPTION
	| _TRUE
	| _FALSE
	| _INF
	| _NAN
	| _REPEATED
	| _OPTIONAL
	| _REQUIRED
	| _DOUBLE
	| _FLOAT
	| _INT32
	| _INT64
	| _UINT32
	| _UINT64
	| _SINT32
	| _SINT64
	| _FIXED32
	| _FIXED64
	| _SFIXED32
	| _SFIXED64
	| _BOOL
	| _STRING
	| _BYTES
	| _GROUP
	| _ONEOF
	| _MAP
	| _EXTENSIONS
	| _TO
	| _MAX
	| _RESERVED
	| _ENUM
	| _MESSAGE
	| _EXTEND
	| _SERVICE
	| _RPC
	| _STREAM
	| _RETURNS

%%
