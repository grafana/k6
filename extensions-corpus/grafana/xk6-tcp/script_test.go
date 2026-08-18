package tcp

import (
	"path/filepath"
	"testing"

	"github.com/grafana/xk6-tcp/internal/testscript"
	tcpmodule "github.com/grafana/xk6-tcp/tcp"
)

func TestModule(t *testing.T) {
	t.Parallel()

	testscript.RunGlob(t, filepath.Join("test", "*.test.js"), tcpmodule.ImportPath, tcpmodule.New())
}
