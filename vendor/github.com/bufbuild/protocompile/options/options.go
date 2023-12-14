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

// Package options contains the logic for interpreting options. The parse step
// of compilation stores the options in uninterpreted form, which contains raw
// identifiers and literal values.
//
// The process of interpreting an option is to resolve identifiers, by examining
// descriptors for the google.protobuf.*Options types and their available
// extensions (custom options). As field names are resolved, the values can be
// type-checked against the types indicated in field descriptors.
//
// On success, the various fields and extensions of the options message are
// populated and the field holding the uninterpreted form is cleared.
package options

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/internal"
	"github.com/bufbuild/protocompile/linker"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/bufbuild/protocompile/sourceinfo"
)

const (
	// featuresFieldName is the name of a field in every options message.
	featuresFieldName = "features"
)

var (
	featureSetType = (*descriptorpb.FeatureSet)(nil).ProtoReflect().Type()
	featureSetName = featureSetType.Descriptor().FullName()
)

type interpreter struct {
	file                    file
	resolver                linker.Resolver
	container               optionsContainer
	overrideDescriptorProto linker.File
	lenient                 bool
	reporter                *reporter.Handler
	index                   sourceinfo.OptionIndex
}

type file interface {
	parser.Result
	ResolveMessageLiteralExtensionName(ast.IdentValueNode) string
}

type noResolveFile struct {
	parser.Result
}

func (n noResolveFile) ResolveMessageLiteralExtensionName(ast.IdentValueNode) string {
	return ""
}

// InterpreterOption is an option that can be passed to InterpretOptions and
// its variants.
type InterpreterOption func(*interpreter)

// WithOverrideDescriptorProto returns an option that indicates that the given file
// should be consulted when looking up a definition for an option type. The given
// file should usually have the path "google/protobuf/descriptor.proto". The given
// file will only be consulted if the option type is otherwise not visible to the
// file whose options are being interpreted.
func WithOverrideDescriptorProto(f linker.File) InterpreterOption {
	return func(interp *interpreter) {
		interp.overrideDescriptorProto = f
	}
}

// InterpretOptions interprets options in the given linked result, returning
// an index that can be used to generate source code info. This step mutates
// the linked result's underlying proto to move option elements out of the
// "uninterpreted_option" fields and into proper option fields and extensions.
//
// The given handler is used to report errors and warnings. If any errors are
// reported, this function returns a non-nil error.
func InterpretOptions(linked linker.Result, handler *reporter.Handler, opts ...InterpreterOption) (sourceinfo.OptionIndex, error) {
	return interpretOptions(false, linked, linker.ResolverFromFile(linked), handler, opts)
}

// InterpretOptionsLenient interprets options in a lenient/best-effort way in
// the given linked result, returning an index that can be used to generate
// source code info. This step mutates the linked result's underlying proto to
// move option elements out of the "uninterpreted_option" fields and into proper
// option fields and extensions.
//
// In lenient more, errors resolving option names and type errors are ignored.
// Any options that are uninterpretable (due to such errors) will remain in the
// "uninterpreted_option" fields.
func InterpretOptionsLenient(linked linker.Result, opts ...InterpreterOption) (sourceinfo.OptionIndex, error) {
	return interpretOptions(true, linked, linker.ResolverFromFile(linked), reporter.NewHandler(nil), opts)
}

// InterpretUnlinkedOptions does a best-effort attempt to interpret options in
// the given parsed result, returning an index that can be used to generate
// source code info. This step mutates the parsed result's underlying proto to
// move option elements out of the "uninterpreted_option" fields and into proper
// option fields and extensions.
//
// This is the same as InterpretOptionsLenient except that it accepts an
// unlinked result. Because the file is unlinked, custom options cannot be
// interpreted. Other errors resolving option names or type errors will be
// effectively ignored. Any options that are uninterpretable (due to such
// errors) will remain in the "uninterpreted_option" fields.
func InterpretUnlinkedOptions(parsed parser.Result, opts ...InterpreterOption) (sourceinfo.OptionIndex, error) {
	return interpretOptions(true, noResolveFile{parsed}, nil, reporter.NewHandler(nil), opts)
}

func interpretOptions(lenient bool, file file, res linker.Resolver, handler *reporter.Handler, interpOpts []InterpreterOption) (sourceinfo.OptionIndex, error) {
	interp := interpreter{
		file:     file,
		resolver: res,
		lenient:  lenient,
		reporter: handler,
		index:    sourceinfo.OptionIndex{},
	}
	interp.container, _ = file.(optionsContainer)
	for _, opt := range interpOpts {
		opt(&interp)
	}

	fd := file.FileDescriptorProto()
	prefix := fd.GetPackage()
	if prefix != "" {
		prefix += "."
	}
	err := interpretElementOptions(&interp, fd.GetName(), targetTypeFile, fd)
	if err != nil {
		return nil, err
	}
	for _, md := range fd.GetMessageType() {
		fqn := prefix + md.GetName()
		if err := interp.interpretMessageOptions(fqn, md); err != nil {
			return nil, err
		}
	}
	for _, fld := range fd.GetExtension() {
		fqn := prefix + fld.GetName()
		if err := interp.interpretFieldOptions(fqn, fld); err != nil {
			return nil, err
		}
	}
	for _, ed := range fd.GetEnumType() {
		fqn := prefix + ed.GetName()
		if err := interp.interpretEnumOptions(fqn, ed); err != nil {
			return nil, err
		}
	}
	for _, sd := range fd.GetService() {
		fqn := prefix + sd.GetName()
		err := interpretElementOptions(&interp, fqn, targetTypeService, sd)
		if err != nil {
			return nil, err
		}
		for _, mtd := range sd.GetMethod() {
			mtdFqn := fqn + "." + mtd.GetName()
			err := interpretElementOptions(&interp, mtdFqn, targetTypeMethod, mtd)
			if err != nil {
				return nil, err
			}
		}
	}
	return interp.index, nil
}

func resolveDescriptor[T protoreflect.Descriptor](res linker.Resolver, name string) T {
	var zero T
	if res == nil {
		return zero
	}
	if len(name) > 0 && name[0] == '.' {
		name = name[1:]
	}
	desc, _ := res.FindDescriptorByName(protoreflect.FullName(name))
	typedDesc, ok := desc.(T)
	if ok {
		return typedDesc
	}
	return zero
}

func (interp *interpreter) resolveExtensionType(name string) (protoreflect.ExtensionTypeDescriptor, error) {
	if interp.resolver == nil {
		return nil, protoregistry.NotFound
	}
	if len(name) > 0 && name[0] == '.' {
		name = name[1:]
	}
	ext, err := interp.resolver.FindExtensionByName(protoreflect.FullName(name))
	if err != nil {
		return nil, err
	}
	return ext.TypeDescriptor(), nil
}

func (interp *interpreter) resolveOptionsType(name string) protoreflect.MessageDescriptor {
	md := resolveDescriptor[protoreflect.MessageDescriptor](interp.resolver, name)
	if md != nil {
		return md
	}
	if interp.overrideDescriptorProto == nil {
		return nil
	}
	if len(name) > 0 && name[0] == '.' {
		name = name[1:]
	}
	desc := interp.overrideDescriptorProto.FindDescriptorByName(protoreflect.FullName(name))
	if md, ok := desc.(protoreflect.MessageDescriptor); ok {
		return md
	}
	return nil
}

func (interp *interpreter) nodeInfo(n ast.Node) ast.NodeInfo {
	return interp.file.FileNode().NodeInfo(n)
}

func (interp *interpreter) interpretMessageOptions(fqn string, md *descriptorpb.DescriptorProto) error {
	err := interpretElementOptions(interp, fqn, targetTypeMessage, md)
	if err != nil {
		return err
	}
	for _, fld := range md.GetField() {
		fldFqn := fqn + "." + fld.GetName()
		if err := interp.interpretFieldOptions(fldFqn, fld); err != nil {
			return err
		}
	}
	for _, ood := range md.GetOneofDecl() {
		oodFqn := fqn + "." + ood.GetName()
		err := interpretElementOptions(interp, oodFqn, targetTypeOneof, ood)
		if err != nil {
			return err
		}
	}
	for _, fld := range md.GetExtension() {
		fldFqn := fqn + "." + fld.GetName()
		if err := interp.interpretFieldOptions(fldFqn, fld); err != nil {
			return err
		}
	}
	for _, er := range md.GetExtensionRange() {
		erFqn := fmt.Sprintf("%s.%d-%d", fqn, er.GetStart(), er.GetEnd())
		err := interpretElementOptions(interp, erFqn, targetTypeExtensionRange, er)
		if err != nil {
			return err
		}
	}
	for _, nmd := range md.GetNestedType() {
		nmdFqn := fqn + "." + nmd.GetName()
		if err := interp.interpretMessageOptions(nmdFqn, nmd); err != nil {
			return err
		}
	}
	for _, ed := range md.GetEnumType() {
		edFqn := fqn + "." + ed.GetName()
		if err := interp.interpretEnumOptions(edFqn, ed); err != nil {
			return err
		}
	}

	// We also copy features for map fields down to their synthesized key and value fields.
	for _, fld := range md.GetField() {
		entryName := internal.InitCap(internal.JSONName(fld.GetName())) + "Entry"
		if fld.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED ||
			fld.GetType() != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE &&
				fld.GetTypeName() != "."+fqn+"."+entryName {
			// can't be a map field
			continue
		}
		if fld.Options == nil || fld.Options.Features == nil {
			// no features to propagate
			continue
		}
		for _, nmd := range md.GetNestedType() {
			if nmd.GetName() == entryName {
				// found the entry message
				if !nmd.GetOptions().GetMapEntry() {
					break // not a map
				}
				for _, mapField := range nmd.Field {
					if mapField.Options == nil {
						mapField.Options = &descriptorpb.FieldOptions{}
					}
					features := proto.Clone(fld.Options.Features).(*descriptorpb.FeatureSet) //nolint:errcheck
					if mapField.Options.Features != nil {
						proto.Merge(features, mapField.Options.Features)
					}
					mapField.Options.Features = features
				}
				break
			}
		}
	}

	return nil
}

var emptyFieldOptions = &descriptorpb.FieldOptions{}

