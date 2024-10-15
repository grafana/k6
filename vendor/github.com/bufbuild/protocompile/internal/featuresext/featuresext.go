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

// Package featuresext provides file descriptors for the
// "google/protobuf/cpp_features.proto" and "google/protobuf/java_features.proto"
// standard import files. Unlike the other standard/well-known
// imports, these files have no standard Go package in their
// runtime with generated code. So in order to make them available
// as "standard imports" to compiler users, we must embed these
// descriptors into a Go package.
package featuresext

import (
	_ "embed"
	"fmt"
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

var (
	//go:embed cpp_features.protoset
	cppFeatures []byte

	//go:embed java_features.protoset
	javaFeatures []byte

	initOnce         sync.Once
	initCppFeatures  protoreflect.FileDescriptor
	initCppErr       error
	initJavaFeatures protoreflect.FileDescriptor
	initJavaErr      error
)

func initDescriptors() {
	initOnce.Do(func() {
		initCppFeatures, initCppErr = buildDescriptor("google/protobuf/cpp_features.proto", cppFeatures)
		initJavaFeatures, initJavaErr = buildDescriptor("google/protobuf/java_features.proto", javaFeatures)
	})
}

func CppFeaturesDescriptor() (protoreflect.FileDescriptor, error) {
	initDescriptors()
	return initCppFeatures, initCppErr
}

func JavaFeaturesDescriptor() (protoreflect.FileDescriptor, error) {
	initDescriptors()
	return initJavaFeatures, initJavaErr
}

func buildDescriptor(name string, data []byte) (protoreflect.FileDescriptor, error) {
	var files descriptorpb.FileDescriptorSet
	err := proto.Unmarshal(data, &files)
	if err != nil {
		return nil, fmt.Errorf("failed to load descriptor for %q: %w", name, err)
	}
	if len(files.File) != 1 {
		return nil, fmt.Errorf("failed to load descriptor for %q: expected embedded descriptor set to contain exactly one file but it instead has %d", name, len(files.File))
	}
	if files.File[0].GetName() != name {
		return nil, fmt.Errorf("failed to load descriptor for %q: embedded descriptor contains wrong file %q", name, files.File[0].GetName())
	}
	descriptor, err := protodesc.NewFile(files.File[0], protoregistry.GlobalFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to load descriptor for %q: %w", name, err)
	}
	return descriptor, nil
}
