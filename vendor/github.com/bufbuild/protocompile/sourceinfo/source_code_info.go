// Copyright 2020-2022 Buf Technologies, Inc.
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

// Package sourceinfo contains the logic for computing source code info for a
// file descriptor.
//
// The inputs to the computation are an AST for a file as well as the index of
// interpreted options for that file.
package sourceinfo

import (
	"bytes"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/internal"
	"github.com/bufbuild/protocompile/options"
)

// GenerateSourceInfo generates source code info for the given AST. If the given
// opts is present, it can generate source code info for interpreted options.
// Otherwise, any options in the AST will get source code info as uninterpreted
// options.
//
// This includes comments only for locations that represent complete declarations.
// This is the same behavior as protoc, the reference compiler for Protocol Buffers.
func GenerateSourceInfo(file *ast.FileNode, opts options.Index) *descriptorpb.SourceCodeInfo {
	return generateSourceInfo(file, opts, false)
}

// GenerateSourceInfoWithExtraComments generates source code info for the given
// AST. If the given opts is present, it can generate source code info for
// interpreted options. Otherwise, any options in the AST will get source code
// info as uninterpreted options.
//
// This includes comments for all locations. This is still lossy, but less so as
// it preserves far more comments from the source file.
func GenerateSourceInfoWithExtraComments(file *ast.FileNode, opts options.Index) *descriptorpb.SourceCodeInfo {
	return generateSourceInfo(file, opts, true)
}

func generateSourceInfo(file *ast.FileNode, opts options.Index, extraComments bool) *descriptorpb.SourceCodeInfo {
	if file == nil {
		return nil
	}

	sci := sourceCodeInfo{file: file, commentsUsed: map[ast.SourcePos]struct{}{}, extraComments: extraComments}
	path := make([]int32, 0, 10)

	sci.newLocWithoutComments(file, nil)

	if file.Syntax != nil {
		sci.newLocWithComments(file.Syntax, append(path, internal.FileSyntaxTag))
	}

	var depIndex, pubDepIndex, weakDepIndex, optIndex, msgIndex, enumIndex, extendIndex, svcIndex int32

	for _, child := range file.Decls {
		switch child := child.(type) {
		case *ast.ImportNode:
			sci.newLocWithComments(child, append(path, internal.FileDependencyTag, depIndex))
			depIndex++
			if child.Public != nil {
				sci.newLoc(child.Public, append(path, internal.FilePublicDependencyTag, pubDepIndex))
				pubDepIndex++
			} else if child.Weak != nil {
				sci.newLoc(child.Weak, append(path, internal.FileWeakDependencyTag, weakDepIndex))
				weakDepIndex++
			}
		case *ast.PackageNode:
			sci.newLocWithComments(child, append(path, internal.FilePackageTag))
		case *ast.OptionNode:
			generateSourceCodeInfoForOption(opts, &sci, child, false, &optIndex, append(path, internal.FileOptionsTag))
		case *ast.MessageNode:
			generateSourceCodeInfoForMessage(opts, &sci, child, nil, append(path, internal.FileMessagesTag, msgIndex))
			msgIndex++
		case *ast.EnumNode:
			generateSourceCodeInfoForEnum(opts, &sci, child, append(path, internal.FileEnumsTag, enumIndex))
			enumIndex++
		case *ast.ExtendNode:
			generateSourceCodeInfoForExtensions(opts, &sci, child, &extendIndex, &msgIndex, append(path, internal.FileExtensionsTag), append(dup(path), internal.FileMessagesTag))
		case *ast.ServiceNode:
			generateSourceCodeInfoForService(opts, &sci, child, append(path, internal.FileServicesTag, svcIndex))
			svcIndex++
		}
	}

	return &descriptorpb.SourceCodeInfo{Location: sci.locs}
}

