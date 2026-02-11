package protoresolve

import (
	"errors"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// Combine returns a resolver that iterates through the given resolvers to find elements.
// The first resolver given is the first one checked, so will always be the preferred resolver.
// When that returns a protoregistry.NotFound error, the next resolver will be checked, and so on.
//
// The NumFiles and NumFilesByPackage methods only return the number of files reported by the first
// resolver. (Computing an accurate number of files across all resolvers could be an expensive
// operation.) However, RangeFiles and RangeFilesByPackage do return files across all resolvers.
// They emit files for the first resolver first. If any subsequent resolver contains duplicates,
// they are suppressed such that the callback will only ever be invoked once for a given file path.
func Combine(res ...Resolver) Resolver {
	if len(res) == 0 {
		return combined(res)
	}
	allPools := true
	for _, r := range res {
		_, isPool := r.(interface {
			Resolver
			AsTypePool() TypePool
		})
		if !isPool {
			allPools = false
			break
		}
	}
	if !allPools {
		return combined(res)
	}

	pools := make([]TypePool, len(res))
	for i, r := range res {
		pools[i] = r.(interface {
			Resolver
			AsTypePool() TypePool
		}).AsTypePool()
	}
	return &combinedWithPool{combined: combined(res), pool: combinedPool(pools)}
}

// CombinePools is just like Combine, except that the returned value provides am
// AsTypePool() method that returns a TypePool that iterates through the TypePools
// of the given resolvers to find and enumerate elements. The AsTypeResolver()
// method of the returned value will also implement the broader TypePool interface.
func CombinePools(res ...interface {
	Resolver
	AsTypePool() TypePool
}) interface {
	Resolver
	AsTypePool() TypePool
} {
	pools := make([]TypePool, len(res))
	for i, r := range res {
		pools[i] = r.AsTypePool()
	}
	baseRes := make([]Resolver, len(res))
	for i, r := range res {
		baseRes[i] = r
	}
	return &combinedWithPool{combined: combined(baseRes), pool: combinedPool(pools)}
}

type combined []Resolver

func (c combined) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	for _, res := range c {
		file, err := res.FindFileByPath(path)
		if errors.Is(err, protoregistry.NotFound) {
			continue
		}
		return file, err
	}
	return nil, protoregistry.NotFound
}

func (c combined) NumFiles() int {
	if len(c) == 0 {
		return 0
	}
	return c[0].NumFiles()
}

func (c combined) RangeFiles(f func(protoreflect.FileDescriptor) bool) {
	observed := map[string]struct{}{}
	for _, res := range c {
		res.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
			if _, ok := observed[fd.Path()]; ok {
				return true
			}
			observed[fd.Path()] = struct{}{}
			return f(fd)
		})
	}
}

func (c combined) NumFilesByPackage(name protoreflect.FullName) int {
	if len(c) == 0 {
		return 0
	}
	return c[0].NumFilesByPackage(name)
}

func (c combined) RangeFilesByPackage(name protoreflect.FullName, f func(protoreflect.FileDescriptor) bool) {
	observed := map[string]struct{}{}
	for _, res := range c {
		res.RangeFilesByPackage(name, func(fd protoreflect.FileDescriptor) bool {
			if _, ok := observed[fd.Path()]; ok {
				return true
			}
			observed[fd.Path()] = struct{}{}
			return f(fd)
		})
	}
}

func (c combined) FindDescriptorByName(name protoreflect.FullName) (protoreflect.Descriptor, error) {
	for _, res := range c {
		d, err := res.FindDescriptorByName(name)
		if errors.Is(err, protoregistry.NotFound) {
			continue
		}
		return d, err
	}
	return nil, protoregistry.NotFound
}

func (c combined) FindMessageByName(name protoreflect.FullName) (protoreflect.MessageDescriptor, error) {
	for _, res := range c {
		msg, err := res.FindMessageByName(name)
		if errors.Is(err, protoregistry.NotFound) {
			continue
		}
		return msg, err
	}
	return nil, protoregistry.NotFound
}

func (c combined) FindExtensionByName(name protoreflect.FullName) (protoreflect.ExtensionDescriptor, error) {
	for _, res := range c {
		ext, err := res.FindExtensionByName(name)
		if errors.Is(err, protoregistry.NotFound) {
			continue
		}
		return ext, err
	}
	return nil, protoregistry.NotFound
}

func (c combined) FindExtensionByNumber(message protoreflect.FullName, number protoreflect.FieldNumber) (protoreflect.ExtensionDescriptor, error) {
	for _, res := range c {
		ext, err := res.FindExtensionByNumber(message, number)
		if errors.Is(err, protoregistry.NotFound) {
			continue
		}
		return ext, err
	}
	return nil, protoregistry.NotFound
}

