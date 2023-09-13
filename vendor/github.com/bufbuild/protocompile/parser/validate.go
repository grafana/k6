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
	"fmt"
	"sort"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/internal"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/bufbuild/protocompile/walk"
)

func validateBasic(res *result, handler *reporter.Handler) {
	fd := res.proto
	isProto3 := fd.GetSyntax() == "proto3"

	if err := validateImports(res, handler); err != nil {
		return
	}

	_ = walk.DescriptorProtos(fd,
		func(name protoreflect.FullName, d proto.Message) error {
			switch d := d.(type) {
			case *descriptorpb.DescriptorProto:
				if err := validateMessage(res, isProto3, name, d, handler); err != nil {
					// exit func is not called when enter returns error
					return err
				}
			case *descriptorpb.EnumDescriptorProto:
				if err := validateEnum(res, isProto3, name, d, handler); err != nil {
					return err
				}
			case *descriptorpb.FieldDescriptorProto:
				if err := validateField(res, isProto3, name, d, handler); err != nil {
					return err
				}
			}
			return nil
		})
}

func validateImports(res *result, handler *reporter.Handler) error {
	fileNode := res.file
	if fileNode == nil {
		return nil
	}
	imports := make(map[string]ast.SourcePos)
	for _, decl := range fileNode.Decls {
		imp, ok := decl.(*ast.ImportNode)
		if !ok {
			continue
		}
		startPos := fileNode.NodeInfo(decl).Start()
		name := imp.Name.AsString()
		if prev, ok := imports[name]; ok {
			return handler.HandleErrorf(startPos, "%q was already imported at %v", name, prev)
		}
		imports[name] = startPos
	}
	return nil
}

