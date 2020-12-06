/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package types

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/pkg/errors"
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

// Regex description of hostname pattern to enforce blocks by. Global var
// to avoid compilation penalty at runtime.
// based on regex from https://stackoverflow.com/a/106223/5427244
//nolint:gochecknoglobals,lll
var validHostnamePattern *regexp.Regexp = regexp.MustCompile(`^(\*\.?)?((([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9]))?$`)

func isValidHostnamePattern(s string) error {
	if len(validHostnamePattern.FindString(s)) != len(s) {
		return errors.Errorf("invalid hostname pattern %s", s)
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

	return t.trieNode.insert(s)
}

// Contains returns whether s matches a pattern in the HostnameTrie
// along with the matching pattern, if one was found.
func (t *HostnameTrie) Contains(s string) (matchedPattern string, matchFound bool) {
	return t.trieNode.contains(s)
}

type trieNode struct {
	isLeaf   bool
	children map[rune]*trieNode
}

func (t *trieNode) insert(s string) error {
	if len(s) == 0 {
		t.isLeaf = true
		return nil
	}

	// mask creation of the trie by initializing the root here
	if t.children == nil {
		t.children = make(map[rune]*trieNode)
	}

	rStr := []rune(s) // need to iterate by runes for intl' names
	last := len(rStr) - 1
	if c, ok := t.children[rStr[last]]; ok {
		return c.insert(string(rStr[:last]))
	}

	t.children[rStr[last]] = &trieNode{children: make(map[rune]*trieNode)}
	return t.children[rStr[last]].insert(string(rStr[:last]))
}

func (t *trieNode) contains(s string) (matchedPattern string, matchFound bool) {
	s = strings.ToLower(s)
	if len(s) == 0 {
		if t.isLeaf {
			return "", true
		}
	} else {
		rStr := []rune(s)
		last := len(rStr) - 1
		if c, ok := t.children[rStr[last]]; ok {
			if match, matched := c.contains(string(rStr[:last])); matched {
				return match + string(rStr[last]), true
			}
		}
	}

	if _, wild := t.children['*']; wild {
		return "*", true
	}

	return "", false
}
