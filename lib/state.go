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

package lib

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/cookiejar"
	"sync"

	"github.com/oxtoacart/bpool"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/stats"
)

// DialContexter is an interface that can dial with a context
type DialContexter interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// State provides the volatile state for a VU.
type State struct {
	// Global options.
	Options Options

	// Logger. Avoid using the global logger.
	// TODO change to logrus.FieldLogger when there is time to fix all the tests
	Logger *logrus.Logger

	// Current group; all emitted metrics are tagged with this.
	Group *Group

	// Networking equipment.
	Dialer DialContexter

	// TODO: move a lot of the things below to the k6/http ModuleInstance, see
	// https://github.com/grafana/k6/issues/2293.
	Transport http.RoundTripper
	CookieJar *cookiejar.Jar
	TLSConfig *tls.Config

	// Rate limits.
	RPSLimit *rate.Limiter

	// Sample channel, possibly buffered
	Samples chan<- stats.SampleContainer

	// Buffer pool; use instead of allocating fresh buffers when possible.
	// TODO: maybe use https://golang.org/pkg/sync/#Pool ?
	BPool *bpool.BufferPool

	VUID, VUIDGlobal uint64
	Iteration        int64
	Tags             *TagMap
	// These will be assigned on VU activation.
	// Returns the iteration number of this VU in the current scenario.
	GetScenarioVUIter func() uint64
	// Returns the iteration number across all VUs in the current scenario
	// unique to this single k6 instance.
	// TODO: Maybe this doesn't belong here but in ScenarioState?
	GetScenarioLocalVUIter func() uint64
	// Returns the iteration number across all VUs in the current scenario
	// unique globally across k6 instances (taking into account execution
	// segments).
	GetScenarioGlobalVUIter func() uint64

	BuiltinMetrics *metrics.BuiltinMetrics
}

// CloneTags makes a copy of the tags map and returns it.
func (s *State) CloneTags() map[string]string {
	return s.Tags.Clone()
}

// TagMap is a safe-concurrent Tags lookup.
type TagMap struct {
	m     map[string]string
	mutex sync.RWMutex
}

// NewTagMap creates a TagMap,
// if a not-nil map is passed then it will be used as the internal map
// otherwise a new one will be created.
func NewTagMap(m map[string]string) *TagMap {
	if m == nil {
		m = make(map[string]string)
	}
	return &TagMap{
		m:     m,
		mutex: sync.RWMutex{},
	}
}

// Set sets a Tag.
func (tg *TagMap) Set(k, v string) {
	tg.mutex.Lock()
	defer tg.mutex.Unlock()
	tg.m[k] = v
}

// Get returns the Tag value and true
// if the provided key has been found.
func (tg *TagMap) Get(k string) (string, bool) {
	tg.mutex.RLock()
	defer tg.mutex.RUnlock()
	v, ok := tg.m[k]
	return v, ok
}

// Len returns the number of the set keys.
func (tg *TagMap) Len() int {
	tg.mutex.RLock()
	defer tg.mutex.RUnlock()
	return len(tg.m)
}

// Delete deletes a map's item based on the provided key.
func (tg *TagMap) Delete(k string) {
	tg.mutex.Lock()
	defer tg.mutex.Unlock()
	delete(tg.m, k)
}

// Clone returns a map with the entire set of items.
func (tg *TagMap) Clone() map[string]string {
	tg.mutex.RLock()
	defer tg.mutex.RUnlock()

	tags := make(map[string]string, len(tg.m))
	for k, v := range tg.m {
		tags[k] = v
	}
	return tags
}
