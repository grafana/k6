// Copyright 2020-2023 Buf Technologies, Inc.
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
	"bytes"
	"math"
	"strings"
	"unicode"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/internal"
	"github.com/bufbuild/protocompile/reporter"
)

type result struct {
	file  *ast.FileNode
	proto *descriptorpb.FileDescriptorProto

	nodes map[proto.Message]ast.Node
}

// ResultWithoutAST returns a parse result that has no AST. All methods for
// looking up AST nodes return a placeholder node that contains only the filename
// in position information.
func ResultWithoutAST(proto *descriptorpb.FileDescriptorProto) Result {
	return &result{proto: proto}
}

// ResultFromAST constructs a descriptor proto from the given AST. The returned
// result includes the descriptor proto and also contains an index that can be
// used to lookup AST node information for elements in the descriptor proto
// hierarchy.
//
// If validate is true, some basic validation is performed, to make sure the
// resulting descriptor proto is valid per protobuf rules and semantics. Only
// some language elements can be validated since some rules and semantics can
// only be checked after all symbols are all resolved, which happens in the
// linking step.
//
// The given handler is used to report any errors or warnings encountered. If any
// errors are reported, this function returns a non-nil error.
func ResultFromAST(file *ast.FileNode, validate bool, handler *reporter.Handler) (Result, error) {
	filename := file.Name()
	r := &result{file: file, nodes: map[proto.Message]ast.Node{}}
	r.createFileDescriptor(filename, file, handler)
	if validate {
		validateBasic(r, handler)
	}
	// Now that we're done validating, we can set any missing labels to optional
	// (we leave them absent in first pass if label was missing in source, so we
	// can do validation on presence of label, but final descriptors are expected
	// to always have them present).
	fillInMissingLabels(r.proto)
	return r, handler.Error()
}

func (r *result) AST() *ast.FileNode {
	return r.file
}

func (r *result) FileDescriptorProto() *descriptorpb.FileDescriptorProto {
	return r.proto
}

func (r *result) createFileDescriptor(filename string, file *ast.FileNode, handler *reporter.Handler) {
	fd := &descriptorpb.FileDescriptorProto{Name: proto.String(filename)}
	r.proto = fd

	r.putFileNode(fd, file)

	isProto3 := false
	if file.Syntax != nil {
		if file.Syntax.Syntax.AsString() == "proto3" {
			isProto3 = true
		} else if file.Syntax.Syntax.AsString() != "proto2" {
			nodeInfo := file.NodeInfo(file.Syntax.Syntax)
			if handler.HandleErrorf(nodeInfo.Start(), `syntax value must be "proto2" or "proto3"`) != nil {
				return
			}
		}

		// proto2 is the default, so no need to set unless proto3
		if isProto3 {
			fd.Syntax = proto.String(file.Syntax.Syntax.AsString())
		}
	} else {
		nodeInfo := file.NodeInfo(file)
		handler.HandleWarningWithPos(nodeInfo.Start(), ErrNoSyntax)
	}

	for _, decl := range file.Decls {
		if handler.ReporterError() != nil {
			return
		}
		switch decl := decl.(type) {
		case *ast.EnumNode:
			fd.EnumType = append(fd.EnumType, r.asEnumDescriptor(decl, handler))
		case *ast.ExtendNode:
			r.addExtensions(decl, &fd.Extension, &fd.MessageType, isProto3, handler, 0)
		case *ast.ImportNode:
			index := len(fd.Dependency)
			fd.Dependency = append(fd.Dependency, decl.Name.AsString())
			if decl.Public != nil {
				fd.PublicDependency = append(fd.PublicDependency, int32(index))
			} else if decl.Weak != nil {
				fd.WeakDependency = append(fd.WeakDependency, int32(index))
			}
		case *ast.MessageNode:
			fd.MessageType = append(fd.MessageType, r.asMessageDescriptor(decl, isProto3, handler, 1))
		case *ast.OptionNode:
			if fd.Options == nil {
				fd.Options = &descriptorpb.FileOptions{}
			}
			fd.Options.UninterpretedOption = append(fd.Options.UninterpretedOption, r.asUninterpretedOption(decl))
		case *ast.ServiceNode:
			fd.Service = append(fd.Service, r.asServiceDescriptor(decl))
		case *ast.PackageNode:
			if fd.Package != nil {
				nodeInfo := file.NodeInfo(decl)
				if handler.HandleErrorf(nodeInfo.Start(), "files should have only one package declaration") != nil {
					return
				}
			}
			pkgName := string(decl.Name.AsIdentifier())
			if len(pkgName) >= 512 {
				nodeInfo := file.NodeInfo(decl.Name)
				if handler.HandleErrorf(nodeInfo.Start(), "package name (with whitespace removed) must be less than 512 characters long") != nil {
					return
				}
			}
			if strings.Count(pkgName, ".") > 100 {
				nodeInfo := file.NodeInfo(decl.Name)
				if handler.HandleErrorf(nodeInfo.Start(), "package name may not contain more than 100 periods") != nil {
					return
				}
			}
			fd.Package = proto.String(string(decl.Name.AsIdentifier()))
		}
	}
}

