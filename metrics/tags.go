package metrics

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/mstoykov/atlas"
)

// TagSet represents a set of tags.
type TagSet struct {
	tags *atlas.Node
}

// NewTagSet creates a TagSet, if a not-nil map is passed
// then it adds all the pairs to the set, otherwise an empty set is created.
// Under the hood it initializes an Atlas root node.
// It should be only used in a centralized place and
// all the new TagSet should branch from it for getting
// the optimal performances.
func NewTagSet(m map[string]string) *TagSet {
	node := atlas.New()
	for k, v := range m {
		node = node.AddLink(k, v)
	}

	return &TagSet{
		tags: node,
	}
}

// TagSetFromSampleTags creates a TagSet starting
// from the set defined by SampleTags.
// Adding a tag on the new set doesn't impact the SampleTags.
func TagSetFromSampleTags(st *SampleTags) *TagSet {
	tm := &TagSet{}
	tm.tags = st.tags
	return tm
}

// AddTag adds a tag in the set.
func (tg *TagSet) AddTag(k, v string) {
	tg.tags = tg.tags.AddLink(k, v)
}

// Get returns the Tag value and true
// if the provided key has been found.
func (tg *TagSet) Get(k string) (string, bool) {
	return tg.tags.ValueByKey(k)
}

// Len returns the number of the set keys.
func (tg *TagSet) Len() int {
	return tg.tags.Len()
}

// Delete deletes the item related to the provided key.
func (tg *TagSet) Delete(k string) {
	tg.tags = tg.tags.DeleteKey(k)
}

// Map returns a map of string pairs with the items in the TagSet.
func (tg *TagSet) Map() map[string]string {
	return tg.tags.Path()
}

// BranchOut creates a new TagSet with the same set of items
// as in the current TagSet.
// Any new added tag to the new set creates a new dedicated path.
func (tg *TagSet) BranchOut() *TagSet {
	tmcopy := &TagSet{}
	tmcopy.tags = tg.tags
	return tmcopy
}

// SampleTags creates a SampleTags using the current tag set.
func (tg *TagSet) SampleTags() *SampleTags {
	st := &SampleTags{}
	st.tags = tg.tags
	return st
}

// EnabledTags is a string to bool map (for lookup efficiency) that is used to keep track
// of which system tags and non-system tags to include.
type EnabledTags map[string]bool

// UnmarshalText converts the tag list to EnabledTags.
func (i *EnabledTags) UnmarshalText(data []byte) error {
	list := bytes.Split(data, []byte(","))
	if *i == nil {
		*i = make(EnabledTags, len(list))
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

// MarshalJSON converts the EnabledTags to a list (JS array).
func (i *EnabledTags) MarshalJSON() ([]byte, error) {
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
func (i *EnabledTags) UnmarshalJSON(data []byte) error {
	var tags []string
	if err := json.Unmarshal(data, &tags); err != nil {
		return err
	}
	*i = make(EnabledTags, len(tags))
	for _, tag := range tags {
		(*i)[tag] = true
	}

	return nil
}
