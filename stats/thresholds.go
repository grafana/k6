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
package stats

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"go.k6.io/k6/lib/types"
)

// Threshold is a representation of a single threshold for a single metric
type Threshold struct {
	// Source is the text based source of the threshold
	Source string
	// LastFailed is a marker if the last testing of this threshold failed
	LastFailed bool
	// AbortOnFail marks if a given threshold fails that the whole test should be aborted
	AbortOnFail bool
	// AbortGracePeriod is a the minimum amount of time a test should be running before a failing
	// this threshold will abort the test
	AbortGracePeriod types.NullDuration
	// Parsed is the threshold expression Parsed from the Source
	Parsed *ThresholdExpression
}

func newThreshold(src string, abortOnFail bool, gracePeriod types.NullDuration) (*Threshold, error) {
	parsedExpression, err := parseThresholdExpression(src)
	if err != nil {
		return nil, err
	}

	return &Threshold{
		Source:           src,
		AbortOnFail:      abortOnFail,
		AbortGracePeriod: gracePeriod,
		Parsed:           parsedExpression,
	}, nil
}

func (t *Threshold) runNoTaint(sinks map[string]float64) (bool, error) {
	// Extract the sink value for the aggregation method used in the threshold
	// expression
	lhs, ok := sinks[t.Parsed.SinkKey()]
	if !ok {
		return false, fmt.Errorf("unable to apply threshold %s over metrics; reason: "+
			"no metric supporting the %s aggregation method found",
			t.Source,
			t.Parsed.AggregationMethod)
	}

	// Apply the threshold expression operator to the left and
	// right hand side values
	var passes bool
	switch t.Parsed.Operator {
	case ">":
		passes = lhs > t.Parsed.Value
	case ">=":
		passes = lhs >= t.Parsed.Value
	case "<=":
		passes = lhs <= t.Parsed.Value
	case "<":
		passes = lhs < t.Parsed.Value
	case "==", "===":
		// Considering a sink always maps to float64 values,
		// strictly equal is equivalent to loosely equal
		passes = lhs == t.Parsed.Value
	case "!=":
		passes = lhs != t.Parsed.Value
	default:
		// The parseThresholdExpression function should ensure that no invalid
		// operator gets through, but let's protect our future selves anyhow.
		return false, fmt.Errorf("unable to apply threshold %s over metrics; "+
			"reason: %s is an invalid operator",
			t.Source,
			t.Parsed.Operator,
		)
	}

	// Perform the actual threshold verification
	return passes, nil
}

func (t *Threshold) run(sinks map[string]float64) (bool, error) {
	passes, err := t.runNoTaint(sinks)
	t.LastFailed = !passes
	return passes, err
}

type thresholdConfig struct {
	Threshold        string             `json:"threshold"`
	AbortOnFail      bool               `json:"abortOnFail"`
	AbortGracePeriod types.NullDuration `json:"delayAbortEval"`
}

// used internally for JSON marshalling
type rawThresholdConfig thresholdConfig

func (tc *thresholdConfig) UnmarshalJSON(data []byte) error {
	// shortcircuit unmarshalling for simple string format
	if err := json.Unmarshal(data, &tc.Threshold); err == nil {
		return nil
	}

	rawConfig := (*rawThresholdConfig)(tc)
	return json.Unmarshal(data, rawConfig)
}

func (tc thresholdConfig) MarshalJSON() ([]byte, error) {
	var data interface{} = tc.Threshold
	if tc.AbortOnFail {
		data = rawThresholdConfig(tc)
	}

	return MarshalJSONWithoutHTMLEscape(data)
}

// Thresholds is the combination of all Thresholds for a given metric
type Thresholds struct {
	Thresholds []*Threshold
	Abort      bool
	sinked     map[string]float64
}

// NewThresholds returns Thresholds objects representing the provided source strings
func NewThresholds(sources []string) (Thresholds, error) {
	tcs := make([]thresholdConfig, len(sources))
	for i, source := range sources {
		tcs[i].Threshold = source
	}

	return newThresholdsWithConfig(tcs)
}

