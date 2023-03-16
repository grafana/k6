//go:build !amd64 && !arm64

package compiler

import (
	"fmt"
	"runtime"
)

// archContext is empty on an unsupported architecture.
type archContext struct{}

// newCompiler panics with an unsupported error.
func newCompiler() compiler {
	panic(fmt.Sprintf("unsupported GOARCH %s", runtime.GOARCH))
}
