package metrics

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/mailru/easyjson"
	"github.com/mailru/easyjson/jlexer"
	"github.com/mailru/easyjson/jwriter"
	"github.com/mstoykov/atlas"
)

// A TagSet represents an immutable set of metric tags. For the efficient and
// thread-safe storage of the key=value tag pairs, it uses the
// https://github.com/mstoykov/atlas data structure.
type TagSet atlas.Node

// With returns another TagSet object that contains the combination of the
// current receiver tags and the name=value tag from its parameters. It doesn't
// modify the receiver, it will either return an already existing TagSet with
// these tags, if it exists, or create a new TagSet with them and return it.
func (ts *TagSet) With(name, value string) *TagSet {
	return (*TagSet)(((*atlas.Node)(ts)).AddLink(name, value))
}

// Without returns another TagSet, either an already existing one or a newly
// created one, with all of the tags from the existing TagSet except the one
// with the given key.
func (ts *TagSet) Without(name string) *TagSet {
	return (*TagSet)(((*atlas.Node)(ts)).DeleteKey(name))
}

// Get returns the value of the tag with the given name and true, if that tag
// exists in the set, and an empty string and false otherwise.
func (ts *TagSet) Get(name string) (string, bool) {
	return ((*atlas.Node)(ts)).ValueByKey(name)
}

// Contains checks that each key=value tag pair in the provided TagSet exists in
// the receiver tag set as well, i.e. that the given set is a sub-set of it.
func (ts *TagSet) Contains(other *TagSet) bool {
	return ((*atlas.Node)(ts)).Contains((*atlas.Node)(other))
}

// IsEmpty checks if the tag set is empty, i.e. if it's the root atlas node.
func (ts *TagSet) IsEmpty() bool {
	return ((*atlas.Node)(ts)).IsRoot()
}

// Map returns a {key: value} string map with all of the tags in the set.
func (ts *TagSet) Map() map[string]string {
	return ((*atlas.Node)(ts)).Path()
}

// SortAndAddTags sorts the given tags by their keys and adds them to the
// current tag set one by one, without branching out. This is generally
// discouraged and sequential usage of the AddTag() method should be preferred
// and used whenever possible.
//
// The only place this method should be used is if we already have a
// map[string]string of tags, e.g. with the test-wide --tags from the root, with
// scenario.tags, with custom per-reqeust tags from a user, etc. Then it's more
// efficient to sort their keys before we add them. If we don't, go map
// iteration happens in pseudo-random order and this will generate a lot of
// useless dead-end atlas Nodes.
func (ts *TagSet) SortAndAddTags(m map[string]string) *TagSet {
	if len(m) == 0 {
		return ts
	}

	// We sort the keys so the TagSet generation is consistent across multiple
	// invocations. This should create fewer dead-end atlas Nodes.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	tags := ts
	for i := 0; i < len(keys); i++ {
		tags = tags.With(keys[i], m[keys[i]])
	}

	return tags
}

// MarshalEasyJSON supports easyjson.Marshaler interface for better performance.
func (ts *TagSet) MarshalEasyJSON(w *jwriter.Writer) {
	w.RawByte('{')
	first := true

	n := (*atlas.Node)(ts)
	for !n.IsRoot() {
		prev, key, value := n.Data()
		if first {
			first = false
		} else {
			w.RawByte(',')
		}
		w.String(key)
		w.RawByte(':')
		w.String(value)
		n = prev
	}
	w.RawByte('}')
}

// MarshalJSON serializes the tags to a JSON string.
func (ts *TagSet) MarshalJSON() ([]byte, error) {
	w := &jwriter.Writer{NoEscapeHTML: true}
	ts.MarshalEasyJSON(w)
	return w.Buffer.Buf, w.Error
}

// UnmarshalEasyJSON WILL ALWAYS RETURN AN ERROR because a TagSet needs to be
// started from a common atlas root. This function exists to prevent any
// automatic reflection-based attempts at unmarshaling.
func (ts *TagSet) UnmarshalEasyJSON(l *jlexer.Lexer) {
	l.AddError(errors.New("metrics.TagSet cannot be directly unmarshalled from JSON"))
}

// UnmarshalJSON WILL ALWAYS RETURN AN ERROR because a TagSet needs to be
// started from a common atlas root. This function exists to prevent any
// automatic reflection-based attempts at unmarshaling.
func (ts *TagSet) UnmarshalJSON([]byte) error {
	return errors.New("metrics.TagSet cannot be directly unmarshalled from JSON")
}

var _ interface {
	easyjson.Marshaler
	easyjson.Unmarshaler
	json.Marshaler
	json.Unmarshaler
} = &TagSet{}

// EnabledTags is a string to bool map (for lookup efficiency) that is used to keep track
// of which system tags and non-system tags to include.
//
// TODO: move to types.StringSet or something like that, this isn't metrics specific
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
