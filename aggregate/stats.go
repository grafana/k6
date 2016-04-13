package aggregate

import (
	"github.com/loadimpact/speedboat/runner"
)

type Stats struct {
	Results int64
	Time    DurationStat
}

func (s *Stats) Ingest(res *runner.Result) {
	s.Results++
	s.Time.Ingest(res.Time)
}

func (s *Stats) End() {
	s.Time.End()
}
