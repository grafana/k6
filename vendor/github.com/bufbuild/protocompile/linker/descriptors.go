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

package linker

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/internal"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/protoutil"
)

var (
	// These "noOp*" values are all descriptors. The protoreflect.Descriptor
	// interface and its sub-interfaces are all marked with an unexported
	// method so that they cannot be implemented outside of the google.golang.org/protobuf
	// module. So, to provide implementations from this package, we must embed
	// them. If we simply left the embedded interface field nil, then if/when
	// new methods are added to the interfaces, it could induce panics in this
	// package or users of this module (since trying to invoke one of these new
	// methods would end up trying to call a method on a nil interface value).
	//
	// So instead of leaving the embedded interface fields nil, we embed an actual
	// value. While new methods are unlikely to return the correct value (since
	// the calls will be delegated to these no-op instances), it is a less
	// dangerous latent bug than inducing a nil-dereference panic.

	noOpFile      protoreflect.FileDescriptor
	noOpMessage   protoreflect.MessageDescriptor
	noOpOneof     protoreflect.OneofDescriptor
	noOpField     protoreflect.FieldDescriptor
	noOpEnum      protoreflect.EnumDescriptor
	noOpEnumValue protoreflect.EnumValueDescriptor
	noOpExtension protoreflect.ExtensionDescriptor
	noOpService   protoreflect.ServiceDescriptor
	noOpMethod    protoreflect.MethodDescriptor
)

func init() {
	noOpFile, _ = protodesc.NewFile(
		&descriptorpb.FileDescriptorProto{
			Name:       proto.String("no-op.proto"),
			Syntax:     proto.String("proto2"),
			Dependency: []string{"google/protobuf/descriptor.proto"},
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: proto.String("NoOpMsg"),
					Field: []*descriptorpb.FieldDescriptorProto{
						{
							Name:       proto.String("no_op"),
							Type:       descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
							Label:      descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							Number:     proto.Int32(1),
							JsonName:   proto.String("noOp"),
							OneofIndex: proto.Int32(0),
						},
					},
					OneofDecl: []*descriptorpb.OneofDescriptorProto{
						{
							Name: proto.String("no_op_oneof"),
						},
					},
				},
			},
			EnumType: []*descriptorpb.EnumDescriptorProto{
				{
					Name: proto.String("NoOpEnum"),
					Value: []*descriptorpb.EnumValueDescriptorProto{
						{
							Name:   proto.String("NO_OP"),
							Number: proto.Int32(0),
						},
					},
				},
			},
			Extension: []*descriptorpb.FieldDescriptorProto{
				{
					Extendee: proto.String(".google.protobuf.FileOptions"),
					Name:     proto.String("no_op"),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Number:   proto.Int32(50000),
				},
			},
			Service: []*descriptorpb.ServiceDescriptorProto{
				{
					Name: proto.String("NoOpService"),
					Method: []*descriptorpb.MethodDescriptorProto{
						{
							Name:       proto.String("NoOp"),
							InputType:  proto.String(".NoOpMsg"),
							OutputType: proto.String(".NoOpMsg"),
						},
					},
				},
			},
		},
		protoregistry.GlobalFiles,
	)
	noOpMessage = noOpFile.Messages().Get(0)
	noOpOneof = noOpMessage.Oneofs().Get(0)
	noOpField = noOpMessage.Fields().Get(0)
	noOpEnum = noOpFile.Enums().Get(0)
	noOpEnumValue = noOpEnum.Values().Get(0)
	noOpExtension = noOpFile.Extensions().Get(0)
	noOpService = noOpFile.Services().Get(0)
	noOpMethod = noOpService.Methods().Get(0)
}

// This file contains implementations of protoreflect.Descriptor. Note that
// this is a hack since those interfaces have a "doNotImplement" tag
// interface therein. We do just enough to make dynamicpb happy; constructing
// a regular descriptor would fail because we haven't yet interpreted options
// at the point we need these, and some validations will fail if the options
// aren't present.

type result struct {
	protoreflect.FileDescriptor
	parser.Result
	prefix string
	deps   Files

	// A map of all descriptors keyed by their fully-qualified name (without
	// any leading dot).
	descriptors map[string]protoreflect.Descriptor

	// A set of imports that have been used in the course of linking and
	// interpreting options.
	usedImports map[string]struct{}

	// A map of AST nodes that represent identifiers in ast.FieldReferenceNodes
	// to their fully-qualified name. The identifiers are for field names in
	// message literals (in option values) that are extension fields. These names
	// are resolved during linking and stored here, to be used to interpret options.
	optionQualifiedNames map[ast.IdentValueNode]string

	imports      fileImports
	messages     msgDescriptors
	enums        enumDescriptors
	extensions   extDescriptors
	services     svcDescriptors
	srcLocations srcLocs
}

var _ protoreflect.FileDescriptor = (*result)(nil)
var _ Result = (*result)(nil)
var _ protoutil.DescriptorProtoWrapper = (*result)(nil)

func (r *result) RemoveAST() {
	r.Result = parser.ResultWithoutAST(r.FileDescriptorProto())
	r.optionQualifiedNames = nil
}

func (r *result) AsProto() proto.Message {
	return r.FileDescriptorProto()
}

func (r *result) ParentFile() protoreflect.FileDescriptor {
	return r
}

func (r *result) Parent() protoreflect.Descriptor {
	return nil
}

func (r *result) Index() int {
	return 0
}

func (r *result) Syntax() protoreflect.Syntax {
	switch r.FileDescriptorProto().GetSyntax() {
	case "proto2", "":
		return protoreflect.Proto2
	case "proto3":
		return protoreflect.Proto3
	case "editions":
		return protoreflect.Editions
	default:
		return 0 // ???
	}
}

func (r *result) Edition() int32 {
	switch r.Syntax() {
	case protoreflect.Proto2:
		return int32(descriptorpb.Edition_EDITION_PROTO2)
	case protoreflect.Proto3:
		return int32(descriptorpb.Edition_EDITION_PROTO3)
	case protoreflect.Editions:
		return int32(r.FileDescriptorProto().GetEdition())
	default:
		return int32(descriptorpb.Edition_EDITION_UNKNOWN) // ???
	}
}

func (r *result) Name() protoreflect.Name {
	return ""
}

func (r *result) FullName() protoreflect.FullName {
	return r.Package()
}

func (r *result) IsPlaceholder() bool {
	return false
}

func (r *result) Options() protoreflect.ProtoMessage {
	return r.FileDescriptorProto().Options
}

func (r *result) Path() string {
	return r.FileDescriptorProto().GetName()
}

func (r *result) Package() protoreflect.FullName {
	return protoreflect.FullName(r.FileDescriptorProto().GetPackage())
}

