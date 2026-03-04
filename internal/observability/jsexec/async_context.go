// Package jsexec contains JS execution observability helpers.
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

func (t *sobekAsyncContextTracker) Grab() any {
	t.mu.Lock()
	defer t.mu.Unlock()
	return cloneLabels(t.current)
}

func (t *sobekAsyncContextTracker) Resumed(trackingObject any) {
	labels, _ := trackingObject.(map[string]string)
	t.setCurrent(labels)
	pprof.SetGoroutineLabels(pprof.WithLabels(context.Background(), LabelsFromMap(labels)))
}

func (t *sobekAsyncContextTracker) Exited() {
	pprof.SetGoroutineLabels(context.Background())
}

// InstallRuntimeAsyncContextTracker wires Sobek async context propagation on a runtime.
func InstallRuntimeAsyncContextTracker(rt *sobek.Runtime, baseLabels map[string]string) {
	if rt == nil {
		return
	}
	m := activeManager()
	if m == nil {
		return
	}
	tr := &sobekAsyncContextTracker{}
	tr.setCurrent(baseLabels)
	rt.SetAsyncContextTracker(tr)
	m.asyncTrackersMu.Lock()
	m.asyncTrackers[rt] = tr
	m.asyncTrackersMu.Unlock()
}

// UpdateRuntimeAsyncLabels updates the current async context labels for a runtime.
func UpdateRuntimeAsyncLabels(rt *sobek.Runtime, labels map[string]string) {
	if rt == nil {
		return
	}
	m := activeManager()
	if m == nil {
		return
	}
	m.asyncTrackersMu.RLock()
	tr := m.asyncTrackers[rt]
	m.asyncTrackersMu.RUnlock()
	if tr == nil {
		return
	}
	tr.setCurrent(labels)
}
