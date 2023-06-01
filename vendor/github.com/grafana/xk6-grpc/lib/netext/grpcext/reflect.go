package grpcext

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ReflectionClient wraps a grpc.ServerReflectionClient.
type reflectionClient struct {
	Conn grpc.ClientConnInterface
}

// Reflect will use the grpc reflection api to make the file descriptors available to request.
// It is called in the connect function the first time the Client.Connect function is called.
func (rc *reflectionClient) Reflect(ctx context.Context) (*descriptorpb.FileDescriptorSet, error) {
	client := reflectpb.NewServerReflectionClient(rc.Conn)
	methodClient, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't get server info: %w", err)
	}
	req := &reflectpb.ServerReflectionRequest{
		MessageRequest: &reflectpb.ServerReflectionRequest_ListServices{},
	}
	resp, err := sendReceive(methodClient, req)
	if err != nil {
		return nil, fmt.Errorf("can't list services: %w", err)
	}
	listResp := resp.GetListServicesResponse()
	if listResp == nil {
		return nil, fmt.Errorf("can't list services, nil response")
	}
	fdset, err := rc.resolveServiceFileDescriptors(methodClient, listResp)
	if err != nil {
		return nil, fmt.Errorf("can't resolve services' file descriptors: %w", err)
	}
	return fdset, nil
}

func (rc *reflectionClient) resolveServiceFileDescriptors(
	client sendReceiver,
	res *reflectpb.ListServiceResponse,
) (*descriptorpb.FileDescriptorSet, error) {
	services := res.GetService()
	seen := make(map[fileDescriptorLookupKey]bool, len(services))
	fdset := &descriptorpb.FileDescriptorSet{
		File: make([]*descriptorpb.FileDescriptorProto, 0, len(services)),
	}

	for _, service := range services {
		req := &reflectpb.ServerReflectionRequest{
			MessageRequest: &reflectpb.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: service.GetName(),
			},
		}
		resp, err := sendReceive(client, req)
		if err != nil {
			return nil, fmt.Errorf("can't get method on service %q: %w", service, err)
		}
		fdResp := resp.GetFileDescriptorResponse()
		for _, raw := range fdResp.GetFileDescriptorProto() {
			var fdp descriptorpb.FileDescriptorProto
			if err = proto.Unmarshal(raw, &fdp); err != nil {
				return nil, fmt.Errorf("can't unmarshal proto on service %q: %w", service, err)
			}
			fdkey := fileDescriptorLookupKey{
				Package: *fdp.Package,
				Name:    *fdp.Name,
			}
			if seen[fdkey] {
				// When a proto file contains declarations for multiple services
				// then the same proto file is returned multiple times,
				// this prevents adding the returned proto file as a duplicate.
				continue
			}
			seen[fdkey] = true
			fdset.File = append(fdset.File, &fdp)
		}
	}
	return fdset, nil
}

// sendReceiver is a smaller interface for decoupling
// from `reflectpb.ServerReflection_ServerReflectionInfoClient`,
// that has the dependency from `grpc.ClientStream`,
// which is too much in the case the requirement is to just make a reflection's request.
// It makes the API more restricted and with a controlled surface,
// in this way the testing should be easier also.
type sendReceiver interface {
	Send(*reflectpb.ServerReflectionRequest) error
	Recv() (*reflectpb.ServerReflectionResponse, error)
}

// sendReceive sends a request to a reflection client and,
// receives a response.
func sendReceive(
	client sendReceiver,
	req *reflectpb.ServerReflectionRequest,
) (*reflectpb.ServerReflectionResponse, error) {
	if err := client.Send(req); err != nil {
		return nil, fmt.Errorf("can't send request: %w", err)
	}
	resp, err := client.Recv()
	if err != nil {
		return nil, fmt.Errorf("can't receive response: %w", err)
	}
	return resp, nil
}

type fileDescriptorLookupKey struct {
	Package string
	Name    string
}