func (interp *interpreter) interpretFieldOptions(fqn string, fld *descriptorpb.FieldDescriptorProto) error {
	opts := fld.GetOptions()
	emptyOptionsAlreadyPresent := opts != nil && len(opts.GetUninterpretedOption()) == 0

	// First process pseudo-options
	if len(opts.GetUninterpretedOption()) > 0 {
		if err := interp.interpretFieldPseudoOptions(fqn, fld, opts); err != nil {
			return err
		}
	}

	// Must re-check length of uninterpreted options since above step could remove some.
	if len(opts.GetUninterpretedOption()) == 0 {
		// If the message has no other interpreted options, we clear it out. But don't
		// do that if the descriptor came in with empty options or if it already has
		// interpreted option fields.
		if opts != nil && !emptyOptionsAlreadyPresent && proto.Equal(fld.Options, emptyFieldOptions) {
			fld.Options = nil
		}
		return nil
	}

	// Then process actual options.
	return interpretElementOptions(interp, fqn, targetTypeField, fld)
}

func (interp *interpreter) interpretFieldPseudoOptions(fqn string, fld *descriptorpb.FieldDescriptorProto, opts *descriptorpb.FieldOptions) error {
	scope := fmt.Sprintf("field %s", fqn)
	uo := opts.UninterpretedOption

	// process json_name pseudo-option
	index, err := internal.FindOption(interp.file, interp.reporter, scope, uo, "json_name")
	if err != nil && !interp.lenient {
		return err
	}
	if index >= 0 {
		opt := uo[index]
		optNode := interp.file.OptionNode(opt)
		if opt.StringValue == nil {
			return interp.reporter.HandleErrorf(interp.nodeInfo(optNode.GetValue()), "%s: expecting string value for json_name option", scope)
		}
		jsonName := string(opt.StringValue)
		// Extensions don't support custom json_name values.
		// If the value is already set (via the descriptor) and doesn't match the default value, return an error.
		if fld.GetExtendee() != "" && jsonName != "" && jsonName != internal.JSONName(fld.GetName()) {
			return interp.reporter.HandleErrorf(interp.nodeInfo(optNode.GetName()), "%s: option json_name is not allowed on extensions", scope)
		}
		// attribute source code info
		if on, ok := optNode.(*ast.OptionNode); ok {
			interp.index[on] = &sourceinfo.OptionSourceInfo{Path: []int32{-1, internal.FieldJSONNameTag}}
		}
		uo = internal.RemoveOption(uo, index)
		if strings.HasPrefix(jsonName, "[") && strings.HasSuffix(jsonName, "]") {
			return interp.reporter.HandleErrorf(interp.nodeInfo(optNode.GetValue()), "%s: option json_name value cannot start with '[' and end with ']'; that is reserved for representing extensions", scope)
		}
		fld.JsonName = proto.String(jsonName)
	}

	// and process default pseudo-option
	if index, err := interp.processDefaultOption(scope, fqn, fld, uo); err != nil && !interp.lenient {
		return err
	} else if index >= 0 {
		// attribute source code info
		optNode := interp.file.OptionNode(uo[index])
		if on, ok := optNode.(*ast.OptionNode); ok {
			interp.index[on] = &sourceinfo.OptionSourceInfo{Path: []int32{-1, internal.FieldDefaultTag}}
		}
		uo = internal.RemoveOption(uo, index)
	}

	opts.UninterpretedOption = uo
	return nil
}

func (interp *interpreter) processDefaultOption(scope string, fqn string, fld *descriptorpb.FieldDescriptorProto, uos []*descriptorpb.UninterpretedOption) (defaultIndex int, err error) {
	found, err := internal.FindOption(interp.file, interp.reporter, scope, uos, "default")
	if err != nil || found == -1 {
		return -1, err
	}
	opt := uos[found]
	optNode := interp.file.OptionNode(opt)
	if fld.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
		return -1, interp.reporter.HandleErrorf(interp.nodeInfo(optNode.GetName()), "%s: default value cannot be set because field is repeated", scope)
	}
	if fld.GetType() == descriptorpb.FieldDescriptorProto_TYPE_GROUP || fld.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
		return -1, interp.reporter.HandleErrorf(interp.nodeInfo(optNode.GetName()), "%s: default value cannot be set because field is a message", scope)
	}
	mc := &internal.MessageContext{
		File:        interp.file,
		ElementName: fqn,
		ElementType: descriptorType(fld),
		Option:      opt,
	}

	val := optNode.GetValue()
	var v interface{}
	if val.Value() == nil {
		// no value in the AST, so we dig the value out of the uninterpreted option proto
		v, err = interp.defaultValueFromProto(mc, fld, opt, val)
	} else {
		// compute value from AST
		v, err = interp.defaultValue(mc, fld, val)
	}
	if err != nil {
		return -1, interp.reporter.HandleError(err)
	}

	if str, ok := v.(string); ok {
		fld.DefaultValue = proto.String(str)
	} else if b, ok := v.([]byte); ok {
		fld.DefaultValue = proto.String(encodeDefaultBytes(b))
	} else {
		var flt float64
		var ok bool
		if flt, ok = v.(float64); !ok {
			var flt32 float32
			if flt32, ok = v.(float32); ok {
				flt = float64(flt32)
			}
		}
		if ok {
			switch {
			case math.IsInf(flt, 1):
				fld.DefaultValue = proto.String("inf")
			case math.IsInf(flt, -1):
				fld.DefaultValue = proto.String("-inf")
			case math.IsNaN(flt):
				fld.DefaultValue = proto.String("nan")
			default:
				fld.DefaultValue = proto.String(fmt.Sprintf("%v", v))
			}
		} else {
			fld.DefaultValue = proto.String(fmt.Sprintf("%v", v))
		}
	}
	return found, nil
}

func (interp *interpreter) defaultValue(mc *internal.MessageContext, fld *descriptorpb.FieldDescriptorProto, val ast.ValueNode) (interface{}, error) {
	if _, ok := val.(*ast.MessageLiteralNode); ok {
		return -1, reporter.Errorf(interp.nodeInfo(val), "%vdefault value cannot be a message", mc)
	}
	if fld.GetType() == descriptorpb.FieldDescriptorProto_TYPE_ENUM {
		ed := resolveDescriptor[protoreflect.EnumDescriptor](interp.resolver, fld.GetTypeName())
		if ed == nil {
			return -1, reporter.Errorf(interp.nodeInfo(val), "%vunable to resolve enum type %q for field %q", mc, fld.GetTypeName(), fld.GetName())
		}
		_, name, err := interp.enumFieldValue(mc, ed, val, false)
		if err != nil {
			return -1, err
		}
		return string(name), nil
	}
	return interp.scalarFieldValue(mc, fld.GetType(), val, false)
}

func (interp *interpreter) defaultValueFromProto(mc *internal.MessageContext, fld *descriptorpb.FieldDescriptorProto, opt *descriptorpb.UninterpretedOption, node ast.Node) (interface{}, error) {
	if opt.AggregateValue != nil {
		return -1, reporter.Errorf(interp.nodeInfo(node), "%vdefault value cannot be a message", mc)
	}
	if fld.GetType() == descriptorpb.FieldDescriptorProto_TYPE_ENUM {
		ed := resolveDescriptor[protoreflect.EnumDescriptor](interp.resolver, fld.GetTypeName())
		if ed == nil {
			return -1, reporter.Errorf(interp.nodeInfo(node), "%vunable to resolve enum type %q for field %q", mc, fld.GetTypeName(), fld.GetName())
		}
		_, name, err := interp.enumFieldValueFromProto(mc, ed, opt, node)
		if err != nil {
			return nil, err
		}
		return string(name), nil
	}
	return interp.scalarFieldValueFromProto(mc, fld.GetType(), opt, node)
}

func encodeDefaultBytes(b []byte) string {
	var buf bytes.Buffer
	internal.WriteEscapedBytes(&buf, b)
	return buf.String()
}

func (interp *interpreter) interpretEnumOptions(fqn string, ed *descriptorpb.EnumDescriptorProto) error {
	err := interpretElementOptions(interp, fqn, targetTypeEnum, ed)
	if err != nil {
		return err
	}
	for _, evd := range ed.GetValue() {
		evdFqn := fqn + "." + evd.GetName()
		err := interpretElementOptions(interp, evdFqn, targetTypeEnumValue, evd)
		if err != nil {
			return err
		}
	}
	return nil
}

// interpretedOption represents the result of interpreting an option.
// This includes metadata that allows the option to be serialized to
// bytes in a way that is deterministic and can preserve the structure
// of the source (the way the options are de-structured and the order in
// which options appear).
type interpretedOption struct {
	unknown    bool
	pathPrefix []int32
	interpretedField
}

func (o *interpretedOption) toSourceInfo() *sourceinfo.OptionSourceInfo {
	return o.interpretedField.toSourceInfo(o.pathPrefix)
}

func (o *interpretedOption) appendOptionBytes(b []byte) ([]byte, error) {
	return o.appendOptionBytesWithPath(b, o.pathPrefix)
}

func (o *interpretedOption) appendOptionBytesWithPath(b []byte, path []int32) ([]byte, error) {
	if len(path) == 0 {
		return appendOptionBytesSingle(b, &o.interpretedField)
	}
	// NB: if we add functions to compute sizes of the options first, we could
	// allocate precisely sized slice up front, which would be more efficient than
	// repeated creation/growing/concatenation.
	enclosed, err := o.appendOptionBytesWithPath(nil, path[1:])
	if err != nil {
		return nil, err
	}
	b = protowire.AppendTag(b, protowire.Number(path[0]), protowire.BytesType)
	return protowire.AppendBytes(b, enclosed), nil
}

// interpretedField represents a field in an options message that is the
// result of interpreting an option. This is used for the option value
// itself as well as for subfields when an option value is a message
// literal.
type interpretedField struct {
	// the AST node for this field -- an [*ast.OptionNode] for top-level options,
	// an [*ast.MessageFieldNode] for fields in a message literal, or nil for
	// synthetic field values (for keys or values in map entries that were
	// omitted from source).
	node ast.Node
	// field number
	number int32
	// index of this element inside a repeated field; only set if repeated == true
	index int32
	// true if this is a repeated field
	repeated bool
	// true if this is a repeated field that stores scalar values in packed form
	packed bool
	// the field's kind
	kind protoreflect.Kind

	value interpretedFieldValue
}

func (f *interpretedField) path(prefix []int32) []int32 {
	path := make([]int32, 0, len(prefix)+2)
	path = append(path, prefix...)
	path = append(path, f.number)
	if f.repeated {
		path = append(path, f.index)
	}
	return path
}

