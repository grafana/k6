// SPDX-FileCopyrightText: 2023 Iván Szkiba
// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only
// SPDX-License-Identifier: MIT

package dashboard

import (
	"sort"

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
	valTyp metrics.ValueType,
	thresholds []string,
) (*metrics.Metric, error) {
	if metric := reg.Registry.Get(name); metric != nil {
		return metric, nil
	}

	metric, err := reg.Registry.NewMetric(name, typ, valTyp)
	if err != nil {
		return nil, err
	}

	metric.Thresholds = metrics.NewThresholds(thresholds)

	if err := metric.Thresholds.Validate(name, reg.Registry); err != nil {
		return nil, err
	}

	reg.names = append(reg.names, name)

	return metric, nil
}

// mustGetOrNew is like getOrNew, but will panic if there is an error.
func (reg *registry) mustGetOrNew(
	name string,
	typ metrics.MetricType,
	valTyp metrics.ValueType,
	thresholds []string,
) *metrics.Metric {
	metric, err := reg.getOrNew(name, typ, valTyp, thresholds)
	if err != nil {
		panic(err)
	}

	return metric
}

// newbies return newly registered names since last seen.
func (reg *registry) newbies(seen []string) ([]string, []string) {
	var names []string

	process := func(name string) {
		idx := sort.SearchStrings(seen, name)
		if idx == len(seen) {
			seen = append(seen, name)
			names = append(names, name)
		}

		if seen[idx] == name {
			return
		}

		names = append(names, name)

		old := seen
		seen = make([]string, len(old)+1)

		copy(seen[:idx], old[:idx])
		seen[idx] = name
		copy(seen[idx+1:], old[idx:])
	}

	for _, metric := range reg.All() {
		process(metric.Name)

		for _, sub := range metric.Submetrics {
			process(sub.Name)
		}
	}

	return names, seen
}
