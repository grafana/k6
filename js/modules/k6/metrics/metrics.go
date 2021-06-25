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
	metric     *stats.Metric
	getContext func() context.Context
}

// ErrMetricsAddInInitContext is error returned when adding to metric is done in the init context
var ErrMetricsAddInInitContext = common.NewInitContextError("Adding to metrics in the init context is not supported")

func (m *MetricsModule) newMetric(call goja.ConstructorCall, t stats.MetricType) (*goja.Object, error) {
	ctx := m.GetContext()
	if lib.GetState(ctx) != nil {
		return nil, errors.New("metrics must be declared in the init context")
	}
	rt := common.GetRuntime(ctx) // NOTE we can get this differently as well

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

	return rt.ToValue(Metric{metric: stats.New(name, t, valueType), getContext: m.GetContext}).ToObject(rt), nil
}

func (m Metric) Add(v goja.Value, addTags ...map[string]string) (bool, error) {
	ctx := m.getContext()
	state := lib.GetState(ctx)
	if state == nil {
		return false, ErrMetricsAddInInitContext
	}

	tags := state.CloneTags()
	for _, ts := range addTags {
		for k, v := range ts {
			tags[k] = v
		}
	}

	vfloat := v.ToFloat()
	if vfloat == 0 && v.ToBoolean() {
		vfloat = 1.0
	}

	sample := stats.Sample{Time: time.Now(), Metric: m.metric, Value: vfloat, Tags: stats.IntoSampleTags(&tags)}
	stats.PushIfNotDone(ctx, state.Samples, sample)
	return true, nil
}

// GetName returns the metric name
func (m Metric) GetName() string {
	return m.metric.Name
}

type (
	RootMetricsModule struct{}
	MetricsModule     struct {
		common.ModuleInstance
	}
)

func (*RootMetricsModule) NewModuleInstance(m common.ModuleInstance) common.ModuleInstance {
	return &MetricsModule{ModuleInstance: m}
}

func New() *RootMetricsModule {
	return &RootMetricsModule{}
}

func (m *MetricsModule) GetExports() common.Exports {
	return common.GenerateExports(m)
}

// This is not possible after common.Bind as it wraps the object and doesn't return the original one.
func (m *MetricsModule) ReturnMetricType(metric Metric) string {
	return metric.metric.Type.String()
}

// Counter ... // NOTE we still need to use goja.ConstructorCall  somewhere to have access to the
func (m *MetricsModule) XCounter(call goja.ConstructorCall, rt *goja.Runtime) *goja.Object {
	v, err := m.newMetric(call, stats.Counter)
	if err != nil {
		common.Throw(rt, err)
	}
	return v
}

func (m *MetricsModule) XGauge(call goja.ConstructorCall, rt *goja.Runtime) *goja.Object {
	v, err := m.newMetric(call, stats.Gauge)
	if err != nil {
		common.Throw(rt, err)
	}
	return v
}

func (m *MetricsModule) XTrend(call goja.ConstructorCall, rt *goja.Runtime) *goja.Object {
	v, err := m.newMetric(call, stats.Trend)
	if err != nil {
		common.Throw(rt, err)
	}
	return v
}

func (m *MetricsModule) XRate(call goja.ConstructorCall, rt *goja.Runtime) *goja.Object {
	v, err := m.newMetric(call, stats.Rate)
	if err != nil {
		common.Throw(rt, err)
	}
	return v
}
