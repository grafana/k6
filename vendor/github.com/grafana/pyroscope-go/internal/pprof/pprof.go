package pprof

import (
	"io"
	"runtime/pprof"
	"sync"
)

var c struct {
	sync.Mutex
	ref int64
	fn  func()
	Collector
}

type Collector interface {
	StartCPUProfile(w io.Writer) error
	StopCPUProfile()
}

func DefaultCollector() Collector { return defaultCollector{} }

type defaultCollector struct{}

func (c defaultCollector) StartCPUProfile(w io.Writer) error { return pprof.StartCPUProfile(w) }
func (c defaultCollector) StopCPUProfile()                   { pprof.StopCPUProfile() }

func StartCPUProfile(w io.Writer) error {
	c.Lock()
	defer c.Unlock()
	if c.Collector == nil {
		c.Collector = defaultCollector{}
	}
	err := c.StartCPUProfile(w)
	if err == nil {
		c.ref++
	}
	return err
}

func StopCPUProfile() {
	c.Lock()
	defer c.Unlock()
	c.StopCPUProfile()
	if c.ref--; c.ref == 0 && c.fn != nil {
		c.fn()
		c.fn = nil
	}
}

func SetCollector(collector Collector) {
	c.Lock()
	if c.ref == 0 {
		c.Collector = collector
		c.Unlock()
		return
	}
	ch := make(chan struct{})
	fn := c.fn
	c.fn = func() {
		if fn != nil {
			fn()
		}
		c.Collector = collector
		close(ch)
	}
	c.Unlock()
	<-ch
}

func ResetCollector() { SetCollector(nil) }
