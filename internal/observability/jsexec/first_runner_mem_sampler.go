package jsexec

import (
	"fmt"
	"sort"
	"sync"

	"github.com/grafana/sobek"
)

type memLineStat struct {
	File       string
	Line       int
	AllocSpace int64
}

type memMilestone struct {
	ThresholdBytes uint64
	HeapAllocBytes uint64
	TopFile        string
	TopLine        int
	TopAllocSpace  int64
}

type firstRunnerMemSampler struct {
	maxBytes   uint64
	stepBytes  uint64
	stepPct    int64
	nextMark   uint64
	milestones []memMilestone

	mu    sync.Mutex
	lines map[string]*memLineStat
}

func newFirstRunnerMemSampler(maxBytes uint64, stepPct int64) *firstRunnerMemSampler {
	if stepPct <= 0 {
		stepPct = 5
	}
	stepBytes := (maxBytes * uint64(stepPct)) / 100
	if stepBytes == 0 {
		stepBytes = 1
	}
	return &firstRunnerMemSampler{
		maxBytes:  maxBytes,
		stepBytes: stepBytes,
		stepPct:   stepPct,
		nextMark:  stepBytes,
		lines:     make(map[string]*memLineStat),
	}
}

func (s *firstRunnerMemSampler) observe(sample sobek.ProfileSample) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sample.AllocSpace > 0 && sample.TopFrame.SrcName() != "" && sample.TopFrame.SrcName() != "<native>" {
		pos := sample.TopFrame.Position()
		key := fmt.Sprintf("%s:%d", sample.TopFrame.SrcName(), pos.Line)
		st, ok := s.lines[key]
		if !ok {
			st = &memLineStat{
				File: sample.TopFrame.SrcName(),
				Line: pos.Line,
			}
			s.lines[key] = st
		}
		st.AllocSpace += sample.AllocSpace
	}

	if s.maxBytes == 0 {
		return
	}
	for sample.HeapAlloc >= s.nextMark && s.nextMark <= s.maxBytes {
		top := s.topLineLocked()
		s.milestones = append(s.milestones, memMilestone{
			ThresholdBytes: s.nextMark,
			HeapAllocBytes: sample.HeapAlloc,
			TopFile:        top.File,
			TopLine:        top.Line,
			TopAllocSpace:  top.AllocSpace,
		})
		s.nextMark += s.stepBytes
	}
}

func (s *firstRunnerMemSampler) topLineLocked() memLineStat {
	var best memLineStat
	for _, st := range s.lines {
		if st.AllocSpace > best.AllocSpace {
			best = *st
		}
	}
	return best
}

func (s *firstRunnerMemSampler) topN(n int) []memLineStat {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]memLineStat, 0, len(s.lines))
	for _, st := range s.lines {
		rows = append(rows, *st)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].AllocSpace == rows[j].AllocSpace {
			if rows[i].File == rows[j].File {
				return rows[i].Line < rows[j].Line
			}
			return rows[i].File < rows[j].File
		}
		return rows[i].AllocSpace > rows[j].AllocSpace
	})
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}

func (s *firstRunnerMemSampler) snapshotMilestones() []memMilestone {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]memMilestone, len(s.milestones))
	copy(out, s.milestones)
	return out
}
