package sort

import (
	"fmt"

	"google.golang.org/protobuf/types/descriptorpb"
)

// SortFiles topologically sorts the given file descriptor protos. It returns
// an error if the given files include duplicates (more than one entry with the
// same path) or if any of the files refer to imports which are not present in
// the given files.
func SortFiles(files []*descriptorpb.FileDescriptorProto) error {
	allFiles := make(map[string]fileState, len(files))
	for _, file := range files {
		if _, exists := allFiles[file.GetName()]; exists {
			return fmt.Errorf("duplicate file %q", file.GetName())
		}
		allFiles[file.GetName()] = fileState{file: file}
	}
	origLen := len(files)
	files = files[:0]
	for _, file := range files {
		if err := addFileSorted(file, allFiles, &files); err != nil {
			return err
		}
	}
	if origLen != len(files) {
		// should not be possible since we've already removed duplicates...
		return fmt.Errorf("internal: sorted files has length %d, but original had length %d", len(files), origLen)
	}
	return nil
}

func addFileSorted(file *descriptorpb.FileDescriptorProto, allFiles map[string]fileState, sorted *[]*descriptorpb.FileDescriptorProto) error {
	state := allFiles[file.GetName()]
	if state.added {
		return nil
	}
	state.added = true
	allFiles[file.GetName()] = state
	for _, dep := range file.GetDependency() {
		depFile := allFiles[dep]
		if depFile.file == nil {
			return fmt.Errorf("file %q imports %q, but %q is not present", file.GetName(), dep, dep)
		}
		if err := addFileSorted(depFile.file, allFiles, sorted); err != nil {
			return err
		}
	}
	*sorted = append(*sorted, file)
	return nil
}

type fileState struct {
	file  *descriptorpb.FileDescriptorProto
	added bool
}
