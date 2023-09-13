%{
package parser

//lint:file-ignore SA4006 generated parser has unused values

import (
	"math"

	"github.com/bufbuild/protocompile/ast"
)

%}

// fields inside this union end up as the fields in a structure known
// as ${PREFIX}SymType, of which a reference is passed to the lexer.
%union{
	file         *ast.FileNode
	syn          *ast.SyntaxNode
	fileElement  ast.FileElement
	fileElements []ast.FileElement
	pkg          *ast.PackageNode
	imprt        *ast.ImportNode
	msg          *ast.MessageNode
	msgElement   ast.MessageElement
	msgElements  []ast.MessageElement
	fld          *ast.FieldNode
	mapFld       *ast.MapFieldNode
	mapType      *ast.MapTypeNode
	grp          *ast.GroupNode
	oo           *ast.OneofNode
	ooElement    ast.OneofElement
	ooElements   []ast.OneofElement
	ext          *ast.ExtensionRangeNode
	resvd        *ast.ReservedNode
	en           *ast.EnumNode
	enElement    ast.EnumElement
	enElements   []ast.EnumElement
	env          *ast.EnumValueNode
	extend       *ast.ExtendNode
	extElement   ast.ExtendElement
	extElements  []ast.ExtendElement
	svc          *ast.ServiceNode
	svcElement   ast.ServiceElement
	svcElements  []ast.ServiceElement
	mtd          *ast.RPCNode
	mtdMsgType   *ast.RPCTypeNode
	mtdElement   ast.RPCElement
	mtdElements  []ast.RPCElement
	opt          *ast.OptionNode
	opts         *compactOptionSlices
	ref          *ast.FieldReferenceNode
	optNms       *fieldRefSlices
	cmpctOpts    *ast.CompactOptionsNode
	rng          *ast.RangeNode
	rngs         *rangeSlices
	names        *nameSlices
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
	err          error
}

// any non-terminal which returns a value needs a type, which is
// really a field name in the above union struct
%type <file>         file
%type <syn>          syntaxDecl
%type <fileElement>  fileElement
%type <fileElements> fileElements
%type <imprt>        importDecl
%type <pkg>          packageDecl
%type <opt>          optionDecl compactOption
%type <opts>         compactOptionDecls
%type <ref>          extensionName messageLiteralFieldName
%type <optNms>       optionName
%type <cmpctOpts>    compactOptions
%type <v>            value optionValue scalarValue messageLiteralWithBraces messageLiteral numLit listLiteral listElement listOfMessagesLiteral messageValue
%type <il>           enumValueNumber
%type <id>           identifier mapKeyType msgElementName extElementName oneofElementName notGroupElementName mtdElementName enumValueName fieldCardinality
%type <cid>          qualifiedIdentifier msgElementIdent extElementIdent oneofElementIdent notGroupElementIdent mtdElementIdent
%type <tid>          typeName msgElementTypeIdent extElementTypeIdent oneofElementTypeIdent notGroupElementTypeIdent mtdElementTypeIdent
%type <sl>           listElements messageLiterals
%type <msgLitFlds>   messageLiteralFieldEntry messageLiteralFields messageTextFormat
%type <msgLitFld>    messageLiteralField
%type <fld>          messageFieldDecl oneofFieldDecl extensionFieldDecl
%type <oo>           oneofDecl
%type <grp>          groupDecl oneofGroupDecl
%type <mapFld>       mapFieldDecl
%type <mapType>      mapType
%type <msg>          messageDecl
%type <msgElement>   messageElement
%type <msgElements>  messageElements messageBody
%type <ooElement>    oneofElement
%type <ooElements>   oneofElements oneofBody
%type <names>        fieldNames
%type <resvd>        msgReserved enumReserved reservedNames
%type <rng>          tagRange enumValueRange
%type <rngs>         tagRanges enumValueRanges
%type <ext>          extensionRangeDecl
%type <en>           enumDecl
%type <enElement>    enumElement
%type <enElements>   enumElements enumBody
%type <env>          enumValueDecl
%type <extend>       extensionDecl
%type <extElement>   extensionElement
%type <extElements>  extensionElements extensionBody
%type <str>          stringLit
%type <svc>          serviceDecl
%type <svcElement>   serviceElement
%type <svcElements>  serviceElements serviceBody
%type <mtd>          methodDecl
%type <mtdElement>   methodElement
%type <mtdElements>  methodElements methodBody
%type <mtdMsgType>   methodMessageType

