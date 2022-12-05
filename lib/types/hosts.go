package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

const nullJSON = "null"

// NullHosts is a wrapper around Hosts like guregu/null
type NullHosts struct {
	Trie  *Hosts
	Valid bool
}

// NewNullHosts returns valid (Valid: true) Hosts
func NewNullHosts(source map[string]Host) (NullHosts, error) {
	hosts, err := NewHosts(source)
	if err != nil {
		return NullHosts{}, err
	}

	return NullHosts{
		Trie:  hosts,
		Valid: true,
	}, nil
}

// MarshalJSON converts NullHosts to valid JSON
func (n NullHosts) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte(nullJSON), nil
	}

	jsonMap := make(map[string]string)
	for k, v := range n.Trie.source {
		var right string
		if v.Port != 0 {
			right = v.String()
		} else {
			right = v.IP.String()
		}
		jsonMap[k] = right
	}

	return json.Marshal(jsonMap)
}

// UnmarshalJSON converts JSON to NullHosts
func (n *NullHosts) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte(nullJSON)) {
		n.Trie = nil
		n.Valid = false
		return nil
	}

	jsonSource := make(map[string]string)
	if err := json.Unmarshal(data, &jsonSource); err != nil {
		return err
	}

	source := make(map[string]Host)
	for k, v := range jsonSource {
		ip, port, err := net.SplitHostPort(v)
		if err == nil {
			pInt, err := strconv.Atoi(port)
			if err != nil {
				return err
			}

			source[k] = Host{IP: net.ParseIP(ip), Port: pInt}
		} else {
			source[k] = Host{IP: net.ParseIP(v)}
		}
	}

	hosts, err := NewHosts(source)
	if err != nil {
		return err
	}
	n.Trie = hosts
	n.Valid = true
	return nil
}

// Hosts is wrapper around trieNode to integrate with net.TCPAddr
type Hosts struct {
	n      *trieNode
	source map[string]Host
}

// NewHosts returns new Hosts from given addresses.
func NewHosts(source map[string]Host) (*Hosts, error) {
	h := &Hosts{
		source: source,
		n: &trieNode{
			children: make(map[rune]*trieNode),
		},
	}

	for k := range h.source {
		err := h.insert(k)
		if err != nil {
			return nil, err
		}
	}

	return h, nil
}

// Regex description of domain(:port)? pattern to enforce blocks by.
// Global var to avoid compilation penalty at runtime.
// Based on regex from https://stackoverflow.com/a/106223/5427244
//
//nolint:lll
var validHostPattern *regexp.Regexp = regexp.MustCompile(`^(\*\.?)?((([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9]))?(:[0-9]{1,5})?$`)

func isValidHostPattern(s string) error {
	if len(validHostPattern.FindString(s)) != len(s) {
		return fmt.Errorf("invalid host pattern '%s'", s)
	}
	return nil
}

func (t *Hosts) insert(s string) error {
	s = strings.ToLower(s) // domains are not case-sensitive

	if err := isValidHostPattern(s); err != nil {
		return err
	}

	t.n.insert(s)

	return nil
}

// Match returns the host matching s, where the value can be one of:
// - nil (no match)
// - IP:0 (Only IP match, record does not have port information)
// - IP:Port
func (t *Hosts) Match(s string) *Host {
	s = strings.ToLower(s)
	match, ok := t.n.contains(s)

	if !ok {
		return nil
	}

	address := t.source[match]

	return &address
}
