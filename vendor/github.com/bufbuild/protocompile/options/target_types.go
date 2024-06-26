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

package options

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type optionsType[T any] interface {
	*T
	proto.Message
	GetFeatures() *descriptorpb.FeatureSet
	GetUninterpretedOption() []*descriptorpb.UninterpretedOption
}

type elementType[OptsStruct any, Opts optionsType[OptsStruct]] interface {
	proto.Message
	GetOptions() Opts
}

type targetType[Elem elementType[OptsStruct, Opts], OptsStruct any, Opts optionsType[OptsStruct]] struct {
	t                       descriptorpb.FieldOptions_OptionTargetType
	setUninterpretedOptions func(opts Opts, uninterpreted []*descriptorpb.UninterpretedOption)
	setOptions              func(elem Elem, opts Opts)
}

var (
	targetTypeFile = newTargetType[*descriptorpb.FileDescriptorProto](
		descriptorpb.FieldOptions_TARGET_TYPE_FILE, setUninterpretedFileOptions, setFileOptions,
	)
	targetTypeMessage = newTargetType[*descriptorpb.DescriptorProto](
		descriptorpb.FieldOptions_TARGET_TYPE_MESSAGE, setUninterpretedMessageOptions, setMessageOptions,
	)
	targetTypeField = newTargetType[*descriptorpb.FieldDescriptorProto](
		descriptorpb.FieldOptions_TARGET_TYPE_FIELD, setUninterpretedFieldOptions, setFieldOptions,
	)
	targetTypeOneof = newTargetType[*descriptorpb.OneofDescriptorProto](
		descriptorpb.FieldOptions_TARGET_TYPE_ONEOF, setUninterpretedOneofOptions, setOneofOptions,
	)
	targetTypeExtensionRange = newTargetType[*descriptorpb.DescriptorProto_ExtensionRange](
		descriptorpb.FieldOptions_TARGET_TYPE_EXTENSION_RANGE, setUninterpretedExtensionRangeOptions, setExtensionRangeOptions,
	)
	targetTypeEnum = newTargetType[*descriptorpb.EnumDescriptorProto](
		descriptorpb.FieldOptions_TARGET_TYPE_ENUM, setUninterpretedEnumOptions, setEnumOptions,
	)
	targetTypeEnumValue = newTargetType[*descriptorpb.EnumValueDescriptorProto](
		descriptorpb.FieldOptions_TARGET_TYPE_ENUM_ENTRY, setUninterpretedEnumValueOptions, setEnumValueOptions,
	)
	targetTypeService = newTargetType[*descriptorpb.ServiceDescriptorProto](
		descriptorpb.FieldOptions_TARGET_TYPE_SERVICE, setUninterpretedServiceOptions, setServiceOptions,
	)
	targetTypeMethod = newTargetType[*descriptorpb.MethodDescriptorProto](
		descriptorpb.FieldOptions_TARGET_TYPE_METHOD, setUninterpretedMethodOptions, setMethodOptions,
	)
)

func newTargetType[Elem elementType[OptsStruct, Opts], OptsStruct any, Opts optionsType[OptsStruct]](
	t descriptorpb.FieldOptions_OptionTargetType,
	setUninterpretedOptions func(opts Opts, uninterpreted []*descriptorpb.UninterpretedOption),
	setOptions func(elem Elem, opts Opts),
) *targetType[Elem, OptsStruct, Opts] {
	return &targetType[Elem, OptsStruct, Opts]{
		t:                       t,
		setUninterpretedOptions: setUninterpretedOptions,
		setOptions:              setOptions,
	}
}

func setUninterpretedFileOptions(opts *descriptorpb.FileOptions, uninterpreted []*descriptorpb.UninterpretedOption) {
	opts.UninterpretedOption = uninterpreted
}

func setUninterpretedMessageOptions(opts *descriptorpb.MessageOptions, uninterpreted []*descriptorpb.UninterpretedOption) {
	opts.UninterpretedOption = uninterpreted
}

func setUninterpretedFieldOptions(opts *descriptorpb.FieldOptions, uninterpreted []*descriptorpb.UninterpretedOption) {
	opts.UninterpretedOption = uninterpreted
}

func setUninterpretedOneofOptions(opts *descriptorpb.OneofOptions, uninterpreted []*descriptorpb.UninterpretedOption) {
	opts.UninterpretedOption = uninterpreted
}

func setUninterpretedExtensionRangeOptions(opts *descriptorpb.ExtensionRangeOptions, uninterpreted []*descriptorpb.UninterpretedOption) {
	opts.UninterpretedOption = uninterpreted
}

func setUninterpretedEnumOptions(opts *descriptorpb.EnumOptions, uninterpreted []*descriptorpb.UninterpretedOption) {
	opts.UninterpretedOption = uninterpreted
}

func setUninterpretedEnumValueOptions(opts *descriptorpb.EnumValueOptions, uninterpreted []*descriptorpb.UninterpretedOption) {
	opts.UninterpretedOption = uninterpreted
}

func setUninterpretedServiceOptions(opts *descriptorpb.ServiceOptions, uninterpreted []*descriptorpb.UninterpretedOption) {
	opts.UninterpretedOption = uninterpreted
}

func setUninterpretedMethodOptions(opts *descriptorpb.MethodOptions, uninterpreted []*descriptorpb.UninterpretedOption) {
	opts.UninterpretedOption = uninterpreted
}

func setFileOptions(desc *descriptorpb.FileDescriptorProto, opts *descriptorpb.FileOptions) {
	desc.Options = opts
}

func setMessageOptions(desc *descriptorpb.DescriptorProto, opts *descriptorpb.MessageOptions) {
	desc.Options = opts
}

func setFieldOptions(desc *descriptorpb.FieldDescriptorProto, opts *descriptorpb.FieldOptions) {
	desc.Options = opts
}

func setOneofOptions(desc *descriptorpb.OneofDescriptorProto, opts *descriptorpb.OneofOptions) {
	desc.Options = opts
}

func setExtensionRangeOptions(desc *descriptorpb.DescriptorProto_ExtensionRange, opts *descriptorpb.ExtensionRangeOptions) {
	desc.Options = opts
}

func setEnumOptions(desc *descriptorpb.EnumDescriptorProto, opts *descriptorpb.EnumOptions) {
	desc.Options = opts
}

func setEnumValueOptions(desc *descriptorpb.EnumValueDescriptorProto, opts *descriptorpb.EnumValueOptions) {
	desc.Options = opts
}

func setServiceOptions(desc *descriptorpb.ServiceDescriptorProto, opts *descriptorpb.ServiceOptions) {
	desc.Options = opts
}

func setMethodOptions(desc *descriptorpb.MethodDescriptorProto, opts *descriptorpb.MethodOptions) {
	desc.Options = opts
}