func generateSourceCodeInfoForOption(opts options.Index, sci *sourceCodeInfo, n *ast.OptionNode, compact bool, uninterpIndex *int32, path []int32) {
	if !compact {
		sci.newLocWithoutComments(n, path)
	}
	subPath := opts[n]
	if len(subPath) > 0 {
		p := make([]int32, len(path), len(path)+len(subPath))
		copy(p, path)
		if subPath[0] == -1 {
			// used by "default" and "json_name" field pseudo-options
			// to attribute path to parent element (since those are
			// stored directly on the descriptor, not its options)
			subPath = subPath[1:]
			p = p[:len(path)-1]
		}
		p = append(p, subPath...)
		if compact {
			sci.newLoc(n, p)
		} else {
			sci.newLocWithComments(n, p)
		}
		return
	}

	// it's an uninterpreted option
	optPath := path
	optPath = append(optPath, internal.UninterpretedOptionsTag, *uninterpIndex)
	*uninterpIndex++
	sci.newLoc(n, optPath)
	var valTag int32
	switch n.Val.(type) {
	case ast.IdentValueNode:
		valTag = internal.UninterpretedIdentTag
	case *ast.NegativeIntLiteralNode:
		valTag = internal.UninterpretedNegIntTag
	case ast.IntValueNode:
		valTag = internal.UninterpretedPosIntTag
	case ast.FloatValueNode:
		valTag = internal.UninterpretedDoubleTag
	case ast.StringValueNode:
		valTag = internal.UninterpretedStringTag
	case *ast.MessageLiteralNode:
		valTag = internal.UninterpretedAggregateTag
	}
	if valTag != 0 {
		sci.newLoc(n.Val, append(optPath, valTag))
	}
	for j, nn := range n.Name.Parts {
		optNmPath := optPath
		optNmPath = append(optNmPath, internal.UninterpretedNameTag, int32(j))
		sci.newLoc(nn, optNmPath)
		sci.newLoc(nn.Name, append(optNmPath, internal.UninterpretedNameNameTag))
	}
}

func generateSourceCodeInfoForMessage(opts options.Index, sci *sourceCodeInfo, n ast.MessageDeclNode, fieldPath []int32, path []int32) {
	var openBrace ast.Node

	var decls []ast.MessageElement
	switch n := n.(type) {
	case *ast.MessageNode:
		openBrace = n.OpenBrace
		decls = n.Decls
	case *ast.GroupNode:
		openBrace = n.OpenBrace
		decls = n.Decls
	case *ast.MapFieldNode:
		sci.newLoc(n, path)
		// map entry so nothing else to do
		return
	}
	sci.newBlockLocWithComments(n, openBrace, path)

	sci.newLoc(n.MessageName(), append(path, internal.MessageNameTag))
	// matching protoc, which emits the corresponding field type name (for group fields)
	// right after the source location for the group message name
	if fieldPath != nil {
		sci.newLoc(n.MessageName(), append(fieldPath, internal.FieldTypeNameTag))
	}

	var optIndex, fieldIndex, oneOfIndex, extendIndex, nestedMsgIndex int32
	var nestedEnumIndex, extRangeIndex, reservedRangeIndex, reservedNameIndex int32
	for _, child := range decls {
		switch child := child.(type) {
		case *ast.OptionNode:
			generateSourceCodeInfoForOption(opts, sci, child, false, &optIndex, append(path, internal.MessageOptionsTag))
		case *ast.FieldNode:
			generateSourceCodeInfoForField(opts, sci, child, append(path, internal.MessageFieldsTag, fieldIndex))
			fieldIndex++
		case *ast.GroupNode:
			fldPath := path
			fldPath = append(fldPath, internal.MessageFieldsTag, fieldIndex)
			generateSourceCodeInfoForField(opts, sci, child, fldPath)
			fieldIndex++
			generateSourceCodeInfoForMessage(opts, sci, child, fldPath, append(dup(path), internal.MessageNestedMessagesTag, nestedMsgIndex))
			nestedMsgIndex++
		case *ast.MapFieldNode:
			generateSourceCodeInfoForField(opts, sci, child, append(path, internal.MessageFieldsTag, fieldIndex))
			fieldIndex++
			nestedMsgIndex++
		case *ast.OneOfNode:
			generateSourceCodeInfoForOneOf(opts, sci, child, &fieldIndex, &nestedMsgIndex, append(path, internal.MessageFieldsTag), append(dup(path), internal.MessageNestedMessagesTag), append(dup(path), internal.MessageOneOfsTag, oneOfIndex))
			oneOfIndex++
		case *ast.MessageNode:
			generateSourceCodeInfoForMessage(opts, sci, child, nil, append(path, internal.MessageNestedMessagesTag, nestedMsgIndex))
			nestedMsgIndex++
		case *ast.EnumNode:
			generateSourceCodeInfoForEnum(opts, sci, child, append(path, internal.MessageEnumsTag, nestedEnumIndex))
			nestedEnumIndex++
		case *ast.ExtendNode:
			generateSourceCodeInfoForExtensions(opts, sci, child, &extendIndex, &nestedMsgIndex, append(path, internal.MessageExtensionsTag), append(dup(path), internal.MessageNestedMessagesTag))
		case *ast.ExtensionRangeNode:
			generateSourceCodeInfoForExtensionRanges(opts, sci, child, &extRangeIndex, append(path, internal.MessageExtensionRangesTag))
		case *ast.ReservedNode:
			if len(child.Names) > 0 {
				resPath := path
				resPath = append(resPath, internal.MessageReservedNamesTag)
				sci.newLocWithComments(child, resPath)
				for _, rn := range child.Names {
					sci.newLoc(rn, append(resPath, reservedNameIndex))
					reservedNameIndex++
				}
			}
			if len(child.Ranges) > 0 {
				resPath := path
				resPath = append(resPath, internal.MessageReservedRangesTag)
				sci.newLocWithComments(child, resPath)
				for _, rr := range child.Ranges {
					generateSourceCodeInfoForReservedRange(sci, rr, append(resPath, reservedRangeIndex))
					reservedRangeIndex++
				}
			}
		}
	}
}

