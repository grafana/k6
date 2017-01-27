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

package js

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/robertkrimen/otto"
	"strconv"
	"sync/atomic"
	"time"
)

type JSAPI struct {
	vu *VU
}

func (a JSAPI) Sleep(secs float64) {
	d := time.Duration(secs * float64(time.Second))
	t := time.NewTimer(d)
	select {
	case <-t.C:
	case <-a.vu.ctx.Done():
	}
	t.Stop()
}

func (a JSAPI) Log(level int, msg string, args []otto.Value) {
	fields := make(log.Fields, len(args))
	for i, arg := range args {
		if arg.IsObject() {
			obj := arg.Object()
			for _, key := range obj.Keys() {
				v, err := obj.Get(key)
				if err != nil {
					throw(a.vu.vm, err)
				}
				fields[key] = v.String()
			}
			continue
		}
		fields["arg"+strconv.Itoa(i)] = arg.String()
	}

	entry := log.WithFields(fields)
	switch level {
	case 0:
		entry.Debug(msg)
	case 1:
		entry.Info(msg)
	case 2:
		entry.Warn(msg)
	case 3:
		entry.Error(msg)
	}
}

func (a JSAPI) DoGroup(call otto.FunctionCall) otto.Value {
	name := call.Argument(0).String()
	group, err := a.vu.group.Group(name)
	if err != nil {
		throw(call.Otto, err)
	}
	a.vu.group = group
	defer func() { a.vu.group = group.Parent }()

	fn := call.Argument(1)
	if !fn.IsFunction() {
		panic(call.Otto.MakeSyntaxError("fn must be a function"))
	}

	val, err := fn.Call(call.This)
	if err != nil {
		throw(call.Otto, err)
	}

	if val.IsUndefined() {
		return otto.TrueValue()
	}
	return val
}

func (a JSAPI) DoCheck(call otto.FunctionCall) otto.Value {
	if len(call.ArgumentList) < 2 {
		return otto.UndefinedValue()
	}

	t := time.Now()

	success := true
	arg0 := call.Argument(0)
	for _, v := range call.ArgumentList[1:] {
		obj := v.Object()
		if obj == nil {
			panic(call.Otto.MakeTypeError("checks must be objects"))
		}
		keys := obj.Keys()
		samples := make([]stats.Sample, len(keys))
		for i, name := range keys {
			val, err := obj.Get(name)
			if err != nil {
				throw(call.Otto, err)
			}

			result, err := Check(val, arg0)
			if err != nil {
				throw(call.Otto, err)
			}

			check, err := a.vu.group.Check(name)
			if err != nil {
				throw(call.Otto, err)
			}

			sampleValue := 1.0
			if result {
				atomic.AddInt64(&(check.Passes), 1)
			} else {
				atomic.AddInt64(&(check.Fails), 1)
				success = false
				sampleValue = 0.0
			}

			samples[i] = stats.Sample{
				Time:   t,
				Metric: lib.MetricChecks,
				Tags: map[string]string{
					"group_id": check.Group.ID,
					"check_id": check.ID,
					"path":     check.Path, // Included for human readability.
				},
				Value: sampleValue,
			}
		}
		a.vu.Samples = append(a.vu.Samples, samples...)
	}

	if !success {
		a.vu.Taint = true
		return otto.FalseValue()
	}
	return otto.TrueValue()
}

func (a JSAPI) Taint() {
	a.vu.Taint = true
}

func (a JSAPI) ElapsedMs() float64 {
	return float64(time.Since(a.vu.started)) / float64(time.Millisecond)
}
