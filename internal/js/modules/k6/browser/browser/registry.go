package browser

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"sync"

	oteltrace "go.opentelemetry.io/otel/trace"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/chromium"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext"
	browsertrace "go.k6.io/k6/v2/internal/js/modules/k6/browser/trace"
	"go.k6.io/k6/v2/internal/js/taskqueue"

	k6event "go.k6.io/k6/v2/internal/event"
	k6modules "go.k6.io/k6/v2/js/modules"
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

	tracer         *browsertrace.Tracer
	tracerInit     sync.Once
	tracesMetadata map[string]string

	mu sync.RWMutex
	m  map[int64]*common.Browser
	// userManaged holds browsers created via chromium.connectOverCDP, keyed by
	// iteration. Unlike m (browsers k6 launches/connects from scenario options),
	// these are connected by the script at runtime. k6 still sweeps them on
	// IterEnd and Exit. A slice (not a single slot) so a repeat connect in the
	// same iteration does not clobber the first.
	userManaged map[int64][]*common.Browser

	buildFn browserBuildFunc
}

type browserBuildFunc func(ctx, vuCtx context.Context) (*common.Browser, error)

// newBrowserRegistry should only take a background context, not a context from
// k6 (i.e. vu). The reason for this is that we want to control the chromium
// lifecycle with the k6 event system.
//
// The k6 event system gives this extension time to properly cleanup any running
// chromium subprocesses or connections to a remote chromium instance.
//
// A vu context (a context on an iteration) doesn't allow us to do this. Once k6
// closes a vu context, it basically pulls the rug from under the extensions feet.
func newBrowserRegistry(
	ctx context.Context,
	vu k6modules.VU,
	remote *remoteRegistry,
	pids *pidRegistry,
	tracesMetadata map[string]string,
) *browserRegistry {
	bt := chromium.NewBrowserType(vu)
	builder := func(ctx, vuCtx context.Context) (*common.Browser, error) {
		var (
			err                    error
			b                      *common.Browser
			wsURL, isRemoteBrowser = remote.isRemoteBrowser()
		)

		if isRemoteBrowser {
			b, err = bt.Connect(ctx, vuCtx, wsURL)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
		} else {
			var pid int
			b, pid, err = bt.Launch(ctx, vuCtx)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			pids.registerPid(pid)
		}

		return b, nil
	}

	r := &browserRegistry{
		vu:             vu,
		tracesMetadata: tracesMetadata,
		m:              make(map[int64]*common.Browser),
		userManaged:    make(map[int64][]*common.Browser),
		buildFn:        builder,
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
	go r.handleIterEvents(ctx, eventsCh)

	return r
}

func (r *browserRegistry) handleIterEvents(
	ctx context.Context, eventsCh <-chan *k6event.Event,
) {
	var (
		ok   bool
		data k6event.IterData
	)

	for e := range eventsCh {
		// If the browser module is imported, NewModuleInstance runs for every VU, so
		// this registry subscribes to iteration events for every VU. At VU init we
		// cannot tell which scenario a VU belongs to (state is nil), so we cannot
		// subscribe selectively; we also cannot unsubscribe later, since a VU may be
		// reused by a later scenario that does use the browser.
		//
		// This handler used to skip non-browser iterations entirely (an early
		// `continue` when !isBrowserIter), because only managed browser scenarios had
		// anything to clean up. That is no longer safe: chromium.connectOverCDP lets a
		// script create a browser in any iteration, without options.browser. So we now
		// process every iteration event and gate only the managed-browser work
		// (buildFn on IterStart, deleteBrowser on IterEnd) behind isBrowserIter below;
		// the user-managed-browser sweep (closeUserManaged) runs
		// unconditionally on IterEnd.

		// The context in the VU is not thread safe. It can
		// be safely accessed during an iteration but not
		// before one is started. This is why it is being
		// accessed and used here.
		vuCtx := k6ext.WithVU(r.vu.Context(), r.vu)

		if data, ok = e.Data.(k6event.IterData); !ok {
			e.Done()
			k6ext.Abortf(vuCtx, "unexpected iteration event data format: %v", e.Data)
			// Continue so we don't block the k6 event system producer.
			// Test will be aborted by k6, which will previously send the
			// 'Exit' event so browser resources cleanup can be guaranteed.
			continue
		}

		browserIter := isBrowserIter(r.vu)

		switch e.Type {
		case k6event.IterStart:
			// Managed browsers are only built for browser scenarios. Connected
			// (connectOverCDP) browsers are built lazily at connection time, so
			// there is nothing to do at IterStart for them.
			if !browserIter {
				break
			}

			// Because VU.State is nil when browser registry is initialized,
			// we have to initialize the tracer on the first VU iteration
			// so we can get access to the k6 TracerProvider.
			//
			// The VU context already carries the iteration span started by k6,
			// so the browser spans are created as its children.
			tracedCtx := r.withTracer(r.vu.Context())

			b, err := r.buildFn(ctx, tracedCtx)
			if err != nil {
				e.Done()
				k6ext.Abortf(vuCtx, "error building browser on IterStart: %v", err)
				// Continue so we don't block the k6 event system producer.
				// Test will be aborted by k6, which will previously send the
				// 'Exit' event so browser resources cleanup can be guaranteed.
				continue
			}
			r.setBrowser(data.Iteration, b)
		case k6event.IterEnd:
			// Always sweep user-managed browsers, even for non-browser
			// iterations (connectOverCDP runs in iterations without
			// options.browser).
			if browserIter {
				r.deleteBrowser(data.Iteration)
			}
			r.closeUserManaged(data.Iteration)
			// k6 ends the iteration span after the IterEnd event has been handled, so the
			// navigation spans of the iteration end before it.
			if r.tracer != nil {
				r.tracer.EndLiveSpans()
			}
		default:
			r.vu.State().Logger.Warnf("received unexpected event type: %v", e.Type)
		}

		e.Done()
	}
}

func (r *browserRegistry) handleExitEvent(exitCh <-chan *k6event.Event, unsubscribeFn func()) {
	var e *k6event.Event
	// Defers run LIFO: unsubscribeFn (registered second) runs before e.Done
	// (registered first). unsubscribeFn must complete before Done is signalled
	// so that by the time the caller's waitDone returns, the subscription is
	// already removed. Otherwise a concurrent Exit emission (e.g. from
	// t.Cleanup in tests) can be delivered to a closed goroutine, causing
	// e.Done to never be called and waitDone to block forever.
	defer func() {
		if e != nil {
			e.Done()
		}
	}()
	defer unsubscribeFn()

	var ok bool
	e, ok = <-exitCh
	if !ok {
		return
	}
	r.clear()
}

func (r *browserRegistry) setBrowser(id int64, b *common.Browser) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.m[id] = b
}