func generateSourceCodeInfoForEnum(opts options.Index, sci *sourceCodeInfo, n *ast.EnumNode, path []int32) {
	sci.newBlockLocWithComments(n, n.OpenBrace, path)
	sci.newLoc(n.Name, append(path, internal.EnumNameTag))

	var optIndex, valIndex, reservedNameIndex, reservedRangeIndex int32
	for _, child := range n.Decls {
		switch child := child.(type) {
		case *ast.OptionNode:
			generateSourceCodeInfoForOption(opts, sci, child, false, &optIndex, append(path, internal.EnumOptionsTag))
		case *ast.EnumValueNode:
			generateSourceCodeInfoForEnumValue(opts, sci, child, append(path, internal.EnumValuesTag, valIndex))
			valIndex++
		case *ast.ReservedNode:
			if len(child.Names) > 0 {
				resPath := path
				resPath = append(resPath, internal.EnumReservedNamesTag)
				sci.newLocWithComments(child, resPath)
				for _, rn := range child.Names {
					sci.newLoc(rn, append(resPath, reservedNameIndex))
					reservedNameIndex++
				}
			}
			if len(child.Ranges) > 0 {
				resPath := path
				resPath = append(resPath, internal.EnumReservedRangesTag)
				sci.newLocWithComments(child, resPath)
				for _, rr := range child.Ranges {
					generateSourceCodeInfoForReservedRange(sci, rr, append(resPath, reservedRangeIndex))
					reservedRangeIndex++
				}
			}
		}
	}
}

func generateSourceCodeInfoForEnumValue(opts options.Index, sci *sourceCodeInfo, n *ast.EnumValueNode, path []int32) {
	sci.newLocWithComments(n, path)
	sci.newLoc(n.Name, append(path, internal.EnumValNameTag))
	sci.newLoc(n.Number, append(path, internal.EnumValNumberTag))

	// enum value options
	if n.Options != nil {
		optsPath := path
		optsPath = append(optsPath, internal.EnumValOptionsTag)
		sci.newLoc(n.Options, optsPath)
		var optIndex int32
		for _, opt := range n.Options.GetElements() {
			generateSourceCodeInfoForOption(opts, sci, opt, true, &optIndex, optsPath)
		}
	}
}

