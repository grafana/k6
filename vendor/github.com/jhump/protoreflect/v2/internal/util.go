package internal

import (
	"math"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/reflect/protoreflect"
)

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
	// FileSyntaxTag is the tag number of the syntax element in a file
	// descriptor proto.
	FileSyntaxTag = 12
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
	// MessageExtensionRangeTag is the tag number of the extension ranges
	// element in a message descriptor proto.
	MessageExtensionRangeTag = 5
	// MessageExtensionsTag is the tag number of the extensions element in a
	// message descriptor proto.
	MessageExtensionsTag = 6
	// MessageOptionsTag is the tag number of the options element in a message
	// descriptor proto.
	MessageOptionsTag = 7
	// MessageOneofsTag is the tag number of the one-ofs element in a message
	// descriptor proto.
	MessageOneofsTag = 8
	// MessageReservedRangeTag is the tag number of the reserved ranges element
	// in a message descriptor proto.
	MessageReservedRangeTag = 9
	// MessageReservedNameTag is the tag number of the reserved names element
	// in a message descriptor proto.
	MessageReservedNameTag = 10
	// ExtensionRangeStartTag is the tag number of the start index in an
	// extension range proto.
	ExtensionRangeStartTag = 1
	// ExtensionRangeEndTag is the tag number of the end index in an
	// extension range proto.
	ExtensionRangeEndTag = 2
	// ExtensionRangeOptionsTag is the tag number of the options element in an
	// extension range proto.
	ExtensionRangeOptionsTag = 3
	// ReservedRangeStartTag is the tag number of the start index in a reserved
	// range proto.
	ReservedRangeStartTag = 1
	// ReservedRangeEndTag is the tag number of the end index in a reserved
	// range proto.
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
	// EnumNameTag is the tag number of the name element in an enum descriptor
	// proto.
	EnumNameTag = 1
	// EnumValuesTag is the tag number of the values element in an enum
	// descriptor proto.
	EnumValuesTag = 2
	// EnumOptionsTag is the tag number of the options element in an enum
	// descriptor proto.
	EnumOptionsTag = 3
	// EnumReservedRangeTag is the tag number of the reserved ranges element in
	// an enum descriptor proto.
	EnumReservedRangeTag = 4
	// EnumReservedNameTag is the tag number of the reserved names element in
	// an enum descriptor proto.
	EnumReservedNameTag = 5
	// EnumValueNameTag is the tag number of the name element in an enum value
	// descriptor proto.
	EnumValueNameTag = 1
	// EnumValueNumberTag is the tag number of the number element in an enum
	// value descriptor proto.
	EnumValueNumberTag = 2
	// EnumValueOptionsTag is the tag number of the options element in an enum
	// value descriptor proto.
	EnumValueOptionsTag = 3
	// ServiceNameTag is the tag number of the name element in a service
	// descriptor proto.
	ServiceNameTag = 1
	// ServiceMethodsTag is the tag number of the methods element in a service
	// descriptor proto.
	ServiceMethodsTag = 2
	// ServiceOptionsTag is the tag number of the options element in a service
	// descriptor proto.
	ServiceOptionsTag = 3
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

	// UninterpretedOptionNameTag is the tag number of the name element in an
	// uninterpreted options proto.
	UninterpretedOptionNameTag = 2
	// UninterpretedOptionIdentTag is the tag number of the identifier value in an
	// uninterpreted options proto.
	UninterpretedOptionIdentTag = 3
	// UninterpretedOptionPosIntTag is the tag number of the positive int value in an
	// uninterpreted options proto.
	UninterpretedOptionPosIntTag = 4
	// UninterpretedOptionNegIntTag is the tag number of the negative int value in an
	// uninterpreted options proto.
	UninterpretedOptionNegIntTag = 5
	// UninterpretedOptionDoubleTag is the tag number of the double value in an
	// uninterpreted options proto.
	UninterpretedOptionDoubleTag = 6
	// UninterpretedOptionStringTag is the tag number of the string value in an
	// uninterpreted options proto.
	UninterpretedOptionStringTag = 7
	// UninterpretedOptionAggregateTag is the tag number of the aggregate value in an
	// uninterpreted options proto.
	UninterpretedOptionAggregateTag = 8
	// UninterpretedNameNameTag is the tag number of the name element in an
	// uninterpreted option name proto.
	UninterpretedNameNameTag = 1
)

// JsonName returns the default JSON name for a field with the given name.
// This mirrors the algorithm in protoc:
//
//	https://github.com/protocolbuffers/protobuf/blob/v21.3/src/google/protobuf/descriptor.cc#L95
func JsonName(name protoreflect.Name) string {
	var js []rune
	nextUpper := false
	for _, r := range name {
		if r == '_' {
			nextUpper = true
			continue
		}
		if nextUpper {
			nextUpper = false
			js = append(js, unicode.ToUpper(r))
		} else {
			js = append(js, r)
		}
	}
	return string(js)
}

// InitCap returns the given field name, but with the first letter capitalized.
func InitCap(name string) string {
	r, sz := utf8.DecodeRuneInString(name)
	return string(unicode.ToUpper(r)) + name[sz:]
}

// GetMaxTag returns the max tag number allowed, based on whether a message uses
// message set wire format or not.
func GetMaxTag(isMessageSet bool) protoreflect.FieldNumber {
	if isMessageSet {
		return MaxMessageSetTag
	}
	return MaxNormalTag
}

// PathKey returns a string that corresponds to the given path that can be used
// as a map key.
func PathKey(path protoreflect.SourcePath) string {
	b := make([]byte, len(path)*4)
	j := 0
	for _, s := range path {
		b[j] = byte(s)
		b[j+1] = byte(s >> 8)
		b[j+2] = byte(s >> 16)
		b[j+3] = byte(s >> 24)
		j += 4
	}
	return string(b)
}

// IsMessageKind returns true if the given kind indicates a message type. Both
// messages and groups are message types.
func IsMessageKind(k protoreflect.Kind) bool {
	return k == protoreflect.MessageKind || k == protoreflect.GroupKind
}
