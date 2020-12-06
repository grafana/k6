%{
package protoparse

//lint:file-ignore SA4006 generated parser has unused values

import (
	"fmt"
	"math"
)

%}

// fields inside this union end up as the fields in a structure known
// as ${PREFIX}SymType, of which a reference is passed to the lexer.
%union{
	file      *fileNode
	fileDecls []*fileElement
	syn       *syntaxNode
	pkg       *packageNode
	imprt     *importNode
	msg       *messageNode
	msgDecls  []*messageElement
	fld       *fieldNode
	mapFld    *mapFieldNode
	mapType   *mapTypeNode
	grp       *groupNode
	oo        *oneOfNode
	ooDecls   []*oneOfElement
	ext       *extensionRangeNode
	resvd     *reservedNode
	en        *enumNode
	enDecls   []*enumElement
	env       *enumValueNode
	extend    *extendNode
	extDecls  []*extendElement
	svc       *serviceNode
	svcDecls  []*serviceElement
	mtd       *methodNode
	rpcType   *rpcTypeNode
	opts      []*optionNode
	optNm     []*optionNamePartNode
	cmpctOpts *compactOptionsNode
	rngs      []*rangeNode
	names     []*compoundStringNode
	cid       *compoundIdentNode
	sl        []valueNode
	agg       []*aggregateEntryNode
	aggName   *aggregateNameNode
	v         valueNode
	il        *compoundIntNode
	str       *compoundStringNode
	s         *stringLiteralNode
	i         *intLiteralNode
	f         *floatLiteralNode
	id        *identNode
	b         *basicNode
	err       error
}

// any non-terminal which returns a value needs a type, which is
// really a field name in the above union struct
%type <file>      file
%type <syn>       syntax
%type <fileDecls> fileDecl fileDecls
%type <imprt>     import
%type <pkg>       package
%type <opts>      option compactOption compactOptionDecls rpcOption rpcOptions
%type <optNm>     optionName optionNameComponent
%type <cmpctOpts> compactOptions
%type <v>         constant scalarConstant aggregate numLit
%type <il>        intLit
%type <id>        name keyType
%type <cid>       ident typeIdent
%type <aggName>   aggName
%type <sl>        constantList
%type <agg>       aggFields aggField aggFieldEntry
%type <fld>       field oneofField
%type <oo>        oneof
%type <grp>       group oneofGroup
%type <mapFld>    mapField
%type <mapType>   mapType
%type <msg>       message
%type <msgDecls>  messageItem messageBody
%type <ooDecls>   oneofItem oneofBody
%type <names>     fieldNames
%type <resvd>     msgReserved enumReserved reservedNames
%type <rngs>      tagRange tagRanges enumRange enumRanges
%type <ext>       extensions
%type <en>        enum
%type <enDecls>   enumItem enumBody
%type <env>       enumField
%type <extend>    extend
%type <extDecls>  extendItem extendBody
%type <str>       stringLit
%type <svc>       service
%type <svcDecls>  serviceItem serviceBody
%type <mtd>       rpc
%type <rpcType>   rpcType

// same for terminals
%token <s> _STRING_LIT
%token <i>  _INT_LIT
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

file : syntax {
		$$ = &fileNode{syntax: $1}
		$$.setRange($1, $1)
		protolex.(*protoLex).res = $$
	}
	| fileDecls  {
		$$ = &fileNode{decls: $1}
		if len($1) > 0 {
			$$.setRange($1[0], $1[len($1)-1])
		}
		protolex.(*protoLex).res = $$
	}
	| syntax fileDecls {
		$$ = &fileNode{syntax: $1, decls: $2}
		var end node
		if len($2) > 0 {
			end = $2[len($2)-1]
		} else {
			end = $1
		}
		$$.setRange($1, end)
		protolex.(*protoLex).res = $$
	}
	| {
	}

fileDecls : fileDecls fileDecl {
		$$ = append($1, $2...)
	}
	| fileDecl

fileDecl : import {
		$$ = []*fileElement{{imp: $1}}
	}
	| package {
		$$ = []*fileElement{{pkg: $1}}
	}
	| option {
		$$ = []*fileElement{{option: $1[0]}}
	}
	| message {
		$$ = []*fileElement{{message: $1}}
	}
	| enum {
		$$ = []*fileElement{{enum: $1}}
	}
	| extend {
		$$ = []*fileElement{{extend: $1}}
	}
	| service {
		$$ = []*fileElement{{service: $1}}
	}
	| ';' {
		$$ = []*fileElement{{empty: $1}}
	}
	| error ';' {
	}
	| error {
	}