func validateMessage(res *result, isProto3 bool, name protoreflect.FullName, md *descriptorpb.DescriptorProto, handler *reporter.Handler) error {
	scope := fmt.Sprintf("message %s", name)

	if isProto3 && len(md.ExtensionRange) > 0 {
		n := res.ExtensionRangeNode(md.ExtensionRange[0])
		nInfo := res.file.NodeInfo(n)
		if err := handler.HandleErrorf(nInfo.Start(), "%s: extension ranges are not allowed in proto3", scope); err != nil {
			return err
		}
	}

	if index, err := internal.FindOption(res, handler, scope, md.Options.GetUninterpretedOption(), "map_entry"); err != nil {
		return err
	} else if index >= 0 {
		opt := md.Options.UninterpretedOption[index]
		optn := res.OptionNode(opt)
		md.Options.UninterpretedOption = internal.RemoveOption(md.Options.UninterpretedOption, index)
		valid := false
		if opt.IdentifierValue != nil {
			if opt.GetIdentifierValue() == "true" {
				valid = true
				optionNodeInfo := res.file.NodeInfo(optn.GetValue())
				if err := handler.HandleErrorf(optionNodeInfo.Start(), "%s: map_entry option should not be set explicitly; use map type instead", scope); err != nil {
					return err
				}
			} else if opt.GetIdentifierValue() == "false" {
				valid = true
				md.Options.MapEntry = proto.Bool(false)
			}
		}
		if !valid {
			optionNodeInfo := res.file.NodeInfo(optn.GetValue())
			if err := handler.HandleErrorf(optionNodeInfo.Start(), "%s: expecting bool value for map_entry option", scope); err != nil {
				return err
			}
		}
	}

	// reserved ranges should not overlap
	rsvd := make(tagRanges, len(md.ReservedRange))
	for i, r := range md.ReservedRange {
		n := res.MessageReservedRangeNode(r)
		rsvd[i] = tagRange{start: r.GetStart(), end: r.GetEnd(), node: n}
	}
	sort.Sort(rsvd)
	for i := 1; i < len(rsvd); i++ {
		if rsvd[i].start < rsvd[i-1].end {
			rangeNodeInfo := res.file.NodeInfo(rsvd[i].node)
			if err := handler.HandleErrorf(rangeNodeInfo.Start(), "%s: reserved ranges overlap: %d to %d and %d to %d", scope, rsvd[i-1].start, rsvd[i-1].end-1, rsvd[i].start, rsvd[i].end-1); err != nil {
				return err
			}
		}
	}

	// extensions ranges should not overlap
	exts := make(tagRanges, len(md.ExtensionRange))
	for i, r := range md.ExtensionRange {
		n := res.ExtensionRangeNode(r)
		exts[i] = tagRange{start: r.GetStart(), end: r.GetEnd(), node: n}
	}
	sort.Sort(exts)
	for i := 1; i < len(exts); i++ {
		if exts[i].start < exts[i-1].end {
			rangeNodeInfo := res.file.NodeInfo(exts[i].node)
			if err := handler.HandleErrorf(rangeNodeInfo.Start(), "%s: extension ranges overlap: %d to %d and %d to %d", scope, exts[i-1].start, exts[i-1].end-1, exts[i].start, exts[i].end-1); err != nil {
				return err
			}
		}
	}

	// see if any extension range overlaps any reserved range
	var i, j int // i indexes rsvd; j indexes exts
	for i < len(rsvd) && j < len(exts) {
		if rsvd[i].start >= exts[j].start && rsvd[i].start < exts[j].end ||
			exts[j].start >= rsvd[i].start && exts[j].start < rsvd[i].end {
			var pos ast.SourcePos
			if rsvd[i].start >= exts[j].start && rsvd[i].start < exts[j].end {
				rangeNodeInfo := res.file.NodeInfo(rsvd[i].node)
				pos = rangeNodeInfo.Start()
			} else {
				rangeNodeInfo := res.file.NodeInfo(exts[j].node)
				pos = rangeNodeInfo.Start()
			}
			// ranges overlap
			if err := handler.HandleErrorf(pos, "%s: extension range %d to %d overlaps reserved range %d to %d", scope, exts[j].start, exts[j].end-1, rsvd[i].start, rsvd[i].end-1); err != nil {
				return err
			}
		}
		if rsvd[i].start < exts[j].start {
			i++
		} else {
			j++
		}
	}

	// now, check that fields don't re-use tags and don't try to use extension
	// or reserved ranges or reserved names
	rsvdNames := map[string]struct{}{}
	for _, n := range md.ReservedName {
		// validate reserved name while we're here
		if !isIdentifier(n) {
			node := findMessageReservedNameNode(res.MessageNode(md), n)
			nodeInfo := res.file.NodeInfo(node)
			if err := handler.HandleErrorf(nodeInfo.Start(), "%s: reserved name %q is not a valid identifier", scope, n); err != nil {
				return err
			}
		}
		rsvdNames[n] = struct{}{}
	}
	fieldTags := map[int32]string{}
	for _, fld := range md.Field {
		fn := res.FieldNode(fld)
		if _, ok := rsvdNames[fld.GetName()]; ok {
			fieldNameNodeInfo := res.file.NodeInfo(fn.FieldName())
			if err := handler.HandleErrorf(fieldNameNodeInfo.Start(), "%s: field %s is using a reserved name", scope, fld.GetName()); err != nil {
				return err
			}
		}
		if existing := fieldTags[fld.GetNumber()]; existing != "" {
			fieldTagNodeInfo := res.file.NodeInfo(fn.FieldTag())
			if err := handler.HandleErrorf(fieldTagNodeInfo.Start(), "%s: fields %s and %s both have the same tag %d", scope, existing, fld.GetName(), fld.GetNumber()); err != nil {
				return err
			}
		}
		fieldTags[fld.GetNumber()] = fld.GetName()
		// check reserved ranges
		r := sort.Search(len(rsvd), func(index int) bool { return rsvd[index].end > fld.GetNumber() })
		if r < len(rsvd) && rsvd[r].start <= fld.GetNumber() {
			fieldTagNodeInfo := res.file.NodeInfo(fn.FieldTag())
			if err := handler.HandleErrorf(fieldTagNodeInfo.Start(), "%s: field %s is using tag %d which is in reserved range %d to %d", scope, fld.GetName(), fld.GetNumber(), rsvd[r].start, rsvd[r].end-1); err != nil {
				return err
			}
		}
		// and check extension ranges
		e := sort.Search(len(exts), func(index int) bool { return exts[index].end > fld.GetNumber() })
		if e < len(exts) && exts[e].start <= fld.GetNumber() {
			fieldTagNodeInfo := res.file.NodeInfo(fn.FieldTag())
			if err := handler.HandleErrorf(fieldTagNodeInfo.Start(), "%s: field %s is using tag %d which is in extension range %d to %d", scope, fld.GetName(), fld.GetNumber(), exts[e].start, exts[e].end-1); err != nil {
				return err
			}
		}
	}

	return nil
}

func isIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, r := range s {
		if i == 0 && r >= '0' && r <= '9' {
			// can't start with number
			return false
		}
		// alphanumeric and underscore ok; everything else bad
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r == '_':
		default:
			return false
		}
	}
	return true
}

func findMessageReservedNameNode(msgNode ast.MessageDeclNode, name string) ast.Node {
	var decls []ast.MessageElement
	switch msgNode := msgNode.(type) {
	case *ast.MessageNode:
		decls = msgNode.Decls
	case *ast.GroupNode:
		decls = msgNode.Decls
	default:
		// leave decls empty
	}
	return findReservedNameNode(msgNode, decls, name)
}

