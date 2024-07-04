// Copyright 2020-2024 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import "google.golang.org/protobuf/types/descriptorpb"

var FieldTypes = map[string]descriptorpb.FieldDescriptorProto_Type{
	"double":   descriptorpb.FieldDescriptorProto_TYPE_DOUBLE,
	"float":    descriptorpb.FieldDescriptorProto_TYPE_FLOAT,
	"int32":    descriptorpb.FieldDescriptorProto_TYPE_INT32,
	"int64":    descriptorpb.FieldDescriptorProto_TYPE_INT64,
	"uint32":   descriptorpb.FieldDescriptorProto_TYPE_UINT32,
	"uint64":   descriptorpb.FieldDescriptorProto_TYPE_UINT64,
	"sint32":   descriptorpb.FieldDescriptorProto_TYPE_SINT32,
	"sint64":   descriptorpb.FieldDescriptorProto_TYPE_SINT64,
	"fixed32":  descriptorpb.FieldDescriptorProto_TYPE_FIXED32,
	"fixed64":  descriptorpb.FieldDescriptorProto_TYPE_FIXED64,
	"sfixed32": descriptorpb.FieldDescriptorProto_TYPE_SFIXED32,
	"sfixed64": descriptorpb.FieldDescriptorProto_TYPE_SFIXED64,
	"bool":     descriptorpb.FieldDescriptorProto_TYPE_BOOL,
	"string":   descriptorpb.FieldDescriptorProto_TYPE_STRING,
	"bytes":    descriptorpb.FieldDescriptorProto_TYPE_BYTES,
}