func (f *interpretedField) toSourceInfo(prefix []int32) *sourceinfo.OptionSourceInfo {
	path := f.path(prefix)
	var children sourceinfo.OptionChildrenSourceInfo
	if len(f.value.msgListVal) > 0 {
		elements := make([]sourceinfo.OptionSourceInfo, len(f.value.msgListVal))
		for i, msgVal := range f.value.msgListVal {
			// With an array literal, the index in path is that of the first element.
			elementPath := append(([]int32)(nil), path...)
			elementPath[len(elementPath)-1] += int32(i)
			elements[i].Path = elementPath
			elements[i].Children = msgSourceInfo(elementPath, msgVal)
		}
		children = &sourceinfo.ArrayLiteralSourceInfo{Elements: elements}
	} else if len(f.value.msgVal) > 0 {
		children = msgSourceInfo(path, f.value.msgVal)
	}
	return &sourceinfo.OptionSourceInfo{
		Path:     path,
		Children: children,
	}
}

func msgSourceInfo(prefix []int32, fields []*interpretedField) *sourceinfo.MessageLiteralSourceInfo {
	fieldInfo := map[*ast.MessageFieldNode]*sourceinfo.OptionSourceInfo{}
	for _, field := range fields {
		msgFieldNode, ok := field.node.(*ast.MessageFieldNode)
		if !ok {
			continue
		}
		fieldInfo[msgFieldNode] = field.toSourceInfo(prefix)
	}
	return &sourceinfo.MessageLiteralSourceInfo{Fields: fieldInfo}
}

// interpretedFieldValue is a wrapper around protoreflect.Value that
// includes extra metadata.
type interpretedFieldValue struct {
	// the bytes for this field value if already pre-serialized
	// (when this is set, the other fields are ignored)
	preserialized []byte

	// the field value
	val protoreflect.Value
	// if true, this value is a list of values, not a singular value
	isList bool
	// non-nil for singular message values
	msgVal []*interpretedField
	// non-nil for non-empty lists of message values
	msgListVal [][]*interpretedField
}

func appendOptionBytes(b []byte, flds []*interpretedField) ([]byte, error) {
	// protoc emits messages sorted by field number
	if len(flds) > 1 {
		sort.SliceStable(flds, func(i, j int) bool {
			return flds[i].number < flds[j].number
		})
	}

	for i := 0; i < len(flds); i++ {
		f := flds[i]
		if f.value.preserialized != nil {
			b = append(b, f.value.preserialized...)
			continue
		}
		switch {
		case f.packed && canPack(f.kind):
			// for packed repeated numeric fields, all runs of values are merged into one packed list
			num := f.number
			j := i
			for j < len(flds) && flds[j].number == num {
				j++
			}
			// now flds[i:j] is the range of contiguous fields for the same field number
			enclosed, err := appendOptionBytesPacked(nil, f.kind, flds[i:j])
			if err != nil {
				return nil, err
			}
			b = protowire.AppendTag(b, protowire.Number(f.number), protowire.BytesType)
			b = protowire.AppendBytes(b, enclosed)
			// skip over the other subsequent fields we just serialized
			i = j - 1
		case f.value.isList:
			// if not packed, then emit one value at a time
			single := *f
			single.value.isList = false
			single.value.msgListVal = nil
			l := f.value.val.List()
			for i := 0; i < l.Len(); i++ {
				single.value.val = l.Get(i)
				if f.kind == protoreflect.MessageKind || f.kind == protoreflect.GroupKind {
					single.value.msgVal = f.value.msgListVal[i]
				}
				var err error
				b, err = appendOptionBytesSingle(b, &single)
				if err != nil {
					return nil, err
				}
			}
		default:
			// simple singular value
			var err error
			b, err = appendOptionBytesSingle(b, f)
			if err != nil {
				return nil, err
			}
		}
	}

	return b, nil
}

func canPack(k protoreflect.Kind) bool {
	switch k {
	case protoreflect.MessageKind, protoreflect.GroupKind, protoreflect.StringKind, protoreflect.BytesKind:
		return false
	default:
		return true
	}
}

func appendOptionBytesPacked(b []byte, k protoreflect.Kind, flds []*interpretedField) ([]byte, error) {
	for i := range flds {
		val := flds[i].value
		if val.isList {
			l := val.val.List()
			var err error
			b, err = appendNumericValueBytesPacked(b, k, l)
			if err != nil {
				return nil, err
			}
		} else {
			var err error
			b, err = appendNumericValueBytes(b, k, val.val)
			if err != nil {
				return nil, err
			}
		}
	}
	return b, nil
}

func appendOptionBytesSingle(b []byte, f *interpretedField) ([]byte, error) {
	if f.value.preserialized != nil {
		return append(b, f.value.preserialized...), nil
	}
	num := protowire.Number(f.number)
	switch f.kind {
	case protoreflect.MessageKind:
		enclosed, err := appendOptionBytes(nil, f.value.msgVal)
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, num, protowire.BytesType)
		return protowire.AppendBytes(b, enclosed), nil

	case protoreflect.GroupKind:
		b = protowire.AppendTag(b, num, protowire.StartGroupType)
		var err error
		b, err = appendOptionBytes(b, f.value.msgVal)
		if err != nil {
			return nil, err
		}
		return protowire.AppendTag(b, num, protowire.EndGroupType), nil

	case protoreflect.StringKind:
		b = protowire.AppendTag(b, num, protowire.BytesType)
		return protowire.AppendString(b, f.value.val.String()), nil

	case protoreflect.BytesKind:
		b = protowire.AppendTag(b, num, protowire.BytesType)
		return protowire.AppendBytes(b, f.value.val.Bytes()), nil

	case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind, protoreflect.EnumKind, protoreflect.BoolKind:
		b = protowire.AppendTag(b, num, protowire.VarintType)
		return appendNumericValueBytes(b, f.kind, f.value.val)

	case protoreflect.Fixed32Kind, protoreflect.Sfixed32Kind, protoreflect.FloatKind:
		b = protowire.AppendTag(b, num, protowire.Fixed32Type)
		return appendNumericValueBytes(b, f.kind, f.value.val)

	case protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind, protoreflect.DoubleKind:
		b = protowire.AppendTag(b, num, protowire.Fixed64Type)
		return appendNumericValueBytes(b, f.kind, f.value.val)

	default:
		return nil, fmt.Errorf("unknown field kind: %v", f.kind)
	}
}

