package grpcreflect

import (
	refv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	refv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

func toV1Request(v1alpha *refv1alpha.ServerReflectionRequest) *refv1.ServerReflectionRequest {
	var v1 refv1.ServerReflectionRequest
	v1.Host = v1alpha.Host
	switch mr := v1alpha.MessageRequest.(type) {
	case *refv1alpha.ServerReflectionRequest_FileByFilename:
		v1.MessageRequest = &refv1.ServerReflectionRequest_FileByFilename{
			FileByFilename: mr.FileByFilename,
		}
	case *refv1alpha.ServerReflectionRequest_FileContainingSymbol:
		v1.MessageRequest = &refv1.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: mr.FileContainingSymbol,
		}
	case *refv1alpha.ServerReflectionRequest_FileContainingExtension:
		if mr.FileContainingExtension != nil {
			v1.MessageRequest = &refv1.ServerReflectionRequest_FileContainingExtension{
				FileContainingExtension: &refv1.ExtensionRequest{
					ContainingType:  mr.FileContainingExtension.GetContainingType(),
					ExtensionNumber: mr.FileContainingExtension.GetExtensionNumber(),
				},
			}
		}
	case *refv1alpha.ServerReflectionRequest_AllExtensionNumbersOfType:
		v1.MessageRequest = &refv1.ServerReflectionRequest_AllExtensionNumbersOfType{
			AllExtensionNumbersOfType: mr.AllExtensionNumbersOfType,
		}
	case *refv1alpha.ServerReflectionRequest_ListServices:
		v1.MessageRequest = &refv1.ServerReflectionRequest_ListServices{
			ListServices: mr.ListServices,
		}
	default:
		// no value set
	}
	return &v1
}

func toV1AlphaRequest(v1 *refv1.ServerReflectionRequest) *refv1alpha.ServerReflectionRequest {
	var v1alpha refv1alpha.ServerReflectionRequest
	v1alpha.Host = v1.Host
	switch mr := v1.MessageRequest.(type) {
	case *refv1.ServerReflectionRequest_FileByFilename:
		if mr != nil {
			v1alpha.MessageRequest = &refv1alpha.ServerReflectionRequest_FileByFilename{
				FileByFilename: mr.FileByFilename,
			}
		}
	case *refv1.ServerReflectionRequest_FileContainingSymbol:
		if mr != nil {
			v1alpha.MessageRequest = &refv1alpha.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: mr.FileContainingSymbol,
			}
		}
	case *refv1.ServerReflectionRequest_FileContainingExtension:
		if mr != nil {
			v1alpha.MessageRequest = &refv1alpha.ServerReflectionRequest_FileContainingExtension{
				FileContainingExtension: &refv1alpha.ExtensionRequest{
					ContainingType:  mr.FileContainingExtension.GetContainingType(),
					ExtensionNumber: mr.FileContainingExtension.GetExtensionNumber(),
				},
			}
		}
	case *refv1.ServerReflectionRequest_AllExtensionNumbersOfType:
		if mr != nil {
			v1alpha.MessageRequest = &refv1alpha.ServerReflectionRequest_AllExtensionNumbersOfType{
				AllExtensionNumbersOfType: mr.AllExtensionNumbersOfType,
			}
		}
	case *refv1.ServerReflectionRequest_ListServices:
		if mr != nil {
			v1alpha.MessageRequest = &refv1alpha.ServerReflectionRequest_ListServices{
				ListServices: mr.ListServices,
			}
		}
	default:
		// no value set
	}
	return &v1alpha
}

func toV1AlphaResponse(v1 *refv1.ServerReflectionResponse) *refv1alpha.ServerReflectionResponse {
	var v1alpha refv1alpha.ServerReflectionResponse
	v1alpha.ValidHost = v1.ValidHost
	if v1.OriginalRequest != nil {
		v1alpha.OriginalRequest = toV1AlphaRequest(v1.OriginalRequest)
	}
	switch mr := v1.MessageResponse.(type) {
	case *refv1.ServerReflectionResponse_FileDescriptorResponse:
		if mr != nil {
			v1alpha.MessageResponse = &refv1alpha.ServerReflectionResponse_FileDescriptorResponse{
				FileDescriptorResponse: &refv1alpha.FileDescriptorResponse{
					FileDescriptorProto: mr.FileDescriptorResponse.GetFileDescriptorProto(),
				},
			}
		}
	case *refv1.ServerReflectionResponse_AllExtensionNumbersResponse:
		if mr != nil {
			v1alpha.MessageResponse = &refv1alpha.ServerReflectionResponse_AllExtensionNumbersResponse{
				AllExtensionNumbersResponse: &refv1alpha.ExtensionNumberResponse{
					BaseTypeName:    mr.AllExtensionNumbersResponse.GetBaseTypeName(),
					ExtensionNumber: mr.AllExtensionNumbersResponse.GetExtensionNumber(),
				},
			}
		}
	case *refv1.ServerReflectionResponse_ListServicesResponse:
		if mr != nil {
			svcs := make([]*refv1alpha.ServiceResponse, len(mr.ListServicesResponse.GetService()))
			for i, svc := range mr.ListServicesResponse.GetService() {
				svcs[i] = &refv1alpha.ServiceResponse{
					Name: svc.GetName(),
				}
			}
			v1alpha.MessageResponse = &refv1alpha.ServerReflectionResponse_ListServicesResponse{
				ListServicesResponse: &refv1alpha.ListServiceResponse{
					Service: svcs,
				},
			}
		}
	case *refv1.ServerReflectionResponse_ErrorResponse:
		if mr != nil {
			v1alpha.MessageResponse = &refv1alpha.ServerReflectionResponse_ErrorResponse{
				ErrorResponse: &refv1alpha.ErrorResponse{
					ErrorCode:    mr.ErrorResponse.GetErrorCode(),
					ErrorMessage: mr.ErrorResponse.GetErrorMessage(),
				},
			}
		}
	default:
		// no value set
	}
	return &v1alpha
}
