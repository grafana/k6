// Package events is a minimal xk6 extension that demonstrates every event
// type exposed by go.k6.io/k6/v2/x/events. It logs each event as it fires
// (via the k6 logger) and keeps a running count of every event type seen,
// exposed to JS so the companion example.js script can assert on it.
package events

import (
	"sync"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/x/events"
)

func init() {
	modules.Register("k6/x/events", New())
}

// RootModule is the single, shared module instance for the whole test run.
// k6 calls NewModuleInstance on it once per VU that imports the module.
type RootModule struct {
	// subscribeOnce ensures the *global* events (Init, TestStart, TestEnd,
	// Exit) are subscribed to exactly once for the run, no matter how many
	// VUs import the module: since RootModule is shared, and global events
	// are each emitted exactly once, subscribing per-VU would log/count
	// every global event once per importing VU instead of once overall.
	subscribeOnce sync.Once

	// exit is closed once the global handler has processed Exit, signalling
	// every per-VU local handler goroutine to unsubscribe and stop.
	exit chan struct{}

	counts *eventCounts
}

// Instance is the per-VU module instance.
type Instance struct {
	vu   modules.VU
	root *RootModule
}

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &Instance{}
)

// New returns a new RootModule.
func New() *RootModule {
	return &RootModule{
		exit:   make(chan struct{}),
		counts: newEventCounts(),
	}
}

// NewModuleInstance implements modules.Module. It's called once per VU that
// `import`s k6/x/events.
func (rm *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	rm.subscribeOnce.Do(func() { rm.watchGlobalEvents(vu) })
	rm.watchLocalEvents(vu)

	return &Instance{vu: vu, root: rm}
}

// watchGlobalEvents subscribes to the run-wide events (Init, TestStart,
// TestEnd, Exit — each emitted exactly once per run).
func (rm *RootModule) watchGlobalEvents(vu modules.VU) {
	subID, eventsCh := vu.Events().Global.Subscribe(
		events.Init, events.TestStart, events.TestEnd, events.Exit,
	)
	logger := vu.InitEnv().Logger

	go func() {
		for evt := range eventsCh {
			rm.counts.inc(evt.Type)
			logGlobalEvent(logger, evt)
			evt.Done()
			if evt.Type == events.Exit {
				vu.Events().Global.Unsubscribe(subID)
				close(rm.exit) // tell local handlers to stop
				return
			}
		}
	}()
}

// watchLocalEvents subscribes to the per-VU events (IterStart, IterEnd —
// emitted once per iteration, for every VU that imports the module).
//
// It relies solely on rm.exit (closed by watchGlobalEvents once Exit has
// been processed) to know when to stop: vu.Context() is not a reliable
// liveness signal here, since k6 reassigns moduleVUImpl.ctx repeatedly over
// a VU's life (per scenario activation, per iteration) and the context
// captured at subscribe time — right after this VU was instantiated — is
// typically already canceled by the time this goroutine starts running, long
// before the VU's real iterations begin.
func (rm *RootModule) watchLocalEvents(vu modules.VU) {
	subID, eventsCh := vu.Events().Local.Subscribe(events.IterStart, events.IterEnd)
	logger := vu.InitEnv().Logger

	go func() {
		for {
			select {
			case evt, ok := <-eventsCh:
				if !ok {
					return
				}
				rm.counts.inc(evt.Type)
				logLocalEvent(logger, evt)
				evt.Done()
			case <-rm.exit:
				vu.Events().Local.Unsubscribe(subID)
				return
			}
		}
	}()
}

func logGlobalEvent(logger logrus.FieldLogger, evt *events.Event) {
	if evt.Type == events.Exit {
		var errMsg any
		if data, ok := evt.Data.(*events.ExitData); ok && data != nil && data.Error != nil {
			errMsg = data.Error.Error()
		}
		logger.WithField("error", errMsg).Infof("[k6/x/events] %s", evt.Type)
		return
	}
	logger.Infof("[k6/x/events] %s", evt.Type)
}

func logLocalEvent(logger logrus.FieldLogger, evt *events.Event) {
	data, _ := evt.Data.(events.IterData)
	logger.WithFields(logrus.Fields{
		"vuID":      data.VUID,
		"iteration": data.Iteration,
		"scenario":  data.ScenarioName,
	}).Infof("[k6/x/events] %s", evt.Type)
}

// Exports implements modules.Instance.
func (i *Instance) Exports() modules.Exports {
	return modules.Exports{Default: i}
}

// Counts returns how many times each event type has fired so far. Exposed
// to JS purely so example.js can assert that every event type was observed.
func (i *Instance) Counts() map[string]uint64 {
	return i.root.counts.snapshot()
}

// eventCounts is a concurrency-safe tally: it's written from the single
// global-event goroutine and from one local-event goroutine per VU, so
// access must be synchronized.
type eventCounts struct {
	mu     sync.Mutex
	counts map[string]uint64
}

func newEventCounts() *eventCounts {
	return &eventCounts{counts: make(map[string]uint64)}
}

func (c *eventCounts) inc(t events.Type) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts[t.String()]++
}

func (c *eventCounts) snapshot() map[string]uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]uint64, len(c.counts))
	for k, v := range c.counts {
		out[k] = v
	}
	return out
}
