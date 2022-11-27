package types

import "strings"

type trieNode struct {
	isLeaf   bool
	children map[rune]*trieNode
}

func (t *trieNode) insert(s string) {
	runes := []rune(s)

	if t.children == nil {
		t.children = map[rune]*trieNode{}
	}

	ptr := t
	for i := len(runes) - 1; i >= 0; i-- {
		c, ok := ptr.children[runes[i]]

		if !ok {
			ptr.children[runes[i]] = &trieNode{children: map[rune]*trieNode{}}
			c = ptr.children[runes[i]]
		}

		ptr = c
	}

	ptr.isLeaf = true
}

func (t *trieNode) contains(s string) (string, bool) {
	rs := []rune(s)

	builder, wMatch := strings.Builder{}, ""
	found := true

	ptr := t
	for i := len(rs) - 1; i >= 0; i-- {
		child, ok := ptr.children[rs[i]]

		if _, wOk := ptr.children['*']; wOk {
			wMatch = builder.String() + string('*')
		}

		if !ok {
			found = false
			break
		}

		builder.WriteRune(rs[i])
		ptr = child
	}

	if found && ptr.isLeaf {
		return reverseString(builder.String()), true
	}

	if _, ok := ptr.children['*']; ok {
		builder.WriteRune('*')
		return reverseString(builder.String()), true
	}

	return reverseString(wMatch), wMatch != ""
}

func reverseString(s string) string {
	rs := []rune(s)
	for i, j := 0, len(rs)-1; i < len(rs)/2; i, j = i+1, j-1 {
		rs[i], rs[j] = rs[j], rs[i]
	}

	return string(rs)
}
