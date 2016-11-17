package lib

import (
	// "math"
	"time"
)

// StageAt returns the stage at the specified offset (in nanoseconds) and the time remaining of
// said stage. If the interval is past the end of the test, an empty stage and 0 is returned.
func StageAt(stages []Stage, offset time.Duration) (s Stage, stageLeft time.Duration, ok bool) {
	var counter time.Duration
	for _, stage := range stages {
		counter += time.Duration(stage.Duration.Int64)
		if counter >= offset {
			return stage, counter - offset, true
		}
	}
	return stages[len(stages)-1], 0, false
}

// Lerp is a linear interpolation between two values x and y, returning the value at the point t,
// where t is a fraction in the range [0.0 - 1.0].
func Lerp(x, y int64, t float64) int64 {
	return x + int64(t*float64(y-x))
}
