package jsexec

import (
	"context"
	"runtime/pprof"
	"sync"

	"github.com/grafana/sobek"
)

type sobekAsyncContextTracker struct {
	mu      sync.Mutex
	current map[string]string
}

func cloneLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	out := make(map[string]string, len(labels))
	for k, v := range labels {
		if k == "" || v == "" {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (t *sobekAsyncContextTracker) setCurrent(labels map[string]string) {
	t.mu.Lock()
	t.current = cloneLabels(labels)
	t.mu.Unlock()
}

func (t *sobekAsyncContextTracker) Grab() interface{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return cloneLabels(t.current)
}

func (t *sobekAsyncContextTracker) Resumed(trackingObject interface{}) {
	labels, _ := trackingObject.(map[string]string)
	t.setCurrent(labels)
	pprof.SetGoroutineLabels(pprof.WithLabels(context.Background(), LabelsFromMap(labels)))
}

func (t *sobekAsyncContextTracker) Exited() {
	pprof.SetGoroutineLabels(context.Background())
}

var runtimeTrackers = struct {
	mu sync.RWMutex
	m  map[*sobek.Runtime]*sobekAsyncContextTracker
}{
	m: make(map[*sobek.Runtime]*sobekAsyncContextTracker),
}

// InstallRuntimeAsyncContextTracker wires Sobek async context propagation on a runtime.
func InstallRuntimeAsyncContextTracker(rt *sobek.Runtime, baseLabels map[string]string) {
	if rt == nil {
		return
	}
	tr := &sobekAsyncContextTracker{}
	tr.setCurrent(baseLabels)
	rt.SetAsyncContextTracker(tr)
	runtimeTrackers.mu.Lock()
	runtimeTrackers.m[rt] = tr
	runtimeTrackers.mu.Unlock()
}

// UpdateRuntimeAsyncLabels updates the current async context labels for a runtime.
func UpdateRuntimeAsyncLabels(rt *sobek.Runtime, labels map[string]string) {
	if rt == nil {
		return
	}
	runtimeTrackers.mu.RLock()
	tr := runtimeTrackers.m[rt]
	runtimeTrackers.mu.RUnlock()
	if tr == nil {
		return
	}
	tr.setCurrent(labels)
}
