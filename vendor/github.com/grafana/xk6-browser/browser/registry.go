package browser

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/chromium"
	"github.com/grafana/xk6-browser/env"
	"github.com/grafana/xk6-browser/k6ext"

	k6event "go.k6.io/k6/event"
	k6modules "go.k6.io/k6/js/modules"
)

// errBrowserNotFoundInRegistry indicates that the browser instance
// for the iteration, which should have been initialized as a result
// of the IterStart event, has not been found in the registry. This
// might happen if browser type option is not set in scenario definition.
var errBrowserNotFoundInRegistry = errors.New("browser not found in registry. " +
	"make sure to set browser type option in scenario definition in order to use the browser module")

// pidRegistry keeps track of the launched browser process IDs.
type pidRegistry struct {
	mu  sync.RWMutex
	ids []int
}

// registerPid registers the launched browser process ID.
func (r *pidRegistry) registerPid(pid int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ids = append(r.ids, pid)
}

// Pids returns the launched browser process IDs.
func (r *pidRegistry) Pids() []int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pids := make([]int, len(r.ids))
	copy(pids, r.ids)

	return pids
}

// remoteRegistry contains the details of the remote web browsers.
// At the moment it's the WS URLs.
type remoteRegistry struct {
	isRemote bool
	wsURLs   []string
}

// newRemoteRegistry will create a new RemoteRegistry. This will
// parse the K6_BROWSER_WS_URL env var to retrieve the defined
// list of WS URLs.
//
// K6_BROWSER_WS_URL can be defined as a single WS URL or a
// comma separated list of URLs.
func newRemoteRegistry(envLookup env.LookupFunc) (*remoteRegistry, error) {
	r := &remoteRegistry{}

	isRemote, wsURLs, err := checkForScenarios(envLookup)
	if err != nil {
		return nil, err
	}
	if isRemote {
		r.isRemote = isRemote
		r.wsURLs = wsURLs
		return r, nil
	}

	r.isRemote, r.wsURLs = checkForBrowserWSURLs(envLookup)

	return r, nil
}

func checkForBrowserWSURLs(envLookup env.LookupFunc) (bool, []string) {
	wsURL, isRemote := envLookup(env.WebSocketURLs)
	if !isRemote {
		return false, nil
	}

	if !strings.ContainsRune(wsURL, ',') {
		return true, []string{wsURL}
	}

	// If last parts element is a void string,
	// because WS URL contained an ending comma,
	// remove it
	parts := strings.Split(wsURL, ",")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	return true, parts
}

// checkForScenarios will parse the K6_INSTANCE_SCENARIOS env var if
// it has been defined.
func checkForScenarios(envLookup env.LookupFunc) (bool, []string, error) {
	scenariosJSON, isRemote := envLookup(env.InstanceScenarios)
	if !isRemote {
		return false, nil, nil
	}
	// prevent failing in unquoting empty string.
	if scenariosJSON == "" {
		return false, nil, nil
	}
	scenariosJSON, err := strconv.Unquote(scenariosJSON)
	if err != nil {
		return false, nil, fmt.Errorf("unqouting K6_INSTANCE_SCENARIOS: %w", err)
	}

	var scenarios []struct {
		ID       string `json:"id"`
		Browsers []struct {
			Handle string `json:"handle"`
		} `json:"browsers"`
	}
	if err := json.Unmarshal([]byte(scenariosJSON), &scenarios); err != nil {
		return false, nil, fmt.Errorf("parsing K6_INSTANCE_SCENARIOS: %w", err)
	}

	var wsURLs []string
	for _, s := range scenarios {
		for _, b := range s.Browsers {
			if strings.TrimSpace(b.Handle) == "" {
				continue
			}
			wsURLs = append(wsURLs, b.Handle)
		}
	}
	if len(wsURLs) == 0 {
		return false, wsURLs, nil
	}

	return true, wsURLs, nil
}

// isRemoteBrowser returns a WS URL and true when a WS URL is defined,
// otherwise it returns an empty string and false. If more than one
// WS URL was registered in newRemoteRegistry, a randomly chosen URL from
// the list in a round-robin fashion is selected and returned.
func (r *remoteRegistry) isRemoteBrowser() (string, bool) {
	if !r.isRemote {
		return "", false
	}

	// Choose a random WS URL from the provided list
	i, _ := rand.Int(rand.Reader, big.NewInt(int64(len(r.wsURLs))))
	wsURL := r.wsURLs[i.Int64()]

	return wsURL, true
}

// browserRegistry stores a single VU browser instances
// indexed per iteration.
type browserRegistry struct {
	vu k6modules.VU

	mu sync.RWMutex
	m  map[int64]api.Browser

	buildFn browserBuildFunc

	stopped atomic.Bool // testing purposes
}

