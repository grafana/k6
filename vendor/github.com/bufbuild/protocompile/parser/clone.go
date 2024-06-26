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

package parser

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/reporter"
)

// Clone returns a copy of the given result. Since descriptor protos may be
// mutated during linking, this can return a defensive copy so that mutations
// don't impact concurrent operations in an unsafe way. This is called if the
// parse result could be re-used across concurrent operations and has unresolved
// references and options which will require mutation by the linker.
//
// If the given value has a method with the following signature, it will be
// called to perform the operation:
//
//	Clone() Result
//
// If the given value does not provide a Clone method and is not the implementation
// provided by this package, it is possible for an error to occur in creating the
// copy, which may result in a panic. This can happen if the AST of the given result
// is not actually valid and a file descriptor proto cannot be successfully derived
// from it.
func Clone(r Result) Result {
	if cl, ok := r.(interface{ Clone() Result }); ok {
		return cl.Clone()
	}
	if res, ok := r.(*result); ok {
		newProto := proto.Clone(res.proto).(*descriptorpb.FileDescriptorProto) //nolint:errcheck
		newNodes := make(map[proto.Message]ast.Node, len(res.nodes))
		newResult := &result{
			file:  res.file,
			proto: newProto,
			nodes: newNodes,
		}
		recreateNodeIndexForFile(res, newResult, res.proto, newProto)
		return newResult
	}

	// Can't do the deep-copy we know how to do. So we have to take a
	// different tactic.
	if r.AST() == nil {
		// no AST? all we have to do is copy the proto
		fileProto := proto.Clone(r.FileDescriptorProto()).(*descriptorpb.FileDescriptorProto) //nolint:errcheck
		return ResultWithoutAST(fileProto)
	}
	// Otherwise, we have an AST, but no way to clone the result's
	// internals. So just re-create them from scratch.
	res, err := ResultFromAST(r.AST(), false, reporter.NewHandler(nil))
	if err != nil {
		panic(err)
	}
	return res
}

func recreateNodeIndexForFile(orig, clone *result, origProto, cloneProto *descriptorpb.FileDescriptorProto) {
	updateNodeIndexWithOptions[*descriptorpb.FileOptions](orig, clone, origProto, cloneProto)
	for i, origMd := range origProto.MessageType {
		cloneMd := cloneProto.MessageType[i]
		recreateNodeIndexForMessage(orig, clone, origMd, cloneMd)
	}
	for i, origEd := range origProto.EnumType {
		cloneEd := cloneProto.EnumType[i]
		recreateNodeIndexForEnum(orig, clone, origEd, cloneEd)
	}
	for i, origExtd := range origProto.Extension {
		cloneExtd := cloneProto.Extension[i]
		updateNodeIndexWithOptions[*descriptorpb.FieldOptions](orig, clone, origExtd, cloneExtd)
	}
	for i, origSd := range origProto.Service {
		cloneSd := cloneProto.Service[i]
		updateNodeIndexWithOptions[*descriptorpb.ServiceOptions](orig, clone, origSd, cloneSd)
		for j, origMtd := range origSd.Method {
			cloneMtd := cloneSd.Method[j]
			updateNodeIndexWithOptions[*descriptorpb.MethodOptions](orig, clone, origMtd, cloneMtd)
		}
	}
}

func recreateNodeIndexForMessage(orig, clone *result, origProto, cloneProto *descriptorpb.DescriptorProto) {
	updateNodeIndexWithOptions[*descriptorpb.MessageOptions](orig, clone, origProto, cloneProto)
	for i, origFld := range origProto.Field {
		cloneFld := cloneProto.Field[i]
		updateNodeIndexWithOptions[*descriptorpb.FieldOptions](orig, clone, origFld, cloneFld)
	}
	for i, origOod := range origProto.OneofDecl {
		cloneOod := cloneProto.OneofDecl[i]
		updateNodeIndexWithOptions[*descriptorpb.OneofOptions](orig, clone, origOod, cloneOod)
	}
	for i, origExtr := range origProto.ExtensionRange {
		cloneExtr := cloneProto.ExtensionRange[i]
		updateNodeIndex(orig, clone, asExtsNode(origExtr), asExtsNode(cloneExtr))
		updateNodeIndexWithOptions[*descriptorpb.ExtensionRangeOptions](orig, clone, origExtr, cloneExtr)
	}
	for i, origRr := range origProto.ReservedRange {
		cloneRr := cloneProto.ReservedRange[i]
		updateNodeIndex(orig, clone, origRr, cloneRr)
	}
	for i, origNmd := range origProto.NestedType {
		cloneNmd := cloneProto.NestedType[i]
		recreateNodeIndexForMessage(orig, clone, origNmd, cloneNmd)
	}
	for i, origEd := range origProto.EnumType {
		cloneEd := cloneProto.EnumType[i]
		recreateNodeIndexForEnum(orig, clone, origEd, cloneEd)
	}
	for i, origExtd := range origProto.Extension {
		cloneExtd := cloneProto.Extension[i]
		updateNodeIndexWithOptions[*descriptorpb.FieldOptions](orig, clone, origExtd, cloneExtd)
	}
}

func recreateNodeIndexForEnum(orig, clone *result, origProto, cloneProto *descriptorpb.EnumDescriptorProto) {
	updateNodeIndexWithOptions[*descriptorpb.EnumOptions](orig, clone, origProto, cloneProto)
	for i, origEvd := range origProto.Value {
		cloneEvd := cloneProto.Value[i]
		updateNodeIndexWithOptions[*descriptorpb.EnumValueOptions](orig, clone, origEvd, cloneEvd)
	}
	for i, origRr := range origProto.ReservedRange {
		cloneRr := cloneProto.ReservedRange[i]
		updateNodeIndex(orig, clone, origRr, cloneRr)
	}
}

func recreateNodeIndexForOptions(orig, clone *result, origProtos, cloneProtos []*descriptorpb.UninterpretedOption) {
	for i, origOpt := range origProtos {
		cloneOpt := cloneProtos[i]
		updateNodeIndex(orig, clone, origOpt, cloneOpt)
		for j, origName := range origOpt.Name {
			cloneName := cloneOpt.Name[j]
			updateNodeIndex(orig, clone, origName, cloneName)
		}
	}
}

func updateNodeIndex[M proto.Message](orig, clone *result, origProto, cloneProto M) {
	node := orig.nodes[origProto]
	if node != nil {
		clone.nodes[cloneProto] = node
	}
}

type pointerMessage[T any] interface {
	*T
	proto.Message
}

type options[T any] interface {
	// need this type instead of just proto.Message so we can check for nil pointer
	pointerMessage[T]
	GetUninterpretedOption() []*descriptorpb.UninterpretedOption
}

type withOptions[O options[T], T any] interface {
	proto.Message
	GetOptions() O
}

func updateNodeIndexWithOptions[O options[T], M withOptions[O, T], T any](orig, clone *result, origProto, cloneProto M) {
	updateNodeIndex(orig, clone, origProto, cloneProto)
	origOpts := origProto.GetOptions()
	cloneOpts := cloneProto.GetOptions()
	if origOpts != nil {
		recreateNodeIndexForOptions(orig, clone, origOpts.GetUninterpretedOption(), cloneOpts.GetUninterpretedOption())
	}
}