func findReservedNameNode[T ast.Node](parent ast.Node, decls []T, name string) ast.Node {
	for _, decl := range decls {
		// NB: We have to convert to empty interface first, before we can do a type
		// assertion because type assertions on type parameters aren't allowed. (The
		// compiler cannot yet know whether T is an interface type or not.)
		rsvd, ok := any(decl).(*ast.ReservedNode)
		if !ok {
			continue
		}
		for _, rsvdName := range rsvd.Names {
			if rsvdName.AsString() == name {
				return rsvdName
			}
		}
	}
	// couldn't find it? Instead of puking, report position of the parent.
	return parent
}

func validateEnum(res *result, isProto3 bool, name protoreflect.FullName, ed *descriptorpb.EnumDescriptorProto, handler *reporter.Handler) error {
	scope := fmt.Sprintf("enum %s", name)

	if len(ed.Value) == 0 {
		enNode := res.EnumNode(ed)
		enNodeInfo := res.file.NodeInfo(enNode)
		if err := handler.HandleErrorf(enNodeInfo.Start(), "%s: enums must define at least one value", scope); err != nil {
			return err
		}
	}

	allowAlias := false
	var allowAliasOpt *descriptorpb.UninterpretedOption
	if index, err := internal.FindOption(res, handler, scope, ed.Options.GetUninterpretedOption(), "allow_alias"); err != nil {
		return err
	} else if index >= 0 {
		allowAliasOpt = ed.Options.UninterpretedOption[index]
		valid := false
		if allowAliasOpt.IdentifierValue != nil {
			if allowAliasOpt.GetIdentifierValue() == "true" {
				allowAlias = true
				valid = true
			} else if allowAliasOpt.GetIdentifierValue() == "false" {
				valid = true
			}
		}
		if !valid {
			optNode := res.OptionNode(allowAliasOpt)
			optNodeInfo := res.file.NodeInfo(optNode.GetValue())
			if err := handler.HandleErrorf(optNodeInfo.Start(), "%s: expecting bool value for allow_alias option", scope); err != nil {
				return err
			}
		}
	}

	if isProto3 && len(ed.Value) > 0 && ed.Value[0].GetNumber() != 0 {
		evNode := res.EnumValueNode(ed.Value[0])
		evNodeInfo := res.file.NodeInfo(evNode.GetNumber())
		if err := handler.HandleErrorf(evNodeInfo.Start(), "%s: proto3 requires that first value in enum have numeric value of 0", scope); err != nil {
			return err
		}
	}

	// check for aliases
	vals := map[int32]string{}
	hasAlias := false
	for _, evd := range ed.Value {
		existing := vals[evd.GetNumber()]
		if existing != "" {
			if allowAlias {
				hasAlias = true
			} else {
				evNode := res.EnumValueNode(evd)
				evNodeInfo := res.file.NodeInfo(evNode.GetNumber())
				if err := handler.HandleErrorf(evNodeInfo.Start(), "%s: values %s and %s both have the same numeric value %d; use allow_alias option if intentional", scope, existing, evd.GetName(), evd.GetNumber()); err != nil {
					return err
				}
			}
		}
		vals[evd.GetNumber()] = evd.GetName()
	}
	if allowAlias && !hasAlias {
		optNode := res.OptionNode(allowAliasOpt)
		optNodeInfo := res.file.NodeInfo(optNode.GetValue())
		if err := handler.HandleErrorf(optNodeInfo.Start(), "%s: allow_alias is true but no values are aliases", scope); err != nil {
			return err
		}
	}

	// reserved ranges should not overlap
	rsvd := make(tagRanges, len(ed.ReservedRange))
	for i, r := range ed.ReservedRange {
		n := res.EnumReservedRangeNode(r)
		rsvd[i] = tagRange{start: r.GetStart(), end: r.GetEnd(), node: n}
	}
	sort.Sort(rsvd)
	for i := 1; i < len(rsvd); i++ {
		if rsvd[i].start <= rsvd[i-1].end {
			rangeNodeInfo := res.file.NodeInfo(rsvd[i].node)
			if err := handler.HandleErrorf(rangeNodeInfo.Start(), "%s: reserved ranges overlap: %d to %d and %d to %d", scope, rsvd[i-1].start, rsvd[i-1].end, rsvd[i].start, rsvd[i].end); err != nil {
				return err
			}
		}
	}

	// now, check that fields don't re-use tags and don't try to use extension
	// or reserved ranges or reserved names
	rsvdNames := map[string]struct{}{}
	for _, n := range ed.ReservedName {
		// validate reserved name while we're here
		if !isIdentifier(n) {
			node := findEnumReservedNameNode(res.EnumNode(ed), n)
			nodeInfo := res.file.NodeInfo(node)
			if err := handler.HandleErrorf(nodeInfo.Start(), "%s: reserved name %q is not a valid identifier", scope, n); err != nil {
				return err
			}
		}
		rsvdNames[n] = struct{}{}
	}
	for _, ev := range ed.Value {
		evn := res.EnumValueNode(ev)
		if _, ok := rsvdNames[ev.GetName()]; ok {
			enumValNodeInfo := res.file.NodeInfo(evn.GetName())
			if err := handler.HandleErrorf(enumValNodeInfo.Start(), "%s: value %s is using a reserved name", scope, ev.GetName()); err != nil {
				return err
			}
		}
		// check reserved ranges
		r := sort.Search(len(rsvd), func(index int) bool { return rsvd[index].end >= ev.GetNumber() })
		if r < len(rsvd) && rsvd[r].start <= ev.GetNumber() {
			enumValNodeInfo := res.file.NodeInfo(evn.GetNumber())
			if err := handler.HandleErrorf(enumValNodeInfo.Start(), "%s: value %s is using number %d which is in reserved range %d to %d", scope, ev.GetName(), ev.GetNumber(), rsvd[r].start, rsvd[r].end); err != nil {
				return err
			}
		}
	}

	return nil
}

