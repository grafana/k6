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

package metrics

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/dop251/goja"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/stats"
)

var nameRegexString = "^[\\p{L}\\p{N}\\._ !\\?/&#\\(\\)<>%-]{1,128}$"

var compileNameRegex = regexp.MustCompile(nameRegexString)

func checkName(name string) bool {
	return compileNameRegex.Match([]byte(name))
}

type Metric struct {
	metric *stats.Metric
}

// ErrMetricsAddInInitContext is error returned when adding to metric is done in the init context
var ErrMetricsAddInInitContext = common.NewInitContextError("Adding to metrics in the init context is not supported")

func newMetric(call goja.ConstructorCall, rt *goja.Runtime, t stats.MetricType) (*goja.Object, error) {
	// TODO this can probably be done by a `common.GetContext(rt)`
	ctx := rt.Get("context").Export().(context.Context) //nolint:forcetypeassert
	if lib.GetState(ctx) != nil {
		return nil, errors.New("metrics must be declared in the init context")
	}

	// TODO this kind of conversions can possibly be automated by the parts of common.Bind that are curently automating
	// it and some wrapping
	name := call.Argument(0).String()
	isTime := call.Argument(1).ToBoolean()
	// TODO: move verification outside the JS
	if !checkName(name) {
		return nil, common.NewInitContextError(fmt.Sprintf("Invalid metric name: '%s'", name))
	}

	valueType := stats.Default
	if isTime {
		valueType = stats.Time
	}

	return rt.ToValue(Metric{stats.New(name, t, valueType)}).ToObject(rt), nil
}

func (m Metric) Add(call goja.FunctionCall, rt *goja.Runtime) goja.Value {
	ctx := rt.Get("context").Export().(context.Context) //nolint:forcetypeassert
	state := lib.GetState(ctx)
	if state == nil {
		common.Throw(rt, ErrMetricsAddInInitContext)
	}
	v := call.Argument(0)
	var addTags map[string]string
	if len(call.Arguments) > 1 {
		// Technically the previous implementation support multiple maps but I don't think that is a good design and can
		// be reproduced if desired
		err := rt.ExportTo(call.Argument(1), &addTags)
		if err != nil {
			common.Throw(rt, err)
		}
	}

	tags := state.CloneTags()
	for k, v := range addTags {
		tags[k] = v
	}

	vfloat := v.ToFloat()
	if vfloat == 0 && v.ToBoolean() {
		vfloat = 1.0
	}

	sample := stats.Sample{Time: time.Now(), Metric: m.metric, Value: vfloat, Tags: stats.IntoSampleTags(&tags)}
	stats.PushIfNotDone(ctx, state.Samples, sample)
	return rt.ToValue(true)
}

// GetName returns the metric name
func (m Metric) GetName() string {
	return m.metric.Name
}

func New() map[string]interface{} {
	// This can definitely be automated more
	// One thing that we can add is to differentiate between
	// import something from "somewhere"; // where something is the *default* exports
	// import * as something from "somewhere"; /// where something is an "object" with all the defined exports
	// This likely will need a change once import/export syntax is part of goja as well :(
	return map[string]interface{}{
		"Counter":          Counter,
		"Gauge":            Gauge,
		"Trend":            Trend,
		"Rate":             Rate,
		"returnMetricType": ReturnMetricType,
	}
}

// This is not possible after common.Bind as it wraps the object and doesn't return the original one.
func ReturnMetricType(m Metric) string {
	return m.metric.Type.String()
}

func Counter(call goja.ConstructorCall, rt *goja.Runtime) *goja.Object {
	v, err := newMetric(call, rt, stats.Counter)
	if err != nil {
		common.Throw(rt, err)
	}
	return v
}

func Gauge(call goja.ConstructorCall, rt *goja.Runtime) *goja.Object {
	v, err := newMetric(call, rt, stats.Gauge)
	if err != nil {
		common.Throw(rt, err)
	}
	return v
}

func Trend(call goja.ConstructorCall, rt *goja.Runtime) *goja.Object {
	v, err := newMetric(call, rt, stats.Trend)
	if err != nil {
		common.Throw(rt, err)
	}
	return v
}

func Rate(call goja.ConstructorCall, rt *goja.Runtime) *goja.Object {
	v, err := newMetric(call, rt, stats.Rate)
	if err != nil {
		common.Throw(rt, err)
	}
	return v
}
