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
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/internal"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/bufbuild/protocompile/walk"
)

func (r *result) ResolveMessageLiteralExtensionName(node ast.IdentValueNode) string {
	return r.optionQualifiedNames[node]
}

func (r *result) resolveElement(name protoreflect.FullName) protoreflect.Descriptor {
	if len(name) > 0 && name[0] == '.' {
		name = name[1:]
	}
	res, _ := resolveInFile(r, false, nil, func(f File) (protoreflect.Descriptor, error) {
		d := resolveElementInFile(name, f)
		if d != nil {
			return d, nil
		}
		return nil, protoregistry.NotFound
	})
	return res
}

func resolveInFile[T any](f File, publicImportsOnly bool, checked []string, fn func(File) (T, error)) (T, error) {
	var zero T
	path := f.Path()
	for _, str := range checked {
		if str == path {
			// already checked
			return zero, protoregistry.NotFound
		}
	}
	checked = append(checked, path)

	res, err := fn(f)
	if err == nil {
		// found it
		return res, nil
	}
	if !errors.Is(err, protoregistry.NotFound) {
		return zero, err
	}

	imports := f.Imports()
	for i, l := 0, imports.Len(); i < l; i++ {
		imp := imports.Get(i)
		if publicImportsOnly && !imp.IsPublic {
			continue
		}
		res, err := resolveInFile(f.FindImportByPath(imp.Path()), true, checked, fn)
		if errors.Is(err, protoregistry.NotFound) {
			continue
		}
		if err != nil {
			return zero, err
		}
		if !imp.IsPublic {
			if r, ok := f.(*result); ok {
				r.markUsed(imp.Path())
			}
		}
		return res, nil
	}
	return zero, err
}

func (r *result) markUsed(importPath string) {
	r.usedImports[importPath] = struct{}{}
}

func (r *result) CheckForUnusedImports(handler *reporter.Handler) {
	fd := r.FileDescriptorProto()
	file, _ := r.FileNode().(*ast.FileNode)
	for i, dep := range fd.Dependency {
		if _, ok := r.usedImports[dep]; !ok {
			isPublic := false
			// it's fine if it's a public import
			for _, j := range fd.PublicDependency {
				if i == int(j) {
					isPublic = true
					break
				}
			}
			if isPublic {
				continue
			}
			span := ast.UnknownSpan(fd.GetName())
			if file != nil {
				for _, decl := range file.Decls {
					imp, ok := decl.(*ast.ImportNode)
					if ok && imp.Name.AsString() == dep {
						span = file.NodeInfo(imp)
					}
				}
			}
			handler.HandleWarningWithPos(span, errUnusedImport(dep))
		}
	}
}

func descriptorTypeWithArticle(d protoreflect.Descriptor) string {
	switch d := d.(type) {
	case protoreflect.MessageDescriptor:
		return "a message"
	case protoreflect.FieldDescriptor:
		if d.IsExtension() {
			return "an extension"
		}
		return "a field"
	case protoreflect.OneofDescriptor:
		return "a oneof"
	case protoreflect.EnumDescriptor:
		return "an enum"
	case protoreflect.EnumValueDescriptor:
		return "an enum value"
	case protoreflect.ServiceDescriptor:
		return "a service"
	case protoreflect.MethodDescriptor:
		return "a method"
	case protoreflect.FileDescriptor:
		return "a file"
	default:
		// shouldn't be possible
		return fmt.Sprintf("a %T", d)
	}
}

