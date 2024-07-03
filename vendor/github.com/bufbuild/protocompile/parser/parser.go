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
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/reporter"
)

// The path ../.tmp/bin/goyacc is built when using `make generate` from repo root.
//go:generate ../.tmp/bin/goyacc -o proto.y.go -l -p proto proto.y

func init() {
	protoErrorVerbose = true

	// fix up the generated "token name" array so that error messages are nicer
	setTokenName(_STRING_LIT, "string literal")
	setTokenName(_INT_LIT, "int literal")
	setTokenName(_FLOAT_LIT, "float literal")
	setTokenName(_NAME, "identifier")
	setTokenName(_ERROR, "error")
	// for keywords, just show the keyword itself wrapped in quotes
	for str, i := range keywords {
		setTokenName(i, fmt.Sprintf(`"%s"`, str))
	}
}

func setTokenName(token int, text string) {
	// NB: this is based on logic in generated parse code that translates the
	// int returned from the lexer into an internal token number.
	var intern int8
	if token < len(protoTok1) {
		intern = protoTok1[token]
	} else {
		if token >= protoPrivate {
			if token < protoPrivate+len(protoTok2) {
				intern = protoTok2[token-protoPrivate]
			}
		}
		if intern == 0 {
			for i := 0; i+1 < len(protoTok3); i += 2 {
				if int(protoTok3[i]) == token {
					intern = protoTok3[i+1]
					break
				}
			}
		}
	}

	if intern >= 1 && int(intern-1) < len(protoToknames) {
		protoToknames[intern-1] = text
		return
	}

	panic(fmt.Sprintf("Unknown token value: %d", token))
}

// Parse parses the given source code info and returns an AST. The given filename
// is used to construct error messages and position information. The given reader
// supplies the source code. The given handler is used to report errors and
// warnings encountered while parsing. If any errors are reported, this function
// returns a non-nil error.
//
// If the error returned is due to a syntax error in the source, then a non-nil
// AST is also returned. If the handler chooses to not abort the parse (e.g. the
// underlying error reporter returns nil instead of an error), the parser will
// attempt to recover and keep going. This allows multiple syntax errors to be
// reported in a single pass. And it also means that more of the AST can be
// populated (erroneous productions around the syntax error will of course be
// absent).
//
// The degree to which the parser can recover from errors and populate the AST
// depends on the nature of the syntax error and if there are any tokens after the
// syntax error that can help the parser recover. This error recovery and partial
// AST production is best effort.
func Parse(filename string, r io.Reader, handler *reporter.Handler) (*ast.FileNode, error) {
	lx, err := newLexer(r, filename, handler)
	if err != nil {
		return nil, err
	}
	protoParse(lx)
	if lx.res == nil {
		// nil AST means there was an error that prevented any parsing
		// or the file was empty; synthesize empty non-nil AST
		lx.res = ast.NewEmptyFileNode(filename)
	}
	return lx.res, handler.Error()
}

