package browser

import (
	"log"
	"net/http"
	_ "net/http/pprof" // nolint:gosec
	"os"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/chromium"
	"github.com/grafana/xk6-browser/common"

	k6modules "go.k6.io/k6/js/modules"
)

const version = "0.6.0"

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// JSModule exposes the properties available to the JS script.
	JSModule struct {
		Chromium api.BrowserType
		Devices  map[string]common.Device
		Version  string
	}

	// ModuleInstance represents an instance of the JS module.
	ModuleInstance struct {
		mod *JSModule
	}
)

var (
	_ k6modules.Module   = &RootModule{}
	_ k6modules.Instance = &ModuleInstance{}
)

func init() {
	if _, ok := os.LookupEnv("K6_BROWSER_PPROF"); ok {
		go func() {
			address := "localhost:6060"
			log.Println("Starting http debug server", address)
			log.Println(http.ListenAndServe(address, nil))
		}()
	}
}

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the k6modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu k6modules.VU) k6modules.Instance {
	return &ModuleInstance{
		mod: &JSModule{
			Chromium: chromium.NewBrowserType(vu),
			Devices:  common.GetDevices(),
			Version:  version,
		},
	}
}

// Exports returns the exports of the JS module so that it can be used in test
// scripts.
func (mi *ModuleInstance) Exports() k6modules.Exports {
	return k6modules.Exports{Default: mi.mod}
}

func init() {
	k6modules.Register("k6/x/browser", New())
}
