package protoresolve

import (
	"bytes"
	"fmt"
	"math/bits"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// FindExtensionByNumber searches the given descriptor pool for the requested extension.
// This performs an inefficient search through all files and extensions in the pool.
// It returns nil if the extension is not found in the file.
func FindExtensionByNumber(res DescriptorPool, message protoreflect.FullName, field protoreflect.FieldNumber) protoreflect.ExtensionDescriptor {
	var ext protoreflect.ExtensionDescriptor
	res.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		ext = FindExtensionByNumberInFile(fd, message, field)
		return ext == nil
	})
	return ext
}

// FindExtensionByNumberInFile searches all extension in the given file for the requested
// extension. It returns nil if the extension is not found in the file.
func FindExtensionByNumberInFile(file protoreflect.FileDescriptor, message protoreflect.FullName, field protoreflect.FieldNumber) protoreflect.ExtensionDescriptor {
	return findExtension(file, message, field)
}

func findExtension(container TypeContainer, message protoreflect.FullName, field protoreflect.FieldNumber) protoreflect.FieldDescriptor {
	// search extensions in this scope
	exts := container.Extensions()
	for i, length := 0, exts.Len(); i < length; i++ {
		ext := exts.Get(i)
		if ext.Number() == field && ext.ContainingMessage().FullName() == message {
			return ext
		}
	}

	// if not found, search nested scopes
	msgs := container.Messages()
	for i, length := 0, msgs.Len(); i < length; i++ {
		msg := msgs.Get(i)
		ext := findExtension(msg, message, field)
		if ext != nil {
			return ext
		}
	}
	return nil
}

// RangeExtensionsByMessage enumerates all extensions in the given descriptor pool that
// extend the given message. It stops early if the given function returns false.
func RangeExtensionsByMessage(res DescriptorPool, message protoreflect.FullName, fn func(descriptor protoreflect.ExtensionDescriptor) bool) {
	var rangeInContext func(container TypeContainer, fn func(protoreflect.ExtensionDescriptor) bool) bool
	rangeInContext = func(container TypeContainer, fn func(protoreflect.ExtensionDescriptor) bool) bool {
		exts := container.Extensions()
		for i, length := 0, exts.Len(); i < length; i++ {
			ext := exts.Get(i)
			if ext.ContainingMessage().FullName() == message {
				if !fn(ext) {
					return false
				}
			}
		}
		msgs := container.Messages()
		for i, length := 0, msgs.Len(); i < length; i++ {
			msg := msgs.Get(i)
			if !rangeInContext(msg, fn) {
				return false
			}
		}
		return true
	}
	res.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		return rangeInContext(file, fn)
	})
}

// FindDescriptorByNameInFile searches the given file for the element with the given
// fully-qualified name. This could be used to implement the
// [DescriptorResolver.FindDescriptorByName] method for a resolver that doesn't want
// to create an index of all descriptors. This returns nil if no element with the
// given name belongs to this file.
//
// This does not perform a brute-force search of all elements to find the given name.
// It breaks up the given name into components and then descends the descriptor
// hierarchy one element at a time. If the given name does not start with the file's
// package, it immediately returns nil.
func FindDescriptorByNameInFile(file protoreflect.FileDescriptor, sym protoreflect.FullName) protoreflect.Descriptor {
	symNoPkg := string(sym)
	if file.Package() != "" {
		symNoPkg = strings.TrimPrefix(string(sym), string(file.Package())+".")
		if symNoPkg == string(sym) {
			// symbol is not in this file's package
			return nil
		}
	}
	parts := strings.Split(symNoPkg, ".")
	return findSymbolInFile(parts, file)
}

