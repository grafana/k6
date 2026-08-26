package tcp

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
)

func Test_module(t *testing.T) {
	t.Parallel()

	runtime := modulestest.NewRuntime(t)

	root := new(rootModule)
	mod := root.NewModuleInstance(runtime.VU)

	exports := mod.Exports()
	require.NotNil(t, exports)

	require.Nil(t, exports.Default)
	require.Contains(t, exports.Named, "Socket")
}