func newThresholdsWithConfig(configs []thresholdConfig) (Thresholds, error) {
	thresholds := make([]*Threshold, len(configs))
	sinked := make(map[string]float64)

	for i, config := range configs {
		t, err := newThreshold(config.Threshold, config.AbortOnFail, config.AbortGracePeriod)
		if err != nil {
			return Thresholds{}, fmt.Errorf("threshold %d error: %w", i, err)
		}
		thresholds[i] = t
	}

	return Thresholds{thresholds, false, sinked}, nil
}

func (ts *Thresholds) runAll(timeSpentInTest time.Duration) (bool, error) {
	succeeded := true
	for i, threshold := range ts.Thresholds {
		b, err := threshold.run(ts.sinked)
		if err != nil {
			return false, fmt.Errorf("threshold %d run error: %w", i, err)
		}

		if !b {
			succeeded = false

			if ts.Abort || !threshold.AbortOnFail {
				continue
			}

			ts.Abort = !threshold.AbortGracePeriod.Valid ||
				threshold.AbortGracePeriod.Duration < types.Duration(timeSpentInTest)
		}
	}

	return succeeded, nil
}

// Run processes all the thresholds with the provided Sink at the provided time and returns if any
// of them fails
func (ts *Thresholds) Run(sink Sink, duration time.Duration) (bool, error) {
	// Initialize the sinks store
	ts.sinked = make(map[string]float64)

	// FIXME: Remove this comment as soon as the stats.Sink does not expose Format anymore.
	//
	// As of December 2021, this block reproduces the behavior of the
	// stats.Sink.Format behavior. As we intend to try to get away from it,
	// we instead implement the behavior directly here.
	//
	// For more details, see https://github.com/grafana/k6/issues/2320
	switch sinkImpl := sink.(type) {
	case *CounterSink:
		ts.sinked["count"] = sinkImpl.Value
		ts.sinked["rate"] = sinkImpl.Value / (float64(duration) / float64(time.Second))
	case *GaugeSink:
		ts.sinked["value"] = sinkImpl.Value
	case *TrendSink:
		ts.sinked["min"] = sinkImpl.Min
		ts.sinked["max"] = sinkImpl.Max
		ts.sinked["avg"] = sinkImpl.Avg
		ts.sinked["med"] = sinkImpl.Med

		// Parse the percentile thresholds and insert them in
		// the sinks mapping.
		for _, threshold := range ts.Thresholds {
			if threshold.Parsed.AggregationMethod != TokenPercentile {
				continue
			}

			key := fmt.Sprintf("p(%g)", threshold.Parsed.AggregationValue.Float64)
			ts.sinked[key] = sinkImpl.P(threshold.Parsed.AggregationValue.Float64 / 100)
		}
	case *RateSink:
		ts.sinked["rate"] = float64(sinkImpl.Trues) / float64(sinkImpl.Total)
	case DummySink:
		for k, v := range sinkImpl {
			ts.sinked[k] = v
		}
	default:
		return false, fmt.Errorf("unable to run Thresholds; reason: unknown sink type")
	}

	return ts.runAll(duration)
}

// UnmarshalJSON is implementation of json.Unmarshaler
func (ts *Thresholds) UnmarshalJSON(data []byte) error {
	var configs []thresholdConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return err
	}
	newts, err := newThresholdsWithConfig(configs)
	if err != nil {
		return err
	}
	*ts = newts
	return nil
}

// MarshalJSON is implementation of json.Marshaler
func (ts Thresholds) MarshalJSON() ([]byte, error) {
	configs := make([]thresholdConfig, len(ts.Thresholds))
	for i, t := range ts.Thresholds {
		configs[i].Threshold = t.Source
		configs[i].AbortOnFail = t.AbortOnFail
		configs[i].AbortGracePeriod = t.AbortGracePeriod
	}

	return MarshalJSONWithoutHTMLEscape(configs)
}

// MarshalJSONWithoutHTMLEscape marshals t to JSON without escaping characters
// for safe use in HTML.
func MarshalJSONWithoutHTMLEscape(t interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	bytes := buffer.Bytes()
	if err == nil && len(bytes) > 0 {
		// Remove the newline appended by Encode() :-/
		// See https://github.com/golang/go/issues/37083
		bytes = bytes[:len(bytes)-1]
	}
	return bytes, err
}

var _ json.Unmarshaler = &Thresholds{}
var _ json.Marshaler = &Thresholds{}