func findEnumReservedNameNode(enumNode ast.Node, name string) ast.Node {
	var decls []ast.EnumElement
	if enumNode, ok := enumNode.(*ast.EnumNode); ok {
		decls = enumNode.Decls
		// if not the right type, we leave decls empty
	}
	return findReservedNameNode(enumNode, decls, name)
}

func validateField(res *result, isProto3 bool, name protoreflect.FullName, fld *descriptorpb.FieldDescriptorProto, handler *reporter.Handler) error {
	scope := fmt.Sprintf("field %s", name)

	node := res.FieldNode(fld)
	if isProto3 {
		if fld.GetType() == descriptorpb.FieldDescriptorProto_TYPE_GROUP {
			groupNodeInfo := res.file.NodeInfo(node.GetGroupKeyword())
			if err := handler.HandleErrorf(groupNodeInfo.Start(), "%s: groups are not allowed in proto3", scope); err != nil {
				return err
			}
		} else if fld.Label != nil && fld.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REQUIRED {
			fieldLabelNodeInfo := res.file.NodeInfo(node.FieldLabel())
			if err := handler.HandleErrorf(fieldLabelNodeInfo.Start(), "%s: label 'required' is not allowed in proto3", scope); err != nil {
				return err
			}
		}
		if index, err := internal.FindOption(res, handler, scope, fld.Options.GetUninterpretedOption(), "default"); err != nil {
			return err
		} else if index >= 0 {
			optNode := res.OptionNode(fld.Options.GetUninterpretedOption()[index])
			optNameNodeInfo := res.file.NodeInfo(optNode.GetName())
			if err := handler.HandleErrorf(optNameNodeInfo.Start(), "%s: default values are not allowed in proto3", scope); err != nil {
				return err
			}
		}
	} else {
		if fld.Label == nil && fld.OneofIndex == nil {
			fieldNameNodeInfo := res.file.NodeInfo(node.FieldName())
			if err := handler.HandleErrorf(fieldNameNodeInfo.Start(), "%s: field has no label; proto2 requires explicit 'optional' label", scope); err != nil {
				return err
			}
		}
		if fld.GetExtendee() != "" && fld.Label != nil && fld.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REQUIRED {
			fieldLabelNodeInfo := res.file.NodeInfo(node.FieldLabel())
			if err := handler.HandleErrorf(fieldLabelNodeInfo.Start(), "%s: extension fields cannot be 'required'", scope); err != nil {
				return err
			}
		}
	}

	return nil
}

type tagRange struct {
	start int32
	end   int32
	node  ast.RangeDeclNode
}

type tagRanges []tagRange

func (r tagRanges) Len() int {
	return len(r)
}

func (r tagRanges) Less(i, j int) bool {
	return r[i].start < r[j].start ||
		(r[i].start == r[j].start && r[i].end < r[j].end)
}

func (r tagRanges) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func fillInMissingLabels(fd *descriptorpb.FileDescriptorProto) {
	for _, md := range fd.MessageType {
		fillInMissingLabelsInMsg(md)
	}
	for _, extd := range fd.Extension {
		fillInMissingLabel(extd)
	}
}

func fillInMissingLabelsInMsg(md *descriptorpb.DescriptorProto) {
	for _, fld := range md.Field {
		fillInMissingLabel(fld)
	}
	for _, nmd := range md.NestedType {
		fillInMissingLabelsInMsg(nmd)
	}
	for _, extd := range md.Extension {
		fillInMissingLabel(extd)
	}
}

func fillInMissingLabel(fld *descriptorpb.FieldDescriptorProto) {
	if fld.Label == nil {
		fld.Label = descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	}
}
