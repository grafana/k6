// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        v3.21.12
// source: v1/k6/request_metadata.proto

package k6

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type RequestMetadata struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TraceID           string         `protobuf:"bytes,1,opt,name=TraceID,proto3" json:"TraceID,omitempty"`
	StartTimeUnixNano int64          `protobuf:"varint,2,opt,name=StartTimeUnixNano,proto3" json:"StartTimeUnixNano,omitempty"`
	EndTimeUnixNano   int64          `protobuf:"varint,3,opt,name=EndTimeUnixNano,proto3" json:"EndTimeUnixNano,omitempty"`
	TestRunLabels     *TestRunLabels `protobuf:"bytes,4,opt,name=TestRunLabels,proto3" json:"TestRunLabels,omitempty"`
	// Types that are assignable to ProtocolLabels:
	//
	//	*RequestMetadata_HTTPLabels
	ProtocolLabels isRequestMetadata_ProtocolLabels `protobuf_oneof:"ProtocolLabels"`
}

func (x *RequestMetadata) Reset() {
	*x = RequestMetadata{}
	if protoimpl.UnsafeEnabled {
		mi := &file_v1_k6_request_metadata_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RequestMetadata) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RequestMetadata) ProtoMessage() {}

func (x *RequestMetadata) ProtoReflect() protoreflect.Message {
	mi := &file_v1_k6_request_metadata_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RequestMetadata.ProtoReflect.Descriptor instead.
func (*RequestMetadata) Descriptor() ([]byte, []int) {
	return file_v1_k6_request_metadata_proto_rawDescGZIP(), []int{0}
}

func (x *RequestMetadata) GetTraceID() string {
	if x != nil {
		return x.TraceID
	}
	return ""
}

func (x *RequestMetadata) GetStartTimeUnixNano() int64 {
	if x != nil {
		return x.StartTimeUnixNano
	}
	return 0
}

func (x *RequestMetadata) GetEndTimeUnixNano() int64 {
	if x != nil {
		return x.EndTimeUnixNano
	}
	return 0
}

func (x *RequestMetadata) GetTestRunLabels() *TestRunLabels {
	if x != nil {
		return x.TestRunLabels
	}
	return nil
}

func (m *RequestMetadata) GetProtocolLabels() isRequestMetadata_ProtocolLabels {
	if m != nil {
		return m.ProtocolLabels
	}
	return nil
}

func (x *RequestMetadata) GetHTTPLabels() *HTTPLabels {
	if x, ok := x.GetProtocolLabels().(*RequestMetadata_HTTPLabels); ok {
		return x.HTTPLabels
	}
	return nil
}

type isRequestMetadata_ProtocolLabels interface {
	isRequestMetadata_ProtocolLabels()
}

type RequestMetadata_HTTPLabels struct {
	HTTPLabels *HTTPLabels `protobuf:"bytes,5,opt,name=HTTPLabels,proto3,oneof"`
}

func (*RequestMetadata_HTTPLabels) isRequestMetadata_ProtocolLabels() {}

var File_v1_k6_request_metadata_proto protoreflect.FileDescriptor

var file_v1_k6_request_metadata_proto_rawDesc = []byte{
	0x0a, 0x1c, 0x76, 0x31, 0x2f, 0x6b, 0x36, 0x2f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x5f,
	0x6d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x1d,
	0x6b, 0x36, 0x2e, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x2e, 0x69, 0x6e, 0x73, 0x69, 0x67, 0x68, 0x74,
	0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x76, 0x31, 0x2e, 0x6b, 0x36, 0x1a, 0x12, 0x76,
	0x31, 0x2f, 0x6b, 0x36, 0x2f, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0xb6, 0x02, 0x0a, 0x0f, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x4d, 0x65, 0x74,
	0x61, 0x64, 0x61, 0x74, 0x61, 0x12, 0x18, 0x0a, 0x07, 0x54, 0x72, 0x61, 0x63, 0x65, 0x49, 0x44,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x54, 0x72, 0x61, 0x63, 0x65, 0x49, 0x44, 0x12,
	0x2c, 0x0a, 0x11, 0x53, 0x74, 0x61, 0x72, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x55, 0x6e, 0x69, 0x78,
	0x4e, 0x61, 0x6e, 0x6f, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x11, 0x53, 0x74, 0x61, 0x72,
	0x74, 0x54, 0x69, 0x6d, 0x65, 0x55, 0x6e, 0x69, 0x78, 0x4e, 0x61, 0x6e, 0x6f, 0x12, 0x28, 0x0a,
	0x0f, 0x45, 0x6e, 0x64, 0x54, 0x69, 0x6d, 0x65, 0x55, 0x6e, 0x69, 0x78, 0x4e, 0x61, 0x6e, 0x6f,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0f, 0x45, 0x6e, 0x64, 0x54, 0x69, 0x6d, 0x65, 0x55,
	0x6e, 0x69, 0x78, 0x4e, 0x61, 0x6e, 0x6f, 0x12, 0x52, 0x0a, 0x0d, 0x54, 0x65, 0x73, 0x74, 0x52,
	0x75, 0x6e, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2c,
	0x2e, 0x6b, 0x36, 0x2e, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x2e, 0x69, 0x6e, 0x73, 0x69, 0x67, 0x68,
	0x74, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x76, 0x31, 0x2e, 0x6b, 0x36, 0x2e, 0x54,
	0x65, 0x73, 0x74, 0x52, 0x75, 0x6e, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x52, 0x0d, 0x54, 0x65,
	0x73, 0x74, 0x52, 0x75, 0x6e, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x12, 0x4b, 0x0a, 0x0a, 0x48,
	0x54, 0x54, 0x50, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x29, 0x2e, 0x6b, 0x36, 0x2e, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x2e, 0x69, 0x6e, 0x73, 0x69, 0x67,
	0x68, 0x74, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x76, 0x31, 0x2e, 0x6b, 0x36, 0x2e,
	0x48, 0x54, 0x54, 0x50, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x48, 0x00, 0x52, 0x0a, 0x48, 0x54,
	0x54, 0x50, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x42, 0x10, 0x0a, 0x0e, 0x50, 0x72, 0x6f, 0x74,
	0x6f, 0x63, 0x6f, 0x6c, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x42, 0x34, 0x5a, 0x32, 0x67, 0x6f,
	0x2e, 0x6b, 0x36, 0x2e, 0x69, 0x6f, 0x2f, 0x6b, 0x36, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e,
	0x61, 0x6c, 0x2f, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x61, 0x70, 0x69, 0x2f, 0x69, 0x6e, 0x73, 0x69,
	0x67, 0x68, 0x74, 0x73, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x76, 0x31, 0x2f, 0x6b, 0x36,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_v1_k6_request_metadata_proto_rawDescOnce sync.Once
	file_v1_k6_request_metadata_proto_rawDescData = file_v1_k6_request_metadata_proto_rawDesc
)

func file_v1_k6_request_metadata_proto_rawDescGZIP() []byte {
	file_v1_k6_request_metadata_proto_rawDescOnce.Do(func() {
		file_v1_k6_request_metadata_proto_rawDescData = protoimpl.X.CompressGZIP(file_v1_k6_request_metadata_proto_rawDescData)
	})
	return file_v1_k6_request_metadata_proto_rawDescData
}

var file_v1_k6_request_metadata_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_v1_k6_request_metadata_proto_goTypes = []interface{}{
	(*RequestMetadata)(nil), // 0: k6.cloud.insights.proto.v1.k6.RequestMetadata
	(*TestRunLabels)(nil),   // 1: k6.cloud.insights.proto.v1.k6.TestRunLabels
	(*HTTPLabels)(nil),      // 2: k6.cloud.insights.proto.v1.k6.HTTPLabels
}
var file_v1_k6_request_metadata_proto_depIdxs = []int32{
	1, // 0: k6.cloud.insights.proto.v1.k6.RequestMetadata.TestRunLabels:type_name -> k6.cloud.insights.proto.v1.k6.TestRunLabels
	2, // 1: k6.cloud.insights.proto.v1.k6.RequestMetadata.HTTPLabels:type_name -> k6.cloud.insights.proto.v1.k6.HTTPLabels
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_v1_k6_request_metadata_proto_init() }
func file_v1_k6_request_metadata_proto_init() {
	if File_v1_k6_request_metadata_proto != nil {
		return
	}
	file_v1_k6_labels_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_v1_k6_request_metadata_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RequestMetadata); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_v1_k6_request_metadata_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*RequestMetadata_HTTPLabels)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_v1_k6_request_metadata_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_v1_k6_request_metadata_proto_goTypes,
		DependencyIndexes: file_v1_k6_request_metadata_proto_depIdxs,
		MessageInfos:      file_v1_k6_request_metadata_proto_msgTypes,
	}.Build()
	File_v1_k6_request_metadata_proto = out.File
	file_v1_k6_request_metadata_proto_rawDesc = nil
	file_v1_k6_request_metadata_proto_goTypes = nil
	file_v1_k6_request_metadata_proto_depIdxs = nil
}
