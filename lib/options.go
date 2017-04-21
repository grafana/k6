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
	"time"

	"gopkg.in/guregu/null.v3"
)

type Duration time.Duration

func (d *Duration) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	v, err := time.ParseDuration(str)
	if err != nil {
		return err
	}

	*d = Duration(v)

	return nil
}

type Options struct {
	Paused     null.Bool   `json:"paused"`
	VUs        null.Int    `json:"vus"`
	VUsMax     null.Int    `json:"vusMax"`
	Duration   null.String `json:"duration"`
	Iterations null.Int    `json:"iterations"`
	Stages     []Stage     `json:"stages"`

	Linger        null.Bool `json:"linger"`
	NoUsageReport null.Bool `json:"noUsageReport"`

	MaxRedirects          null.Int  `json:"maxRedirects"`
	InsecureSkipTLSVerify null.Bool `json:"insecureSkipTLSVerify"`

	Thresholds map[string]Thresholds `json:"thresholds"`
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
	if opts.Iterations.Valid {
		o.Iterations = opts.Iterations
	}
	if opts.Stages != nil {
		o.Stages = opts.Stages
	}
	if opts.Linger.Valid {
		o.Linger = opts.Linger
	}
	if opts.NoUsageReport.Valid {
		o.NoUsageReport = opts.NoUsageReport
	}
	if opts.MaxRedirects.Valid {
		o.MaxRedirects = opts.MaxRedirects
	}
	if opts.InsecureSkipTLSVerify.Valid {
		o.InsecureSkipTLSVerify = opts.InsecureSkipTLSVerify
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
	o.NoUsageReport.Valid = valid
	return o
}
