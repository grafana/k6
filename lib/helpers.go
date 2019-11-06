/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// StrictJSONUnmarshal decodes a JSON in a strict manner, emitting an error if there
// are unknown fields or unexpected data
func StrictJSONUnmarshal(data []byte, v interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	dec.UseNumber()

	if err := dec.Decode(&v); err != nil {
		return err
	}
	if dec.More() {
		//TODO: use a custom error?
		return fmt.Errorf("unexpected data after the JSON object")
	}
	return nil
}

// GetMaxPlannedVUs returns the maximum number of planned VUs at any stage of
// the execution plan.
func GetMaxPlannedVUs(steps []ExecutionStep) (result uint64) {
	for _, s := range steps {
		stepMaxPlannedVUs := s.PlannedVUs
		if stepMaxPlannedVUs > result {
			result = stepMaxPlannedVUs
		}
	}
	return result
}

// GetMaxPossibleVUs returns the maximum number of planned + unplanned (i.e.
// initialized mid-test) VUs at any stage of the execution plan. Unplanned VUs
// are possible in some executors, like the arrival-rate ones, as a way to have
// a low number of pre-allocated VUs, but be able to initialize new ones in the
// middle of the test, if needed. For example, if the remote system starts
// responding very slowly and all of the pre-allocated VUs are waiting for it.
//
// IMPORTANT 1: Getting planned and unplanned VUs separately for the whole
// duration of a test can often lead to mistakes. That's why this function is
// called GetMaxPossibleVUs() and why there is no GetMaxUnplannedVUs() function.
//
// As an example, imagine that you have an executor with MaxPlannedVUs=20 and
// MaxUnplannedVUs=0, followed immediately after by another executor with
// MaxPlannedVUs=10 and MaxUnplannedVUs=10. The MaxPlannedVUs number for the
// whole test is 20, and MaxUnplannedVUs is 10, but since those executors won't
// run concurrently, MaxVUs for the whole test is not 30, rather it's 20, since
// 20 VUs will be sufficient to run the whole test.
//
// IMPORTANT 2: this has one very important exception. The externally controlled
// executor doesn't use the MaxUnplannedVUs (i.e. this function will return 0),
// since their initialization and usage is directly controlled by the user and
// is effectively bounded only by the resources of the machine k6 is running on.
func GetMaxPossibleVUs(steps []ExecutionStep) (result uint64) {
	for _, s := range steps {
		stepMaxPossibleVUs := s.PlannedVUs + s.MaxUnplannedVUs
		if stepMaxPossibleVUs > result {
			result = stepMaxPossibleVUs
		}
	}
	return result
}

// GetEndOffset returns the time offset of the last step of the execution plan,
// and whether that step is a final one, i.e. whether the number of planned or
// unplanned is 0.
func GetEndOffset(steps []ExecutionStep) (lastStepOffset time.Duration, isFinal bool) {
	if len(steps) == 0 {
		return 0, true
	}
	lastStep := steps[len(steps)-1]
	return lastStep.TimeOffset, (lastStep.PlannedVUs == 0 && lastStep.MaxUnplannedVUs == 0)
}

// ConcatErrors is a a helper function for joining error messages into a single
// string.
//
// TODO: use Go 2.0/xerrors style errors so we don't loose error type information and
// metadata.
func ConcatErrors(errors []error, separator string) string {
	errStrings := make([]string, len(errors))
	for i, e := range errors {
		errStrings[i] = e.Error()
	}
	return strings.Join(errStrings, separator)
}

// StreamExecutionSteps launches a new goroutine and emits all execution steps
// at their appropriate time offsets over the returned unbuffered channel. If
// closeChanWhenDone is specified, it will close the channel after it sends the
// last step. If it isn't, or if the context is cancelled, the internal
// goroutine will be stopped, *but the channel will remain open*!
//
// As usual, steps in the supplied slice have to be sorted by their TimeOffset
// values in an ascending order. Of course, multiple events can have the same
// time offset (incl. 0).
func StreamExecutionSteps(
	ctx context.Context, startTime time.Time, steps []ExecutionStep, closeChanWhenDone bool,
) <-chan ExecutionStep {
	ch := make(chan ExecutionStep)
	go func() {
		for _, step := range steps {
			offsetDiff := step.TimeOffset - time.Since(startTime)
			if offsetDiff > 0 { // wait until time of event arrives
				select {
				case <-ctx.Done():
					return // exit if context is cancelled
				case <-time.After(offsetDiff): //TODO: reuse a timer?
					// do nothing
				}
			}
			select {
			case <-ctx.Done():
				// exit if context is cancelled
				return
			case ch <- step:
				// ... otherwise, just send the step - the out channel is
				// unbuffered, so we don't need to worry whether the other side
				// will keep reading after the context is done.
			}
		}

		// Close the channel only if all steps were sent successfully (i.e. the
		// parent context didn't die) and we were instructed to do so.
		if closeChanWhenDone {
			close(ch)
		}
	}()
	return ch
}
