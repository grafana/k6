package protoresolve

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ProtoFileOracle can recover a [*descriptorpb.FileDescriptorProto] for a
// given [protoreflect.FileDescriptor]. Implementations may be more efficient
// than the [protodesc] package, which re-creates the file descriptor proto
// each time.
//
// Callers should treat all returned messages as read-only. Mutating any
// of them could corrupt internal state and invalidate the results of
// future calls to this method.
type ProtoFileOracle interface {
	ProtoFromFileDescriptor(file protoreflect.FileDescriptor) (*descriptorpb.FileDescriptorProto, error)
}

// ProtoFileRegistry is a registry of descriptors that can work directly with
// descriptor protos. Registering a file requires registering all of its imports
// first. The registry will then use the registered dependencies to convert the
// given proto into a [protoreflect.FileDescriptor] and register the result.
//
// Unlike a normal registry that registers already-built [protoreflect.FileDescriptor]
// values, this kind can make the original descriptor proto available to callers
// (without having to reconstruct it, such as via the [protodesc] package).
//
// Callers should not mutate the proto after it has been registered, or attempt
// to re-use the message in a way that could cause it to be mutated. After this
// registration, the ProtoFileRegistry "owns" the proto.
type ProtoFileRegistry interface {
	ProtoFileOracle
	RegisterFileProto(fd *descriptorpb.FileDescriptorProto) (protoreflect.FileDescriptor, error)
}

var _ protodesc.Resolver = nil // just so we have an import of protodesc, for doc comments

// ProtoOracle has methods for recovering descriptor proto messages
// that correspond to given [protoreflect.Descriptor] instances.
//
// Callers should treat all returned messages as read-only. Mutating any
// of them could corrupt internal state and invalidate the results of
// future calls to these methods.
type ProtoOracle interface {
	ProtoFileOracle
	ProtoFromDescriptor(protoreflect.Descriptor) (proto.Message, error)
	ProtoFromMessageDescriptor(protoreflect.MessageDescriptor) (*descriptorpb.DescriptorProto, error)
	ProtoFromFieldDescriptor(protoreflect.FieldDescriptor) (*descriptorpb.FieldDescriptorProto, error)
	ProtoFromOneofDescriptor(protoreflect.OneofDescriptor) (*descriptorpb.OneofDescriptorProto, error)
	ProtoFromEnumDescriptor(protoreflect.EnumDescriptor) (*descriptorpb.EnumDescriptorProto, error)
	ProtoFromEnumValueDescriptor(protoreflect.EnumValueDescriptor) (*descriptorpb.EnumValueDescriptorProto, error)
	ProtoFromServiceDescriptor(protoreflect.ServiceDescriptor) (*descriptorpb.ServiceDescriptorProto, error)
	ProtoFromMethodDescriptor(protoreflect.MethodDescriptor) (*descriptorpb.MethodDescriptorProto, error)
}

// NewProtoOracle returns a value that can recover all kinds of
// descriptor proto messages given a function for recovering the file
// descriptor proto from a [protoreflect.FileDescriptor].
func NewProtoOracle(fileOracle ProtoFileOracle) ProtoOracle {
	return descProtosFromFile(fileOracle.ProtoFromFileDescriptor)
}

type descProtosFromFile func(protoreflect.FileDescriptor) (*descriptorpb.FileDescriptorProto, error)

func (d descProtosFromFile) ProtoFromFileDescriptor(descriptor protoreflect.FileDescriptor) (*descriptorpb.FileDescriptorProto, error) {
	return d(descriptor)
}

func (d descProtosFromFile) ProtoFromDescriptor(descriptor protoreflect.Descriptor) (proto.Message, error) {
	switch descriptor := descriptor.(type) {
	case protoreflect.FileDescriptor:
		return d.ProtoFromFileDescriptor(descriptor)
	case protoreflect.MessageDescriptor:
		return d.ProtoFromMessageDescriptor(descriptor)
	case protoreflect.FieldDescriptor:
		return d.ProtoFromFieldDescriptor(descriptor)
	case protoreflect.OneofDescriptor:
		return d.ProtoFromOneofDescriptor(descriptor)
	case protoreflect.EnumDescriptor:
		return d.ProtoFromEnumDescriptor(descriptor)
	case protoreflect.EnumValueDescriptor:
		return d.ProtoFromEnumValueDescriptor(descriptor)
	case protoreflect.ServiceDescriptor:
		return d.ProtoFromServiceDescriptor(descriptor)
	case protoreflect.MethodDescriptor:
		return d.ProtoFromMethodDescriptor(descriptor)
	default:
		return nil, fmt.Errorf("unsupported descriptor type: %T", descriptor)
	}
}

func (d descProtosFromFile) ProtoFromMessageDescriptor(descriptor protoreflect.MessageDescriptor) (*descriptorpb.DescriptorProto, error) {
	return protoFromDescriptor[*descriptorpb.DescriptorProto](descriptor, d)
}

func (d descProtosFromFile) ProtoFromFieldDescriptor(descriptor protoreflect.FieldDescriptor) (*descriptorpb.FieldDescriptorProto, error) {
	return protoFromDescriptor[*descriptorpb.FieldDescriptorProto](descriptor, d)
}

func (d descProtosFromFile) ProtoFromOneofDescriptor(descriptor protoreflect.OneofDescriptor) (*descriptorpb.OneofDescriptorProto, error) {
	return protoFromDescriptor[*descriptorpb.OneofDescriptorProto](descriptor, d)
}

func (d descProtosFromFile) ProtoFromEnumDescriptor(descriptor protoreflect.EnumDescriptor) (*descriptorpb.EnumDescriptorProto, error) {
	return protoFromDescriptor[*descriptorpb.EnumDescriptorProto](descriptor, d)
}