func (r *result) asUninterpretedOptions(nodes []*ast.OptionNode) []*descriptorpb.UninterpretedOption {
	if len(nodes) == 0 {
		return nil
	}
	opts := make([]*descriptorpb.UninterpretedOption, len(nodes))
	for i, n := range nodes {
		opts[i] = r.asUninterpretedOption(n)
	}
	return opts
}

func (r *result) asUninterpretedOption(node *ast.OptionNode) *descriptorpb.UninterpretedOption {
	opt := &descriptorpb.UninterpretedOption{Name: r.asUninterpretedOptionName(node.Name.Parts)}
	r.putOptionNode(opt, node)

	switch val := node.Val.Value().(type) {
	case bool:
		if val {
			opt.IdentifierValue = proto.String("true")
		} else {
			opt.IdentifierValue = proto.String("false")
		}
	case int64:
		opt.NegativeIntValue = proto.Int64(val)
	case uint64:
		opt.PositiveIntValue = proto.Uint64(val)
	case float64:
		opt.DoubleValue = proto.Float64(val)
	case string:
		opt.StringValue = []byte(val)
	case ast.Identifier:
		opt.IdentifierValue = proto.String(string(val))
	default:
		// the grammar does not allow arrays here, so the only possible case
		// left should be []*ast.MessageFieldNode, which corresponds to an
		// *ast.MessageLiteralNode
		if n, ok := node.Val.(*ast.MessageLiteralNode); ok {
			var buf bytes.Buffer
			for i, el := range n.Elements {
				flattenNode(r.file, el, &buf)
				if len(n.Seps) > i && n.Seps[i] != nil {
					buf.WriteRune(' ')
					buf.WriteRune(n.Seps[i].Rune)
				}
			}
			aggStr := buf.String()
			opt.AggregateValue = proto.String(aggStr)
		}
		// TODO: else that reports an error or panics??
	}
	return opt
}

func flattenNode(f *ast.FileNode, n ast.Node, buf *bytes.Buffer) {
	if cn, ok := n.(ast.CompositeNode); ok {
		for _, ch := range cn.Children() {
			flattenNode(f, ch, buf)
		}
		return
	}

	if buf.Len() > 0 {
		buf.WriteRune(' ')
	}
	buf.WriteString(f.NodeInfo(n).RawText())
}

func (r *result) asUninterpretedOptionName(parts []*ast.FieldReferenceNode) []*descriptorpb.UninterpretedOption_NamePart {
	ret := make([]*descriptorpb.UninterpretedOption_NamePart, len(parts))
	for i, part := range parts {
		np := &descriptorpb.UninterpretedOption_NamePart{
			NamePart:    proto.String(string(part.Name.AsIdentifier())),
			IsExtension: proto.Bool(part.IsExtension()),
		}
		r.putOptionNamePartNode(np, part)
		ret[i] = np
	}
	return ret
}

func (r *result) addExtensions(ext *ast.ExtendNode, flds *[]*descriptorpb.FieldDescriptorProto, msgs *[]*descriptorpb.DescriptorProto, isProto3 bool, handler *reporter.Handler, depth int) {
	extendee := string(ext.Extendee.AsIdentifier())
	count := 0
	for _, decl := range ext.Decls {
		switch decl := decl.(type) {
		case *ast.FieldNode:
			count++
			// use higher limit since we don't know yet whether extendee is messageset wire format
			fd := r.asFieldDescriptor(decl, internal.MaxTag, isProto3, handler)
			fd.Extendee = proto.String(extendee)
			*flds = append(*flds, fd)
		case *ast.GroupNode:
			count++
			// ditto: use higher limit right now
			fd, md := r.asGroupDescriptors(decl, isProto3, internal.MaxTag, handler, depth+1)
			fd.Extendee = proto.String(extendee)
			*flds = append(*flds, fd)
			*msgs = append(*msgs, md)
		}
	}
	if count == 0 {
		nodeInfo := r.file.NodeInfo(ext)
		_ = handler.HandleErrorf(nodeInfo.Start(), "extend sections must define at least one extension")
	}
}

func asLabel(lbl *ast.FieldLabel) *descriptorpb.FieldDescriptorProto_Label {
	if !lbl.IsPresent() {
		return nil
	}
	switch {
	case lbl.Repeated:
		return descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	case lbl.Required:
		return descriptorpb.FieldDescriptorProto_LABEL_REQUIRED.Enum()
	default:
		return descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	}
}

