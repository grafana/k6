package protoresolve

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// TypePool is a type resolver that allows for iteration over all known types.
type TypePool interface {
	TypeResolver
	RangeMessages(fn func(protoreflect.MessageType) bool)
	RangeEnums(fn func(protoreflect.EnumType) bool)
	RangeExtensions(fn func(protoreflect.ExtensionType) bool)
	RangeExtensionsByMessage(message protoreflect.FullName, fn func(protoreflect.ExtensionType) bool)
}

var _ TypePool = (*protoregistry.Types)(nil)

// TypeRegistry is a type resolver that allows the caller to add elements to
// the set of types it can resolve.
type TypeRegistry interface {
	TypePool
	RegisterMessage(protoreflect.MessageType) error
	RegisterEnum(protoreflect.EnumType) error
	RegisterExtension(protoreflect.ExtensionType) error
}

var _ TypeRegistry = (*protoregistry.Types)(nil)

// TypesFromResolver adapts a resolver that returns descriptors into a resolver
// that returns types. This can be used by implementations of Resolver to
// implement the [Resolver.AsTypeResolver] method.
//
// It returns all dynamic types except for extensions, in which case, if an
// extension implements [protoreflect.ExtensionTypeDescriptor], it will return
// its associated [protoreflect.ExtensionType]. (Otherwise it returns a dynamic
// extension.)
//
// If the given value implements DescriptorPool, then the returned value will
// implement TypePool.
//
// If the given value implements ExtensionPool, then the returned value will
// implement an additional method:
//
//	RangeExtensionsByMessage(message protoreflect.FullName, fn func(protoreflect.ExtensionType) bool)
func TypesFromResolver(resolver interface {
	DescriptorResolver
	ExtensionResolver
}) TypeResolver {
	switch pool := resolver.(type) {
	case DescriptorPool:
		return TypesFromDescriptorPool(pool)
	case ExtensionPool:
		return &typesAndExtensionPool{&typesFromResolver{resolver}, pool}
	default:
		return &typesFromResolver{resolver}
	}
}

// TypesFromDescriptorPool adapts a descriptor pool into a pool that returns
// types. This can be used by implementations of Resolver to implement the
// [Resolver.AsTypeResolver] method.
//
// If the given resolver implements ExtensionResolver, then the returned type
// pool provides an efficient implementation for the
// [ExtensionTypeResolver.FindExtensionByNumber] method. Otherwise, it will
// use an inefficient implementation that searches through all files for the
// requested extension.
func TypesFromDescriptorPool(pool DescriptorPool) TypePool {
	return &typesFromDescriptorPool{pool: pool}
}

type typesFromResolver struct {
	// The underlying resolver. It must be able to provide descriptors by name
	// and also be able to provide extension descriptors by extendee+tag number.
	resolver interface {
		DescriptorResolver
		ExtensionResolver
	}
}

func (t *typesFromResolver) FindExtensionByName(field protoreflect.FullName) (protoreflect.ExtensionType, error) {
	d, err := t.resolver.FindDescriptorByName(field)
	if err != nil {
		return nil, err
	}
	ext, ok := d.(protoreflect.ExtensionDescriptor)
	if !ok {
		return nil, NewUnexpectedTypeError(DescriptorKindExtension, d, "")
	}
	if !ext.IsExtension() {
		return nil, fmt.Errorf("%s is a normal field, not an extension", field)
	}
	return ExtensionType(ext), nil
}

func (t *typesFromResolver) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	ext, err := t.resolver.FindExtensionByNumber(message, field)
	if err != nil {
		return nil, err
	}
	return ExtensionType(ext), nil
}

func (t *typesFromResolver) FindMessageByName(message protoreflect.FullName) (protoreflect.MessageType, error) {
	d, err := t.resolver.FindDescriptorByName(message)
	if err != nil {
		return nil, err
	}
	msg, ok := d.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, NewUnexpectedTypeError(DescriptorKindMessage, d, "")
	}
	return dynamicpb.NewMessageType(msg), nil
}

func (t *typesFromResolver) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	return t.FindMessageByName(TypeNameFromURL(url))
}

func (t *typesFromResolver) FindEnumByName(enum protoreflect.FullName) (protoreflect.EnumType, error) {
	d, err := t.resolver.FindDescriptorByName(enum)
	if err != nil {
		return nil, err
	}
	en, ok := d.(protoreflect.EnumDescriptor)
	if !ok {
		return nil, NewUnexpectedTypeError(DescriptorKindEnum, d, "")
	}
	return dynamicpb.NewEnumType(en), nil
}

type typesAndExtensionPool struct {
	TypeResolver
	pool ExtensionPool
}