func (r *result) resolveReferences(handler *reporter.Handler, s *Symbols) error {
	// first create the full descriptor hierarchy
	fd := r.FileDescriptorProto()
	prefix := ""
	if fd.GetPackage() != "" {
		prefix = fd.GetPackage() + "."
	}
	r.imports = r.createImports()
	r.messages = r.createMessages(prefix, r, fd.MessageType)
	r.enums = r.createEnums(prefix, r, fd.EnumType)
	r.extensions = r.createExtensions(prefix, r, fd.Extension)
	r.services = r.createServices(prefix, fd.Service)

	// then resolve symbol references
	scopes := []scope{fileScope(r)}
	if fd.Options != nil {
		if err := r.resolveOptions(handler, "file", protoreflect.FullName(fd.GetName()), fd.Options.UninterpretedOption, scopes); err != nil {
			return err
		}
	}

	return walk.DescriptorsEnterAndExit(r,
		func(d protoreflect.Descriptor) error {
			fqn := d.FullName()
			switch d := d.(type) {
			case *msgDescriptor:
				// Strangely, when protoc resolves extension names, it uses the *enclosing* scope
				// instead of the message's scope. So if the message contains an extension named "i",
				// an option cannot refer to it as simply "i" but must qualify it (at a minimum "Msg.i").
				// So we don't add this messages scope to our scopes slice until *after* we do options.
				if d.proto.Options != nil {
					if err := r.resolveOptions(handler, "message", fqn, d.proto.Options.UninterpretedOption, scopes); err != nil {
						return err
					}
				}
				scopes = append(scopes, messageScope(r, fqn)) // push new scope on entry
				// walk only visits descriptors, so we need to loop over extension ranges ourselves
				for _, er := range d.proto.ExtensionRange {
					if er.Options != nil {
						erName := protoreflect.FullName(fmt.Sprintf("%s:%d-%d", fqn, er.GetStart(), er.GetEnd()-1))
						if err := r.resolveOptions(handler, "extension range", erName, er.Options.UninterpretedOption, scopes); err != nil {
							return err
						}
					}
				}
			case *extTypeDescriptor:
				if d.field.proto.Options != nil {
					if err := r.resolveOptions(handler, "extension", fqn, d.field.proto.Options.UninterpretedOption, scopes); err != nil {
						return err
					}
				}
				if err := resolveFieldTypes(d.field, handler, s, scopes); err != nil {
					return err
				}
				if r.Syntax() == protoreflect.Proto3 && !allowedProto3Extendee(d.field.proto.GetExtendee()) {
					file := r.FileNode()
					node := r.FieldNode(d.field.proto).FieldExtendee()
					if err := handler.HandleErrorf(file.NodeInfo(node), "extend blocks in proto3 can only be used to define custom options"); err != nil {
						return err
					}
				}
			case *fldDescriptor:
				if d.proto.Options != nil {
					if err := r.resolveOptions(handler, "field", fqn, d.proto.Options.UninterpretedOption, scopes); err != nil {
						return err
					}
				}
				if err := resolveFieldTypes(d, handler, s, scopes); err != nil {
					return err
				}
			case *oneofDescriptor:
				if d.proto.Options != nil {
					if err := r.resolveOptions(handler, "oneof", fqn, d.proto.Options.UninterpretedOption, scopes); err != nil {
						return err
					}
				}
			case *enumDescriptor:
				if d.proto.Options != nil {
					if err := r.resolveOptions(handler, "enum", fqn, d.proto.Options.UninterpretedOption, scopes); err != nil {
						return err
					}
				}
			case *enValDescriptor:
				if d.proto.Options != nil {
					if err := r.resolveOptions(handler, "enum value", fqn, d.proto.Options.UninterpretedOption, scopes); err != nil {
						return err
					}
				}
			case *svcDescriptor:
				if d.proto.Options != nil {
					if err := r.resolveOptions(handler, "service", fqn, d.proto.Options.UninterpretedOption, scopes); err != nil {
						return err
					}
				}
				// not a message, but same scoping rules for nested elements as if it were
				scopes = append(scopes, messageScope(r, fqn)) // push new scope on entry
			case *mtdDescriptor:
				if d.proto.Options != nil {
					if err := r.resolveOptions(handler, "method", fqn, d.proto.Options.UninterpretedOption, scopes); err != nil {
						return err
					}
				}
				if err := resolveMethodTypes(d, handler, scopes); err != nil {
					return err
				}
			}
			return nil
		},
		func(d protoreflect.Descriptor) error {
			switch d.(type) {
			case protoreflect.MessageDescriptor, protoreflect.ServiceDescriptor:
				// pop message scope on exit
				scopes = scopes[:len(scopes)-1]
			}
			return nil
		})
}

