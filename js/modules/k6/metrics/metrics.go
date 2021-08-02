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
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/dop251/goja"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
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
	core   modules.InstanceCore
}

// ErrMetricsAddInInitContext is error returned when adding to metric is done in the init context
var ErrMetricsAddInInitContext = common.NewInitContextError("Adding to metrics in the init context is not supported")

func (mm *MetricsModule) newMetric(call goja.ConstructorCall, t stats.MetricType) (*goja.Object, error) {
	ctx := mm.GetContext()
	initEnv := common.GetInitEnv(ctx)
	if initEnv == nil {
		return nil, errors.New("metrics must be declared in the init context")
	}
	rt := common.GetRuntime(ctx) // NOTE we can get this differently as well
	c, _ := goja.AssertFunction(rt.ToValue(func(name string, isTime ...bool) (*goja.Object, error) {
		// TODO: move verification outside the JS
		if !checkName(name) {
			return nil, common.NewInitContextError(fmt.Sprintf("Invalid metric name: '%s'", name))
		}

		valueType := stats.Default
		if len(isTime) > 0 && isTime[0] {
			valueType = stats.Time
		}
		m := stats.New(name, t, valueType)

		metric := &Metric{metric: m, core: mm.InstanceCore}
		o := rt.NewObject()
		err := o.DefineDataProperty("name", rt.ToValue(name), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE)
		if err != nil {
			return nil, err
		}
		if err = o.Set("add", rt.ToValue(metric.Add)); err != nil {
			return nil, err
		}
		return o, nil
	}))
	v, err := c(call.This, call.Arguments...)
	if err != nil {
		return nil, err
	}

	return v.ToObject(rt), nil
}

func (m Metric) Add(v goja.Value, addTags ...map[string]string) (bool, error) {
	ctx := m.core.GetContext()
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

type (
	RootMetricsModule struct{}
	MetricsModule     struct {
		modules.InstanceCore
	}
)

func (*RootMetricsModule) NewModuleInstance(m modules.InstanceCore) modules.ModuleInstance {
	return &MetricsModule{InstanceCore: m}
}

func New() *RootMetricsModule {
	return &RootMetricsModule{}
}

func (m *MetricsModule) GetExports() modules.Exports {
	return modules.GenerateExports(m)
}

// This is not possible after common.Bind as it wraps the object and doesn't return the original one.
func (m *MetricsModule) ReturnMetricType(metric Metric) string {
	return metric.metric.Type.String()
}

// Counter ... // NOTE we still need to use goja.ConstructorCall  somewhere to have automatic constructor support by
// goja
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