func generateSourceCodeInfoForReservedRange(sci *sourceCodeInfo, n *ast.RangeNode, path []int32) {
	sci.newLoc(n, path)
	sci.newLoc(n.StartVal, append(path, internal.ReservedRangeStartTag))
	switch {
	case n.EndVal != nil:
		sci.newLoc(n.EndVal, append(path, internal.ReservedRangeEndTag))
	case n.Max != nil:
		sci.newLoc(n.Max, append(path, internal.ReservedRangeEndTag))
	default:
		sci.newLoc(n.StartVal, append(path, internal.ReservedRangeEndTag))
	}
}

func generateSourceCodeInfoForExtensions(opts options.Index, sci *sourceCodeInfo, n *ast.ExtendNode, extendIndex, msgIndex *int32, extendPath, msgPath []int32) {
	sci.newBlockLocWithComments(n, n.OpenBrace, extendPath)
	for _, decl := range n.Decls {
		switch decl := decl.(type) {
		case *ast.FieldNode:
			generateSourceCodeInfoForField(opts, sci, decl, append(extendPath, *extendIndex))
			*extendIndex++
		case *ast.GroupNode:
			fldPath := extendPath
			fldPath = append(fldPath, *extendIndex)
			generateSourceCodeInfoForField(opts, sci, decl, fldPath)
			*extendIndex++
			generateSourceCodeInfoForMessage(opts, sci, decl, fldPath, append(msgPath, *msgIndex))
			*msgIndex++
		}
	}
}

func generateSourceCodeInfoForOneOf(opts options.Index, sci *sourceCodeInfo, n *ast.OneOfNode, fieldIndex, nestedMsgIndex *int32, fieldPath, nestedMsgPath, oneOfPath []int32) {
	sci.newBlockLocWithComments(n, n.OpenBrace, oneOfPath)
	sci.newLoc(n.Name, append(oneOfPath, internal.OneOfNameTag))

	var optIndex int32
	for _, child := range n.Decls {
		switch child := child.(type) {
		case *ast.OptionNode:
			generateSourceCodeInfoForOption(opts, sci, child, false, &optIndex, append(oneOfPath, internal.OneOfOptionsTag))
		case *ast.FieldNode:
			generateSourceCodeInfoForField(opts, sci, child, append(fieldPath, *fieldIndex))
			*fieldIndex++
		case *ast.GroupNode:
			fldPath := fieldPath
			fldPath = append(fldPath, *fieldIndex)
			generateSourceCodeInfoForField(opts, sci, child, fldPath)
			*fieldIndex++
			generateSourceCodeInfoForMessage(opts, sci, child, fldPath, append(nestedMsgPath, *nestedMsgIndex))
			*nestedMsgIndex++
		}
	}
}

func generateSourceCodeInfoForField(opts options.Index, sci *sourceCodeInfo, n ast.FieldDeclNode, path []int32) {
	var fieldType string
	if f, ok := n.(*ast.FieldNode); ok {
		fieldType = string(f.FldType.AsIdentifier())
	}

	if n.GetGroupKeyword() != nil {
		// comments will appear on group message
		sci.newLocWithoutComments(n, path)
		if n.FieldExtendee() != nil {
			sci.newLoc(n.FieldExtendee(), append(path, internal.FieldExtendeeTag))
		}
		if n.FieldLabel() != nil {
			// no comments here either (label is first token for group, so we want
			// to leave the comments to be associated with the group message instead)
			sci.newLocWithoutComments(n.FieldLabel(), append(path, internal.FieldLabelTag))
		}
		sci.newLoc(n.FieldType(), append(path, internal.FieldTypeTag))
		// let the name comments be attributed to the group name
		sci.newLocWithoutComments(n.FieldName(), append(path, internal.FieldNameTag))
	} else {
		sci.newLocWithComments(n, path)
		if n.FieldExtendee() != nil {
			sci.newLoc(n.FieldExtendee(), append(path, internal.FieldExtendeeTag))
		}
		if n.FieldLabel() != nil {
			sci.newLoc(n.FieldLabel(), append(path, internal.FieldLabelTag))
		}
		var tag int32
		if _, isScalar := internal.FieldTypes[fieldType]; isScalar {
			tag = internal.FieldTypeTag
		} else {
			// this is a message or an enum, so attribute type location
			// to the type name field
			tag = internal.FieldTypeNameTag
		}
		sci.newLoc(n.FieldType(), append(path, tag))
		sci.newLoc(n.FieldName(), append(path, internal.FieldNameTag))
	}
	sci.newLoc(n.FieldTag(), append(path, internal.FieldNumberTag))

	if n.GetOptions() != nil {
		optsPath := path
		optsPath = append(optsPath, internal.FieldOptionsTag)
		sci.newLoc(n.GetOptions(), optsPath)
		var optIndex int32
		for _, opt := range n.GetOptions().GetElements() {
			generateSourceCodeInfoForOption(opts, sci, opt, true, &optIndex, optsPath)
		}
	}
}

