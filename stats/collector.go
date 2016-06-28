package stats

import (
	"sync"
	"time"
)

type Collector struct {
	Batch []Point
	mutex sync.Mutex
}

func (c *Collector) Add(p Point) {
	if p.Stat == nil || len(p.Values) == 0 {
		return
	}
	if p.Time.IsZero() {
		p.Time = time.Now()
	}

	c.mutex.Lock()
	c.Batch = append(c.Batch, p)
	c.mutex.Unlock()
}

func (c *Collector) drain() []Point {
	c.mutex.Lock()
	oldBatch := c.Batch
	c.Batch = nil
	c.mutex.Unlock()

	return oldBatch
}
