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

package linker

import "google.golang.org/protobuf/types/descriptorpb"

// allocPool helps allocate descriptor instances. Instead of allocating
// them one at a time, we allocate a pool -- a large, flat slice to hold
// all descriptors of a particular kind for a file. We then use capacity
// in the pool when we need space for individual descriptors.
type allocPool struct {
	numMessages   int
	numFields     int
	numOneofs     int
	numEnums      int
	numEnumValues int
	numExtensions int
	numServices   int
	numMethods    int

	messages   []msgDescriptor
	fields     []fldDescriptor
	oneofs     []oneofDescriptor
	enums      []enumDescriptor
	enumVals   []enValDescriptor
	extensions []extTypeDescriptor
	services   []svcDescriptor
	methods    []mtdDescriptor
}

func newAllocPool(file *descriptorpb.FileDescriptorProto) *allocPool {
	var pool allocPool
	pool.countElements(file)
	pool.messages = make([]msgDescriptor, pool.numMessages)
	pool.fields = make([]fldDescriptor, pool.numFields)
	pool.oneofs = make([]oneofDescriptor, pool.numOneofs)
	pool.enums = make([]enumDescriptor, pool.numEnums)
	pool.enumVals = make([]enValDescriptor, pool.numEnumValues)
	pool.extensions = make([]extTypeDescriptor, pool.numExtensions)
	pool.services = make([]svcDescriptor, pool.numServices)
	pool.methods = make([]mtdDescriptor, pool.numMethods)
	return &pool
}

func (p *allocPool) getMessages(count int) []msgDescriptor {
	allocated := p.messages[:count]
	p.messages = p.messages[count:]
	return allocated
}

func (p *allocPool) getFields(count int) []fldDescriptor {
	allocated := p.fields[:count]
	p.fields = p.fields[count:]
	return allocated
}

func (p *allocPool) getOneofs(count int) []oneofDescriptor {
	allocated := p.oneofs[:count]
	p.oneofs = p.oneofs[count:]
	return allocated
}

func (p *allocPool) getEnums(count int) []enumDescriptor {
	allocated := p.enums[:count]
	p.enums = p.enums[count:]
	return allocated
}

func (p *allocPool) getEnumValues(count int) []enValDescriptor {
	allocated := p.enumVals[:count]
	p.enumVals = p.enumVals[count:]
	return allocated
}

func (p *allocPool) getExtensions(count int) []extTypeDescriptor {
	allocated := p.extensions[:count]
	p.extensions = p.extensions[count:]
	return allocated
}

func (p *allocPool) getServices(count int) []svcDescriptor {
	allocated := p.services[:count]
	p.services = p.services[count:]
	return allocated
}

func (p *allocPool) getMethods(count int) []mtdDescriptor {
	allocated := p.methods[:count]
	p.methods = p.methods[count:]
	return allocated
}

func (p *allocPool) countElements(file *descriptorpb.FileDescriptorProto) {
	p.countElementsInMessages(file.MessageType)
	p.countElementsInEnums(file.EnumType)
	p.numExtensions += len(file.Extension)
	p.numServices += len(file.Service)
	for _, svc := range file.Service {
		p.numMethods += len(svc.Method)
	}
}

func (p *allocPool) countElementsInMessages(msgs []*descriptorpb.DescriptorProto) {
	p.numMessages += len(msgs)
	for _, msg := range msgs {
		p.numFields += len(msg.Field)
		p.numOneofs += len(msg.OneofDecl)
		p.countElementsInMessages(msg.NestedType)
		p.countElementsInEnums(msg.EnumType)
		p.numExtensions += len(msg.Extension)
	}
}

func (p *allocPool) countElementsInEnums(enums []*descriptorpb.EnumDescriptorProto) {
	p.numEnums += len(enums)
	for _, enum := range enums {
		p.numEnumValues += len(enum.Value)
	}
}
