/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package segment

import (
	"sync"

	"go.k6.io/k6/lib"
)

// SegmentedIndexResult wraps a computed segment.
type SegmentedIndexResult struct {
	Scaled, Unscaled int64
}

// SegmentedIndex wraps a lib.SegmentedIndex making it a concurrent-safe iterator.
type SegmentedIndex struct {
	index *lib.SegmentedIndex
	rwm   sync.RWMutex
}

// NewSegmentedIndex returns a pointer to a new SegmentedIndex instance,
// given a lib.ExecutionTuple.
func NewSegmentedIndex(et *lib.ExecutionTuple) *SegmentedIndex {
	return &SegmentedIndex{
		index: lib.NewSegmentedIndex(et),
	}
}

// Next goes to the next segment's point in the iterator.
func (s *SegmentedIndex) Next() SegmentedIndexResult {
	s.rwm.Lock()
	defer s.rwm.Unlock()

	scaled, unscaled := s.index.Next()
	return SegmentedIndexResult{
		Scaled:   scaled,
		Unscaled: unscaled,
	}
}

// Prev goes to the previous segment's point in the iterator.
func (s *SegmentedIndex) Prev() SegmentedIndexResult {
	s.rwm.Lock()
	defer s.rwm.Unlock()

	scaled, unscaled := s.index.Prev()
	return SegmentedIndexResult{
		Scaled:   scaled,
		Unscaled: unscaled,
	}
}

// GoTo TODO: document
func (s *SegmentedIndex) GoTo(value int64) SegmentedIndexResult {
	s.rwm.Lock()
	defer s.rwm.Unlock()

	scaled, unscaled := s.index.GoTo(value)
	return SegmentedIndexResult{Scaled: scaled, Unscaled: unscaled}
}

type sharedSegmentedIndexes struct {
	data map[string]*SegmentedIndex
	mu   sync.RWMutex
}

func (s *sharedSegmentedIndexes) SegmentedIndex(state *lib.State, name string) (*SegmentedIndex, error) {
	s.mu.RLock()
	array, ok := s.data[name]
	s.mu.RUnlock()
	if ok {
		return array, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	array, ok = s.data[name]
	if !ok {
		tuple, err := lib.NewExecutionTuple(state.Options.ExecutionSegment, state.Options.ExecutionSegmentSequence)
		if err != nil {
			return nil, err
		}
		array = NewSegmentedIndex(tuple)
		s.data[name] = array
	}
	return array, nil
}
