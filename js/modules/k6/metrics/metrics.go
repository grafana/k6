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

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/internal/modules"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
)

func init() {
	modules.Register("k6/metrics", New())
}

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

func newMetric(ctxPtr *context.Context, name string, t stats.MetricType, isTime []bool) (interface{}, error) {
	if lib.GetState(*ctxPtr) != nil {
		return nil, errors.New("metrics must be declared in the init context")
	}

	//TODO: move verification outside the JS
	if !checkName(name) {
		return nil, common.NewInitContextError(fmt.Sprintf("Invalid metric name: '%s'", name))
	}

	valueType := stats.Default
	if len(isTime) > 0 && isTime[0] {
		valueType = stats.Time
	}

	rt := common.GetRuntime(*ctxPtr)
	return common.Bind(rt, Metric{stats.New(name, t, valueType)}, ctxPtr), nil
}

func (m Metric) Add(ctx context.Context, v goja.Value, addTags ...map[string]string) (bool, error) {
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

type Metrics struct{}

func New() *Metrics {
	return &Metrics{}
}

func (*Metrics) XCounter(ctx *context.Context, name string, isTime ...bool) (interface{}, error) {
	return newMetric(ctx, name, stats.Counter, isTime)
}

func (*Metrics) XGauge(ctx *context.Context, name string, isTime ...bool) (interface{}, error) {
	return newMetric(ctx, name, stats.Gauge, isTime)
}

func (*Metrics) XTrend(ctx *context.Context, name string, isTime ...bool) (interface{}, error) {
	return newMetric(ctx, name, stats.Trend, isTime)
}

func (*Metrics) XRate(ctx *context.Context, name string, isTime ...bool) (interface{}, error) {
	return newMetric(ctx, name, stats.Rate, isTime)
}
