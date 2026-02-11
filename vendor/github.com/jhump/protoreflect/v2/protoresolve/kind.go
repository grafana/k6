package protoresolve

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// DescriptorKind represents the kind of a descriptor. Unlike other
// descriptor-related APIs, DescriptorKind distinguishes between
// extension fields (DescriptorKindExtension) and "regular", non-extension
// fields (DescriptorKindField).
type DescriptorKind int

// The various supported DescriptorKind values.
const (
	DescriptorKindUnknown = DescriptorKind(iota)
	DescriptorKindFile
	DescriptorKindMessage
	DescriptorKindField
	DescriptorKindOneof
	DescriptorKindEnum
	DescriptorKindEnumValue
	DescriptorKindExtension
	DescriptorKindService
	DescriptorKindMethod
)

// KindOf returns the DescriptorKind of the given descriptor d.
func KindOf(d protoreflect.Descriptor) DescriptorKind {
	switch d := d.(type) {
	case protoreflect.FileDescriptor:
		return DescriptorKindFile
	case protoreflect.MessageDescriptor:
		return DescriptorKindMessage
	case protoreflect.FieldDescriptor:
		if d.IsExtension() {
			return DescriptorKindExtension
		}
		return DescriptorKindField
	case protoreflect.OneofDescriptor:
		return DescriptorKindOneof
	case protoreflect.EnumDescriptor:
		return DescriptorKindEnum
	case protoreflect.EnumValueDescriptor:
		return DescriptorKindEnumValue
	case protoreflect.ServiceDescriptor:
		return DescriptorKindService
	case protoreflect.MethodDescriptor:
		return DescriptorKindMethod
	default:
		return DescriptorKindUnknown
	}
}

// String returns a textual representation of k.
func (k DescriptorKind) String() string {
	switch k {
	case DescriptorKindFile:
		return "file"
	case DescriptorKindMessage:
		return "message"
	case DescriptorKindField:
		return "field"
	case DescriptorKindOneof:
		return "oneof"
	case DescriptorKindEnum:
		return "enum"
	case DescriptorKindEnumValue:
		return "enum value"
	case DescriptorKindExtension:
		return "extension"
	case DescriptorKindService:
		return "service"
	case DescriptorKindMethod:
		return "method"
	case DescriptorKindUnknown:
		return "unknown"
	default:
		return fmt.Sprintf("unknown kind (%d)", k)
	}
}

func (k DescriptorKind) withArticle() string {
	switch k {
	case DescriptorKindFile:
		return "a file"
	case DescriptorKindMessage:
		return "a message"
	case DescriptorKindField:
		return "a field"
	case DescriptorKindOneof:
		return "a oneof"
	case DescriptorKindEnum:
		return "an enum"
	case DescriptorKindEnumValue:
		return "an enum value"
	case DescriptorKindExtension:
		return "an extension"
	case DescriptorKindService:
		return "a service"
	case DescriptorKindMethod:
		return "a method"
	case DescriptorKindUnknown:
		return "unknown"
	default:
		return fmt.Sprintf("unknown kind (%d)", k)
	}
}
