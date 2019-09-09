package tagset

import (
	"bytes"
	"encoding/json"
	"strings"
)

// TagSet is a bitmask that is used to keep track
// which system tags should be included with which metrics.
//go:generate enumer -type=TagSet -transform=snake -output tagset_gen.go
type TagSet uint32

//nolint: golint
const (
	// Default system tags includes all of the system tags emitted with metrics by default.
	Proto TagSet = 1 << iota
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
func (ts *TagSet) Add(tag TagSet) {
	*ts |= tag
}

// Has checks a tag included in tag set.
func (ts *TagSet) Has(tag TagSet) bool {
	return *ts&tag != 0
}

// FromList converts list of tags to TagSet
func FromList(tags []string) *TagSet {
	ts := TagSet(0)
	for _, tag := range tags {
		if v, err := TagSetString(tag); err == nil {
			ts.Add(v)
		}
	}
	return &ts
}

// MarshalJSON converts the TagSet to a list (JS array).
func (ts *TagSet) MarshalJSON() ([]byte, error) {
	var tags []string
	for _, tag := range TagSetValues() {
		if ts.Has(tag) {
			tags = append(tags, tag.String())
		}
	}
	return json.Marshal(tags)
}

// UnmarshalJSON converts the tag list back to expected tag set.
func (ts *TagSet) UnmarshalJSON(data []byte) error {
	var tags []string
	if err := json.Unmarshal(data, &tags); err != nil {
		return err
	}
	if len(tags) != 0 {
		*ts = *FromList(tags)
	}
	return nil
}

// UnmarshalText converts the tag list to TagSet.
func (ts *TagSet) UnmarshalText(data []byte) error {
	var list = bytes.Split(data, []byte(","))

	for _, key := range list {
		key := strings.TrimSpace(string(key))
		if key == "" {
			continue
		}
		if v, err := TagSetString(key); err == nil {
			ts.Add(v)
		}
	}
	return nil
}
