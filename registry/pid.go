package registry

import "sync"

// PidRegistry keeps track of the launched browser process IDs.
type PidRegistry struct {
	mu  sync.RWMutex
	ids []int
}

// RegisterPid registers the launched browser process ID.
func (r *PidRegistry) RegisterPid(pid int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ids = append(r.ids, pid)
}

// Pids returns the launched browser process IDs.
func (r *PidRegistry) Pids() []int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pids := make([]int, len(r.ids))
	copy(pids, r.ids)

	return pids
}
