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
	"fmt"
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
	Source string
	Failed bool

	pgm *goja.Program
	rt  *goja.Runtime
}

func NewThreshold(src string, rt *goja.Runtime) (*Threshold, error) {
	pgm, err := goja.Compile("__threshold__", src, true)
	if err != nil {
		return nil, err
	}

	return &Threshold{
		Source: src,
		pgm:    pgm,
		rt:     rt,
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

type Thresholds struct {
	Runtime    *goja.Runtime
	Thresholds []*Threshold
}

func NewThresholds(sources []string) (Thresholds, error) {
	rt := goja.New()
	if _, err := rt.RunProgram(jsEnv); err != nil {
		return Thresholds{}, errors.Wrap(err, "builtin")
	}

	ts := make([]*Threshold, len(sources))
	for i, src := range sources {
		t, err := NewThreshold(src, rt)
		if err != nil {
			return Thresholds{}, errors.Wrapf(err, "%d", i)
		}
		ts[i] = t
	}
	return Thresholds{rt, ts}, nil
}

func (ts *Thresholds) UpdateVM(sink Sink, t time.Duration) error {
	ts.Runtime.Set("__sink__", sink)
	f := sink.Format(t)
	fmt.Println(f)
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
	var sources []string
	if err := json.Unmarshal(data, &sources); err != nil {
		return err
	}

	newts, err := NewThresholds(sources)
	if err != nil {
		return err
	}
	*ts = newts
	return nil
}

func (ts Thresholds) MarshalJSON() ([]byte, error) {
	sources := make([]string, len(ts.Thresholds))
	for i, t := range ts.Thresholds {
		sources[i] = t.Source
	}
	return json.Marshal(sources)
}
