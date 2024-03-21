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
var DefaultSystemTagSet = NewNullSystemTagSet(
	TagProto | TagSubproto | TagStatus | TagMethod | TagURL | TagName | TagGroup |
		TagCheck | TagError | TagErrorCode | TagTLSVersion | TagScenario | TagService | TagExpectedResponse)

// NonIndexableSystemTags are high cardinality system tags (i.e. metadata).
//
//nolint:gochecknoglobals
var NonIndexableSystemTags = NewNullSystemTagSet(TagIter | TagVU)

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

// NullSystemTagSet is a wrapper around SystemTagSet like guregu/null
type NullSystemTagSet struct {
	Set   *SystemTagSet
	Valid bool
}

// ToNullSystemTagSet converts list of tags to NullSystemTagSet
func ToNullSystemTagSet(tags []string) NullSystemTagSet {
	return NullSystemTagSet{
		Set:   ToSystemTagSet(tags),
		Valid: true,
	}
}

// NewNullSystemTagSet returns valid (Valid: true) SystemTagSet
func NewNullSystemTagSet(tags ...SystemTag) NullSystemTagSet {
	return NullSystemTagSet{
		Set:   NewSystemTagSet(tags...),
		Valid: true,
	}
}

// Add adds a tag to tag set.
func (n *NullSystemTagSet) Add(tag SystemTag) {
	if n.Set == nil {
		n.Set = new(SystemTagSet)
		n.Valid = true
	}
	n.Set.Add(tag)
}

// Has checks a tag included in tag set.
func (n NullSystemTagSet) Has(tag SystemTag) bool {
	if !n.Valid {
		return false
	}
	return n.Set.Has(tag)
}

// Map returns the EnabledTags with current value from SystemTagSet
func (n NullSystemTagSet) Map() EnabledTags {
	return n.Set.Map()
}

// SetString returns comma separated list of the string representation of all values in the set
func (n NullSystemTagSet) SetString() string {
	return n.Set.SetString()
}

// MarshalJSON converts NullSystemTagSet to valid JSON
func (n NullSystemTagSet) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte(nullJSON), nil
	}

	return n.Set.MarshalJSON()
}

// UnmarshalJSON converts JSON to NullSystemTagSet
func (n *NullSystemTagSet) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte(nullJSON)) {
		n.Set = nil
		n.Valid = false
		return nil
	}

	var set SystemTagSet
	if err := json.Unmarshal(data, &set); err != nil {
		return err
	}
	n.Set = &set
	n.Valid = true
	return nil
}

// UnmarshalText converts the tag list to SystemTagSet.
func (n *NullSystemTagSet) UnmarshalText(data []byte) error {
	var set SystemTagSet
	if err := set.UnmarshalText(data); err != nil {
		return err
	}
	n.Set = &set
	n.Valid = true
	return nil
}

const nullJSON = "null"
