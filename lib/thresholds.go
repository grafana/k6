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

	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
)

const jsEnv = `
function p(pct) {
	return __sink__.P(pct/100.0);
};
`

type Threshold struct {
	Source string
	Failed bool

	script *otto.Script
	vm     *otto.Otto
}

func NewThreshold(src string, vm *otto.Otto) (*Threshold, error) {
	script, err := vm.Compile("__threshold__", src)
	if err != nil {
		return nil, err
	}

	return &Threshold{
		Source: src,
		script: script,
		vm:     vm,
	}, nil
}

func (t Threshold) RunNoTaint() (bool, error) {
	v, err := t.vm.Run(t.script)
	if err != nil {
		return false, err
	}
	return v.ToBoolean()
}

func (t *Threshold) Run() (bool, error) {
	b, err := t.RunNoTaint()
	if !b {
		t.Failed = true
	}
	return b, err
}

type Thresholds struct {
	VM         *otto.Otto
	Thresholds []*Threshold
}

func NewThresholds(sources []string) (Thresholds, error) {
	vm := otto.New()

	if _, err := vm.Eval(jsEnv); err != nil {
		return Thresholds{}, errors.Wrap(err, "builtin")
	}

	ts := make([]*Threshold, len(sources))
	for i, src := range sources {
		t, err := NewThreshold(src, vm)
		if err != nil {
			return Thresholds{}, errors.Wrapf(err, "%d", i)
		}
		ts[i] = t
	}
	return Thresholds{vm, ts}, nil
}

func (ts *Thresholds) UpdateVM(sink stats.Sink) error {
	if err := ts.VM.Set("__sink__", sink); err != nil {
		return err
	}
	for k, v := range sink.Format() {
		if err := ts.VM.Set(k, v); err != nil {
			return errors.Wrapf(err, "%s", k)
		}
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

func (ts *Thresholds) Run(sink stats.Sink) (bool, error) {
	if err := ts.UpdateVM(sink); err != nil {
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