syntax : _SYNTAX '=' stringLit ';' {
		$$ = &syntaxNode{syntax: $3}
		$$.setRange($1, $4)
	}

import : _IMPORT stringLit ';' {
		$$ = &importNode{ name: $2 }
		$$.setRange($1, $3)
	}
	| _IMPORT _WEAK stringLit ';' {
		$$ = &importNode{ name: $3, weak: true }
		$$.setRange($1, $4)
	}
	| _IMPORT _PUBLIC stringLit ';' {
		$$ = &importNode{ name: $3, public: true }
		$$.setRange($1, $4)
	}

package : _PACKAGE ident ';' {
		$$ = &packageNode{name: $2}
		$$.setRange($1, $3)
	}

ident : name {
        $$ = &compoundIdentNode{val: $1.val}
        $$.setRange($1, $1)
    }
	| ident '.' name {
        $$ = &compoundIdentNode{val: $1.val + "." + $3.val}
        $$.setRange($1, $3)
	}

option : _OPTION optionName '=' constant ';' {
		n := &optionNameNode{parts: $2}
		n.setRange($2[0], $2[len($2)-1])
		o := &optionNode{name: n, val: $4}
		o.setRange($1, $5)
		$$ = []*optionNode{o}
	}

optionName : optionNameComponent
    |
    optionName '.' optionNameComponent {
		$$ = append($1, $3...)
	}


optionNameComponent : name {
        nm := &compoundIdentNode{val: $1.val}
        nm.setRange($1, $1)
		$$ = toNameParts(nm)
	}
	| '(' typeIdent ')' {
		p := &optionNamePartNode{text: $2, isExtension: true}
		p.setRange($1, $3)
		$$ = []*optionNamePartNode{p}
	}

constant : scalarConstant
	| aggregate

scalarConstant : stringLit {
		$$ = $1
	}
	| numLit
	| name {
		if $1.val == "true" {
			$$ = &boolLiteralNode{identNode: $1, val: true}
		} else if $1.val == "false" {
			$$ = &boolLiteralNode{identNode: $1, val: false}
		} else if $1.val == "inf" {
			f := &compoundFloatNode{val: math.Inf(1)}
			f.setRange($1, $1)
			$$ = f
		} else if $1.val == "nan" {
			f := &compoundFloatNode{val: math.NaN()}
			f.setRange($1, $1)
			$$ = f
		} else {
			$$ = $1
		}
	}

numLit : _FLOAT_LIT {
        $$ = $1
    }
	| '-' _FLOAT_LIT {
		f := &compoundFloatNode{val: -$2.val}
		f.setRange($1, $2)
		$$ = f
	}
	| '+' _FLOAT_LIT {
		f := &compoundFloatNode{val: $2.val}
		f.setRange($1, $2)
		$$ = f
	}
	| '+' _INF {
		f := &compoundFloatNode{val: math.Inf(1)}
		f.setRange($1, $2)
		$$ = f
	}
	| '-' _INF {
		f := &compoundFloatNode{val: math.Inf(-1)}
		f.setRange($1, $2)
		$$ = f
	}
	| _INT_LIT {
        $$ = $1
    }
    | '+' _INT_LIT {
          i := &compoundUintNode{val: $2.val}
          i.setRange($1, $2)
          $$ = i
    }
    | '-' _INT_LIT {
        if $2.val > math.MaxInt64 + 1 {
            // can't represent as int so treat as float literal
            f := &compoundFloatNode{val: -float64($2.val)}
            f.setRange($1, $2)
            $$ = f
        } else {
            i := &compoundIntNode{val: -int64($2.val)}
            i.setRange($1, $2)
            $$ = i
        }
    }

stringLit : _STRING_LIT {
        $$ = &compoundStringNode{val: $1.val}
        $$.setRange($1, $1)
    }
    | stringLit _STRING_LIT {
        $$ = &compoundStringNode{val: $1.val + $2.val}
        $$.setRange($1, $2)
    }

aggregate : '{' aggFields '}' {
		a := &aggregateLiteralNode{elements: $2}
		a.setRange($1, $3)
		$$ = a
	}

aggFields : aggField
	| aggFields aggField {
		$$ = append($1, $2...)
	}
	| {
		$$ = nil
	}

