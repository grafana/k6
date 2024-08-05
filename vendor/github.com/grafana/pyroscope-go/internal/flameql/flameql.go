package flameql

import "regexp"

type Query struct {
	AppName  string
	Matchers []*TagMatcher

	q string // The original query string.
}

func (q *Query) String() string { return q.q }

type TagMatcher struct {
	Key   string
	Value string
	Op

	R *regexp.Regexp
}

type Op int

const (
	// The order should respect operator priority and cost.
	// Negating operators go first. See IsNegation.
	_               Op = iota
	OpNotEqual         // !=
	OpNotEqualRegex    // !~
	OpEqual            // =
	OpEqualRegex       // =~
)

const (
	ReservedTagKeyName = "__name__"
)

var reservedTagKeys = []string{
	ReservedTagKeyName,
}

// IsNegation reports whether the operator assumes negation.
func (o Op) IsNegation() bool { return o < OpEqual }

// ByPriority is a supplemental type for sorting tag matchers.
type ByPriority []*TagMatcher

func (p ByPriority) Len() int           { return len(p) }
func (p ByPriority) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p ByPriority) Less(i, j int) bool { return p[i].Op < p[j].Op }

func (m *TagMatcher) Match(v string) bool {
	switch m.Op {
	case OpEqual:
		return m.Value == v
	case OpNotEqual:
		return m.Value != v
	case OpEqualRegex:
		return m.R.Match([]byte(v))
	case OpNotEqualRegex:
		return !m.R.Match([]byte(v))
	default:
		panic("invalid match operator")
	}
}

// ValidateTagKey report an error if the given key k violates constraints.
//
// The function should be used to validate user input. The function returns
// ErrTagKeyReserved if the key is valid but reserved for internal use.
func ValidateTagKey(k string) error {
	if len(k) == 0 {
		return ErrTagKeyIsRequired
	}
	for _, r := range k {
		if !IsTagKeyRuneAllowed(r) {
			return newInvalidTagKeyRuneError(k, r)
		}
	}
	if IsTagKeyReserved(k) {
		return newErr(ErrTagKeyReserved, k)
	}
	return nil
}

// ValidateAppName report an error if the given app name n violates constraints.
func ValidateAppName(n string) error {
	if len(n) == 0 {
		return ErrAppNameIsRequired
	}
	for _, r := range n {
		if !IsAppNameRuneAllowed(r) {
			return newInvalidAppNameRuneError(n, r)
		}
	}
	return nil
}

func IsTagKeyRuneAllowed(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

func IsAppNameRuneAllowed(r rune) bool {
	return r == '-' || r == '.' || IsTagKeyRuneAllowed(r)
}

func IsTagKeyReserved(k string) bool {
	for _, s := range reservedTagKeys {
		if s == k {
			return true
		}
	}
	return false
}
