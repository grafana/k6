package protoresolve

import (
	"errors"
	"fmt"
	"sync"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/jhump/protoreflect/v2/internal/reparse"
	"github.com/jhump/protoreflect/v2/internal/sort"
)

// Registry implements the full Resolver interface defined in this package. It is
// thread-safe and can be used for all kinds of operations where types or descriptors
// may need to be resolved from names or numbers.
//
// Furthermore, it memoizes the underlying descriptor protos, so one can efficiently
// recover a FileDescriptorProto for a particular FileDescriptor, without having to
// fully reconstruct it (which is what the [protodesc] package does). In order for
// this to function most efficiently, use [Registry.RegisterFileProto] to convert the
// descriptor proto into a [protoreflect.FileDescriptor] and then use
// [Registry.ProtoFromFileDescriptor] to recover the original proto.
type Registry struct {
	mu     sync.RWMutex
	files  protoregistry.Files
	exts   map[protoreflect.FullName]map[protoreflect.FieldNumber]protoreflect.FieldDescriptor
	protos map[protoreflect.FileDescriptor]*descriptorpb.FileDescriptorProto
}

var _ Resolver = (*Registry)(nil)
var _ DescriptorRegistry = (*Registry)(nil)
var _ ProtoFileRegistry = (*Registry)(nil)

// FromFiles returns a new registry that wraps the given files. After creating
// this registry, callers should not directly use files -- most especially, they
// should not register any additional descriptors with files and should instead
// use the RegisterFile method of the returned registry.
//
// This may return an error if the given files includes conflicting extension
// definitions (i.e. more than one extension for the same extended message and
// tag number).
//
// If protoregistry.GlobalFiles is supplied, a deep copy is made first. To avoid
// such a copy, use GlobalDescriptors instead.
func FromFiles(files *protoregistry.Files) (*Registry, error) {
	if files == protoregistry.GlobalFiles {
		// Don't wrap files if it's the global registry; make an effective copy
		reg := &Registry{}
		var err error
		files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
			err = reg.RegisterFile(fd)
			return err == nil
		})
		if err != nil {
			return nil, err
		}
		return reg, nil
	}

	reg := &Registry{
		files: *files,
	}
	// NB: It's okay to call methods below without first acquiring
	// lock because reg is not visible to any other goroutines yet.
	var err error
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		err = reg.checkExtensionsLocked(fd)
		return err == nil
	})
	if err != nil {
		return nil, err
	}
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		reg.registerExtensionsLocked(fd)
		return true
	})
	return reg, nil
}

// FromFileDescriptorSet constructs a *Registry from the given file descriptor set.
func FromFileDescriptorSet(files *descriptorpb.FileDescriptorSet) (*Registry, error) {
	var reg Registry
	if err := sort.SortFiles(files.File); err != nil {
		return nil, err
	}
	for _, file := range files.File {
		if _, err := reg.RegisterFileProto(file); err != nil {
			return nil, fmt.Errorf("failed to register %q: %w", file, err)
		}
	}
	return &reg, nil
}

