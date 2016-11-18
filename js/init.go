package js

import (
	"errors"
	"fmt"
	"github.com/loadimpact/speedboat/stats"
	"github.com/robertkrimen/otto"
	"strings"
)

type InitAPI struct {
	r *Runtime
}

func (i InitAPI) NewMetric(it int, name string) *stats.Metric {
	t := stats.MetricType(it)
	if m, ok := i.r.Metrics[name]; ok {
		if m.Type != t {
			throw(i.r.VM, errors.New(fmt.Sprintf("attempted to redeclare %s with a different type (%s != %s)", name, m.Type, t)))
			return nil
		}

		return m
	}

	m := stats.New(name, t)
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
