package grpcext

import (
	"context"
	"fmt"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ReflectionClient wraps a grpc.ServerReflectionClient.
type reflectionClient struct {
	Conn grpc.ClientConnInterface
}

// Reflect will use the grpc reflection api to make the file descriptors available to request.
// It is called in the connect function the first time the Client.Connect function is called.
func (rc *reflectionClient) Reflect(ctx context.Context) (*descriptorpb.FileDescriptorSet, error) {
	client := grpcreflect.NewClientAuto(ctx, rc.Conn)

	services, err := client.ListServices()
	if err != nil {
		return nil, fmt.Errorf("can't list services: %w", err)
	}

	seen := make(map[fileDescriptorLookupKey]bool, len(services))
	fdset := &descriptorpb.FileDescriptorSet{
		File: make([]*descriptorpb.FileDescriptorProto, 0, len(services)),
	}

	for _, srv := range services {
		srvDescriptor, err := client.ResolveService(srv)
		if err != nil {
			return nil, fmt.Errorf("can't get method on service %q: %w", srv, err)
		}

		stack := []*desc.FileDescriptor{srvDescriptor.GetFile()}

		for len(stack) > 0 {
			fdp := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			fdkey := fileDescriptorLookupKey{
				Package: fdp.GetPackage(),
				Name:    fdp.GetName(),
			}

			stack = append(stack, fdp.GetDependencies()...)

			if seen[fdkey] {
				// When a proto file contains declarations for multiple services
				// then the same proto file is returned multiple times,
				// this prevents adding the returned proto file as a duplicate.
				continue
			}
			seen[fdkey] = true
			fdset.File = append(fdset.File, fdp.AsFileDescriptorProto())
		}
	}

	return fdset, nil
}

type fileDescriptorLookupKey struct {
	Package string
	Name    string
}
