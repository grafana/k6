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

// Package walk provides helper functions for traversing all elements in a
// protobuf file descriptor. There are versions both for traversing "rich"
// descriptors (protoreflect.Descriptor) and for traversing the underlying
// "raw" descriptor protos.
//
// # Enter And Exit
//
// This package includes variants of the functions that accept two callback
// functions. These variants have names ending with "EnterAndExit". One function
// is called as each element is visited ("enter") and the other is called after
// the element and all of its descendants have been visited ("exit"). This
// can be useful when you need to track state that is scoped to the visitation
// of a single element.
//
// # Source Path
//
// When traversing raw descriptor protos, this package include variants whose
// callback accepts a protoreflect.SourcePath. These variants have names that
// include "WithPath". This path can be used to locate corresponding data in the
// file's source code info (if present).
package walk

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/bufbuild/protocompile/internal"
)

// Descriptors walks all descriptors in the given file using a depth-first
// traversal, calling the given function for each descriptor in the hierarchy.
// The walk ends when traversal is complete or when the function returns an
// error. If the function returns an error, that is returned as the result of the
// walk operation.
//
// Descriptors are visited using a pre-order traversal, where the function is
// called for a descriptor before it is called for any of its descendants.
func Descriptors(file protoreflect.FileDescriptor, fn func(protoreflect.Descriptor) error) error {
	return DescriptorsEnterAndExit(file, fn, nil)
}

// DescriptorsEnterAndExit walks all descriptors in the given file using a
// depth-first traversal, calling the given functions on entry and on exit
// for each descriptor in the hierarchy. The walk ends when traversal is
// complete or when a function returns an error. If a function returns an error,
// that is returned as the result of the walk operation.
//
// The enter function is called using a pre-order traversal, where the function
// is called for a descriptor before it is called for any of its descendants.
// The exit function is called using a post-order traversal, where the function
// is called for a descriptor only after it is called for any descendants.
func DescriptorsEnterAndExit(file protoreflect.FileDescriptor, enter, exit func(protoreflect.Descriptor) error) error {
	for i := 0; i < file.Messages().Len(); i++ {
		msg := file.Messages().Get(i)
		if err := messageDescriptor(msg, enter, exit); err != nil {
			return err
		}
	}
	for i := 0; i < file.Enums().Len(); i++ {
		en := file.Enums().Get(i)
		if err := enumDescriptor(en, enter, exit); err != nil {
			return err
		}
	}
	for i := 0; i < file.Extensions().Len(); i++ {
		ext := file.Extensions().Get(i)
		if err := enter(ext); err != nil {
			return err
		}
		if exit != nil {
			if err := exit(ext); err != nil {
				return err
			}
		}
	}
	for i := 0; i < file.Services().Len(); i++ {
		svc := file.Services().Get(i)
		if err := enter(svc); err != nil {
			return err
		}
		for i := 0; i < svc.Methods().Len(); i++ {
			mtd := svc.Methods().Get(i)
			if err := enter(mtd); err != nil {
				return err
			}
			if exit != nil {
				if err := exit(mtd); err != nil {
					return err
				}
			}
		}
		if exit != nil {
			if err := exit(svc); err != nil {
				return err
			}
		}
	}
	return nil
}

func messageDescriptor(msg protoreflect.MessageDescriptor, enter, exit func(protoreflect.Descriptor) error) error {
	if err := enter(msg); err != nil {
		return err
	}
	for i := 0; i < msg.Fields().Len(); i++ {
		fld := msg.Fields().Get(i)
		if err := enter(fld); err != nil {
			return err
		}
		if exit != nil {
			if err := exit(fld); err != nil {
				return err
			}
		}
	}
	for i := 0; i < msg.Oneofs().Len(); i++ {
		oo := msg.Oneofs().Get(i)
		if err := enter(oo); err != nil {
			return err
		}
		if exit != nil {
			if err := exit(oo); err != nil {
				return err
			}
		}
	}
	for i := 0; i < msg.Messages().Len(); i++ {
		nested := msg.Messages().Get(i)
		if err := messageDescriptor(nested, enter, exit); err != nil {
			return err
		}
	}
	for i := 0; i < msg.Enums().Len(); i++ {
		en := msg.Enums().Get(i)
		if err := enumDescriptor(en, enter, exit); err != nil {
			return err
		}
	}
	for i := 0; i < msg.Extensions().Len(); i++ {
		ext := msg.Extensions().Get(i)
		if err := enter(ext); err != nil {
			return err
		}
		if exit != nil {
			if err := exit(ext); err != nil {
				return err
			}
		}
	}
	if exit != nil {
		if err := exit(msg); err != nil {
			return err
		}
	}
	return nil
}

