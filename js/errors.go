package js

import (
	"fmt"
)

type DirectoryTraversalError struct {
	Filename string
	Root     string
}

func (e DirectoryTraversalError) Error() string {
	return fmt.Sprintf("loading files outside your working directory is prohibited, to protect against directory traversal attacks (%s is outside %s)", e.Filename, e.Root)
}