func appendNumericValueBytesPacked(b []byte, k protoreflect.Kind, l protoreflect.List) ([]byte, error) {
	for i := 0; i < l.Len(); i++ {
		var err error
		b, err = appendNumericValueBytes(b, k, l.Get(i))
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

func appendNumericValueBytes(b []byte, k protoreflect.Kind, v protoreflect.Value) ([]byte, error) {
	switch k {
	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		return protowire.AppendVarint(b, uint64(v.Int())), nil
	case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		return protowire.AppendVarint(b, v.Uint()), nil
	case protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		return protowire.AppendVarint(b, protowire.EncodeZigZag(v.Int())), nil
	case protoreflect.Fixed32Kind:
		return protowire.AppendFixed32(b, uint32(v.Uint())), nil
	case protoreflect.Fixed64Kind:
		return protowire.AppendFixed64(b, v.Uint()), nil
	case protoreflect.Sfixed32Kind:
		return protowire.AppendFixed32(b, uint32(v.Int())), nil
	case protoreflect.Sfixed64Kind:
		return protowire.AppendFixed64(b, uint64(v.Int())), nil
	case protoreflect.FloatKind:
		return protowire.AppendFixed32(b, math.Float32bits(float32(v.Float()))), nil
	case protoreflect.DoubleKind:
		return protowire.AppendFixed64(b, math.Float64bits(v.Float())), nil
	case protoreflect.BoolKind:
		return protowire.AppendVarint(b, protowire.EncodeBool(v.Bool())), nil
	case protoreflect.EnumKind:
		return protowire.AppendVarint(b, uint64(v.Enum())), nil
	default:
		return nil, fmt.Errorf("unknown field kind: %v", k)
	}
}

// optionsContainer may be optionally implemented by a linker.Result. It is
// not part of the linker.Result interface as it is meant only for internal use.
// This allows the option interpreter step to store extra metadata about the
// serialized structure of options.
type optionsContainer interface {
	// AddOptionBytes adds the given pre-serialized option bytes to a file,
	// associated with the given options message. The type of the given message
	// should be an options message, for example *descriptorpb.MessageOptions.
	// This value should be part of the message hierarchy whose root is the
	// *descriptorpb.FileDescriptorProto that corresponds to this result.
	AddOptionBytes(pm proto.Message, opts []byte)
}

func interpretElementOptions[Elem elementType[OptsStruct, Opts], OptsStruct any, Opts optionsType[OptsStruct]](
	interp *interpreter,
	fqn string,
	target *targetType[Elem, OptsStruct, Opts],
	elem Elem,
) error {
	opts := elem.GetOptions()
	uo := opts.GetUninterpretedOption()
	if len(uo) > 0 {
		remain, err := interp.interpretOptions(fqn, target.t, elem, opts, uo)
		if err != nil {
			return err
		}
		target.setUninterpretedOptions(opts, remain)
	}
	return nil
}

// interpretOptions processes the options in uninterpreted, which are interpreted as fields
// of the given opts message. The first return value is the features to use for child elements.
// On success, the latter two return values will usually be nil, nil. But if the current
// operation is lenient, it may return a non-nil slice of uninterpreted options on success.
// In such a case, the returned slice contains the options which could not be interpreted.
func (interp *interpreter) interpretOptions(
	fqn string,
	targetType descriptorpb.FieldOptions_OptionTargetType,
	element, opts proto.Message,
	uninterpreted []*descriptorpb.UninterpretedOption,
) ([]*descriptorpb.UninterpretedOption, error) {
	optsDesc := opts.ProtoReflect().Descriptor()
	optsFqn := string(optsDesc.FullName())
	var msg protoreflect.Message
	// see if the parse included an override copy for these options
	if md := interp.resolveOptionsType(optsFqn); md != nil {
		dm := dynamicpb.NewMessage(md)
		if err := cloneInto(dm, opts, nil); err != nil {
			node := interp.file.Node(element)
			return nil, interp.reporter.HandleError(reporter.Error(interp.nodeInfo(node), err))
		}
		msg = dm
	} else {
		msg = proto.Clone(opts).ProtoReflect()
	}

	mc := &internal.MessageContext{
		File:        interp.file,
		ElementName: fqn,
		ElementType: descriptorType(element),
	}
	var remain []*descriptorpb.UninterpretedOption
	results := make([]*interpretedOption, 0, len(uninterpreted))
	var featuresInfo []*interpretedOption
	for _, uo := range uninterpreted {
		node := interp.file.OptionNode(uo)
		if !uo.Name[0].GetIsExtension() && uo.Name[0].GetNamePart() == "uninterpreted_option" {
			if interp.lenient {
				remain = append(remain, uo)
				continue
			}
			// uninterpreted_option might be found reflectively, but is not actually valid for use
			if err := interp.reporter.HandleErrorf(interp.nodeInfo(node.GetName()), "%vinvalid option 'uninterpreted_option'", mc); err != nil {
				return nil, err
			}
		}
		mc.Option = uo
		res, err := interp.interpretField(mc, msg, uo, 0, nil)
		if err != nil {
			if interp.lenient {
				remain = append(remain, uo)
				continue
			}
			return nil, err
		}
		res.unknown = !isKnownField(optsDesc, res)
		results = append(results, res)
		if !uo.Name[0].GetIsExtension() && uo.Name[0].GetNamePart() == featuresFieldName {
			featuresInfo = append(featuresInfo, res)
		}
		if optn, ok := node.(*ast.OptionNode); ok {
			si := res.toSourceInfo()
			interp.index[optn] = si
		}
	}

	if err := interp.validateFeatures(targetType, msg, featuresInfo); err != nil && !interp.lenient {
		return nil, err
	}

	if interp.lenient {
		// If we're lenient, then we don't want to clobber the passed in message
		// and leave it partially populated. So we convert into a copy first
		optsClone := opts.ProtoReflect().New().Interface()
		if err := cloneInto(optsClone, msg.Interface(), interp.resolver); err != nil {
			// TODO: do this in a more granular way, so we can convert individual
			// fields and leave bad ones uninterpreted instead of skipping all of
			// the work we've done so far.
			return uninterpreted, nil
		}
		// conversion from dynamic message above worked, so now
		// it is safe to overwrite the passed in message
		proto.Reset(opts)
		proto.Merge(opts, optsClone)

		if interp.container != nil {
			b, err := interp.toOptionBytes(mc, results)
			if err != nil {
				return nil, err
			}
			interp.container.AddOptionBytes(opts, b)
		}

		return remain, nil
	}

	if err := validateRecursive(msg, ""); err != nil {
		node := interp.file.Node(element)
		if err := interp.reporter.HandleErrorf(interp.nodeInfo(node), "error in %s options: %v", descriptorType(element), err); err != nil {
			return nil, err
		}
	}

	// now try to convert into the passed in message and fail if not successful
	if err := cloneInto(opts, msg.Interface(), interp.resolver); err != nil {
		node := interp.file.Node(element)
		return nil, interp.reporter.HandleError(reporter.Error(interp.nodeInfo(node), err))
	}

	if interp.container != nil {
		b, err := interp.toOptionBytes(mc, results)
		if err != nil {
			return nil, err
		}
		interp.container.AddOptionBytes(opts, b)
	}

	return nil, nil
}

func (interp *interpreter) validateFeatures(
	targetType descriptorpb.FieldOptions_OptionTargetType,
	opts protoreflect.Message,
	featuresInfo []*interpretedOption,
) error {
	fld := opts.Descriptor().Fields().ByName(featuresFieldName)
	if fld == nil {
		// no features to resolve
		return nil
	}
	if fld.IsList() || fld.Message() == nil || fld.Message().FullName() != featureSetName {
		// features field doesn't look right... abort
		// TODO: should this return an error?
		return nil
	}
	features := opts.Get(fld).Message()
	var err error
	features.Range(func(featureField protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		opts, ok := featureField.Options().(*descriptorpb.FieldOptions)
		if !ok {
			return true
		}
		targetTypes := opts.GetTargets()
		var allowed bool
		for _, allowedType := range targetTypes {
			if allowedType == targetType {
				allowed = true
				break
			}
		}
		if !allowed {
			allowedTypes := make([]string, len(targetTypes))
			for i, t := range opts.Targets {
				allowedTypes[i] = targetTypeString(t)
			}
			pos := interp.positionOfFeature(featuresInfo, fld.Number(), featureField.Number())
			if len(opts.Targets) == 1 && opts.Targets[0] == descriptorpb.FieldOptions_TARGET_TYPE_UNKNOWN {
				err = interp.reporter.HandleErrorf(pos, "feature field %q may not be used explicitly", featureField.Name())
			} else {
				err = interp.reporter.HandleErrorf(pos, "feature %q is allowed on [%s], not on %s", featureField.Name(), strings.Join(allowedTypes, ","), targetTypeString(targetType))
			}
		}
		return err == nil
	})
	return err
}

func (interp *interpreter) positionOfFeature(featuresInfo []*interpretedOption, fieldNumbers ...protoreflect.FieldNumber) ast.SourceSpan {
	if interp.file.AST() == nil {
		return ast.UnknownSpan(interp.file.FileDescriptorProto().GetName())
	}
	for _, info := range featuresInfo {
		matched, remainingNumbers, node := matchInterpretedOption(info, fieldNumbers)
		if !matched {
			continue
		}
		if len(remainingNumbers) > 0 {
			node = findInterpretedFieldForFeature(&(info.interpretedField), remainingNumbers)
		}
		if node != nil {
			return interp.file.FileNode().NodeInfo(node)
		}
	}
	return ast.UnknownSpan(interp.file.FileDescriptorProto().GetName())
}

func matchInterpretedOption(info *interpretedOption, path []protoreflect.FieldNumber) (bool, []protoreflect.FieldNumber, ast.Node) {
	for i := 0; i < len(path) && i < len(info.pathPrefix); i++ {
		if info.pathPrefix[i] != int32(path[i]) {
			return false, nil, nil
		}
	}
	if len(path) <= len(info.pathPrefix) {
		// no more path elements to match
		node := info.node
		if optsNode, ok := node.(*ast.OptionNode); ok {
			// Do we need to check this? It should always be true...
			if len(optsNode.Name.Parts) == len(info.pathPrefix)+1 {
				node = optsNode.Name.Parts[len(path)-1]
			}
		}
		return true, nil, node
	}
	if info.number != int32(path[len(info.pathPrefix)]) {
		return false, nil, nil
	}
	return true, path[len(info.pathPrefix)+1:], info.node
}

func findInterpretedFieldForFeature(opt *interpretedField, path []protoreflect.FieldNumber) ast.Node {
	if len(path) == 0 {
		return opt.node
	}
	for _, fld := range opt.value.msgVal {
		if fld.number == int32(path[0]) {
			if res := findInterpretedFieldForFeature(fld, path[1:]); res != nil {
				return res
			}
		}
	}
	return nil
}

// isKnownField returns true if the given option is for a known field of the
// given options message descriptor and will be serialized using the expected
// wire type for that known field.
func isKnownField(desc protoreflect.MessageDescriptor, opt *interpretedOption) bool {
	var num int32
	if len(opt.pathPrefix) > 0 {
		num = opt.pathPrefix[0]
	} else {
		num = opt.number
	}
	fd := desc.Fields().ByNumber(protoreflect.FieldNumber(num))
	if fd == nil {
		return false
	}

	// Before the full wire type check, we do a quick check that will usually pass
	// and allow us to short-circuit the logic below.
	if fd.IsList() == opt.repeated && fd.Kind() == opt.kind {
		return true
	}

	// We figure out the wire type this interpreted field will use when serialized.
	var wireType protowire.Type
	switch {
	case len(opt.pathPrefix) > 0:
		// If path prefix exists, this field is nested inside a message.
		// And messages use bytes wire type.
		wireType = protowire.BytesType
	case opt.repeated && opt.packed && canPack(opt.kind):
		// Packed repeated numeric scalars use bytes wire type.
		wireType = protowire.BytesType
	default:
		wireType = wireTypeForKind(opt.kind)
	}

	// And then we see if the wire type we just determined is compatible with
	// the field descriptor we found.
	if fd.IsList() && canPack(fd.Kind()) && wireType == protowire.BytesType {
		// Even if fd.IsPacked() is false, bytes type is still accepted for
		// repeated scalar numerics, so that changing a repeated field from
		// packed to not-packed (or vice versa) is a compatible change.
		return true
	}
	return wireType == wireTypeForKind(fd.Kind())
}

func wireTypeForKind(kind protoreflect.Kind) protowire.Type {
	switch kind {
	case protoreflect.StringKind, protoreflect.BytesKind, protoreflect.MessageKind:
		return protowire.BytesType
	case protoreflect.GroupKind:
		return protowire.StartGroupType
	case protoreflect.Fixed32Kind, protoreflect.Sfixed32Kind, protoreflect.FloatKind:
		return protowire.Fixed32Type
	case protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind, protoreflect.DoubleKind:
		return protowire.Fixed64Type
	default:
		// everything else uses varint
		return protowire.VarintType
	}
}

func targetTypeString(t descriptorpb.FieldOptions_OptionTargetType) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(t.String(), "TARGET_TYPE_"), "_", " "))
}

func cloneInto(dest proto.Message, src proto.Message, res linker.Resolver) error {
	if dest.ProtoReflect().Descriptor() == src.ProtoReflect().Descriptor() {
		proto.Reset(dest)
		proto.Merge(dest, src)
		return proto.CheckInitialized(dest)
	}

	// If descriptors are not the same, we could have field descriptors in src that
	// don't match the ones in dest. There's no easy/sane way to handle that. So we
	// just marshal to bytes and back to do this
	data, err := proto.Marshal(src)
	if err != nil {
		return err
	}
	return proto.UnmarshalOptions{Resolver: res}.Unmarshal(data, dest)
}

func (interp *interpreter) toOptionBytes(mc *internal.MessageContext, results []*interpretedOption) ([]byte, error) {
	// protoc emits non-custom options in tag order and then
	// the rest are emitted in the order they are defined in source
	sort.SliceStable(results, func(i, j int) bool {
		if !results[i].unknown && results[j].unknown {
			return true
		}
		if !results[i].unknown && !results[j].unknown {
			return results[i].number < results[j].number
		}
		return false
	})
	var b []byte
	for _, res := range results {
		var err error
		b, err = res.appendOptionBytes(b)
		if err != nil {
			if _, ok := err.(reporter.ErrorWithPos); !ok {
				span := ast.UnknownSpan(interp.file.AST().Name())
				err = reporter.Errorf(span, "%sfailed to encode options: %w", mc, err)
			}
			if err := interp.reporter.HandleError(err); err != nil {
				return nil, err
			}
		}
	}
	return b, nil
}

