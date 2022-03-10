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

// Package output contains the interfaces that k6 outputs (and output
// extensions) have to implement, as well as some helpers to make their
// implementation and management easier.
package output

import (
	"encoding/json"
	"io"
	"net/url"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

// Params contains all possible constructor parameters an output may need.
type Params struct {
	OutputType     string // --out $OutputType=$ConfigArgument, K6_OUT="$OutputType=$ConfigArgument"
	ConfigArgument string
	JSONConfig     json.RawMessage

	Logger         logrus.FieldLogger
	Environment    map[string]string
	StdOut         io.Writer
	StdErr         io.Writer
	FS             afero.Fs
	ScriptPath     *url.URL
	ScriptOptions  lib.Options
	RuntimeOptions lib.RuntimeOptions
	ExecutionPlan  []lib.ExecutionStep
}

// TODO: make v2 with buffered channels?

// An Output abstracts the process of funneling samples to an external storage
// backend, such as a file or something like an InfluxDB instance.
//
// N.B: All outputs should have non-blocking AddMetricSamples() methods and
// should spawn their own goroutine to flush metrics asynchronously.
type Output interface {
	// Returns a human-readable description of the output that will be shown in
	// `k6 run`. For extensions it probably should include the version as well.
	Description() string

	// Start is called before the Engine tries to use the output and should be
	// used for any long initialization tasks, as well as for starting a
	// goroutine to asynchronously flush metrics to the output.
	Start() error

	// A method to receive the latest metric samples from the Engine. This
	// method is never called concurrently, so do not do anything blocking here
	// that might take a long time. Preferably, just use the SampleBuffer or
	// something like it to buffer metrics until they are flushed.
	AddMetricSamples(samples []metrics.SampleContainer)

	// Flush all remaining metrics and finalize the test run.
	Stop() error
}

// WithThresholds is an output that requires the Engine to give it the
// thresholds before it can be started.
type WithThresholds interface {
	Output
	SetThresholds(map[string]metrics.Thresholds)
}

// WithTestRunStop is an output that can stop the Engine mid-test, interrupting
// the whole test run execution if some internal condition occurs, completely
// independently from the thresholds. It requires a callback function which
// expects an error and triggers the Engine to stop.
type WithTestRunStop interface {
	Output
	SetTestRunStopCallback(func(error))
}

// WithRunStatusUpdates means the output can receive test run status updates.
type WithRunStatusUpdates interface {
	Output
	SetRunStatus(latestStatus lib.RunStatus)
}

// WithBuiltinMetrics means the output can receive the builtin metrics.
type WithBuiltinMetrics interface {
	Output
	SetBuiltinMetrics(builtinMetrics *metrics.BuiltinMetrics)
}
