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

// DefaultSystemTagList includes all of the system tags emitted with metrics by default.
// Other tags that are not enabled by default include: iter, vu, ocsp_status, ip
//nolint:gochecknoglobals
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

// Map returns the SystemTagMap with current value from SystemTagSet
func (i *SystemTagSet) Map() SystemTagMap {
	m := SystemTagMap{}
	for _, tag := range SystemTagSetValues() {
		if i.Has(tag) {
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
func (i *SystemTagSet) MarshalJSON() ([]byte, error) {
	var tags []string
	for _, tag := range SystemTagSetValues() {
		if i.Has(tag) {
			tags = append(tags, tag.String())
		}
	}
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
	var list = bytes.Split(data, []byte(","))

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