func validateRecursive(msg protoreflect.Message, prefix string) error {
	flds := msg.Descriptor().Fields()
	var missingFields []string
	for i := 0; i < flds.Len(); i++ {
		fld := flds.Get(i)
		if fld.Cardinality() == protoreflect.Required && !msg.Has(fld) {
			missingFields = append(missingFields, fmt.Sprintf("%s%s", prefix, fld.Name()))
		}
	}
	if len(missingFields) > 0 {
		return fmt.Errorf("some required fields missing: %v", strings.Join(missingFields, ", "))
	}

	var err error
	msg.Range(func(fld protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		if fld.IsMap() {
			md := fld.MapValue().Message()
			if md != nil {
				val.Map().Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
					chprefix := fmt.Sprintf("%s%s[%v].", prefix, fieldName(fld), k)
					err = validateRecursive(v.Message(), chprefix)
					return err == nil
				})
				if err != nil {
					return false
				}
			}
		} else {
			md := fld.Message()
			if md != nil {
				if fld.IsList() {
					sl := val.List()
					for i := 0; i < sl.Len(); i++ {
						v := sl.Get(i)
						chprefix := fmt.Sprintf("%s%s[%d].", prefix, fieldName(fld), i)
						err = validateRecursive(v.Message(), chprefix)
						if err != nil {
							return false
						}
					}
				} else {
					chprefix := fmt.Sprintf("%s%s.", prefix, fieldName(fld))
					err = validateRecursive(val.Message(), chprefix)
					if err != nil {
						return false
					}
				}
			}
		}
		return true
	})
	return err
}

// interpretField interprets the option described by opt, as a field inside the given msg. This
// interprets components of the option name starting at nameIndex. When nameIndex == 0, then
// msg must be an options message. For nameIndex > 0, msg is a nested message inside of the
// options message. The given pathPrefix is the path (sequence of field numbers and indices
// with a FileDescriptorProto as the start) up to but not including the given nameIndex.
func (interp *interpreter) interpretField(mc *internal.MessageContext, msg protoreflect.Message, opt *descriptorpb.UninterpretedOption, nameIndex int, pathPrefix []int32) (*interpretedOption, error) {
	var fld protoreflect.FieldDescriptor
	nm := opt.GetName()[nameIndex]
	node := interp.file.OptionNamePartNode(nm)
	if nm.GetIsExtension() {
		extName := nm.GetNamePart()
		if extName[0] == '.' {
			extName = extName[1:] /* skip leading dot */
		}
		var err error
		fld, err = interp.resolveExtensionType(extName)
		if errors.Is(err, protoregistry.NotFound) {
			return nil, interp.reporter.HandleErrorf(interp.nodeInfo(node),
				"%vunrecognized extension %s of %s",
				mc, extName, msg.Descriptor().FullName())
		} else if err != nil {
			return nil, interp.reporter.HandleErrorWithPos(interp.nodeInfo(node), err)
		}
		if fld.ContainingMessage().FullName() != msg.Descriptor().FullName() {
			return nil, interp.reporter.HandleErrorf(interp.nodeInfo(node),
				"%vextension %s should extend %s but instead extends %s",
				mc, extName, msg.Descriptor().FullName(), fld.ContainingMessage().FullName())
		}
	} else {
		fld = msg.Descriptor().Fields().ByName(protoreflect.Name(nm.GetNamePart()))
		if fld == nil {
			return nil, interp.reporter.HandleErrorf(interp.nodeInfo(node),
				"%vfield %s of %s does not exist",
				mc, nm.GetNamePart(), msg.Descriptor().FullName())
		}
	}

	if len(opt.GetName()) > nameIndex+1 {
		nextnm := opt.GetName()[nameIndex+1]
		nextnode := interp.file.OptionNamePartNode(nextnm)
		k := fld.Kind()
		if k != protoreflect.MessageKind && k != protoreflect.GroupKind {
			return nil, interp.reporter.HandleErrorf(interp.nodeInfo(nextnode),
				"%vcannot set field %s because %s is not a message",
				mc, nextnm.GetNamePart(), nm.GetNamePart())
		}
		if fld.Cardinality() == protoreflect.Repeated {
			return nil, interp.reporter.HandleErrorf(interp.nodeInfo(nextnode),
				"%vcannot set field %s because %s is repeated (must use an aggregate)",
				mc, nextnm.GetNamePart(), nm.GetNamePart())
		}
		var fdm protoreflect.Message
		if msg.Has(fld) {
			v := msg.Mutable(fld)
			fdm = v.Message()
		} else {
			if ood := fld.ContainingOneof(); ood != nil {
				existingFld := msg.WhichOneof(ood)
				if existingFld != nil && existingFld.Number() != fld.Number() {
					return nil, interp.reporter.HandleErrorf(interp.nodeInfo(node),
						"%voneof %q already has field %q set",
						mc, ood.Name(), fieldName(existingFld))
				}
			}
			fldVal := msg.NewField(fld)
			fdm = fldVal.Message()
			msg.Set(fld, fldVal)
		}
		// recurse to set next part of name
		return interp.interpretField(mc, fdm, opt, nameIndex+1, append(pathPrefix, int32(fld.Number())))
	}

	optNode := interp.file.OptionNode(opt)
	optValNode := optNode.GetValue()
	var val interpretedFieldValue
	var index int
	var err error
	if optValNode.Value() == nil {
		// We don't have an AST, so we get the value from the uninterpreted option proto.
		// It's okay that we don't populate index as it is used to populate source code info,
		// which can't be done without an AST.
		val, err = interp.setOptionFieldFromProto(mc, msg, fld, node, opt, optValNode)
	} else {
		val, index, err = interp.setOptionField(mc, msg, fld, node, optValNode, false)
	}
	if err != nil {
		return nil, interp.reporter.HandleError(err)
	}
	return &interpretedOption{
		pathPrefix: pathPrefix,
		interpretedField: interpretedField{
			node:     optNode,
			number:   int32(fld.Number()),
			index:    int32(index),
			kind:     fld.Kind(),
			repeated: fld.Cardinality() == protoreflect.Repeated,
			value:    val,
			// NB: don't set packed here in a top-level option
			// (only values in message literals will be serialized
			// in packed format)
		},
	}, nil
}

// setOptionField sets the value for field fld in the given message msg to the value represented
// by AST node val. The given name is the AST node that corresponds to the name of fld. On success,
// it returns additional metadata about the field that was set.
func (interp *interpreter) setOptionField(mc *internal.MessageContext, msg protoreflect.Message, fld protoreflect.FieldDescriptor, name ast.Node, val ast.ValueNode, insideMsgLiteral bool) (interpretedFieldValue, int, error) {
	v := val.Value()
	if sl, ok := v.([]ast.ValueNode); ok {
		// handle slices a little differently than the others
		if fld.Cardinality() != protoreflect.Repeated {
			return interpretedFieldValue{}, 0, reporter.Errorf(interp.nodeInfo(val), "%vvalue is an array but field is not repeated", mc)
		}
		origPath := mc.OptAggPath
		defer func() {
			mc.OptAggPath = origPath
		}()
		var resVal listValue
		var resMsgVals [][]*interpretedField
		var firstIndex int
		for index, item := range sl {
			mc.OptAggPath = fmt.Sprintf("%s[%d]", origPath, index)
			value, err := interp.fieldValue(mc, msg, fld, item, insideMsgLiteral)
			if err != nil {
				return interpretedFieldValue{}, 0, err
			}
			if fld.IsMap() {
				mv := msg.Mutable(fld).Map()
				if index == 0 {
					firstIndex = mv.Len()
				}
				setMapEntry(fld, msg, mv, &value)
			} else {
				lv := msg.Mutable(fld).List()
				if index == 0 {
					firstIndex = lv.Len()
				}
				lv.Append(value.val)
			}
			resVal = append(resVal, value.val)
			if value.msgVal != nil {
				resMsgVals = append(resMsgVals, value.msgVal)
			}
		}
		return interpretedFieldValue{
			isList:     true,
			val:        protoreflect.ValueOfList(&resVal),
			msgListVal: resMsgVals,
		}, firstIndex, nil
	}

	value, err := interp.fieldValue(mc, msg, fld, val, insideMsgLiteral)
	if err != nil {
		return interpretedFieldValue{}, 0, err
	}

	if ood := fld.ContainingOneof(); ood != nil {
		existingFld := msg.WhichOneof(ood)
		if existingFld != nil && existingFld.Number() != fld.Number() {
			return interpretedFieldValue{}, 0, reporter.Errorf(interp.nodeInfo(name), "%voneof %q already has field %q set", mc, ood.Name(), fieldName(existingFld))
		}
	}

	var index int
	switch {
	case fld.IsMap():
		mv := msg.Mutable(fld).Map()
		index = mv.Len()
		setMapEntry(fld, msg, mv, &value)
	case fld.IsList():
		lv := msg.Mutable(fld).List()
		index = lv.Len()
		lv.Append(value.val)
	default:
		if msg.Has(fld) {
			return interpretedFieldValue{}, 0, reporter.Errorf(interp.nodeInfo(name), "%vnon-repeated option field %s already set", mc, fieldName(fld))
		}
		msg.Set(fld, value.val)
	}
	return value, index, nil
}