aggField : aggFieldEntry
	| aggFieldEntry ',' {
		$$ = $1
	}
	| aggFieldEntry ';' {
		$$ = $1
	}
	| error ',' {
	}
	| error ';' {
	}
	| error {
	}

aggFieldEntry : aggName ':' scalarConstant {
		a := &aggregateEntryNode{name: $1, val: $3}
		a.setRange($1, $3)
		$$ = []*aggregateEntryNode{a}
	}
	| aggName ':' '[' ']' {
		s := &sliceLiteralNode{}
		s.setRange($3, $4)
		a := &aggregateEntryNode{name: $1, val: s}
		a.setRange($1, $4)
		$$ = []*aggregateEntryNode{a}
	}
	| aggName ':' '[' constantList ']' {
		s := &sliceLiteralNode{elements: $4}
		s.setRange($3, $5)
		a := &aggregateEntryNode{name: $1, val: s}
		a.setRange($1, $5)
		$$ = []*aggregateEntryNode{a}
	}
	| aggName ':' '[' error ']' {
	}
	| aggName ':' aggregate {
		a := &aggregateEntryNode{name: $1, val: $3}
		a.setRange($1, $3)
		$$ = []*aggregateEntryNode{a}
	}
	| aggName aggregate {
		a := &aggregateEntryNode{name: $1, val: $2}
		a.setRange($1, $2)
		$$ = []*aggregateEntryNode{a}
	}
	| aggName ':' '<' aggFields '>' {
		s := &aggregateLiteralNode{elements: $4}
		s.setRange($3, $5)
		a := &aggregateEntryNode{name: $1, val: s}
		a.setRange($1, $5)
		$$ = []*aggregateEntryNode{a}
	}
	| aggName '<' aggFields '>' {
		s := &aggregateLiteralNode{elements: $3}
		s.setRange($2, $4)
		a := &aggregateEntryNode{name: $1, val: s}
		a.setRange($1, $4)
		$$ = []*aggregateEntryNode{a}
	}
	| aggName ':' '<' error '>' {
	}
	| aggName '<' error '>' {
	}

aggName : name {
        n := &compoundIdentNode{val: $1.val}
        n.setRange($1, $1)
		$$ = &aggregateNameNode{name: n}
		$$.setRange($1, $1)
	}
	| '[' typeIdent ']' {
		$$ = &aggregateNameNode{name: $2, isExtension: true}
		$$.setRange($1, $3)
	}
	| '[' error ']' {
	}

constantList : constant {
		$$ = []valueNode{$1}
	}
	| constantList ',' constant {
		$$ = append($1, $3)
	}
	| constantList ';' constant {
		$$ = append($1, $3)
	}
	| '<' aggFields '>' {
		s := &aggregateLiteralNode{elements: $2}
		s.setRange($1, $3)
		$$ = []valueNode{s}
	}
	| constantList ','  '<' aggFields '>' {
		s := &aggregateLiteralNode{elements: $4}
		s.setRange($3, $5)
		$$ = append($1, s)
	}
	| constantList ';'  '<' aggFields '>' {
		s := &aggregateLiteralNode{elements: $4}
		s.setRange($3, $5)
		$$ = append($1, s)
	}
	| '<' error '>' {
	}
	| constantList ','  '<' error '>' {
	}
	| constantList ';'  '<' error '>' {
	}

typeIdent : ident
    | '.' ident {
          $$ = &compoundIdentNode{val: "." + $2.val}
          $$.setRange($1, $2)
    }