func (c combined) RangeExtensionsByMessage(message protoreflect.FullName, fn func(protoreflect.ExtensionDescriptor) bool) {
	seen := map[protoreflect.FieldNumber]struct{}{}
	for _, res := range c {
		var keepGoing bool
		res.RangeExtensionsByMessage(message, func(ext protoreflect.ExtensionDescriptor) bool {
			if _, ok := seen[ext.Number()]; ok {
				return true
			}
			keepGoing = fn(ext)
			return keepGoing
		})
		if !keepGoing {
			return
		}
	}
}

func (c combined) FindMessageByURL(url string) (protoreflect.MessageDescriptor, error) {
	for _, res := range c {
		msg, err := res.FindMessageByURL(url)
		if errors.Is(err, protoregistry.NotFound) {
			continue
		}
		return msg, err
	}
	return nil, protoregistry.NotFound
}

func (c combined) AsTypeResolver() TypeResolver {
	return TypesFromResolver(c)
}

type combinedPool []TypePool

func (c combinedPool) FindExtensionByName(name protoreflect.FullName) (protoreflect.ExtensionType, error) {
	for _, res := range c {
		ext, err := res.FindExtensionByName(name)
		if errors.Is(err, protoregistry.NotFound) {
			continue
		}
		return ext, err
	}
	return nil, protoregistry.NotFound
}

func (c combinedPool) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	for _, res := range c {
		ext, err := res.FindExtensionByNumber(message, field)
		if errors.Is(err, protoregistry.NotFound) {
			continue
		}
		return ext, err
	}
	return nil, protoregistry.NotFound
}

func (c combinedPool) FindMessageByName(name protoreflect.FullName) (protoreflect.MessageType, error) {
	for _, res := range c {
		msg, err := res.FindMessageByName(name)
		if errors.Is(err, protoregistry.NotFound) {
			continue
		}
		return msg, err
	}
	return nil, protoregistry.NotFound
}

func (c combinedPool) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	for _, res := range c {
		msg, err := res.FindMessageByURL(url)
		if errors.Is(err, protoregistry.NotFound) {
			continue
		}
		return msg, err
	}
	return nil, protoregistry.NotFound
}

func (c combinedPool) FindEnumByName(name protoreflect.FullName) (protoreflect.EnumType, error) {
	for _, res := range c {
		en, err := res.FindEnumByName(name)
		if errors.Is(err, protoregistry.NotFound) {
			continue
		}
		return en, err
	}
	return nil, protoregistry.NotFound
}

func (c combinedPool) RangeMessages(fn func(protoreflect.MessageType) bool) {
	seen := map[protoreflect.FullName]struct{}{}
	for _, res := range c {
		var keepGoing bool
		res.RangeMessages(func(msg protoreflect.MessageType) bool {
			if _, ok := seen[msg.Descriptor().FullName()]; ok {
				return true
			}
			keepGoing = fn(msg)
			return keepGoing
		})
		if !keepGoing {
			return
		}
	}
}

func (c combinedPool) RangeEnums(fn func(protoreflect.EnumType) bool) {
	seen := map[protoreflect.FullName]struct{}{}
	for _, res := range c {
		var keepGoing bool
		res.RangeEnums(func(en protoreflect.EnumType) bool {
			if _, ok := seen[en.Descriptor().FullName()]; ok {
				return true
			}
			keepGoing = fn(en)
			return keepGoing
		})
		if !keepGoing {
			return
		}
	}
}

func (c combinedPool) RangeExtensions(fn func(protoreflect.ExtensionType) bool) {
	seen := map[protoreflect.FullName]struct{}{}
	for _, res := range c {
		var keepGoing bool
		res.RangeExtensions(func(ext protoreflect.ExtensionType) bool {
			if _, ok := seen[ext.TypeDescriptor().FullName()]; ok {
				return true
			}
			keepGoing = fn(ext)
			return keepGoing
		})
		if !keepGoing {
			return
		}
	}
}

func (c combinedPool) RangeExtensionsByMessage(message protoreflect.FullName, fn func(protoreflect.ExtensionType) bool) {
	seen := map[protoreflect.FieldNumber]struct{}{}
	for _, res := range c {
		var keepGoing bool
		res.RangeExtensionsByMessage(message, func(ext protoreflect.ExtensionType) bool {
			if _, ok := seen[ext.TypeDescriptor().Number()]; ok {
				return true
			}
			keepGoing = fn(ext)
			return keepGoing
		})
		if !keepGoing {
			return
		}
	}
}

type combinedWithPool struct {
	combined
	pool TypePool
}

func (c *combinedWithPool) AsTypeResolver() TypeResolver {
	return c.pool
}

func (c *combinedWithPool) AsTypePool() TypePool {
	return c.pool
}
