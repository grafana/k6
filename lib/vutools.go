package lib

import (
	"errors"
	"golang.org/x/net/context"
	"sync"
)

type VUPool struct {
	New func() (VU, error)

	vus   []VU
	mutex sync.Mutex
}

func (p *VUPool) Get() (VU, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	l := len(p.vus)
	if l == 0 {
		return p.New()
	}

	vu := p.vus[l-1]
	p.vus = p.vus[:l-1]
	return vu, nil
}

func (p *VUPool) Put(vu VU) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.vus = append(p.vus, vu)
}

func (p *VUPool) Count() int {
	return len(p.vus)
}

type VUGroup struct {
	Pool    VUPool
	RunOnce func(ctx context.Context, vu VU)

	ctx       context.Context
	cancelAll context.CancelFunc
	cancelers []context.CancelFunc
}

func (g *VUGroup) Start(ctx context.Context) {
	g.ctx, g.cancelAll = context.WithCancel(ctx)
}

func (g *VUGroup) Stop() {
	g.cancelAll()
	g.ctx = nil
}

func (g *VUGroup) Scale(count int) error {
	if g.ctx == nil {
		panic(errors.New("Group not running"))
	}

	for len(g.cancelers) < count {
		vu, err := g.Pool.Get()
		if err != nil {
			return err
		}

		id := int64(len(g.cancelers) + 1)
		if err := vu.Reconfigure(id); err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(g.ctx)
		g.cancelers = append(g.cancelers, cancel)

		go g.runVU(ctx, vu)
	}

	for len(g.cancelers) > count {
		g.cancelers[len(g.cancelers)-1]()
		g.cancelers = g.cancelers[:len(g.cancelers)-1]
	}

	return nil
}

func (g *VUGroup) runVU(ctx context.Context, vu VU) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			g.RunOnce(ctx, vu)
		}
	}
}