func (d descProtosFromFile) ProtoFromEnumValueDescriptor(descriptor protoreflect.EnumValueDescriptor) (*descriptorpb.EnumValueDescriptorProto, error) {
	return protoFromDescriptor[*descriptorpb.EnumValueDescriptorProto](descriptor, d)
}

func (d descProtosFromFile) ProtoFromServiceDescriptor(descriptor protoreflect.ServiceDescriptor) (*descriptorpb.ServiceDescriptorProto, error) {
	return protoFromDescriptor[*descriptorpb.ServiceDescriptorProto](descriptor, d)
}

func (d descProtosFromFile) ProtoFromMethodDescriptor(descriptor protoreflect.MethodDescriptor) (*descriptorpb.MethodDescriptorProto, error) {
	return protoFromDescriptor[*descriptorpb.MethodDescriptorProto](descriptor, d)
}

func protoFromDescriptor[M proto.Message](
	descriptor protoreflect.Descriptor,
	resolveFile func(protoreflect.FileDescriptor) (*descriptorpb.FileDescriptorProto, error),
) (M, error) {
	file, err := resolveFile(descriptor.ParentFile())
	if err != nil {
		var zero M
		return zero, err
	}
	msg, err := findProto(descriptor, file)
	if err != nil {
		var zero M
		return zero, err
	}
	return msg.(M), nil
}

func findProto(search protoreflect.Descriptor, file *descriptorpb.FileDescriptorProto) (proto.Message, error) {
	if _, isFile := search.(protoreflect.FileDescriptor); isFile {
		return file, nil
	}
	if search.Parent() == nil {
		return nil, fmt.Errorf("descriptor %q is not a file descriptor but also has no parent", search.FullName())
	}
	switch search := search.(type) {
	case protoreflect.MessageDescriptor:
		parent, err := findProto(search.Parent(), file)
		if err != nil {
			return nil, err
		}
		var candidates []*descriptorpb.DescriptorProto
		switch parent := parent.(type) {
		case *descriptorpb.FileDescriptorProto:
			candidates = parent.MessageType
		case *descriptorpb.DescriptorProto:
			candidates = parent.NestedType
		}
		return getCandidate(candidates, search)
	case protoreflect.FieldDescriptor:
		parent, err := findProto(search.Parent(), file)
		if err != nil {
			return nil, err
		}
		var candidates []*descriptorpb.FieldDescriptorProto
		switch parent := parent.(type) {
		case *descriptorpb.FileDescriptorProto:
			if search.IsExtension() {
				candidates = parent.Extension
			}
		case *descriptorpb.DescriptorProto:
			if search.IsExtension() {
				candidates = parent.Extension
			} else {
				candidates = parent.Field
			}
		}
		return getCandidate(candidates, search)
	case protoreflect.OneofDescriptor:
		parent, err := findProto(search.Parent(), file)
		if err != nil {
			return nil, err
		}
		var candidates []*descriptorpb.OneofDescriptorProto
		if parent, ok := parent.(*descriptorpb.DescriptorProto); ok {
			candidates = parent.OneofDecl
		}
		return getCandidate(candidates, search)
	case protoreflect.EnumDescriptor:
		parent, err := findProto(search.Parent(), file)
		if err != nil {
			return nil, err
		}
		var candidates []*descriptorpb.EnumDescriptorProto
		switch parent := parent.(type) {
		case *descriptorpb.FileDescriptorProto:
			candidates = parent.EnumType
		case *descriptorpb.DescriptorProto:
			candidates = parent.EnumType
		}
		return getCandidate(candidates, search)
	case protoreflect.EnumValueDescriptor:
		parent, err := findProto(search.Parent(), file)
		if err != nil {
			return nil, err
		}
		var candidates []*descriptorpb.EnumValueDescriptorProto
		if parent, ok := parent.(*descriptorpb.EnumDescriptorProto); ok {
			candidates = parent.Value
		}
		return getCandidate(candidates, search)
	case protoreflect.ServiceDescriptor:
		parent, err := findProto(search.Parent(), file)
		if err != nil {
			return nil, err
		}
		var candidates []*descriptorpb.ServiceDescriptorProto
		if parent, ok := parent.(*descriptorpb.FileDescriptorProto); ok {
			candidates = parent.Service
		}
		return getCandidate(candidates, search)
	case protoreflect.MethodDescriptor:
		parent, err := findProto(search.Parent(), file)
		if err != nil {
			return nil, err
		}
		var candidates []*descriptorpb.MethodDescriptorProto
		if parent, ok := parent.(*descriptorpb.ServiceDescriptorProto); ok {
			candidates = parent.Method
		}
		return getCandidate(candidates, search)
	default:
		return nil, fmt.Errorf("unexpected descriptor type: %T", search)
	}
}

func getCandidate[M interface {
	proto.Message
	GetName() string
}](candidates []M, d protoreflect.Descriptor) (M, error) {
	var zero M
	if len(candidates) == 0 {
		return zero, fmt.Errorf("could not find descriptor proto for %q: parent descriptor proto has no candidate children", d.FullName())
	}
	if d.Index() >= len(candidates) {
		return zero, fmt.Errorf("could not find descriptor proto for %q: parent descriptor proto has %d candidate children but descriptor index is %d", d.FullName(), len(candidates), d.Index())
	}
	result := candidates[d.Index()]
	if result.GetName() != string(d.Name()) {
		return zero, fmt.Errorf("could not find descriptor proto for %q: found descriptor with name %q instead", d.FullName(), result.GetName())
	}
	return result, nil
}