field : _REQUIRED typeIdent name '=' _INT_LIT ';' {
		lbl := fieldLabel{identNode: $1, required: true}
		$$ = &fieldNode{label: lbl, fldType: $2, name: $3, tag: $5}
		$$.setRange($1, $6)
	}
	| _OPTIONAL typeIdent name '=' _INT_LIT ';' {
		lbl := fieldLabel{identNode: $1}
		$$ = &fieldNode{label: lbl, fldType: $2, name: $3, tag: $5}
		$$.setRange($1, $6)
	}
	| _REPEATED typeIdent name '=' _INT_LIT ';' {
		lbl := fieldLabel{identNode: $1, repeated: true}
		$$ = &fieldNode{label: lbl, fldType: $2, name: $3, tag: $5}
		$$.setRange($1, $6)
	}
	| typeIdent name '=' _INT_LIT ';' {
		$$ = &fieldNode{fldType: $1, name: $2, tag: $4}
		$$.setRange($1, $5)
	}
	| _REQUIRED typeIdent name '=' _INT_LIT compactOptions ';' {
		lbl := fieldLabel{identNode: $1, required: true}
		$$ = &fieldNode{label: lbl, fldType: $2, name: $3, tag: $5, options: $6}
		$$.setRange($1, $7)
	}
	| _OPTIONAL typeIdent name '=' _INT_LIT compactOptions ';' {
		lbl := fieldLabel{identNode: $1}
		$$ = &fieldNode{label: lbl, fldType: $2, name: $3, tag: $5, options: $6}
		$$.setRange($1, $7)
	}
	| _REPEATED typeIdent name '=' _INT_LIT compactOptions ';' {
		lbl := fieldLabel{identNode: $1, repeated: true}
		$$ = &fieldNode{label: lbl, fldType: $2, name: $3, tag: $5, options: $6}
		$$.setRange($1, $7)
	}
	| typeIdent name '=' _INT_LIT compactOptions ';' {
		$$ = &fieldNode{fldType: $1, name: $2, tag: $4, options: $5}
		$$.setRange($1, $6)
	}

compactOptions: '[' compactOptionDecls ']' {
        $$ = &compactOptionsNode{decls: $2}
        $$.setRange($1, $3)
    }

compactOptionDecls : compactOptionDecls ',' compactOption {
		$$ = append($1, $3...)
	}
	| compactOption

compactOption: optionName '=' constant {
		n := &optionNameNode{parts: $1}
		n.setRange($1[0], $1[len($1)-1])
		o := &optionNode{name: n, val: $3}
		o.setRange($1[0], $3)
		$$ = []*optionNode{o}
	}

group : _REQUIRED _GROUP name '=' _INT_LIT '{' messageBody '}' {
		lbl := fieldLabel{identNode: $1, required: true}
		$$ = &groupNode{groupKeyword: $2, label: lbl, name: $3, tag: $5, decls: $7}
		$$.setRange($1, $8)
	}
	| _OPTIONAL _GROUP name '=' _INT_LIT '{' messageBody '}' {
		lbl := fieldLabel{identNode: $1}
		$$ = &groupNode{groupKeyword: $2, label: lbl, name: $3, tag: $5, decls: $7}
		$$.setRange($1, $8)
	}
	| _REPEATED _GROUP name '=' _INT_LIT '{' messageBody '}' {
		lbl := fieldLabel{identNode: $1, repeated: true}
		$$ = &groupNode{groupKeyword: $2, label: lbl, name: $3, tag: $5, decls: $7}
		$$.setRange($1, $8)
	}
	| _REQUIRED _GROUP name '=' _INT_LIT compactOptions '{' messageBody '}' {
		lbl := fieldLabel{identNode: $1, required: true}
		$$ = &groupNode{groupKeyword: $2, label: lbl, name: $3, tag: $5, options: $6, decls: $8}
		$$.setRange($1, $9)
	}
	| _OPTIONAL _GROUP name '=' _INT_LIT compactOptions '{' messageBody '}' {
		lbl := fieldLabel{identNode: $1}
		$$ = &groupNode{groupKeyword: $2, label: lbl, name: $3, tag: $5, options: $6, decls: $8}
		$$.setRange($1, $9)
	}
	| _REPEATED _GROUP name '=' _INT_LIT compactOptions '{' messageBody '}' {
		lbl := fieldLabel{identNode: $1, repeated: true}
		$$ = &groupNode{groupKeyword: $2, label: lbl, name: $3, tag: $5, options: $6, decls: $8}
		$$.setRange($1, $9)
	}

oneof : _ONEOF name '{' oneofBody '}' {
		$$ = &oneOfNode{name: $2, decls: $4}
		$$.setRange($1, $5)
	}

oneofBody : oneofBody oneofItem {
		$$ = append($1, $2...)
	}
	| oneofItem
	| {
		$$ = nil
	}

oneofItem : option {
		$$ = []*oneOfElement{{option: $1[0]}}
	}
	| oneofField {
		$$ = []*oneOfElement{{field: $1}}
	}
	| oneofGroup {
		$$ = []*oneOfElement{{group: $1}}
	}
	| ';' {
		$$ = []*oneOfElement{{empty: $1}}
	}
	| error ';' {
	}
	| error {
	}

