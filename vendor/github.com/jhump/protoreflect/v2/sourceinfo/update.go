package sourceinfo

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// AddSourceInfoToFile will return a new file descriptor that is a copy
// of fd except that it includes source code info. If the given file
// already contains source info, was not registered from generated code,
// or was not processed with the protoc-gen-gosrcinfo plugin, then fd
// is returned as is, unchanged.
func AddSourceInfoToFile(fd protoreflect.FileDescriptor) (protoreflect.FileDescriptor, error) {
	return getFile(fd)
}

// AddSourceInfoToMessage will return a new message descriptor that is a
// copy of md except that it includes source code info. If the file that
// contains the given message descriptor already contains source info,
// was not registered from generated code, or was not processed with the
// protoc-gen-gosrcinfo plugin, then md is returned as is, unchanged.
func AddSourceInfoToMessage(md protoreflect.MessageDescriptor) (protoreflect.MessageDescriptor, error) {
	return updateDescriptor(md)
}

// AddSourceInfoToEnum will return a new enum descriptor that is a copy
// of ed except that it includes source code info. If the file that
// contains the given enum descriptor already contains source info, was
// not registered from generated code, or was not processed with the
// protoc-gen-gosrcinfo plugin, then ed is returned as is, unchanged.
func AddSourceInfoToEnum(ed protoreflect.EnumDescriptor) (protoreflect.EnumDescriptor, error) {
	return updateDescriptor(ed)
}

// AddSourceInfoToField will return a new field descriptor that is a copy
// of ed except that it includes source code info. If the file that
// contains the given enum descriptor already contains source info, was
// not registered from generated code, or was not processed with the
// protoc-gen-gosrcinfo plugin, then ed is returned as is, unchanged.
func AddSourceInfoToField(fld protoreflect.FieldDescriptor) (protoreflect.FieldDescriptor, error) {
	return updateField(fld)
}

// AddSourceInfoToService will return a new service descriptor that is
// a copy of sd except that it includes source code info. If the file
// that contains the given service descriptor already contains source
// info, was not registered from generated code, or was not processed
// with the protoc-gen-gosrcinfo plugin, then ed is returned as is,
// unchanged.
func AddSourceInfoToService(sd protoreflect.ServiceDescriptor) (protoreflect.ServiceDescriptor, error) {
	return updateDescriptor(sd)
}

// AddSourceInfoToExtensionType will return a new extension type that
// is a copy of xt except that its associated descriptor includes
// source code info. If the file that contains the given extension
// already contains source info, was not registered from generated
// code, or was not processed with the protoc-gen-gosrcinfo plugin,
// then xt is returned as is, unchanged.
func AddSourceInfoToExtensionType(xt protoreflect.ExtensionType) (protoreflect.ExtensionType, error) {
	if genTypeDesc, err := protoregistry.GlobalFiles.FindDescriptorByName(xt.TypeDescriptor().FullName()); err != nil || genTypeDesc != xt.TypeDescriptor().Descriptor() {
		return xt, nil // not from generated code
	}
	ext, err := updateField(xt.TypeDescriptor().Descriptor())
	if err != nil {
		return nil, err
	}
	return extensionType{ExtensionType: xt, extDesc: ext}, nil
}

// AddSourceInfoToMessageType will return a new message type that
// is a copy of mt except that its associated descriptor includes
// source code info. If the file that contains the given message
// already contains source info, was not registered from generated
// code, or was not processed with the protoc-gen-gosrcinfo plugin,
// then mt is returned as is, unchanged.
func AddSourceInfoToMessageType(mt protoreflect.MessageType) (protoreflect.MessageType, error) {
	if genTypeDesc, err := protoregistry.GlobalFiles.FindDescriptorByName(mt.Descriptor().FullName()); err != nil || genTypeDesc != mt.Descriptor() {
		return mt, nil // not from generated code
	}
	msg, err := updateDescriptor(mt.Descriptor())
	if err != nil {
		return nil, err
	}
	return messageType{MessageType: mt, msgDesc: msg}, nil
}

// AddSourceInfoToEnumType will return a new enum type that is a
// copy of et except that its associated descriptor includes
// source code info. If the file that contains the given enum
// already contains source info, was not registered from generated
// code, or was not processed with the protoc-gen-gosrcinfo plugin,
// then et is returned as is, unchanged.
func AddSourceInfoToEnumType(et protoreflect.EnumType) (protoreflect.EnumType, error) {
	if genTypeDesc, err := protoregistry.GlobalFiles.FindDescriptorByName(et.Descriptor().FullName()); err != nil || genTypeDesc != et.Descriptor() {
		return et, nil // not from generated code
	}
	en, err := updateDescriptor(et.Descriptor())
	if err != nil {
		return nil, err
	}
	return enumType{EnumType: et, enumDesc: en}, nil
}

