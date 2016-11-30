package lib

import (
	"encoding/json"
	"github.com/robertkrimen/otto"
	"gopkg.in/guregu/null.v3"
)

type Options struct {
	Run      null.Bool   `json:"run"`
	VUs      null.Int    `json:"vus"`
	VUsMax   null.Int    `json:"vus-max"`
	Duration null.String `json:"duration"`

	Quit        null.Bool  `json:"quit"`
	QuitOnTaint null.Bool  `json:"quit-on-taint"`
	Acceptance  null.Float `json:"acceptance"`

	Thresholds map[string][]*Threshold `json:"thresholds"`
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
	if opts.Acceptance.Valid {
		o.Acceptance = opts.Acceptance
	}
	if opts.Thresholds != nil {
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
