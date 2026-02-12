package sourceinfo

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/jhump/protoreflect/v2/protoresolve"
)

var (
	// Files is a registry of descriptors that include source code info, if the
	// files they belong to were processed with protoc-gen-gosrcinfo.
	//
	// It is meant to serve as a drop-in alternative to protoregistry.GlobalFiles
	// that can include source code info in the returned descriptors.
	Files protoresolve.DescriptorPool = files{}

	// Types is a registry of types that include source code info, if the
	// files they belong to were processed with protoc-gen-gosrcinfo.
	//
	// It is meant to serve as a drop-in alternative to protoregistry.GlobalTypes
	// that can include source code info in the returned types.
	Types protoresolve.TypePool = types{}

	mu                   sync.RWMutex
	sourceInfoDataByFile = map[string][]byte{}
	sourceInfoByFile     = map[string]*descriptorpb.SourceCodeInfo{}
	fileDescriptors      = map[protoreflect.FileDescriptor]protoreflect.FileDescriptor{}
	updatedDescriptors   filesWithFallback
)

// Register registers the given source code info, which is a serialized
// and gzipped form of a google.protobuf.SourceCodeInfo message.
//
// This is automatically used from generated code if using the protoc-gen-gosrcinfo
// plugin.
func Register(file string, data []byte) {
	mu.Lock()
	defer mu.Unlock()
	sourceInfoDataByFile[file] = data
}

// ForFile queries for any registered source code info for the file
// descriptor with the given path/name. It returns nil if no source code info
// was registered.
func ForFile(file string) (*descriptorpb.SourceCodeInfo, error) {
	mu.RLock()
	srcInfo := sourceInfoByFile[file]
	var data []byte
	if srcInfo == nil {
		data = sourceInfoDataByFile[file]
	}
	mu.RUnlock()

	if srcInfo != nil {
		return srcInfo, nil
	}
	if data == nil {
		return nil, nil
	}

	srcInfo, err := processSourceInfoData(data)
	if err != nil {
		return nil, err
	}

	mu.Lock()
	defer mu.Unlock()
	// check again after upgrading lock
	if existing := sourceInfoByFile[file]; existing != nil {
		srcInfo = existing
	} else {
		sourceInfoByFile[file] = srcInfo
	}
	return srcInfo, nil
}

func processSourceInfoData(data []byte) (*descriptorpb.SourceCodeInfo, error) {
	zipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = zipReader.Close()
	}()
	unzipped, err := io.ReadAll(zipReader)
	if err != nil {
		return nil, err
	}
	var srcInfo descriptorpb.SourceCodeInfo
	if err := proto.Unmarshal(unzipped, &srcInfo); err != nil {
		return nil, err
	}
	return &srcInfo, nil
}

func forFileLocked(file string) (*descriptorpb.SourceCodeInfo, error) {
	if existing := sourceInfoByFile[file]; existing != nil {
		return existing, nil
	}
	data := sourceInfoDataByFile[file]
	if data == nil {
		return nil, nil
	}
	srcInfo, err := processSourceInfoData(data)
	if err != nil {
		return nil, err
	}
	sourceInfoByFile[file] = srcInfo
	return srcInfo, nil
}

func canUpgrade(d protoreflect.Descriptor) bool {
	if d == nil {
		return false
	}
	fd := d.ParentFile()
	if fd.SourceLocations().Len() > 0 {
		// already has source info
		return false
	}
	if genFile, err := protoregistry.GlobalFiles.FindFileByPath(fd.Path()); err != nil || genFile != fd {
		// given descriptor is not from generated code
		return false
	}
	return true
}

func getFile(fd protoreflect.FileDescriptor) (protoreflect.FileDescriptor, error) {
	if !canUpgrade(fd) {
		return fd, nil
	}

	mu.RLock()
	result := fileDescriptors[fd]
	mu.RUnlock()

	if result != nil {
		return result, nil
	}

	mu.Lock()
	defer mu.Unlock()
	result, err := getFileLocked(fd)
	if err != nil {
		return nil, fmt.Errorf("adding source info to file %q: %w", fd.Path(), err)
	}
	return result, nil
}

