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

package stats

import (
	"encoding/json"
	"time"

	"github.com/dop251/goja"
	"github.com/pkg/errors"
)

const jsEnvSrc = `
function p(pct) {
	return __sink__.P(pct/100.0);
};
`

var jsEnv *goja.Program

func init() {
	pgm, err := goja.Compile("__env__", jsEnvSrc, true)
	if err != nil {
		panic(err)
	}
	jsEnv = pgm
}

type Threshold struct {
	Source      string
	Failed      bool
	AbortOnFail bool

	pgm *goja.Program
	rt  *goja.Runtime
}

func NewThreshold(src string, rt *goja.Runtime, abortOnFail bool) (*Threshold, error) {
	pgm, err := goja.Compile("__threshold__", src, true)
	if err != nil {
		return nil, err
	}

	return &Threshold{
		Source:      src,
		AbortOnFail: abortOnFail,
		pgm:         pgm,
		rt:          rt,
	}, nil
}

func (t Threshold) RunNoTaint() (bool, error) {
	v, err := t.rt.RunProgram(t.pgm)
	if err != nil {
		return false, err
	}
	return v.ToBoolean(), nil
}

func (t *Threshold) Run() (bool, error) {
	b, err := t.RunNoTaint()
	if !b {
		t.Failed = true
	}
	return b, err
}

type ThresholdConfig struct {
	Threshold    string `json:"threshold"`
	AbortOnTaint bool   `json:"abortOnTaint"`
}

//used internally for JSON marshalling
type rawThresholdConfig ThresholdConfig

func (tc *ThresholdConfig) UnmarshalJSON(data []byte) error {
	//shortcircuit unmarshalling for simple string format
	if err := json.Unmarshal(data, &tc.Threshold); err == nil {
		return nil
	}

	rawConfig := (*rawThresholdConfig)(tc)
	return json.Unmarshal(data, rawConfig)
}

func (tc ThresholdConfig) MarshalJSON() ([]byte, error) {
	if tc.AbortOnTaint {
		return json.Marshal(rawThresholdConfig(tc))
	}
	return json.Marshal(tc.Threshold)
}

type Thresholds struct {
	Runtime    *goja.Runtime
	Thresholds []*Threshold
	Abort      bool
}

func NewThresholds(sources []string) (Thresholds, error) {
	tcs := make([]ThresholdConfig, len(sources))
	for i, source := range sources {
		tcs[i].Threshold = source
	}

	return NewThresholdsWithConfig(tcs)
}

func NewThresholdsWithConfig(configs []ThresholdConfig) (Thresholds, error) {
	rt := goja.New()
	if _, err := rt.RunProgram(jsEnv); err != nil {
		return Thresholds{}, errors.Wrap(err, "builtin")
	}

	ts := make([]*Threshold, len(configs))
	for i, config := range configs {
		t, err := NewThreshold(config.Threshold, rt, config.AbortOnTaint)
		if err != nil {
			return Thresholds{}, errors.Wrapf(err, "%d", i)
		}
		ts[i] = t
	}

	return Thresholds{rt, ts, false}, nil
}

func (ts *Thresholds) UpdateVM(sink Sink, t time.Duration) error {
	ts.Runtime.Set("__sink__", sink)
	f := sink.Format(t)
	for k, v := range f {
		ts.Runtime.Set(k, v)
	}
	return nil
}

func (ts *Thresholds) RunAll() (bool, error) {
	succ := true
	for i, th := range ts.Thresholds {
		b, err := th.Run()
		if err != nil {
			return false, errors.Wrapf(err, "%d", i)
		}
		if !b {
			succ = false
			if !ts.Abort && th.AbortOnFail {
				ts.Abort = true
			}
		}
	}
	return succ, nil
}

func (ts *Thresholds) Run(sink Sink, t time.Duration) (bool, error) {
	if err := ts.UpdateVM(sink, t); err != nil {
		return false, err
	}
	return ts.RunAll()
}

func (ts *Thresholds) UnmarshalJSON(data []byte) error {
	var configs []ThresholdConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return err
	}
	newts, err := NewThresholdsWithConfig(configs)
	if err != nil {
		return err
	}
	*ts = newts
	return nil
}

func (ts Thresholds) MarshalJSON() ([]byte, error) {
	configs := make([]ThresholdConfig, len(ts.Thresholds))
	for i, t := range ts.Thresholds {
		configs[i].Threshold = t.Source
		configs[i].AbortOnTaint = t.AbortOnFail
	}
	return json.Marshal(configs)
}
