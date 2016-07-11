package stats

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestFormat(t *testing.T) {
	stat := Stat{Name: "test"}
	v := (JSONBackend{}).format(&Sample{
		Stat:   &stat,
		Tags:   Tags{"a": "b"},
		Values: Values{"value": 12345.0},
	})

	assert.Equal(t, "test", v["stat"])
	assert.Equal(t, time.Time{}, v["time"])

	assert.IsType(t, Tags{}, v["tags"])
	assert.Len(t, v["tags"], 1)
	assert.Equal(t, "b", v["tags"].(Tags)["a"])

	assert.IsType(t, Values{}, v["values"])
	assert.Len(t, v["values"], 1)
	assert.Equal(t, 12345.0, v["values"].(Values)["value"])
}

func TestFormatNilTagsBecomeEmptyMap(t *testing.T) {
	stat := Stat{Name: "test"}
	v := (JSONBackend{}).format(&Sample{
		Stat:   &stat,
		Values: Values{"value": 12345.0},
	})

	assert.IsType(t, Tags{}, v["tags"])
	assert.Len(t, v["tags"], 0)
}