func (r *result) Imports() protoreflect.FileImports {
	return &r.imports
}

func (r *result) Enums() protoreflect.EnumDescriptors {
	return &r.enums
}

func (r *result) Messages() protoreflect.MessageDescriptors {
	return &r.messages
}

func (r *result) Extensions() protoreflect.ExtensionDescriptors {
	return &r.extensions
}

func (r *result) Services() protoreflect.ServiceDescriptors {
	return &r.services
}

func (r *result) PopulateSourceCodeInfo() {
	srcLocProtos := asSourceLocations(r.FileDescriptorProto().GetSourceCodeInfo().GetLocation())
	srcLocIndex := computeSourceLocIndex(srcLocProtos)
	r.srcLocations = srcLocs{file: r, locs: srcLocProtos, index: srcLocIndex}
}

func (r *result) SourceLocations() protoreflect.SourceLocations {
	return &r.srcLocations
}

func computeSourceLocIndex(locs []protoreflect.SourceLocation) map[interface{}]int {
	index := map[interface{}]int{}
	for i, loc := range locs {
		if loc.Next == 0 {
			index[pathKey(loc.Path)] = i
		}
	}
	return index
}

func asSourceLocations(srcInfoProtos []*descriptorpb.SourceCodeInfo_Location) []protoreflect.SourceLocation {
	locs := make([]protoreflect.SourceLocation, len(srcInfoProtos))
	prev := map[any]*protoreflect.SourceLocation{}
	for i, loc := range srcInfoProtos {
		var stLin, stCol, enLin, enCol int
		if len(loc.Span) == 3 {
			stLin, stCol, enCol = int(loc.Span[0]), int(loc.Span[1]), int(loc.Span[2])
			enLin = stLin
		} else {
			stLin, stCol, enLin, enCol = int(loc.Span[0]), int(loc.Span[1]), int(loc.Span[2]), int(loc.Span[3])
		}
		locs[i] = protoreflect.SourceLocation{
			Path:                    loc.Path,
			LeadingComments:         loc.GetLeadingComments(),
			LeadingDetachedComments: loc.GetLeadingDetachedComments(),
			TrailingComments:        loc.GetTrailingComments(),
			StartLine:               stLin,
			StartColumn:             stCol,
			EndLine:                 enLin,
			EndColumn:               enCol,
		}
		str := pathKey(loc.Path)
		pr := prev[str]
		if pr != nil {
			pr.Next = i
		}
		prev[str] = &locs[i]
	}
	return locs
}

type fileImports struct {
	protoreflect.FileImports
	files []protoreflect.FileImport
}

func (r *result) createImports() fileImports {
	fd := r.FileDescriptorProto()
	imps := make([]protoreflect.FileImport, len(fd.Dependency))
	for i, dep := range fd.Dependency {
		desc := r.deps.FindFileByPath(dep)
		imps[i] = protoreflect.FileImport{FileDescriptor: desc}
	}
	for _, publicIndex := range fd.PublicDependency {
		imps[int(publicIndex)].IsPublic = true
	}
	for _, weakIndex := range fd.WeakDependency {
		imps[int(weakIndex)].IsWeak = true
	}
	return fileImports{files: imps}
}

func (f *fileImports) Len() int {
	return len(f.files)
}

func (f *fileImports) Get(i int) protoreflect.FileImport {
	return f.files[i]
}

type srcLocs struct {
	protoreflect.SourceLocations
	file  *result
	locs  []protoreflect.SourceLocation
	index map[interface{}]int
}

func (s *srcLocs) Len() int {
	return len(s.locs)
}

func (s *srcLocs) Get(i int) protoreflect.SourceLocation {
	return s.locs[i]
}

func (s *srcLocs) ByPath(p protoreflect.SourcePath) protoreflect.SourceLocation {
	index, ok := s.index[pathKey(p)]
	if !ok {
		return protoreflect.SourceLocation{}
	}
	return s.locs[index]
}

func (s *srcLocs) ByDescriptor(d protoreflect.Descriptor) protoreflect.SourceLocation {
	if d.ParentFile() != s.file {
		return protoreflect.SourceLocation{}
	}
	path, ok := internal.ComputePath(d)
	if !ok {
		return protoreflect.SourceLocation{}
	}
	return s.ByPath(path)
}

type msgDescriptors struct {
	protoreflect.MessageDescriptors
	msgs []*msgDescriptor
}

func (r *result) createMessages(prefix string, parent protoreflect.Descriptor, msgProtos []*descriptorpb.DescriptorProto) msgDescriptors {
	msgs := make([]*msgDescriptor, len(msgProtos))
	for i, msgProto := range msgProtos {
		msgs[i] = r.createMessageDescriptor(msgProto, parent, i, prefix+msgProto.GetName())
	}
	return msgDescriptors{msgs: msgs}
}

func (m *msgDescriptors) Len() int {
	return len(m.msgs)
}

func (m *msgDescriptors) Get(i int) protoreflect.MessageDescriptor {
	return m.msgs[i]
}

func (m *msgDescriptors) ByName(s protoreflect.Name) protoreflect.MessageDescriptor {
	for _, msg := range m.msgs {
		if msg.Name() == s {
			return msg
		}
	}
	return nil
}

type msgDescriptor struct {
	protoreflect.MessageDescriptor
	file   *result
	parent protoreflect.Descriptor
	index  int
	proto  *descriptorpb.DescriptorProto
	fqn    string

	fields           fldDescriptors
	oneofs           oneofDescriptors
	nestedMessages   msgDescriptors
	nestedEnums      enumDescriptors
	nestedExtensions extDescriptors

	extRanges  fieldRanges
	rsvdRanges fieldRanges
	rsvdNames  names
}

var _ protoreflect.MessageDescriptor = (*msgDescriptor)(nil)
var _ protoutil.DescriptorProtoWrapper = (*msgDescriptor)(nil)

func (r *result) createMessageDescriptor(md *descriptorpb.DescriptorProto, parent protoreflect.Descriptor, index int, fqn string) *msgDescriptor {
	ret := &msgDescriptor{MessageDescriptor: noOpMessage, file: r, parent: parent, index: index, proto: md, fqn: fqn}
	r.descriptors[fqn] = ret

	prefix := fqn + "."
	// NB: We MUST create fields before oneofs so that we can populate the
	//  set of fields that belong to the oneof
	ret.fields = r.createFields(prefix, ret, md.Field)
	ret.oneofs = r.createOneofs(prefix, ret, md.OneofDecl)
	ret.nestedMessages = r.createMessages(prefix, ret, md.NestedType)
	ret.nestedEnums = r.createEnums(prefix, ret, md.EnumType)
	ret.nestedExtensions = r.createExtensions(prefix, ret, md.Extension)
	ret.extRanges = createFieldRanges(md.ExtensionRange)
	ret.rsvdRanges = createFieldRanges(md.ReservedRange)
	ret.rsvdNames = names{s: md.ReservedName}

	return ret
}

