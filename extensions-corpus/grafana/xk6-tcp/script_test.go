package tcp

import (
	"path/filepath"
	"testing"

	"github.com/grafana/xk6-tcp/internal/testscript"
)

func TestModule(t *testing.T) {
	t.Parallel()

	testscript.RunGlob(t, filepath.Join("test", "*.test.js"))
}

func TestIntegration(t *testing.T) { //nolint:paralleltest
	testscript.RunGlobIntegration(t, filepath.Join("test", "*.test.js"))
	testscript.RunGlobIntegration(t, filepath.Join("examples", "*.[tj]s"))
}
