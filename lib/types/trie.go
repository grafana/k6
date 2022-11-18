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
		r := runes[i]
		c, ok := ptr.children[r]

		if !ok {
			ptr.children[r] = &trieNode{children: map[rune]*trieNode{}}
			c = ptr.children[r]
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
		_, wOk := ptr.children['*']

		if wOk {
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
		return reverse(builder.String()), true
	}

	if _, ok := ptr.children['*']; ok {
		builder.WriteRune('*')
		return reverse(builder.String()), true
	}

	return reverse(wMatch), wMatch != ""
}

func reverse(s string) string {
	rs := []rune(s)
	for i, j := 0, len(rs)-1; i < len(rs)/2; i, j = i+1, j-1 {
		rs[i], rs[j] = rs[j], rs[i]
	}

	return string(rs)
}
