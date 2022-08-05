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

	"go.k6.io/k6/metrics"
)

// DialContexter is an interface that can dial with a context
type DialContexter interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// State provides the volatile state for a VU.
type State struct {
	// Global options and built-in metrics.
	//
	// TODO: remove them from here, the built-in metrics and the script options
	// are not part of a VU's unique "state", they are global and the same for
	// all VUs. Figure out how to thread them some other way, e.g. through the
	// TestPreInitState. The Samples channel might also benefit from that...
	Options        Options
	BuiltinMetrics *metrics.BuiltinMetrics

	// Logger. Avoid using the global logger.
	// TODO: change to logrus.FieldLogger when there is time to fix all the tests
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
	Samples chan<- metrics.SampleContainer

	// Buffer pool; use instead of allocating fresh buffers when possible.
	// TODO: maybe use https://golang.org/pkg/sync/#Pool ?
	BPool *bpool.BufferPool

	VUID, VUIDGlobal uint64
	Iteration        int64
	Tags             *VUStateTags
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
}

// VUStateTags wraps the current VU's tags and ensures a thread-safe way to
// access and modify them exists. This is necessary because the VU tags and
// metadata can be modified from the JS scripts via the `vu.tags` API in the
// `k6/execution` built-in module.
type VUStateTags struct {
	mutex sync.RWMutex
	tags  *metrics.TagSet
	// TODO: Add metadata map[string]string
}

// NewVUStateTags initializes a new VUStateTags and returns it. It's important
// that tags is not nil and initialized via metrics.Registry.RootTagSet().
func NewVUStateTags(tags *metrics.TagSet) *VUStateTags {
	if tags == nil {
		panic("the metrics.TagSet must be initialized for creating a new lib.VUStateTags")
	}
	return &VUStateTags{
		mutex: sync.RWMutex{},
		tags:  tags,
		// metadata is intentionally nil by default
	}
}

// GetCurrentValues returns the value of the VU tags in a thread-safe way.
func (tg *VUStateTags) GetCurrentValues() *metrics.TagSet {
	tg.mutex.RLock()
	defer tg.mutex.RUnlock()
	return tg.tags
}

// Modify allows the thread-safe modification of the current VU tags.
func (tg *VUStateTags) Modify(callback func(currentTags *metrics.TagSet) (newTags *metrics.TagSet)) {
	tg.mutex.Lock()
	defer tg.mutex.Unlock()
	tg.tags = callback(tg.tags)
}
