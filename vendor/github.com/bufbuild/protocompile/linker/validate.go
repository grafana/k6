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

import (
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/internal"
	"github.com/bufbuild/protocompile/protoutil"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/bufbuild/protocompile/walk"
)

// ValidateOptions runs some validation checks on the result that can only
// be done after options are interpreted.
func (r *result) ValidateOptions(handler *reporter.Handler, symbols *Symbols) error {
	if err := r.validateFile(handler); err != nil {
		return err
	}
	return walk.Descriptors(r, func(d protoreflect.Descriptor) error {
		switch d := d.(type) {
		case protoreflect.FieldDescriptor:
			if err := r.validateField(d, handler); err != nil {
				return err
			}
		case protoreflect.MessageDescriptor:
			if symbols == nil {
				symbols = &Symbols{}
			}
			if err := r.validateMessage(d, handler, symbols); err != nil {
				return err
			}
		case protoreflect.EnumDescriptor:
			if err := r.validateEnum(d, handler); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *result) validateFile(handler *reporter.Handler) error {
	opts := r.FileDescriptorProto().GetOptions()
	if opts.GetOptimizeFor() != descriptorpb.FileOptions_LITE_RUNTIME {
		// Non-lite files may not import lite files.
		imports := r.Imports()
		for i, length := 0, imports.Len(); i < length; i++ {
			dep := imports.Get(i)
			depOpts, ok := dep.Options().(*descriptorpb.FileOptions)
			if !ok {
				continue // what else to do?
			}
			if depOpts.GetOptimizeFor() == descriptorpb.FileOptions_LITE_RUNTIME {
				err := handler.HandleErrorf(r.getImportLocation(dep.Path()), "a file that does not use optimize_for=LITE_RUNTIME may not import file %q that does", dep.Path())
				if err != nil {
					return err
				}
			}
		}
	}
	if isEditions(r) {
		// Validate features
		if opts.GetFeatures().GetFieldPresence() == descriptorpb.FeatureSet_LEGACY_REQUIRED {
			span := r.findOptionSpan(r, internal.FileOptionsFeaturesTag, internal.FeatureSetFieldPresenceTag)
			err := handler.HandleErrorf(span, "LEGACY_REQUIRED field presence cannot be set as the default for a file")
			if err != nil {
				return err
			}
		}
		if opts != nil && opts.JavaStringCheckUtf8 != nil {
			span := r.findOptionSpan(r, internal.FileOptionsJavaStringCheckUTF8Tag)
			err := handler.HandleErrorf(span, `file option java_string_check_utf8 is not allowed with editions; import "google/protobuf/java_features.proto" and use (pb.java).utf8_validation instead`)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *result) validateField(fld protoreflect.FieldDescriptor, handler *reporter.Handler) error {
	if xtd, ok := fld.(protoreflect.ExtensionTypeDescriptor); ok {
		fld = xtd.Descriptor()
	}
	fd, ok := fld.(*fldDescriptor)
	if !ok {
		// should not be possible
		return fmt.Errorf("field descriptor is wrong type: expecting %T, got %T", (*fldDescriptor)(nil), fld)
	}

	if err := r.validatePacked(fd, handler); err != nil {
		return err
	}
	if fd.Kind() == protoreflect.EnumKind {
		requiresOpen := !fd.IsList() && !fd.HasPresence()
		if requiresOpen && fd.Enum().IsClosed() {
			// Fields in a proto3 message cannot refer to proto2 enums.
			// In editions, this translates to implicit presence fields
			// not being able to refer to closed enums.
			// TODO: This really should be based solely on whether the enum's first
			//       value is zero, NOT based on if it's open vs closed.
			//       https://github.com/protocolbuffers/protobuf/issues/16249
			file := r.FileNode()
			info := file.NodeInfo(r.FieldNode(fd.proto).FieldType())
			if err := handler.HandleErrorf(info, "cannot use closed enum %s in a field with implicit presence", fd.Enum().FullName()); err != nil {
				return err
			}
		}
	}
	if fd.HasDefault() && !fd.HasPresence() {
		span := r.findScalarOptionSpan(r.FieldNode(fd.proto), "default")
		err := handler.HandleErrorf(span, "default value is not allowed on fields with implicit presence")
		if err != nil {
			return err
		}
	}
	if fd.proto.Options != nil && fd.proto.Options.Ctype != nil {
		if descriptorpb.Edition(r.Edition()) >= descriptorpb.Edition_EDITION_2024 {
			// We don't support edition 2024 yet, but we went ahead and mimic'ed this check
			// from protoc, which currently has experimental support for 2024.
			span := r.findOptionSpan(fd, internal.FieldOptionsCTypeTag)
			if err := handler.HandleErrorf(span, "ctype option cannot be used as of edition 2024; use features.string_type instead"); err != nil {
				return err
			}
		} else if descriptorpb.Edition(r.Edition()) == descriptorpb.Edition_EDITION_2023 {
			if fld.Kind() != protoreflect.StringKind && fld.Kind() != protoreflect.BytesKind {
				span := r.findOptionSpan(fd, internal.FieldOptionsCTypeTag)
				if err := handler.HandleErrorf(span, "ctype option can only be used on string and bytes fields"); err != nil {
					return err
				}
			}
			if fd.proto.Options.GetCtype() == descriptorpb.FieldOptions_CORD && fd.IsExtension() {
				span := r.findOptionSpan(fd, internal.FieldOptionsCTypeTag)
				if err := handler.HandleErrorf(span, "ctype option cannot be CORD for extension fields"); err != nil {
					return err
				}
			}
		}
	}
	if (fd.proto.Options.GetLazy() || fd.proto.Options.GetUnverifiedLazy()) && fd.Kind() != protoreflect.MessageKind {
		var span ast.SourceSpan
		var optionName string
		if fd.proto.Options.GetLazy() {
			span = r.findOptionSpan(fd, internal.FieldOptionsLazyTag)
			optionName = "lazy"
		} else {
			span = r.findOptionSpan(fd, internal.FieldOptionsUnverifiedLazyTag)
			optionName = "unverified_lazy"
		}
		var suffix string
		if fd.Kind() == protoreflect.GroupKind {
			if isEditions(r) {
				suffix = " that use length-prefixed encoding"
			} else {
				suffix = ", not groups"
			}
		}
		if err := handler.HandleErrorf(span, "%s option can only be used with message fields%s", optionName, suffix); err != nil {
			return err
		}
	}
	if fd.proto.Options.GetJstype() != descriptorpb.FieldOptions_JS_NORMAL {
		switch fd.Kind() {
		case protoreflect.Int64Kind, protoreflect.Uint64Kind, protoreflect.Sint64Kind,
			protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind:
			// allowed only for 64-bit integer types
		default:
			span := r.findOptionSpan(fd, internal.FieldOptionsJSTypeTag)
			err := handler.HandleErrorf(span, "only 64-bit integer fields (int64, uint64, sint64, fixed64, and sfixed64) can specify a jstype other than JS_NORMAL")
			if err != nil {
				return err
			}
		}
	}
	if isEditions(r) {
		if err := r.validateFieldFeatures(fd, handler); err != nil {
			return err
		}
	}

	if fld.IsExtension() {
		// More checks if this is an extension field.
		if err := r.validateExtension(fd, handler); err != nil {
			return err
		}
	}

	return nil
}

func (r *result) validateExtension(fd *fldDescriptor, handler *reporter.Handler) error {
	// NB: It's a little gross that we don't enforce these in validateBasic().
	// But it requires linking to resolve the extendee, so we can interrogate
	// its descriptor.
	msg := fd.ContainingMessage()
	if msg.Options().(*descriptorpb.MessageOptions).GetMessageSetWireFormat() {
		// Message set wire format requires that all extensions be messages
		// themselves (no scalar extensions)
		if fd.Kind() != protoreflect.MessageKind {
			file := r.FileNode()
			info := file.NodeInfo(r.FieldNode(fd.proto).FieldType())
			err := handler.HandleErrorf(info, "messages with message-set wire format cannot contain scalar extensions, only messages")
			if err != nil {
				return err
			}
		}
		if fd.Cardinality() == protoreflect.Repeated {
			file := r.FileNode()
			info := file.NodeInfo(r.FieldNode(fd.proto).FieldLabel())
			err := handler.HandleErrorf(info, "messages with message-set wire format cannot contain repeated extensions, only optional")
			if err != nil {
				return err
			}
		}
	} else if fd.Number() > internal.MaxNormalTag {
		// In validateBasic() we just made sure these were within bounds for any message. But
		// now that things are linked, we can check if the extendee is messageset wire format
		// and, if not, enforce tighter limit.
		file := r.FileNode()
		info := file.NodeInfo(r.FieldNode(fd.proto).FieldTag())
		err := handler.HandleErrorf(info, "tag number %d is higher than max allowed tag number (%d)", fd.Number(), internal.MaxNormalTag)
		if err != nil {
			return err
		}
	}

	fileOpts := r.FileDescriptorProto().GetOptions()
	if fileOpts.GetOptimizeFor() == descriptorpb.FileOptions_LITE_RUNTIME {
		extendeeFileOpts, _ := msg.ParentFile().Options().(*descriptorpb.FileOptions)
		if extendeeFileOpts.GetOptimizeFor() != descriptorpb.FileOptions_LITE_RUNTIME {
			file := r.FileNode()
			info := file.NodeInfo(r.FieldNode(fd.proto))
			err := handler.HandleErrorf(info, "extensions in a file that uses optimize_for=LITE_RUNTIME may not extend messages in file %q which does not", msg.ParentFile().Path())
			if err != nil {
				return err
			}
		}
	}

	// If the extendee uses extension declarations, make sure this extension matches.
	md := protoutil.ProtoFromMessageDescriptor(msg)
	for i, extRange := range md.ExtensionRange {
		if int32(fd.Number()) < extRange.GetStart() || int32(fd.Number()) >= extRange.GetEnd() {
			continue
		}
		extRangeOpts := extRange.GetOptions()
		if extRangeOpts == nil {
			break
		}
		if extRangeOpts.GetVerification() == descriptorpb.ExtensionRangeOptions_UNVERIFIED {
			break
		}
		var found bool
		for j, extDecl := range extRangeOpts.Declaration {
			if extDecl.GetNumber() != int32(fd.Number()) {
				continue
			}
			found = true
			if extDecl.GetReserved() {
				file := r.FileNode()
				info := file.NodeInfo(r.FieldNode(fd.proto).FieldTag())
				span, _ := findExtensionRangeOptionSpan(msg.ParentFile(), msg, i, extRange,
					internal.ExtensionRangeOptionsDeclarationTag, int32(j), internal.ExtensionRangeOptionsDeclarationReservedTag)
				err := handler.HandleErrorf(info, "cannot use field number %d for an extension because it is reserved in declaration at %v",
					fd.Number(), span.Start())
				if err != nil {
					return err
				}
				break
			}
			if extDecl.GetFullName() != "."+string(fd.FullName()) {
				file := r.FileNode()
				info := file.NodeInfo(r.FieldNode(fd.proto).FieldName())
				span, _ := findExtensionRangeOptionSpan(msg.ParentFile(), msg, i, extRange,
					internal.ExtensionRangeOptionsDeclarationTag, int32(j), internal.ExtensionRangeOptionsDeclarationFullNameTag)
				err := handler.HandleErrorf(info, "expected extension with number %d to be named %s, not %s, per declaration at %v",
					fd.Number(), extDecl.GetFullName(), fd.FullName(), span.Start())
				if err != nil {
					return err
				}
			}
			if extDecl.GetType() != getTypeName(fd) {
				file := r.FileNode()
				info := file.NodeInfo(r.FieldNode(fd.proto).FieldType())
				span, _ := findExtensionRangeOptionSpan(msg.ParentFile(), msg, i, extRange,
					internal.ExtensionRangeOptionsDeclarationTag, int32(j), internal.ExtensionRangeOptionsDeclarationTypeTag)
				err := handler.HandleErrorf(info, "expected extension with number %d to have type %s, not %s, per declaration at %v",
					fd.Number(), extDecl.GetType(), getTypeName(fd), span.Start())
				if err != nil {
					return err
				}
			}
			if extDecl.GetRepeated() != (fd.Cardinality() == protoreflect.Repeated) {
				expected, actual := "repeated", "optional"
				if !extDecl.GetRepeated() {
					expected, actual = actual, expected
				}
				file := r.FileNode()
				info := file.NodeInfo(r.FieldNode(fd.proto).FieldLabel())
				span, _ := findExtensionRangeOptionSpan(msg.ParentFile(), msg, i, extRange,
					internal.ExtensionRangeOptionsDeclarationTag, int32(j), internal.ExtensionRangeOptionsDeclarationRepeatedTag)
				err := handler.HandleErrorf(info, "expected extension with number %d to be %s, not %s, per declaration at %v",
					fd.Number(), expected, actual, span.Start())
				if err != nil {
					return err
				}
			}
			break
		}
		if !found {
			file := r.FileNode()
			info := file.NodeInfo(r.FieldNode(fd.proto).FieldTag())
			span, _ := findExtensionRangeOptionSpan(fd.ParentFile(), msg, i, extRange,
				internal.ExtensionRangeOptionsVerificationTag)
			err := handler.HandleErrorf(info, "expected extension with number %d to be declared in type %s, but no declaration found at %v",
				fd.Number(), fd.ContainingMessage().FullName(), span.Start())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *result) validatePacked(fd *fldDescriptor, handler *reporter.Handler) error {
	if fd.proto.Options != nil && fd.proto.Options.Packed != nil && isEditions(r) {
		span := r.findOptionSpan(fd, internal.FieldOptionsPackedTag)
		err := handler.HandleErrorf(span, "packed option cannot be used with editions; use features.repeated_field_encoding=PACKED instead")
		if err != nil {
			return err
		}
	}
	if !fd.proto.GetOptions().GetPacked() {
		// if packed isn't true, nothing to validate
		return nil
	}
	if fd.proto.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
		file := r.FileNode()
		info := file.NodeInfo(r.FieldNode(fd.proto).FieldLabel())
		err := handler.HandleErrorf(info, "packed option is only allowed on repeated fields")
		if err != nil {
			return err
		}
	}
	switch fd.proto.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_TYPE_BYTES,
		descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_TYPE_GROUP:
		file := r.FileNode()
		info := file.NodeInfo(r.FieldNode(fd.proto).FieldType())
		err := handler.HandleErrorf(info, "packed option is only allowed on numeric, boolean, and enum fields")
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *result) validateFieldFeatures(fld *fldDescriptor, handler *reporter.Handler) error {
	if msg, ok := fld.Parent().(*msgDescriptor); ok && msg.proto.GetOptions().GetMapEntry() {
		// Skip validating features on fields of synthetic map entry messages.
		// We blindly propagate them from the map field's features, but some may
		// really only apply to the map field and not to a key or value entry field.
		return nil
	}
	features := fld.proto.GetOptions().GetFeatures()
	if features == nil {
		// No features to validate.
		return nil
	}
	if features.FieldPresence != nil {
		switch {
		case fld.proto.OneofIndex != nil:
			span := r.findOptionSpan(fld, internal.FieldOptionsFeaturesTag, internal.FeatureSetFieldPresenceTag)
			if err := handler.HandleErrorf(span, "oneof fields may not specify field presence"); err != nil {
				return err
			}
		case fld.Cardinality() == protoreflect.Repeated:
			span := r.findOptionSpan(fld, internal.FieldOptionsFeaturesTag, internal.FeatureSetFieldPresenceTag)
			if err := handler.HandleErrorf(span, "repeated fields may not specify field presence"); err != nil {
				return err
			}
		case fld.IsExtension():
			span := r.findOptionSpan(fld, internal.FieldOptionsFeaturesTag, internal.FeatureSetFieldPresenceTag)
			if err := handler.HandleErrorf(span, "extension fields may not specify field presence"); err != nil {
				return err
			}
		case fld.Message() != nil && features.GetFieldPresence() == descriptorpb.FeatureSet_IMPLICIT:
			span := r.findOptionSpan(fld, internal.FieldOptionsFeaturesTag, internal.FeatureSetFieldPresenceTag)
			if err := handler.HandleErrorf(span, "message fields may not specify implicit presence"); err != nil {
				return err
			}
		}
	}
	if features.RepeatedFieldEncoding != nil {
		if fld.Cardinality() != protoreflect.Repeated {
			span := r.findOptionSpan(fld, internal.FieldOptionsFeaturesTag, internal.FeatureSetRepeatedFieldEncodingTag)
			if err := handler.HandleErrorf(span, "only repeated fields may specify repeated field encoding"); err != nil {
				return err
			}
		} else if !internal.CanPack(fld.Kind()) && features.GetRepeatedFieldEncoding() == descriptorpb.FeatureSet_PACKED {
			span := r.findOptionSpan(fld, internal.FieldOptionsFeaturesTag, internal.FeatureSetRepeatedFieldEncodingTag)
			if err := handler.HandleErrorf(span, "only repeated primitive fields may specify packed encoding"); err != nil {
				return err
			}
		}
	}
	if features.Utf8Validation != nil {
		isMap := fld.IsMap()
		if (!isMap && fld.Kind() != protoreflect.StringKind) ||
			(isMap &&
				fld.MapKey().Kind() != protoreflect.StringKind &&
				fld.MapValue().Kind() != protoreflect.StringKind) {
			span := r.findOptionSpan(fld, internal.FieldOptionsFeaturesTag, internal.FeatureSetUTF8ValidationTag)
			if err := handler.HandleErrorf(span, "only string fields may specify UTF8 validation"); err != nil {
				return err
			}
		}
	}
	if features.MessageEncoding != nil {
		if fld.Message() == nil || fld.IsMap() {
			span := r.findOptionSpan(fld, internal.FieldOptionsFeaturesTag, internal.FeatureSetMessageEncodingTag)
			if err := handler.HandleErrorf(span, "only message fields may specify message encoding"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *result) validateMessage(d protoreflect.MessageDescriptor, handler *reporter.Handler, symbols *Symbols) error {
	md, ok := d.(*msgDescriptor)
	if !ok {
		// should not be possible
		return fmt.Errorf("message descriptor is wrong type: expecting %T, got %T", (*msgDescriptor)(nil), d)
	}

	if err := r.validateJSONNamesInMessage(md, handler); err != nil {
		return err
	}

	return r.validateExtensionDeclarations(md, handler, symbols)
}

func (r *result) validateJSONNamesInMessage(md *msgDescriptor, handler *reporter.Handler) error {
	if err := r.validateFieldJSONNames(md, false, handler); err != nil {
		return err
	}
	if err := r.validateFieldJSONNames(md, true, handler); err != nil {
		return err
	}
	return nil
}

func (r *result) validateEnum(d protoreflect.EnumDescriptor, handler *reporter.Handler) error {
	ed, ok := d.(*enumDescriptor)
	if !ok {
		// should not be possible
		return fmt.Errorf("enum descriptor is wrong type: expecting %T, got %T", (*enumDescriptor)(nil), d)
	}

	firstValue := ed.Values().Get(0)
	if !ed.IsClosed() && firstValue.Number() != 0 {
		// TODO: This check doesn't really belong here. Whether the
		//       first value is zero s/b orthogonal to whether the
		//       allowed values are open or closed.
		//       https://github.com/protocolbuffers/protobuf/issues/16249
		file := r.FileNode()
		evd, ok := firstValue.(*enValDescriptor)
		if !ok {
			// should not be possible
			return fmt.Errorf("enum value descriptor is wrong type: expecting %T, got %T", (*enValDescriptor)(nil), firstValue)
		}
		info := file.NodeInfo(r.EnumValueNode(evd.proto).GetNumber())
		if err := handler.HandleErrorf(info, "first value of open enum %s must have numeric value zero", ed.FullName()); err != nil {
			return err
		}
	}

	if err := r.validateJSONNamesInEnum(ed, handler); err != nil {
		return err
	}

	return nil
}

func (r *result) validateJSONNamesInEnum(ed *enumDescriptor, handler *reporter.Handler) error {
	seen := map[string]*descriptorpb.EnumValueDescriptorProto{}
	for _, evd := range ed.proto.GetValue() {
		scope := "enum value " + ed.proto.GetName() + "." + evd.GetName()

		name := canonicalEnumValueName(evd.GetName(), ed.proto.GetName())
		if existing, ok := seen[name]; ok && evd.GetNumber() != existing.GetNumber() {
			fldNode := r.EnumValueNode(evd)
			existingNode := r.EnumValueNode(existing)
			conflictErr := fmt.Errorf("%s: camel-case name (with optional enum name prefix removed) %q conflicts with camel-case name of enum value %s, defined at %v",
				scope, name, existing.GetName(), r.FileNode().NodeInfo(existingNode).Start())

			// Since proto2 did not originally have a JSON format, we report conflicts as just warnings.
			// With editions, not fully supporting JSON is allowed via feature: json_format == BEST_EFFORT
			if !isJSONCompliant(ed) {
				handler.HandleWarningWithPos(r.FileNode().NodeInfo(fldNode), conflictErr)
			} else if err := handler.HandleErrorf(r.FileNode().NodeInfo(fldNode), conflictErr.Error()); err != nil {
				return err
			}
		} else {
			seen[name] = evd
		}
	}
	return nil
}

func (r *result) validateFieldJSONNames(md *msgDescriptor, useCustom bool, handler *reporter.Handler) error {
	type jsonName struct {
		source *descriptorpb.FieldDescriptorProto
		// true if orig is a custom JSON name (vs. the field's default JSON name)
		custom bool
	}
	seen := map[string]jsonName{}

	for _, fd := range md.proto.GetField() {
		scope := "field " + md.proto.GetName() + "." + fd.GetName()
		defaultName := internal.JSONName(fd.GetName())
		name := defaultName
		custom := false
		if useCustom {
			n := fd.GetJsonName()
			if n != defaultName || r.hasCustomJSONName(fd) {
				name = n
				custom = true
			}
		}
		if existing, ok := seen[name]; ok {
			// When useCustom is true, we'll only report an issue when a conflict is
			// due to a custom name. That way, we don't double report conflicts on
			// non-custom names.
			if !useCustom || custom || existing.custom {
				fldNode := r.FieldNode(fd)
				customStr, srcCustomStr := "custom", "custom"
				if !custom {
					customStr = "default"
				}
				if !existing.custom {
					srcCustomStr = "default"
				}
				info := r.FileNode().NodeInfo(fldNode)
				conflictErr := reporter.Errorf(info, "%s: %s JSON name %q conflicts with %s JSON name of field %s, defined at %v",
					scope, customStr, name, srcCustomStr, existing.source.GetName(), r.FileNode().NodeInfo(r.FieldNode(existing.source)).Start())

				// Since proto2 did not originally have default JSON names, we report conflicts
				// between default names (neither is a custom name) as just warnings.
				// With editions, not fully supporting JSON is allowed via feature: json_format == BEST_EFFORT
				if !isJSONCompliant(md) && !custom && !existing.custom {
					handler.HandleWarning(conflictErr)
				} else if err := handler.HandleError(conflictErr); err != nil {
					return err
				}
			}
		} else {
			seen[name] = jsonName{source: fd, custom: custom}
		}
	}
	return nil
}

func (r *result) validateExtensionDeclarations(md *msgDescriptor, handler *reporter.Handler, symbols *Symbols) error {
	for i, extRange := range md.proto.ExtensionRange {
		opts := extRange.GetOptions()
		if len(opts.GetDeclaration()) == 0 {
			// nothing to check
			continue
		}
		if len(opts.GetDeclaration()) > 0 && opts.GetVerification() == descriptorpb.ExtensionRangeOptions_UNVERIFIED {
			span, ok := findExtensionRangeOptionSpan(r, md, i, extRange, internal.ExtensionRangeOptionsVerificationTag)
			if !ok {
				span, _ = findExtensionRangeOptionSpan(r, md, i, extRange, internal.ExtensionRangeOptionsDeclarationTag, 0)
			}
			if err := handler.HandleErrorf(span, "extension range cannot have declarations and have verification of UNVERIFIED"); err != nil {
				return err
			}
		}
		declsByTag := map[int32]ast.SourcePos{}
		for i, extDecl := range extRange.GetOptions().GetDeclaration() {
			if extDecl.Number == nil {
				span, _ := findExtensionRangeOptionSpan(r, md, i, extRange, internal.ExtensionRangeOptionsDeclarationTag, int32(i))
				if err := handler.HandleErrorf(span, "extension declaration is missing required field number"); err != nil {
					return err
				}
			} else {
				extensionNumberSpan, _ := findExtensionRangeOptionSpan(r, md, i, extRange,
					internal.ExtensionRangeOptionsDeclarationTag, int32(i), internal.ExtensionRangeOptionsDeclarationNumberTag)
				if extDecl.GetNumber() < extRange.GetStart() || extDecl.GetNumber() >= extRange.GetEnd() {
					// Number is out of range.
					// See if one of the other ranges on the same extends statement includes the number,
					// so we can provide a helpful message.
					var suffix string
					if extRange, ok := r.ExtensionsNode(extRange).(*ast.ExtensionRangeNode); ok {
						for _, rng := range extRange.Ranges {
							start, _ := rng.StartVal.AsInt64()
							var end int64
							switch {
							case rng.Max != nil:
								end = math.MaxInt64
							case rng.EndVal != nil:
								end, _ = rng.EndVal.AsInt64()
							default:
								end = start
							}
							if int64(extDecl.GetNumber()) >= start && int64(extDecl.GetNumber()) <= end {
								// Found another range that matches
								suffix = "; when using declarations, extends statements should indicate only a single span of field numbers"
								break
							}
						}
					}
					err := handler.HandleErrorf(extensionNumberSpan, "extension declaration has number outside the range: %d not in [%d,%d]%s",
						extDecl.GetNumber(), extRange.GetStart(), extRange.GetEnd()-1, suffix)
					if err != nil {
						return err
					}
				} else {
					// Valid number; make sure it's not a duplicate
					if existing, ok := declsByTag[extDecl.GetNumber()]; ok {
						err := handler.HandleErrorf(extensionNumberSpan, "extension for tag number %d already declared at %v",
							extDecl.GetNumber(), existing)
						if err != nil {
							return err
						}
					} else {
						declsByTag[extDecl.GetNumber()] = extensionNumberSpan.Start()
					}
				}
			}

			if extDecl.GetReserved() {
				if extDecl.FullName != nil {
					span, _ := findExtensionRangeOptionSpan(r, md, i, extRange,
						internal.ExtensionRangeOptionsDeclarationTag, int32(i), internal.ExtensionRangeOptionsDeclarationFullNameTag)
					if err := handler.HandleErrorf(span, "extension declaration is marked reserved so full_name should not be present"); err != nil {
						return err
					}
				}
				if extDecl.Type != nil {
					span, _ := findExtensionRangeOptionSpan(r, md, i, extRange,
						internal.ExtensionRangeOptionsDeclarationTag, int32(i), internal.ExtensionRangeOptionsDeclarationTypeTag)
					if err := handler.HandleErrorf(span, "extension declaration is marked reserved so type should not be present"); err != nil {
						return err
					}
				}
				continue
			}

			if extDecl.FullName == nil {
				span, _ := findExtensionRangeOptionSpan(r, md, i, extRange, internal.ExtensionRangeOptionsDeclarationTag, int32(i))
				if err := handler.HandleErrorf(span, "extension declaration that is not marked reserved must have a full_name"); err != nil {
					return err
				}
			}
			var extensionFullName protoreflect.FullName
			extensionNameSpan, _ := findExtensionRangeOptionSpan(r, md, i, extRange,
				internal.ExtensionRangeOptionsDeclarationTag, int32(i), internal.ExtensionRangeOptionsDeclarationFullNameTag)
			if !strings.HasPrefix(extDecl.GetFullName(), ".") {
				if err := handler.HandleErrorf(extensionNameSpan, "extension declaration full name %q should start with a leading dot (.)", extDecl.GetFullName()); err != nil {
					return err
				}
				extensionFullName = protoreflect.FullName(extDecl.GetFullName())
			} else {
				extensionFullName = protoreflect.FullName(extDecl.GetFullName()[1:])
			}
			if !extensionFullName.IsValid() {
				if err := handler.HandleErrorf(extensionNameSpan, "extension declaration full name %q is not a valid qualified name", extDecl.GetFullName()); err != nil {
					return err
				}
			}
			if err := symbols.AddExtensionDeclaration(extensionFullName, md.FullName(), protoreflect.FieldNumber(extDecl.GetNumber()), extensionNameSpan, handler); err != nil {
				return err
			}

			if extDecl.Type == nil {
				span, _ := findExtensionRangeOptionSpan(r, md, i, extRange, internal.ExtensionRangeOptionsDeclarationTag, int32(i))
				if err := handler.HandleErrorf(span, "extension declaration that is not marked reserved must have a type"); err != nil {
					return err
				}
			}
			if strings.HasPrefix(extDecl.GetType(), ".") {
				if !protoreflect.FullName(extDecl.GetType()[1:]).IsValid() {
					span, _ := findExtensionRangeOptionSpan(r, md, i, extRange,
						internal.ExtensionRangeOptionsDeclarationTag, int32(i), internal.ExtensionRangeOptionsDeclarationTypeTag)
					if err := handler.HandleErrorf(span, "extension declaration type %q is not a valid qualified name", extDecl.GetType()); err != nil {
						return err
					}
				}
			} else if !isBuiltinTypeName(extDecl.GetType()) {
				span, _ := findExtensionRangeOptionSpan(r, md, i, extRange,
					internal.ExtensionRangeOptionsDeclarationTag, int32(i), internal.ExtensionRangeOptionsDeclarationTypeTag)
				if err := handler.HandleErrorf(span, "extension declaration type %q must be a builtin type or start with a leading dot (.)", extDecl.GetType()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (r *result) hasCustomJSONName(fdProto *descriptorpb.FieldDescriptorProto) bool {
	// if we have the AST, we can more precisely determine if there was a custom
	// JSON named defined, even if it is explicitly configured to tbe the same
	// as the default JSON name for the field.
	opts := r.FieldNode(fdProto).GetOptions()
	if opts == nil {
		return false
	}
	for _, opt := range opts.Options {
		if len(opt.Name.Parts) == 1 &&
			opt.Name.Parts[0].Name.AsIdentifier() == "json_name" &&
			!opt.Name.Parts[0].IsExtension() {
			return true
		}
	}
	return false
}

func canonicalEnumValueName(enumValueName, enumName string) string {
	return enumValCamelCase(removePrefix(enumValueName, enumName))
}

// removePrefix is used to remove the given prefix from the given str. It does not require
// an exact match and ignores case and underscores. If the all non-underscore characters
// would be removed from str, str is returned unchanged. If str does not have the given
// prefix (even with the very lenient matching, in regard to case and underscores), then
// str is returned unchanged.
//
// The algorithm is adapted from the protoc source:
//
//	https://github.com/protocolbuffers/protobuf/blob/v21.3/src/google/protobuf/descriptor.cc#L922
func removePrefix(str, prefix string) string {
	j := 0
	for i, r := range str {
		if r == '_' {
			// skip underscores in the input
			continue
		}

		p, sz := utf8.DecodeRuneInString(prefix[j:])
		for p == '_' {
			j += sz // consume/skip underscore
			p, sz = utf8.DecodeRuneInString(prefix[j:])
		}

		if j == len(prefix) {
			// matched entire prefix; return rest of str
			// but skipping any leading underscores
			result := strings.TrimLeft(str[i:], "_")
			if len(result) == 0 {
				// result can't be empty string
				return str
			}
			return result
		}
		if unicode.ToLower(r) != unicode.ToLower(p) {
			// does not match prefix
			return str
		}
		j += sz // consume matched rune of prefix
	}
	return str
}

// enumValCamelCase converts the given string to upper-camel-case.
//
// The algorithm is adapted from the protoc source:
//
//	https://github.com/protocolbuffers/protobuf/blob/v21.3/src/google/protobuf/descriptor.cc#L887
func enumValCamelCase(name string) string {
	var js []rune
	nextUpper := true
	for _, r := range name {
		if r == '_' {
			nextUpper = true
			continue
		}
		if nextUpper {
			nextUpper = false
			js = append(js, unicode.ToUpper(r))
		} else {
			js = append(js, unicode.ToLower(r))
		}
	}
	return string(js)
}

func isBuiltinTypeName(typeName string) bool {
	switch typeName {
	case "int32", "int64", "uint32", "uint64", "sint32", "sint64",
		"fixed32", "fixed64", "sfixed32", "sfixed64",
		"bool", "double", "float", "string", "bytes":
		return true
	default:
		return false
	}
}

func getTypeName(fd protoreflect.FieldDescriptor) string {
	switch fd.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return "." + string(fd.Message().FullName())
	case protoreflect.EnumKind:
		return "." + string(fd.Enum().FullName())
	default:
		return fd.Kind().String()
	}
}

func findExtensionRangeOptionSpan(
	file protoreflect.FileDescriptor,
	extended protoreflect.MessageDescriptor,
	extRangeIndex int,
	extRange *descriptorpb.DescriptorProto_ExtensionRange,
	path ...int32,
) (ast.SourceSpan, bool) {
	// NB: Typically, we have an AST for a file and NOT source code info, because the
	// compiler validates options before computing source code info. However, we might
	// be validating an extension (whose source/AST we have), but whose extendee (and
	// thus extension range options for declarations) could be in some other file, which
	// could be provided to the compiler as an already-compiled descriptor. So this
	// function can fallback to using source code info if an AST is not available.

	if r, ok := file.(Result); ok && r.AST() != nil {
		// Find the location using the AST, which will generally be higher fidelity
		// than what we might find in a file descriptor's source code info.
		exts := r.ExtensionsNode(extRange)
		return findOptionSpan(r.FileNode(), exts, extRange.Options.ProtoReflect().Descriptor(), path...)
	}

	srcLocs := file.SourceLocations()
	if srcLocs.Len() == 0 {
		// no source code info, can't do any better than the filename. We
		// return true as the boolean so the caller doesn't try again with
		// an alternate path, since we won't be able to do any better.
		return ast.UnknownSpan(file.Path()), true
	}
	msgPath, ok := internal.ComputePath(extended)
	if !ok {
		// Same as above: return true since no subsequent query can do better.
		return ast.UnknownSpan(file.Path()), true
	}

	//nolint:gocritic // intentionally assigning to different slice variables
	extRangePath := append(msgPath, internal.MessageExtensionRangesTag, int32(extRangeIndex))
	optsPath := append(extRangePath, internal.ExtensionRangeOptionsTag) //nolint:gocritic
	fullPath := append(optsPath, path...)                               //nolint:gocritic
	srcLoc := srcLocs.ByPath(fullPath)
	if srcLoc.Path != nil {
		// found it
		return asSpan(file.Path(), srcLoc), true
	}

	// Slow path to find closest match :/
	// We look for longest matching path that is at least len(extRangePath)
	// long. If we find a path that is longer (meaning a path that points INSIDE
	// the request element), accept the first such location.
	var bestMatch protoreflect.SourceLocation
	var bestMatchPathLen int
	for i, length := 0, srcLocs.Len(); i < length; i++ {
		srcLoc := srcLocs.Get(i)
		if len(srcLoc.Path) >= len(extRangePath) &&
			isDescendantPath(fullPath, srcLoc.Path) &&
			len(srcLoc.Path) > bestMatchPathLen {
			bestMatch = srcLoc
			bestMatchPathLen = len(srcLoc.Path)
		} else if isDescendantPath(srcLoc.Path, path) {
			return asSpan(file.Path(), srcLoc), false
		}
	}
	if bestMatchPathLen > 0 {
		return asSpan(file.Path(), bestMatch), false
	}
	return ast.UnknownSpan(file.Path()), false
}

func (r *result) findScalarOptionSpan(
	root ast.NodeWithOptions,
	name string,
) ast.SourceSpan {
	match := ast.Node(root)
	root.RangeOptions(func(n *ast.OptionNode) bool {
		if len(n.Name.Parts) == 1 && !n.Name.Parts[0].IsExtension() &&
			string(n.Name.Parts[0].Name.AsIdentifier()) == name {
			match = n
			return false
		}
		return true
	})
	return r.FileNode().NodeInfo(match)
}

func (r *result) findOptionSpan(
	d protoutil.DescriptorProtoWrapper,
	path ...int32,
) ast.SourceSpan {
	node := r.Node(d.AsProto())
	nodeWithOpts, ok := node.(ast.NodeWithOptions)
	if !ok {
		return r.FileNode().NodeInfo(node)
	}
	span, _ := findOptionSpan(r.FileNode(), nodeWithOpts, d.Options().ProtoReflect().Descriptor(), path...)
	return span
}

func findOptionSpan(
	file ast.FileDeclNode,
	root ast.NodeWithOptions,
	md protoreflect.MessageDescriptor,
	path ...int32,
) (ast.SourceSpan, bool) {
	bestMatch := ast.Node(root)
	var bestMatchLen int
	var repeatedIndices []int
	root.RangeOptions(func(n *ast.OptionNode) bool {
		desc := md
		limit := len(n.Name.Parts)
		if limit > len(path) {
			limit = len(path)
		}
		var nextIsIndex bool
		for i := 0; i < limit; i++ {
			if desc == nil || nextIsIndex {
				// Can't match anymore. Try next option.
				return true
			}
			wantField := desc.Fields().ByNumber(protoreflect.FieldNumber(path[i]))
			if wantField == nil {
				// Should not be possible... next option won't fare any better since
				// it's a disagreement between given path and given descriptor so bail.
				return false
			}
			if n.Name.Parts[i].Open != nil ||
				string(n.Name.Parts[i].Name.AsIdentifier()) != string(wantField.Name()) {
				// This is an extension/custom option or indicates the wrong name.
				// Try the next one.
				return true
			}
			desc = wantField.Message()
			nextIsIndex = wantField.Cardinality() == protoreflect.Repeated
		}
		// If we made it this far, we've matched everything so far.
		if len(n.Name.Parts) >= len(path) {
			// Either an exact match (if equal) or this option points *inside* the
			// item we care about (if greater). Either way, the first such result
			// is a keeper.
			bestMatch = n.Name.Parts[len(path)-1]
			bestMatchLen = len(n.Name.Parts)
			return false
		}
		// We've got more path elements to try to match with the value.
		match, matchLen := findMatchingValueNode(
			desc,
			path[len(n.Name.Parts):],
			nextIsIndex,
			0,
			&repeatedIndices,
			n,
			n.Val)
		if match != nil {
			totalMatchLen := matchLen + len(n.Name.Parts)
			if totalMatchLen > bestMatchLen {
				bestMatch, bestMatchLen = match, totalMatchLen
			}
		}
		return bestMatchLen != len(path) // no exact match, so keep looking
	})
	return file.NodeInfo(bestMatch), bestMatchLen == len(path)
}

func findMatchingValueNode(
	md protoreflect.MessageDescriptor,
	path protoreflect.SourcePath,
	currIsRepeated bool,
	repeatedCount int,
	repeatedIndices *[]int,
	node ast.Node,
	val ast.ValueNode,
) (ast.Node, int) {
	var matchLen int
	var index int
	if currIsRepeated {
		// Compute the index of the current value (or, if an array literal, the
		// index of the first value in the array).
		if len(*repeatedIndices) > repeatedCount {
			(*repeatedIndices)[repeatedCount]++
			index = (*repeatedIndices)[repeatedCount]
		} else {
			*repeatedIndices = append(*repeatedIndices, 0)
			index = 0
		}
		repeatedCount++
	}

	if arrayVal, ok := val.(*ast.ArrayLiteralNode); ok {
		if !currIsRepeated {
			// This should not happen.
			return nil, 0
		}
		offset := int(path[0]) - index
		if offset >= len(arrayVal.Elements) {
			// The index we are looking for is not in this array.
			return nil, 0
		}
		elem := arrayVal.Elements[offset]
		// We've matched the index!
		matchLen++
		path = path[1:]
		// Recurse into array element.
		nextMatch, nextMatchLen := findMatchingValueNode(
			md,
			path,
			false,
			repeatedCount,
			repeatedIndices,
			elem,
			elem,
		)
		return nextMatch, nextMatchLen + matchLen
	}

	if currIsRepeated {
		if index != int(path[0]) {
			// Not a match!
			return nil, 0
		}
		// We've matched the index!
		matchLen++
		path = path[1:]
		if len(path) == 0 {
			// We're done matching!
			return node, matchLen
		}
	}

	msgValue, ok := val.(*ast.MessageLiteralNode)
	if !ok {
		// We can't go any further
		return node, matchLen
	}

	var wantField protoreflect.FieldDescriptor
	if md != nil {
		wantField = md.Fields().ByNumber(protoreflect.FieldNumber(path[0]))
	}
	if wantField == nil {
		// Should not be possible... next option won't fare any better since
		// it's a disagreement between given path and given descriptor so bail.
		return nil, 0
	}
	for _, field := range msgValue.Elements {
		if field.Name.Open != nil ||
			string(field.Name.Name.AsIdentifier()) != string(wantField.Name()) {
			// This is an extension/custom option or indicates the wrong name.
			// Try the next one.
			continue
		}
		// We've matched this field.
		matchLen++
		path = path[1:]
		if len(path) == 0 {
			// Perfect match!
			return field, matchLen
		}
		nextMatch, nextMatchLen := findMatchingValueNode(
			wantField.Message(),
			path,
			wantField.Cardinality() == protoreflect.Repeated,
			repeatedCount,
			repeatedIndices,
			field,
			field.Val,
		)
		return nextMatch, nextMatchLen + matchLen
	}

	// If we didn't find the right field, just return what we have so far.
	return node, matchLen
}

func isDescendantPath(descendant, ancestor protoreflect.SourcePath) bool {
	if len(descendant) < len(ancestor) {
		return false
	}
	for i := range ancestor {
		if descendant[i] != ancestor[i] {
			return false
		}
	}
	return true
}

func asSpan(file string, srcLoc protoreflect.SourceLocation) ast.SourceSpan {
	return ast.NewSourceSpan(
		ast.SourcePos{
			Filename: file,
			Line:     srcLoc.StartLine + 1,
			Col:      srcLoc.StartColumn + 1,
		},
		ast.SourcePos{
			Filename: file,
			Line:     srcLoc.EndLine + 1,
			Col:      srcLoc.EndColumn + 1,
		},
	)
}

func (r *result) getImportLocation(path string) ast.SourceSpan {
	node, ok := r.FileNode().(*ast.FileNode)
	if !ok {
		return ast.UnknownSpan(path)
	}
	for _, decl := range node.Decls {
		imp, ok := decl.(*ast.ImportNode)
		if !ok {
			continue
		}
		if imp.Name.AsString() == path {
			return node.NodeInfo(imp.Name)
		}
	}
	// Couldn't find it? Should never happen...
	return ast.UnknownSpan(path)
}

func isEditions(r *result) bool {
	return descriptorpb.Edition(r.Edition()) >= descriptorpb.Edition_EDITION_2023
}
