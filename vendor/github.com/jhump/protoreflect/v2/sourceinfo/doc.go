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
// In order to make the reflection service use this functionality, you will need to
// be using v1.45 or higher of the Go runtime for gRPC (google.golang.org/grpc). The
// following snippet demonstrates how to do this in your server. Do this instead of
// using the reflection.Register function:
//
//	refSvr := reflection.NewServer(reflection.ServerOptions{
//	    Services:           grpcServer,
//	    DescriptorResolver: sourceinfo.Files,
//	    ExtensionResolver:  sourceinfo.Types,
//	})
//	grpc_reflection_v1alpha.RegisterServerReflectionServer(grpcServer, refSvr)
package sourceinfo
