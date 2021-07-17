package grpc

import (
	"context"
	"fmt"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func doReflect(
	client reflectpb.ServerReflection_ServerReflectionInfoClient,
	req *reflectpb.ServerReflectionRequest) (*reflectpb.ServerReflectionResponse, error) {
	if err := client.Send(req); err != nil {
		return nil, err
	}
	return client.Recv()
}

// reflect will use the grpc reflection api to make the file descriptors available to request.
// It is called in the connect function the first time the Client.Connect function is called.
func (c *Client) reflect(ctxPtr *context.Context, addr string, params map[string]interface{}) ([]MethodInfo, error) {
	ok, err := c.Connect(ctxPtr, addr, params)
	if err != nil || !ok {
		return nil, fmt.Errorf("error connecting with grpc server: %s", addr)
	}
	defer c.conn.Close() //nolint: errcheck
	client := reflectpb.NewServerReflectionClient(c.conn)
	methodClient, err := client.ServerReflectionInfo(*ctxPtr)
	if err != nil {
		return nil, fmt.Errorf("error using reflection API: %s", addr)
	}
	req := &reflectpb.ServerReflectionRequest{MessageRequest: &reflectpb.ServerReflectionRequest_ListServices{}}
	resp, err := doReflect(methodClient, req)
	if err != nil {
		return nil, fmt.Errorf("error using reflection API: %s", addr)
	}
	listResp := resp.GetListServicesResponse()
	if listResp == nil {
		return nil, fmt.Errorf("can't list services")
	}
	fdset := &descriptorpb.FileDescriptorSet{}
	for _, service := range listResp.GetService() {
		req = &reflectpb.ServerReflectionRequest{
			MessageRequest: &reflectpb.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: service.GetName(),
			},
		}
		resp, err = doReflect(methodClient, req)
		if err != nil {
			return nil, fmt.Errorf("error listing methods on service: %s", service)
		}
		fdResp := resp.GetFileDescriptorResponse()
		for _, f := range fdResp.GetFileDescriptorProto() {
			a := &descriptorpb.FileDescriptorProto{}
			if err = proto.Unmarshal(f, a); err != nil {
				return nil, err
			}
			fdset.File = append(fdset.File, a)
		}
	}
	return c.convertToMethodInfo(fdset)
}
