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
	"sync"
	"time"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"

	k6event "go.k6.io/k6/v2/internal/event"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/chromium"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/storage"

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
		PidRegistry    *pidRegistry
		remoteRegistry *remoteRegistry
		initOnce       *sync.Once
		tracesMetadata map[string]string
		filePersister  filePersister
		testRunID      string
	}

	// JSModule exposes the properties available to the JS script.
	JSModule struct {
		Browser         *sobek.Object
		Chromium        *sobek.Object
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

	mvu := moduleVU{
		VU:          vu,
		pidRegistry: m.PidRegistry,
		browserRegistry: newBrowserRegistry(
			context.Background(),
			vu,
			m.remoteRegistry,
			m.PidRegistry,
			m.tracesMetadata,
		),
		taskQueueRegistry: newTaskQueueRegistry(vu),
		filePersister:     m.filePersister,
		testRunID:         m.testRunID,
	}

	return &ModuleInstance{
		mod: &JSModule{
			Browser:         mapToSobek(mvu, mapBrowser(mvu, mvu.browser)),
			Chromium:        mapToSobek(mvu, mapChromium(mvu, chromium.NewBrowserType(vu))),
			Devices:         common.GetDevices(),
			NetworkProfiles: common.GetNetworkProfiles(),
		},
	}
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
	m.filePersister, err = newScreenshotPersister(initEnv.LookupEnv, initEnv.Logger)
	if err != nil {
		k6ext.Abortf(vu.Context(), "failed to create file persister: %v", err)
	}
	if ap, ok := m.filePersister.(*storage.AsyncPersister); ok {
		flushOnTestEnd(vu, ap, initEnv.Logger)
	}
	if e, ok := initEnv.LookupEnv(env.K6TestRunID); ok && e != "" {
		m.testRunID = e
	}
}

// screenshotsFlushTimeout is how long the test end waits for queued screenshot
// uploads to finish.
const screenshotsFlushTimeout = 60 * time.Second

// flushOnTestEnd waits for the queued screenshot uploads when the test ends. The
// test end event is handled before outputs are stopped, so the uploads finish
// before the test run is reported as finished.
func flushOnTestEnd(vu k6modules.VU, p *storage.AsyncPersister, logger logrus.FieldLogger) {
	subID, events := vu.Events().Global.Subscribe(k6event.TestEnd)
	go func() {
		e := <-events
		defer vu.Events().Global.Unsubscribe(subID)
		if e == nil {
			return
		}
		defer e.Done()

		ctx, cancel := context.WithTimeout(context.Background(), screenshotsFlushTimeout)
		defer cancel()
		if err := p.Flush(ctx); err != nil {
			logger.Warnf("waiting for screenshot uploads: %v", err)
		}
	}()
}
