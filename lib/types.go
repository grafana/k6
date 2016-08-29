package lib

import (
	"time"
)

type Status struct {
	StartTime time.Time `json:"startTime" yaml:"startTime"`

	Running bool  `json:"running" yaml:"running"`
	VUs     int64 `json:"vus" yaml:"vus"`
}

type Info struct {
	Version string `json:"version"`
}
