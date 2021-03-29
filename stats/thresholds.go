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

	"github.com/k6io/k6/lib/types"
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

// Threshold is a representation of a single threshold for a single metric
type Threshold struct {
	// Source is the text based source of the threshold
	Source string
	// LastFailed is a makrer if the last testing of this threshold failed
	LastFailed bool
	// AbortOnFail marks if a given threshold fails that the whole test should be aborted
	AbortOnFail bool
	// AbortGracePeriod is a the minimum amount of time a test should be running before a failing
	// this threshold will abort the test
	AbortGracePeriod types.NullDuration

	pgm *goja.Program
	rt  *goja.Runtime
}

func newThreshold(src string, newThreshold *goja.Runtime, abortOnFail bool, gracePeriod types.NullDuration) (*Threshold, error) {
	pgm, err := goja.Compile("__threshold__", src, true)
	if err != nil {
		return nil, err
	}

	return &Threshold{
		Source:           src,
		AbortOnFail:      abortOnFail,
		AbortGracePeriod: gracePeriod,
		pgm:              pgm,
		rt:               newThreshold,
	}, nil
}

func (t Threshold) runNoTaint() (bool, error) {
	v, err := t.rt.RunProgram(t.pgm)
	if err != nil {
		return false, err
	}
	return v.ToBoolean(), nil
}

func (t *Threshold) run() (bool, error) {
	b, err := t.runNoTaint()
	t.LastFailed = !b
	return b, err
}

type thresholdConfig struct {
	Threshold        string             `json:"threshold"`
	AbortOnFail      bool               `json:"abortOnFail"`
	AbortGracePeriod types.NullDuration `json:"delayAbortEval"`
}

//used internally for JSON marshalling
type rawThresholdConfig thresholdConfig

func (tc *thresholdConfig) UnmarshalJSON(data []byte) error {
	//shortcircuit unmarshalling for simple string format
	if err := json.Unmarshal(data, &tc.Threshold); err == nil {
		return nil
	}

	rawConfig := (*rawThresholdConfig)(tc)
	return json.Unmarshal(data, rawConfig)
}

func (tc thresholdConfig) MarshalJSON() ([]byte, error) {
	if tc.AbortOnFail {
		return json.Marshal(rawThresholdConfig(tc))
	}
	return json.Marshal(tc.Threshold)
}

// Thresholds is the combination of all Thresholds for a given metric
type Thresholds struct {
	Runtime    *goja.Runtime
	Thresholds []*Threshold
	Abort      bool
}

// NewThresholds returns Thresholds objects representing the provided source strings
func NewThresholds(sources []string) (Thresholds, error) {
	tcs := make([]thresholdConfig, len(sources))
	for i, source := range sources {
		tcs[i].Threshold = source
	}

	return newThresholdsWithConfig(tcs)
}

func newThresholdsWithConfig(configs []thresholdConfig) (Thresholds, error) {
	rt := goja.New()
	if _, err := rt.RunProgram(jsEnv); err != nil {
		return Thresholds{}, errors.Wrap(err, "builtin")
	}

	ts := make([]*Threshold, len(configs))
	for i, config := range configs {
		t, err := newThreshold(config.Threshold, rt, config.AbortOnFail, config.AbortGracePeriod)
		if err != nil {
			return Thresholds{}, errors.Wrapf(err, "%d", i)
		}
		ts[i] = t
	}

	return Thresholds{rt, ts, false}, nil
}

func (ts *Thresholds) updateVM(sink Sink, t time.Duration) error {
	ts.Runtime.Set("__sink__", sink)
	f := sink.Format(t)
	for k, v := range f {
		ts.Runtime.Set(k, v)
	}
	return nil
}

func (ts *Thresholds) runAll(t time.Duration) (bool, error) {
	succ := true
	for i, th := range ts.Thresholds {
		b, err := th.run()
		if err != nil {
			return false, errors.Wrapf(err, "%d", i)
		}
		if !b {
			succ = false

			if ts.Abort || !th.AbortOnFail {
				continue
			}

			ts.Abort = !th.AbortGracePeriod.Valid ||
				th.AbortGracePeriod.Duration < types.Duration(t)
		}
	}
	return succ, nil
}

// Run processes all the thresholds with the provided Sink at the provided time and returns if any
// of them fails
func (ts *Thresholds) Run(sink Sink, t time.Duration) (bool, error) {
	if err := ts.updateVM(sink, t); err != nil {
		return false, err
	}
	return ts.runAll(t)
}

// UnmarshalJSON is implementation of json.Unmarshaler
func (ts *Thresholds) UnmarshalJSON(data []byte) error {
	var configs []thresholdConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return err
	}
	newts, err := newThresholdsWithConfig(configs)
	if err != nil {
		return err
	}
	*ts = newts
	return nil
}

// MarshalJSON is implementation of json.Marshaler
func (ts Thresholds) MarshalJSON() ([]byte, error) {
	configs := make([]thresholdConfig, len(ts.Thresholds))
	for i, t := range ts.Thresholds {
		configs[i].Threshold = t.Source
		configs[i].AbortOnFail = t.AbortOnFail
		configs[i].AbortGracePeriod = t.AbortGracePeriod
	}
	return json.Marshal(configs)
}

var _ json.Unmarshaler = &Thresholds{}
var _ json.Marshaler = &Thresholds{}
