package stats

import (
	"sync"
	"time"
)

type Collector struct {
	Batch []Sample
	mutex sync.Mutex
}

func (c *Collector) Add(s Sample) {
	if s.Stat == nil || len(s.Values) == 0 {
		return
	}
	if s.Time.IsZero() {
		s.Time = time.Now()
	}

	c.mutex.Lock()
	c.Batch = append(c.Batch, s)
	c.mutex.Unlock()
}

func (c *Collector) drain() []Sample {
	c.mutex.Lock()
	oldBatch := c.Batch
	c.Batch = nil
	c.mutex.Unlock()

	return oldBatch
}
