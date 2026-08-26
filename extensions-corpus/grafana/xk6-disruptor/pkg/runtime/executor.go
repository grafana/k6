package runtime

import (
	"os/exec"
)

// Executor offers methods for running processes
type Executor interface {
	// Exec executes a process and waits for its completion, returning
	// the combined stdout and stdout
	Exec(cmd string, args ...string) ([]byte, error)
}

// An instance of an executor that uses the os/exec package for
// executing processes
type executor struct{}

// DefaultExecutor returns a default executor
func DefaultExecutor() Executor {
	return &executor{}
}

// Exec executes a process and returns the combined stdout and stdin
func (e *executor) Exec(cmd string, args ...string) ([]byte, error) {
	return exec.Command(cmd, args...).CombinedOutput()
}
