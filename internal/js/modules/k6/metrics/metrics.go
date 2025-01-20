// Package metrics implements k6/metrics and let script work with custom metrics.
package metrics

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
)

// Metric is a wrapper around custom metrics
type Metric struct {
	metric *metrics.Metric
	vu     modules.VU
}

// ErrMetricsAddInInitContext is error returned when adding to metric is done in the init context
var ErrMetricsAddInInitContext = common.NewInitContextError("Adding to metrics in the init context is not supported")

func (mi *ModuleInstance) newMetric(call sobek.ConstructorCall, t metrics.MetricType) (*sobek.Object, error) {
	initEnv := mi.vu.InitEnv()
	if initEnv == nil {
		return nil, errors.New("metrics must be declared in the init context")
	}
	rt := mi.vu.Runtime()
	c, _ := sobek.AssertFunction(rt.ToValue(func(name string, isTime ...bool) (*sobek.Object, error) {
		valueType := metrics.Default
		if len(isTime) > 0 && isTime[0] {
			valueType = metrics.Time
		}
		m, err := initEnv.Registry.NewMetric(name, t, valueType)
		if err != nil {
			return nil, err
		}
		metric := &Metric{metric: m, vu: mi.vu}
		o := rt.NewObject()
		err = o.DefineDataProperty("name", rt.ToValue(name), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE)
		if err != nil {
			return nil, err
		}
		if err = o.Set("add", rt.ToValue(metric.add)); err != nil {
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

const warnMessageValueMaxSize = 100

func limitValue(v string) string {
	vRunes := []rune(v)
	if len(vRunes) < warnMessageValueMaxSize {
		return v
	}
	difference := int64(len(vRunes) - warnMessageValueMaxSize)
	omitMsg := append(strconv.AppendInt([]byte("... omitting "), difference, 10), " characters ..."...)
	return strings.Join([]string{
		string(vRunes[:warnMessageValueMaxSize/2]),
		string(vRunes[len(vRunes)-warnMessageValueMaxSize/2:]),
	}, string(omitMsg))
}

func (m Metric) add(v sobek.Value, addTags sobek.Value) (bool, error) {
	state := m.vu.State()
	if state == nil {
		return false, ErrMetricsAddInInitContext
	}

	// return/throw exception if throw enabled, otherwise just log
	raiseErr := func(err error) (bool, error) { //nolint:unparam // we want to just do `return raiseErr(...)`
		if state.Options.Throw.Bool {
			return false, err
		}
		state.Logger.Warn(err)
		return false, nil
	}
	raiseNan := func() (bool, error) {
		return raiseErr(fmt.Errorf("'%s' is an invalid value for metric '%s', a number or a boolean value is expected",
			limitValue(v.String()), m.metric.Name))
	}

	if v == nil {
		return raiseErr(fmt.Errorf("no value was provided for metric '%s', a number or a boolean value is expected",
			m.metric.Name))
	}
	if sobek.IsNull(v) {
		return raiseNan()
	}

	vfloat := v.ToFloat()
	if vfloat == 0 && v.ToBoolean() {
		vfloat = 1.0
	}

	if math.IsNaN(vfloat) {
		return raiseNan()
	}

	ctm := state.Tags.GetCurrentValues()
	if err := common.ApplyCustomUserTags(m.vu.Runtime(), &ctm, addTags); err != nil {
		return false, fmt.Errorf("cannot add tags for the '%s' custom metric: %w", m.metric.Name, err)
	}

	sample := metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: m.metric,
			Tags:   ctm.Tags,
		},
		Time:     time.Now(),
		Metadata: ctm.Metadata,
		Value:    vfloat,
	}
	metrics.PushIfNotDone(m.vu.Context(), state.Samples, sample)
	return true, nil
}

type (
	// RootModule is the root metrics module
	RootModule struct{}
	// ModuleInstance represents an instance of the metrics module
	ModuleInstance struct {
		vu modules.VU
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// NewModuleInstance implements modules.Module interface
func (*RootModule) NewModuleInstance(m modules.VU) modules.Instance {
	return &ModuleInstance{vu: m}
}

// New returns a new RootModule.
func New() *RootModule {
	return &RootModule{}
}

// Exports returns the exports of the metrics module
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"Counter": mi.XCounter,
			"Gauge":   mi.XGauge,
			"Trend":   mi.XTrend,
			"Rate":    mi.XRate,
		},
	}
}

// XCounter is a counter constructor
func (mi *ModuleInstance) XCounter(call sobek.ConstructorCall, rt *sobek.Runtime) *sobek.Object {
	v, err := mi.newMetric(call, metrics.Counter)
	if err != nil {
		common.Throw(rt, err)
	}
	return v
}

// XGauge is a gauge constructor
func (mi *ModuleInstance) XGauge(call sobek.ConstructorCall, rt *sobek.Runtime) *sobek.Object {
	v, err := mi.newMetric(call, metrics.Gauge)
	if err != nil {
		common.Throw(rt, err)
	}
	return v
}

// XTrend is a trend constructor
func (mi *ModuleInstance) XTrend(call sobek.ConstructorCall, rt *sobek.Runtime) *sobek.Object {
	v, err := mi.newMetric(call, metrics.Trend)
	if err != nil {
		common.Throw(rt, err)
	}
	return v
}

// XRate is a rate constructor
func (mi *ModuleInstance) XRate(call sobek.ConstructorCall, rt *sobek.Runtime) *sobek.Object {
	v, err := mi.newMetric(call, metrics.Rate)
	if err != nil {
		common.Throw(rt, err)
	}
	return v
}
