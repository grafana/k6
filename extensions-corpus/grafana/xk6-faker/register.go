// Package faker is a xk6-faker extension module.
package faker

import (
	"github.com/grafana/xk6-faker/module"
	"go.k6.io/k6/v2/js/modules"
)

func register() {
	modules.Register(module.ImportPath, module.New())
}

func init() { //nolint:gochecknoinits
	register()
}
