package stats

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAdd(t *testing.T) {
	c := Collector{}
	stat := Stat{Name: "test"}
	c.Add(Point{Stat: &stat, Values: Values{"value": 12345}})
	assert.Equal(t, 1, len(c.Batch))
	assert.Equal(t, &stat, c.Batch[0].Stat)
	assert.Equal(t, 12345.0, c.Batch[0].Values["value"])
}

func TestAddNoStat(t *testing.T) {
	c := Collector{}
	c.Add(Point{Values: Values{"value": 12345}})
	assert.Equal(t, 0, len(c.Batch))
}

func TestAddNoValues(t *testing.T) {
	c := Collector{}
	c.Add(Point{Stat: &Stat{Name: "test"}})
	assert.Equal(t, 0, len(c.Batch))
}

func TestAddFixesTime(t *testing.T) {
	c := Collector{}
	c.Add(Point{Stat: &Stat{Name: "test"}, Values: Values{"value": 12345}})
	assert.False(t, c.Batch[0].Time.IsZero())
}

func TestDrain(t *testing.T) {
	c := Collector{}
	c.Add(Point{Stat: &Stat{Name: "test"}, Values: Values{"value": 12345}})
	batch := c.drain()
	assert.Equal(t, 1, len(batch))
}