// RegisterFileProto registers the given file descriptor proto and returns the
// corresponding [protoreflect.FileDescriptor]. All the file's dependencies must
// have already been registered.
//
// This will retain the given proto message, so calling code should not attempt to
// mutate it (or re-use it in a way that could mutate it). The given proto message
// will be supplied to callers of [ProtoFromFileDescriptor], to improve performance
// and fidelity over use of the [protodesc] package to recover the descriptor proto.
//
// In general, prefer calling this method instead of calling [protodesc.NewFile]
// followed by RegisterFile.
func (r *Registry) RegisterFileProto(fd *descriptorpb.FileDescriptorProto) (protoreflect.FileDescriptor, error) {
	file, err := protodesc.NewFile(fd, r)
	if err != nil {
		return nil, err
	}
	if reparse.ReparseUnrecognized(fd.ProtoReflect(), &extResolverForFile{file, r}) {
		// We were able to recognize some custom options, so re-create the
		// file with these newly recognized fields.
		file, err = protodesc.NewFile(fd, r)
		if err != nil {
			return nil, err
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.registerFileLocked(file, fd); err != nil {
		return nil, err
	}
	return file, nil
}

// RegisterFile implements part of the Resolver interface.
func (r *Registry) RegisterFile(file protoreflect.FileDescriptor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.registerFileLocked(file, nil)
}

func (r *Registry) registerFileLocked(file protoreflect.FileDescriptor, fd *descriptorpb.FileDescriptorProto) error {
	if err := r.checkExtensionsLocked(file); err != nil {
		_, findFileErr := r.files.FindFileByPath(file.Path())
		if findFileErr == nil {
			return fmt.Errorf("file %q already registered", file.Path())
		}
		return err
	}
	if err := r.files.RegisterFile(file); err != nil {
		return err
	}
	r.registerExtensionsLocked(file)
	if fd != nil {
		if r.protos == nil {
			r.protos = map[protoreflect.FileDescriptor]*descriptorpb.FileDescriptorProto{}
		}
		r.protos[file] = fd
	}
	return nil
}

func (r *Registry) checkExtensionsLocked(container TypeContainer) error {
	exts := container.Extensions()
	for i, length := 0, exts.Len(); i < length; i++ {
		ext := exts.Get(i)
		existing := r.exts[ext.ContainingMessage().FullName()][ext.Number()]
		if existing != nil {
			if existing.FullName() == ext.FullName() {
				return fmt.Errorf("extension named %q already registered", ext.FullName())
			}
			return fmt.Errorf("extension number %d for message %q already registered (existing: %q; trying to register: %q)",
				ext.Number(), ext.ContainingMessage().FullName(), existing.FullName(), ext.FullName())
		}
	}

	msgs := container.Messages()
	for i, length := 0, msgs.Len(); i < length; i++ {
		if err := r.checkExtensionsLocked(msgs.Get(i)); err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) registerExtensionsLocked(container TypeContainer) {
	exts := container.Extensions()
	for i, length := 0, exts.Len(); i < length; i++ {
		ext := exts.Get(i)
		if r.exts == nil {
			r.exts = map[protoreflect.FullName]map[protoreflect.FieldNumber]protoreflect.FieldDescriptor{}
		}
		extsForMsg := r.exts[ext.ContainingMessage().FullName()]
		if extsForMsg == nil {
			extsForMsg = map[protoreflect.FieldNumber]protoreflect.FieldDescriptor{}
			r.exts[ext.ContainingMessage().FullName()] = extsForMsg
		}
		extsForMsg[ext.Number()] = ext
	}

	msgs := container.Messages()
	for i, length := 0, msgs.Len(); i < length; i++ {
		r.registerExtensionsLocked(msgs.Get(i))
	}
}

// ProtoFromFileDescriptor recovers the file descriptor proto that
// corresponds to the given file.
//
// If the given file was created using RegisterFileProto, this will
// return the original file descriptor proto that was supplied to that
// method.
//
// Otherwise, a file descriptor proto will be created using the
// [protodesc] package. If the file does not already belong to this
// registry (i.e. was added via RegisterFile), this function will make
// a best effort attempt to add it. If the addition is successful or
// if the file already belonged to the registry, the resulting file
// descriptor proto will be memoized, so the same proto can be
// returned if this method is ever called again for the same file,
// without having to reconstruct it.
func (r *Registry) ProtoFromFileDescriptor(file protoreflect.FileDescriptor) (*descriptorpb.FileDescriptorProto, error) {
	if imp, ok := file.(protoreflect.FileImport); ok {
		file = imp.FileDescriptor
	}
	r.mu.RLock()
	fd := r.protos[file]
	r.mu.RUnlock()
	if fd != nil {
		return fd, nil
	}
	fd = protodesc.ToFileDescriptorProto(file)
	registered, err := r.FindFileByPath(file.Path())
	if err == nil && registered == file {
		r.mu.Lock()
		defer r.mu.Unlock()
		// this file already belongs to the registry
		// so go ahead and save this proto.
		if r.protos == nil {
			r.protos = map[protoreflect.FileDescriptor]*descriptorpb.FileDescriptorProto{}
		}
		r.protos[file] = fd
	} else if errors.Is(err, ErrNotFound) {
		r.mu.Lock()
		defer r.mu.Unlock()
		// best effort attempt to add file (and save proto if successful)
		_ = r.registerFileLocked(file, fd)
	}
	return fd, nil
}

// FindFileByPath implements part of the Resolver interface.
func (r *Registry) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.files.FindFileByPath(path)
}

// NumFiles implements part of the FilePool interface.
func (r *Registry) NumFiles() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.files.NumFiles()
}

// RangeFiles implements part of the FilePool interface.
func (r *Registry) RangeFiles(fn func(protoreflect.FileDescriptor) bool) {
	var files []protoreflect.FileDescriptor
	func() {
		r.mu.RLock()
		defer r.mu.RUnlock()
		files = make([]protoreflect.FileDescriptor, r.files.NumFiles())
		i := 0
		r.files.RangeFiles(func(f protoreflect.FileDescriptor) bool {
			files[i] = f
			i++
			return true
		})
	}()
	for _, file := range files {
		if !fn(file) {
			return
		}
	}
}

// NumFilesByPackage implements part of the FilePool interface.
func (r *Registry) NumFilesByPackage(name protoreflect.FullName) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.files.NumFilesByPackage(name)
}