func (t *typesAndExtensionPool) RangeExtensionsByMessage(message protoreflect.FullName, fn func(protoreflect.ExtensionType) bool) {
	t.pool.RangeExtensionsByMessage(message, func(ext protoreflect.ExtensionDescriptor) bool {
		return fn(ExtensionType(ext))
	})
}

type typesFromDescriptorPool struct {
	pool DescriptorPool
}

func (t *typesFromDescriptorPool) FindExtensionByName(field protoreflect.FullName) (protoreflect.ExtensionType, error) {
	d, err := t.pool.FindDescriptorByName(field)
	if err != nil {
		return nil, err
	}
	ext, ok := d.(protoreflect.ExtensionDescriptor)
	if !ok {
		return nil, NewUnexpectedTypeError(DescriptorKindExtension, d, "")
	}
	if !ext.IsExtension() {
		return nil, fmt.Errorf("%s is a normal field, not an extension", field)
	}
	return ExtensionType(ext), nil
}

func (t *typesFromDescriptorPool) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	var ext protoreflect.ExtensionDescriptor
	var err error
	if extRes, ok := t.pool.(ExtensionResolver); ok {
		ext, err = extRes.FindExtensionByNumber(message, field)
	} else {
		ext = FindExtensionByNumber(t.pool, message, field)
		if ext == nil {
			err = protoregistry.NotFound
		}
	}
	if err != nil {
		return nil, err
	}
	return ExtensionType(ext), nil
}

func (t *typesFromDescriptorPool) FindMessageByName(message protoreflect.FullName) (protoreflect.MessageType, error) {
	d, err := t.pool.FindDescriptorByName(message)
	if err != nil {
		return nil, err
	}
	msg, ok := d.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, NewUnexpectedTypeError(DescriptorKindMessage, d, "")
	}
	return dynamicpb.NewMessageType(msg), nil
}

func (t *typesFromDescriptorPool) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	return t.FindMessageByName(TypeNameFromURL(url))
}

func (t *typesFromDescriptorPool) FindEnumByName(enum protoreflect.FullName) (protoreflect.EnumType, error) {
	d, err := t.pool.FindDescriptorByName(enum)
	if err != nil {
		return nil, err
	}
	en, ok := d.(protoreflect.EnumDescriptor)
	if !ok {
		return nil, NewUnexpectedTypeError(DescriptorKindEnum, d, "")
	}
	return dynamicpb.NewEnumType(en), nil
}

func (t *typesFromDescriptorPool) RangeMessages(fn func(protoreflect.MessageType) bool) {
	var rangeInContext func(container TypeContainer, fn func(protoreflect.MessageType) bool) bool
	rangeInContext = func(container TypeContainer, fn func(protoreflect.MessageType) bool) bool {
		msgs := container.Messages()
		for i, length := 0, msgs.Len(); i < length; i++ {
			msg := msgs.Get(i)
			if !fn(dynamicpb.NewMessageType(msg)) {
				return false
			}
			if !rangeInContext(msg, fn) {
				return false
			}
		}
		return true
	}
	t.pool.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		return rangeInContext(file, fn)
	})
}

func (t *typesFromDescriptorPool) RangeEnums(fn func(protoreflect.EnumType) bool) {
	var rangeInContext func(container TypeContainer, fn func(protoreflect.EnumType) bool) bool
	rangeInContext = func(container TypeContainer, fn func(protoreflect.EnumType) bool) bool {
		enums := container.Enums()
		for i, length := 0, enums.Len(); i < length; i++ {
			enum := enums.Get(i)
			if !fn(dynamicpb.NewEnumType(enum)) {
				return false
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
	t.pool.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		return rangeInContext(file, fn)
	})
}

func (t *typesFromDescriptorPool) RangeExtensions(fn func(protoreflect.ExtensionType) bool) {
	var rangeInContext func(container TypeContainer, fn func(protoreflect.ExtensionType) bool) bool
	rangeInContext = func(container TypeContainer, fn func(protoreflect.ExtensionType) bool) bool {
		exts := container.Extensions()
		for i, length := 0, exts.Len(); i < length; i++ {
			ext := exts.Get(i)
			if !fn(ExtensionType(ext)) {
				return false
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
	t.pool.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		return rangeInContext(file, fn)
	})
}

func (t *typesFromDescriptorPool) RangeExtensionsByMessage(message protoreflect.FullName, fn func(protoreflect.ExtensionType) bool) {
	if extPool, ok := t.pool.(ExtensionPool); ok {
		extPool.RangeExtensionsByMessage(message, func(ext protoreflect.ExtensionDescriptor) bool {
			return fn(ExtensionType(ext))
		})
		return
	}
	RangeExtensionsByMessage(t.pool, message, func(ext protoreflect.ExtensionDescriptor) bool {
		return fn(ExtensionType(ext))
	})
}
