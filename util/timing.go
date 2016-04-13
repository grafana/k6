package util

import (
	"runtime"
	"time"
)

func Time(fn func()) time.Duration {
	m := runtime.MemStats{}
	runtime.ReadMemStats(&m)

	numGC1 := m.NumGC

	startTime := time.Now()
	fn()
	duration := time.Since(startTime)

	runtime.ReadMemStats(&m)
	numGC2 := m.NumGC

	gcTotal := uint64(0)
	for i := numGC1; i < numGC2; i++ {
		gcTotal += m.PauseNs[(i+255)%256]
	}

	return duration - time.Duration(gcTotal)
}