func generateSourceCodeInfoForExtensionRanges(opts options.Index, sci *sourceCodeInfo, n *ast.ExtensionRangeNode, extRangeIndex *int32, path []int32) {
	sci.newLocWithComments(n, path)
	startExtRangeIndex := *extRangeIndex
	for _, child := range n.Ranges {
		path := append(path, *extRangeIndex)
		*extRangeIndex++
		sci.newLoc(child, path)
		sci.newLoc(child.StartVal, append(path, internal.ExtensionRangeStartTag))
		switch {
		case child.EndVal != nil:
			sci.newLoc(child.EndVal, append(path, internal.ExtensionRangeEndTag))
		case child.Max != nil:
			sci.newLoc(child.Max, append(path, internal.ExtensionRangeEndTag))
		default:
			sci.newLoc(child.StartVal, append(path, internal.ExtensionRangeEndTag))
		}
	}
	// options for all ranges go after the start+end values
	for range n.Ranges {
		path := append(path, startExtRangeIndex)
		startExtRangeIndex++
		if n.Options != nil {
			optsPath := path
			optsPath = append(optsPath, internal.ExtensionRangeOptionsTag)
			sci.newLoc(n.Options, optsPath)
			var optIndex int32
			for _, opt := range n.Options.GetElements() {
				generateSourceCodeInfoForOption(opts, sci, opt, true, &optIndex, optsPath)
			}
		}
	}
}

func generateSourceCodeInfoForService(opts options.Index, sci *sourceCodeInfo, n *ast.ServiceNode, path []int32) {
	sci.newBlockLocWithComments(n, n.OpenBrace, path)
	sci.newLoc(n.Name, append(path, internal.ServiceNameTag))
	var optIndex, rpcIndex int32
	for _, child := range n.Decls {
		switch child := child.(type) {
		case *ast.OptionNode:
			generateSourceCodeInfoForOption(opts, sci, child, false, &optIndex, append(path, internal.ServiceOptionsTag))
		case *ast.RPCNode:
			generateSourceCodeInfoForMethod(opts, sci, child, append(path, internal.ServiceMethodsTag, rpcIndex))
			rpcIndex++
		}
	}
}

func generateSourceCodeInfoForMethod(opts options.Index, sci *sourceCodeInfo, n *ast.RPCNode, path []int32) {
	if n.OpenBrace != nil {
		sci.newBlockLocWithComments(n, n.OpenBrace, path)
	} else {
		sci.newLocWithComments(n, path)
	}
	sci.newLoc(n.Name, append(path, internal.MethodNameTag))
	if n.Input.Stream != nil {
		sci.newLoc(n.Input.Stream, append(path, internal.MethodInputStreamTag))
	}
	sci.newLoc(n.Input.MessageType, append(path, internal.MethodInputTag))
	if n.Output.Stream != nil {
		sci.newLoc(n.Output.Stream, append(path, internal.MethodOutputStreamTag))
	}
	sci.newLoc(n.Output.MessageType, append(path, internal.MethodOutputTag))

	optsPath := path
	optsPath = append(optsPath, internal.MethodOptionsTag)
	var optIndex int32
	for _, decl := range n.Decls {
		if opt, ok := decl.(*ast.OptionNode); ok {
			generateSourceCodeInfoForOption(opts, sci, opt, false, &optIndex, optsPath)
		}
	}
}

