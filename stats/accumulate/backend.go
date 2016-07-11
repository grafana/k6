package accumulate

import (
	"fmt"
	"github.com/loadimpact/speedboat/stats"
	"sort"
	"strings"
	"sync"
)

type StatTree map[StatTreeKey]*StatTreeNode

type StatTreeKey struct {
	Tag   string
	Value interface{}
}

type StatTreeNode struct {
	Stat     *stats.Stat
	Substats *StatTree
}

type Backend struct {
	Data    map[*stats.Stat]map[string]*Dimension
	Only    map[string]bool
	Exclude map[string]bool
	GroupBy []string

	vstats      map[*stats.Stat]*StatTree
	submitMutex sync.Mutex
}

func New() *Backend {
	return &Backend{
		Data:    make(map[*stats.Stat]map[string]*Dimension),
		Exclude: make(map[string]bool),
		Only:    make(map[string]bool),
		vstats:  make(map[*stats.Stat]*StatTree),
	}
}

func (b *Backend) getVStat(stat *stats.Stat, tags stats.Tags) *stats.Stat {
	tree := b.vstats[stat]
	if tree == nil {
		tmp := make(StatTree)
		tree = &tmp
		b.vstats[stat] = tree
	}

	ret := stat
	for n, tag := range b.GroupBy {
		val, ok := tags[tag]
		if !ok {
			continue
		}

		key := StatTreeKey{Tag: tag, Value: val}
		node := (*tree)[key]
		if node == nil {
			tagStrings := make([]string, 0, n)
			for i := 0; i <= n; i++ {
				t := b.GroupBy[i]
				v, ok := tags[t]
				if !ok {
					continue
				}
				tagStrings = append(tagStrings, fmt.Sprintf("%s: %v", t, v))
			}

			name := stat.Name
			if len(tagStrings) > 0 {
				name = fmt.Sprintf("%s{%s}", name, strings.Join(tagStrings, ", "))
			}

			substats := make(StatTree)
			node = &StatTreeNode{
				Stat: &stats.Stat{
					Name:   name,
					Type:   stat.Type,
					Intent: stat.Intent,
				},
				Substats: &substats,
			}
			(*tree)[key] = node
		}

		ret = node.Stat
	}

	return ret
}

func (b *Backend) Submit(batches [][]stats.Sample) error {
	b.submitMutex.Lock()

	hasOnly := len(b.Only) > 0

	for _, batch := range batches {
		for _, s := range batch {
			if hasOnly && !b.Only[s.Stat.Name] {
				continue
			}

			if b.Exclude[s.Stat.Name] {
				continue
			}

			stat := b.getVStat(s.Stat, s.Tags)
			dimensions, ok := b.Data[stat]
			if !ok {
				dimensions = make(map[string]*Dimension)
				b.Data[stat] = dimensions
			}

			for dname, val := range s.Values {
				dim, ok := dimensions[dname]
				if !ok {
					dim = &Dimension{}
					dimensions[dname] = dim
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
