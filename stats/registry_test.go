package stats

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type testBackend struct {
	submitted []Point
}

func (b *testBackend) Submit(batches [][]Point) error {
	for _, batch := range batches {
		for _, p := range batch {
			b.submitted = append(b.submitted, p)
		}
	}

	return nil
}

func TestNewCollector(t *testing.T) {
	r := Registry{}
	c := r.NewCollector()
	assert.Equal(t, 1, len(r.collectors))
	assert.Equal(t, c, r.collectors[0])
}

func TestSubmit(t *testing.T) {
	backend := &testBackend{}
	r := Registry{
		Backends: []Backend{backend},
	}
	stat := Stat{Name: "test"}

	c1 := r.NewCollector()
	c1.Add(Point{Stat: &stat, Values: Value(1)})
	c1.Add(Point{Stat: &stat, Values: Value(2)})

	c2 := r.NewCollector()
	c2.Add(Point{Stat: &stat, Values: Value(3)})
	c2.Add(Point{Stat: &stat, Values: Value(4)})

	err := r.Submit()
	assert.NoError(t, err)
	assert.Len(t, backend.submitted, 4)
}