type sourceCodeInfo struct {
	file          *ast.FileNode
	extraComments bool
	locs          []*descriptorpb.SourceCodeInfo_Location
	commentsUsed  map[ast.SourcePos]struct{}
}

func (sci *sourceCodeInfo) newLocWithoutComments(n ast.Node, path []int32) {
	dup := make([]int32, len(path))
	copy(dup, path)
	var start, end ast.SourcePos
	if n == sci.file {
		// For files, we don't want to consider trailing EOF token
		// as part of the span. We want the span to only include
		// actual lexical elements in the file (which also excludes
		// whitespace and comments).
		children := sci.file.Children()
		if len(children) > 0 && isEOF(children[len(children)-1]) {
			children = children[:len(children)-1]
		}
		if len(children) == 0 {
			start = ast.SourcePos{Filename: sci.file.Name(), Line: 1, Col: 1}
			end = start
		} else {
			start = sci.file.TokenInfo(n.Start()).Start()
			end = sci.file.TokenInfo(children[len(children)-1].End()).End()
		}
	} else {
		info := sci.file.NodeInfo(n)
		start, end = info.Start(), info.End()
	}
	sci.locs = append(sci.locs, &descriptorpb.SourceCodeInfo_Location{
		Path: dup,
		Span: makeSpan(start, end),
	})
}

func (sci *sourceCodeInfo) newLoc(n ast.Node, path []int32) {
	info := sci.file.NodeInfo(n)
	if !sci.extraComments {
		dup := make([]int32, len(path))
		copy(dup, path)
		start, end := info.Start(), info.End()
		sci.locs = append(sci.locs, &descriptorpb.SourceCodeInfo_Location{
			Path: dup,
			Span: makeSpan(start, end),
		})
	} else {
		detachedComments, leadingComments := sci.getLeadingComments(n)
		trailingComments := sci.getTrailingComments(n)
		sci.newLocWithGivenComments(info, detachedComments, leadingComments, trailingComments, path)
	}
}

func isEOF(n ast.Node) bool {
	r, ok := n.(*ast.RuneNode)
	return ok && r.Rune == 0
}

func (sci *sourceCodeInfo) newBlockLocWithComments(n, openBrace ast.Node, path []int32) {
	// Block definitions use trailing comments after the open brace "{" as the
	// element's trailing comments. For example:
	//
	//    message Foo { // this is a trailing comment for a message
	//
	//    }             // not this
	//
	nodeInfo := sci.file.NodeInfo(n)
	detachedComments, leadingComments := sci.getLeadingComments(n)
	trailingComments := sci.getTrailingComments(openBrace)
	sci.newLocWithGivenComments(nodeInfo, detachedComments, leadingComments, trailingComments, path)
}

func (sci *sourceCodeInfo) newLocWithComments(n ast.Node, path []int32) {
	nodeInfo := sci.file.NodeInfo(n)
	detachedComments, leadingComments := sci.getLeadingComments(n)
	trailingComments := sci.getTrailingComments(n)
	sci.newLocWithGivenComments(nodeInfo, detachedComments, leadingComments, trailingComments, path)
}

func (sci *sourceCodeInfo) newLocWithGivenComments(nodeInfo ast.NodeInfo, detachedComments []comments, leadingComments comments, trailingComments comments, path []int32) {
	if (len(detachedComments) > 0 && sci.commentUsed(detachedComments[0])) ||
		(len(detachedComments) == 0 && sci.commentUsed(leadingComments)) {
		detachedComments = nil
		leadingComments = ast.EmptyComments
	}
	if sci.commentUsed(trailingComments) {
		trailingComments = ast.EmptyComments
	}

	var trail *string
	if trailingComments.Len() > 0 {
		trail = proto.String(sci.combineComments(trailingComments))
	}

	var lead *string
	if leadingComments.Len() > 0 {
		lead = proto.String(sci.combineComments(leadingComments))
	}

	detached := make([]string, len(detachedComments))
	for i, cmts := range detachedComments {
		detached[i] = sci.combineComments(cmts)
	}

	dup := make([]int32, len(path))
	copy(dup, path)
	sci.locs = append(sci.locs, &descriptorpb.SourceCodeInfo_Location{
		LeadingDetachedComments: detached,
		LeadingComments:         lead,
		TrailingComments:        trail,
		Path:                    dup,
		Span:                    makeSpan(nodeInfo.Start(), nodeInfo.End()),
	})
}

