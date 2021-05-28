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
	Transport http.RoundTripper
	Dialer    DialContexter
	CookieJar *cookiejar.Jar
	TLSConfig *tls.Config

	// Rate limits.
	RPSLimit *rate.Limiter

	// Sample channel, possibly buffered
	Samples chan<- stats.SampleContainer

	// Buffer pool; use instead of allocating fresh buffers when possible.
	// TODO: maybe use https://golang.org/pkg/sync/#Pool ?
	BPool *bpool.BufferPool

	Vu                   uint64
	iteration            int64
	Tags                 map[string]string
	ScenarioName         string
	scenarioVUID         map[string]uint64
	IncrScIter           func() int64
	IncrScIterGlobal     func() int64
	scIterMx             sync.RWMutex
	scenarioVUIter       map[string]int64
	scIter, scIterGlobal int64
}

// Init initializes some private state fields.
func (s *State) Init() {
	s.scenarioVUID = make(map[string]uint64)
	s.scenarioVUIter = make(map[string]int64)
	s.iteration, s.scIter, s.scIterGlobal = -1, -1, -1
}

// CloneTags makes a copy of the tags map and returns it.
func (s *State) CloneTags() map[string]string {
	tags := make(map[string]string, len(s.Tags))
	for k, v := range s.Tags {
		tags[k] = v
	}
	return tags
}

// GetScenarioVUID returns the scenario-specific ID of this VU.
func (s *State) GetScenarioVUID() (uint64, bool) {
	id, ok := s.scenarioVUID[s.ScenarioName]
	return id, ok
}

// SetScenarioVUID sets the scenario-specific ID for this VU.
func (s *State) SetScenarioVUID(id uint64) {
	s.scenarioVUID[s.ScenarioName] = id
}

// GetScenarioVUIter returns the scenario-specific count of completed iterations
// for this VU.
func (s *State) GetScenarioVUIter() int64 {
	s.scIterMx.RLock()
	defer s.scIterMx.RUnlock()
	return s.scenarioVUIter[s.ScenarioName]
}

// IncrIteration increments all iteration counters for the specific VU with this
// State.
func (s *State) IncrIteration() {
	s.scIterMx.Lock()
	defer s.scIterMx.Unlock()

	s.iteration++
	if _, ok := s.scenarioVUIter[s.ScenarioName]; ok {
		s.scenarioVUIter[s.ScenarioName]++
	} else {
		s.scenarioVUIter[s.ScenarioName] = 0
	}
	if s.IncrScIter != nil {
		s.scIter = s.IncrScIter()
	}
	if s.IncrScIterGlobal != nil {
		s.scIterGlobal = s.IncrScIterGlobal()
	}
}

// GetScenarioLocalVUIter returns the iteration local to the scenario currently
// executing the VU with this State.
func (s *State) GetScenarioLocalVUIter() int64 {
	s.scIterMx.RLock()
	defer s.scIterMx.RUnlock()
	return s.scIter
}

// GetScenarioGlobalVUIter returns the global iteration of the scenario
// currently executing the VU with this State, or -1 if the executor doesn't
// keep track of global iterations.
func (s *State) GetScenarioGlobalVUIter() int64 {
	s.scIterMx.RLock()
	defer s.scIterMx.RUnlock()
	if s.IncrScIterGlobal == nil {
		return -1
	}
	return s.scIterGlobal
}

// GetIteration returns the iteration local to the VU with this State.
func (s *State) GetIteration() int64 {
	return s.iteration
}
