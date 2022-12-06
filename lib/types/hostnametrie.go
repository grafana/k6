package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// NullHostnameTrie is a nullable HostnameTrie, in the same vein as the nullable types provided by
// package gopkg.in/guregu/null.v3
type NullHostnameTrie struct {
	Trie  *HostnameTrie
	Valid bool
}

// UnmarshalText converts text data to a valid NullHostnameTrie
func (d *NullHostnameTrie) UnmarshalText(data []byte) error {
	if len(data) == 0 {
		*d = NullHostnameTrie{}
		return nil
	}
	var err error
	d.Trie, err = NewHostnameTrie(strings.Split(string(data), ","))
	if err != nil {
		return err
	}
	d.Valid = true
	return nil
}

// UnmarshalJSON converts JSON data to a valid NullHostnameTrie
func (d *NullHostnameTrie) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte(`null`)) {
		d.Valid = false
		return nil
	}

	var m []string
	var err error
	if err = json.Unmarshal(data, &m); err != nil {
		return err
	}
	d.Trie, err = NewHostnameTrie(m)
	if err != nil {
		return err
	}
	d.Valid = true
	return nil
}

// Source return source hostnames that were used diring the construction
func (d *NullHostnameTrie) Source() []string {
	if d.Trie == nil {
		return []string{}
	}

	return d.Trie.source
}

// MarshalJSON implements json.Marshaler interface
func (d NullHostnameTrie) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte(`null`), nil
	}
	return json.Marshal(d.Trie.source)
}

// HostnameTrie is a tree-structured list of hostname matches with support
// for wildcards exclusively at the start of the pattern. Items may only
// be inserted and searched. Internationalized hostnames are valid.
type HostnameTrie struct {
	*trieNode
	source []string
}

// NewNullHostnameTrie returns a NullHostnameTrie encapsulating HostnameTrie or an error if the
// input is incorrect
func NewNullHostnameTrie(source []string) (NullHostnameTrie, error) {
	h, err := NewHostnameTrie(source)
	if err != nil {
		return NullHostnameTrie{}, err
	}
	return NullHostnameTrie{
		Valid: true,
		Trie:  h,
	}, nil
}

// NewHostnameTrie returns a pointer to a new HostnameTrie or an error if the input is incorrect
func NewHostnameTrie(source []string) (*HostnameTrie, error) {
	h := &HostnameTrie{
		source: source,
		trieNode: &trieNode{
			isLeaf:   false,
			children: make(map[rune]*trieNode),
		},
	}
	for _, s := range h.source {
		if err := h.insert(s); err != nil {
			return nil, err
		}
	}
	return h, nil
}

// Regex description of hostname pattern to enforce blocks by.
// Global var to avoid compilation penalty at runtime.
// Based on regex from https://stackoverflow.com/a/106223/5427244
//
//nolint:lll
var validHostnamePattern *regexp.Regexp = regexp.MustCompile(`^(\*\.?)?((([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9]))?$`)

func isValidHostnamePattern(s string) error {
	if len(validHostnamePattern.FindString(s)) != len(s) {
		return fmt.Errorf("invalid hostname pattern '%s'", s)
	}
	return nil
}

// insert a hostname pattern into the given HostnameTrie. Returns an error
// if hostname pattern is invalid.
func (t *HostnameTrie) insert(s string) error {
	s = strings.ToLower(s)
	if err := isValidHostnamePattern(s); err != nil {
		return err
	}

	t.trieNode.insert(s)
	return nil
}

// Contains returns whether s matches a pattern in the HostnameTrie
// along with the matching pattern, if one was found.
func (t *HostnameTrie) Contains(s string) (matchedPattern string, matchFound bool) {
	s = strings.ToLower(s)
	return t.trieNode.contains(s)
}
