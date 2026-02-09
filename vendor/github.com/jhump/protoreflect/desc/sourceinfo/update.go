package sourceinfo

import (
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/jhump/protoreflect/v2/sourceinfo"
)

// AddSourceInfoToFile will return a new file descriptor that is a copy
// of fd except that it includes source code info. If the given file
// already contains source info, was not registered from generated code,
// or was not processed with the protoc-gen-gosrcinfo plugin, then fd
// is returned as is, unchanged.
func AddSourceInfoToFile(fd protoreflect.FileDescriptor) (protoreflect.FileDescriptor, error) {
	return sourceinfo.AddSourceInfoToFile(fd)
}

// AddSourceInfoToMessage will return a new message descriptor that is a
// copy of md except that it includes source code info. If the file that
// contains the given message descriptor already contains source info,
// was not registered from generated code, or was not processed with the
// protoc-gen-gosrcinfo plugin, then md is returned as is, unchanged.
func AddSourceInfoToMessage(md protoreflect.MessageDescriptor) (protoreflect.MessageDescriptor, error) {
	return sourceinfo.AddSourceInfoToMessage(md)
}

// AddSourceInfoToEnum will return a new enum descriptor that is a copy
// of ed except that it includes source code info. If the file that
// contains the given enum descriptor already contains source info, was
// not registered from generated code, or was not processed with the
// protoc-gen-gosrcinfo plugin, then ed is returned as is, unchanged.
func AddSourceInfoToEnum(ed protoreflect.EnumDescriptor) (protoreflect.EnumDescriptor, error) {
	return sourceinfo.AddSourceInfoToEnum(ed)
}

// AddSourceInfoToService will return a new service descriptor that is
// a copy of sd except that it includes source code info. If the file
// that contains the given service descriptor already contains source
// info, was not registered from generated code, or was not processed
// with the protoc-gen-gosrcinfo plugin, then ed is returned as is,
// unchanged.
func AddSourceInfoToService(sd protoreflect.ServiceDescriptor) (protoreflect.ServiceDescriptor, error) {
	return sourceinfo.AddSourceInfoToService(sd)
}

// AddSourceInfoToExtensionType will return a new extension type that
// is a copy of xt except that its associated descriptors includes
// source code info. If the file that contains the given extension
// already contains source info, was not registered from generated
// code, or was not processed with the protoc-gen-gosrcinfo plugin,
// then xt is returned as is, unchanged.
func AddSourceInfoToExtensionType(xt protoreflect.ExtensionType) (protoreflect.ExtensionType, error) {
	return sourceinfo.AddSourceInfoToExtensionType(xt)
}

// AddSourceInfoToMessageType will return a new message type that
// is a copy of mt except that its associated descriptors includes
// source code info. If the file that contains the given message
// already contains source info, was not registered from generated
// code, or was not processed with the protoc-gen-gosrcinfo plugin,
// then mt is returned as is, unchanged.
func AddSourceInfoToMessageType(mt protoreflect.MessageType) (protoreflect.MessageType, error) {
	return sourceinfo.AddSourceInfoToMessageType(mt)
}

// WrapFile is present for backwards-compatibility reasons. It calls
// AddSourceInfoToFile and panics if that function returns an error.
//
// Deprecated: Use AddSourceInfoToFile directly instead. The word "wrap" is
// a misnomer since this method does not actually wrap the given value.
// Though unlikely, the operation can technically fail, so the recommended
// function allows the return of an error instead of panic'ing.
func WrapFile(fd protoreflect.FileDescriptor) protoreflect.FileDescriptor {
	result, err := AddSourceInfoToFile(fd)
	if err != nil {
		panic(err)
	}
	return result
}

// WrapMessage is present for backwards-compatibility reasons. It calls
// AddSourceInfoToMessage and panics if that function returns an error.
//
// Deprecated: Use AddSourceInfoToMessage directly instead. The word
// "wrap" is a misnomer since this method does not actually wrap the
// given value. Though unlikely, the operation can technically fail,
// so the recommended function allows the return of an error instead
// of panic'ing.
func WrapMessage(md protoreflect.MessageDescriptor) protoreflect.MessageDescriptor {
	result, err := AddSourceInfoToMessage(md)
	if err != nil {
		panic(err)
	}
	return result
}

// WrapEnum is present for backwards-compatibility reasons. It calls
// AddSourceInfoToEnum and panics if that function returns an error.
//
// Deprecated: Use AddSourceInfoToEnum directly instead. The word
// "wrap" is a misnomer since this method does not actually wrap the
// given value. Though unlikely, the operation can technically fail,
// so the recommended function allows the return of an error instead
// of panic'ing.
func WrapEnum(ed protoreflect.EnumDescriptor) protoreflect.EnumDescriptor {
	result, err := AddSourceInfoToEnum(ed)
	if err != nil {
		panic(err)
	}
	return result
}

// WrapService is present for backwards-compatibility reasons. It calls
// AddSourceInfoToService and panics if that function returns an error.
//
// Deprecated: Use AddSourceInfoToService directly instead. The word
// "wrap" is a misnomer since this method does not actually wrap the
// given value. Though unlikely, the operation can technically fail,
// so the recommended function allows the return of an error instead
// of panic'ing.
func WrapService(sd protoreflect.ServiceDescriptor) protoreflect.ServiceDescriptor {
	result, err := AddSourceInfoToService(sd)
	if err != nil {
		panic(err)
	}
	return result
}

// WrapExtensionType is present for backwards-compatibility reasons. It
// calls AddSourceInfoToExtensionType and panics if that function
// returns an error.
//
// Deprecated: Use AddSourceInfoToExtensionType directly instead. The
// word "wrap" is a misnomer since this method does not actually wrap
// the given value. Though unlikely, the operation can technically fail,
// so the recommended function allows the return of an error instead
// of panic'ing.
func WrapExtensionType(xt protoreflect.ExtensionType) protoreflect.ExtensionType {
	result, err := AddSourceInfoToExtensionType(xt)
	if err != nil {
		panic(err)
	}
	return result
}

// WrapMessageType is present for backwards-compatibility reasons. It
// calls AddSourceInfoToMessageType and panics if that function returns
// an error.
//
// Deprecated: Use AddSourceInfoToMessageType directly instead. The word
// "wrap" is a misnomer since this method does not actually wrap the
// given value. Though unlikely, the operation can technically fail, so
// the recommended function allows the return of an error instead of
// panic'ing.
func WrapMessageType(mt protoreflect.MessageType) protoreflect.MessageType {
	result, err := AddSourceInfoToMessageType(mt)
	if err != nil {
		panic(err)
	}
	return result
}
