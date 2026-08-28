// Package faker is a xk6-faker extension module.
package faker

import (
	"github.com/grafana/xk6-faker/module"
	extensionapi "go.k6.io/k6-extension-api"
)

func register() {
	extensionapi.Register(module.ImportPath, module.New())
}

func init() { //nolint:gochecknoinits
	register()
}