func (m *msgDescriptor) MessageDescriptorProto() *descriptorpb.DescriptorProto {
	return m.proto
}

func (m *msgDescriptor) AsProto() proto.Message {
	return m.proto
}

func (m *msgDescriptor) ParentFile() protoreflect.FileDescriptor {
	return m.file
}

func (m *msgDescriptor) Parent() protoreflect.Descriptor {
	return m.parent
}

func (m *msgDescriptor) Index() int {
	return m.index
}

func (m *msgDescriptor) Syntax() protoreflect.Syntax {
	return m.file.Syntax()
}

func (m *msgDescriptor) Name() protoreflect.Name {
	return protoreflect.Name(m.proto.GetName())
}

func (m *msgDescriptor) FullName() protoreflect.FullName {
	return protoreflect.FullName(m.fqn)
}

func (m *msgDescriptor) IsPlaceholder() bool {
	return false
}

func (m *msgDescriptor) Options() protoreflect.ProtoMessage {
	return m.proto.Options
}

func (m *msgDescriptor) IsMapEntry() bool {
	return m.proto.Options.GetMapEntry()
}

func (m *msgDescriptor) Fields() protoreflect.FieldDescriptors {
	return &m.fields
}

func (m *msgDescriptor) Oneofs() protoreflect.OneofDescriptors {
	return &m.oneofs
}

func (m *msgDescriptor) ReservedNames() protoreflect.Names {
	return m.rsvdNames
}

func (m *msgDescriptor) ReservedRanges() protoreflect.FieldRanges {
	return m.rsvdRanges
}

func (m *msgDescriptor) RequiredNumbers() protoreflect.FieldNumbers {
	var indexes fieldNums
	for _, fld := range m.proto.Field {
		if fld.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REQUIRED {
			indexes.s = append(indexes.s, fld.GetNumber())
		}
	}
	return indexes
}

func (m *msgDescriptor) ExtensionRanges() protoreflect.FieldRanges {
	return m.extRanges
}

func (m *msgDescriptor) ExtensionRangeOptions(i int) protoreflect.ProtoMessage {
	return m.proto.ExtensionRange[i].Options
}

func (m *msgDescriptor) Enums() protoreflect.EnumDescriptors {
	return &m.nestedEnums
}

func (m *msgDescriptor) Messages() protoreflect.MessageDescriptors {
	return &m.nestedMessages
}

func (m *msgDescriptor) Extensions() protoreflect.ExtensionDescriptors {
	return &m.nestedExtensions
}

type names struct {
	protoreflect.Names
	s []string
}

func (n names) Len() int {
	return len(n.s)
}

func (n names) Get(i int) protoreflect.Name {
	return protoreflect.Name(n.s[i])
}

func (n names) Has(s protoreflect.Name) bool {
	for _, name := range n.s {
		if name == string(s) {
			return true
		}
	}
	return false
}

type fieldNums struct {
	protoreflect.FieldNumbers
	s []int32
}

func (n fieldNums) Len() int {
	return len(n.s)
}

func (n fieldNums) Get(i int) protoreflect.FieldNumber {
	return protoreflect.FieldNumber(n.s[i])
}

func (n fieldNums) Has(s protoreflect.FieldNumber) bool {
	for _, num := range n.s {
		if num == int32(s) {
			return true
		}
	}
	return false
}

type fieldRanges struct {
	protoreflect.FieldRanges
	ranges [][2]protoreflect.FieldNumber
}

type fieldRange interface {
	GetStart() int32
	GetEnd() int32
}

func createFieldRanges[T fieldRange](rangeProtos []T) fieldRanges {
	ranges := make([][2]protoreflect.FieldNumber, len(rangeProtos))
	for i, r := range rangeProtos {
		ranges[i] = [2]protoreflect.FieldNumber{
			protoreflect.FieldNumber(r.GetStart()),
			protoreflect.FieldNumber(r.GetEnd()),
		}
	}
	return fieldRanges{ranges: ranges}
}

func (f fieldRanges) Len() int {
	return len(f.ranges)
}

func (f fieldRanges) Get(i int) [2]protoreflect.FieldNumber {
	return f.ranges[i]
}

func (f fieldRanges) Has(n protoreflect.FieldNumber) bool {
	for _, r := range f.ranges {
		if r[0] <= n && r[1] > n {
			return true
		}
	}
	return false
}

type enumDescriptors struct {
	protoreflect.EnumDescriptors
	enums []*enumDescriptor
}

func (r *result) createEnums(prefix string, parent protoreflect.Descriptor, enumProtos []*descriptorpb.EnumDescriptorProto) enumDescriptors {
	enums := make([]*enumDescriptor, len(enumProtos))
	for i, enumProto := range enumProtos {
		enums[i] = r.createEnumDescriptor(enumProto, parent, i, prefix+enumProto.GetName())
	}
	return enumDescriptors{enums: enums}
}

func (e *enumDescriptors) Len() int {
	return len(e.enums)
}

func (e *enumDescriptors) Get(i int) protoreflect.EnumDescriptor {
	return e.enums[i]
}

func (e *enumDescriptors) ByName(s protoreflect.Name) protoreflect.EnumDescriptor {
	for _, en := range e.enums {
		if en.Name() == s {
			return en
		}
	}
	return nil
}

type enumDescriptor struct {
	protoreflect.EnumDescriptor
	file   *result
	parent protoreflect.Descriptor
	index  int
	proto  *descriptorpb.EnumDescriptorProto
	fqn    string

	values enValDescriptors

	rsvdRanges enumRanges
	rsvdNames  names
}

var _ protoreflect.EnumDescriptor = (*enumDescriptor)(nil)
var _ protoutil.DescriptorProtoWrapper = (*enumDescriptor)(nil)

func (r *result) createEnumDescriptor(ed *descriptorpb.EnumDescriptorProto, parent protoreflect.Descriptor, index int, fqn string) *enumDescriptor {
	ret := &enumDescriptor{EnumDescriptor: noOpEnum, file: r, parent: parent, index: index, proto: ed, fqn: fqn}
	r.descriptors[fqn] = ret

	// Unlike all other elements, the fully-qualified name of enum values
	// is NOT scoped to their parent element (the enum), but rather to
	// the enum's parent element. This follows C++ scoping rules for
	// enum values.
	prefix := strings.TrimSuffix(fqn, ed.GetName())
	ret.values = r.createEnumValues(prefix, ret, ed.Value)
	ret.rsvdRanges = createEnumRanges(ed.ReservedRange)
	ret.rsvdNames = names{s: ed.ReservedName}
	return ret
}

