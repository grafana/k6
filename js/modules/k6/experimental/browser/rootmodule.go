package browser

import (
	"errors"
	"os"
	"strconv"

	xk6browser "github.com/grafana/xk6-browser/browser"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct {
		rm *xk6browser.RootModule
	}
)

func New() *RootModule {
	return &RootModule{
		rm: xk6browser.New(),
	}
}

func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	throwError := func() {
		msg := "To run browser tests set env var K6_BROWSER_ENABLE_RUN=true"
		if m, ok := os.LookupEnv("K6_BROWSER_ENABLE_RUN_MSG"); ok && m != "" {
			msg = m
		}

		common.Throw(vu.Runtime(), errors.New(msg))
	}

	vs, ok := os.LookupEnv("K6_BROWSER_ENABLE_RUN")
	if !ok {
		throwError()
	}

	v, err := strconv.ParseBool(vs)
	if err != nil || !v {
		throwError()
	}

	return r.rm.NewModuleInstance(vu)
}
