// Package sourceinfo provides the ability to register and query source code info
// for file descriptors that are compiled into the binary. This data is registered
// by code generated from the protoc-gen-gosrcinfo plugin.
//
// The standard descriptors bundled into the compiled binary are stripped of source
// code info, to reduce binary size and reduce runtime memory footprint. However,
// the source code info can be very handy and worth the size cost when used with
// gRPC services and the server reflection service. Without source code info, the
// descriptors that a client downloads from the reflection service have no comments.
// But the presence of comments, and the ability to show them to humans, can greatly
// improve the utility of user agents that use the reflection service.
//
// When the protoc-gen-gosrcinfo plugin is used, the desc.Load* methods, which load
// descriptors for compiled-in elements, will automatically include source code
// info, using the data registered with this package.
//
// In order to make the reflection service use this functionality, you will need to
// be using v1.45 or higher of the Go runtime for gRPC (google.golang.org/grpc). The
// following snippet demonstrates how to do this in your server. Do this instead of
// using the reflection.Register function:
//
//	refSvr := reflection.NewServer(reflection.ServerOptions{
//	    Services:           grpcServer,
//	    DescriptorResolver: sourceinfo.GlobalFiles,
//	    ExtensionResolver:  sourceinfo.GlobalFiles,
//	})
//	grpc_reflection_v1alpha.RegisterServerReflectionServer(grpcServer, refSvr)
package sourceinfo

import (
	"bytes"
	"compress/gzip"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/jhump/protoreflect/v2/sourceinfo"
)

var (
	// GlobalFiles is a registry of descriptors that include source code info, if the
	// files they belong to were processed with protoc-gen-gosrcinfo.
	//
	// If is mean to serve as a drop-in alternative to protoregistry.GlobalFiles that
	// can include source code info in the returned descriptors.
	GlobalFiles Resolver = registry{}

	// GlobalTypes is a registry of descriptors that include source code info, if the
	// files they belong to were processed with protoc-gen-gosrcinfo.
	//
	// If is mean to serve as a drop-in alternative to protoregistry.GlobalTypes that
	// can include source code info in the returned descriptors.
	GlobalTypes TypeResolver = registry{}
)

// Resolver can resolve file names into file descriptors and also provides methods for
// resolving extensions.
type Resolver interface {
	protodesc.Resolver
	protoregistry.ExtensionTypeResolver
	RangeExtensionsByMessage(message protoreflect.FullName, f func(protoreflect.ExtensionType) bool)
}

// NB: These interfaces are far from ideal. Ideally, Resolver would have
//    * EITHER been named FileResolver and not included the extension methods.
//    * OR also included message methods (i.e. embed protoregistry.MessageTypeResolver).
//   Now (since it's been released) we can't add the message methods to the interface as
//   that's not a backwards-compatible change. So we have to introduce the new interface
//   below, which is now a little confusing since it has some overlap with Resolver.

// TypeResolver can resolve message names and URLs into message descriptors and also
// provides methods for resolving extensions.
type TypeResolver interface {
	protoregistry.MessageTypeResolver
	protoregistry.ExtensionTypeResolver
	RangeExtensionsByMessage(message protoreflect.FullName, f func(protoreflect.ExtensionType) bool)
}

// RegisterSourceInfo registers the given source code info for the file descriptor
// with the given path/name.
//
// This is automatically used from older generated code if using a previous release of
// the protoc-gen-gosrcinfo plugin.
func RegisterSourceInfo(file string, srcInfo *descriptorpb.SourceCodeInfo) {
	siBytes, _ := proto.Marshal(srcInfo)
	var encodedBuf bytes.Buffer
	zipWriter := gzip.NewWriter(&encodedBuf)
	_, _ = zipWriter.Write(siBytes)
	_ = zipWriter.Close()
	encodedBytes := encodedBuf.Bytes()

	sourceinfo.Register(file, encodedBytes)
}

// RegisterEncodedSourceInfo registers the given source code info, which is a serialized
// and gzipped form of a google.protobuf.SourceCodeInfo message.
//
// This is automatically used from generated code if using the protoc-gen-gosrcinfo
// plugin.
func RegisterEncodedSourceInfo(file string, data []byte) error {
	sourceinfo.Register(file, data)
	return nil
}

// SourceInfoForFile queries for any registered source code info for the file
// descriptor with the given path/name. It returns nil if no source code info
// was registered.
func SourceInfoForFile(file string) *descriptorpb.SourceCodeInfo {
	ret, _ := sourceinfo.ForFile(file)
	return ret
}

type registry struct{}

var _ protodesc.Resolver = &registry{}

func (r registry) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	return sourceinfo.Files.FindFileByPath(path)
}

func (r registry) FindDescriptorByName(name protoreflect.FullName) (protoreflect.Descriptor, error) {
	return sourceinfo.Files.FindDescriptorByName(name)
}

func (r registry) FindMessageByName(message protoreflect.FullName) (protoreflect.MessageType, error) {
	return sourceinfo.Types.FindMessageByName(message)
}

func (r registry) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	return sourceinfo.Types.FindMessageByURL(url)
}

func (r registry) FindExtensionByName(field protoreflect.FullName) (protoreflect.ExtensionType, error) {
	return sourceinfo.Types.FindExtensionByName(field)
}

func (r registry) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	return sourceinfo.Types.FindExtensionByNumber(message, field)
}

func (r registry) RangeExtensionsByMessage(message protoreflect.FullName, fn func(protoreflect.ExtensionType) bool) {
	sourceinfo.Types.RangeExtensionsByMessage(message, fn)
}
