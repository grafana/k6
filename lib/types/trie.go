package types

import (
	"slices"
	"strings"
)

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
	for _, rune0 := range slices.Backward(runes) {
		c, ok := ptr.children[rune0]

		if !ok {
			ptr.children[rune0] = &trieNode{children: map[rune]*trieNode{}}
			c = ptr.children[rune0]
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
	for _, r := range slices.Backward(rs) {
		child, ok := ptr.children[r]

		if _, wOk := ptr.children['*']; wOk {
			wMatch = builder.String() + string('*')
		}

		if !ok {
			found = false
			break
		}

		builder.WriteRune(r)
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
