package syntax

import (
	"bytes"
	"errors"
)

type ReplacerData struct {
	Rep     string
	Strings []string
	Rules   []int
}

const (
	replaceSpecials     = 4
	replaceLeftPortion  = -1
	replaceRightPortion = -2
	replaceLastGroup    = -3
	replaceWholeString  = -4
)

// ErrReplacementError is a general error during parsing the replacement text
var ErrReplacementError = errors.New("replacement pattern error")

// NewReplacerData will populate a reusable replacer data struct based on the given replacement string
// and the capture group data from a regexp
func NewReplacerData(rep string, caps map[int]int, capsize int, capnames map[string]int, op RegexOptions) (*ReplacerData, error) {
	return newReplacerData(rep, caps, capsize, capnames, op, nil)
}

// NewReplacerDataWithGroupNames is like NewReplacerData, with group names in
// capture-slot order so duplicate ECMAScript names resolve all their slots.
// Numeric replacement references still select individual groups.
func NewReplacerDataWithGroupNames(rep string, caps map[int]int, capsize int, capnames map[string]int, op RegexOptions, groupNames []string) (*ReplacerData, error) {
	var namedCaptures map[string][]int
	if op&ECMAScript != 0 {
		namedCaptures = make(map[string][]int)
		numbers := make([]int, len(groupNames))
		for i := range numbers {
			numbers[i] = i
		}
		for number, slot := range caps {
			numbers[slot] = number
		}
		for slot, name := range groupNames {
			if name != "" {
				namedCaptures[name] = append(namedCaptures[name], numbers[slot])
			}
		}
	}
	return NewReplacerDataWithGroupNumbers(rep, caps, capsize, capnames, op, namedCaptures)
}

// NewReplacerDataWithGroupNumbers is like NewReplacerData, with ECMAScript
// duplicate names mapped to their public group numbers in declaration order.
// The map and its slices are read only during this call and may be reused.
// Numeric replacement references still select individual groups.
func NewReplacerDataWithGroupNumbers(rep string, caps map[int]int, capsize int, capnames map[string]int, op RegexOptions, groupNumbers map[string][]int) (*ReplacerData, error) {
	return newReplacerData(rep, caps, capsize, capnames, op, groupNumbers)
}

func newReplacerData(rep string, caps map[int]int, capsize int, capnames map[string]int, op RegexOptions, namedCaptures map[string][]int) (*ReplacerData, error) {
	p := parser{
		options:       op,
		caps:          caps,
		capsize:       capsize,
		capnames:      capnames,
		namedCaptures: namedCaptures,
	}
	p.setPattern(rep)
	concat, err := p.scanReplacement()
	if err != nil {
		return nil, err
	}

	if concat.T != NtConcatenate {
		panic(ErrReplacementError)
	}

	sb := &bytes.Buffer{}
	var (
		strings []string
		rules   []int
	)

	for _, child := range concat.Children {
		switch child.T {
		case NtMulti:
			child.writeStrToBuf(sb)

		case NtOne:
			sb.WriteRune(child.Ch)

		case NtRef:
			if sb.Len() > 0 {
				rules = append(rules, len(strings))
				strings = append(strings, sb.String())
				sb.Reset()
			}
			slot := child.M

			if len(caps) > 0 && slot >= 0 {
				slot = caps[slot]
			}

			rules = append(rules, -replaceSpecials-1-slot)

		default:
			panic(ErrReplacementError)
		}
	}

	if sb.Len() > 0 {
		rules = append(rules, len(strings))
		strings = append(strings, sb.String())
	}

	return &ReplacerData{
		Rep:     rep,
		Strings: strings,
		Rules:   rules,
	}, nil
}