// setOptionFieldFromProto sets the value for field fld in the given message msg to the value
// represented by the given uninterpreted option. The given ast.Node, if non-nil, will be used
// to report source positions in error messages. On success, it returns additional metadata
// about the field that was set.
func (interp *interpreter) setOptionFieldFromProto(mc *internal.MessageContext, msg protoreflect.Message, fld protoreflect.FieldDescriptor, name ast.Node, opt *descriptorpb.UninterpretedOption, node ast.Node) (interpretedFieldValue, error) {
	k := fld.Kind()
	var value interpretedFieldValue
	switch k {
	case protoreflect.EnumKind:
		num, _, err := interp.enumFieldValueFromProto(mc, fld.Enum(), opt, node)
		if err != nil {
			return interpretedFieldValue{}, err
		}
		value = interpretedFieldValue{val: protoreflect.ValueOfEnum(num)}

	case protoreflect.MessageKind, protoreflect.GroupKind:
		if opt.AggregateValue == nil {
			return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(node), "%vexpecting message, got %s", mc, optionValueKind(opt))
		}
		// We must parse the text format from the aggregate value string
		fmd := fld.Message()
		tmpMsg := dynamicpb.NewMessage(fmd)
		err := prototext.UnmarshalOptions{
			Resolver:     &msgLiteralResolver{interp: interp, pkg: fld.ParentFile().Package()},
			AllowPartial: true,
		}.Unmarshal([]byte(opt.GetAggregateValue()), tmpMsg)
		if err != nil {
			return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(node), "%vfailed to parse message literal %w", mc, err)
		}
		msgData, err := proto.MarshalOptions{
			AllowPartial: true,
		}.Marshal(tmpMsg)
		if err != nil {
			return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(node), "%vfailed to serialize data from message literal %w", mc, err)
		}
		var data []byte
		if k == protoreflect.GroupKind {
			data = protowire.AppendTag(data, fld.Number(), protowire.StartGroupType)
			data = append(data, msgData...)
			data = protowire.AppendTag(data, fld.Number(), protowire.EndGroupType)
		} else {
			data = protowire.AppendTag(nil, fld.Number(), protowire.BytesType)
			data = protowire.AppendBytes(data, msgData)
		}
		// NB: At this point, the serialized fields may no longer be in the same
		//     order as in the text literal. So for this case, the linker result's
		//     CanonicalProto won't be in *quite* the right order. ¯\_(ツ)_/¯
		value = interpretedFieldValue{preserialized: data}

	default:
		v, err := interp.scalarFieldValueFromProto(mc, descriptorpb.FieldDescriptorProto_Type(k), opt, node)
		if err != nil {
			return interpretedFieldValue{}, err
		}
		value = interpretedFieldValue{val: protoreflect.ValueOf(v)}
	}

	if ood := fld.ContainingOneof(); ood != nil {
		existingFld := msg.WhichOneof(ood)
		if existingFld != nil && existingFld.Number() != fld.Number() {
			return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(name), "%voneof %q already has field %q set", mc, ood.Name(), fieldName(existingFld))
		}
	}

	switch {
	case value.preserialized != nil:
		if !fld.IsList() && !fld.IsMap() && msg.Has(fld) {
			return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(name), "%vnon-repeated option field %s already set", mc, fieldName(fld))
		}
		// We have to merge the bytes for this field into the message.
		// TODO: if a map field, error if key for this entry already set?
		err := proto.UnmarshalOptions{
			Resolver:     &msgLiteralResolver{interp: interp, pkg: fld.ParentFile().Package()},
			AllowPartial: true,
			Merge:        true,
		}.Unmarshal(value.preserialized, msg.Interface())
		if err != nil {
			return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(name), "%v failed to set value for field %v: %w", mc, fieldName(fld), err)
		}
	case fld.IsList():
		msg.Mutable(fld).List().Append(value.val)
	default:
		if msg.Has(fld) {
			return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(name), "%vnon-repeated option field %s already set", mc, fieldName(fld))
		}
		msg.Set(fld, value.val)
	}
	return value, nil
}

func setMapEntry(fld protoreflect.FieldDescriptor, msg protoreflect.Message, mapVal protoreflect.Map, value *interpretedFieldValue) {
	entry := value.val.Message()
	keyFld, valFld := fld.MapKey(), fld.MapValue()
	// if an entry is missing a key or value, we add in an explicit
	// zero value to msgVals to match protoc (which also odds these
	// in even if not present in source)
	if !entry.Has(keyFld) {
		// put key before value
		value.msgVal = append(append(([]*interpretedField)(nil), zeroValue(keyFld)), value.msgVal...)
	}
	if !entry.Has(valFld) {
		value.msgVal = append(value.msgVal, zeroValue(valFld))
	}
	key := entry.Get(keyFld)
	val := entry.Get(valFld)
	if fld.MapValue().Kind() == protoreflect.MessageKind {
		// Replace any nil/invalid values with an empty message
		dm, valIsDynamic := val.Interface().(*dynamicpb.Message)
		if (valIsDynamic && dm == nil) || !val.Message().IsValid() {
			val = protoreflect.ValueOfMessage(dynamicpb.NewMessage(valFld.Message()))
		}
		_, containerIsDynamic := msg.Interface().(*dynamicpb.Message)
		if valIsDynamic && !containerIsDynamic {
			// This happens because we create dynamic messages to represent map entries,
			// but the container of the map may expect a non-dynamic, generated type.
			dest := mapVal.NewValue()
			_, destIsDynamic := dest.Message().Interface().(*dynamicpb.Message)
			if !destIsDynamic {
				// reflection Set methods do not support cases where destination is
				// generated but source is dynamic (or vice versa). But proto.Merge
				// *DOES* support that, as long as dest and source use the same
				// descriptor.
				proto.Merge(dest.Message().Interface(), val.Message().Interface())
				val = dest
			}
		}
	}
	// TODO: error if key is already present
	mapVal.Set(key.MapKey(), val)
}

// zeroValue returns the zero value for the field types as a *interpretedField.
// The given fld must NOT be a repeated field.
func zeroValue(fld protoreflect.FieldDescriptor) *interpretedField {
	var val protoreflect.Value
	var msgVal []*interpretedField
	switch fld.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		// needs to be non-nil, but empty
		msgVal = []*interpretedField{}
		msg := dynamicpb.NewMessage(fld.Message())
		val = protoreflect.ValueOfMessage(msg)
	case protoreflect.EnumKind:
		val = protoreflect.ValueOfEnum(0)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		val = protoreflect.ValueOfInt32(0)
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		val = protoreflect.ValueOfUint32(0)
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		val = protoreflect.ValueOfInt64(0)
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		val = protoreflect.ValueOfUint64(0)
	case protoreflect.BoolKind:
		val = protoreflect.ValueOfBool(false)
	case protoreflect.FloatKind:
		val = protoreflect.ValueOfFloat32(0)
	case protoreflect.DoubleKind:
		val = protoreflect.ValueOfFloat64(0)
	case protoreflect.BytesKind:
		val = protoreflect.ValueOfBytes(nil)
	case protoreflect.StringKind:
		val = protoreflect.ValueOfString("")
	}
	return &interpretedField{
		number: int32(fld.Number()),
		kind:   fld.Kind(),
		value: interpretedFieldValue{
			val:    val,
			msgVal: msgVal,
		},
	}
}

type listValue []protoreflect.Value

var _ protoreflect.List = (*listValue)(nil)

func (l listValue) Len() int {
	return len(l)
}

func (l listValue) Get(i int) protoreflect.Value {
	return l[i]
}

func (l listValue) Set(i int, value protoreflect.Value) {
	l[i] = value
}

func (l *listValue) Append(value protoreflect.Value) {
	*l = append(*l, value)
}

func (l listValue) AppendMutable() protoreflect.Value {
	panic("AppendMutable not supported")
}

func (l *listValue) Truncate(i int) {
	*l = (*l)[:i]
}

func (l listValue) NewElement() protoreflect.Value {
	panic("NewElement not supported")
}

func (l listValue) IsValid() bool {
	return true
}

type msgLiteralResolver struct {
	interp *interpreter
	pkg    protoreflect.FullName
}

func (r *msgLiteralResolver) FindMessageByName(message protoreflect.FullName) (protoreflect.MessageType, error) {
	return r.interp.resolver.FindMessageByName(message)
}

func (r *msgLiteralResolver) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	// In a message literal, we don't allow arbitrary URL prefixes
	pos := strings.LastIndexByte(url, '/')
	var urlPrefix string
	if pos > 0 {
		urlPrefix = url[:pos]
	}
	if urlPrefix != "type.googleapis.com" && urlPrefix != "type.googleprod.com" {
		return nil, fmt.Errorf("could not resolve type reference %s", url)
	}
	return r.FindMessageByName(protoreflect.FullName(url[pos+1:]))
}

func (r *msgLiteralResolver) FindExtensionByName(field protoreflect.FullName) (protoreflect.ExtensionType, error) {
	// In a message literal, extension name may be partially qualified, relative to package.
	// So we have to search through package scopes.
	pkg := r.pkg
	for {
		// TODO: This does not *fully* implement the insane logic of protoc with regards
		//       to resolving relative references.
		//       https://protobuf.com/docs/language-spec#reference-resolution
		name := pkg.Append(protoreflect.Name(field))
		ext, err := r.interp.resolver.FindExtensionByName(name)
		if err == nil {
			return ext, nil
		}
		if pkg == "" {
			// no more namespaces to check
			return nil, err
		}
		pkg = pkg.Parent()
	}
}

func (r *msgLiteralResolver) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	return r.interp.resolver.FindExtensionByNumber(message, field)
}

func fieldName(fld protoreflect.FieldDescriptor) string {
	if fld.IsExtension() {
		return fmt.Sprintf("(%s)", fld.FullName())
	}
	return string(fld.Name())
}

func valueKind(val interface{}) string {
	switch val := val.(type) {
	case ast.Identifier:
		return "identifier"
	case bool:
		return "bool"
	case int64:
		if val < 0 {
			return "negative integer"
		}
		return "integer"
	case uint64:
		return "integer"
	case float64:
		return "double"
	case string, []byte:
		return "string"
	case []*ast.MessageFieldNode:
		return "message"
	case []ast.ValueNode:
		return "array"
	default:
		return fmt.Sprintf("%T", val)
	}
}

func optionValueKind(opt *descriptorpb.UninterpretedOption) string {
	switch {
	case opt.IdentifierValue != nil:
		return "identifier"
	case opt.PositiveIntValue != nil:
		return "integer"
	case opt.NegativeIntValue != nil:
		return "negative integer"
	case opt.DoubleValue != nil:
		return "double"
	case opt.StringValue != nil:
		return "string"
	case opt.AggregateValue != nil:
		return "message"
	default:
		// should not be possible
		return "<nil>"
	}
}