func (r *browserRegistry) getBrowser(id int64) (*common.Browser, error) {
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

// This is only used in a test. Avoids having to manipulate the mutex in the
// test itself.
func (r *browserRegistry) browserCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.m)
}

func (r *browserRegistry) clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, b := range r.m {
		b.Close()
		delete(r.m, id)
	}
	for iter, browsers := range r.userManaged {
		for _, b := range browsers {
			b.Close()
		}
		delete(r.userManaged, iter)
	}
}

func (r *browserRegistry) trackUserManagedBrowser(iter int64, b *common.Browser) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.userManaged[iter] = append(r.userManaged[iter], b)
}

func (r *browserRegistry) untrackUserManagedBrowser(iter int64, b *common.Browser) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s := r.userManaged[iter]
	for i, x := range s {
		if x == b {
			r.userManaged[iter] = append(s[:i], s[i+1:]...)
			break
		}
	}
	if len(r.userManaged[iter]) == 0 {
		delete(r.userManaged, iter)
	}
}

// userManagedIter returns the iteration a user-managed browser is tracked under
// and whether it is tracked at all. It's used to decide whether a browser
// mapping should expose close() (only user-managed browsers do).
func (r *browserRegistry) userManagedIter(b *common.Browser) (int64, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for iter, browsers := range r.userManaged {
		if slices.Contains(browsers, b) {
			return iter, true
		}
	}
	return 0, false
}

