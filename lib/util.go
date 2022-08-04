package lib

// Returns the maximum value of a and b.
func Max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// Returns the minimum value of a and b.
func Min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