func getFileLocked(fd protoreflect.FileDescriptor) (protoreflect.FileDescriptor, error) {
	result := fileDescriptors[fd]
	if result != nil {
		return result, nil
	}

	// We have to build its dependencies, too, so that the descriptor's
	// references *all* have source code info.
	var deps []protoreflect.FileDescriptor
	imps := fd.Imports()
	for i, length := 0, imps.Len(); i < length; i++ {
		origDep := imps.Get(i).FileDescriptor
		updatedDep, err := getFileLocked(origDep)
		if err != nil {
			return nil, fmt.Errorf("updating import %q: %w", origDep.Path(), err)
		}
		if updatedDep != origDep && deps == nil {
			// lazily init slice of deps and copy over deps before this one
			deps = make([]protoreflect.FileDescriptor, i, length)
			for j := 0; j < i; j++ {
				deps[j] = imps.Get(i).FileDescriptor
			}
		}
		if deps != nil {
			deps = append(deps, updatedDep)
		}
	}

	srcInfo, err := forFileLocked(fd.Path())
	if err != nil {
		return nil, err
	}
	if len(srcInfo.GetLocation()) == 0 && len(deps) == 0 {
		// nothing to do; don't bother changing
		return fd, nil
	}

	// Add source code info and rebuild.
	fdProto := protodesc.ToFileDescriptorProto(fd)
	fdProto.SourceCodeInfo = srcInfo

	result, err = protodesc.NewFile(fdProto, &updatedDescriptors)
	if err != nil {
		return nil, err
	}
	if err := updatedDescriptors.RegisterFile(result); err != nil {
		return nil, fmt.Errorf("registering import %q: %w", result.Path(), err)
	}

	fileDescriptors[fd] = result
	return result, nil
}

type files struct{}

func (files) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	fd, err := protoregistry.GlobalFiles.FindFileByPath(path)
	if err != nil {
		return nil, err
	}
	return getFile(fd)
}

func (files) FindDescriptorByName(name protoreflect.FullName) (protoreflect.Descriptor, error) {
	d, err := protoregistry.GlobalFiles.FindDescriptorByName(name)
	if err != nil {
		return nil, err
	}
	if !canUpgrade(d) {
		return d, nil
	}
	switch d := d.(type) {
	case protoreflect.FileDescriptor:
		return getFile(d)
	case protoreflect.MessageDescriptor:
		return updateDescriptor(d)
	case protoreflect.FieldDescriptor:
		return updateField(d)
	case protoreflect.OneofDescriptor:
		return updateDescriptor(d)
	case protoreflect.EnumDescriptor:
		return updateDescriptor(d)
	case protoreflect.EnumValueDescriptor:
		return updateDescriptor(d)
	case protoreflect.ServiceDescriptor:
		return updateDescriptor(d)
	case protoreflect.MethodDescriptor:
		return updateDescriptor(d)
	default:
		return nil, fmt.Errorf("unrecognized descriptor type: %T", d)
	}
}

func (files) NumFiles() int {
	return protoregistry.GlobalFiles.NumFiles()
}

func (files) RangeFiles(fn func(protoreflect.FileDescriptor) bool) {
	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		updated, err := getFile(file)
		if err != nil {
			return fn(file)
		}
		return fn(updated)
	})
}

func (files) NumFilesByPackage(name protoreflect.FullName) int {
	return protoregistry.GlobalFiles.NumFilesByPackage(name)
}

func (files) RangeFilesByPackage(name protoreflect.FullName, fn func(protoreflect.FileDescriptor) bool) {
	protoregistry.GlobalFiles.RangeFilesByPackage(name, func(file protoreflect.FileDescriptor) bool {
		updated, err := getFile(file)
		if err != nil {
			return fn(file)
		}
		return fn(updated)
	})
}

type types struct{}

