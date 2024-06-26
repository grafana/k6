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

package options

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/bufbuild/protocompile/internal"
)

// StripSourceRetentionOptionsFromFile returns a file descriptor proto that omits any
// options in file that are defined to be retained only in source. If file has no
// such options, then it is returned as is. If it does have such options, a copy is
// made; the given file will not be mutated.
//
// Even when a copy is returned, it is not a deep copy: it may share data with the
// original file. So callers should not mutate the returned file unless mutating the
// input file is also safe.
func StripSourceRetentionOptionsFromFile(file *descriptorpb.FileDescriptorProto) (*descriptorpb.FileDescriptorProto, error) {
	var path sourcePath
	var removedPaths *sourcePathTrie
	if file.SourceCodeInfo != nil && len(file.SourceCodeInfo.Location) > 0 {
		path = make(sourcePath, 0, 16)
		removedPaths = &sourcePathTrie{}
	}
	var dirty bool
	optionsPath := path.push(internal.FileOptionsTag)
	newOpts, err := stripSourceRetentionOptions(file.GetOptions(), optionsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if newOpts != file.GetOptions() {
		dirty = true
	}
	msgsPath := path.push(internal.FileMessagesTag)
	newMsgs, changed, err := stripOptionsFromAll(file.GetMessageType(), stripSourceRetentionOptionsFromMessage, msgsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if changed {
		dirty = true
	}
	enumsPath := path.push(internal.FileEnumsTag)
	newEnums, changed, err := stripOptionsFromAll(file.GetEnumType(), stripSourceRetentionOptionsFromEnum, enumsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if changed {
		dirty = true
	}
	extsPath := path.push(internal.FileExtensionsTag)
	newExts, changed, err := stripOptionsFromAll(file.GetExtension(), stripSourceRetentionOptionsFromField, extsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if changed {
		dirty = true
	}
	svcsPath := path.push(internal.FileServicesTag)
	newSvcs, changed, err := stripOptionsFromAll(file.GetService(), stripSourceRetentionOptionsFromService, svcsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if changed {
		dirty = true
	}

	if !dirty {
		return file, nil
	}

	newFile, err := shallowCopy(file)
	if err != nil {
		return nil, err
	}
	newFile.Options = newOpts
	newFile.MessageType = newMsgs
	newFile.EnumType = newEnums
	newFile.Extension = newExts
	newFile.Service = newSvcs
	newFile.SourceCodeInfo = stripSourcePathsForSourceRetentionOptions(newFile.SourceCodeInfo, removedPaths)
	return newFile, nil
}

type sourcePath protoreflect.SourcePath

func (p sourcePath) push(element int32) sourcePath {
	if p == nil {
		return nil
	}
	return append(p, element)
}

type sourcePathTrie struct {
	removed  bool
	children map[int32]*sourcePathTrie
}

func (t *sourcePathTrie) addPath(p sourcePath) {
	if t == nil {
		return
	}
	if len(p) == 0 {
		t.removed = true
		return
	}
	child := t.children[p[0]]
	if child == nil {
		if t.children == nil {
			t.children = map[int32]*sourcePathTrie{}
		}
		child = &sourcePathTrie{}
		t.children[p[0]] = child
	}
	child.addPath(p[1:])
}

func (t *sourcePathTrie) isRemoved(p []int32) bool {
	if t == nil {
		return false
	}
	if t.removed {
		return true
	}
	if len(p) == 0 {
		return false
	}
	child := t.children[p[0]]
	if child == nil {
		return false
	}
	return child.isRemoved(p[1:])
}

func stripSourceRetentionOptions[M proto.Message](
	options M,
	path sourcePath,
	removedPaths *sourcePathTrie,
) (M, error) {
	optionsRef := options.ProtoReflect()
	// See if there are any options to strip.
	var hasFieldToStrip bool
	var numFieldsToKeep int
	var err error
	optionsRef.Range(func(field protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		fieldOpts, ok := field.Options().(*descriptorpb.FieldOptions)
		if !ok {
			err = fmt.Errorf("field options is unexpected type: got %T, want %T", field.Options(), fieldOpts)
			return false
		}
		if fieldOpts.GetRetention() == descriptorpb.FieldOptions_RETENTION_SOURCE {
			hasFieldToStrip = true
		} else {
			numFieldsToKeep++
		}
		return true
	})
	var zero M
	if err != nil {
		return zero, err
	}
	if !hasFieldToStrip {
		return options, nil
	}

	if numFieldsToKeep == 0 {
		// Stripping the message would remove *all* options. In that case,
		// we'll clear out the options by returning the zero value (i.e. nil).
		removedPaths.addPath(path) // clear out all source locations, too
		return zero, nil
	}

	// There is at least one option to remove. So we need to make a copy that does not have those options.
	newOptions := optionsRef.New()
	ret, ok := newOptions.Interface().(M)
	if !ok {
		return zero, fmt.Errorf("creating new message of same type resulted in unexpected type; got %T, want %T", newOptions.Interface(), zero)
	}
	optionsRef.Range(func(field protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		fieldOpts, ok := field.Options().(*descriptorpb.FieldOptions)
		if !ok {
			err = fmt.Errorf("field options is unexpected type: got %T, want %T", field.Options(), fieldOpts)
			return false
		}
		if fieldOpts.GetRetention() != descriptorpb.FieldOptions_RETENTION_SOURCE {
			newOptions.Set(field, val)
		} else {
			removedPaths.addPath(path.push(int32(field.Number())))
		}
		return true
	})
	if err != nil {
		return zero, err
	}
	return ret, nil
}

func stripSourceRetentionOptionsFromMessage(
	msg *descriptorpb.DescriptorProto,
	path sourcePath,
	removedPaths *sourcePathTrie,
) (*descriptorpb.DescriptorProto, error) {
	var dirty bool
	optionsPath := path.push(internal.MessageOptionsTag)
	newOpts, err := stripSourceRetentionOptions(msg.Options, optionsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if newOpts != msg.Options {
		dirty = true
	}
	fieldsPath := path.push(internal.MessageFieldsTag)
	newFields, changed, err := stripOptionsFromAll(msg.Field, stripSourceRetentionOptionsFromField, fieldsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if changed {
		dirty = true
	}
	oneofsPath := path.push(internal.MessageOneofsTag)
	newOneofs, changed, err := stripOptionsFromAll(msg.OneofDecl, stripSourceRetentionOptionsFromOneof, oneofsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if changed {
		dirty = true
	}
	extRangesPath := path.push(internal.MessageExtensionRangesTag)
	newExtRanges, changed, err := stripOptionsFromAll(msg.ExtensionRange, stripSourceRetentionOptionsFromExtensionRange, extRangesPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if changed {
		dirty = true
	}
	msgsPath := path.push(internal.MessageNestedMessagesTag)
	newMsgs, changed, err := stripOptionsFromAll(msg.NestedType, stripSourceRetentionOptionsFromMessage, msgsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if changed {
		dirty = true
	}
	enumsPath := path.push(internal.MessageEnumsTag)
	newEnums, changed, err := stripOptionsFromAll(msg.EnumType, stripSourceRetentionOptionsFromEnum, enumsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if changed {
		dirty = true
	}
	extsPath := path.push(internal.MessageExtensionsTag)
	newExts, changed, err := stripOptionsFromAll(msg.Extension, stripSourceRetentionOptionsFromField, extsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if changed {
		dirty = true
	}

	if !dirty {
		return msg, nil
	}

	newMsg, err := shallowCopy(msg)
	if err != nil {
		return nil, err
	}
	newMsg.Options = newOpts
	newMsg.Field = newFields
	newMsg.OneofDecl = newOneofs
	newMsg.ExtensionRange = newExtRanges
	newMsg.NestedType = newMsgs
	newMsg.EnumType = newEnums
	newMsg.Extension = newExts
	return newMsg, nil
}

func stripSourceRetentionOptionsFromField(
	field *descriptorpb.FieldDescriptorProto,
	path sourcePath,
	removedPaths *sourcePathTrie,
) (*descriptorpb.FieldDescriptorProto, error) {
	optionsPath := path.push(internal.FieldOptionsTag)
	newOpts, err := stripSourceRetentionOptions(field.Options, optionsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if newOpts == field.Options {
		return field, nil
	}
	newField, err := shallowCopy(field)
	if err != nil {
		return nil, err
	}
	newField.Options = newOpts
	return newField, nil
}

func stripSourceRetentionOptionsFromOneof(
	oneof *descriptorpb.OneofDescriptorProto,
	path sourcePath,
	removedPaths *sourcePathTrie,
) (*descriptorpb.OneofDescriptorProto, error) {
	optionsPath := path.push(internal.OneofOptionsTag)
	newOpts, err := stripSourceRetentionOptions(oneof.Options, optionsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if newOpts == oneof.Options {
		return oneof, nil
	}
	newOneof, err := shallowCopy(oneof)
	if err != nil {
		return nil, err
	}
	newOneof.Options = newOpts
	return newOneof, nil
}

func stripSourceRetentionOptionsFromExtensionRange(
	extRange *descriptorpb.DescriptorProto_ExtensionRange,
	path sourcePath,
	removedPaths *sourcePathTrie,
) (*descriptorpb.DescriptorProto_ExtensionRange, error) {
	optionsPath := path.push(internal.ExtensionRangeOptionsTag)
	newOpts, err := stripSourceRetentionOptions(extRange.Options, optionsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if newOpts == extRange.Options {
		return extRange, nil
	}
	newExtRange, err := shallowCopy(extRange)
	if err != nil {
		return nil, err
	}
	newExtRange.Options = newOpts
	return newExtRange, nil
}

func stripSourceRetentionOptionsFromEnum(
	enum *descriptorpb.EnumDescriptorProto,
	path sourcePath,
	removedPaths *sourcePathTrie,
) (*descriptorpb.EnumDescriptorProto, error) {
	var dirty bool
	optionsPath := path.push(internal.EnumOptionsTag)
	newOpts, err := stripSourceRetentionOptions(enum.Options, optionsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if newOpts != enum.Options {
		dirty = true
	}
	valsPath := path.push(internal.EnumValuesTag)
	newVals, changed, err := stripOptionsFromAll(enum.Value, stripSourceRetentionOptionsFromEnumValue, valsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if changed {
		dirty = true
	}

	if !dirty {
		return enum, nil
	}

	newEnum, err := shallowCopy(enum)
	if err != nil {
		return nil, err
	}
	newEnum.Options = newOpts
	newEnum.Value = newVals
	return newEnum, nil
}

func stripSourceRetentionOptionsFromEnumValue(
	enumVal *descriptorpb.EnumValueDescriptorProto,
	path sourcePath,
	removedPaths *sourcePathTrie,
) (*descriptorpb.EnumValueDescriptorProto, error) {
	optionsPath := path.push(internal.EnumValOptionsTag)
	newOpts, err := stripSourceRetentionOptions(enumVal.Options, optionsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if newOpts == enumVal.Options {
		return enumVal, nil
	}
	newEnumVal, err := shallowCopy(enumVal)
	if err != nil {
		return nil, err
	}
	newEnumVal.Options = newOpts
	return newEnumVal, nil
}

func stripSourceRetentionOptionsFromService(
	svc *descriptorpb.ServiceDescriptorProto,
	path sourcePath,
	removedPaths *sourcePathTrie,
) (*descriptorpb.ServiceDescriptorProto, error) {
	var dirty bool
	optionsPath := path.push(internal.ServiceOptionsTag)
	newOpts, err := stripSourceRetentionOptions(svc.Options, optionsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if newOpts != svc.Options {
		dirty = true
	}
	methodsPath := path.push(internal.ServiceMethodsTag)
	newMethods, changed, err := stripOptionsFromAll(svc.Method, stripSourceRetentionOptionsFromMethod, methodsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if changed {
		dirty = true
	}

	if !dirty {
		return svc, nil
	}

	newSvc, err := shallowCopy(svc)
	if err != nil {
		return nil, err
	}
	newSvc.Options = newOpts
	newSvc.Method = newMethods
	return newSvc, nil
}

func stripSourceRetentionOptionsFromMethod(
	method *descriptorpb.MethodDescriptorProto,
	path sourcePath,
	removedPaths *sourcePathTrie,
) (*descriptorpb.MethodDescriptorProto, error) {
	optionsPath := path.push(internal.MethodOptionsTag)
	newOpts, err := stripSourceRetentionOptions(method.Options, optionsPath, removedPaths)
	if err != nil {
		return nil, err
	}
	if newOpts == method.Options {
		return method, nil
	}
	newMethod, err := shallowCopy(method)
	if err != nil {
		return nil, err
	}
	newMethod.Options = newOpts
	return newMethod, nil
}

func stripSourcePathsForSourceRetentionOptions(
	sourceInfo *descriptorpb.SourceCodeInfo,
	removedPaths *sourcePathTrie,
) *descriptorpb.SourceCodeInfo {
	if sourceInfo == nil || len(sourceInfo.Location) == 0 || removedPaths == nil {
		// nothing to do
		return sourceInfo
	}
	newLocations := make([]*descriptorpb.SourceCodeInfo_Location, len(sourceInfo.Location))
	var i int
	for _, loc := range sourceInfo.Location {
		if removedPaths.isRemoved(loc.Path) {
			continue
		}
		newLocations[i] = loc
		i++
	}
	newLocations = newLocations[:i]
	return &descriptorpb.SourceCodeInfo{Location: newLocations}
}

func shallowCopy[M proto.Message](msg M) (M, error) {
	msgRef := msg.ProtoReflect()
	other := msgRef.New()
	ret, ok := other.Interface().(M)
	if !ok {
		return ret, fmt.Errorf("creating new message of same type resulted in unexpected type; got %T, want %T", other.Interface(), ret)
	}
	msgRef.Range(func(field protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		other.Set(field, val)
		return true
	})
	return ret, nil
}

// stripOptionsFromAll applies the given function to each element in the given
// slice in order to remove source-retention options from it. It returns the new
// slice and a bool indicating whether anything was actually changed. If the
// second value is false, then the returned slice is the same slice as the input
// slice. Usually, T is a pointer type, in which case the given updateFunc should
// NOT mutate the input value. Instead, it should return the input value if only
// if there is no update needed. If a mutation is needed, it should return a new
// value.
func stripOptionsFromAll[T comparable](
	slice []T,
	updateFunc func(T, sourcePath, *sourcePathTrie) (T, error),
	path sourcePath,
	removedPaths *sourcePathTrie,
) ([]T, bool, error) {
	var updated []T // initialized lazily, only when/if a copy is needed
	for i, item := range slice {
		newItem, err := updateFunc(item, path.push(int32(i)), removedPaths)
		if err != nil {
			return nil, false, err
		}
		if updated != nil {
			updated[i] = newItem
		} else if newItem != item {
			updated = make([]T, len(slice))
			copy(updated[:i], slice)
			updated[i] = newItem
		}
	}
	if updated != nil {
		return updated, true, nil
	}
	return slice, false, nil
}
