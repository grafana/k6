package metrics

import (
	"time"
)

const timeUnit = time.Millisecond

// D formats a duration for emission.
// The reverse of D() is ToD().
func D(d time.Duration) float64 {
	return float64(d) / float64(timeUnit)
}

// ToD converts an emitted duration to a time.Duration.
// The reverse of ToD() is D().
func ToD(d float64) time.Duration {
	return time.Duration(d * float64(timeUnit))
}

// B formats a boolean value for emission.
func B(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
