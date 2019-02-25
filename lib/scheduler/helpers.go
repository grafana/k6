package scheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// A helper function to verify percentage distributions
func checkPercentagesSum(percentages []float64) error {
	var sum float64
	for _, v := range percentages {
		sum += v
	}
	if math.Abs(100-sum) >= minPercentage {
		return fmt.Errorf("split percentage sum is %.2f while it should be 100", sum)
	}
	return nil
}

// A helper function for joining error messages into a single string
func concatErrors(errors []error, separator string) string {
	errStrings := make([]string, len(errors))
	for i, e := range errors {
		errStrings[i] = e.Error()
	}
	return strings.Join(errStrings, separator)
}

// Decode a JSON in a strict manner, emitting an error if there are unknown fields
func strictJSONUnmarshal(data []byte, v interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	dec.UseNumber()

	if err := dec.Decode(&v); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("unexpected data after the JSON object")
	}
	return nil
}

// A helper function to avoid code duplication
func validateStages(stages []Stage) []error {
	var errors []error
	if len(stages) == 0 {
		errors = append(errors, fmt.Errorf("at least one stage has to be specified"))
	} else {
		for i, s := range stages {
			stageNum := i + 1
			if !s.Duration.Valid {
				errors = append(errors, fmt.Errorf("stage %d doesn't have a duration", stageNum))
			} else if s.Duration.Duration < 0 {
				errors = append(errors, fmt.Errorf("the duration for stage %d shouldn't be negative", stageNum))
			}
			if !s.Target.Valid {
				errors = append(errors, fmt.Errorf("stage %d doesn't have a target", stageNum))
			} else if s.Target.Int64 < 0 {
				errors = append(errors, fmt.Errorf("the target for stage %d shouldn't be negative", stageNum))
			}
		}
	}
	return errors
}
