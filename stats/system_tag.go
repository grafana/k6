package stats

import (
	"bytes"
	"encoding/json"
	"strings"
)

// SystemTagSet is a bitmask that is used to keep track
// which system tags should be included with which metrics.
//go:generate enumer -type=SystemTagSet -transform=snake -trimprefix=Tag -output system_tag_set_gen.go
type SystemTagSet uint32

// SystemTagMap is a string to bool map (for lookup efficiency) that is used to keep track
// which system tags should be included with with metrics.
type SystemTagMap map[string]bool

// DefaultSystemTagList includes all of the system tags emitted with metrics by default.
// Other tags that are not enabled by default include: iter, vu, ocsp_status, ip
var DefaultSystemTagList = []string{
	TagProto.String(),
	TagSubProto.String(),
	TagStatus.String(),
	TagMethod.String(),
	TagURL.String(),
	TagName.String(),
	TagGroup.String(),
	TagCheck.String(),
	TagCheck.String(),
	TagError.String(),
	TagErrorCode.String(),
	TagTLSVersion.String(),
}

//nolint: golint
const (
	// Default system tags includes all of the system tags emitted with metrics by default.
	TagProto SystemTagSet = 1 << iota
	TagSubProto
	TagStatus
	TagMethod
	TagURL
	TagName
	TagGroup
	TagCheck
	TagError
	TagErrorCode
	TagTLSVersion

	// System tags not enabled by default.
	TagIter
	TagVU
	TagOCSPStatus
	TagIP
)

// Add adds a tag to tag set.
func (ts *SystemTagSet) Add(tag SystemTagSet) {
	if ts == nil {
		ts = new(SystemTagSet)
	}
	*ts |= tag
}

// Has checks a tag included in tag set.
func (ts *SystemTagSet) Has(tag SystemTagSet) bool {
	if ts == nil {
		return false
	}
	return *ts&tag != 0
}

// Map returns the SystemTagMap with current value from SystemTagSet
func (ts *SystemTagSet) Map() SystemTagMap {
	m := SystemTagMap{}
	for _, tag := range SystemTagSetValues() {
		if ts.Has(tag) {
			m[tag.String()] = true
		}
	}
	return m
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
