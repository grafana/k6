package tcp

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

func Test_module(t *testing.T) {
	t.Parallel()

	vu := extensionapitest.NewVU()

	root := new(rootModule)
	mod := root.NewModuleInstance(vu)

	exports := mod.Exports()
	require.NotNil(t, exports)

	require.Nil(t, exports.Default)
	require.Contains(t, exports.Named, "Socket")
}