type comments interface {
	Len() int
	Index(int) ast.Comment
}

type subComments struct {
	offs, n int
	c       ast.Comments
}

func (s subComments) Len() int {
	return s.n
}

func (s subComments) Index(i int) ast.Comment {
	if i < 0 || i >= s.n {
		panic(fmt.Errorf("runtime error: index out of range [%d] with length %d", i, s.n))
	}
	return s.c.Index(i + s.offs)
}

func (sci *sourceCodeInfo) getLeadingComments(n ast.Node) ([]comments, comments) {
	s := n.Start()
	info := sci.file.TokenInfo(s)
	var prevInfo ast.NodeInfo
	if prev, ok := sci.file.Tokens().Previous(s); ok {
		prevInfo = sci.file.TokenInfo(prev)
	}
	_, d, l := sci.attributeComments(prevInfo, info)
	return d, l
}

func (sci *sourceCodeInfo) getTrailingComments(n ast.Node) comments {
	e := n.End()
	next, ok := sci.file.Tokens().Next(e)
	if !ok {
		return ast.EmptyComments
	}
	info := sci.file.TokenInfo(e)
	nextInfo := sci.file.TokenInfo(next)
	t, _, _ := sci.attributeComments(info, nextInfo)
	return t
}

func (sci *sourceCodeInfo) attributeComments(prevInfo, info ast.NodeInfo) (t comments, d []comments, l comments) {
	detached := groupComments(info.LeadingComments())
	var trail comments
	if prevInfo.IsValid() {
		trail = comments(prevInfo.TrailingComments())
		if trail.Len() == 0 {
			trail, detached = sci.maybeDonate(prevInfo, info, detached)
		}
	} else {
		trail = ast.EmptyComments
	}
	detached, lead := sci.maybeAttach(prevInfo, info, trail.Len() > 0, detached)
	return trail, detached, lead
}

func (sci *sourceCodeInfo) maybeDonate(prevInfo ast.NodeInfo, info ast.NodeInfo, lead []comments) (t comments, l []comments) {
	if len(lead) == 0 {
		// nothing to donate
		return ast.EmptyComments, nil
	}
	firstCommentPos := lead[0].Index(0)
	if firstCommentPos.Start().Line > prevInfo.End().Line+1 {
		// first comment is detached from previous token, so can't be a trailing comment
		return ast.EmptyComments, lead
	}
	if len(lead) > 1 {
		// multiple groups? then donate first comment to previous token
		return lead[0], lead[1:]
	}
	// there is only one element in lead
	comment := lead[0]
	lastCommentPos := comment.Index(comment.Len() - 1)
	if lastCommentPos.End().Line < info.Start().Line-1 {
		// there is a blank line between the comments and subsequent token, so
		// we can donate the comment to previous token
		return comment, nil
	}
	if txt := info.RawText(); txt == "" || (len(txt) == 1 && strings.ContainsAny(txt, "}]),;")) {
		// token is a symbol for the end of a scope or EOF, which doesn't need a leading comment
		if !sci.extraComments && txt != "" &&
			firstCommentPos.Start().Line == prevInfo.End().Line &&
			lastCommentPos.End().Line == info.Start().Line {
			// protoc does not donate if prev and next token are on the same line since it's
			// ambiguous which one should get the comment; so we mirror that here
			return ast.EmptyComments, lead
		}
		// But with extra comments, we always donate in this situation in order to capture
		// more comments. Because otherwise, these comments are lost since these symbols
		// don't map to a location in source code info.
		return comment, nil
	}
	// cannot donate
	return ast.EmptyComments, lead
}

