package flameql

import (
	"regexp"
	"sort"
	"strings"
)

// ParseQuery parses a string of $app_name<{<$tag_matchers>}> form.
func ParseQuery(s string) (*Query, error) {
	s = strings.TrimSpace(s)
	q := Query{q: s}

	for offset, c := range s {
		switch c {
		case '{':
			if offset == 0 {
				return nil, ErrAppNameIsRequired
			}
			if s[len(s)-1] != '}' {
				return nil, newErr(ErrInvalidQuerySyntax, "expected } at the end")
			}
			m, err := ParseMatchers(s[offset+1 : len(s)-1])
			if err != nil {
				return nil, err
			}
			q.AppName = s[:offset]
			q.Matchers = m
			return &q, nil
		default:
			if !IsAppNameRuneAllowed(c) {
				return nil, newErr(ErrInvalidAppName, s[:offset+1])
			}
		}
	}

	if len(s) == 0 {
		return nil, ErrAppNameIsRequired
	}

	q.AppName = s
	return &q, nil
}

// ParseMatchers parses a string of $tag_matcher<,$tag_matchers> form.
func ParseMatchers(s string) ([]*TagMatcher, error) {
	var matchers []*TagMatcher
	for _, t := range split(s) {
		if t == "" {
			continue
		}
		m, err := ParseMatcher(strings.TrimSpace(t))
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, m)
	}
	if len(matchers) == 0 && len(s) != 0 {
		return nil, newErr(ErrInvalidMatchersSyntax, s)
	}
	sort.Sort(ByPriority(matchers))
	return matchers, nil
}

// ParseMatcher parses a string of $tag_key$op"$tag_value" form,
// where $op is one of the supported match operators.
func ParseMatcher(s string) (*TagMatcher, error) {
	var tm TagMatcher
	var offset int
	var c rune

loop:
	for offset, c = range s {
		r := len(s) - (offset + 1)
		switch c {
		case '=':
			switch {
			case r <= 2:
				return nil, newErr(ErrInvalidTagValueSyntax, s)
			case s[offset+1] == '"':
				tm.Op = OpEqual
			case s[offset+1] == '~':
				if r <= 3 {
					return nil, newErr(ErrInvalidTagValueSyntax, s)
				}
				tm.Op = OpEqualRegex
			default:
				// Just for more meaningful error message.
				if s[offset+2] != '"' {
					return nil, newErr(ErrInvalidTagValueSyntax, s)
				}
				return nil, newErr(ErrUnknownOp, s)
			}
			break loop
		case '!':
			if r <= 3 {
				return nil, newErr(ErrInvalidTagValueSyntax, s)
			}
			switch s[offset+1] {
			case '=':
				tm.Op = OpNotEqual
			case '~':
				tm.Op = OpNotEqualRegex
			default:
				return nil, newErr(ErrUnknownOp, s)
			}
			break loop
		default:
			if !IsTagKeyRuneAllowed(c) {
				return nil, newInvalidTagKeyRuneError(s, c)
			}
		}
	}

	k := s[:offset]
	if IsTagKeyReserved(k) {
		return nil, newErr(ErrTagKeyReserved, k)
	}

	var v string
	var ok bool
	switch tm.Op {
	default:
		return nil, newErr(ErrMatchOperatorIsRequired, s)
	case OpEqual:
		v, ok = unquote(s[offset+1:])
	case OpNotEqual, OpEqualRegex, OpNotEqualRegex:
		v, ok = unquote(s[offset+2:])
	}
	if !ok {
		return nil, newErr(ErrInvalidTagValueSyntax, v)
	}

	// Compile regex, if applicable.
	switch tm.Op {
	case OpEqualRegex, OpNotEqualRegex:
		r, err := regexp.Compile(v)
		if err != nil {
			return nil, newErr(err, v)
		}
		tm.R = r
	}

	tm.Key = k
	tm.Value = v
	return &tm, nil
}

func unquote(s string) (string, bool) {
	if s[0] != '"' || s[len(s)-1] != '"' {
		return s, false
	}
	return s[1 : len(s)-1], true
}

func split(s string) []string {
	var r []string
	var x int
	var y bool
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == ',' && !y:
			r = append(r, s[x:i])
			x = i + 1
		case s[i] == '"':
			if y && i > 0 && s[i-1] != '\\' {
				y = false
				continue
			}
			y = true
		}
	}
	return append(r, s[x:])
}
