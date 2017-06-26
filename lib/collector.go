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
	"io"

	"github.com/loadimpact/k6/stats"
)

// A Collector abstracts away the details of a storage backend from the application.
type Collector interface {
	// Init is called between the collector's creation and the call to Run(), right after the k6
	// banner has been printed to stdout. The argument is the result of calling MakeConfig() and
	// then deserializing from JSON the config value stored to disk (if any).
	Init() error

	// MakeConfig is called before Init() and should instantiate a blank configuration struct.
	// Do not apply defaults here, instead use null'able values and apply defaults in Init().
	MakeConfig() interface{}

	// Run is called in a goroutine and starts the collector. Should commit samples to the backend
	// at regular intervals and when the context is terminated.
	Run(ctx context.Context)

	// Returns whether the collector is ready to receive samples. The engine will wait for this to
	// return true before doing anything else.
	IsReady() bool

	// Collect receives a set of samples. This method is never called concurrently, and only while
	// the context for Run() is valid, but should defer as much work as possible to Run().
	Collect(samples []stats.Sample)
}

// An AuthenticatedCollector is a collector that can store persistent authentication.
type AuthenticatedCollector interface {
	Collector

	// Present a login form to the user.
	Login(conf interface{}, in io.Reader, out io.Writer) (interface{}, error)
}
