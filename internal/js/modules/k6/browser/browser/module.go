// Package browser is the browser module's entry point, and
// initializer of various global types, and a translation layer
// between sobek and the internal business logic.
//
// It initializes and drives the downstream components by passing
// the necessary concrete dependencies.
package browser

import (
	"context"
	"io"
	"strconv"
	"sync"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common/autoscreenshot"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"

	k6modules "go.k6.io/k6/v2/js/modules"
)

type (
	// filePersister is the type that all file persisters must implement. It's job is
	// to persist a file somewhere, hiding the details of where and how from the caller.
	filePersister interface {
		Persist(ctx context.Context, path string, data io.Reader) (err error)
	}

	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct {
		PidRegistry           *pidRegistry
		remoteRegistry        *remoteRegistry
		initOnce              *sync.Once
		tracesMetadata        map[string]string
		filePersister         filePersister
		testRunID             string
		autoScreenshotMode    autoscreenshot.Mode
		autoScreenshotDedup   bool
		autoScreenshotPersist bool
	}

	// JSModule exposes the properties available to the JS script.
	JSModule struct {
		Browser         *sobek.Object
		Devices         map[string]common.Device
		NetworkProfiles map[string]common.NetworkProfile `js:"networkProfiles"`
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

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{
		PidRegistry: &pidRegistry{},
		initOnce:    &sync.Once{},
	}
}

// NewModuleInstance implements the k6modules.Module interface to return
// a new instance for each VU.
func (m *RootModule) NewModuleInstance(vu k6modules.VU) k6modules.Instance {
	// initialization should be done once per module as it initializes
	// globally used values across the whole test run and not just the
	// current VU. Since initialization can fail with an error,
	// we've had to place it here so that if an error occurs a
	// panic can be initiated and safely handled by k6.
	m.initOnce.Do(func() {
		m.initialize(vu)
	})

	asReg := autoscreenshot.NewRegistry(
		m.autoScreenshotMode,
		m.filePersister,
		resolveTestName(vu),
		log.NewNullLogger(),
		m.autoScreenshotDedup,
		m.autoScreenshotPersist,
	)

	return &ModuleInstance{
		mod: &JSModule{
			Browser: mapBrowserToSobek(moduleVU{
				VU:          vu,
				pidRegistry: m.PidRegistry,
				browserRegistry: newBrowserRegistry(
					context.Background(),
					vu,
					m.remoteRegistry,
					m.PidRegistry,
					m.tracesMetadata,
					asReg,
				),
				taskQueueRegistry: newTaskQueueRegistry(vu),
				filePersister:     m.filePersister,
				autoScreenshot:    asReg,
				testRunID:         m.testRunID,
			}),
			Devices:         common.GetDevices(),
			NetworkProfiles: common.GetNetworkProfiles(),
		},
	}
}

// resolveTestName picks a stable name for auto-screenshot path scoping. It
// prefers the first configured scenario name and falls back to a generic
// default. The scenario name is available via the VU's lib.Options when
// running under k6.
//
// This is provisional: a follow-up could derive a more informative name
// from the script filename or test-run metadata.
func resolveTestName(vu k6modules.VU) string {
	if state := vu.State(); state != nil {
		for name := range state.Options.Scenarios {
			return name
		}
	}
	return "k6-test"
}

// Exports returns the exports of the JS module so that it can be used in test
// scripts.
func (mi *ModuleInstance) Exports() k6modules.Exports {
	return k6modules.Exports{Default: mi.mod}
}

// initialize initializes the module instance with a new remote registry
// and debug server, etc.
func (m *RootModule) initialize(vu k6modules.VU) {
	var (
		err     error
		initEnv = vu.InitEnv()
	)
	m.remoteRegistry, err = newRemoteRegistry(initEnv.LookupEnv)
	if err != nil {
		k6ext.Abortf(vu.Context(), "failed to create remote registry: %v", err)
	}
	m.tracesMetadata, err = parseTracesMetadata(initEnv.LookupEnv)
	if err != nil {
		k6ext.Abortf(vu.Context(), "parsing browser traces metadata: %v", err)
	}
	m.filePersister, err = newScreenshotPersister(initEnv.LookupEnv)
	if err != nil {
		k6ext.Abortf(vu.Context(), "failed to create file persister: %v", err)
	}
	if e, ok := initEnv.LookupEnv(env.K6TestRunID); ok && e != "" {
		m.testRunID = e
	}
	if v, ok := initEnv.LookupEnv(env.AutoScreenshot); ok {
		m.autoScreenshotMode = autoscreenshot.ParseMode(v)
	}

	// CRC32 dedup defaults to enabled (current behaviour). Customers
	// can opt out by setting K6_BROWSER_AUTO_SCREENSHOT_DEDUP=false
	// (or 0/off) when the downstream consumer wants every triggered
	// frame regardless of pixel equality. Unparseable values are
	// silently treated as the default.
	m.autoScreenshotDedup = true
	if v, ok := initEnv.LookupEnv(env.AutoScreenshotDedup); ok {
		if b, perr := strconv.ParseBool(v); perr == nil {
			m.autoScreenshotDedup = b
		}
	}

	// Persist defaults to enabled (matches existing on-disk / S3
	// behaviour). Set K6_BROWSER_AUTO_SCREENSHOT_PERSIST=false to opt
	// into event-only mode (e.g. when a page.on('auto-screenshot')
	// handler ships the bytes elsewhere).
	m.autoScreenshotPersist = true
	if v, ok := initEnv.LookupEnv(env.AutoScreenshotPersist); ok {
		if b, perr := strconv.ParseBool(v); perr == nil {
			m.autoScreenshotPersist = b
		}
	}
}
