package stats

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type testBackend struct {
	submitted []Sample
}

func (b *testBackend) Submit(batches [][]Sample) error {
	for _, batch := range batches {
		for _, s := range batch {
			b.submitted = append(b.submitted, s)
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
	c1.Add(Sample{Stat: &stat, Values: Value(1)})
	c1.Add(Sample{Stat: &stat, Values: Value(2)})

	c2 := r.NewCollector()
	c2.Add(Sample{Stat: &stat, Values: Value(3)})
	c2.Add(Sample{Stat: &stat, Values: Value(4)})

	err := r.Submit()
	assert.NoError(t, err)
	assert.Len(t, backend.submitted, 4)
}

func TestSubmitExtraTagsNilTags(t *testing.T) {
	backend := &testBackend{}
	r := Registry{Backends: []Backend{backend}, ExtraTags: Tags{"key": "value"}}
	stat := Stat{Name: "test"}

	c1 := r.NewCollector()
	c1.Add(Sample{Stat: &stat, Values: Value(1)})
	assert.NoError(t, r.Submit())
	assert.Equal(t, "value", backend.submitted[0].Tags["key"])
}

func TestSubmitExtraTags(t *testing.T) {
	backend := &testBackend{}
	r := Registry{Backends: []Backend{backend}, ExtraTags: Tags{"key": "value"}}
	stat := Stat{Name: "test"}

	c1 := r.NewCollector()
	c1.Add(Sample{Stat: &stat, Values: Value(1), Tags: Tags{"a": "b"}})
	assert.NoError(t, r.Submit())
	assert.Equal(t, "value", backend.submitted[0].Tags["key"])
}
