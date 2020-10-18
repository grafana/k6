/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package stats

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

// SystemTagSet is a bitmask that is used to keep track
// which system tags should be included with which metrics.
//go:generate enumer -type=SystemTagSet -transform=snake -trimprefix=Tag -output system_tag_set_gen.go
type SystemTagSet uint32

// TagSet is a string to bool map (for lookup efficiency) that is used to keep track
// of which system tags and non-system tags to include.
type TagSet map[string]bool

// UnmarshalText converts the tag list to TagSet.
func (i *TagSet) UnmarshalText(data []byte) error {
	list := bytes.Split(data, []byte(","))
	if *i == nil {
		*i = make(TagSet, len(list))
	}

	for _, key := range list {
		key := strings.TrimSpace(string(key))
		if key == "" {
			continue
		}
		(*i)[key] = true
	}

	return nil
}

// MarshalJSON converts the TagSet to a list (JS array).
func (i *TagSet) MarshalJSON() ([]byte, error) {
	var tags []string
	if *i != nil {
		tags = make([]string, 0, len(*i))
		for tag := range *i {
			tags = append(tags, tag)
		}
		sort.Strings(tags)
	}

	return json.Marshal(tags)
}

// UnmarshalJSON converts the tag list back to expected tag set.
func (i *TagSet) UnmarshalJSON(data []byte) error {
	var tags []string
	if err := json.Unmarshal(data, &tags); err != nil {
		return err
	}
	*i = make(TagSet, len(tags))
	for _, tag := range tags {
		(*i)[tag] = true
	}

	return nil
}

// Default system tags includes all of the system tags emitted with metrics by default.
const (
	TagProto SystemTagSet = 1 << iota
	TagSubproto
	TagStatus
	TagMethod
	TagURL
	TagName
	TagGroup
	TagCheck
	TagError
	TagErrorCode
	TagTLSVersion
	TagScenario
	TagService

	// System tags not enabled by default.
	TagIter
	TagVU
	TagOCSPStatus
	TagIP
)

// DefaultSystemTagSet includes all of the system tags emitted with metrics by default.
// Other tags that are not enabled by default include: iter, vu, ocsp_status, ip
//nolint:gochecknoglobals
var DefaultSystemTagSet = TagProto | TagSubproto | TagStatus | TagMethod | TagURL | TagName | TagGroup |
	TagCheck | TagCheck | TagError | TagErrorCode | TagTLSVersion | TagScenario | TagService

// Add adds a tag to tag set.
func (i *SystemTagSet) Add(tag SystemTagSet) {
	if i == nil {
		i = new(SystemTagSet)
	}
	*i |= tag
}

// Has checks a tag included in tag set.
func (i *SystemTagSet) Has(tag SystemTagSet) bool {
	if i == nil {
		return false
	}
	return *i&tag != 0
}

// Map returns the TagSet with current value from SystemTagSet
func (i SystemTagSet) Map() TagSet {
	m := TagSet{}
	for _, tag := range SystemTagSetValues() {
		if i.Has(tag) {
			m[tag.String()] = true
		}
	}
	return m
}

// SetString returns comma separated list of the string representation of all values in the set
func (i SystemTagSet) SetString() string {
	var keys []string
	for _, tag := range SystemTagSetValues() {
		if i.Has(tag) {
			keys = append(keys, tag.String())
		}
	}
	return strings.Join(keys, ",")
}

// ToSystemTagSet converts list of tags to SystemTagSet
// TODO: emit error instead of discarding invalid values.
func ToSystemTagSet(tags []string) *SystemTagSet {
	ts := new(SystemTagSet)
	for _, tag := range tags {
		if v, err := SystemTagSetString(tag); err == nil {
			ts.Add(v)
		}
	}
	return ts
}

// NewSystemTagSet returns a SystemTagSet from input.
func NewSystemTagSet(tags ...SystemTagSet) *SystemTagSet {
	ts := new(SystemTagSet)
	for _, tag := range tags {
		ts.Add(tag)
	}
	return ts
}

// MarshalJSON converts the SystemTagSet to a list (JS array).
func (i *SystemTagSet) MarshalJSON() ([]byte, error) {
	var tags []string
	for _, tag := range SystemTagSetValues() {
		if i.Has(tag) {
			tags = append(tags, tag.String())
		}
	}
	sort.Strings(tags)

	return json.Marshal(tags)
}

// UnmarshalJSON converts the tag list back to expected tag set.
func (i *SystemTagSet) UnmarshalJSON(data []byte) error {
	var tags []string
	if err := json.Unmarshal(data, &tags); err != nil {
		return err
	}
	if len(tags) != 0 {
		*i = *ToSystemTagSet(tags)
	}

	return nil
}

// UnmarshalText converts the tag list to SystemTagSet.
func (i *SystemTagSet) UnmarshalText(data []byte) error {
	list := bytes.Split(data, []byte(","))

	for _, key := range list {
		key := strings.TrimSpace(string(key))
		if key == "" {
			continue
		}
		if v, err := SystemTagSetString(key); err == nil {
			i.Add(v)
		}
	}
	return nil
}