// RangeFilesByPackage implements part of the FilePool interface.
func (r *Registry) RangeFilesByPackage(name protoreflect.FullName, fn func(protoreflect.FileDescriptor) bool) {
	var files []protoreflect.FileDescriptor
	func() {
		r.mu.RLock()
		defer r.mu.RUnlock()
		files = make([]protoreflect.FileDescriptor, r.files.NumFilesByPackage(name))
		i := 0
		r.files.RangeFilesByPackage(name, func(f protoreflect.FileDescriptor) bool {
			files[i] = f
			i++
			return true
		})
	}()
	for _, file := range files {
		if !fn(file) {
			return
		}
	}
}

// FindDescriptorByName implements part of the Resolver interface.
func (r *Registry) FindDescriptorByName(name protoreflect.FullName) (protoreflect.Descriptor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.files.FindDescriptorByName(name)
}

// FindMessageByName implements part of the Resolver interface.
func (r *Registry) FindMessageByName(name protoreflect.FullName) (protoreflect.MessageDescriptor, error) {
	d, err := r.FindDescriptorByName(name)
	if err != nil {
		return nil, err
	}
	msg, ok := d.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, NewUnexpectedTypeError(DescriptorKindMessage, d, "")
	}
	return msg, nil
}

// FindExtensionByName implements part of the Resolver interface.
func (r *Registry) FindExtensionByName(name protoreflect.FullName) (protoreflect.ExtensionDescriptor, error) {
	d, err := r.FindDescriptorByName(name)
	if err != nil {
		return nil, err
	}
	fld, ok := d.(protoreflect.FieldDescriptor)
	if !ok {
		return nil, NewUnexpectedTypeError(DescriptorKindExtension, d, "")
	}
	if !fld.IsExtension() {
		return nil, NewUnexpectedTypeError(DescriptorKindExtension, fld, "")
	}
	return fld, nil
}

// FindExtensionByNumber implements part of the Resolver interface.
func (r *Registry) FindExtensionByNumber(message protoreflect.FullName, fieldNumber protoreflect.FieldNumber) (protoreflect.ExtensionDescriptor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ext := r.exts[message][fieldNumber]
	if ext == nil {
		return nil, protoregistry.NotFound
	}
	return ext, nil
}

// FindMessageByURL implements part of the Resolver interface.
func (r *Registry) FindMessageByURL(url string) (protoreflect.MessageDescriptor, error) {
	return r.FindMessageByName(TypeNameFromURL(url))
}

// RangeExtensionsByMessage implements part of the Resolver interface.
func (r *Registry) RangeExtensionsByMessage(message protoreflect.FullName, fn func(protoreflect.ExtensionDescriptor) bool) {
	var exts []protoreflect.ExtensionDescriptor
	func() {
		r.mu.RLock()
		defer r.mu.RUnlock()
		extMap := r.exts[message]
		if len(extMap) == 0 {
			return
		}
		exts = make([]protoreflect.ExtensionDescriptor, len(extMap))
		i := 0
		for _, v := range extMap {
			exts[i] = v
			i++
		}
	}()
	for _, ext := range exts {
		if !fn(ext) {
			return
		}
	}
}

// AsTypeResolver implements part of the Resolver interface.
func (r *Registry) AsTypeResolver() TypeResolver {
	return r.AsTypePool()
}

// AsTypePool returns a view of this registry as a TypePool. This offers more methods
// than AsTypeResolver, providing the ability to enumerate types.
func (r *Registry) AsTypePool() TypePool {
	return TypesFromDescriptorPool(r)
}

type extResolverForFile struct {
	f protoreflect.FileDescriptor
	r ExtensionResolver
}

func (e *extResolverForFile) FindExtensionByName(field protoreflect.FullName) (protoreflect.ExtensionType, error) {
	ext, err := e.r.FindExtensionByName(field)
	if err == nil {
		return ExtensionType(ext), nil
	}
	desc := FindDescriptorByNameInFile(e.f, field)
	if desc == nil {
		return nil, ErrNotFound
	}
	ext, ok := desc.(protoreflect.FieldDescriptor)
	if !ok || !ext.IsExtension() {
		return nil, NewUnexpectedTypeError(DescriptorKindExtension, desc, "")
	}
	return ExtensionType(ext), nil
}

func (e *extResolverForFile) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	ext, err := e.r.FindExtensionByNumber(message, field)
	if err == nil {
		return ExtensionType(ext), nil
	}
	ext = FindExtensionByNumberInFile(e.f, message, field)
	if ext == nil {
		return nil, ErrNotFound
	}
	return ExtensionType(ext), nil
}
