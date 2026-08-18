package tcp

import (
	"testing"

	extensionapitest "go.k6.io/k6-extension-api/test"
)

func newTestModuleInstance(t *testing.T) *module {
	t.Helper()

	vu := extensionapitest.NewVU()
	root := new(rootModule)
	moduleInstance := root.NewModuleInstance(vu)

	mod, ok := moduleInstance.(*module)
	if !ok {
		t.Fatalf("failed to assert module instance")
	}

	return mod
}