func (sci *sourceCodeInfo) maybeAttach(prevInfo ast.NodeInfo, info ast.NodeInfo, hasTrail bool, lead []comments) (d []comments, l comments) {
	if len(lead) == 0 {
		return nil, ast.EmptyComments
	}

	if len(lead) == 1 && !hasTrail && prevInfo.IsValid() {
		// If the one comment appears attached to both previous and next tokens,
		// don't attach to either.
		comment := lead[0]
		attachedToPrevious := comment.Index(0).Start().Line == prevInfo.End().Line
		attachedToNext := comment.Index(comment.Len()-1).End().Line == info.Start().Line
		if attachedToPrevious && attachedToNext {
			// Since attachment is ambiguous, leave it detached.
			return lead, ast.EmptyComments
		}
	}

	lastComment := lead[len(lead)-1]
	if lastComment.Index(lastComment.Len()-1).End().Line >= info.Start().Line-1 {
		return lead[:len(lead)-1], lastComment
	}

	return lead, ast.EmptyComments
}

func makeSpan(start, end ast.SourcePos) []int32 {
	if start.Line == end.Line {
		return []int32{int32(start.Line) - 1, int32(start.Col) - 1, int32(end.Col) - 1}
	}
	return []int32{int32(start.Line) - 1, int32(start.Col) - 1, int32(end.Line) - 1, int32(end.Col) - 1}
}

func (sci *sourceCodeInfo) commentUsed(c comments) bool {
	if c.Len() == 0 {
		return false
	}
	pos := c.Index(0).Start()
	if _, ok := sci.commentsUsed[pos]; ok {
		return true
	}

	sci.commentsUsed[pos] = struct{}{}
	return false
}

func groupComments(cmts ast.Comments) []comments {
	if cmts.Len() == 0 {
		return nil
	}
	var groups []comments
	singleLineStyle := cmts.Index(0).RawText()[:2] == "//"
	line := cmts.Index(0).End().Line
	start := 0
	for i := 1; i < cmts.Len(); i++ {
		c := cmts.Index(i)
		prevSingleLine := singleLineStyle
		singleLineStyle = strings.HasPrefix(c.RawText(), "//")
		if !singleLineStyle || prevSingleLine != singleLineStyle || c.Start().Line > line+1 {
			// new group!
			groups = append(groups, subComments{offs: start, n: i - start, c: cmts})
			start = i
		}
		line = c.End().Line
	}
	// don't forget last group
	groups = append(groups, subComments{offs: start, n: cmts.Len() - start, c: cmts})
	return groups
}

func (sci *sourceCodeInfo) combineComments(comments comments) string {
	if comments.Len() == 0 {
		return ""
	}
	var buf bytes.Buffer
	for i, l := 0, comments.Len(); i < l; i++ {
		c := comments.Index(i)
		txt := c.RawText()
		if txt[:2] == "//" {
			buf.WriteString(txt[2:])
			// protoc includes trailing newline for line comments,
			// but it's not present in the AST comment. So we need
			// to add it if present.
			if i, ok := sci.file.Items().Next(c.AsItem()); ok {
				info := sci.file.ItemInfo(i)
				if strings.HasPrefix(info.LeadingWhitespace(), "\n") {
					buf.WriteRune('\n')
				}
			}
		} else {
			lines := strings.Split(txt[2:len(txt)-2], "\n")
			first := true
			for _, l := range lines {
				if first {
					first = false
					buf.WriteString(l)
					continue
				}
				buf.WriteByte('\n')

				// strip a prefix of whitespace followed by '*'
				j := 0
				for j < len(l) {
					if l[j] != ' ' && l[j] != '\t' {
						break
					}
					j++
				}
				switch {
				case j == len(l):
					l = ""
				case l[j] == '*':
					l = l[j+1:]
				case j > 0:
					l = l[j:]
				}

				buf.WriteString(l)
			}
		}
	}
	return buf.String()
}

func dup(p []int32) []int32 {
	return append(([]int32)(nil), p...)
}