func enumDescriptor(en protoreflect.EnumDescriptor, enter, exit func(protoreflect.Descriptor) error) error {
	if err := enter(en); err != nil {
		return err
	}
	for i := 0; i < en.Values().Len(); i++ {
		enVal := en.Values().Get(i)
		if err := enter(enVal); err != nil {
			return err
		}
		if exit != nil {
			if err := exit(enVal); err != nil {
				return err
			}
		}
	}
	if exit != nil {
		if err := exit(en); err != nil {
			return err
		}
	}
	return nil
}

// DescriptorProtosWithPath walks all descriptor protos in the given file using
// a depth-first traversal. This is the same as DescriptorProtos except that the
// callback function, fn, receives a protoreflect.SourcePath, that indicates the
// path for the element in the file's source code info.
func DescriptorProtosWithPath(file *descriptorpb.FileDescriptorProto, fn func(protoreflect.FullName, protoreflect.SourcePath, proto.Message) error) error {
	return DescriptorProtosWithPathEnterAndExit(file, fn, nil)
}

// DescriptorProtosWithPathEnterAndExit walks all descriptor protos in the given
// file using a depth-first traversal. This is the same as
// DescriptorProtosEnterAndExit except that the callback function, fn, receives
// a protoreflect.SourcePath, that indicates the path for the element in the
// file's source code info.
func DescriptorProtosWithPathEnterAndExit(file *descriptorpb.FileDescriptorProto, enter, exit func(protoreflect.FullName, protoreflect.SourcePath, proto.Message) error) error {
	w := &protoWalker{usePath: true, enter: enter, exit: exit}
	return w.walkDescriptorProtos(file)
}

// DescriptorProtos walks all descriptor protos in the given file using a
// depth-first traversal, calling the given function for each descriptor proto
// in the hierarchy. The walk ends when traversal is complete or when the
// function returns an error. If the function returns an error, that is
// returned as the result of the walk operation.
//
// Descriptor protos are visited using a pre-order traversal, where the function
// is called for a descriptor before it is called for any of its descendants.
func DescriptorProtos(file *descriptorpb.FileDescriptorProto, fn func(protoreflect.FullName, proto.Message) error) error {
	return DescriptorProtosEnterAndExit(file, fn, nil)
}

// DescriptorProtosEnterAndExit walks all descriptor protos in the given file
// using a depth-first traversal, calling the given functions on entry and on
// exit for each descriptor in the hierarchy. The walk ends when traversal is
// complete or when a function returns an error. If a function returns an error,
// that is returned as the result of the walk operation.
//
// The enter function is called using a pre-order traversal, where the function
// is called for a descriptor proto before it is called for any of its
// descendants. The exit function is called using a post-order traversal, where
// the function is called for a descriptor proto only after it is called for any
// descendants.
func DescriptorProtosEnterAndExit(file *descriptorpb.FileDescriptorProto, enter, exit func(protoreflect.FullName, proto.Message) error) error {
	enterWithPath := func(n protoreflect.FullName, p protoreflect.SourcePath, m proto.Message) error {
		return enter(n, m)
	}
	var exitWithPath func(n protoreflect.FullName, p protoreflect.SourcePath, m proto.Message) error
	if exit != nil {
		exitWithPath = func(n protoreflect.FullName, p protoreflect.SourcePath, m proto.Message) error {
			return exit(n, m)
		}
	}
	w := &protoWalker{
		enter: enterWithPath,
		exit:  exitWithPath,
	}
	return w.walkDescriptorProtos(file)
}

type protoWalker struct {
	usePath     bool
	enter, exit func(protoreflect.FullName, protoreflect.SourcePath, proto.Message) error
}

