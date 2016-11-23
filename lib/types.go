package lib

import (
	"gopkg.in/guregu/null.v3"
)

type Options struct {
	VUs      null.Int
	VUsMax   null.Int
	Duration null.String

	// Thresholds are JS snippets keyed by metrics.
	Thresholds map[string][]string
}