type extensionType struct {
	protoreflect.ExtensionType
	extDesc protoreflect.ExtensionDescriptor
}

func (xt extensionType) TypeDescriptor() protoreflect.ExtensionTypeDescriptor {
	return extensionTypeDescriptor{ExtensionDescriptor: xt.extDesc, extType: xt.ExtensionType}
}

type extensionTypeDescriptor struct {
	protoreflect.ExtensionDescriptor
	extType protoreflect.ExtensionType
}

func (xtd extensionTypeDescriptor) Type() protoreflect.ExtensionType {
	return extensionType{ExtensionType: xtd.extType, extDesc: xtd.ExtensionDescriptor}
}

func (xtd extensionTypeDescriptor) Descriptor() protoreflect.ExtensionDescriptor {
	return xtd.ExtensionDescriptor
}

type messageType struct {
	protoreflect.MessageType
	msgDesc protoreflect.MessageDescriptor
}

func (mt messageType) Descriptor() protoreflect.MessageDescriptor {
	return mt.msgDesc
}

type enumType struct {
	protoreflect.EnumType
	enumDesc protoreflect.EnumDescriptor
}

func (et enumType) Descriptor() protoreflect.EnumDescriptor {
	return et.enumDesc
}

func updateField(fd protoreflect.FieldDescriptor) (protoreflect.FieldDescriptor, error) {
	if xtd, ok := fd.(protoreflect.ExtensionTypeDescriptor); ok {
		ext, err := updateDescriptor(xtd.Descriptor())
		if err != nil {
			return nil, err
		}
		return extensionTypeDescriptor{ExtensionDescriptor: ext, extType: xtd.Type()}, nil
	}
	return updateDescriptor(fd)
}

func updateDescriptor[D protoreflect.Descriptor](d D) (D, error) {
	updatedFile, err := getFile(d.ParentFile())
	if err != nil {
		var zero D
		return zero, err
	}
	if updatedFile == d.ParentFile() {
		// no change
		return d, nil
	}
	updated := findDescriptor(updatedFile, d)
	result, ok := updated.(D)
	if !ok {
		var zero D
		return zero, fmt.Errorf("updated result is type %T which could not be converted to %T", updated, result)
	}
	return result, nil
}

func findDescriptor(fd protoreflect.FileDescriptor, d protoreflect.Descriptor) protoreflect.Descriptor {
	if d == nil {
		return nil
	}
	if _, isFile := d.(protoreflect.FileDescriptor); isFile {
		return fd
	}
	if d.Parent() == nil {
		return d
	}
	switch d := d.(type) {
	case protoreflect.MessageDescriptor:
		parent := findDescriptor(fd, d.Parent()).(messageContainer)
		return parent.Messages().Get(d.Index())
	case protoreflect.FieldDescriptor:
		if d.IsExtension() {
			parent := findDescriptor(fd, d.Parent()).(extensionContainer)
			return parent.Extensions().Get(d.Index())
		}
		parent := findDescriptor(fd, d.Parent()).(fieldContainer)
		return parent.Fields().Get(d.Index())
	case protoreflect.OneofDescriptor:
		parent := findDescriptor(fd, d.Parent()).(oneofContainer)
		return parent.Oneofs().Get(d.Index())
	case protoreflect.EnumDescriptor:
		parent := findDescriptor(fd, d.Parent()).(enumContainer)
		return parent.Enums().Get(d.Index())
	case protoreflect.EnumValueDescriptor:
		parent := findDescriptor(fd, d.Parent()).(enumValueContainer)
		return parent.Values().Get(d.Index())
	case protoreflect.ServiceDescriptor:
		parent := findDescriptor(fd, d.Parent()).(serviceContainer)
		return parent.Services().Get(d.Index())
	case protoreflect.MethodDescriptor:
		parent := findDescriptor(fd, d.Parent()).(methodContainer)
		return parent.Methods().Get(d.Index())
	}
	return d
}

type messageContainer interface {
	Messages() protoreflect.MessageDescriptors
}

type extensionContainer interface {
	Extensions() protoreflect.ExtensionDescriptors
}

type fieldContainer interface {
	Fields() protoreflect.FieldDescriptors
}

type oneofContainer interface {
	Oneofs() protoreflect.OneofDescriptors
}

type enumContainer interface {
	Enums() protoreflect.EnumDescriptors
}

type enumValueContainer interface {
	Values() protoreflect.EnumValueDescriptors
}

type serviceContainer interface {
	Services() protoreflect.ServiceDescriptors
}

type methodContainer interface {
	Methods() protoreflect.MethodDescriptors
}
