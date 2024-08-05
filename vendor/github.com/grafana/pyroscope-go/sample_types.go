package pyroscope

import (
	"github.com/grafana/pyroscope-go/upstream"
)

var (
	sampleTypeConfigHeap = map[string]*upstream.SampleType{
		"alloc_objects": {
			Units:      "objects",
			Cumulative: false,
		},
		"alloc_space": {
			Units:      "bytes",
			Cumulative: false,
		},
		"inuse_space": {
			Units:       "bytes",
			Aggregation: "average",
			Cumulative:  false,
		},
		"inuse_objects": {
			Units:       "objects",
			Aggregation: "average",
			Cumulative:  false,
		},
	}
	sampleTypeConfigMutex = map[string]*upstream.SampleType{
		"contentions": {
			DisplayName: "mutex_count",
			Units:       "lock_samples",
			Cumulative:  false,
		},
		"delay": {
			DisplayName: "mutex_duration",
			Units:       "lock_nanoseconds",
			Cumulative:  false,
		},
	}
	sampleTypeConfigBlock = map[string]*upstream.SampleType{
		"contentions": {
			DisplayName: "block_count",
			Units:       "lock_samples",
			Cumulative:  false,
		},
		"delay": {
			DisplayName: "block_duration",
			Units:       "lock_nanoseconds",
			Cumulative:  false,
		},
	}
)
