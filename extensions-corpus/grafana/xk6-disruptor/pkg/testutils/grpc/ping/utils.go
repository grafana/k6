package ping

import (
	"strings"

	"google.golang.org/grpc/metadata"
)

// CompareResponses returns a bool indicating if the actual and expected PingResponses are equal
func CompareResponses(actual, expected *PingResponse) bool {
	if expected == nil && actual == nil {
		return true
	}

	if expected == nil || actual == nil {
		return false
	}

	if expected.Message == actual.Message {
		return true
	}

	return false
}

// CompareHeaders compares the actual metadata with an expected map of headers.
// The actual header's values are expected as a comma-separated list of values (instead of a string array)
func CompareHeaders(actual metadata.MD, expected map[string]string) bool {
	for key, value := range expected {
		// expected value is a list of comma separated values
		expectedValues := strings.Split(value, ",")
		actualValues := actual.Get(key)
		if len(actualValues) != len(expectedValues) {
			return false
		}
		for i, v := range actualValues {
			if v != expectedValues[i] {
				return false
			}
		}
	}
	return true
}
