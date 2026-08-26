package tcp

import (
	"testing"

	"go.k6.io/k6/v2/js/modulestest"
)

func newTestModuleInstance(t *testing.T) *module {
	t.Helper()

	runtime := modulestest.NewRuntime(t)
	root := new(rootModule)
	moduleInstance := root.NewModuleInstance(runtime.VU)

	mod, ok := moduleInstance.(*module)
	if !ok {
		t.Fatalf("failed to assert module instance")
	}

	return mod
}
