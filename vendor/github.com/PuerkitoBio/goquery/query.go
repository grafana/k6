package goquery

import "golang.org/x/net/html"

// Is checks the current matched set of elements against a selector and
// returns true if at least one of these elements matches.
func (s *Selection) Is(selector string) bool {
	return s.IsMatcher(compileMatcher(selector))
}

// IsMatcher checks the current matched set of elements against a matcher and
// returns true if at least one of these elements matches.
func (s *Selection) IsMatcher(m Matcher) bool {
	for _, n := range s.Nodes {
		if m.Match(n) {
			return true
		}
	}
	return false
}

// IsFunction checks the current matched set of elements against a predicate and
// returns true if at least one of these elements matches.
func (s *Selection) IsFunction(f func(int, *Selection) bool) bool {
	for i, n := range s.Nodes {
		if f(i, newSingleSelection(n, s.document)) {
			return true
		}
	}
	return false
}

// IsSelection checks the current matched set of elements against a Selection object
// and returns true if at least one of these elements matches.
func (s *Selection) IsSelection(sel *Selection) bool {
	if sel == nil {
		return false
	}
	return s.IsNodes(sel.Nodes...)
}

// IsNodes checks the current matched set of elements against the specified nodes
// and returns true if at least one of these elements matches.
func (s *Selection) IsNodes(nodes ...*html.Node) bool {
	for _, n := range s.Nodes {
		if isInSlice(nodes, n) {
			return true
		}
	}
	return false
}

// Contains returns true if the specified Node is within,
// at any depth, one of the nodes in the Selection object.
// It is NOT inclusive, to behave like jQuery's implementation, and
// unlike Javascript's .contains, so if the contained
// node is itself in the selection, it returns false.
func (s *Selection) Contains(n *html.Node) bool {
	return sliceContains(s.Nodes, n)
}
