package accumulate

import (
	"github.com/loadimpact/speedboat/stats"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetNonexistent(t *testing.T) {
	b := New()
	stat := stats.Stat{Name: "test"}
	assert.Nil(t, b.Get(&stat, "value"))
}

func TestGet(t *testing.T) {
	b := New()
	stat := stats.Stat{Name: "test"}
	b.Submit([][]stats.Point{
		[]stats.Point{
			stats.Point{Stat: &stat, Values: stats.Values{"value": 1}},
		},
	})

	assert.NotNil(t, b.Get(&stat, "value"))
}

func TestSubmitInternsNames(t *testing.T) {
	b := New()
	stat := stats.Stat{Name: "test"}
	b.Submit([][]stats.Point{
		[]stats.Point{
			stats.Point{Stat: &stat, Values: stats.Values{"value": 1}},
			stats.Point{Stat: &stat, Values: stats.Values{"value": 2}},
			stats.Point{Stat: &stat, Values: stats.Values{"value": 3}},
		},
	})
	assert.Len(t, b.interned, 1)
	assert.Len(t, b.Data, 1)
	assert.Len(t, b.Data[&stat], 1)
	assert.Contains(t, b.Data[&stat], b.interned["value"])
}

func TestSubmitSortsValues(t *testing.T) {
	b := New()
	stat := stats.Stat{Name: "test"}
	b.Submit([][]stats.Point{
		[]stats.Point{
			stats.Point{Stat: &stat, Values: stats.Values{"value": 3}},
			stats.Point{Stat: &stat, Values: stats.Values{"value": 1}},
			stats.Point{Stat: &stat, Values: stats.Values{"value": 2}},
		},
	})

	dim := b.Get(&stat, "value")
	assert.EqualValues(t, []float64{1, 2, 3}, dim.Values)
	assert.False(t, dim.dirty)
}

func TestSubmitSortsValuesContinously(t *testing.T) {
	b := New()
	stat := stats.Stat{Name: "test"}
	b.Submit([][]stats.Point{
		[]stats.Point{
			stats.Point{Stat: &stat, Values: stats.Values{"value": 3}},
			stats.Point{Stat: &stat, Values: stats.Values{"value": 1}},
			stats.Point{Stat: &stat, Values: stats.Values{"value": 2}},
		},
	})
	b.Submit([][]stats.Point{
		[]stats.Point{
			stats.Point{Stat: &stat, Values: stats.Values{"value": 6}},
			stats.Point{Stat: &stat, Values: stats.Values{"value": 5}},
			stats.Point{Stat: &stat, Values: stats.Values{"value": 4}},
		},
	})

	dim := b.Get(&stat, "value")
	assert.EqualValues(t, []float64{1, 2, 3, 4, 5, 6}, dim.Values)
	assert.False(t, dim.dirty)
}

func TestSubmitKeepsLast(t *testing.T) {
	b := New()
	stat := stats.Stat{Name: "test"}
	b.Submit([][]stats.Point{
		[]stats.Point{
			stats.Point{Stat: &stat, Values: stats.Values{"value": 3}},
			stats.Point{Stat: &stat, Values: stats.Values{"value": 1}},
			stats.Point{Stat: &stat, Values: stats.Values{"value": 2}},
		},
	})
	assert.Equal(t, float64(2), b.Get(&stat, "value").Last)
}

func TestSubmitIgnoresExcluded(t *testing.T) {
	b := New()
	stat1 := stats.Stat{Name: "test"}
	stat2 := stats.Stat{Name: "test2"}
	b.Exclude["test2"] = true
	b.Submit([][]stats.Point{
		[]stats.Point{
			stats.Point{Stat: &stat1, Values: stats.Values{"value": 3}},
			stats.Point{Stat: &stat1, Values: stats.Values{"value": 1}},
			stats.Point{Stat: &stat2, Values: stats.Values{"value": 2}},
		},
	})
	assert.Len(t, b.Data, 1)
}

func TestSubmitIgnoresNotInOnly(t *testing.T) {
	b := New()
	stat1 := stats.Stat{Name: "test"}
	stat2 := stats.Stat{Name: "test2"}
	b.Only["test2"] = true
	b.Submit([][]stats.Point{
		[]stats.Point{
			stats.Point{Stat: &stat1, Values: stats.Values{"value": 3}},
			stats.Point{Stat: &stat1, Values: stats.Values{"value": 1}},
			stats.Point{Stat: &stat2, Values: stats.Values{"value": 2}},
		},
	})
	assert.Len(t, b.Data, 1)
}

func TestGetVStatDefault(t *testing.T) {
	b := New()
	stat := stats.Stat{Name: "test"}
	assert.Equal(t, &stat, b.getVStat(&stat, stats.Tags{}))
}

func TestGetVStatNoMatch(t *testing.T) {
	b := New()
	b.GroupBy = []string{"no-match"}
	stat := stats.Stat{Name: "test"}
	assert.Equal(t, &stat, b.getVStat(&stat, stats.Tags{}))
}

func TestGetVStatOneTag(t *testing.T) {
	b := New()
	b.GroupBy = []string{"tag"}
	stat := stats.Stat{Name: "test"}
	vstat := b.getVStat(&stat, stats.Tags{"tag": "value"})
	assert.NotNil(t, vstat)
	assert.Equal(t, "test{tag: value}", vstat.Name)
}

func TestGetVStatTwoTags(t *testing.T) {
	b := New()
	b.GroupBy = []string{"tag", "blah"}
	stat := stats.Stat{Name: "test"}
	vstat := b.getVStat(&stat, stats.Tags{"tag": "value", "blah": 12345})
	assert.NotNil(t, vstat)
	assert.Equal(t, "test{tag: value, blah: 12345}", vstat.Name)
}

func TestGetVStatTwoTagsOneMiss(t *testing.T) {
	b := New()
	b.GroupBy = []string{"tag", "weh", "blah"}
	stat := stats.Stat{Name: "test"}
	vstat := b.getVStat(&stat, stats.Tags{"tag": "value", "blah": 12345})
	assert.NotNil(t, vstat)
	assert.Equal(t, "test{tag: value, blah: 12345}", vstat.Name)
}
