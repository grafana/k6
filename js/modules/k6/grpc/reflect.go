package grpc

import (
	"context"
	"fmt"

	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// reflect will use the grpc reflection api to make the file descriptors available to request.
// It is called in the connect function the first time the Client.Connect function is called.
func (c *Client) reflect(ctxPtr *context.Context) error {
	client := reflectpb.NewServerReflectionClient(c.conn)
	methodClient, err := client.ServerReflectionInfo(*ctxPtr)
	if err != nil {
		return err
	}
	req := &reflectpb.ServerReflectionRequest{MessageRequest: &reflectpb.ServerReflectionRequest_ListServices{}}
	if err = methodClient.Send(req); err != nil {
		return err
	}
	resp, err := methodClient.Recv()
	if err != nil {
		return err
	}
	listResp := resp.GetListServicesResponse()
	if listResp == nil {
		return fmt.Errorf("can't list services")
	}
	fdset := &descriptorpb.FileDescriptorSet{}
	for _, service := range listResp.GetService() {
		req = &reflectpb.ServerReflectionRequest{
			MessageRequest: &reflectpb.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: service.GetName(),
			},
		}
		if err = methodClient.Send(req); err != nil {
			return err
		}
		resp, err = methodClient.Recv()
		if err != nil {
			return fmt.Errorf("error listing methods on '%s': %w", service, err)
		}
		fdResp := resp.GetFileDescriptorResponse()
		for _, f := range fdResp.GetFileDescriptorProto() {
			a := &descriptorpb.FileDescriptorProto{}
			if err = proto.Unmarshal(f, a); err != nil {
				return err
			}
			fdset.File = append(fdset.File, a)
		}
	}
	_, err = c.convertToMethodInfo(fdset)
	return err
}
