package types

import (
	"bytes"
	"encoding/json"
	"net"
	"strconv"
	"strings"
)

// NullAddressTrie is a wrapper around AddressTrie like guregu/null
type NullAddressTrie struct {
	Trie  *AddressTrie
	Valid bool
}

// NewNullAddressTrie returns valid (Valid: true) AddressTrie
func NewNullAddressTrie(source map[string]HostAddress) NullAddressTrie {
	return NullAddressTrie{
		Trie:  NewAddressTrie(source),
		Valid: true,
	}
}

// MarshalJSON converts NullAddressTrie to valid JSON
func (n NullAddressTrie) MarshalJSON() ([]byte, error) {
	if !n.Valid { // envconfig bug?
		return []byte("null"), nil
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

// UnmarshalJSON converts JSON to NullAddressTrie
func (n *NullAddressTrie) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		n.Trie = nil
		n.Valid = false
		return nil
	}

	jsonSource := make(map[string]string)
	if err := json.Unmarshal(data, &jsonSource); err != nil {
		return err
	}

	source := make(map[string]HostAddress)
	for k, v := range jsonSource {
		ip, port, err := net.SplitHostPort(v)
		if err == nil {
			pInt, err := strconv.Atoi(port)
			if err != nil {
				return err
			}

			source[k] = HostAddress{IP: net.ParseIP(ip), Port: pInt}
		} else {
			source[k] = HostAddress{IP: net.ParseIP(v)}
		}
	}

	n.Trie = NewAddressTrie(source)
	n.Valid = true
	return nil
}

// AddressTrie is wrapper around trieNode to integrate with net.TCPAddr
type AddressTrie struct {
	n      *trieNode
	source map[string]HostAddress
}

// NewAddressTrie returns new Address Trie from given addresses
func NewAddressTrie(source map[string]HostAddress) *AddressTrie {
	at := &AddressTrie{
		source: source,
		n: &trieNode{
			children: make(map[rune]*trieNode),
		},
	}

	for k := range at.source {
		at.insert(k)
	}

	return at
}

func (t *AddressTrie) insert(s string) {
	s = strings.ToLower(s) // domains are not case-sensitive
	t.n.insert(s)
}

// Match returns matched address can return three types of value
// - nil (no match)
// - IP:0 (Only IP match, record does not have port information)
// - IP:Port
func (t *AddressTrie) Match(s string) *HostAddress {
	s = strings.ToLower(s)
	match, ok := t.n.contains(s)

	if !ok {
		return nil
	}

	address := t.source[match]

	return &address
}
