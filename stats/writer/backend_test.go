package writer

import (
	"github.com/loadimpact/speedboat/stats"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestFormat(t *testing.T) {
	stat := stats.Stat{Name: "test"}
	v := (Backend{}).Format(&stats.Sample{
		Stat:   &stat,
		Tags:   stats.Tags{"a": "b"},
		Values: stats.Values{"value": 12345.0},
	})

	assert.Equal(t, "test", v["stat"])
	assert.Equal(t, time.Time{}, v["time"])

	assert.IsType(t, stats.Tags{}, v["tags"])
	assert.Len(t, v["tags"], 1)
	assert.Equal(t, "b", v["tags"].(stats.Tags)["a"])

	assert.IsType(t, stats.Values{}, v["values"])
	assert.Len(t, v["values"], 1)
	assert.Equal(t, 12345.0, v["values"].(stats.Values)["value"])
}

func TestFormatNilTagsBecomeEmptyMap(t *testing.T) {
	stat := stats.Stat{Name: "test"}
	v := (Backend{}).Format(&stats.Sample{
		Stat:   &stat,
		Values: stats.Values{"value": 12345.0},
	})

	assert.IsType(t, stats.Tags{}, v["tags"])
	assert.Len(t, v["tags"], 0)
}
