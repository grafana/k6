package metrics

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

// SystemTag values are bit-shifted identifiers of all of the various system tags that k6 has.
//
//go:generate enumer -type=SystemTag -transform=snake -trimprefix=Tag -output system_tag_gen.go
type SystemTag uint32

// SystemTagSet is a bitmask that is used to keep track which system tags should
// be included in metric Samples.
type SystemTagSet SystemTag

// Default system tags includes all of the system tags emitted with metrics by default.
const (
	TagProto SystemTag = 1 << iota
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
	TagExpectedResponse

	// System tags not enabled by default.
	TagIter // non-indexable
	TagVU   // non-indexable
	TagOCSPStatus
	TagIP
)

// DefaultSystemTagSet includes all of the system tags emitted with metrics by default.
// Other tags that are not enabled by default include: iter, vu, ocsp_status, ip
//
//nolint:gochecknoglobals
var DefaultSystemTagSet = SystemTagSet(
	TagProto | TagSubproto | TagStatus | TagMethod | TagURL | TagName | TagGroup |
		TagCheck | TagError | TagErrorCode | TagTLSVersion | TagScenario | TagService | TagExpectedResponse)

// NonIndexableSystemTags are high cardinality system tags (i.e. metadata).
//
//nolint:gochecknoglobals
var NonIndexableSystemTags = SystemTagSet(TagIter | TagVU)

// Add adds a tag to tag set.
func (i *SystemTagSet) Add(tag SystemTag) {
	if i == nil {
		i = new(SystemTagSet)
	}
	*i |= SystemTagSet(tag)
}

// Has checks a tag included in tag set.
func (i *SystemTagSet) Has(tag SystemTag) bool {
	if i == nil {
		return false
	}
	return *i&SystemTagSet(tag) != 0
}

// Map returns the EnabledTags with current value from SystemTagSet
func (i SystemTagSet) Map() EnabledTags {
	m := EnabledTags{}
	for _, tag := range SystemTagValues() {
		if i.Has(tag) {
			m[tag.String()] = true
		}
	}
	return m
}

// SetString returns comma separated list of the string representation of all values in the set
func (i SystemTagSet) SetString() string {
	var keys []string
	for _, tag := range SystemTagValues() {
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
		if v, err := SystemTagString(tag); err == nil {
			ts.Add(v)
		}
	}
	return ts
}

// NewSystemTagSet returns a SystemTagSet from input.
func NewSystemTagSet(tags ...SystemTag) *SystemTagSet {
	ts := new(SystemTagSet)
	for _, tag := range tags {
		ts.Add(tag)
	}
	return ts
}

// MarshalJSON converts the SystemTagSet to a list (JS array).
func (i *SystemTagSet) MarshalJSON() ([]byte, error) {
	var tags []string
	for _, tag := range SystemTagValues() {
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
		if v, err := SystemTagString(key); err == nil {
			i.Add(v)
		}
	}
	return nil
}