// fieldValue computes a compile-time value (constant or list or message literal) for the given
// AST node val. The value in val must be assignable to the field fld.
func (interp *interpreter) fieldValue(mc *internal.MessageContext, msg protoreflect.Message, fld protoreflect.FieldDescriptor, val ast.ValueNode, insideMsgLiteral bool) (interpretedFieldValue, error) {
	k := fld.Kind()
	switch k {
	case protoreflect.EnumKind:
		num, _, err := interp.enumFieldValue(mc, fld.Enum(), val, insideMsgLiteral)
		if err != nil {
			return interpretedFieldValue{}, err
		}
		return interpretedFieldValue{val: protoreflect.ValueOfEnum(num)}, nil

	case protoreflect.MessageKind, protoreflect.GroupKind:
		v := val.Value()
		if aggs, ok := v.([]*ast.MessageFieldNode); ok {
			var childMsg protoreflect.Message
			switch {
			case fld.IsList():
				// List of messages
				val := msg.NewField(fld)
				childMsg = val.List().NewElement().Message()
			case fld.IsMap():
				// No generated type for map entries, so we use a dynamic type
				childMsg = dynamicpb.NewMessage(fld.Message())
			default:
				// Normal message field
				childMsg = msg.NewField(fld).Message()
			}
			return interp.messageLiteralValue(mc, aggs, childMsg)
		}
		return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(val), "%vexpecting message, got %s", mc, valueKind(v))

	default:
		v, err := interp.scalarFieldValue(mc, descriptorpb.FieldDescriptorProto_Type(k), val, insideMsgLiteral)
		if err != nil {
			return interpretedFieldValue{}, err
		}
		return interpretedFieldValue{val: protoreflect.ValueOf(v)}, nil
	}
}

// enumFieldValue resolves the given AST node val as an enum value descriptor. If the given
// value is not a valid identifier (or number if allowed), an error is returned instead.
func (interp *interpreter) enumFieldValue(mc *internal.MessageContext, ed protoreflect.EnumDescriptor, val ast.ValueNode, allowNumber bool) (protoreflect.EnumNumber, protoreflect.Name, error) {
	v := val.Value()
	var num protoreflect.EnumNumber
	switch v := v.(type) {
	case ast.Identifier:
		name := protoreflect.Name(v)
		ev := ed.Values().ByName(name)
		if ev == nil {
			return 0, "", reporter.Errorf(interp.nodeInfo(val), "%venum %s has no value named %s", mc, ed.FullName(), v)
		}
		return ev.Number(), name, nil
	case int64:
		if !allowNumber {
			return 0, "", reporter.Errorf(interp.nodeInfo(val), "%vexpecting enum name, got %s", mc, valueKind(v))
		}
		if v > math.MaxInt32 || v < math.MinInt32 {
			return 0, "", reporter.Errorf(interp.nodeInfo(val), "%vvalue %d is out of range for an enum", mc, v)
		}
		num = protoreflect.EnumNumber(v)
	case uint64:
		if !allowNumber {
			return 0, "", reporter.Errorf(interp.nodeInfo(val), "%vexpecting enum name, got %s", mc, valueKind(v))
		}
		if v > math.MaxInt32 {
			return 0, "", reporter.Errorf(interp.nodeInfo(val), "%vvalue %d is out of range for an enum", mc, v)
		}
		num = protoreflect.EnumNumber(v)
	default:
		return 0, "", reporter.Errorf(interp.nodeInfo(val), "%vexpecting enum, got %s", mc, valueKind(v))
	}
	ev := ed.Values().ByNumber(num)
	if ev != nil {
		return num, ev.Name(), nil
	}
	if ed.Syntax() != protoreflect.Proto3 {
		return 0, "", reporter.Errorf(interp.nodeInfo(val), "%vclosed enum %s has no value with number %d", mc, ed.FullName(), num)
	}
	// unknown value, but enum is open, so we allow it and return blank name
	return num, "", nil
}

// enumFieldValueFromProto resolves the given uninterpreted option value as an enum value descriptor.
// If the given value is not a valid identifier, an error is returned instead.
func (interp *interpreter) enumFieldValueFromProto(mc *internal.MessageContext, ed protoreflect.EnumDescriptor, opt *descriptorpb.UninterpretedOption, node ast.Node) (protoreflect.EnumNumber, protoreflect.Name, error) {
	// We don't have to worry about allowing numbers because numbers are never allowed
	// in uninterpreted values; they are only allowed inside aggregate values (i.e.
	// message literals).
	switch {
	case opt.IdentifierValue != nil:
		name := protoreflect.Name(opt.GetIdentifierValue())
		ev := ed.Values().ByName(name)
		if ev == nil {
			return 0, "", reporter.Errorf(interp.nodeInfo(node), "%venum %s has no value named %s", mc, ed.FullName(), name)
		}
		return ev.Number(), name, nil
	default:
		return 0, "", reporter.Errorf(interp.nodeInfo(node), "%vexpecting enum, got %s", mc, optionValueKind(opt))
	}
}

// scalarFieldValue resolves the given AST node val as a value whose type is assignable to a
// field with the given fldType.
func (interp *interpreter) scalarFieldValue(mc *internal.MessageContext, fldType descriptorpb.FieldDescriptorProto_Type, val ast.ValueNode, insideMsgLiteral bool) (interface{}, error) {
	v := val.Value()
	switch fldType {
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		if b, ok := v.(bool); ok {
			return b, nil
		}
		if id, ok := v.(ast.Identifier); ok {
			if insideMsgLiteral {
				// inside a message literal, values use the protobuf text format,
				// which is lenient in that it accepts "t" and "f" or "True" and "False"
				switch id {
				case "t", "true", "True":
					return true, nil
				case "f", "false", "False":
					return false, nil
				}
			} else {
				// options with simple scalar values (no message literal) are stricter
				switch id {
				case "true":
					return true, nil
				case "false":
					return false, nil
				}
			}
		}
		return nil, reporter.Errorf(interp.nodeInfo(val), "%vexpecting bool, got %s", mc, valueKind(v))
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		if str, ok := v.(string); ok {
			return []byte(str), nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(val), "%vexpecting bytes, got %s", mc, valueKind(v))
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		if str, ok := v.(string); ok {
			return str, nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(val), "%vexpecting string, got %s", mc, valueKind(v))
	case descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_TYPE_SINT32, descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		if i, ok := v.(int64); ok {
			if i > math.MaxInt32 || i < math.MinInt32 {
				return nil, reporter.Errorf(interp.nodeInfo(val), "%vvalue %d is out of range for int32", mc, i)
			}
			return int32(i), nil
		}
		if ui, ok := v.(uint64); ok {
			if ui > math.MaxInt32 {
				return nil, reporter.Errorf(interp.nodeInfo(val), "%vvalue %d is out of range for int32", mc, ui)
			}
			return int32(ui), nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(val), "%vexpecting int32, got %s", mc, valueKind(v))
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32, descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		if i, ok := v.(int64); ok {
			if i > math.MaxUint32 || i < 0 {
				return nil, reporter.Errorf(interp.nodeInfo(val), "%vvalue %d is out of range for uint32", mc, i)
			}
			return uint32(i), nil
		}
		if ui, ok := v.(uint64); ok {
			if ui > math.MaxUint32 {
				return nil, reporter.Errorf(interp.nodeInfo(val), "%vvalue %d is out of range for uint32", mc, ui)
			}
			return uint32(ui), nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(val), "%vexpecting uint32, got %s", mc, valueKind(v))
	case descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_TYPE_SINT64, descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		if i, ok := v.(int64); ok {
			return i, nil
		}
		if ui, ok := v.(uint64); ok {
			if ui > math.MaxInt64 {
				return nil, reporter.Errorf(interp.nodeInfo(val), "%vvalue %d is out of range for int64", mc, ui)
			}
			return int64(ui), nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(val), "%vexpecting int64, got %s", mc, valueKind(v))
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64, descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		if i, ok := v.(int64); ok {
			if i < 0 {
				return nil, reporter.Errorf(interp.nodeInfo(val), "%vvalue %d is out of range for uint64", mc, i)
			}
			return uint64(i), nil
		}
		if ui, ok := v.(uint64); ok {
			return ui, nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(val), "%vexpecting uint64, got %s", mc, valueKind(v))
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		if id, ok := v.(ast.Identifier); ok {
			switch id {
			case "inf":
				return math.Inf(1), nil
			case "nan":
				return math.NaN(), nil
			}
		}
		if d, ok := v.(float64); ok {
			return d, nil
		}
		if i, ok := v.(int64); ok {
			return float64(i), nil
		}
		if u, ok := v.(uint64); ok {
			return float64(u), nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(val), "%vexpecting double, got %s", mc, valueKind(v))
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		if id, ok := v.(ast.Identifier); ok {
			switch id {
			case "inf":
				return float32(math.Inf(1)), nil
			case "nan":
				return float32(math.NaN()), nil
			}
		}
		if d, ok := v.(float64); ok {
			return float32(d), nil
		}
		if i, ok := v.(int64); ok {
			return float32(i), nil
		}
		if u, ok := v.(uint64); ok {
			return float32(u), nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(val), "%vexpecting float, got %s", mc, valueKind(v))
	default:
		return nil, reporter.Errorf(interp.nodeInfo(val), "%vunrecognized field type: %s", mc, fldType)
	}
}

