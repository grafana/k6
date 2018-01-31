/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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

package har

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsAllowedURL(t *testing.T) {
	var allowed = []struct {
		url      string
		only     []string
		skip     []string
		expected bool
	}{
		{"http://www.google.com/", []string{}, []string{}, true},
		{"http://www.google.com/", []string{"google.com"}, []string{}, true},
		{"https://www.google.com/", []string{"google.com"}, []string{}, true},
		{"https://www.google.com/", []string{"http://"}, []string{}, false},
		{"http://www.google.com/?hl=en", []string{"http://www.google.com"}, []string{}, true},
		{"http://www.google.com/?hl=en", []string{"google.com", "google.co.uk"}, []string{}, true},
		{"http://www.google.com/?hl=en", []string{}, []string{"google.com"}, false},
		{"http://www.google.com/?hl=en", []string{}, []string{"google.co.uk"}, true},
	}

	for _, s := range allowed {
		v := IsAllowedURL(s.url, s.only, s.skip)
		assert.Equal(t, v, s.expected, fmt.Sprintf("params: %v, %v, %v", s.url, s.only, s.skip))
	}
}

func TestSplitEntriesInBatches(t *testing.T) {
	t1 := time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)

	entries := []*Entry{}

	// 10 time entries with increments of 100ms or 200ms
	for i := 1; i <= 10; i++ {

		period := 100
		if i%2 == 0 {
			period = 200
		}
		t1 = t1.Add(time.Duration(period) * time.Millisecond)
		entries = append(entries, &Entry{StartedDateTime: t1})
	}

	splitValues := []struct {
		diff, groups uint
	}{
		{0, 1},
		{100, 10},
		{150, 6},
		{200, 6},
		{201, 1},
		{500, 1},
	}

	sort.Sort(EntryByStarted(entries))

	for _, v := range splitValues {
		result := SplitEntriesInBatches(entries, v.diff)
		assert.Equal(t, len(result), int(v.groups), fmt.Sprintf("params: entries, %v", v.diff))
	}
}
