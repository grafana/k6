package accumulate

import (
	"github.com/loadimpact/speedboat/stats"
	"sort"
	"sync"
)

type Backend struct {
	Data    map[*stats.Stat]map[*string]*Dimension
	Only    map[string]bool
	Exclude map[string]bool

	interned    map[string]*string
	submitMutex sync.Mutex
}

func New() *Backend {
	return &Backend{
		Data:     make(map[*stats.Stat]map[*string]*Dimension),
		Exclude:  make(map[string]bool),
		Only:     make(map[string]bool),
		interned: make(map[string]*string),
	}
}

func (b *Backend) Get(stat *stats.Stat, dname string) *Dimension {
	dimensions, ok := b.Data[stat]
	if !ok {
		return nil
	}

	return dimensions[b.interned[dname]]
}

func (b *Backend) Submit(batches [][]stats.Point) error {
	b.submitMutex.Lock()

	hasOnly := len(b.Only) > 0

	for _, batch := range batches {
		for _, p := range batch {
			if hasOnly && !b.Only[p.Stat.Name] {
				continue
			}

			if b.Exclude[p.Stat.Name] {
				continue
			}

			dimensions, ok := b.Data[p.Stat]
			if !ok {
				dimensions = make(map[*string]*Dimension)
				b.Data[p.Stat] = dimensions
			}

			for dname, val := range p.Values {
				interned, ok := b.interned[dname]
				if !ok {
					interned = &dname
					b.interned[dname] = interned
				}

				dim, ok := dimensions[interned]
				if !ok {
					dim = &Dimension{}
					dimensions[interned] = dim
				}

				dim.Values = append(dim.Values, val)
				dim.Last = val
				dim.dirty = true
			}
		}
	}

	for _, dimensions := range b.Data {
		for _, dim := range dimensions {
			if dim.dirty {
				sort.Float64s(dim.Values)
				dim.dirty = false
			}
		}
	}

	b.submitMutex.Unlock()

	return nil
}