func (e *enumDescriptor) EnumDescriptorProto() *descriptorpb.EnumDescriptorProto {
	return e.proto
}

func (e *enumDescriptor) AsProto() proto.Message {
	return e.proto
}

func (e *enumDescriptor) ParentFile() protoreflect.FileDescriptor {
	return e.file
}

func (e *enumDescriptor) Parent() protoreflect.Descriptor {
	return e.parent
}

func (e *enumDescriptor) Index() int {
	return e.index
}

func (e *enumDescriptor) Syntax() protoreflect.Syntax {
	return e.file.Syntax()
}

func (e *enumDescriptor) Name() protoreflect.Name {
	return protoreflect.Name(e.proto.GetName())
}

func (e *enumDescriptor) FullName() protoreflect.FullName {
	return protoreflect.FullName(e.fqn)
}

func (e *enumDescriptor) IsPlaceholder() bool {
	return false
}

func (e *enumDescriptor) Options() protoreflect.ProtoMessage {
	return e.proto.Options
}

func (e *enumDescriptor) Values() protoreflect.EnumValueDescriptors {
	return &e.values
}

func (e *enumDescriptor) ReservedNames() protoreflect.Names {
	return e.rsvdNames
}

func (e *enumDescriptor) ReservedRanges() protoreflect.EnumRanges {
	return e.rsvdRanges
}

func (e *enumDescriptor) IsClosed() bool {
	enumType := resolveFeature(e, enumTypeField)
	return descriptorpb.FeatureSet_EnumType(enumType.Enum()) == descriptorpb.FeatureSet_CLOSED
}

type enumRanges struct {
	protoreflect.EnumRanges
	ranges [][2]protoreflect.EnumNumber
}

func createEnumRanges(rangeProtos []*descriptorpb.EnumDescriptorProto_EnumReservedRange) enumRanges {
	ranges := make([][2]protoreflect.EnumNumber, len(rangeProtos))
	for i, r := range rangeProtos {
		ranges[i] = [2]protoreflect.EnumNumber{
			protoreflect.EnumNumber(r.GetStart()),
			protoreflect.EnumNumber(r.GetEnd()),
		}
	}
	return enumRanges{ranges: ranges}
}

func (e enumRanges) Len() int {
	return len(e.ranges)
}

func (e enumRanges) Get(i int) [2]protoreflect.EnumNumber {
	return e.ranges[i]
}

func (e enumRanges) Has(n protoreflect.EnumNumber) bool {
	for _, r := range e.ranges {
		if r[0] <= n && r[1] >= n {
			return true
		}
	}
	return false
}

type enValDescriptors struct {
	protoreflect.EnumValueDescriptors
	vals []*enValDescriptor
}

func (r *result) createEnumValues(prefix string, parent *enumDescriptor, enValProtos []*descriptorpb.EnumValueDescriptorProto) enValDescriptors {
	vals := make([]*enValDescriptor, len(enValProtos))
	for i, enValProto := range enValProtos {
		vals[i] = r.createEnumValueDescriptor(enValProto, parent, i, prefix+enValProto.GetName())
	}
	return enValDescriptors{vals: vals}
}

func (e *enValDescriptors) Len() int {
	return len(e.vals)
}

func (e *enValDescriptors) Get(i int) protoreflect.EnumValueDescriptor {
	return e.vals[i]
}

func (e *enValDescriptors) ByName(s protoreflect.Name) protoreflect.EnumValueDescriptor {
	for _, val := range e.vals {
		if val.Name() == s {
			return val
		}
	}
	return nil
}

func (e *enValDescriptors) ByNumber(n protoreflect.EnumNumber) protoreflect.EnumValueDescriptor {
	for _, val := range e.vals {
		if val.Number() == n {
			return val
		}
	}
	return nil
}

type enValDescriptor struct {
	protoreflect.EnumValueDescriptor
	file   *result
	parent *enumDescriptor
	index  int
	proto  *descriptorpb.EnumValueDescriptorProto
	fqn    string
}

var _ protoreflect.EnumValueDescriptor = (*enValDescriptor)(nil)
var _ protoutil.DescriptorProtoWrapper = (*enValDescriptor)(nil)

func (r *result) createEnumValueDescriptor(ed *descriptorpb.EnumValueDescriptorProto, parent *enumDescriptor, index int, fqn string) *enValDescriptor {
	ret := &enValDescriptor{EnumValueDescriptor: noOpEnumValue, file: r, parent: parent, index: index, proto: ed, fqn: fqn}
	r.descriptors[fqn] = ret
	return ret
}

func (e *enValDescriptor) EnumValueDescriptorProto() *descriptorpb.EnumValueDescriptorProto {
	return e.proto
}

func (e *enValDescriptor) AsProto() proto.Message {
	return e.proto
}

func (e *enValDescriptor) ParentFile() protoreflect.FileDescriptor {
	return e.file
}

func (e *enValDescriptor) Parent() protoreflect.Descriptor {
	return e.parent
}

func (e *enValDescriptor) Index() int {
	return e.index
}

func (e *enValDescriptor) Syntax() protoreflect.Syntax {
	return e.file.Syntax()
}

func (e *enValDescriptor) Name() protoreflect.Name {
	return protoreflect.Name(e.proto.GetName())
}

func (e *enValDescriptor) FullName() protoreflect.FullName {
	return protoreflect.FullName(e.fqn)
}

func (e *enValDescriptor) IsPlaceholder() bool {
	return false
}

func (e *enValDescriptor) Options() protoreflect.ProtoMessage {
	return e.proto.Options
}

func (e *enValDescriptor) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(e.proto.GetNumber())
}

type extDescriptors struct {
	protoreflect.ExtensionDescriptors
	exts []*extTypeDescriptor
}

func (r *result) createExtensions(prefix string, parent protoreflect.Descriptor, extProtos []*descriptorpb.FieldDescriptorProto) extDescriptors {
	exts := make([]*extTypeDescriptor, len(extProtos))
	for i, extProto := range extProtos {
		exts[i] = r.createExtTypeDescriptor(extProto, parent, i, prefix+extProto.GetName())
	}
	return extDescriptors{exts: exts}
}

func (e *extDescriptors) Len() int {
	return len(e.exts)
}

func (e *extDescriptors) Get(i int) protoreflect.ExtensionDescriptor {
	return e.exts[i]
}

