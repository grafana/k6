package metrics

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

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