// Result is the result of constructing a descriptor proto from a parsed AST.
// From this result, the AST and the file descriptor proto can be had. This
// also contains numerous lookup functions, for looking up AST nodes that
// correspond to various elements of the descriptor hierarchy.
//
// Results can be created without AST information, using the ResultWithoutAST()
// function. All functions other than AST() will still return non-nil values,
// allowing compile operations to work with files that have only intermediate
// descriptor protos and no source code. For such results, the function that
// return AST nodes will return placeholder nodes. The position information for
// placeholder nodes contains only the filename.
type Result interface {
	// AST returns the parsed abstract syntax tree. This returns nil if the
	// Result was created without an AST.
	AST() *ast.FileNode
	// FileDescriptorProto returns the file descriptor proto.
	FileDescriptorProto() *descriptorpb.FileDescriptorProto

	// FileNode returns the root of the AST. If this result has no AST then a
	// placeholder node is returned.
	FileNode() ast.FileDeclNode
	// Node returns the AST node from which the given message was created. This
	// can return nil, such as if the given message is not part of the
	// FileDescriptorProto hierarchy. If this result has no AST, this returns a
	// placeholder node.
	Node(proto.Message) ast.Node
	// OptionNode returns the AST node corresponding to the given uninterpreted
	// option. This can return nil, such as if the given option is not part of
	// the FileDescriptorProto hierarchy. If this result has no AST, this
	// returns a placeholder node.
	OptionNode(*descriptorpb.UninterpretedOption) ast.OptionDeclNode
	// OptionNamePartNode returns the AST node corresponding to the given name
	// part for an uninterpreted option. This can return nil, such as if the
	// given name part is not part of the FileDescriptorProto hierarchy. If this
	// result has no AST, this returns a placeholder node.
	OptionNamePartNode(*descriptorpb.UninterpretedOption_NamePart) ast.Node
	// MessageNode returns the AST node corresponding to the given message. This
	// can return nil, such as if the given message is not part of the
	// FileDescriptorProto hierarchy. If this result has no AST, this returns a
	// placeholder node.
	MessageNode(*descriptorpb.DescriptorProto) ast.MessageDeclNode
	// FieldNode returns the AST node corresponding to the given field. This can
	// return nil, such as if the given field is not part of the
	// FileDescriptorProto hierarchy. If this result has no AST, this returns a
	// placeholder node.
	FieldNode(*descriptorpb.FieldDescriptorProto) ast.FieldDeclNode
	// OneofNode returns the AST node corresponding to the given oneof. This can
	// return nil, such as if the given oneof is not part of the
	// FileDescriptorProto hierarchy. If this result has no AST, this returns a
	// placeholder node.
	OneofNode(*descriptorpb.OneofDescriptorProto) ast.OneofDeclNode
	// ExtensionRangeNode returns the AST node corresponding to the given
	// extension range. This can return nil, such as if the given range is not
	// part of the FileDescriptorProto hierarchy. If this result has no AST,
	// this returns a placeholder node.
	ExtensionRangeNode(*descriptorpb.DescriptorProto_ExtensionRange) ast.RangeDeclNode

	// ExtensionsNode returns the AST node corresponding to the "extensions"
	// statement in a message that corresponds to the given range. This will be
	// the parent of the node returned by ExtensionRangeNode, which contains the
	// options that apply to all child ranges.
	ExtensionsNode(*descriptorpb.DescriptorProto_ExtensionRange) ast.NodeWithOptions

	// MessageReservedRangeNode returns the AST node corresponding to the given
	// reserved range. This can return nil, such as if the given range is not
	// part of the FileDescriptorProto hierarchy. If this result has no AST,
	// this returns a placeholder node.
	MessageReservedRangeNode(*descriptorpb.DescriptorProto_ReservedRange) ast.RangeDeclNode
	// EnumNode returns the AST node corresponding to the given enum. This can
	// return nil, such as if the given enum is not part of the
	// FileDescriptorProto hierarchy. If this result has no AST, this returns a
	// placeholder node.
	EnumNode(*descriptorpb.EnumDescriptorProto) ast.NodeWithOptions
	// EnumValueNode returns the AST node corresponding to the given enum. This
	// can return nil, such as if the given enum value is not part of the
	// FileDescriptorProto hierarchy. If this result has no AST, this returns a
	// placeholder node.
	EnumValueNode(*descriptorpb.EnumValueDescriptorProto) ast.EnumValueDeclNode
	// EnumReservedRangeNode returns the AST node corresponding to the given
	// reserved range. This can return nil, such as if the given range is not
	// part of the FileDescriptorProto hierarchy. If this result has no AST,
	// this returns a placeholder node.
	EnumReservedRangeNode(*descriptorpb.EnumDescriptorProto_EnumReservedRange) ast.RangeDeclNode
	// ServiceNode returns the AST node corresponding to the given service. This
	// can return nil, such as if the given service is not part of the
	// FileDescriptorProto hierarchy. If this result has no AST, this returns a
	// placeholder node.
	ServiceNode(*descriptorpb.ServiceDescriptorProto) ast.NodeWithOptions
	// MethodNode returns the AST node corresponding to the given method. This
	// can return nil, such as if the given method is not part of the
	// FileDescriptorProto hierarchy. If this result has no AST, this returns a
	// placeholder node.
	MethodNode(*descriptorpb.MethodDescriptorProto) ast.RPCDeclNode
}
