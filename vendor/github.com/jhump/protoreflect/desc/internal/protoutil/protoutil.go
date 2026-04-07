package protoutil

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Note: copied from ./protoparse/internal/protocompile/protoutil

type DescriptorProtoWrapper interface {
	protoreflect.Descriptor
	AsProto() proto.Message
}

func ProtoFromFileDescriptor(d protoreflect.FileDescriptor) *descriptorpb.FileDescriptorProto {
	if imp, ok := d.(protoreflect.FileImport); ok {
		d = imp.FileDescriptor
	}
	type canProto interface {
		FileDescriptorProto() *descriptorpb.FileDescriptorProto
	}
	if res, ok := d.(canProto); ok {
		return res.FileDescriptorProto()
	}
	if res, ok := d.(DescriptorProtoWrapper); ok {
		if fd, ok := res.AsProto().(*descriptorpb.FileDescriptorProto); ok {
			return fd
		}
	}
	return protodesc.ToFileDescriptorProto(d)
}