type browserBuildFunc func(ctx context.Context) (api.Browser, error)

func newBrowserRegistry(vu k6modules.VU, remote *remoteRegistry, pids *pidRegistry) *browserRegistry {
	bt := chromium.NewBrowserType(vu)
	builder := func(ctx context.Context) (api.Browser, error) {
		var (
			err                    error
			b                      api.Browser
			wsURL, isRemoteBrowser = remote.isRemoteBrowser()
		)

		if isRemoteBrowser {
			b, err = bt.Connect(ctx, wsURL)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
		} else {
			var pid int
			b, pid, err = bt.Launch(ctx)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			pids.registerPid(pid)
		}

		return b, nil
	}

	r := &browserRegistry{
		vu:      vu,
		m:       make(map[int64]api.Browser),
		buildFn: builder,
	}

	exitSubID, exitCh := vu.Events().Global.Subscribe(
		k6event.Exit,
	)
	iterSubID, eventsCh := vu.Events().Local.Subscribe(
		k6event.IterStart,
		k6event.IterEnd,
	)
	unsubscribe := func() {
		vu.Events().Local.Unsubscribe(iterSubID)
		vu.Events().Global.Unsubscribe(exitSubID)
	}

	go r.handleExitEvent(exitCh, unsubscribe)
	go r.handleIterEvents(eventsCh, unsubscribe)

	return r
}

func (r *browserRegistry) handleIterEvents(eventsCh <-chan *k6event.Event, unsubscribeFn func()) {
	var (
		ok   bool
		data k6event.IterData
		ctx  = context.Background()
	)

	for e := range eventsCh {
		// If browser module is imported in the test, NewModuleInstance will be called for
		// every VU. Because on VU init stage we can not distinguish to which scenario it
		// belongs or access its options (because state is nil), we have to always subscribe
		// to each VU iter events, including VUs that do not make use of the browser in their
		// iterations.
		// Therefore, if we get an event that does not correspond to a browser iteration, then
		// unsubscribe for the VU events and exit the loop in order to reduce unuseful overhead.
		if !isBrowserIter(r.vu) {
			unsubscribeFn()
			r.stop()
			e.Done()
			return
		}

		// The context in the VU is not thread safe. It can
		// be safely accessed during an iteration but not
		// before one is started. This is why it is being
		// accessed and used here.
		vuCtx := k6ext.WithVU(r.vu.Context(), r.vu)

		if data, ok = e.Data.(k6event.IterData); !ok {
			e.Done()
			k6ext.Abort(vuCtx, "unexpected iteration event data format: %v", e.Data)
			// Continue so we don't block the k6 event system producer.
			// Test will be aborted by k6, which will previously send the
			// 'Exit' event so browser resources cleanup can be guaranteed.
			continue
		}

		switch e.Type { //nolint:exhaustive
		case k6event.IterStart:
			b, err := r.buildFn(ctx)
			if err != nil {
				e.Done()
				k6ext.Abort(vuCtx, "error building browser on IterStart: %v", err)
				// Continue so we don't block the k6 event system producer.
				// Test will be aborted by k6, which will previously send the
				// 'Exit' event so browser resources cleanup can be guaranteed.
				continue
			}
			r.setBrowser(data.Iteration, b)
		case k6event.IterEnd:
			r.deleteBrowser(data.Iteration)
		default:
			r.vu.State().Logger.Warnf("received unexpected event type: %v", e.Type)
		}

		e.Done()
	}
}

func (r *browserRegistry) handleExitEvent(exitCh <-chan *k6event.Event, unsubscribeFn func()) {
	defer unsubscribeFn()

	e, ok := <-exitCh
	if !ok {
		return
	}
	defer e.Done()
	r.clear()
}

func (r *browserRegistry) setBrowser(id int64, b api.Browser) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.m[id] = b
}

func (r *browserRegistry) getBrowser(id int64) (api.Browser, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if b, ok := r.m[id]; ok {
		return b, nil
	}

	return nil, errBrowserNotFoundInRegistry
}

func (r *browserRegistry) deleteBrowser(id int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if b, ok := r.m[id]; ok {
		b.Close()
		delete(r.m, id)
	}
}

func (r *browserRegistry) clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, b := range r.m {
		b.Close()
		delete(r.m, id)
	}
}

func (r *browserRegistry) stop() {
	r.stopped.Store(true)
}

func isBrowserIter(vu k6modules.VU) bool {
	opts := k6ext.GetScenarioOpts(vu.Context(), vu)
	_, ok := opts["type"] // Check if browser type option is set
	return ok
}
