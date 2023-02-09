// Package browser contains a RootModule wrapper
// that wraps around the experimental browser
// RootModule.
package browser

import (
	"errors"
	"strconv"

	xk6browser "github.com/grafana/xk6-browser/browser"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

type (
	// RootModule is a wrapper around the experimental
	// browser RootModule. It will prevent browser test
	// runs unless K6_BROWSER_ENABLED env var is set.
	RootModule struct {
		rm *xk6browser.RootModule
	}
)

// New creates an experimental browser RootModule
// and wraps it around this internal RootModule.
func New() *RootModule {
	return &RootModule{
		rm: xk6browser.New(),
	}
}

// NewModuleInstance will check to see if
// K6_BROWSER_ENABLED is set before allowing
// test runs to continue.
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	env := vu.InitEnv()

	throwError := func() {
		msg := "To run browser tests set env var K6_BROWSER_ENABLED=true"
		if m, ok := env.LookupEnv("K6_BROWSER_ENABLED_MSG"); ok && m != "" {
			msg = m
		}

		common.Throw(vu.Runtime(), errors.New(msg))
	}

	vs, ok := env.LookupEnv("K6_BROWSER_ENABLED")
	if !ok {
		throwError()
	}

	v, err := strconv.ParseBool(vs)
	if err != nil || !v {
		throwError()
	}

	return r.rm.NewModuleInstance(vu)
}