func (e *extDescriptors) ByName(s protoreflect.Name) protoreflect.ExtensionDescriptor {
	for _, ext := range e.exts {
		if ext.Name() == s {
			return ext
		}
	}
	return nil
}

type extTypeDescriptor struct {
	protoreflect.ExtensionTypeDescriptor
	field *fldDescriptor
}

var _ protoutil.DescriptorProtoWrapper = &extTypeDescriptor{}

func (r *result) createExtTypeDescriptor(fd *descriptorpb.FieldDescriptorProto, parent protoreflect.Descriptor, index int, fqn string) *extTypeDescriptor {
	ret := &fldDescriptor{FieldDescriptor: noOpExtension, file: r, parent: parent, index: index, proto: fd, fqn: fqn}
	r.descriptors[fqn] = ret
	return &extTypeDescriptor{ExtensionTypeDescriptor: dynamicpb.NewExtensionType(ret).TypeDescriptor(), field: ret}
}

func (e *extTypeDescriptor) FieldDescriptorProto() *descriptorpb.FieldDescriptorProto {
	return e.field.proto
}

func (e *extTypeDescriptor) AsProto() proto.Message {
	return e.field.proto
}

type fldDescriptors struct {
	protoreflect.FieldDescriptors
	fields []*fldDescriptor
}

func (r *result) createFields(prefix string, parent *msgDescriptor, fldProtos []*descriptorpb.FieldDescriptorProto) fldDescriptors {
	fields := make([]*fldDescriptor, len(fldProtos))
	for i, fldProto := range fldProtos {
		fields[i] = r.createFieldDescriptor(fldProto, parent, i, prefix+fldProto.GetName())
	}
	return fldDescriptors{fields: fields}
}

func (f *fldDescriptors) Len() int {
	return len(f.fields)
}

func (f *fldDescriptors) Get(i int) protoreflect.FieldDescriptor {
	return f.fields[i]
}

func (f *fldDescriptors) ByName(s protoreflect.Name) protoreflect.FieldDescriptor {
	for _, fld := range f.fields {
		if fld.Name() == s {
			return fld
		}
	}
	return nil
}

func (f *fldDescriptors) ByJSONName(s string) protoreflect.FieldDescriptor {
	for _, fld := range f.fields {
		if fld.JSONName() == s {
			return fld
		}
	}
	return nil
}

func (f *fldDescriptors) ByTextName(s string) protoreflect.FieldDescriptor {
	return f.ByName(protoreflect.Name(s))
}

func (f *fldDescriptors) ByNumber(n protoreflect.FieldNumber) protoreflect.FieldDescriptor {
	for _, fld := range f.fields {
		if fld.Number() == n {
			return fld
		}
	}
	return nil
}

type fldDescriptor struct {
	protoreflect.FieldDescriptor
	file   *result
	parent protoreflect.Descriptor
	index  int
	proto  *descriptorpb.FieldDescriptorProto
	fqn    string

	msgType  protoreflect.MessageDescriptor
	extendee protoreflect.MessageDescriptor
	enumType protoreflect.EnumDescriptor
	oneof    protoreflect.OneofDescriptor
}

var _ protoreflect.FieldDescriptor = (*fldDescriptor)(nil)
var _ protoutil.DescriptorProtoWrapper = (*fldDescriptor)(nil)

func (r *result) createFieldDescriptor(fd *descriptorpb.FieldDescriptorProto, parent *msgDescriptor, index int, fqn string) *fldDescriptor {
	ret := &fldDescriptor{FieldDescriptor: noOpField, file: r, parent: parent, index: index, proto: fd, fqn: fqn}
	r.descriptors[fqn] = ret
	return ret
}

func (f *fldDescriptor) FieldDescriptorProto() *descriptorpb.FieldDescriptorProto {
	return f.proto
}

func (f *fldDescriptor) AsProto() proto.Message {
	return f.proto
}

func (f *fldDescriptor) ParentFile() protoreflect.FileDescriptor {
	return f.file
}

func (f *fldDescriptor) Parent() protoreflect.Descriptor {
	return f.parent
}

func (f *fldDescriptor) Index() int {
	return f.index
}

func (f *fldDescriptor) Syntax() protoreflect.Syntax {
	return f.file.Syntax()
}

func (f *fldDescriptor) Name() protoreflect.Name {
	return protoreflect.Name(f.proto.GetName())
}

func (f *fldDescriptor) FullName() protoreflect.FullName {
	return protoreflect.FullName(f.fqn)
}

func (f *fldDescriptor) IsPlaceholder() bool {
	return false
}

func (f *fldDescriptor) Options() protoreflect.ProtoMessage {
	return f.proto.Options
}

func (f *fldDescriptor) Number() protoreflect.FieldNumber {
	return protoreflect.FieldNumber(f.proto.GetNumber())
}

func (f *fldDescriptor) Cardinality() protoreflect.Cardinality {
	switch f.proto.GetLabel() {
	case descriptorpb.FieldDescriptorProto_LABEL_REPEATED:
		return protoreflect.Repeated
	case descriptorpb.FieldDescriptorProto_LABEL_REQUIRED:
		return protoreflect.Required
	case descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL:
		if f.Syntax() == protoreflect.Editions {
			// Editions does not use label to indicate required. It instead
			// uses a feature, and label is always optional.
			fieldPresence := descriptorpb.FeatureSet_FieldPresence(resolveFeature(f, fieldPresenceField).Enum())
			if fieldPresence == descriptorpb.FeatureSet_LEGACY_REQUIRED {
				return protoreflect.Required
			}
		}
		return protoreflect.Optional
	default:
		return 0
	}
}

func (f *fldDescriptor) Kind() protoreflect.Kind {
	if f.proto.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE && f.Syntax() == protoreflect.Editions {
		// In editions, "group encoding" (aka "delimited encoding") is toggled
		// via a feature. So we report group kind when that feature is enabled.
		messageEncoding := resolveFeature(f, messageEncodingField)
		if descriptorpb.FeatureSet_MessageEncoding(messageEncoding.Enum()) == descriptorpb.FeatureSet_DELIMITED {
			return protoreflect.GroupKind
		}
	}
	return protoreflect.Kind(f.proto.GetType())
}

func (f *fldDescriptor) HasJSONName() bool {
	return f.proto.JsonName != nil
}

func (f *fldDescriptor) JSONName() string {
	if f.IsExtension() {
		return f.TextName()
	}
	return f.proto.GetJsonName()
}

func (f *fldDescriptor) TextName() string {
	if f.IsExtension() {
		return fmt.Sprintf("[%s]", f.FullName())
	}
	return string(f.Name())
}

