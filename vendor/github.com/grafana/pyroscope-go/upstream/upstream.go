package upstream

import (
	"time"
)

type Format string

const FormatPprof Format = "pprof"

type Upstream interface {
	Upload(*UploadJob)
	Flush()
}

type SampleType struct {
	Units       string `json:"units,omitempty"`
	Aggregation string `json:"aggregation,omitempty"`
	DisplayName string `json:"display-name,omitempty"`
	Sampled     bool   `json:"sampled,omitempty"`
	Cumulative  bool   `json:"cumulative,omitempty"`
}

type UploadJob struct {
	Name             string
	StartTime        time.Time
	EndTime          time.Time
	SpyName          string
	SampleRate       uint32
	Units            string
	AggregationType  string
	Format           Format
	Profile          []byte
	PrevProfile      []byte
	SampleTypeConfig map[string]*SampleType
}