func (r *result) asFieldDescriptor(node *ast.FieldNode, maxTag int32, isProto3 bool, handler *reporter.Handler) *descriptorpb.FieldDescriptorProto {
	tag := node.Tag.Val
	if err := r.checkTag(node.Tag, tag, maxTag); err != nil {
		_ = handler.HandleError(err)
	}
	fd := newFieldDescriptor(node.Name.Val, string(node.FldType.AsIdentifier()), int32(tag), asLabel(&node.Label))
	r.putFieldNode(fd, node)
	if opts := node.Options.GetElements(); len(opts) > 0 {
		fd.Options = &descriptorpb.FieldOptions{UninterpretedOption: r.asUninterpretedOptions(opts)}
	}
	if isProto3 && fd.Label != nil && fd.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL {
		fd.Proto3Optional = proto.Bool(true)
	}
	return fd
}

var fieldTypes = map[string]descriptorpb.FieldDescriptorProto_Type{
	"double":   descriptorpb.FieldDescriptorProto_TYPE_DOUBLE,
	"float":    descriptorpb.FieldDescriptorProto_TYPE_FLOAT,
	"int32":    descriptorpb.FieldDescriptorProto_TYPE_INT32,
	"int64":    descriptorpb.FieldDescriptorProto_TYPE_INT64,
	"uint32":   descriptorpb.FieldDescriptorProto_TYPE_UINT32,
	"uint64":   descriptorpb.FieldDescriptorProto_TYPE_UINT64,
	"sint32":   descriptorpb.FieldDescriptorProto_TYPE_SINT32,
	"sint64":   descriptorpb.FieldDescriptorProto_TYPE_SINT64,
	"fixed32":  descriptorpb.FieldDescriptorProto_TYPE_FIXED32,
	"fixed64":  descriptorpb.FieldDescriptorProto_TYPE_FIXED64,
	"sfixed32": descriptorpb.FieldDescriptorProto_TYPE_SFIXED32,
	"sfixed64": descriptorpb.FieldDescriptorProto_TYPE_SFIXED64,
	"bool":     descriptorpb.FieldDescriptorProto_TYPE_BOOL,
	"string":   descriptorpb.FieldDescriptorProto_TYPE_STRING,
	"bytes":    descriptorpb.FieldDescriptorProto_TYPE_BYTES,
}

