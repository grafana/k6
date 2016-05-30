package speedboat

import (
	"github.com/rcrowley/go-metrics"
)

var (
	Registry = metrics.NewPrefixedRegistry("speedboat.")
)