func findSymbolInFile(symbolParts []string, fd protoreflect.FileDescriptor) protoreflect.Descriptor {
	// ==1 name means it's a direct child of this file
	if len(symbolParts) == 1 {
		n := protoreflect.Name(symbolParts[0])
		if d := fd.Messages().ByName(n); d != nil {
			return d
		}
		if d := fd.Enums().ByName(n); d != nil {
			return d
		}
		if d := fd.Extensions().ByName(n); d != nil {
			return d
		}
		if d := fd.Services().ByName(n); d != nil {
			return d
		}
		// enum values are defined in the scope that encloses the enum, so
		// we have to look in all enums to find top-level enum values
		enums := fd.Enums()
		for i, length := 0, enums.Len(); i < length; i++ {
			enum := enums.Get(i)
			if d := enum.Values().ByName(n); d != nil {
				return d
			}
		}
		// not in this file
		return nil
	}

	// >1 name means it's inside a message or (if ==2) a method inside a service
	first := protoreflect.Name(symbolParts[0])
	if len(symbolParts) == 2 {
		second := protoreflect.Name(symbolParts[1])
		if svc := fd.Services().ByName(first); svc != nil {
			if d := svc.Methods().ByName(second); d != nil {
				return d
			}
			return nil
		}
	}
	rest := symbolParts[1:]
	if msg := fd.Messages().ByName(first); msg != nil {
		return findSymbolInMessage(rest, msg)
	}

	// no other option; can't be in this file
	return nil
}

func findSymbolInMessage(symbolParts []string, md protoreflect.MessageDescriptor) protoreflect.Descriptor {
	// ==1 name means it's a direct child of this message
	if len(symbolParts) == 1 {
		n := protoreflect.Name(symbolParts[0])
		if d := md.Fields().ByName(n); d != nil {
			return d
		}
		if d := md.Oneofs().ByName(n); d != nil {
			return d
		}
		if d := md.Messages().ByName(n); d != nil {
			return d
		}
		if d := md.Enums().ByName(n); d != nil {
			return d
		}
		if d := md.Extensions().ByName(n); d != nil {
			return d
		}
		// enum values are defined in the scope that encloses the enum, so
		// we have to look in all enums to find enum values at this level
		enums := md.Enums()
		for i, length := 0, enums.Len(); i < length; i++ {
			enum := enums.Get(i)
			if d := enum.Values().ByName(n); d != nil {
				return d
			}
		}
		// not in this file
		return nil
	}

	// >1 name means it's inside a nested message
	first := protoreflect.Name(symbolParts[0])
	rest := symbolParts[1:]
	if nested := md.Messages().ByName(first); nested != nil {
		return findSymbolInMessage(rest, nested)
	}

	// no other option; can't be in this message
	return nil
}

// ExtensionType returns a [protoreflect.ExtensionType] for the given descriptor.
// If the given descriptor implements [protoreflect.ExtensionTypeDescriptor], then
// the corresponding type is returned. Otherwise, a dynamic extension type is
// returned (created using "google.golang.org/protobuf/types/dynamicpb").
func ExtensionType(ext protoreflect.ExtensionDescriptor) protoreflect.ExtensionType {
	if xtd, ok := ext.(protoreflect.ExtensionTypeDescriptor); ok {
		return xtd.Type()
	}
	return dynamicpb.NewExtensionType(ext)
}

// TypeNameFromURL extracts the fully-qualified type name from the given URL.
// The URL is one that could be used with a google.protobuf.Any message. The
// last path component is the fully-qualified name.
func TypeNameFromURL(url string) protoreflect.FullName {
	pos := strings.LastIndexByte(url, '/')
	return protoreflect.FullName(url[pos+1:])
}

// TypeKind represents a category of types that can be registered in a TypeRegistry.
// The value for a particular kind is a single bit, so a TypeKind value can also
// represent multiple kinds, by setting multiple bits (by combining values via
// bitwise-OR).
type TypeKind int