oneofField : typeIdent name '=' _INT_LIT ';' {
		$$ = &fieldNode{fldType: $1, name: $2, tag: $4}
		$$.setRange($1, $5)
	}
	| typeIdent name '=' _INT_LIT compactOptions ';' {
		$$ = &fieldNode{fldType: $1, name: $2, tag: $4, options: $5}
		$$.setRange($1, $6)
	}

oneofGroup : _GROUP name '=' _INT_LIT '{' messageBody '}' {
		$$ = &groupNode{groupKeyword: $1, name: $2, tag: $4, decls: $6}
		$$.setRange($1, $7)
	}
	| _GROUP name '=' _INT_LIT compactOptions '{' messageBody '}' {
		$$ = &groupNode{groupKeyword: $1, name: $2, tag: $4, options: $5, decls: $7}
		$$.setRange($1, $8)
	}

mapField : mapType name '=' _INT_LIT ';' {
		$$ = &mapFieldNode{mapType: $1, name: $2, tag: $4}
		$$.setRange($1, $5)
	}
	| mapType name '=' _INT_LIT compactOptions ';' {
		$$ = &mapFieldNode{mapType: $1, name: $2, tag: $4, options: $5}
		$$.setRange($1, $6)
	}

mapType : _MAP '<' keyType ',' typeIdent '>' {
        $$ = &mapTypeNode{mapKeyword: $1, keyType: $3, valueType: $5}
        $$.setRange($1, $6)
}

keyType : _INT32
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

extensions : _EXTENSIONS tagRanges ';' {
		$$ = &extensionRangeNode{ranges: $2}
		$$.setRange($1, $3)
	}
	| _EXTENSIONS tagRanges compactOptions ';' {
		$$ = &extensionRangeNode{ranges: $2, options: $3}
		$$.setRange($1, $4)
	}

tagRanges : tagRanges ',' tagRange {
		$$ = append($1, $3...)
	}
	| tagRange

tagRange : _INT_LIT {
		r := &rangeNode{startNode: $1}
		r.setRange($1, $1)
		$$ = []*rangeNode{r}
	}
	| _INT_LIT _TO _INT_LIT {
		r := &rangeNode{startNode: $1, endNode: $3}
		r.setRange($1, $3)
		$$ = []*rangeNode{r}
	}
	| _INT_LIT _TO _MAX {
		r := &rangeNode{startNode: $1, endNode: $3, endMax: true}
		r.setRange($1, $3)
		$$ = []*rangeNode{r}
	}

enumRanges : enumRanges ',' enumRange {
		$$ = append($1, $3...)
	}
	| enumRange

enumRange : intLit {
		r := &rangeNode{startNode: $1}
		r.setRange($1, $1)
		$$ = []*rangeNode{r}
	}
	| intLit _TO intLit {
		r := &rangeNode{startNode: $1, endNode: $3}
		r.setRange($1, $3)
		$$ = []*rangeNode{r}
	}
	| intLit _TO _MAX {
		r := &rangeNode{startNode: $1, endNode: $3, endMax: true}
		r.setRange($1, $3)
		$$ = []*rangeNode{r}
	}

intLit : _INT_LIT {
		i := &compoundIntNode{val: int64($1.val)}
		i.setRange($1, $1)
		$$ = i
	}
	| '-' _INT_LIT {
		if $2.val > math.MaxInt64 + 1 {
			lexError(protolex, $2.start(), fmt.Sprintf("numeric constant %d would underflow 64-bit signed int (allowed range is %d to %d)", $2.val, int64(math.MinInt64), int64(math.MaxInt64)))
		}
		i := &compoundIntNode{val: -int64($2.val)}
		i.setRange($1, $2)
		$$ = i
	}

msgReserved : _RESERVED tagRanges ';' {
		$$ = &reservedNode{ranges: $2}
		$$.setRange($1, $3)
	}
	| reservedNames

enumReserved : _RESERVED enumRanges ';' {
		$$ = &reservedNode{ranges: $2}
		$$.setRange($1, $3)
	}
	| reservedNames

reservedNames : _RESERVED fieldNames ';' {
		$$ = &reservedNode{names: $2}
		$$.setRange($1, $3)
	}

fieldNames : fieldNames ',' stringLit {
		$$ = append($1, $3)
	}
	| stringLit {
		$$ = []*compoundStringNode{$1}
	}

enum : _ENUM name '{' enumBody '}' {
		$$ = &enumNode{name: $2, decls: $4}
		$$.setRange($1, $5)
	}