func (f *fldDescriptor) HasPresence() bool {
	if f.proto.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
		return false
	}
	if f.IsExtension() ||
		f.Kind() == protoreflect.MessageKind || f.Kind() == protoreflect.GroupKind ||
		f.proto.OneofIndex != nil {
		return true
	}
	fieldPresence := descriptorpb.FeatureSet_FieldPresence(resolveFeature(f, fieldPresenceField).Enum())
	return fieldPresence == descriptorpb.FeatureSet_EXPLICIT || fieldPresence == descriptorpb.FeatureSet_LEGACY_REQUIRED
}

func (f *fldDescriptor) IsExtension() bool {
	return f.proto.GetExtendee() != ""
}

func (f *fldDescriptor) HasOptionalKeyword() bool {
	if f.proto.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL {
		return false
	}
	if f.proto.GetProto3Optional() {
		// NB: This smells weird to return false here. If the proto3_optional field
		// is set, it's because the keyword WAS present. However, the Go runtime
		// returns false for this case, so we mirror that behavior.
		return !f.IsExtension()
	}
	// If it's optional, but not a proto3 optional, then the keyword is only
	// present for proto2 files, for fields that are not part of a oneof.
	return f.file.Syntax() == protoreflect.Proto2 && f.proto.OneofIndex == nil
}

func (f *fldDescriptor) IsWeak() bool {
	return f.proto.Options.GetWeak()
}

func (f *fldDescriptor) IsPacked() bool {
	if f.Cardinality() != protoreflect.Repeated || !internal.CanPack(f.Kind()) {
		return false
	}
	opts := f.proto.GetOptions()
	if opts != nil && opts.Packed != nil {
		// packed option is set explicitly
		return *opts.Packed
	}
	fieldEncoding := resolveFeature(f, repeatedFieldEncodingField)
	return descriptorpb.FeatureSet_RepeatedFieldEncoding(fieldEncoding.Enum()) == descriptorpb.FeatureSet_PACKED
}

func (f *fldDescriptor) IsList() bool {
	if f.proto.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
		return false
	}
	return !f.isMapEntry()
}

func (f *fldDescriptor) IsMap() bool {
	if f.proto.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
		return false
	}
	if f.IsExtension() {
		return false
	}
	return f.isMapEntry()
}

func (f *fldDescriptor) isMapEntry() bool {
	if f.proto.GetType() != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
		return false
	}
	return f.Message().IsMapEntry()
}

func (f *fldDescriptor) MapKey() protoreflect.FieldDescriptor {
	if !f.IsMap() {
		return nil
	}
	return f.Message().Fields().ByNumber(1)
}

func (f *fldDescriptor) MapValue() protoreflect.FieldDescriptor {
	if !f.IsMap() {
		return nil
	}
	return f.Message().Fields().ByNumber(2)
}

func (f *fldDescriptor) HasDefault() bool {
	return f.proto.DefaultValue != nil
}

func (f *fldDescriptor) Default() protoreflect.Value {
	// We only return a valid value for scalar fields
	if f.proto.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED ||
		f.Kind() == protoreflect.GroupKind || f.Kind() == protoreflect.MessageKind {
		return protoreflect.Value{}
	}

	if f.proto.DefaultValue != nil {
		defVal := f.parseDefaultValue(f.proto.GetDefaultValue())
		if defVal.IsValid() {
			return defVal
		}
		// if we cannot parse a valid value, fall back to zero value below
	}

	// No custom default value, so return the zero value for the type
	switch f.Kind() {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return protoreflect.ValueOfInt32(0)
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(0)
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return protoreflect.ValueOfUint32(0)
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return protoreflect.ValueOfUint64(0)
	case protoreflect.FloatKind:
		return protoreflect.ValueOfFloat32(0)
	case protoreflect.DoubleKind:
		return protoreflect.ValueOfFloat64(0)
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(false)
	case protoreflect.BytesKind:
		return protoreflect.ValueOfBytes(nil)
	case protoreflect.StringKind:
		return protoreflect.ValueOfString("")
	case protoreflect.EnumKind:
		return protoreflect.ValueOfEnum(f.Enum().Values().Get(0).Number())
	case protoreflect.GroupKind, protoreflect.MessageKind:
		return protoreflect.ValueOfMessage(dynamicpb.NewMessage(f.Message()))
	default:
		panic(fmt.Sprintf("unknown kind: %v", f.Kind()))
	}
}

func (f *fldDescriptor) parseDefaultValue(val string) protoreflect.Value {
	switch f.Kind() {
	case protoreflect.EnumKind:
		vd := f.Enum().Values().ByName(protoreflect.Name(val))
		if vd != nil {
			return protoreflect.ValueOfEnum(vd.Number())
		}
		return protoreflect.Value{}
	case protoreflect.BoolKind:
		switch val {
		case "true":
			return protoreflect.ValueOfBool(true)
		case "false":
			return protoreflect.ValueOfBool(false)
		default:
			return protoreflect.Value{}
		}
	case protoreflect.BytesKind:
		return protoreflect.ValueOfBytes([]byte(unescape(val)))
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(val)
	case protoreflect.FloatKind:
		if f, err := strconv.ParseFloat(val, 32); err == nil {
			return protoreflect.ValueOfFloat32(float32(f))
		}
		return protoreflect.Value{}
	case protoreflect.DoubleKind:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return protoreflect.ValueOfFloat64(f)
		}
		return protoreflect.Value{}
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		if i, err := strconv.ParseInt(val, 10, 32); err == nil {
			return protoreflect.ValueOfInt32(int32(i))
		}
		return protoreflect.Value{}
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		if i, err := strconv.ParseUint(val, 10, 32); err == nil {
			return protoreflect.ValueOfUint32(uint32(i))
		}
		return protoreflect.Value{}
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return protoreflect.ValueOfInt64(i)
		}
		return protoreflect.Value{}
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		if i, err := strconv.ParseUint(val, 10, 64); err == nil {
			return protoreflect.ValueOfUint64(i)
		}
		return protoreflect.Value{}
	default:
		return protoreflect.Value{}
	}
}