// same for terminals
%token <s>   _STRING_LIT
%token <i>   _INT_LIT
%token <f>   _FLOAT_LIT
%token <id>  _NAME
%token <id>  _SYNTAX _IMPORT _WEAK _PUBLIC _PACKAGE _OPTION _TRUE _FALSE _INF _NAN _REPEATED _OPTIONAL _REQUIRED
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
	| fileElements  {
		lex := protolex.(*protoLex)
		$$ = ast.NewFileNode(lex.info, nil, $1, lex.eof)
		lex.res = $$
	}
	| syntaxDecl fileElements {
		lex := protolex.(*protoLex)
		$$ = ast.NewFileNode(lex.info, $1, $2, lex.eof)
		lex.res = $$
	}
	| {
		lex := protolex.(*protoLex)
		$$ = ast.NewFileNode(lex.info, nil, nil, lex.eof)
		lex.res = $$
	}

fileElements : fileElements fileElement {
		if $2 != nil {
			$$ = append($1, $2)
		} else {
			$$ = $1
		}
	}
	| fileElement {
		if $1 != nil {
			$$ = []ast.FileElement{$1}
		} else {
			$$ = nil
		}
	}

fileElement : importDecl {
		$$ = $1
	}
	| packageDecl {
		$$ = $1
	}
	| optionDecl {
		$$ = $1
	}
	| messageDecl {
		$$ = $1
	}
	| enumDecl {
		$$ = $1
	}
	| extensionDecl {
		$$ = $1
	}
	| serviceDecl {
		$$ = $1
	}
	| ';' {
		$$ = ast.NewEmptyDeclNode($1)
	}
	| error {
		$$ = nil
	}

syntaxDecl : _SYNTAX '=' stringLit ';' {
		$$ = ast.NewSyntaxNode($1.ToKeyword(), $2, toStringValueNode($3), $4)
	}

importDecl : _IMPORT stringLit ';' {
		$$ = ast.NewImportNode($1.ToKeyword(), nil, nil, toStringValueNode($2), $3)
	}
	| _IMPORT _WEAK stringLit ';' {
		$$ = ast.NewImportNode($1.ToKeyword(), nil, $2.ToKeyword(), toStringValueNode($3), $4)
	}
	| _IMPORT _PUBLIC stringLit ';' {
		$$ = ast.NewImportNode($1.ToKeyword(), $2.ToKeyword(), nil, toStringValueNode($3), $4)
	}

packageDecl : _PACKAGE qualifiedIdentifier ';' {
		$$ = ast.NewPackageNode($1.ToKeyword(), $2.toIdentValueNode(nil), $3)
	}