enumBody : enumBody enumItem {
		$$ = append($1, $2...)
	}
	| enumItem
	| {
		$$ = nil
	}

enumItem : option {
		$$ = []*enumElement{{option: $1[0]}}
	}
	| enumField {
		$$ = []*enumElement{{value: $1}}
	}
	| enumReserved {
		$$ = []*enumElement{{reserved: $1}}
	}
	| ';' {
		$$ = []*enumElement{{empty: $1}}
	}
	| error ';' {
	}
	| error {
	}

enumField : name '=' intLit ';' {
		$$ = &enumValueNode{name: $1, number: $3}
		$$.setRange($1, $4)
	}
	|  name '=' intLit compactOptions ';' {
		$$ = &enumValueNode{name: $1, number: $3, options: $4}
		$$.setRange($1, $5)
	}

message : _MESSAGE name '{' messageBody '}' {
		$$ = &messageNode{name: $2, decls: $4}
		$$.setRange($1, $5)
	}

messageBody : messageBody messageItem {
		$$ = append($1, $2...)
	}
	| messageItem
	| {
		$$ = nil
	}

messageItem : field {
		$$ = []*messageElement{{field: $1}}
	}
	| enum {
		$$ = []*messageElement{{enum: $1}}
	}
	| message {
		$$ = []*messageElement{{nested: $1}}
	}
	| extend {
		$$ = []*messageElement{{extend: $1}}
	}
	| extensions {
		$$ = []*messageElement{{extensionRange: $1}}
	}
	| group {
		$$ = []*messageElement{{group: $1}}
	}
	| option {
		$$ = []*messageElement{{option: $1[0]}}
	}
	| oneof {
		$$ = []*messageElement{{oneOf: $1}}
	}
	| mapField {
		$$ = []*messageElement{{mapField: $1}}
	}
	| msgReserved {
		$$ = []*messageElement{{reserved: $1}}
	}
	| ';' {
		$$ = []*messageElement{{empty: $1}}
	}
	| error ';' {
	}
	| error {
	}

extend : _EXTEND typeIdent '{' extendBody '}' {
		$$ = &extendNode{extendee: $2, decls: $4}
		$$.setRange($1, $5)
	}

extendBody : extendBody extendItem {
		$$ = append($1, $2...)
	}
	| extendItem
	| {
		$$ = nil
	}

extendItem : field {
		$$ = []*extendElement{{field: $1}}
	}
	| group {
		$$ = []*extendElement{{group: $1}}
	}
	| ';' {
		$$ = []*extendElement{{empty: $1}}
	}
	| error ';' {
	}
	| error {
	}

service : _SERVICE name '{' serviceBody '}' {
		$$ = &serviceNode{name: $2, decls: $4}
		$$.setRange($1, $5)
	}

serviceBody : serviceBody serviceItem {
		$$ = append($1, $2...)
	}
	| serviceItem
	| {
		$$ = nil
	}

// NB: doc suggests support for "stream" declaration, separate from "rpc", but
// it does not appear to be supported in protoc (doc is likely from grammar for
// Google-internal version of protoc, with support for streaming stubby)
serviceItem : option {
		$$ = []*serviceElement{{option: $1[0]}}
	}
	| rpc {
		$$ = []*serviceElement{{rpc: $1}}
	}
	| ';' {
		$$ = []*serviceElement{{empty: $1}}
	}
	| error ';' {
	}
	| error {
	}

rpc : _RPC name '(' rpcType ')' _RETURNS '(' rpcType ')' ';' {
		$$ = &methodNode{name: $2, input: $4, output: $8}
		$$.setRange($1, $10)
	}
	| _RPC name '(' rpcType ')' _RETURNS '(' rpcType ')' '{' rpcOptions '}' {
		$$ = &methodNode{name: $2, input: $4, output: $8, options: $11}
		$$.setRange($1, $12)
	}

rpcType : _STREAM typeIdent {
		$$ = &rpcTypeNode{msgType: $2, streamKeyword: $1}
		$$.setRange($1, $2)
	}
	| typeIdent {
		$$ = &rpcTypeNode{msgType: $1}
		$$.setRange($1, $1)
	}

rpcOptions : rpcOptions rpcOption {
		$$ = append($1, $2...)
	}
	| rpcOption
	| {
		$$ = []*optionNode{}
	}

rpcOption : option {
		$$ = $1
	}
	| ';' {
		$$ = []*optionNode{}
	}
	| error ';' {
	}
	| error {
	}

name : _NAME
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
