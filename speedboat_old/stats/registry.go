package stats

import (
	"sync"
)

type Registry struct {
	Backends  []Backend
	ExtraTags Tags

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
	r.mutex.Lock()
	defer r.mutex.Unlock()

	batches := make([][]Sample, 0, len(r.collectors))
	for _, collector := range r.collectors {
		batch := collector.drain()
		batches = append(batches, batch)
	}

	if len(r.ExtraTags) > 0 {
		for _, batch := range batches {
			for i, p := range batch {
				if p.Tags == nil {
					p.Tags = r.ExtraTags
					batch[i] = p
				} else {
					for key, val := range r.ExtraTags {
						p.Tags[key] = val
					}
				}
			}
		}
	}

	for _, backend := range r.Backends {
		if err := backend.Submit(batches); err != nil {
			return err
		}
	}

	return nil
}
