package lib

import (
	"bytes"
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
		// TODO: use a custom error?
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
// TODO: use Go 2.0/xerrors style errors so we don't lose error type information and
// metadata.
func ConcatErrors(errors []error, separator string) string {
	errStrings := make([]string, len(errors))
	for i, e := range errors {
		errStrings[i] = e.Error()
	}
	return strings.Join(errStrings, separator)
}
