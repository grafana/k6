/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
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
	"fmt"
	"regexp"
	"sort"
	"sync"

	"github.com/mstoykov/atlas"
)

// Registry is what can create metrics
type Registry struct {
	metrics map[string]*Metric
	l       sync.RWMutex

	tagSetRoot *atlas.Node
}

// NewRegistry returns a new registry
func NewRegistry() *Registry {
	return &Registry{
		metrics: make(map[string]*Metric),
		// The Atlas initialization: where the root node is created.
		// All the new tag sets must branch out from the root,
		// so the underhood structure can minimize the memory footprint
		// by allocating the tag pair only once.
		tagSetRoot: atlas.New(),
	}
}

const nameRegexString = "^[\\p{L}\\p{N}\\._ !\\?/&#\\(\\)<>%-]{1,128}$"

var compileNameRegex = regexp.MustCompile(nameRegexString)

func checkName(name string) bool {
	return compileNameRegex.Match([]byte(name))
}

// NewMetric returns new metric registered to this registry
// TODO have multiple versions returning specific metric types when we have such things
func (r *Registry) NewMetric(name string, typ MetricType, t ...ValueType) (*Metric, error) {
	r.l.Lock()
	defer r.l.Unlock()

	if !checkName(name) {
		return nil, fmt.Errorf("Invalid metric name: '%s'", name) //nolint:golint,stylecheck
	}
	oldMetric, ok := r.metrics[name]

	if !ok {
		m := newMetric(name, typ, t...)
		r.metrics[name] = m
		return m, nil
	}
	if oldMetric.Type != typ {
		return nil, fmt.Errorf("metric '%s' already exists but with type %s, instead of %s", name, oldMetric.Type, typ)
	}
	if len(t) > 0 {
		if t[0] != oldMetric.Contains {
			return nil, fmt.Errorf("metric '%s' already exists but with a value type %s, instead of %s",
				name, oldMetric.Contains, t[0])
		}
	}
	return oldMetric, nil
}

// MustNewMetric is like NewMetric, but will panic if there is an error
func (r *Registry) MustNewMetric(name string, typ MetricType, t ...ValueType) *Metric {
	m, err := r.NewMetric(name, typ, t...)
	if err != nil {
		panic(err)
	}
	return m
}

// Get returns the Metric with the given name. If that metric doesn't exist,
// Get() will return a nil value.
func (r *Registry) Get(name string) *Metric {
	return r.metrics[name]
}

// BranchTagSetRoot creates a new TagSet starting from the root.
func (r *Registry) BranchTagSetRoot() *TagSet {
	return &TagSet{tags: r.tagSetRoot}
}

// BranchTagSetRootWith creates a new TagSet starting from the root and
// adds all the key-value pairs provided with the map to the new TagSet.
func (r *Registry) BranchTagSetRootWith(m map[string]string) *TagSet {
	if len(m) < 1 {
		return r.BranchTagSetRoot()
	}

	tset := r.BranchTagSetRoot()

	// it sorts the keys so the TagSet generation is consistent
	// across multiple invocations.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i := 0; i < len(keys); i++ {
		tset.AddTag(keys[i], m[keys[i]])
	}

	return tset
}