func newFieldDescriptor(name string, fieldType string, tag int32, lbl *descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto {
	fd := &descriptorpb.FieldDescriptorProto{
		Name:     proto.String(name),
		JsonName: proto.String(internal.JSONName(name)),
		Number:   proto.Int32(tag),
		Label:    lbl,
	}
	t, ok := fieldTypes[fieldType]
	if ok {
		fd.Type = t.Enum()
	} else {
		// NB: we don't have enough info to determine whether this is an enum
		// or a message type, so we'll leave Type nil and set it later
		// (during linking)
		fd.TypeName = proto.String(fieldType)
	}
	return fd
}

func (r *result) asGroupDescriptors(group *ast.GroupNode, isProto3 bool, maxTag int32, handler *reporter.Handler, depth int) (*descriptorpb.FieldDescriptorProto, *descriptorpb.DescriptorProto) {
	tag := group.Tag.Val
	if err := r.checkTag(group.Tag, tag, maxTag); err != nil {
		_ = handler.HandleError(err)
	}
	if !unicode.IsUpper(rune(group.Name.Val[0])) {
		nameNodeInfo := r.file.NodeInfo(group.Name)
		_ = handler.HandleErrorf(nameNodeInfo.Start(), "group %s should have a name that starts with a capital letter", group.Name.Val)
	}
	fieldName := strings.ToLower(group.Name.Val)
	fd := &descriptorpb.FieldDescriptorProto{
		Name:     proto.String(fieldName),
		JsonName: proto.String(internal.JSONName(fieldName)),
		Number:   proto.Int32(int32(tag)),
		Label:    asLabel(&group.Label),
		Type:     descriptorpb.FieldDescriptorProto_TYPE_GROUP.Enum(),
		TypeName: proto.String(group.Name.Val),
	}
	r.putFieldNode(fd, group)
	if opts := group.Options.GetElements(); len(opts) > 0 {
		fd.Options = &descriptorpb.FieldOptions{UninterpretedOption: r.asUninterpretedOptions(opts)}
	}
	md := &descriptorpb.DescriptorProto{Name: proto.String(group.Name.Val)}
	r.putMessageNode(md, group)
	// don't bother processing body if we've exceeded depth
	if r.checkDepth(depth, group, handler) {
		r.addMessageBody(md, &group.MessageBody, isProto3, handler, depth)
	}
	return fd, md
}

func (r *result) asMapDescriptors(mapField *ast.MapFieldNode, isProto3 bool, maxTag int32, handler *reporter.Handler, depth int) (*descriptorpb.FieldDescriptorProto, *descriptorpb.DescriptorProto) {
	tag := mapField.Tag.Val
	if err := r.checkTag(mapField.Tag, tag, maxTag); err != nil {
		_ = handler.HandleError(err)
	}
	r.checkDepth(depth, mapField, handler)
	var lbl *descriptorpb.FieldDescriptorProto_Label
	if !isProto3 {
		lbl = descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	}
	keyFd := newFieldDescriptor("key", mapField.MapType.KeyType.Val, 1, lbl)
	r.putFieldNode(keyFd, mapField.KeyField())
	valFd := newFieldDescriptor("value", string(mapField.MapType.ValueType.AsIdentifier()), 2, lbl)
	r.putFieldNode(valFd, mapField.ValueField())
	entryName := internal.InitCap(internal.JSONName(mapField.Name.Val)) + "Entry"
	fd := newFieldDescriptor(mapField.Name.Val, entryName, int32(tag), descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum())
	if opts := mapField.Options.GetElements(); len(opts) > 0 {
		fd.Options = &descriptorpb.FieldOptions{UninterpretedOption: r.asUninterpretedOptions(opts)}
	}
	r.putFieldNode(fd, mapField)
	md := &descriptorpb.DescriptorProto{
		Name:    proto.String(entryName),
		Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
		Field:   []*descriptorpb.FieldDescriptorProto{keyFd, valFd},
	}
	r.putMessageNode(md, mapField)
	return fd, md
}

func (r *result) asExtensionRanges(node *ast.ExtensionRangeNode, maxTag int32, handler *reporter.Handler) []*descriptorpb.DescriptorProto_ExtensionRange {
	opts := r.asUninterpretedOptions(node.Options.GetElements())
	ers := make([]*descriptorpb.DescriptorProto_ExtensionRange, len(node.Ranges))
	for i, rng := range node.Ranges {
		start, end := r.getRangeBounds(rng, 1, maxTag, handler)
		er := &descriptorpb.DescriptorProto_ExtensionRange{
			Start: proto.Int32(start),
			End:   proto.Int32(end + 1),
		}
		if len(opts) > 0 {
			er.Options = &descriptorpb.ExtensionRangeOptions{UninterpretedOption: opts}
		}
		r.putExtensionRangeNode(er, rng)
		ers[i] = er
	}
	return ers
}

func (r *result) asEnumValue(ev *ast.EnumValueNode, handler *reporter.Handler) *descriptorpb.EnumValueDescriptorProto {
	num, ok := ast.AsInt32(ev.Number, math.MinInt32, math.MaxInt32)
	if !ok {
		numberNodeInfo := r.file.NodeInfo(ev.Number)
		_ = handler.HandleErrorf(numberNodeInfo.Start(), "value %d is out of range: should be between %d and %d", ev.Number.Value(), math.MinInt32, math.MaxInt32)
	}
	evd := &descriptorpb.EnumValueDescriptorProto{Name: proto.String(ev.Name.Val), Number: proto.Int32(num)}
	r.putEnumValueNode(evd, ev)
	if opts := ev.Options.GetElements(); len(opts) > 0 {
		evd.Options = &descriptorpb.EnumValueOptions{UninterpretedOption: r.asUninterpretedOptions(opts)}
	}
	return evd
}

func (r *result) asMethodDescriptor(node *ast.RPCNode) *descriptorpb.MethodDescriptorProto {
	md := &descriptorpb.MethodDescriptorProto{
		Name:       proto.String(node.Name.Val),
		InputType:  proto.String(string(node.Input.MessageType.AsIdentifier())),
		OutputType: proto.String(string(node.Output.MessageType.AsIdentifier())),
	}
	r.putMethodNode(md, node)
	if node.Input.Stream != nil {
		md.ClientStreaming = proto.Bool(true)
	}
	if node.Output.Stream != nil {
		md.ServerStreaming = proto.Bool(true)
	}
	// protoc always adds a MethodOptions if there are brackets
	// We do the same to match protoc as closely as possible
	// https://github.com/protocolbuffers/protobuf/blob/0c3f43a6190b77f1f68b7425d1b7e1a8257a8d0c/src/google/protobuf/compiler/parser.cc#L2152
	if node.OpenBrace != nil {
		md.Options = &descriptorpb.MethodOptions{}
		for _, decl := range node.Decls {
			if option, ok := decl.(*ast.OptionNode); ok {
				md.Options.UninterpretedOption = append(md.Options.UninterpretedOption, r.asUninterpretedOption(option))
			}
		}
	}
	return md
}

func (r *result) asEnumDescriptor(en *ast.EnumNode, handler *reporter.Handler) *descriptorpb.EnumDescriptorProto {
	ed := &descriptorpb.EnumDescriptorProto{Name: proto.String(en.Name.Val)}
	r.putEnumNode(ed, en)
	for _, decl := range en.Decls {
		switch decl := decl.(type) {
		case *ast.OptionNode:
			if ed.Options == nil {
				ed.Options = &descriptorpb.EnumOptions{}
			}
			ed.Options.UninterpretedOption = append(ed.Options.UninterpretedOption, r.asUninterpretedOption(decl))
		case *ast.EnumValueNode:
			ed.Value = append(ed.Value, r.asEnumValue(decl, handler))
		case *ast.ReservedNode:
			for _, n := range decl.Names {
				ed.ReservedName = append(ed.ReservedName, n.AsString())
			}
			for _, rng := range decl.Ranges {
				ed.ReservedRange = append(ed.ReservedRange, r.asEnumReservedRange(rng, handler))
			}
		}
	}
	return ed
}

func (r *result) asEnumReservedRange(rng *ast.RangeNode, handler *reporter.Handler) *descriptorpb.EnumDescriptorProto_EnumReservedRange {
	start, end := r.getRangeBounds(rng, math.MinInt32, math.MaxInt32, handler)
	rr := &descriptorpb.EnumDescriptorProto_EnumReservedRange{
		Start: proto.Int32(start),
		End:   proto.Int32(end),
	}
	r.putEnumReservedRangeNode(rr, rng)
	return rr
}

func (r *result) asMessageDescriptor(node *ast.MessageNode, isProto3 bool, handler *reporter.Handler, depth int) *descriptorpb.DescriptorProto {
	msgd := &descriptorpb.DescriptorProto{Name: proto.String(node.Name.Val)}
	r.putMessageNode(msgd, node)
	// don't bother processing body if we've exceeded depth
	if r.checkDepth(depth, node, handler) {
		r.addMessageBody(msgd, &node.MessageBody, isProto3, handler, depth)
	}
	return msgd
}

func (r *result) checkDepth(depth int, node ast.MessageDeclNode, handler *reporter.Handler) bool {
	if depth < 32 {
		return true
	}
	n := ast.Node(node)
	if grp, ok := n.(*ast.GroupNode); ok {
		// pinpoint the group keyword if the source is a group
		n = grp.Keyword
	}
	_ = handler.HandleErrorf(r.file.NodeInfo(n).Start(), "message nesting depth must be less than 32")
	return false
}

func (r *result) addMessageBody(msgd *descriptorpb.DescriptorProto, body *ast.MessageBody, isProto3 bool, handler *reporter.Handler, depth int) {
	// first process any options
	for _, decl := range body.Decls {
		if opt, ok := decl.(*ast.OptionNode); ok {
			if msgd.Options == nil {
				msgd.Options = &descriptorpb.MessageOptions{}
			}
			msgd.Options.UninterpretedOption = append(msgd.Options.UninterpretedOption, r.asUninterpretedOption(opt))
		}
	}

	// now that we have options, we can see if this uses messageset wire format, which
	// impacts how we validate tag numbers in any fields in the message
	maxTag := int32(internal.MaxNormalTag)
	messageSetOpt, err := r.isMessageSetWireFormat("message "+msgd.GetName(), msgd, handler)
	if err != nil {
		return
	} else if messageSetOpt != nil {
		if isProto3 {
			node := r.OptionNode(messageSetOpt)
			nodeInfo := r.file.NodeInfo(node)
			_ = handler.HandleErrorf(nodeInfo.Start(), "messages with message-set wire format are not allowed with proto3 syntax")
		}
		maxTag = internal.MaxTag // higher limit for messageset wire format
	}

	rsvdNames := map[string]int{}

	// now we can process the rest
	for _, decl := range body.Decls {
		switch decl := decl.(type) {
		case *ast.EnumNode:
			msgd.EnumType = append(msgd.EnumType, r.asEnumDescriptor(decl, handler))
		case *ast.ExtendNode:
			r.addExtensions(decl, &msgd.Extension, &msgd.NestedType, isProto3, handler, depth)
		case *ast.ExtensionRangeNode:
			msgd.ExtensionRange = append(msgd.ExtensionRange, r.asExtensionRanges(decl, maxTag, handler)...)
		case *ast.FieldNode:
			fd := r.asFieldDescriptor(decl, maxTag, isProto3, handler)
			msgd.Field = append(msgd.Field, fd)
		case *ast.MapFieldNode:
			fd, md := r.asMapDescriptors(decl, isProto3, maxTag, handler, depth+1)
			msgd.Field = append(msgd.Field, fd)
			msgd.NestedType = append(msgd.NestedType, md)
		case *ast.GroupNode:
			fd, md := r.asGroupDescriptors(decl, isProto3, maxTag, handler, depth+1)
			msgd.Field = append(msgd.Field, fd)
			msgd.NestedType = append(msgd.NestedType, md)
		case *ast.OneofNode:
			oodIndex := len(msgd.OneofDecl)
			ood := &descriptorpb.OneofDescriptorProto{Name: proto.String(decl.Name.Val)}
			r.putOneofNode(ood, decl)
			msgd.OneofDecl = append(msgd.OneofDecl, ood)
			ooFields := 0
			for _, oodecl := range decl.Decls {
				switch oodecl := oodecl.(type) {
				case *ast.OptionNode:
					if ood.Options == nil {
						ood.Options = &descriptorpb.OneofOptions{}
					}
					ood.Options.UninterpretedOption = append(ood.Options.UninterpretedOption, r.asUninterpretedOption(oodecl))
				case *ast.FieldNode:
					fd := r.asFieldDescriptor(oodecl, maxTag, isProto3, handler)
					fd.OneofIndex = proto.Int32(int32(oodIndex))
					msgd.Field = append(msgd.Field, fd)
					ooFields++
				case *ast.GroupNode:
					fd, md := r.asGroupDescriptors(oodecl, isProto3, maxTag, handler, depth+1)
					fd.OneofIndex = proto.Int32(int32(oodIndex))
					msgd.Field = append(msgd.Field, fd)
					msgd.NestedType = append(msgd.NestedType, md)
					ooFields++
				}
			}
			if ooFields == 0 {
				declNodeInfo := r.file.NodeInfo(decl)
				_ = handler.HandleErrorf(declNodeInfo.Start(), "oneof must contain at least one field")
			}
		case *ast.MessageNode:
			msgd.NestedType = append(msgd.NestedType, r.asMessageDescriptor(decl, isProto3, handler, depth+1))
		case *ast.ReservedNode:
			for _, n := range decl.Names {
				count := rsvdNames[n.AsString()]
				if count == 1 { // already seen
					nameNodeInfo := r.file.NodeInfo(n)
					_ = handler.HandleErrorf(nameNodeInfo.Start(), "name %q is reserved multiple times", n.AsString())
				}
				rsvdNames[n.AsString()] = count + 1
				msgd.ReservedName = append(msgd.ReservedName, n.AsString())
			}
			for _, rng := range decl.Ranges {
				msgd.ReservedRange = append(msgd.ReservedRange, r.asMessageReservedRange(rng, maxTag, handler))
			}
		}
	}

	if messageSetOpt != nil {
		if len(msgd.Field) > 0 {
			node := r.FieldNode(msgd.Field[0])
			nodeInfo := r.file.NodeInfo(node)
			_ = handler.HandleErrorf(nodeInfo.Start(), "messages with message-set wire format cannot contain non-extension fields")
		}
		if len(msgd.ExtensionRange) == 0 {
			node := r.OptionNode(messageSetOpt)
			nodeInfo := r.file.NodeInfo(node)
			_ = handler.HandleErrorf(nodeInfo.Start(), "messages with message-set wire format must contain at least one extension range")
		}
	}

	// process any proto3_optional fields
	if isProto3 {
		r.processProto3OptionalFields(msgd)
	}
}

func (r *result) isMessageSetWireFormat(scope string, md *descriptorpb.DescriptorProto, handler *reporter.Handler) (*descriptorpb.UninterpretedOption, error) {
	uo := md.GetOptions().GetUninterpretedOption()
	index, err := internal.FindOption(r, handler, scope, uo, "message_set_wire_format")
	if err != nil {
		return nil, err
	}
	if index == -1 {
		// no such option
		return nil, nil
	}

	opt := uo[index]

	switch opt.GetIdentifierValue() {
	case "true":
		return opt, nil
	case "false":
		return nil, nil
	default:
		optNode := r.OptionNode(opt)
		optNodeInfo := r.file.NodeInfo(optNode.GetValue())
		return nil, handler.HandleErrorf(optNodeInfo.Start(), "%s: expecting bool value for message_set_wire_format option", scope)
	}
}

func (r *result) asMessageReservedRange(rng *ast.RangeNode, maxTag int32, handler *reporter.Handler) *descriptorpb.DescriptorProto_ReservedRange {
	start, end := r.getRangeBounds(rng, 1, maxTag, handler)
	rr := &descriptorpb.DescriptorProto_ReservedRange{
		Start: proto.Int32(start),
		End:   proto.Int32(end + 1),
	}
	r.putMessageReservedRangeNode(rr, rng)
	return rr
}

func (r *result) getRangeBounds(rng *ast.RangeNode, minVal, maxVal int32, handler *reporter.Handler) (int32, int32) {
	checkOrder := true
	start, ok := rng.StartValueAsInt32(minVal, maxVal)
	if !ok {
		checkOrder = false
		startValNodeInfo := r.file.NodeInfo(rng.StartVal)
		_ = handler.HandleErrorf(startValNodeInfo.Start(), "range start %d is out of range: should be between %d and %d", rng.StartValue(), minVal, maxVal)
	}

	end, ok := rng.EndValueAsInt32(minVal, maxVal)
	if !ok {
		checkOrder = false
		if rng.EndVal != nil {
			endValNodeInfo := r.file.NodeInfo(rng.EndVal)
			_ = handler.HandleErrorf(endValNodeInfo.Start(), "range end %d is out of range: should be between %d and %d", rng.EndValue(), minVal, maxVal)
		}
	}

	if checkOrder && start > end {
		rangeStartNodeInfo := r.file.NodeInfo(rng.RangeStart())
		_ = handler.HandleErrorf(rangeStartNodeInfo.Start(), "range, %d to %d, is invalid: start must be <= end", start, end)
	}

	return start, end
}

func (r *result) asServiceDescriptor(svc *ast.ServiceNode) *descriptorpb.ServiceDescriptorProto {
	sd := &descriptorpb.ServiceDescriptorProto{Name: proto.String(svc.Name.Val)}
	r.putServiceNode(sd, svc)
	for _, decl := range svc.Decls {
		switch decl := decl.(type) {
		case *ast.OptionNode:
			if sd.Options == nil {
				sd.Options = &descriptorpb.ServiceOptions{}
			}
			sd.Options.UninterpretedOption = append(sd.Options.UninterpretedOption, r.asUninterpretedOption(decl))
		case *ast.RPCNode:
			sd.Method = append(sd.Method, r.asMethodDescriptor(decl))
		}
	}
	return sd
}

func (r *result) checkTag(n ast.Node, v uint64, maxTag int32) error {
	switch {
	case v < 1:
		return reporter.Errorf(r.file.NodeInfo(n).Start(), "tag number %d must be greater than zero", v)
	case v > uint64(maxTag):
		return reporter.Errorf(r.file.NodeInfo(n).Start(), "tag number %d is higher than max allowed tag number (%d)", v, maxTag)
	case v >= internal.SpecialReservedStart && v <= internal.SpecialReservedEnd:
		return reporter.Errorf(r.file.NodeInfo(n).Start(), "tag number %d is in disallowed reserved range %d-%d", v, internal.SpecialReservedStart, internal.SpecialReservedEnd)
	default:
		return nil
	}
}

// processProto3OptionalFields adds synthetic oneofs to the given message descriptor
// for each proto3 optional field. It also updates the fields to have the correct
// oneof index reference.
func (r *result) processProto3OptionalFields(msgd *descriptorpb.DescriptorProto) {
	// add synthetic oneofs to the given message descriptor for each proto3
	// optional field, and update each field to have correct oneof index
	var allNames map[string]struct{}
	for _, fd := range msgd.Field {
		if fd.GetProto3Optional() {
			// lazy init the set of all names
			if allNames == nil {
				allNames = map[string]struct{}{}
				for _, fd := range msgd.Field {
					allNames[fd.GetName()] = struct{}{}
				}
				for _, od := range msgd.OneofDecl {
					allNames[od.GetName()] = struct{}{}
				}
				// NB: protoc only considers names of other fields and oneofs
				// when computing the synthetic oneof name. But that feels like
				// a bug, since it means it could generate a name that conflicts
				// with some other symbol defined in the message. If it's decided
				// that's NOT a bug and is desirable, then we should remove the
				// following four loops to mimic protoc's behavior.
				for _, fd := range msgd.Extension {
					allNames[fd.GetName()] = struct{}{}
				}
				for _, ed := range msgd.EnumType {
					allNames[ed.GetName()] = struct{}{}
					for _, evd := range ed.Value {
						allNames[evd.GetName()] = struct{}{}
					}
				}
				for _, fd := range msgd.NestedType {
					allNames[fd.GetName()] = struct{}{}
				}
			}

			// Compute a name for the synthetic oneof. This uses the same
			// algorithm as used in protoc:
			//  https://github.com/protocolbuffers/protobuf/blob/74ad62759e0a9b5a21094f3fb9bb4ebfaa0d1ab8/src/google/protobuf/compiler/parser.cc#L785-L803
			ooName := fd.GetName()
			if !strings.HasPrefix(ooName, "_") {
				ooName = "_" + ooName
			}
			for {
				_, ok := allNames[ooName]
				if !ok {
					// found a unique name
					allNames[ooName] = struct{}{}
					break
				}
				ooName = "X" + ooName
			}

			fd.OneofIndex = proto.Int32(int32(len(msgd.OneofDecl)))
			ood := &descriptorpb.OneofDescriptorProto{Name: proto.String(ooName)}
			msgd.OneofDecl = append(msgd.OneofDecl, ood)
			ooident := r.FieldNode(fd).(*ast.FieldNode) //nolint:errcheck
			r.putOneofNode(ood, ast.NewSyntheticOneof(ooident))
		}
	}
}

func (r *result) Node(m proto.Message) ast.Node {
	if r.nodes == nil {
		return ast.NewNoSourceNode(r.proto.GetName())
	}
	return r.nodes[m]
}

func (r *result) FileNode() ast.FileDeclNode {
	if r.nodes == nil {
		return ast.NewNoSourceNode(r.proto.GetName())
	}
	return r.nodes[r.proto].(ast.FileDeclNode)
}

func (r *result) OptionNode(o *descriptorpb.UninterpretedOption) ast.OptionDeclNode {
	if r.nodes == nil {
		return ast.NewNoSourceNode(r.proto.GetName())
	}
	return r.nodes[o].(ast.OptionDeclNode)
}

func (r *result) OptionNamePartNode(o *descriptorpb.UninterpretedOption_NamePart) ast.Node {
	if r.nodes == nil {
		return ast.NewNoSourceNode(r.proto.GetName())
	}
	return r.nodes[o]
}

func (r *result) MessageNode(m *descriptorpb.DescriptorProto) ast.MessageDeclNode {
	if r.nodes == nil {
		return ast.NewNoSourceNode(r.proto.GetName())
	}
	return r.nodes[m].(ast.MessageDeclNode)
}

func (r *result) FieldNode(f *descriptorpb.FieldDescriptorProto) ast.FieldDeclNode {
	if r.nodes == nil {
		return ast.NewNoSourceNode(r.proto.GetName())
	}
	return r.nodes[f].(ast.FieldDeclNode)
}

func (r *result) OneofNode(o *descriptorpb.OneofDescriptorProto) ast.Node {
	if r.nodes == nil {
		return ast.NewNoSourceNode(r.proto.GetName())
	}
	return r.nodes[o]
}

func (r *result) ExtensionRangeNode(e *descriptorpb.DescriptorProto_ExtensionRange) ast.RangeDeclNode {
	if r.nodes == nil {
		return ast.NewNoSourceNode(r.proto.GetName())
	}
	return r.nodes[e].(ast.RangeDeclNode)
}

func (r *result) MessageReservedRangeNode(rr *descriptorpb.DescriptorProto_ReservedRange) ast.RangeDeclNode {
	if r.nodes == nil {
		return ast.NewNoSourceNode(r.proto.GetName())
	}
	return r.nodes[rr].(ast.RangeDeclNode)
}

func (r *result) EnumNode(e *descriptorpb.EnumDescriptorProto) ast.Node {
	if r.nodes == nil {
		return ast.NewNoSourceNode(r.proto.GetName())
	}
	return r.nodes[e]
}

func (r *result) EnumValueNode(e *descriptorpb.EnumValueDescriptorProto) ast.EnumValueDeclNode {
	if r.nodes == nil {
		return ast.NewNoSourceNode(r.proto.GetName())
	}
	return r.nodes[e].(ast.EnumValueDeclNode)
}

func (r *result) EnumReservedRangeNode(rr *descriptorpb.EnumDescriptorProto_EnumReservedRange) ast.RangeDeclNode {
	if r.nodes == nil {
		return ast.NewNoSourceNode(r.proto.GetName())
	}
	return r.nodes[rr].(ast.RangeDeclNode)
}

func (r *result) ServiceNode(s *descriptorpb.ServiceDescriptorProto) ast.Node {
	if r.nodes == nil {
		return ast.NewNoSourceNode(r.proto.GetName())
	}
	return r.nodes[s]
}

func (r *result) MethodNode(m *descriptorpb.MethodDescriptorProto) ast.RPCDeclNode {
	if r.nodes == nil {
		return ast.NewNoSourceNode(r.proto.GetName())
	}
	return r.nodes[m].(ast.RPCDeclNode)
}

func (r *result) putFileNode(f *descriptorpb.FileDescriptorProto, n *ast.FileNode) {
	r.nodes[f] = n
}

func (r *result) putOptionNode(o *descriptorpb.UninterpretedOption, n *ast.OptionNode) {
	r.nodes[o] = n
}

func (r *result) putOptionNamePartNode(o *descriptorpb.UninterpretedOption_NamePart, n *ast.FieldReferenceNode) {
	r.nodes[o] = n
}

func (r *result) putMessageNode(m *descriptorpb.DescriptorProto, n ast.MessageDeclNode) {
	r.nodes[m] = n
}

func (r *result) putFieldNode(f *descriptorpb.FieldDescriptorProto, n ast.FieldDeclNode) {
	r.nodes[f] = n
}

func (r *result) putOneofNode(o *descriptorpb.OneofDescriptorProto, n ast.OneofDeclNode) {
	r.nodes[o] = n
}

func (r *result) putExtensionRangeNode(e *descriptorpb.DescriptorProto_ExtensionRange, n *ast.RangeNode) {
	r.nodes[e] = n
}

func (r *result) putMessageReservedRangeNode(rr *descriptorpb.DescriptorProto_ReservedRange, n *ast.RangeNode) {
	r.nodes[rr] = n
}

func (r *result) putEnumNode(e *descriptorpb.EnumDescriptorProto, n *ast.EnumNode) {
	r.nodes[e] = n
}

func (r *result) putEnumValueNode(e *descriptorpb.EnumValueDescriptorProto, n *ast.EnumValueNode) {
	r.nodes[e] = n
}

func (r *result) putEnumReservedRangeNode(rr *descriptorpb.EnumDescriptorProto_EnumReservedRange, n *ast.RangeNode) {
	r.nodes[rr] = n
}

func (r *result) putServiceNode(s *descriptorpb.ServiceDescriptorProto, n *ast.ServiceNode) {
	r.nodes[s] = n
}

func (r *result) putMethodNode(m *descriptorpb.MethodDescriptorProto, n *ast.RPCNode) {
	r.nodes[m] = n
}

// NB: If we ever add other put*Node methods, to index other kinds of elements in the descriptor
//     proto hierarchy, we need to update the index recreation logic in clone.go, too.
