package types

import "strings"

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
