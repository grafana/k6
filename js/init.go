package js

import (
	"errors"
	"fmt"
	"github.com/loadimpact/k6/stats"
	"github.com/robertkrimen/otto"
	"strings"
)

type InitAPI struct {
	r *Runtime
}

func (i InitAPI) NewMetric(it int, name string, isTime bool) *stats.Metric {
	t := stats.MetricType(it)
	vt := stats.Default
	if isTime {
		vt = stats.Time
	}

	if m, ok := i.r.Metrics[name]; ok {
		if m.Type != t {
			throw(i.r.VM, errors.New(fmt.Sprintf("attempted to redeclare %s with a different type (%s != %s)", name, m.Type, t)))
			return nil
		}
		if m.Contains != vt {
			throw(i.r.VM, errors.New(fmt.Sprintf("attempted to redeclare %s with a different kind of value (%s != %s)", name, m.Contains, vt)))
		}
		return m
	}

	m := stats.New(name, t, vt)
	i.r.Metrics[name] = m
	return m
}

func (i InitAPI) Require(name string) otto.Value {
	if !strings.HasPrefix(name, ".") {
		exports, err := i.r.loadLib(name + ".js")
		if err != nil {
			throw(i.r.VM, err)
		}
		return exports
	}

	exports, err := i.r.loadFile(name + ".js")
	if err != nil {
		throw(i.r.VM, err)
	}
	return exports
}
