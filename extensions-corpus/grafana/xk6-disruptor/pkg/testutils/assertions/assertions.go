// Package assertions implements functions that help assess conditions in tests
package assertions

import "sort"

// CompareStringArrays compares if two arrays of strings has the same elements
func CompareStringArrays(a, b []string) bool {
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}

	if len(a) == 0 {
		return true
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