func (w *protoWalker) walkDescriptorProtos(file *descriptorpb.FileDescriptorProto) error {
	prefix := file.GetPackage()
	if prefix != "" {
		prefix += "."
	}
	var path protoreflect.SourcePath
	for i, msg := range file.MessageType {
		var p protoreflect.SourcePath
		if w.usePath {
			p = path
			p = append(p, internal.FileMessagesTag, int32(i))
		}
		if err := w.walkDescriptorProto(prefix, p, msg); err != nil {
			return err
		}
	}
	for i, en := range file.EnumType {
		var p protoreflect.SourcePath
		if w.usePath {
			p = path
			p = append(p, internal.FileEnumsTag, int32(i))
		}
		if err := w.walkEnumDescriptorProto(prefix, p, en); err != nil {
			return err
		}
	}
	for i, ext := range file.Extension {
		var p protoreflect.SourcePath
		if w.usePath {
			p = path
			p = append(p, internal.FileExtensionsTag, int32(i))
		}
		fqn := prefix + ext.GetName()
		if err := w.enter(protoreflect.FullName(fqn), p, ext); err != nil {
			return err
		}
		if w.exit != nil {
			if err := w.exit(protoreflect.FullName(fqn), p, ext); err != nil {
				return err
			}
		}
	}
	for i, svc := range file.Service {
		var p protoreflect.SourcePath
		if w.usePath {
			p = path
			p = append(p, internal.FileServicesTag, int32(i))
		}
		fqn := prefix + svc.GetName()
		if err := w.enter(protoreflect.FullName(fqn), p, svc); err != nil {
			return err
		}
		for j, mtd := range svc.Method {
			var mp protoreflect.SourcePath
			if w.usePath {
				mp = p
				mp = append(mp, internal.ServiceMethodsTag, int32(j))
			}
			mtdFqn := fqn + "." + mtd.GetName()
			if err := w.enter(protoreflect.FullName(mtdFqn), mp, mtd); err != nil {
				return err
			}
			if w.exit != nil {
				if err := w.exit(protoreflect.FullName(mtdFqn), mp, mtd); err != nil {
					return err
				}
			}
		}
		if w.exit != nil {
			if err := w.exit(protoreflect.FullName(fqn), p, svc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *protoWalker) walkDescriptorProto(prefix string, path protoreflect.SourcePath, msg *descriptorpb.DescriptorProto) error {
	fqn := prefix + msg.GetName()
	if err := w.enter(protoreflect.FullName(fqn), path, msg); err != nil {
		return err
	}
	prefix = fqn + "."
	for i, fld := range msg.Field {
		var p protoreflect.SourcePath
		if w.usePath {
			p = path
			p = append(p, internal.MessageFieldsTag, int32(i))
		}
		fqn := prefix + fld.GetName()
		if err := w.enter(protoreflect.FullName(fqn), p, fld); err != nil {
			return err
		}
		if w.exit != nil {
			if err := w.exit(protoreflect.FullName(fqn), p, fld); err != nil {
				return err
			}
		}
	}
	for i, oo := range msg.OneofDecl {
		var p protoreflect.SourcePath
		if w.usePath {
			p = path
			p = append(p, internal.MessageOneofsTag, int32(i))
		}
		fqn := prefix + oo.GetName()
		if err := w.enter(protoreflect.FullName(fqn), p, oo); err != nil {
			return err
		}
		if w.exit != nil {
			if err := w.exit(protoreflect.FullName(fqn), p, oo); err != nil {
				return err
			}
		}
	}
	for i, nested := range msg.NestedType {
		var p protoreflect.SourcePath
		if w.usePath {
			p = path
			p = append(p, internal.MessageNestedMessagesTag, int32(i))
		}
		if err := w.walkDescriptorProto(prefix, p, nested); err != nil {
			return err
		}
	}
	for i, en := range msg.EnumType {
		var p protoreflect.SourcePath
		if w.usePath {
			p = path
			p = append(p, internal.MessageEnumsTag, int32(i))
		}
		if err := w.walkEnumDescriptorProto(prefix, p, en); err != nil {
			return err
		}
	}
	for i, ext := range msg.Extension {
		var p protoreflect.SourcePath
		if w.usePath {
			p = path
			p = append(p, internal.MessageExtensionsTag, int32(i))
		}
		fqn := prefix + ext.GetName()
		if err := w.enter(protoreflect.FullName(fqn), p, ext); err != nil {
			return err
		}
		if w.exit != nil {
			if err := w.exit(protoreflect.FullName(fqn), p, ext); err != nil {
				return err
			}
		}
	}
	if w.exit != nil {
		if err := w.exit(protoreflect.FullName(fqn), path, msg); err != nil {
			return err
		}
	}
	return nil
}

func (w *protoWalker) walkEnumDescriptorProto(prefix string, path protoreflect.SourcePath, en *descriptorpb.EnumDescriptorProto) error {
	fqn := prefix + en.GetName()
	if err := w.enter(protoreflect.FullName(fqn), path, en); err != nil {
		return err
	}
	for i, val := range en.Value {
		var p protoreflect.SourcePath
		if w.usePath {
			p = path
			p = append(p, internal.EnumValuesTag, int32(i))
		}
		fqn := prefix + val.GetName()
		if err := w.enter(protoreflect.FullName(fqn), p, val); err != nil {
			return err
		}
		if w.exit != nil {
			if err := w.exit(protoreflect.FullName(fqn), p, val); err != nil {
				return err
			}
		}
	}
	if w.exit != nil {
		if err := w.exit(protoreflect.FullName(fqn), path, en); err != nil {
			return err
		}
	}
	return nil
}