// closeUserManaged gracefully closes and removes all user-managed browsers for iter.
// Close is done outside the lock because it performs CDP round-trips.
func (r *browserRegistry) closeUserManaged(iter int64) {
	r.mu.Lock()
	browsers := r.userManaged[iter]
	delete(r.userManaged, iter)
	r.mu.Unlock()

	for _, b := range browsers {
		b.Close()
	}
}

// initTracer must only be called within an iteration execution,
// as it requires access to the k6 TracerProvider which is only accessible
// through the VU.State during the iteration execution time span.
func (r *browserRegistry) initTracer() {
	// Use a sync.Once so the tracer is only initialized once
	// per VU, as that is the scope for the browser registry.
	r.tracerInit.Do(func() {
		tp := r.vu.State().TracerProvider
		if tp == nil {
			return
		}
		r.tracer = browsertrace.NewTracer(tp, r.tracesMetadata)
	})
}

// withTracer returns ctx wrapping the tracer, so that browser APIs can create their
// spans as children of the k6 iteration span carried by ctx. It also adds the traces
// metadata to that iteration span. If tracing is disabled it returns ctx unchanged.
func (r *browserRegistry) withTracer(ctx context.Context) context.Context {
	r.initTracer()
	if r.tracer == nil {
		return ctx
	}

	// The iteration span is owned and ended by k6. Setting the same attributes
	// again, e.g. on repeated connectOverCDP calls, is harmless.
	r.tracer.AddMetadata(oteltrace.SpanFromContext(ctx))

	return common.WithTracer(ctx, r.tracer)
}

// startConnectTrace returns a context carrying the tracer, so browser API calls made
// over a connectOverCDP browser are parented under the k6 iteration span in vuCtx.
func (r *browserRegistry) startConnectTrace(vuCtx context.Context) context.Context {
	return r.withTracer(vuCtx)
}

func isBrowserIter(vu k6modules.VU) bool {
	opts := k6ext.GetScenarioOpts(vu.Context(), vu)
	_, ok := opts["type"] // Check if browser type option is set
	return ok
}

func parseTracesMetadata(envLookup env.LookupFunc) (map[string]string, error) {
	var (
		ok bool
		v  string
		m  = make(map[string]string)
	)
	if v, ok = envLookup(env.TracesMetadata); !ok {
		return m, nil
	}

	for elem := range strings.SplitSeq(v, ",") {
		kv := strings.Split(elem, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("%q is not a valid key=value metadata", elem)
		}
		m[kv[0]] = kv[1]
	}

	return m, nil
}

type taskQueueRegistry struct {
	vu k6modules.VU

	tqMu sync.Mutex
	tq   map[string]*taskqueue.TaskQueue
}

func newTaskQueueRegistry(vu k6modules.VU) *taskQueueRegistry {
	return &taskQueueRegistry{
		vu:   vu,
		tqMu: sync.Mutex{},
		tq:   make(map[string]*taskqueue.TaskQueue),
	}
}

// get will retrieve the taskqueue associated with the given targetID. If one
// doesn't exist then a new taskqueue will be created.
//
// ctx must be the context from the VU, so that we can automatically close the
// taskqueue when the iteration ends.
func (t *taskQueueRegistry) get(ctx context.Context, targetID string) *taskqueue.TaskQueue {
	t.tqMu.Lock()
	defer t.tqMu.Unlock()

	tq := t.tq[targetID]
	if tq == nil {
		tq = taskqueue.New(t.vu.RegisterCallback)
		t.tq[targetID] = tq

		// We want to ensure that the taskqueue is closed when the context is
		// closed.
		go func(ctx context.Context) {
			<-ctx.Done()

			tq.Close()
		}(ctx)
	}

	return tq
}

func (t *taskQueueRegistry) close(targetID string) {
	t.tqMu.Lock()
	defer t.tqMu.Unlock()

	tq := t.tq[targetID]
	if tq != nil {
		tq.Close()
		delete(t.tq, targetID)
	}
}
