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
	"encoding/json"
	"io"
	"strings"
	"time"
)

// Define new types to sort
type EntryByStarted []*Entry

func (e EntryByStarted) Len() int { return len(e) }

func (e EntryByStarted) Swap(i, j int) { e[i], e[j] = e[j], e[i] }

func (e EntryByStarted) Less(i, j int) bool {
	return e[i].StartedDateTime.Before(e[j].StartedDateTime)
}

type PageByStarted []Page

func (e PageByStarted) Len() int { return len(e) }

func (e PageByStarted) Swap(i, j int) { e[i], e[j] = e[j], e[i] }

func (e PageByStarted) Less(i, j int) bool {
	return e[i].StartedDateTime.Before(e[j].StartedDateTime)
}

func Decode(r io.Reader) (HAR, error) {
	var har HAR
	if err := json.NewDecoder(r).Decode(&har); err != nil {
		return HAR{}, err
	}

	return har, nil
}

// Returns true if the given url is allowed from the only (only domains) and skip (skip domains) values, otherwise false
func IsAllowedURL(url string, only, skip []string) bool {
	if len(only) != 0 {
		for _, v := range only {
			v = strings.Trim(v, " ")
			if v != "" && strings.Contains(url, v) {
				return true
			}
		}
		return false
	}
	if len(skip) != 0 {
		for _, v := range skip {
			v = strings.Trim(v, " ")
			if v != "" && strings.Contains(url, v) {
				return false
			}
		}
	}
	return true
}

func SplitEntriesInBatches(entries []*Entry, interval uint) [][]*Entry {
	var r [][]*Entry
	r = append(r, []*Entry{})

	if interval > 0 && len(entries) > 1 {
		j := 0
		d := time.Duration(interval) * time.Millisecond
		for i, e := range entries {

			if i != 0 {
				prev := entries[i-1]
				if e.StartedDateTime.Sub(prev.StartedDateTime) >= d {
					r = append(r, []*Entry{})
					j++
				}
			}
			r[j] = append(r[j], e)
		}
	} else {
		r[0] = entries
	}

	return r
}
