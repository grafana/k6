package output

import "go.k6.io/k6/ext"

// Constructor returns an instance of an output extension module.
type Constructor func(Params) (Output, error)

// RegisterExtension registers the given output extension constructor. This
// function panics if a module with the same name is already registered.
func RegisterExtension(name string, c Constructor) {
	ext.Register(name, ext.OutputExtension, c)
}
