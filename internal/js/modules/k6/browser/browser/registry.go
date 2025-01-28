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

	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"go.k6.io/k6/internal/js/modules/k6/browser/chromium"
	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	browsertrace "go.k6.io/k6/internal/js/modules/k6/browser/trace"

	k6event "go.k6.io/k6/internal/event"
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

	tr             *tracesRegistry
	trInit         sync.Once
	tracesMetadata map[string]string

	mu sync.RWMutex
	m  map[int64]*common.Browser

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
		// If browser module is imported in the test, NewModuleInstance will be called for
		// every VU. Because on VU init stage we can not distinguish to which scenario it
		// belongs or access its options (because state is nil), we have to always subscribe
		// to each VU iter events, including VUs that do not make use of the browser in their
		// iterations.
		// Therefore, if we get an event that does not correspond to a browser iteration, then
		// skip this iteration. We can't just unsubscribe as the VU might be reused in a later
		// scenario that does have browser setup.
		// TODO try to maybe do this only once per scenario
		if !isBrowserIter(r.vu) {
			e.Done()
			continue
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

		switch e.Type {
		case k6event.IterStart:
			// Because VU.State is nil when browser registry is initialized,
			// we have to initialize traces registry on the first VU iteration
			// so we can get access to the k6 TracerProvider.
			r.initTracesRegistry()

			// Wrap the tracer into the VU context to make it accessible for the
			// other components during the iteration that inherit the VU context.
			//
			// All browser APIs should work with the vu context, and allow the
			// k6 iteration control its lifecycle.
			tracerCtx := common.WithTracer(r.vu.Context(), r.tr.tracer)
			tracedCtx := r.tr.startIterationTrace(tracerCtx, data)

			b, err := r.buildFn(ctx, tracedCtx)
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
			r.tr.endIterationTrace(data.Iteration)
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

	// Stop traces registry before calling e.Done()
	// so we avoid a race condition between active spans
	// being flushed and test exiting
	r.stopTracesRegistry()
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
}

// initTracesRegistry must only be called within an iteration execution,
// as it requires access to the k6 TracerProvider which is only accessible
// through the VU.State during the iteration execution time span.
func (r *browserRegistry) initTracesRegistry() {
	// Use a sync.Once so the traces registry is only initialized once
	// per VU, as that is the scope for both browser and traces registry.
	r.trInit.Do(func() {
		r.tr = newTracesRegistry(
			browsertrace.NewTracer(r.vu.State().TracerProvider, r.tracesMetadata),
		)
	})
}

func (r *browserRegistry) stopTracesRegistry() {
	// Because traces registry is initialized on iterStart event, it is not
	// initialized for the initial NewModuleInstance call, whose VU does not
	// execute any iteration.
	if r.tr != nil {
		r.tr.stop()
	}
}

func isBrowserIter(vu k6modules.VU) bool {
	opts := k6ext.GetScenarioOpts(vu.Context(), vu)
	_, ok := opts["type"] // Check if browser type option is set
	return ok
}

// trace represents a traces registry entry which holds the
// root span for the trace and a context that wraps that span.
type trace struct {
	ctx      context.Context
	rootSpan oteltrace.Span
}

// tracesRegistry holds the traces for all iterations of a single VU.
type tracesRegistry struct {
	tracer *browsertrace.Tracer

	mu sync.Mutex
	m  map[int64]*trace
}

func newTracesRegistry(tracer *browsertrace.Tracer) *tracesRegistry {
	return &tracesRegistry{
		tracer: tracer,
		m:      make(map[int64]*trace),
	}
}

func (r *tracesRegistry) startIterationTrace(ctx context.Context, data k6event.IterData) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()

	if t, ok := r.m[data.Iteration]; ok {
		return t.ctx
	}

	spanCtx, span := r.tracer.Start(ctx, "iteration", oteltrace.WithAttributes(
		attribute.Int64("test.iteration.number", data.Iteration),
		attribute.Int64("test.vu", int64(data.VUID)), //nolint:gosec
		attribute.String("test.scenario", data.ScenarioName),
	))

	r.m[data.Iteration] = &trace{
		ctx:      spanCtx,
		rootSpan: span,
	}

	return spanCtx
}

func (r *tracesRegistry) endIterationTrace(iter int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if t, ok := r.m[iter]; ok {
		t.rootSpan.End()
		delete(r.m, iter)
	}
}

func (r *tracesRegistry) stop() {
	// End all iteration traces
	r.mu.Lock()
	defer r.mu.Unlock()

	for k, v := range r.m {
		v.rootSpan.End()
		delete(r.m, k)
	}
}

// This is only used in a test. Avoids having to manipulate the mutex in the
// test itself.
func (r *tracesRegistry) iterationTracesCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.m)
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

	for _, elem := range strings.Split(v, ",") {
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