qualifiedIdentifier : identifier {
		$$ = &identSlices{idents: []*ast.IdentNode{$1}}
	}
	| qualifiedIdentifier '.' identifier {
		$1.idents = append($1.idents, $3)
		$1.dots = append($1.dots, $2)
		$$ = $1
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

mtdElementIdent : mtdElementName {
		$$ = &identSlices{idents: []*ast.IdentNode{$1}}
	}
	| mtdElementIdent '.' identifier {
		$1.idents = append($1.idents, $3)
		$1.dots = append($1.dots, $2)
		$$ = $1
	}

optionDecl : _OPTION optionName '=' optionValue ';' {
		optName := ast.NewOptionNameNode($2.refs, $2.dots)
		$$ = ast.NewOptionNode($1.ToKeyword(), optName, $3, $4, $5)
	}

optionName : identifier {
		fieldReferenceNode := ast.NewFieldReferenceNode($1)
		$$ = &fieldRefSlices{refs: []*ast.FieldReferenceNode{fieldReferenceNode}}
	}
	| optionName '.' identifier {
		$1.refs = append($1.refs, ast.NewFieldReferenceNode($3))
		$1.dots = append($1.dots, $2)
		$$ = $1
	}
	| extensionName {
		$$ = &fieldRefSlices{refs: []*ast.FieldReferenceNode{$1}}
	}
	| optionName '.' extensionName {
		$1.refs = append($1.refs, $3)
		$1.dots = append($1.dots, $2)
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
	| identifier {
		$$ = $1
	}

numLit : _FLOAT_LIT {
		$$ = $1
	}
	| '-' _FLOAT_LIT {
		$$ = ast.NewSignedFloatLiteralNode($1, $2)
	}
	| '+' _FLOAT_LIT {
		$$ = ast.NewSignedFloatLiteralNode($1, $2)
	}
	| '+' _INF {
		f := ast.NewSpecialFloatLiteralNode($2.ToKeyword())
		$$ = ast.NewSignedFloatLiteralNode($1, f)
	}
	| '-' _INF {
		f := ast.NewSpecialFloatLiteralNode($2.ToKeyword())
		$$ = ast.NewSignedFloatLiteralNode($1, f)
	}
	| _INT_LIT {
		$$ = $1
	}
	| '+' _INT_LIT {
		$$ = ast.NewPositiveUintLiteralNode($1, $2)
	}
	| '-' _INT_LIT {
		if $2.Val > math.MaxInt64 + 1 {
			// can't represent as int so treat as float literal
			$$ = ast.NewSignedFloatLiteralNode($1, $2)
		} else {
			$$ = ast.NewNegativeIntLiteralNode($1, $2)
		}
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

messageLiteralField : messageLiteralFieldName ':' value {
		if $1 != nil {
			$$ = ast.NewMessageFieldNode($1, $2, $3)
		} else {
			$$ = nil
		}
	}
	| messageLiteralFieldName messageValue {
		if $1 != nil {
			$$ = ast.NewMessageFieldNode($1, nil, $2)
		} else {
			$$ = nil
		}
	}
	| error ':' value {
		$$ = nil
	}

messageLiteralFieldName : identifier {
		$$ = ast.NewFieldReferenceNode($1)
	}
	| '[' qualifiedIdentifier ']' {
		$$ = ast.NewExtensionFieldReferenceNode($1, $2.toIdentValueNode(nil), $3)
	}
	| '[' qualifiedIdentifier '/' qualifiedIdentifier ']' {
		$$ = ast.NewAnyTypeReferenceNode($1, $2.toIdentValueNode(nil), $3, $4.toIdentValueNode(nil), $5)
	}
	| '[' error ']' {
		$$ = nil
	}

value : scalarValue
	| messageLiteral
	| listLiteral

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
		$$ = nil
	}

listElements : listElement {
		$$ = &valueSlices{vals: []ast.ValueNode{$1}}
	}
	| listElements ',' listElement {
		$1.vals = append($1.vals, $3)
		$1.commas = append($1.commas, $2)
		$$ = $1
	}

listElement : scalarValue
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
		$$ = nil
	}

messageLiterals : messageLiteral {
		$$ = &valueSlices{vals: []ast.ValueNode{$1}}
	}
	| messageLiterals ',' messageLiteral {
		$1.vals = append($1.vals, $3)
		$1.commas = append($1.commas, $2)
		$$ = $1
	}

typeName : qualifiedIdentifier {
		$$ = $1.toIdentValueNode(nil)
	}
	| '.' qualifiedIdentifier {
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
	| '.' qualifiedIdentifier {
		$$ = $2.toIdentValueNode($1)
	}

fieldCardinality : _REQUIRED
	| _OPTIONAL
	| _REPEATED

compactOptions: '[' compactOptionDecls ']' {
		$$ = ast.NewCompactOptionsNode($1, $2.options, $2.commas, $3)
	}

compactOptionDecls : compactOption {
		$$ = &compactOptionSlices{options: []*ast.OptionNode{$1}}
	}
	| compactOptionDecls ',' compactOption {
		$1.options = append($1.options, $3)
		$1.commas = append($1.commas, $2)
		$$ = $1
	}

compactOption: optionName '=' optionValue {
		optName := ast.NewOptionNameNode($1.refs, $1.dots)
		$$ = ast.NewCompactOptionNode(optName, $2, $3)
	}

groupDecl : fieldCardinality _GROUP identifier '=' _INT_LIT '{' messageBody '}' {
		$$ = ast.NewGroupNode($1.ToKeyword(), $2.ToKeyword(), $3, $4, $5, nil, $6, $7, $8)
	}
	| fieldCardinality _GROUP identifier '=' _INT_LIT compactOptions '{' messageBody '}' {
		$$ = ast.NewGroupNode($1.ToKeyword(), $2.ToKeyword(), $3, $4, $5, $6, $7, $8, $9)
	}

oneofDecl : _ONEOF identifier '{' oneofBody '}' {
		$$ = ast.NewOneofNode($1.ToKeyword(), $2, $3, $4, $5)
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

oneofElement : optionDecl {
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

oneofFieldDecl : oneofElementTypeIdent identifier '=' _INT_LIT ';' {
		$$ = ast.NewFieldNode(nil, $1, $2, $3, $4, nil, $5)
	}
	| oneofElementTypeIdent identifier '=' _INT_LIT compactOptions ';' {
		$$ = ast.NewFieldNode(nil, $1, $2, $3, $4, $5, $6)
	}

oneofGroupDecl : _GROUP identifier '=' _INT_LIT '{' messageBody '}' {
		$$ = ast.NewGroupNode(nil, $1.ToKeyword(), $2, $3, $4, nil, $5, $6, $7)
	}
	| _GROUP identifier '=' _INT_LIT compactOptions '{' messageBody '}' {
		$$ = ast.NewGroupNode(nil, $1.ToKeyword(), $2, $3, $4, $5, $6, $7, $8)
	}

mapFieldDecl : mapType identifier '=' _INT_LIT ';' {
		$$ = ast.NewMapFieldNode($1, $2, $3, $4, nil, $5)
	}
	| mapType identifier '=' _INT_LIT compactOptions ';' {
		$$ = ast.NewMapFieldNode($1, $2, $3, $4, $5, $6)
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

extensionRangeDecl : _EXTENSIONS tagRanges ';' {
		$$ = ast.NewExtensionRangeNode($1.ToKeyword(), $2.ranges, $2.commas, nil, $3)
	}
	| _EXTENSIONS tagRanges compactOptions ';' {
		$$ = ast.NewExtensionRangeNode($1.ToKeyword(), $2.ranges, $2.commas, $3, $4)
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

msgReserved : _RESERVED tagRanges ';' {
		$$ = ast.NewReservedRangesNode($1.ToKeyword(), $2.ranges, $2.commas, $3)
	}
	| reservedNames

enumReserved : _RESERVED enumValueRanges ';' {
		$$ = ast.NewReservedRangesNode($1.ToKeyword(), $2.ranges, $2.commas, $3)
	}
	| reservedNames

reservedNames : _RESERVED fieldNames ';' {
		$$ = ast.NewReservedNamesNode($1.ToKeyword(), $2.names, $2.commas, $3)
	}

fieldNames : stringLit {
		$$ = &nameSlices{names: []ast.StringValueNode{toStringValueNode($1)}}
	}
	| fieldNames ',' stringLit {
		$1.names = append($1.names, toStringValueNode($3))
		$1.commas = append($1.commas, $2)
		$$ = $1
	}

enumDecl : _ENUM identifier '{' enumBody '}' {
		$$ = ast.NewEnumNode($1.ToKeyword(), $2, $3, $4, $5)
	}

enumBody : {
		$$ = nil
	}
	| enumElements

enumElements : enumElements enumElement {
		if $2 != nil {
			$$ = append($1, $2)
		} else {
			$$ = $1
		}
	}
	| enumElement {
		if $1 != nil {
			$$ = []ast.EnumElement{$1}
		} else {
			$$ = nil
		}
	}

enumElement : optionDecl {
		$$ = $1
	}
	| enumValueDecl {
		$$ = $1
	}
	| enumReserved {
		$$ = $1
	}
	| ';' {
		$$ = ast.NewEmptyDeclNode($1)
	}
	| error {
		$$ = nil
	}

enumValueDecl : enumValueName '=' enumValueNumber ';' {
		$$ = ast.NewEnumValueNode($1, $2, $3, nil, $4)
	}
	|  enumValueName '=' enumValueNumber compactOptions ';' {
		$$ = ast.NewEnumValueNode($1, $2, $3, $4, $5)
	}

messageDecl : _MESSAGE identifier '{' messageBody '}' {
		$$ = ast.NewMessageNode($1.ToKeyword(), $2, $3, $4, $5)
	}

messageBody : {
		$$ = nil
	}
	| messageElements

messageElements : messageElements messageElement {
		if $2 != nil {
			$$ = append($1, $2)
		} else {
			$$ = $1
		}
	}
	| messageElement {
		if $1 != nil {
			$$ = []ast.MessageElement{$1}
		} else {
			$$ = nil
		}
	}

messageElement : messageFieldDecl {
		$$ = $1
	}
	| enumDecl {
		$$ = $1
	}
	| messageDecl {
		$$ = $1
	}
	| extensionDecl {
		$$ = $1
	}
	| extensionRangeDecl {
		$$ = $1
	}
	| groupDecl {
		$$ = $1
	}
	| optionDecl {
		$$ = $1
	}
	| oneofDecl {
		$$ = $1
	}
	| mapFieldDecl {
		$$ = $1
	}
	| msgReserved {
		$$ = $1
	}
	| ';' {
		$$ = ast.NewEmptyDeclNode($1)
	}
	| error {
		$$ = nil
	}

messageFieldDecl : fieldCardinality notGroupElementTypeIdent identifier '=' _INT_LIT ';' {
		$$ = ast.NewFieldNode($1.ToKeyword(), $2, $3, $4, $5, nil, $6)
	}
	| fieldCardinality notGroupElementTypeIdent identifier '=' _INT_LIT compactOptions ';' {
		$$ = ast.NewFieldNode($1.ToKeyword(), $2, $3, $4, $5, $6, $7)
	}
	| msgElementTypeIdent identifier '=' _INT_LIT ';' {
		$$ = ast.NewFieldNode(nil, $1, $2, $3, $4, nil, $5)
	}
	| msgElementTypeIdent identifier '=' _INT_LIT compactOptions ';' {
		$$ = ast.NewFieldNode(nil, $1, $2, $3, $4, $5, $6)
	}

extensionDecl : _EXTEND typeName '{' extensionBody '}' {
		$$ = ast.NewExtendNode($1.ToKeyword(), $2, $3, $4, $5)
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

extensionFieldDecl : fieldCardinality notGroupElementTypeIdent identifier '=' _INT_LIT ';' {
		$$ = ast.NewFieldNode($1.ToKeyword(), $2, $3, $4, $5, nil, $6)
	}
	| fieldCardinality notGroupElementTypeIdent identifier '=' _INT_LIT compactOptions ';' {
		$$ = ast.NewFieldNode($1.ToKeyword(), $2, $3, $4, $5, $6, $7)
	}
	| extElementTypeIdent identifier '=' _INT_LIT ';' {
		$$ = ast.NewFieldNode(nil, $1, $2, $3, $4, nil, $5)
	}
	| extElementTypeIdent identifier '=' _INT_LIT compactOptions ';' {
		$$ = ast.NewFieldNode(nil, $1, $2, $3, $4, $5, $6)
	}

serviceDecl : _SERVICE identifier '{' serviceBody '}' {
		$$ = ast.NewServiceNode($1.ToKeyword(), $2, $3, $4, $5)
	}

serviceBody : {
		$$ = nil
	}
	| serviceElements

serviceElements : serviceElements serviceElement {
		if $2 != nil {
			$$ = append($1, $2)
		} else {
			$$ = $1
		}
	}
	| serviceElement {
		if $1 != nil {
			$$ = []ast.ServiceElement{$1}
		} else {
			$$ = nil
		}
	}

// NB: doc suggests support for "stream" declaration, separate from "rpc", but
// it does not appear to be supported in protoc (doc is likely from grammar for
// Google-internal version of protoc, with support for streaming stubby)
serviceElement : optionDecl {
		$$ = $1
	}
	| methodDecl {
		$$ = $1
	}
	| ';' {
		$$ = ast.NewEmptyDeclNode($1)
	}
	| error {
		$$ = nil
	}

methodDecl : _RPC identifier methodMessageType _RETURNS methodMessageType ';' {
		$$ = ast.NewRPCNode($1.ToKeyword(), $2, $3, $4.ToKeyword(), $5, $6)
	}
	| _RPC identifier methodMessageType _RETURNS methodMessageType '{' methodBody '}' {
		$$ = ast.NewRPCNodeWithBody($1.ToKeyword(), $2, $3, $4.ToKeyword(), $5, $6, $7, $8)
	}

methodMessageType : '(' _STREAM typeName ')' {
		$$ = ast.NewRPCTypeNode($1, $2.ToKeyword(), $3, $4)
	}
	| '(' mtdElementTypeIdent ')' {
		$$ = ast.NewRPCTypeNode($1, nil, $2, $3)
	}

methodBody : {
		$$ = nil
	}
	| methodElements

methodElements : methodElements methodElement {
		if $2 != nil {
			$$ = append($1, $2)
		} else {
			$$ = $1
		}
	}
	| methodElement {
		if $1 != nil {
			$$ = []ast.RPCElement{$1}
		} else {
			$$ = nil
		}
	}

methodElement : optionDecl {
		$$ = $1
	}
	| ';' {
		$$ = ast.NewEmptyDeclNode($1)
	}
	| error {
		$$ = nil
	}

// excludes message, enum, oneof, extensions, reserved, extend,
//   option, group, optional, required, and repeated
msgElementName : _NAME
	| _SYNTAX
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
