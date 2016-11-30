package lib

import (
	"encoding/json"
	"github.com/robertkrimen/otto"
	"gopkg.in/guregu/null.v3"
)

type Options struct {
	Paused   null.Bool   `json:"paused"`
	VUs      null.Int    `json:"vus"`
	VUsMax   null.Int    `json:"vus-max"`
	Duration null.String `json:"duration"`

	Linger       null.Bool  `json:"linger"`
	AbortOnTaint null.Bool  `json:"abort-on-taint"`
	Acceptance   null.Float `json:"acceptance"`

	Thresholds map[string][]*Threshold `json:"thresholds"`
}

func (o Options) Apply(opts Options) Options {
	if opts.Paused.Valid {
		o.Paused = opts.Paused
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
	if opts.Linger.Valid {
		o.Linger = opts.Linger
	}
	if opts.AbortOnTaint.Valid {
		o.AbortOnTaint = opts.AbortOnTaint
	}
	if opts.Acceptance.Valid {
		o.Acceptance = opts.Acceptance
	}
	if opts.Thresholds != nil {
		o.Thresholds = opts.Thresholds
	}
	return o
}

func (o Options) SetAllValid(valid bool) Options {
	o.Paused.Valid = valid
	o.VUs.Valid = valid
	o.VUsMax.Valid = valid
	o.Duration.Valid = valid
	o.Linger.Valid = valid
	o.AbortOnTaint.Valid = valid
	return o
}

type Threshold struct {
	Source string
	Script *otto.Script
	Failed bool
}

func (t Threshold) String() string {
	return t.Source
}

func (t Threshold) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Source)
}

func (t *Threshold) UnmarshalJSON(data []byte) error {
	var src string
	if err := json.Unmarshal(data, &src); err != nil {
		return err
	}
	t.Source = src
	t.Script = nil
	t.Failed = false
	return nil
}
