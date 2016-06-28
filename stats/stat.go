package stats

import (
	"time"
)

type StatType int
type StatIntent int

const (
	CounterType StatType = iota
	GaugeType
	HistogramType
)

const (
	DefaultIntent StatIntent = iota
	TimeIntent
)

type Stat struct {
	Name   string
	Type   StatType
	Intent StatIntent
}

func ApplyIntent(v float64, intent StatIntent) interface{} {
	switch intent {
	case TimeIntent:
		return time.Duration(v)
	default:
		return v
	}
}
