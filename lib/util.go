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
	return Stage{}, 0, false
}

// Ease eases a value x towards y over time, so that: f=f(t) : f(tx)=x, f(ty)=y.
func Ease(t, tx, ty, x, y int64) int64 {
	return x*(ty-t)/(ty-tx) + y*(t-tx)/(ty-tx)
}
