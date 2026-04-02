// Package protoresolve contains named types for various kinds of resolvers for
// use with Protobuf reflection.
//
// # Overview
//
// The core protobuf runtime API (provided by the [google.golang.org/protobuf] module)
// accepts resolvers for a number of things, such as unmarshalling from binary, JSON,
// and text formats and for creating [protoreflect.FileDescriptor] instances from
// *[descriptorpb.FileDescriptorProto] instances. However, it uses anonymous interface
// types for many of these cases. This package provides named types, useful for more
// compact parameter and field declarations as well as type assertions.
//
// The core protobuf runtime API also includes two resolver implementations:
//   - *[protoregistry.Files]: for resolving descriptors.
//   - *[protoregistry.Types]: for resolving types.
//
// When using descriptors, such that all types are dynamic, using the above two
// types requires double the work to register everything with both. The first must
// be used in order to process [FileDescriptorProto] instances into more useful
// [FileDescriptor] instances, and the second must be used to create a resolver
// that the other APIs accept.
//
// The Registry type in this package, on the other hand, allows callers to register
// descriptors once, and the result can automatically be used as a type registry,
// too, with all dynamic types.
//
// This package also provides functions for composition: layering resolvers
// such that one is tried first (the "preferred" resolver), and then others
// can be used if the first fails to resolve. This is useful to blend known
// and unknown types. (See Combine.)
//
// You can use the Resolver interface in this package with the existing global
// registries ([protoregistry.GlobalFiles] and [protoregistry.GlobalTypes]) via the
// GlobalDescriptors value. This implements Resolver and is backed by these two
// global registries.
//
// The next sections describe the taxonomy of elements in this package.
//
// # Resolvers
//
// Named resolver interfaces make up the majority of exported elements in this
// package. This provides nominal types so that type declarations (like in
// function parameters and return types) and type assertions can be more compact
// and readable without requiring packages redefine their own, small named types.
//
// There are two broad classes of resolvers: those that provide *descriptors* and
// those that provide *types*. A resolver that provides types is the marriage of
// descriptors with Go's type system and generated types. A resolver that provides
// types might be able to return information about generated message structs.
// Whereas a resolver that provides descriptors can only return message descriptors
// and nothing that binds that message to a Go type. The bridge between the two is
// the [dynamicpb] package, which can generate dynamic types to represent a
// descriptor.
//
// In this package, resolvers that provide types generally have "Type" or
// "TypeResolver" in their name. The other interfaces provide descriptors.
//
// Below are the interfaces that are expected to be used the most:
//
//   - Resolver: This provides an interface similar to that of [protoregistry.Files]
//     except that it is broader and includes typed accessors for looking up
//     descriptors. So instead of just the generic FindDescriptorByName, it also
//     provides FindMessageDescriptorByName.
//
//   - FileResolver & DescriptorResolver: These two interfaces are implemented by
//     both Resolver implementations and the [protoregistry.Files]. So this can be
//     used as a parameter type, and callers of the function could provide either
//     (or some other implementation of just the necessary methods).
//
//   - SerializationResolver: This kind of resolver is used for marshalling and
//     unmarshalling Protobuf data. It provides types for messages and extensions.
//     When marshalling or unmarshalling a message to/from JSON, for example, the
//     contents of google.protobuf.Any messages must be resolved into descriptors
//     in order to correctly transcode the data to JSON. When unmarshalling from
//     the Protobuf binary format, tag numbers must be resolved into field and
//     extension descriptors in order for the data to be interpreted.
//
// The Resolver interface implements most of the other smaller resolver interfaces
// that provide descriptors. And it also includes an AsTypeResolver method, so it
// can be converted to a resolver that provides types.
//
// # Pools
//
// A pool is a special kind of resolver that can also be used to enumerate known
// elements. There are three kinds of pools:
//
//   - FilePool: A file pool allows enumeration of all file descriptors therein.
//   - ExtensionPool: An extension pool allows enumeration of all known extensions.
//   - TypePool: A type pool allows enumeration of all known types -- messages, enums,
//     and extensions.
//
// The latter provides types; both of the former provide descriptors.
//
// # Registries
//
// A registry is a special kind of pool that allows new entries to be registered.
// It is basically a mutable pool. There are two kinds of registries in this package:
//
//   - TypeRegistry: This interface allows users to record new types. This package
//     does not actually contain any implementations. Instead, *[protoregistry.Types]
//     is the recommended implementation.
//   - DescriptorRegistry: This interface allows users to record new descriptors.
//     This package contains an implementation in the form of *Registry. It is also
//     implemented by *[protoregistry.Files].
//
// The *Registry concrete type is a DescriptorRegistry that also provides the full API
// of the Resolver interface.
//
// # Helpers
//
// This package also contains myriad helper functions related to resolving and handling
// descriptors. Some of them are also useful to actually implement the resolver
// interfaces (FindExtensionByNumber, ExtensionType, TypeNameFromURL, and TypesFromResolver).
// And some are adapters, wrapping types that implement one interface to also provide another
// (ResolverFromPool, ResolverFromPools, TypesFromDescriptorPool, and TypesFromResolver).
//
// [google.golang.org/protobuf]: https://pkg.go.dev/google.golang.org/protobuf
// [FileDescriptorProto]: https://pkg.go.dev/google.golang.org/protobuf/types/descriptorpb#FileDescriptorProto
// [FileDescriptor]: https://pkg.go.dev/google.golang.org/protobuf/reflect/protoreflect#FileDescriptor
package protoresolve

import (
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

var _ = protoreflect.Descriptor(nil)
var _ = (*protoregistry.Files)(nil)
var _ = (*descriptorpb.FileDescriptorProto)(nil)
var _ = (*dynamicpb.Message)(nil)