var allowedProto3Extendees = map[string]struct{}{
	".google.protobuf.FileOptions":           {},
	".google.protobuf.MessageOptions":        {},
	".google.protobuf.FieldOptions":          {},
	".google.protobuf.OneofOptions":          {},
	".google.protobuf.ExtensionRangeOptions": {},
	".google.protobuf.EnumOptions":           {},
	".google.protobuf.EnumValueOptions":      {},
	".google.protobuf.ServiceOptions":        {},
	".google.protobuf.MethodOptions":         {},
}

func allowedProto3Extendee(n string) bool {
	if n == "" {
		// not an extension, allowed
		return true
	}
	_, ok := allowedProto3Extendees[n]
	return ok
}

func resolveFieldTypes(f *fldDescriptor, handler *reporter.Handler, s *Symbols, scopes []scope) error {
	r := f.file
	fld := f.proto
	file := r.FileNode()
	node := r.FieldNode(fld)
	scope := fmt.Sprintf("field %s", f.fqn)
	if fld.GetExtendee() != "" {
		scope := fmt.Sprintf("extension %s", f.fqn)
		dsc := r.resolve(fld.GetExtendee(), false, scopes)
		if dsc == nil {
			return handler.HandleErrorf(file.NodeInfo(node.FieldExtendee()), "unknown extendee type %s", fld.GetExtendee())
		}
		if isSentinelDescriptor(dsc) {
			return handler.HandleErrorf(file.NodeInfo(node.FieldExtendee()), "unknown extendee type %s; resolved to %s which is not defined; consider using a leading dot", fld.GetExtendee(), dsc.FullName())
		}
		extd, ok := dsc.(protoreflect.MessageDescriptor)
		if !ok {
			return handler.HandleErrorf(file.NodeInfo(node.FieldExtendee()), "extendee is invalid: %s is %s, not a message", dsc.FullName(), descriptorTypeWithArticle(dsc))
		}
		f.extendee = extd
		extendeeName := "." + string(dsc.FullName())
		if fld.GetExtendee() != extendeeName {
			fld.Extendee = proto.String(extendeeName)
		}
		// make sure the tag number is in range
		found := false
		tag := protoreflect.FieldNumber(fld.GetNumber())
		for i := 0; i < extd.ExtensionRanges().Len(); i++ {
			rng := extd.ExtensionRanges().Get(i)
			if tag >= rng[0] && tag < rng[1] {
				found = true
				break
			}
		}
		if !found {
			if err := handler.HandleErrorf(file.NodeInfo(node.FieldTag()), "%s: tag %d is not in valid range for extended type %s", scope, tag, dsc.FullName()); err != nil {
				return err
			}
		} else {
			// make sure tag is not a duplicate
			if err := s.AddExtension(packageFor(dsc), dsc.FullName(), tag, file.NodeInfo(node.FieldTag()), handler); err != nil {
				return err
			}
		}
	} else if f.proto.OneofIndex != nil {
		parent := f.parent.(protoreflect.MessageDescriptor) //nolint:errcheck
		index := int(f.proto.GetOneofIndex())
		f.oneof = parent.Oneofs().Get(index)
	}

	if fld.GetTypeName() == "" {
		// scalar type; no further resolution required
		return nil
	}

	dsc := r.resolve(fld.GetTypeName(), true, scopes)
	if dsc == nil {
		return handler.HandleErrorf(file.NodeInfo(node.FieldType()), "%s: unknown type %s", scope, fld.GetTypeName())
	}
	if isSentinelDescriptor(dsc) {
		return handler.HandleErrorf(file.NodeInfo(node.FieldType()), "%s: unknown type %s; resolved to %s which is not defined; consider using a leading dot", scope, fld.GetTypeName(), dsc.FullName())
	}
	switch dsc := dsc.(type) {
	case protoreflect.MessageDescriptor:
		if dsc.IsMapEntry() {
			isValid := false
			switch node.(type) {
			case *ast.MapFieldNode:
				// We have an AST for this file and can see this field is from a map declaration
				isValid = true
			case ast.NoSourceNode:
				// We don't have an AST for the file (it came from a provided descriptor). So we
				// need to validate that it's not an illegal reference. To be valid, the field
				// must be repeated and the entry type must be nested in the same enclosing
				// message as the field.
				isValid = isValidMap(f, dsc)
				if isValid && f.index > 0 {
					// also make sure there are no earlier fields that are valid for this map entry
					flds := f.Parent().(protoreflect.MessageDescriptor).Fields()
					for i := 0; i < f.index; i++ {
						if isValidMap(flds.Get(i), dsc) {
							isValid = false
							break
						}
					}
				}
			}
			if !isValid {
				return handler.HandleErrorf(file.NodeInfo(node.FieldType()), "%s: %s is a synthetic map entry and may not be referenced explicitly", scope, dsc.FullName())
			}
		}
		typeName := "." + string(dsc.FullName())
		if fld.GetTypeName() != typeName {
			fld.TypeName = proto.String(typeName)
		}
		if fld.Type == nil {
			// if type was tentatively unset, we now know it's actually a message
			fld.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		} else if fld.GetType() != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE && fld.GetType() != descriptorpb.FieldDescriptorProto_TYPE_GROUP {
			return handler.HandleErrorf(file.NodeInfo(node.FieldType()), "%s: descriptor proto indicates type %v but should be %v", scope, fld.GetType(), descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
		}
		f.msgType = dsc
	case protoreflect.EnumDescriptor:
		typeName := "." + string(dsc.FullName())
		if fld.GetTypeName() != typeName {
			fld.TypeName = proto.String(typeName)
		}
		if fld.Type == nil {
			// the type was tentatively unset, but now we know it's actually an enum
			fld.Type = descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum()
		} else if fld.GetType() != descriptorpb.FieldDescriptorProto_TYPE_ENUM {
			return handler.HandleErrorf(file.NodeInfo(node.FieldType()), "%s: descriptor proto indicates type %v but should be %v", scope, fld.GetType(), descriptorpb.FieldDescriptorProto_TYPE_ENUM)
		}
		f.enumType = dsc
	default:
		return handler.HandleErrorf(file.NodeInfo(node.FieldType()), "%s: invalid type: %s is %s, not a message or enum", scope, dsc.FullName(), descriptorTypeWithArticle(dsc))
	}
	return nil
}

func packageFor(dsc protoreflect.Descriptor) protoreflect.FullName {
	if dsc.ParentFile() != nil {
		return dsc.ParentFile().Package()
	}
	// Can't access package? Make a best effort guess.
	return dsc.FullName().Parent()
}

func isValidMap(mapField protoreflect.FieldDescriptor, mapEntry protoreflect.MessageDescriptor) bool {
	return !mapField.IsExtension() &&
		mapEntry.Parent() == mapField.ContainingMessage() &&
		mapField.Cardinality() == protoreflect.Repeated &&
		string(mapEntry.Name()) == internal.InitCap(internal.JSONName(string(mapField.Name())))+"Entry"
}

func resolveMethodTypes(m *mtdDescriptor, handler *reporter.Handler, scopes []scope) error {
	scope := fmt.Sprintf("method %s", m.fqn)
	r := m.file
	mtd := m.proto
	file := r.FileNode()
	node := r.MethodNode(mtd)
	dsc := r.resolve(mtd.GetInputType(), false, scopes)
	if dsc == nil {
		if err := handler.HandleErrorf(file.NodeInfo(node.GetInputType()), "%s: unknown request type %s", scope, mtd.GetInputType()); err != nil {
			return err
		}
	} else if isSentinelDescriptor(dsc) {
		if err := handler.HandleErrorf(file.NodeInfo(node.GetInputType()), "%s: unknown request type %s; resolved to %s which is not defined; consider using a leading dot", scope, mtd.GetInputType(), dsc.FullName()); err != nil {
			return err
		}
	} else if msg, ok := dsc.(protoreflect.MessageDescriptor); !ok {
		if err := handler.HandleErrorf(file.NodeInfo(node.GetInputType()), "%s: invalid request type: %s is %s, not a message", scope, dsc.FullName(), descriptorTypeWithArticle(dsc)); err != nil {
			return err
		}
	} else {
		typeName := "." + string(dsc.FullName())
		if mtd.GetInputType() != typeName {
			mtd.InputType = proto.String(typeName)
		}
		m.inputType = msg
	}

	// TODO: make input and output type resolution more DRY
	dsc = r.resolve(mtd.GetOutputType(), false, scopes)
	if dsc == nil {
		if err := handler.HandleErrorf(file.NodeInfo(node.GetOutputType()), "%s: unknown response type %s", scope, mtd.GetOutputType()); err != nil {
			return err
		}
	} else if isSentinelDescriptor(dsc) {
		if err := handler.HandleErrorf(file.NodeInfo(node.GetOutputType()), "%s: unknown response type %s; resolved to %s which is not defined; consider using a leading dot", scope, mtd.GetOutputType(), dsc.FullName()); err != nil {
			return err
		}
	} else if msg, ok := dsc.(protoreflect.MessageDescriptor); !ok {
		if err := handler.HandleErrorf(file.NodeInfo(node.GetOutputType()), "%s: invalid response type: %s is %s, not a message", scope, dsc.FullName(), descriptorTypeWithArticle(dsc)); err != nil {
			return err
		}
	} else {
		typeName := "." + string(dsc.FullName())
		if mtd.GetOutputType() != typeName {
			mtd.OutputType = proto.String(typeName)
		}
		m.outputType = msg
	}

	return nil
}

func (r *result) resolveOptions(handler *reporter.Handler, elemType string, elemName protoreflect.FullName, opts []*descriptorpb.UninterpretedOption, scopes []scope) error {
	mc := &internal.MessageContext{
		File:        r,
		ElementName: string(elemName),
		ElementType: elemType,
	}
	file := r.FileNode()
opts:
	for _, opt := range opts {
		// resolve any extension names found in option names
		for _, nm := range opt.Name {
			if nm.GetIsExtension() {
				node := r.OptionNamePartNode(nm)
				fqn, err := r.resolveExtensionName(nm.GetNamePart(), scopes)
				if err != nil {
					if err := handler.HandleErrorf(file.NodeInfo(node), "%v%v", mc, err); err != nil {
						return err
					}
					continue opts
				}
				nm.NamePart = proto.String(fqn)
			}
		}
		// also resolve any extension names found inside message literals in option values
		mc.Option = opt
		optVal := r.OptionNode(opt).GetValue()
		if err := r.resolveOptionValue(handler, mc, optVal, scopes); err != nil {
			return err
		}
		mc.Option = nil
	}
	return nil
}

func (r *result) resolveOptionValue(handler *reporter.Handler, mc *internal.MessageContext, val ast.ValueNode, scopes []scope) error {
	optVal := val.Value()
	switch optVal := optVal.(type) {
	case []ast.ValueNode:
		origPath := mc.OptAggPath
		defer func() {
			mc.OptAggPath = origPath
		}()
		for i, v := range optVal {
			mc.OptAggPath = fmt.Sprintf("%s[%d]", origPath, i)
			if err := r.resolveOptionValue(handler, mc, v, scopes); err != nil {
				return err
			}
		}
	case []*ast.MessageFieldNode:
		origPath := mc.OptAggPath
		defer func() {
			mc.OptAggPath = origPath
		}()
		for _, fld := range optVal {
			// check for extension name
			if fld.Name.IsExtension() {
				// Confusingly, an extension reference inside a message literal cannot refer to
				// elements in the same enclosing message without a qualifier. Basically, we
				// treat this as if there were no message scopes, so only the package name is
				// used for resolving relative references. (Inconsistent protoc behavior, but
				// likely due to how it re-uses C++ text format implementation, and normal text
				// format doesn't expect that kind of relative reference.)
				scopes := scopes[:1] // first scope is file, the rest are enclosing messages
				fqn, err := r.resolveExtensionName(string(fld.Name.Name.AsIdentifier()), scopes)
				if err != nil {
					if err := handler.HandleErrorf(r.FileNode().NodeInfo(fld.Name.Name), "%v%v", mc, err); err != nil {
						return err
					}
				} else {
					r.optionQualifiedNames[fld.Name.Name] = fqn
				}
			}

			// recurse into value
			mc.OptAggPath = origPath
			if origPath != "" {
				mc.OptAggPath += "."
			}
			if fld.Name.IsExtension() {
				mc.OptAggPath = fmt.Sprintf("%s[%s]", mc.OptAggPath, string(fld.Name.Name.AsIdentifier()))
			} else {
				mc.OptAggPath = fmt.Sprintf("%s%s", mc.OptAggPath, string(fld.Name.Name.AsIdentifier()))
			}

			if err := r.resolveOptionValue(handler, mc, fld.Val, scopes); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *result) resolveExtensionName(name string, scopes []scope) (string, error) {
	dsc := r.resolve(name, false, scopes)
	if dsc == nil {
		return "", fmt.Errorf("unknown extension %s", name)
	}
	if isSentinelDescriptor(dsc) {
		return "", fmt.Errorf("unknown extension %s; resolved to %s which is not defined; consider using a leading dot", name, dsc.FullName())
	}
	if ext, ok := dsc.(protoreflect.FieldDescriptor); !ok {
		return "", fmt.Errorf("invalid extension: %s is %s, not an extension", name, descriptorTypeWithArticle(dsc))
	} else if !ext.IsExtension() {
		return "", fmt.Errorf("invalid extension: %s is a field but not an extension", name)
	}
	return string("." + dsc.FullName()), nil
}

func (r *result) resolve(name string, onlyTypes bool, scopes []scope) protoreflect.Descriptor {
	if strings.HasPrefix(name, ".") {
		// already fully-qualified
		return r.resolveElement(protoreflect.FullName(name[1:]))
	}
	// unqualified, so we look in the enclosing (last) scope first and move
	// towards outermost (first) scope, trying to resolve the symbol
	pos := strings.IndexByte(name, '.')
	firstName := name
	if pos > 0 {
		firstName = name[:pos]
	}
	var bestGuess protoreflect.Descriptor
	for i := len(scopes) - 1; i >= 0; i-- {
		d := scopes[i](firstName, name)
		if d != nil {
			// In `protoc`, it will skip a match of the wrong type and move on
			// to the next scope, but only if the reference is unqualified. So
			// we mirror that behavior here. When we skip and move on, we go
			// ahead and save the match of the wrong type so we can at least use
			// it to construct a better error in the event that we don't find
			// any match of the right type.
			if !onlyTypes || isType(d) || firstName != name {
				return d
			}
			if bestGuess == nil {
				bestGuess = d
			}
		}
	}
	// we return best guess, even though it was not an allowed kind of
	// descriptor, so caller can print a better error message (e.g.
	// indicating that the name was found but that it's the wrong type)
	return bestGuess
}

func isType(d protoreflect.Descriptor) bool {
	switch d.(type) {
	case protoreflect.MessageDescriptor, protoreflect.EnumDescriptor:
		return true
	}
	return false
}

// scope represents a lexical scope in a proto file in which messages and enums
// can be declared.
type scope func(firstName, fullName string) protoreflect.Descriptor

func fileScope(r *result) scope {
	// we search symbols in this file, but also symbols in other files that have
	// the same package as this file or a "parent" package (in protobuf,
	// packages are a hierarchy like C++ namespaces)
	prefixes := internal.CreatePrefixList(r.FileDescriptorProto().GetPackage())
	querySymbol := func(n string) protoreflect.Descriptor {
		return r.resolveElement(protoreflect.FullName(n))
	}
	return func(firstName, fullName string) protoreflect.Descriptor {
		for _, prefix := range prefixes {
			var n1, n string
			if prefix == "" {
				// exhausted all prefixes, so it must be in this one
				n1, n = fullName, fullName
			} else {
				n = prefix + "." + fullName
				n1 = prefix + "." + firstName
			}
			d := resolveElementRelative(n1, n, querySymbol)
			if d != nil {
				return d
			}
		}
		return nil
	}
}

func messageScope(r *result, messageName protoreflect.FullName) scope {
	querySymbol := func(n string) protoreflect.Descriptor {
		return resolveElementInFile(protoreflect.FullName(n), r)
	}
	return func(firstName, fullName string) protoreflect.Descriptor {
		n1 := string(messageName) + "." + firstName
		n := string(messageName) + "." + fullName
		return resolveElementRelative(n1, n, querySymbol)
	}
}

func resolveElementRelative(firstName, fullName string, query func(name string) protoreflect.Descriptor) protoreflect.Descriptor {
	d := query(firstName)
	if d == nil {
		return nil
	}
	if firstName == fullName {
		return d
	}
	if !isAggregateDescriptor(d) {
		// can't possibly find the rest of full name if
		// the first name indicated a leaf descriptor
		return nil
	}
	d = query(fullName)
	if d == nil {
		return newSentinelDescriptor(fullName)
	}
	return d
}

func resolveElementInFile(name protoreflect.FullName, f File) protoreflect.Descriptor {
	d := f.FindDescriptorByName(name)
	if d != nil {
		return d
	}

	if matchesPkgNamespace(name, f.Package()) {
		// this sentinel means the name is a valid namespace but
		// does not refer to a descriptor
		return newSentinelDescriptor(string(name))
	}
	return nil
}

func matchesPkgNamespace(fqn, pkg protoreflect.FullName) bool {
	if pkg == "" {
		return false
	}
	if fqn == pkg {
		return true
	}
	if len(pkg) > len(fqn) && strings.HasPrefix(string(pkg), string(fqn)) {
		// if char after fqn is a dot, then fqn is a namespace
		if pkg[len(fqn)] == '.' {
			return true
		}
	}
	return false
}

func isAggregateDescriptor(d protoreflect.Descriptor) bool {
	if isSentinelDescriptor(d) {
		// this indicates the name matched a package, not a
		// descriptor, but a package is an aggregate, so
		// we return true
		return true
	}
	switch d.(type) {
	case protoreflect.MessageDescriptor, protoreflect.EnumDescriptor, protoreflect.ServiceDescriptor:
		return true
	default:
		return false
	}
}

func isSentinelDescriptor(d protoreflect.Descriptor) bool {
	_, ok := d.(*sentinelDescriptor)
	return ok
}

func newSentinelDescriptor(name string) protoreflect.Descriptor {
	return &sentinelDescriptor{name: name}
}

// sentinelDescriptor is a placeholder descriptor. It is used instead of nil to
// distinguish between two situations:
//  1. The given name could not be found.
//  2. The given name *cannot* be a valid result so stop searching.
//
// In these cases, attempts to resolve an element name will return nil for the
// first case and will return a sentinelDescriptor in the second. The sentinel
// contains the fully-qualified name which caused the search to stop (which may
// be a prefix of the actual name being resolved).
type sentinelDescriptor struct {
	protoreflect.Descriptor
	name string
}

func (p *sentinelDescriptor) ParentFile() protoreflect.FileDescriptor {
	return nil
}

func (p *sentinelDescriptor) Parent() protoreflect.Descriptor {
	return nil
}

func (p *sentinelDescriptor) Index() int {
	return 0
}

func (p *sentinelDescriptor) Syntax() protoreflect.Syntax {
	return 0
}

func (p *sentinelDescriptor) Name() protoreflect.Name {
	return protoreflect.Name(p.name)
}

func (p *sentinelDescriptor) FullName() protoreflect.FullName {
	return protoreflect.FullName(p.name)
}

func (p *sentinelDescriptor) IsPlaceholder() bool {
	return false
}

func (p *sentinelDescriptor) Options() protoreflect.ProtoMessage {
	return nil
}

var _ protoreflect.Descriptor = (*sentinelDescriptor)(nil)
