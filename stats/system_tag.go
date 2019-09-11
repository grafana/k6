package stats

import (
	"bytes"
	"encoding/json"
	"strings"
)

// SystemTagSet is a bitmask that is used to keep track
// which system tags should be included with which metrics.
//go:generate enumer -type=SystemTagSet -transform=snake -output system_tag_set_gen.go
type SystemTagSet uint32

//nolint: golint
const (
	// Default system tags includes all of the system tags emitted with metrics by default.
	Proto SystemTagSet = 1 << iota
	SubProto
	Status
	Method
	URL
	Name
	Group
	Check
	Error
	ErrorCode
	TLSVersion

	// System tags not enabled by default.
	Iter
	VU
	OCSPStatus
	IP
)

// Add adds a tag to tag set.
func (ts *SystemTagSet) Add(tag SystemTagSet) {
	*ts |= tag
}

// Has checks a tag included in tag set.
func (ts *SystemTagSet) Has(tag SystemTagSet) bool {
	return *ts&tag != 0
}

// ToSystemTagSet converts list of tags to SystemTagSet
func ToSystemTagSet(tags []string) *SystemTagSet {
	ts := SystemTagSet(0)
	for _, tag := range tags {
		if v, err := SystemTagSetString(tag); err == nil {
			ts.Add(v)
		}
	}
	return &ts
}

// MarshalJSON converts the SystemTagSet to a list (JS array).
func (ts *SystemTagSet) MarshalJSON() ([]byte, error) {
	var tags []string
	for _, tag := range SystemTagSetValues() {
		if ts.Has(tag) {
			tags = append(tags, tag.String())
		}
	}
	return json.Marshal(tags)
}

// UnmarshalJSON converts the tag list back to expected tag set.
func (ts *SystemTagSet) UnmarshalJSON(data []byte) error {
	var tags []string
	if err := json.Unmarshal(data, &tags); err != nil {
		return err
	}
	if len(tags) != 0 {
		*ts = *ToSystemTagSet(tags)
	}
	return nil
}

// UnmarshalText converts the tag list to SystemTagSet.
func (ts *SystemTagSet) UnmarshalText(data []byte) error {
	var list = bytes.Split(data, []byte(","))

	for _, key := range list {
		key := strings.TrimSpace(string(key))
		if key == "" {
			continue
		}
		if v, err := SystemTagSetString(key); err == nil {
			ts.Add(v)
		}
	}
	return nil
}
