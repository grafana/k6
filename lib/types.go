/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

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

	MaxRedirects null.Int `json:"max-redirects"`

	Thresholds map[string][]*Threshold `json:"thresholds"`
}

type SourceData struct {
	SrcData  []byte
	Filename string
	SrcType  string
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
	if opts.MaxRedirects.Valid {
		o.MaxRedirects = opts.MaxRedirects
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