func unescape(s string) string {
	// protoc encodes default values for 'bytes' fields using C escaping,
	// so this function reverses that escaping
	out := make([]byte, 0, len(s))
	var buf [4]byte
	for len(s) > 0 {
		if s[0] != '\\' || len(s) < 2 {
			// not escape sequence, or too short to be well-formed escape
			out = append(out, s[0])
			s = s[1:]
			continue
		}
		nextIndex := 2 // by default, skip '\' + escaped character
		switch s[1] {
		case 'x', 'X':
			n := matchPrefix(s[2:], 2, isHex)
			if n == 0 {
				// bad escape
				out = append(out, s[:2]...)
			} else {
				c, err := strconv.ParseUint(s[2:2+n], 16, 8)
				if err != nil {
					// shouldn't really happen...
					out = append(out, s[:2+n]...)
				} else {
					out = append(out, byte(c))
				}
				nextIndex = 2 + n
			}
		case '0', '1', '2', '3', '4', '5', '6', '7':
			n := 1 + matchPrefix(s[2:], 2, isOctal)
			c, err := strconv.ParseUint(s[1:1+n], 8, 8)
			if err != nil || c > 0xff {
				out = append(out, s[:1+n]...)
			} else {
				out = append(out, byte(c))
			}
			nextIndex = 1 + n
		case 'u':
			if len(s) < 6 {
				// bad escape
				out = append(out, s...)
				nextIndex = len(s)
			} else {
				c, err := strconv.ParseUint(s[2:6], 16, 16)
				if err != nil {
					// bad escape
					out = append(out, s[:6]...)
				} else {
					w := utf8.EncodeRune(buf[:], rune(c))
					out = append(out, buf[:w]...)
				}
				nextIndex = 6
			}
		case 'U':
			if len(s) < 10 {
				// bad escape
				out = append(out, s...)
				nextIndex = len(s)
			} else {
				c, err := strconv.ParseUint(s[2:10], 16, 32)
				if err != nil || c > 0x10ffff {
					// bad escape
					out = append(out, s[:10]...)
				} else {
					w := utf8.EncodeRune(buf[:], rune(c))
					out = append(out, buf[:w]...)
				}
				nextIndex = 10
			}
		case 'a':
			out = append(out, '\a')
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'v':
			out = append(out, '\v')
		case '\\', '\'', '"', '?':
			out = append(out, s[1])
		default:
			// invalid escape, just copy it as-is
			out = append(out, s[:2]...)
		}
		s = s[nextIndex:]
	}
	return string(out)
}

func isOctal(b byte) bool { return b >= '0' && b <= '7' }
func isHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}
func matchPrefix(s string, limit int, fn func(byte) bool) int {
	l := len(s)
	if l > limit {
		l = limit
	}
	i := 0
	for ; i < l; i++ {
		if !fn(s[i]) {
			return i
		}
	}
	return i
}

func (f *fldDescriptor) DefaultEnumValue() protoreflect.EnumValueDescriptor {
	ed := f.Enum()
	if ed == nil {
		return nil
	}
	if f.proto.DefaultValue != nil {
		if val := ed.Values().ByName(protoreflect.Name(f.proto.GetDefaultValue())); val != nil {
			return val
		}
	}
	// if no default specified in source, return nil
	return nil
}

func (f *fldDescriptor) ContainingOneof() protoreflect.OneofDescriptor {
	return f.oneof
}

func (f *fldDescriptor) ContainingMessage() protoreflect.MessageDescriptor {
	if f.extendee != nil {
		return f.extendee
	}
	return f.parent.(protoreflect.MessageDescriptor)
}

func (f *fldDescriptor) Enum() protoreflect.EnumDescriptor {
	return f.enumType
}

func (f *fldDescriptor) Message() protoreflect.MessageDescriptor {
	return f.msgType
}

type oneofDescriptors struct {
	protoreflect.OneofDescriptors
	oneofs []*oneofDescriptor
}

func (r *result) createOneofs(prefix string, parent *msgDescriptor, ooProtos []*descriptorpb.OneofDescriptorProto) oneofDescriptors {
	oos := make([]*oneofDescriptor, len(ooProtos))
	for i, fldProto := range ooProtos {
		oos[i] = r.createOneofDescriptor(fldProto, parent, i, prefix+fldProto.GetName())
	}
	return oneofDescriptors{oneofs: oos}
}

func (o *oneofDescriptors) Len() int {
	return len(o.oneofs)
}

func (o *oneofDescriptors) Get(i int) protoreflect.OneofDescriptor {
	return o.oneofs[i]
}

func (o *oneofDescriptors) ByName(s protoreflect.Name) protoreflect.OneofDescriptor {
	for _, oo := range o.oneofs {
		if oo.Name() == s {
			return oo
		}
	}
	return nil
}

type oneofDescriptor struct {
	protoreflect.OneofDescriptor
	file   *result
	parent *msgDescriptor
	index  int
	proto  *descriptorpb.OneofDescriptorProto
	fqn    string

	fields fldDescriptors
}

var _ protoreflect.OneofDescriptor = (*oneofDescriptor)(nil)
var _ protoutil.DescriptorProtoWrapper = (*oneofDescriptor)(nil)

func (r *result) createOneofDescriptor(ood *descriptorpb.OneofDescriptorProto, parent *msgDescriptor, index int, fqn string) *oneofDescriptor {
	ret := &oneofDescriptor{OneofDescriptor: noOpOneof, file: r, parent: parent, index: index, proto: ood, fqn: fqn}
	r.descriptors[fqn] = ret

	var fields []*fldDescriptor
	for _, fld := range parent.fields.fields {
		if fld.proto.OneofIndex != nil && int(fld.proto.GetOneofIndex()) == index {
			fields = append(fields, fld)
		}
	}
	ret.fields = fldDescriptors{fields: fields}

	return ret
}

func (o *oneofDescriptor) OneofDescriptorProto() *descriptorpb.OneofDescriptorProto {
	return o.proto
}

func (o *oneofDescriptor) AsProto() proto.Message {
	return o.proto
}

func (o *oneofDescriptor) ParentFile() protoreflect.FileDescriptor {
	return o.file
}

func (o *oneofDescriptor) Parent() protoreflect.Descriptor {
	return o.parent
}

func (o *oneofDescriptor) Index() int {
	return o.index
}

func (o *oneofDescriptor) Syntax() protoreflect.Syntax {
	return o.file.Syntax()
}

func (o *oneofDescriptor) Name() protoreflect.Name {
	return protoreflect.Name(o.proto.GetName())
}

func (o *oneofDescriptor) FullName() protoreflect.FullName {
	return protoreflect.FullName(o.fqn)
}

func (o *oneofDescriptor) IsPlaceholder() bool {
	return false
}

func (o *oneofDescriptor) Options() protoreflect.ProtoMessage {
	return o.proto.Options
}

func (o *oneofDescriptor) IsSynthetic() bool {
	for _, fld := range o.parent.proto.GetField() {
		if fld.OneofIndex != nil && int(fld.GetOneofIndex()) == o.index {
			return fld.GetProto3Optional()
		}
	}
	return false // NB: we should never get here
}

func (o *oneofDescriptor) Fields() protoreflect.FieldDescriptors {
	return &o.fields
}

type svcDescriptors struct {
	protoreflect.ServiceDescriptors
	svcs []*svcDescriptor
}

