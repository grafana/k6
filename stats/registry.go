package stats

import (
	"sync"
)

type Registry struct {
	Backends []Backend

	collectors []*Collector
	mutex      sync.Mutex
}

func (r *Registry) NewCollector() *Collector {
	collector := &Collector{}

	r.mutex.Lock()
	r.collectors = append(r.collectors, collector)
	r.mutex.Unlock()

	return collector
}

func (r *Registry) Submit() error {
	batches := make([][]Point, 0, len(r.collectors))
	for _, collector := range r.collectors {
		batch := collector.drain()
		batches = append(batches, batch)
	}

	for _, backend := range r.Backends {
		if err := backend.Submit(batches); err != nil {
			return err
		}
	}

	return nil
}
