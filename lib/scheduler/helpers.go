package scheduler

import (
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
