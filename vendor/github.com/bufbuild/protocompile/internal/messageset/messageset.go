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

package messageset

import (
	"math"
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
)

var (
	messageSetSupport     bool
	messageSetSupportInit sync.Once
)

// CanSupportMessageSets returns true if the protobuf-go runtime supports
// serializing messages with the message set wire format.
func CanSupportMessageSets() bool {
	messageSetSupportInit.Do(func() {
		// We check using the protodesc package, instead of just relying
		// on protolegacy build tag, in case someone links in a fork of
		// the protobuf-go runtime that supports legacy proto1 features
		// or in case the protobuf-go runtime adds another mechanism to
		// enable or disable it (such as environment variable).
		_, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
			Name: proto.String("test.proto"),
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: proto.String("MessageSet"),
					Options: &descriptorpb.MessageOptions{
						MessageSetWireFormat: proto.Bool(true),
					},
					ExtensionRange: []*descriptorpb.DescriptorProto_ExtensionRange{
						{
							Start: proto.Int32(1),
							End:   proto.Int32(math.MaxInt32),
						},
					},
				},
			},
		}, nil)
		// When message sets are not supported, the above returns an error:
		//    message "MessageSet" is a MessageSet, which is a legacy proto1 feature that is no longer supported
		messageSetSupport = err == nil
	})
	return messageSetSupport
}
