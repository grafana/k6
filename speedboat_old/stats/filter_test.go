package stats

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFilterBlank(t *testing.T) {
	f := MakeFilter(nil, nil)
	assert.True(t, f.Check(Sample{Stat: &Stat{Name: "test"}}))
}

func TestFilterOnly(t *testing.T) {
	f := MakeFilter(nil, []string{"a"})
	assert.True(t, f.Check(Sample{Stat: &Stat{Name: "a"}}))
	assert.False(t, f.Check(Sample{Stat: &Stat{Name: "b"}}))
}

func TestFilterExclude(t *testing.T) {
	f := MakeFilter([]string{"a"}, nil)
	assert.False(t, f.Check(Sample{Stat: &Stat{Name: "a"}}))
	assert.True(t, f.Check(Sample{Stat: &Stat{Name: "b"}}))
}
