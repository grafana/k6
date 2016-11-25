package lib

import (
	"gopkg.in/guregu/null.v3"
)

type Options struct {
	Run      null.Bool   `json:"run"`
	VUs      null.Int    `json:"vus"`
	VUsMax   null.Int    `json:"vus-max"`
	Duration null.String `json:"duration"`

	Quit        null.Bool `json:"quit"`
	QuitOnTaint null.Bool `json:"quit-on-taint"`

	// Thresholds are JS snippets keyed by metrics.
	Thresholds map[string][]string `json:"thresholds"`
}

func (o Options) Apply(opts Options) Options {
	if opts.Run.Valid {
		o.Run = opts.Run
	}
	if opts.VUs.Valid {
		o.VUs = opts.VUs
	}
	if opts.VUsMax.Valid {
		o.VUsMax = opts.VUsMax
	}
	if opts.Duration.Valid {
		o.Duration = opts.Duration
	}
	if opts.Quit.Valid {
		o.Quit = opts.Quit
	}
	if opts.QuitOnTaint.Valid {
		o.QuitOnTaint = opts.QuitOnTaint
	}
	if len(opts.Thresholds) > 0 {
		o.Thresholds = opts.Thresholds
	}
	return o
}

func (o Options) SetAllValid(valid bool) Options {
	o.Run.Valid = valid
	o.VUs.Valid = valid
	o.VUsMax.Valid = valid
	o.Duration.Valid = valid
	o.Quit.Valid = valid
	o.QuitOnTaint.Valid = valid
	return o
}