func (types) FindMessageByName(message protoreflect.FullName) (protoreflect.MessageType, error) {
	mt, err := protoregistry.GlobalTypes.FindMessageByName(message)
	if err != nil {
		return nil, err
	}
	msg, err := updateDescriptor(mt.Descriptor())
	if err != nil {
		return mt, nil
	}
	return messageType{MessageType: mt, msgDesc: msg}, nil
}

func (types) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	mt, err := protoregistry.GlobalTypes.FindMessageByURL(url)
	if err != nil {
		return nil, err
	}
	msg, err := updateDescriptor(mt.Descriptor())
	if err != nil {
		return mt, nil
	}
	return messageType{MessageType: mt, msgDesc: msg}, nil
}

func (types) FindExtensionByName(field protoreflect.FullName) (protoreflect.ExtensionType, error) {
	xt, err := protoregistry.GlobalTypes.FindExtensionByName(field)
	if err != nil {
		return nil, err
	}
	ext, err := updateDescriptor(xt.TypeDescriptor().Descriptor())
	if err != nil {
		return xt, nil
	}
	return extensionType{ExtensionType: xt, extDesc: ext}, nil
}

func (types) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	xt, err := protoregistry.GlobalTypes.FindExtensionByNumber(message, field)
	if err != nil {
		return nil, err
	}
	ext, err := updateDescriptor(xt.TypeDescriptor().Descriptor())
	if err != nil {
		return xt, nil
	}
	return extensionType{ExtensionType: xt, extDesc: ext}, nil
}

func (types) FindEnumByName(enum protoreflect.FullName) (protoreflect.EnumType, error) {
	et, err := protoregistry.GlobalTypes.FindEnumByName(enum)
	if err != nil {
		return nil, err
	}
	en, err := updateDescriptor(et.Descriptor())
	if err != nil {
		return et, nil
	}
	return enumType{EnumType: et, enumDesc: en}, nil
}

func (types) RangeMessages(fn func(protoreflect.MessageType) bool) {
	protoregistry.GlobalTypes.RangeMessages(func(mt protoreflect.MessageType) bool {
		msg, err := updateDescriptor(mt.Descriptor())
		if err != nil {
			return fn(mt)
		}
		return fn(messageType{MessageType: mt, msgDesc: msg})
	})
}

func (types) RangeEnums(fn func(protoreflect.EnumType) bool) {
	protoregistry.GlobalTypes.RangeEnums(func(et protoreflect.EnumType) bool {
		en, err := updateDescriptor(et.Descriptor())
		if err != nil {
			return fn(et)
		}
		return fn(enumType{EnumType: et, enumDesc: en})
	})
}

func (types) RangeExtensions(fn func(protoreflect.ExtensionType) bool) {
	protoregistry.GlobalTypes.RangeExtensions(func(xt protoreflect.ExtensionType) bool {
		ext, err := updateDescriptor(xt.TypeDescriptor().Descriptor())
		if err != nil {
			return fn(xt)
		}
		return fn(extensionType{ExtensionType: xt, extDesc: ext})
	})
}

func (types) RangeExtensionsByMessage(message protoreflect.FullName, fn func(protoreflect.ExtensionType) bool) {
	protoregistry.GlobalTypes.RangeExtensionsByMessage(message, func(xt protoreflect.ExtensionType) bool {
		ext, err := updateDescriptor(xt.TypeDescriptor().Descriptor())
		if err != nil {
			return fn(xt)
		}
		return fn(extensionType{ExtensionType: xt, extDesc: ext})
	})
}

type filesWithFallback struct {
	protoregistry.Files
}

func (f *filesWithFallback) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	fd, err := f.Files.FindFileByPath(path)
	if errors.Is(err, protoregistry.NotFound) {
		fd, err = protoregistry.GlobalFiles.FindFileByPath(path)
	}
	return fd, err
}

func (f *filesWithFallback) FindDescriptorByName(name protoreflect.FullName) (protoreflect.Descriptor, error) {
	fd, err := f.Files.FindDescriptorByName(name)
	if errors.Is(err, protoregistry.NotFound) {
		fd, err = protoregistry.GlobalFiles.FindDescriptorByName(name)
	}
	return fd, err
}
