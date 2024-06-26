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

package internal

import "math"

const (
	// MaxNormalTag is the maximum allowed tag number for a field in a normal message.
	MaxNormalTag = 536870911 // 2^29 - 1

	// MaxMessageSetTag is the maximum allowed tag number of a field in a message that
	// uses the message set wire format.
	MaxMessageSetTag = math.MaxInt32 - 1

	// MaxTag is the maximum allowed tag number. (It is the same as MaxMessageSetTag
	// since that is the absolute highest allowed.)
	MaxTag = MaxMessageSetTag

	// SpecialReservedStart is the first tag in a range that is reserved and not
	// allowed for use in message definitions.
	SpecialReservedStart = 19000
	// SpecialReservedEnd is the last tag in a range that is reserved and not
	// allowed for use in message definitions.
	SpecialReservedEnd = 19999

	// NB: It would be nice to use constants from generated code instead of
	// hard-coding these here. But code-gen does not emit these as constants
	// anywhere. The only places they appear in generated code are struct tags
	// on fields of the generated descriptor protos.

	// FilePackageTag is the tag number of the package element in a file
	// descriptor proto.
	FilePackageTag = 2
	// FileDependencyTag is the tag number of the dependencies element in a
	// file descriptor proto.
	FileDependencyTag = 3
	// FileMessagesTag is the tag number of the messages element in a file
	// descriptor proto.
	FileMessagesTag = 4
	// FileEnumsTag is the tag number of the enums element in a file descriptor
	// proto.
	FileEnumsTag = 5
	// FileServicesTag is the tag number of the services element in a file
	// descriptor proto.
	FileServicesTag = 6
	// FileExtensionsTag is the tag number of the extensions element in a file
	// descriptor proto.
	FileExtensionsTag = 7
	// FileOptionsTag is the tag number of the options element in a file
	// descriptor proto.
	FileOptionsTag = 8
	// FileOptionsJavaStringCheckUTF8Tag is the tag number of the java_string_check_utf8
	// field in the FileOptions proto.
	FileOptionsJavaStringCheckUTF8Tag = 27
	// FileOptionsFeaturesTag is the tag number of the features field in the
	// FileOptions proto.
	FileOptionsFeaturesTag = 50
	// FileSourceCodeInfoTag is the tag number of the source code info element
	// in a file descriptor proto.
	FileSourceCodeInfoTag = 9
	// FilePublicDependencyTag is the tag number of the public dependency element
	// in a file descriptor proto.
	FilePublicDependencyTag = 10
	// FileWeakDependencyTag is the tag number of the weak dependency element
	// in a file descriptor proto.
	FileWeakDependencyTag = 11
	// FileSyntaxTag is the tag number of the syntax element in a file
	// descriptor proto.
	FileSyntaxTag = 12
	// FileEditionTag is the tag number of the edition element in a file
	// descriptor proto.
	FileEditionTag = 14
	// MessageNameTag is the tag number of the name element in a message
	// descriptor proto.
	MessageNameTag = 1
	// MessageFieldsTag is the tag number of the fields element in a message
	// descriptor proto.
	MessageFieldsTag = 2
	// MessageNestedMessagesTag is the tag number of the nested messages
	// element in a message descriptor proto.
	MessageNestedMessagesTag = 3
	// MessageEnumsTag is the tag number of the enums element in a message
	// descriptor proto.
	MessageEnumsTag = 4
	// MessageExtensionRangesTag is the tag number of the extension ranges
	// element in a message descriptor proto.
	MessageExtensionRangesTag = 5
	// MessageExtensionsTag is the tag number of the extensions element in a
	// message descriptor proto.
	MessageExtensionsTag = 6
	// MessageOptionsTag is the tag number of the options element in a message
	// descriptor proto.
	MessageOptionsTag = 7
	// MessageOptionsFeaturesTag is the tag number of the features field in the
	// MessageOptions proto.
	MessageOptionsFeaturesTag = 12
	// MessageOneofsTag is the tag number of the one-ofs element in a message
	// descriptor proto.
	MessageOneofsTag = 8
	// MessageReservedRangesTag is the tag number of the reserved ranges element
	// in a message descriptor proto.
	MessageReservedRangesTag = 9
	// MessageReservedNamesTag is the tag number of the reserved names element
	// in a message descriptor proto.
	MessageReservedNamesTag = 10
	// ExtensionRangeStartTag is the tag number of the start index in an
	// extension range proto.
	ExtensionRangeStartTag = 1
	// ExtensionRangeEndTag is the tag number of the end index in an
	// extension range proto.
	ExtensionRangeEndTag = 2
	// ExtensionRangeOptionsTag is the tag number of the options element in an
	// extension range proto.
	ExtensionRangeOptionsTag = 3
	// ExtensionRangeOptionsDeclarationTag is the tag number of the declaration
	// field in the ExtensionRangeOptions proto.
	ExtensionRangeOptionsDeclarationTag = 2
	// ExtensionRangeOptionsVerificationTag is the tag number of the verification
	// field in the ExtensionRangeOptions proto.
	ExtensionRangeOptionsVerificationTag = 3
	// ExtensionRangeOptionsDeclarationNumberTag is the tag number of the number
	// field in the ExtensionRangeOptions.Declaration proto.
	ExtensionRangeOptionsDeclarationNumberTag = 1
	// ExtensionRangeOptionsDeclarationFullNameTag is the tag number of the full_name
	// field in the ExtensionRangeOptions.Declaration proto.
	ExtensionRangeOptionsDeclarationFullNameTag = 2
	// ExtensionRangeOptionsDeclarationTypeTag is the tag number of the type
	// field in the ExtensionRangeOptions.Declaration proto.
	ExtensionRangeOptionsDeclarationTypeTag = 3
	// ExtensionRangeOptionsDeclarationReservedTag is the tag number of the reserved
	// field in the ExtensionRangeOptions.Declaration proto.
	ExtensionRangeOptionsDeclarationReservedTag = 5
	// ExtensionRangeOptionsDeclarationRepeatedTag is the tag number of the repeated
	// field in the ExtensionRangeOptions.Declaration proto.
	ExtensionRangeOptionsDeclarationRepeatedTag = 6
	// ExtensionRangeOptionsFeaturesTag is the tag number of the features field in the
	// ExtensionRangeOptions proto.
	ExtensionRangeOptionsFeaturesTag = 50
	// ReservedRangeStartTag is the tag number of the start index in a reserved
	// range proto. This field number is the same for both "flavors" of reserved
	// ranges: DescriptorProto.ReservedRange and EnumDescriptorProto.EnumReservedRange.
	ReservedRangeStartTag = 1
	// ReservedRangeEndTag is the tag number of the end index in a reserved
	// range proto. This field number is the same for both "flavors" of reserved
	// ranges: DescriptorProto.ReservedRange and EnumDescriptorProto.EnumReservedRange.
	ReservedRangeEndTag = 2
	// FieldNameTag is the tag number of the name element in a field descriptor
	// proto.
	FieldNameTag = 1
	// FieldExtendeeTag is the tag number of the extendee element in a field
	// descriptor proto.
	FieldExtendeeTag = 2
	// FieldNumberTag is the tag number of the number element in a field
	// descriptor proto.
	FieldNumberTag = 3
	// FieldLabelTag is the tag number of the label element in a field
	// descriptor proto.
	FieldLabelTag = 4
	// FieldTypeTag is the tag number of the type element in a field descriptor
	// proto.
	FieldTypeTag = 5
	// FieldTypeNameTag is the tag number of the type name element in a field
	// descriptor proto.
	FieldTypeNameTag = 6
	// FieldDefaultTag is the tag number of the default value element in a
	// field descriptor proto.
	FieldDefaultTag = 7
	// FieldOptionsTag is the tag number of the options element in a field
	// descriptor proto.
	FieldOptionsTag = 8
	// FieldOptionsCTypeTag is the number of the ctype field in the
	// FieldOptions proto.
	FieldOptionsCTypeTag = 1
	// FieldOptionsPackedTag is the number of the packed field in the
	// FieldOptions proto.
	FieldOptionsPackedTag = 2
	// FieldOptionsLazyTag is the number of the lazy field in the
	// FieldOptions proto.
	FieldOptionsLazyTag = 5
	// FieldOptionsJSTypeTag is the number of the jstype field in the
	// FieldOptions proto.
	FieldOptionsJSTypeTag = 6
	// FieldOptionsUnverifiedLazyTag is the number of the unverified_lazy
	// field in the FieldOptions proto.
	FieldOptionsUnverifiedLazyTag = 15
	// FieldOptionsFeaturesTag is the tag number of the features field in the
	// FieldOptions proto.
	FieldOptionsFeaturesTag = 21
	// FieldOneofIndexTag is the tag number of the oneof index element in a
	// field descriptor proto.
	FieldOneofIndexTag = 9
	// FieldJSONNameTag is the tag number of the JSON name element in a field
	// descriptor proto.
	FieldJSONNameTag = 10
	// FieldProto3OptionalTag is the tag number of the proto3_optional element
	// in a descriptor proto.
	FieldProto3OptionalTag = 17
	// OneofNameTag is the tag number of the name element in a one-of
	// descriptor proto.
	OneofNameTag = 1
	// OneofOptionsTag is the tag number of the options element in a one-of
	// descriptor proto.
	OneofOptionsTag = 2
	// OneofOptionsFeaturesTag is the tag number of the features field in the
	// OneofOptions proto.
	OneofOptionsFeaturesTag = 1
	// EnumNameTag is the tag number of the name element in an enum descriptor
	// proto.
	EnumNameTag = 1
	// EnumValuesTag is the tag number of the values element in an enum
	// descriptor proto.
	EnumValuesTag = 2
	// EnumOptionsTag is the tag number of the options element in an enum
	// descriptor proto.
	EnumOptionsTag = 3
	// EnumOptionsFeaturesTag is the tag number of the features field in the
	// EnumOptions proto.
	EnumOptionsFeaturesTag = 7
	// EnumReservedRangesTag is the tag number of the reserved ranges element in
	// an enum descriptor proto.
	EnumReservedRangesTag = 4
	// EnumReservedNamesTag is the tag number of the reserved names element in
	// an enum descriptor proto.
	EnumReservedNamesTag = 5
	// EnumValNameTag is the tag number of the name element in an enum value
	// descriptor proto.
	EnumValNameTag = 1
	// EnumValNumberTag is the tag number of the number element in an enum
	// value descriptor proto.
	EnumValNumberTag = 2
	// EnumValOptionsTag is the tag number of the options element in an enum
	// value descriptor proto.
	EnumValOptionsTag = 3
	// EnumValOptionsFeaturesTag is the tag number of the features field in the
	// EnumValueOptions proto.
	EnumValOptionsFeaturesTag = 2
	// ServiceNameTag is the tag number of the name element in a service
	// descriptor proto.
	ServiceNameTag = 1
	// ServiceMethodsTag is the tag number of the methods element in a service
	// descriptor proto.
	ServiceMethodsTag = 2
	// ServiceOptionsTag is the tag number of the options element in a service
	// descriptor proto.
	ServiceOptionsTag = 3
	// ServiceOptionsFeaturesTag is the tag number of the features field in the
	// ServiceOptions proto.
	ServiceOptionsFeaturesTag = 34
	// MethodNameTag is the tag number of the name element in a method
	// descriptor proto.
	MethodNameTag = 1
	// MethodInputTag is the tag number of the input type element in a method
	// descriptor proto.
	MethodInputTag = 2
	// MethodOutputTag is the tag number of the output type element in a method
	// descriptor proto.
	MethodOutputTag = 3
	// MethodOptionsTag is the tag number of the options element in a method
	// descriptor proto.
	MethodOptionsTag = 4
	// MethodOptionsFeaturesTag is the tag number of the features field in the
	// MethodOptions proto.
	MethodOptionsFeaturesTag = 35
	// MethodInputStreamTag is the tag number of the input stream flag in a
	// method descriptor proto.
	MethodInputStreamTag = 5
	// MethodOutputStreamTag is the tag number of the output stream flag in a
	// method descriptor proto.
	MethodOutputStreamTag = 6

	// UninterpretedOptionsTag is the tag number of the uninterpreted options
	// element. All *Options messages use the same tag for the field that stores
	// uninterpreted options.
	UninterpretedOptionsTag = 999

	// UninterpretedNameTag is the tag number of the name element in an
	// uninterpreted options proto.
	UninterpretedNameTag = 2
	// UninterpretedIdentTag is the tag number of the identifier value in an
	// uninterpreted options proto.
	UninterpretedIdentTag = 3
	// UninterpretedPosIntTag is the tag number of the positive int value in an
	// uninterpreted options proto.
	UninterpretedPosIntTag = 4
	// UninterpretedNegIntTag is the tag number of the negative int value in an
	// uninterpreted options proto.
	UninterpretedNegIntTag = 5
	// UninterpretedDoubleTag is the tag number of the double value in an
	// uninterpreted options proto.
	UninterpretedDoubleTag = 6
	// UninterpretedStringTag is the tag number of the string value in an
	// uninterpreted options proto.
	UninterpretedStringTag = 7
	// UninterpretedAggregateTag is the tag number of the aggregate value in an
	// uninterpreted options proto.
	UninterpretedAggregateTag = 8
	// UninterpretedNameNameTag is the tag number of the name element in an
	// uninterpreted option name proto.
	UninterpretedNameNameTag = 1

	// AnyTypeURLTag is the tag number of the type_url field of the Any proto.
	AnyTypeURLTag = 1
	// AnyValueTag is the tag number of the value field of the Any proto.
	AnyValueTag = 2

	// FeatureSetFieldPresenceTag is the tag number of the field_presence field
	// in the FeatureSet proto.
	FeatureSetFieldPresenceTag = 1
	// FeatureSetEnumTypeTag is the tag number of the enum_type field in the
	// FeatureSet proto.
	FeatureSetEnumTypeTag = 2
	// FeatureSetRepeatedFieldEncodingTag is the tag number of the repeated_field_encoding
	// field in the FeatureSet proto.
	FeatureSetRepeatedFieldEncodingTag = 3
	// FeatureSetUTF8ValidationTag is the tag number of the utf8_validation field
	// in the FeatureSet proto.
	FeatureSetUTF8ValidationTag = 4
	// FeatureSetMessageEncodingTag is the tag number of the message_encoding
	// field in the FeatureSet proto.
	FeatureSetMessageEncodingTag = 5
	// FeatureSetJSONFormatTag is the tag number of the json_format field in
	// the FeatureSet proto.
	FeatureSetJSONFormatTag = 6
)