// The various supported TypeKind values.
const (
	TypeKindMessage = TypeKind(1 << iota)
	TypeKindEnum
	TypeKindExtension

	// TypeKindsAll is a bitmask that represents all types.
	TypeKindsAll = TypeKindMessage | TypeKindEnum | TypeKindExtension
	// TypeKindsSerialization includes the kinds of types needed for serialization
	// and de-serialization: messages (for interpreting google.protobuf.Any messages)
	// and extensions. These are the same types as supported in a SerializationResolver.
	TypeKindsSerialization = TypeKindMessage | TypeKindExtension
)

func (k TypeKind) String() string {
	switch k {
	case TypeKindMessage:
		return "message"
	case TypeKindEnum:
		return "enum"
	case TypeKindExtension:
		return "extension"
	case 0:
		return "<none>"
	default:
		i := uint(k)
		if bits.OnesCount(i) == 1 {
			return fmt.Sprintf("unknown kind (%d)", k)
		}

		var buf bytes.Buffer
		l := bits.UintSize
		for i != 0 {
			if buf.Len() > 0 {
				buf.WriteByte(',')
			}
			z := bits.LeadingZeros(i)
			if z == l {
				break
			}
			shr := l - z - 1
			elem := TypeKind(1 << shr)
			buf.WriteString(elem.String())
		}
		return buf.String()
	}
}

// RegisterTypesInFile registers all the types (with kinds that match kindMask) with
// the given registry. Only the types directly in file are registered. This will result
// in an error if any of the types in the given file are already registered as belonging
// to a different file.
//
// All types will be dynamic types, created with the "google.golang.org/protobuf/types/dynamicpb"
// package. The only exception is for extension descriptors that also implement
// [protoreflect.ExtensionTypeDescriptor], in which case the corresponding extension type is used.
func RegisterTypesInFile(file protoreflect.FileDescriptor, reg TypeRegistry, kindMask TypeKind) error {
	return registerTypes(file, reg, kindMask)
}

// RegisterTypesInFileRecursive registers all the types (with kinds that match kindMask)
// with the given registry, for the given file and all of its transitive dependencies (i.e.
// its imports, and their imports, etc.). This will result in an error if any of the types in
// the given file (and its dependencies) are already registered as belonging to a different file.
//
// All types will be dynamic types, created with the "google.golang.org/protobuf/types/dynamicpb"
// package. The only exception is for extension descriptors that also implement
// [protoreflect.ExtensionTypeDescriptor], in which case the corresponding extension type is used.
func RegisterTypesInFileRecursive(file protoreflect.FileDescriptor, reg TypeRegistry, kindMask TypeKind) error {
	pathsSeen := map[string]struct{}{}
	return registerTypesInFileRecursive(file, reg, kindMask, pathsSeen)
}

// RegisterTypesInFilesRecursive registers all the types (with kinds that match kindMask)
// with the given registry, for all files in the given pool and their dependencies. This is
// essentially shorthand for this:
//
//		var err error
//		files.RangeFiles(func(file protoreflect.FileDescriptor) bool {
//	 		err = protoresolve.RegisterTypesInFileRecursive(file, reg, kindMask)
//	 		return err == nil
//		})
//		return err
//
// However, the actual implementation is a little more efficient for cases where some files
// are imported by many other files.
//
// All types will be dynamic types, created with the "google.golang.org/protobuf/types/dynamicpb"
// package. The only exception is for extension descriptors that also implement
// [protoreflect.ExtensionTypeDescriptor], in which case the corresponding extension type is used.
func RegisterTypesInFilesRecursive(files FilePool, reg TypeRegistry, kindMask TypeKind) error {
	pathsSeen := map[string]struct{}{}
	var err error
	files.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		err = registerTypesInFileRecursive(file, reg, kindMask, pathsSeen)
		return err == nil
	})
	return err
}

func registerTypesInFileRecursive(file protoreflect.FileDescriptor, reg TypeRegistry, kindMask TypeKind, pathsSeen map[string]struct{}) error {
	if _, ok := pathsSeen[file.Path()]; ok {
		// already processed
		return nil
	}
	pathsSeen[file.Path()] = struct{}{}
	imports := file.Imports()
	for i, length := 0, imports.Len(); i < length; i++ {
		imp := imports.Get(i)
		if err := registerTypesInFileRecursive(imp.FileDescriptor, reg, kindMask, pathsSeen); err != nil {
			return err
		}
	}
	return registerTypes(file, reg, kindMask)
}

