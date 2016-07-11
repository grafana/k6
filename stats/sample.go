package stats

import (
	"time"
)

type Tags map[string]interface{}
type Values map[string]float64

type Sample struct {
	Stat   *Stat
	Time   time.Time
	Tags   Tags
	Values Values
}

func Value(val float64) Values {
	return Values{"value": val}
}
