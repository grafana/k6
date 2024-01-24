package lib

// Max returns the maximum value of a and b.
// TODO: replace after 1.21 is the minimal supported version.
func Max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// Min returns the minimum value of a and b.
// TODO: replace after 1.21 is the minimal supported version.
func Min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