func (r *result) createServices(prefix string, svcProtos []*descriptorpb.ServiceDescriptorProto) svcDescriptors {
	svcs := make([]*svcDescriptor, len(svcProtos))
	for i, svcProto := range svcProtos {
		svcs[i] = r.createServiceDescriptor(svcProto, i, prefix+svcProto.GetName())
	}
	return svcDescriptors{svcs: svcs}
}

func (s *svcDescriptors) Len() int {
	return len(s.svcs)
}

func (s *svcDescriptors) Get(i int) protoreflect.ServiceDescriptor {
	return s.svcs[i]
}

func (s *svcDescriptors) ByName(n protoreflect.Name) protoreflect.ServiceDescriptor {
	for _, svc := range s.svcs {
		if svc.Name() == n {
			return svc
		}
	}
	return nil
}

type svcDescriptor struct {
	protoreflect.ServiceDescriptor
	file  *result
	index int
	proto *descriptorpb.ServiceDescriptorProto
	fqn   string

	methods mtdDescriptors
}

var _ protoreflect.ServiceDescriptor = (*svcDescriptor)(nil)
var _ protoutil.DescriptorProtoWrapper = (*svcDescriptor)(nil)

func (r *result) createServiceDescriptor(sd *descriptorpb.ServiceDescriptorProto, index int, fqn string) *svcDescriptor {
	ret := &svcDescriptor{ServiceDescriptor: noOpService, file: r, index: index, proto: sd, fqn: fqn}
	r.descriptors[fqn] = ret

	prefix := fqn + "."
	ret.methods = r.createMethods(prefix, ret, sd.Method)

	return ret
}

func (s *svcDescriptor) ServiceDescriptorProto() *descriptorpb.ServiceDescriptorProto {
	return s.proto
}

func (s *svcDescriptor) AsProto() proto.Message {
	return s.proto
}

func (s *svcDescriptor) ParentFile() protoreflect.FileDescriptor {
	return s.file
}

func (s *svcDescriptor) Parent() protoreflect.Descriptor {
	return s.file
}

func (s *svcDescriptor) Index() int {
	return s.index
}

func (s *svcDescriptor) Syntax() protoreflect.Syntax {
	return s.file.Syntax()
}

func (s *svcDescriptor) Name() protoreflect.Name {
	return protoreflect.Name(s.proto.GetName())
}

func (s *svcDescriptor) FullName() protoreflect.FullName {
	return protoreflect.FullName(s.fqn)
}

func (s *svcDescriptor) IsPlaceholder() bool {
	return false
}

func (s *svcDescriptor) Options() protoreflect.ProtoMessage {
	return s.proto.Options
}

func (s *svcDescriptor) Methods() protoreflect.MethodDescriptors {
	return &s.methods
}

type mtdDescriptors struct {
	protoreflect.MethodDescriptors
	mtds []*mtdDescriptor
}

func (r *result) createMethods(prefix string, parent *svcDescriptor, mtdProtos []*descriptorpb.MethodDescriptorProto) mtdDescriptors {
	mtds := make([]*mtdDescriptor, len(mtdProtos))
	for i, mtdProto := range mtdProtos {
		mtds[i] = r.createMethodDescriptor(mtdProto, parent, i, prefix+mtdProto.GetName())
	}
	return mtdDescriptors{mtds: mtds}
}

func (m *mtdDescriptors) Len() int {
	return len(m.mtds)
}

func (m *mtdDescriptors) Get(i int) protoreflect.MethodDescriptor {
	return m.mtds[i]
}

func (m *mtdDescriptors) ByName(n protoreflect.Name) protoreflect.MethodDescriptor {
	for _, mtd := range m.mtds {
		if mtd.Name() == n {
			return mtd
		}
	}
	return nil
}

type mtdDescriptor struct {
	protoreflect.MethodDescriptor
	file   *result
	parent *svcDescriptor
	index  int
	proto  *descriptorpb.MethodDescriptorProto
	fqn    string

	inputType, outputType protoreflect.MessageDescriptor
}

var _ protoreflect.MethodDescriptor = (*mtdDescriptor)(nil)
var _ protoutil.DescriptorProtoWrapper = (*mtdDescriptor)(nil)

func (r *result) createMethodDescriptor(mtd *descriptorpb.MethodDescriptorProto, parent *svcDescriptor, index int, fqn string) *mtdDescriptor {
	ret := &mtdDescriptor{MethodDescriptor: noOpMethod, file: r, parent: parent, index: index, proto: mtd, fqn: fqn}
	r.descriptors[fqn] = ret
	return ret
}

func (m *mtdDescriptor) MethodDescriptorProto() *descriptorpb.MethodDescriptorProto {
	return m.proto
}

func (m *mtdDescriptor) AsProto() proto.Message {
	return m.proto
}

func (m *mtdDescriptor) ParentFile() protoreflect.FileDescriptor {
	return m.file
}

func (m *mtdDescriptor) Parent() protoreflect.Descriptor {
	return m.parent
}

func (m *mtdDescriptor) Index() int {
	return m.index
}

func (m *mtdDescriptor) Syntax() protoreflect.Syntax {
	return m.file.Syntax()
}

func (m *mtdDescriptor) Name() protoreflect.Name {
	return protoreflect.Name(m.proto.GetName())
}

func (m *mtdDescriptor) FullName() protoreflect.FullName {
	return protoreflect.FullName(m.fqn)
}

func (m *mtdDescriptor) IsPlaceholder() bool {
	return false
}

func (m *mtdDescriptor) Options() protoreflect.ProtoMessage {
	return m.proto.Options
}

func (m *mtdDescriptor) Input() protoreflect.MessageDescriptor {
	return m.inputType
}

func (m *mtdDescriptor) Output() protoreflect.MessageDescriptor {
	return m.outputType
}

func (m *mtdDescriptor) IsStreamingClient() bool {
	return m.proto.GetClientStreaming()
}

func (m *mtdDescriptor) IsStreamingServer() bool {
	return m.proto.GetServerStreaming()
}

func (r *result) FindImportByPath(path string) File {
	return r.deps.FindFileByPath(path)
}

func (r *result) FindExtensionByNumber(msg protoreflect.FullName, tag protoreflect.FieldNumber) protoreflect.ExtensionTypeDescriptor {
	return findExtension(r, msg, tag)
}

func (r *result) FindDescriptorByName(name protoreflect.FullName) protoreflect.Descriptor {
	fqn := strings.TrimPrefix(string(name), ".")
	return r.descriptors[fqn]
}

func (r *result) hasSource() bool {
	n := r.FileNode()
	_, ok := n.(*ast.FileNode)
	return ok
}