// TypeContainer is a descriptor that contains types. Both [protoreflect.FileDescriptor] and
// [protoreflect.MessageDescriptor] can contain types so both satisfy this interface.
type TypeContainer interface {
	Messages() protoreflect.MessageDescriptors
	Enums() protoreflect.EnumDescriptors
	Extensions() protoreflect.ExtensionDescriptors
}

var _ TypeContainer = (protoreflect.FileDescriptor)(nil)
var _ TypeContainer = (protoreflect.MessageDescriptor)(nil)

func registerTypes(container TypeContainer, reg TypeRegistry, kindMask TypeKind) error {
	msgs := container.Messages()
	for i, length := 0, msgs.Len(); i < length; i++ {
		msg := msgs.Get(i)
		if kindMask&TypeKindMessage != 0 {
			var skip bool
			if existing := findType(reg, msg.FullName()); existing != nil {
				if existing.ParentFile().Path() != msg.ParentFile().Path() {
					return fmt.Errorf("type %s is defined in both %q and %q", msg.FullName(), existing.ParentFile().Path(), msg.ParentFile().Path())
				}
				skip = true
			}
			if !skip {
				if err := reg.RegisterMessage(dynamicpb.NewMessageType(msg)); err != nil {
					return err
				}
			}
		}
		// register nested types
		if err := registerTypes(msg, reg, kindMask); err != nil {
			return err
		}
	}

	if kindMask&TypeKindEnum != 0 {
		enums := container.Enums()
		for i, length := 0, enums.Len(); i < length; i++ {
			enum := enums.Get(i)
			var skip bool
			if existing := findType(reg, enum.FullName()); existing != nil {
				if existing.ParentFile().Path() != enum.ParentFile().Path() {
					return fmt.Errorf("type %s is defined in both %q and %q", enum.FullName(), existing.ParentFile().Path(), enum.ParentFile().Path())
				}
				skip = true
			}
			if !skip {
				if err := reg.RegisterEnum(dynamicpb.NewEnumType(enum)); err != nil {
					return err
				}
			}
		}
	}

	if kindMask&TypeKindExtension != 0 {
		exts := container.Extensions()
		for i, length := 0, exts.Len(); i < length; i++ {
			ext := exts.Get(i)
			var skip bool
			if existing := findType(reg, ext.FullName()); existing != nil {
				if existing.ParentFile().Path() != ext.ParentFile().Path() {
					return fmt.Errorf("type %s is defined in both %q and %q", ext.FullName(), existing.ParentFile().Path(), ext.ParentFile().Path())
				}
				skip = true
			}
			if !skip {
				// also check extendee+tag
				existing, err := reg.FindExtensionByNumber(ext.ContainingMessage().FullName(), ext.Number())
				if err == nil {
					if existing.TypeDescriptor().ParentFile().Path() != ext.ParentFile().Path() {
						return fmt.Errorf("extension number %d for %s is defined in both %q and %q", ext.Number(), ext.ContainingMessage().FullName(), existing.TypeDescriptor().ParentFile().Path(), ext.ParentFile().Path())
					}
					skip = true
				}
			}
			if !skip {
				if err := reg.RegisterExtension(ExtensionType(ext)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func findType(res TypeResolver, name protoreflect.FullName) protoreflect.Descriptor {
	msg, err := res.FindMessageByName(name)
	if err == nil {
		return msg.Descriptor()
	}
	en, err := res.FindEnumByName(name)
	if err == nil {
		return en.Descriptor()
	}
	ext, err := res.FindExtensionByName(name)
	if err == nil {
		return ext.TypeDescriptor()
	}
	return nil
}
