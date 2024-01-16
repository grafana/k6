// SPDX-FileCopyrightText: 2023 Iván Szkiba
// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only
// SPDX-License-Identifier: MIT

package dashboard

import (
	"go.k6.io/k6/metrics"
)

// registry is what can create metrics and make them iterable.
type registry struct {
	*metrics.Registry
	names []string
}

// newRegistry returns a new registry.
func newRegistry() *registry {
	return &registry{
		Registry: metrics.NewRegistry(),
		names:    make([]string, 0),
	}
}

// getOrNew returns existing metric or create new metric registered to this registry.
func (reg *registry) getOrNew(
	name string,
	typ metrics.MetricType,
	valTyp ...metrics.ValueType,
) (*metrics.Metric, error) {
	if metric := reg.Registry.Get(name); metric != nil {
		return metric, nil
	}

	metric, err := reg.Registry.NewMetric(name, typ, valTyp...)
	if err != nil {
		return nil, err
	}

	reg.names = append(reg.names, name)

	return metric, nil
}

// mustGetOrNew is like getOrNew, but will panic if there is an error.
func (reg *registry) mustGetOrNew(
	name string,
	typ metrics.MetricType,
	valTyp ...metrics.ValueType,
) *metrics.Metric {
	metric, err := reg.getOrNew(name, typ, valTyp...)
	if err != nil {
		panic(err)
	}

	return metric
}

// newbies return newly registered names since last seen.
func (reg *registry) newbies(seen map[string]struct{}) []string {
	var names []string

	for _, name := range reg.names {
		if _, ok := seen[name]; !ok {
			names = append(names, name)
			seen[name] = struct{}{}
		}
	}

	return names
}