// scalarFieldValue resolves the given uninterpreted option value as a value whose type is
// assignable to a field with the given fldType.
func (interp *interpreter) scalarFieldValueFromProto(mc *internal.MessageContext, fldType descriptorpb.FieldDescriptorProto_Type, opt *descriptorpb.UninterpretedOption, node ast.Node) (interface{}, error) {
	switch fldType {
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		if opt.IdentifierValue != nil {
			switch opt.GetIdentifierValue() {
			case "true":
				return true, nil
			case "false":
				return false, nil
			}
		}
		return nil, reporter.Errorf(interp.nodeInfo(node), "%vexpecting bool, got %s", mc, optionValueKind(opt))
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		if opt.StringValue != nil {
			return opt.GetStringValue(), nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(node), "%vexpecting bytes, got %s", mc, optionValueKind(opt))
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		if opt.StringValue != nil {
			return string(opt.GetStringValue()), nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(node), "%vexpecting string, got %s", mc, optionValueKind(opt))
	case descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_TYPE_SINT32, descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		if opt.NegativeIntValue != nil {
			i := opt.GetNegativeIntValue()
			if i > math.MaxInt32 || i < math.MinInt32 {
				return nil, reporter.Errorf(interp.nodeInfo(node), "%vvalue %d is out of range for int32", mc, i)
			}
			return int32(i), nil
		}
		if opt.PositiveIntValue != nil {
			ui := opt.GetPositiveIntValue()
			if ui > math.MaxInt32 {
				return nil, reporter.Errorf(interp.nodeInfo(node), "%vvalue %d is out of range for int32", mc, ui)
			}
			return int32(ui), nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(node), "%vexpecting int32, got %s", mc, optionValueKind(opt))
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32, descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		if opt.NegativeIntValue != nil {
			i := opt.GetNegativeIntValue()
			if i > math.MaxUint32 || i < 0 {
				return nil, reporter.Errorf(interp.nodeInfo(node), "%vvalue %d is out of range for uint32", mc, i)
			}
			return uint32(i), nil
		}
		if opt.PositiveIntValue != nil {
			ui := opt.GetPositiveIntValue()
			if ui > math.MaxUint32 {
				return nil, reporter.Errorf(interp.nodeInfo(node), "%vvalue %d is out of range for uint32", mc, ui)
			}
			return uint32(ui), nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(node), "%vexpecting uint32, got %s", mc, optionValueKind(opt))
	case descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_TYPE_SINT64, descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		if opt.NegativeIntValue != nil {
			return opt.GetNegativeIntValue(), nil
		}
		if opt.PositiveIntValue != nil {
			ui := opt.GetPositiveIntValue()
			if ui > math.MaxInt64 {
				return nil, reporter.Errorf(interp.nodeInfo(node), "%vvalue %d is out of range for int64", mc, ui)
			}
			return int64(ui), nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(node), "%vexpecting int64, got %s", mc, optionValueKind(opt))
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64, descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		if opt.NegativeIntValue != nil {
			i := opt.GetNegativeIntValue()
			if i < 0 {
				return nil, reporter.Errorf(interp.nodeInfo(node), "%vvalue %d is out of range for uint64", mc, i)
			}
			// should not be possible since i should always be negative...
			return uint64(i), nil
		}
		if opt.PositiveIntValue != nil {
			return opt.GetPositiveIntValue(), nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(node), "%vexpecting uint64, got %s", mc, optionValueKind(opt))
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		if opt.IdentifierValue != nil {
			switch opt.GetIdentifierValue() {
			case "inf":
				return math.Inf(1), nil
			case "nan":
				return math.NaN(), nil
			}
		}
		if opt.DoubleValue != nil {
			return opt.GetDoubleValue(), nil
		}
		if opt.NegativeIntValue != nil {
			return float64(opt.GetNegativeIntValue()), nil
		}
		if opt.PositiveIntValue != nil {
			return float64(opt.GetPositiveIntValue()), nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(node), "%vexpecting double, got %s", mc, optionValueKind(opt))
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		if opt.IdentifierValue != nil {
			switch opt.GetIdentifierValue() {
			case "inf":
				return float32(math.Inf(1)), nil
			case "nan":
				return float32(math.NaN()), nil
			}
		}
		if opt.DoubleValue != nil {
			return float32(opt.GetDoubleValue()), nil
		}
		if opt.NegativeIntValue != nil {
			return float32(opt.GetNegativeIntValue()), nil
		}
		if opt.PositiveIntValue != nil {
			return float32(opt.GetPositiveIntValue()), nil
		}
		return nil, reporter.Errorf(interp.nodeInfo(node), "%vexpecting float, got %s", mc, optionValueKind(opt))
	default:
		return nil, reporter.Errorf(interp.nodeInfo(node), "%vunrecognized field type: %s", mc, fldType)
	}
}

func descriptorType(m proto.Message) string {
	switch m := m.(type) {
	case *descriptorpb.DescriptorProto:
		return "message"
	case *descriptorpb.DescriptorProto_ExtensionRange:
		return "extension range"
	case *descriptorpb.FieldDescriptorProto:
		if m.GetExtendee() == "" {
			return "field"
		}
		return "extension"
	case *descriptorpb.EnumDescriptorProto:
		return "enum"
	case *descriptorpb.EnumValueDescriptorProto:
		return "enum value"
	case *descriptorpb.ServiceDescriptorProto:
		return "service"
	case *descriptorpb.MethodDescriptorProto:
		return "method"
	case *descriptorpb.FileDescriptorProto:
		return "file"
	default:
		// shouldn't be possible
		return fmt.Sprintf("%T", m)
	}
}

func (interp *interpreter) messageLiteralValue(mc *internal.MessageContext, fieldNodes []*ast.MessageFieldNode, msg protoreflect.Message) (interpretedFieldValue, error) {
	fmd := msg.Descriptor()
	origPath := mc.OptAggPath
	defer func() {
		mc.OptAggPath = origPath
	}()
	// NB: we don't want to leave this nil, even if the
	// message is empty, because that indicates to
	// caller that the result is not a message
	flds := make([]*interpretedField, 0, len(fieldNodes))
	var foundAnyNode bool
	for _, fieldNode := range fieldNodes {
		if origPath == "" {
			mc.OptAggPath = fieldNode.Name.Value()
		} else {
			mc.OptAggPath = origPath + "." + fieldNode.Name.Value()
		}
		if fieldNode.Name.IsAnyTypeReference() {
			if fmd.FullName() != "google.protobuf.Any" {
				return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(fieldNode.Name.URLPrefix), "%vtype references are only allowed for google.protobuf.Any, but this type is %s", mc, fmd.FullName())
			}
			if foundAnyNode {
				return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(fieldNode.Name.URLPrefix), "%vmultiple any type references are not allowed", mc)
			}
			foundAnyNode = true
			urlPrefix := fieldNode.Name.URLPrefix.AsIdentifier()
			msgName := fieldNode.Name.Name.AsIdentifier()
			fullURL := fmt.Sprintf("%s/%s", urlPrefix, msgName)
			// TODO: Support other URLs dynamically -- the caller of protocompile
			// should be able to provide a custom resolver that can resolve type
			// URLs into message descriptors. The default resolver would be
			// implemented as below, only accepting "type.googleapis.com" and
			// "type.googleprod.com" as hosts/prefixes and using the compiled
			// file's transitive closure to find the named message.
			if urlPrefix != "type.googleapis.com" && urlPrefix != "type.googleprod.com" {
				return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(fieldNode.Name.URLPrefix), "%vcould not resolve type reference %s", mc, fullURL)
			}
			anyFields, ok := fieldNode.Val.Value().([]*ast.MessageFieldNode)
			if !ok {
				return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(fieldNode.Val), "%vtype references for google.protobuf.Any must have message literal value", mc)
			}
			anyMd := resolveDescriptor[protoreflect.MessageDescriptor](interp.resolver, string(msgName))
			if anyMd == nil {
				return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(fieldNode.Name.URLPrefix), "%vcould not resolve type reference %s", mc, fullURL)
			}
			// parse the message value
			msgVal, err := interp.messageLiteralValue(mc, anyFields, dynamicpb.NewMessage(anyMd))
			if err != nil {
				return interpretedFieldValue{}, err
			}

			// Any is defined with two fields:
			//   string type_url = 1
			//   bytes value = 2
			typeURLDescriptor := fmd.Fields().ByNumber(1)
			if typeURLDescriptor == nil || typeURLDescriptor.Kind() != protoreflect.StringKind {
				return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(fieldNode.Name), "%vfailed to set type_url string field on Any: %w", mc, err)
			}
			msg.Set(typeURLDescriptor, protoreflect.ValueOfString(fullURL))
			valueDescriptor := fmd.Fields().ByNumber(2)
			if valueDescriptor == nil || valueDescriptor.Kind() != protoreflect.BytesKind {
				return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(fieldNode.Name), "%vfailed to set value bytes field on Any: %w", mc, err)
			}
			b, err := proto.MarshalOptions{Deterministic: true}.Marshal(msgVal.val.Message().Interface())
			if err != nil {
				return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(fieldNode.Val), "%vfailed to serialize message value: %w", mc, err)
			}
			msg.Set(valueDescriptor, protoreflect.ValueOfBytes(b))
		} else {
			var ffld protoreflect.FieldDescriptor
			var err error
			if fieldNode.Name.IsExtension() {
				n := interp.file.ResolveMessageLiteralExtensionName(fieldNode.Name.Name)
				if n == "" {
					// this should not be possible!
					n = string(fieldNode.Name.Name.AsIdentifier())
				}
				ffld, err = interp.resolveExtensionType(n)
				if errors.Is(err, protoregistry.NotFound) {
					// may need to qualify with package name
					// (this should not be necessary!)
					pkg := mc.File.FileDescriptorProto().GetPackage()
					if pkg != "" {
						ffld, err = interp.resolveExtensionType(pkg + "." + n)
					}
				}
			} else {
				ffld = fmd.Fields().ByName(protoreflect.Name(fieldNode.Name.Value()))
				// Groups are indicated in the text format by the group name (which is
				// camel-case), NOT the field name (which is lower-case).
				// ...but only regular fields, not extensions that are groups...
				if ffld != nil && ffld.Kind() == protoreflect.GroupKind && ffld.Message().Name() != protoreflect.Name(fieldNode.Name.Value()) {
					// this is kind of silly to fail here, but this mimics protoc behavior
					return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(fieldNode.Name), "%vfield %s not found (did you mean the group named %s?)", mc, fieldNode.Name.Value(), ffld.Message().Name())
				}
				if ffld == nil {
					err = protoregistry.NotFound
					// could be a group name
					for i := 0; i < fmd.Fields().Len(); i++ {
						fd := fmd.Fields().Get(i)
						if fd.Kind() == protoreflect.GroupKind && fd.Message().Name() == protoreflect.Name(fieldNode.Name.Value()) {
							// found it!
							ffld = fd
							err = nil
							break
						}
					}
				}
			}
			if errors.Is(err, protoregistry.NotFound) {
				return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(fieldNode.Name),
					"%vfield %s not found", mc, string(fieldNode.Name.Name.AsIdentifier()))
			} else if err != nil {
				return interpretedFieldValue{}, reporter.Error(interp.nodeInfo(fieldNode.Name), err)
			}
			if fieldNode.Sep == nil && ffld.Message() == nil {
				// If there is no separator, the field type should be a message.
				// Otherwise it is an error in the text format.
				return interpretedFieldValue{}, reporter.Errorf(interp.nodeInfo(fieldNode.Val), "syntax error: unexpected value, expecting ':'")
			}
			res, index, err := interp.setOptionField(mc, msg, ffld, fieldNode.Name, fieldNode.Val, true)
			if err != nil {
				return interpretedFieldValue{}, err
			}
			flds = append(flds, &interpretedField{
				node:     fieldNode,
				number:   int32(ffld.Number()),
				index:    int32(index),
				kind:     ffld.Kind(),
				repeated: ffld.Cardinality() == protoreflect.Repeated,
				packed:   ffld.IsPacked(),
				value:    res,
			})
		}
	}
	return interpretedFieldValue{
		val:    protoreflect.ValueOfMessage(msg),
		msgVal: flds,
	}, nil
}
