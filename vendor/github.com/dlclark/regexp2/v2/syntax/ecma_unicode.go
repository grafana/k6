package syntax

import (
	"slices"
	"strings"
	"sync"
	"unicode"
)

// Inline at most 256 bytes of ranges per property escape. Larger properties
// share immutable ranges, avoiding copies in every syntax tree and runner.
const ecmaPropertyRangeLimit = 32

// The prefix cannot occur in a parsed property name. Encoding the complement
// and folding in the key also lets generated runners restore the same set.
const ecmaPropertyPrefix = "\x00ecma:"

var ecmaPropertyCache sync.Map // map[string]func() *CharSet

func ecmaPropertySet(key string) *CharSet {
	if cached, ok := ecmaPropertyCache.Load(key); ok {
		return cached.(func() *CharSet)()
	}
	build := sync.OnceValue(func() *CharSet {
		flags := key[len(ecmaPropertyPrefix):]
		ranges := ecmaRangesForProperty(flags[2:])
		set := &CharSet{}
		if flags[0] == 'P' {
			set.ranges = appendECMAComplement(nil, ranges)
		} else {
			// Folding and canonicalization must not mutate generated data.
			set.ranges = slices.Clone(ranges)
		}
		// Complement before folding: /\P{Lowercase_Letter}/iu includes
		// both "a" and "A". Use the same folding as inline character sets.
		// https://tc39.es/ecma262/#sec-runtime-semantics-compiletocharset
		if flags[1] == 'i' {
			set.addCaseEquivalences(ECMAScript | Unicode | IgnoreCase)
		} else {
			set.canonicalize()
		}
		// Discard spare capacity left by merging/folding before sharing.
		set.ranges = append([]SingleRange(nil), set.ranges...)
		return set
	})
	actual, _ := ecmaPropertyCache.LoadOrStore(key, build)
	return actual.(func() *CharSet)()
}

func appendECMAComplement(dst, ranges []SingleRange) []SingleRange {
	next := rune(0)
	for _, r := range ranges {
		if next < r.First {
			dst = append(dst, SingleRange{next, r.First - 1})
		}
		next = r.Last + 1
	}
	if next <= unicode.MaxRune {
		dst = append(dst, SingleRange{next, unicode.MaxRune})
	}
	return dst
}

// ECMAScript accepts exact Unicode aliases, only General_Category values or
// supported binary properties alone, and only gc/sc/scx with an explicit value.
// Keep this resolver separate from the permissive aliases used by other modes.
//
// ECMA-262, UnicodePropertyValueExpression early errors, UnicodeMatchProperty,
// and UnicodeMatchPropertyValue (including the prohibition on loose matching):
// https://tc39.es/ecma262/#sec-patterns-static-semantics-early-errors
// https://tc39.es/ecma262/#sec-runtime-semantics-unicodematchproperty-p
// https://tc39.es/ecma262/#sec-runtime-semantics-unicodematchpropertyvalue-p-v
func canonicalECMAProperty(name string) (string, bool) {
	if property, value, hasValue := strings.Cut(name, "="); hasValue {
		switch property {
		case "General_Category", "gc":
			category, ok := ecmaCategoryAliases[value]
			return "gc=" + category, ok
		case "Script", "sc", "Script_Extensions", "scx":
			script, ok := ecmaScriptAliases[value]
			// Alias spelling is fixed, but available scripts follow the
			// consumer's Go Unicode version. These two special values have
			// no table in unicode.Scripts.
			ok = ok && (unicode.Scripts[script] != nil || script == "Unknown" || script == "Katakana_Or_Hiragana")
			key := "sc=" + script
			if property == "Script_Extensions" || property == "scx" {
				key = "scx=" + script
			}
			return key, ok
		}
		return "", false
	}
	if category, ok := ecmaCategoryAliases[name]; ok {
		return "gc=" + category, true
	}
	property, ok := ecmaBinaryAliases[name]
	return property, ok
}

func (p *parser) addProperty(cc *CharSet, property string, negate, ignoreCase bool) {
	if !p.useOptionE() || !p.useOptionU() {
		cc.addCategory(property, negate, ignoreCase)
		return
	}

	// Assigned is the complement of the current Go toolchain's Unassigned
	// category, rather than a separately embedded Unicode-version snapshot.
	if property == "Assigned" {
		property, negate = "gc=Cn", !negate
	}
	flags := [2]byte{'p', '-'}
	if negate {
		flags[0] = 'P'
	}
	if ignoreCase {
		flags[1] = 'i'
	}
	key := ecmaPropertyPrefix + string(flags[:]) + property
	set := ecmaPropertySet(key)
	if len(set.ranges) > ecmaPropertyRangeLimit {
		cc.addCategories(Category{Cat: key})
		return
	}
	if set.negate {
		cc.ranges = appendECMAComplement(cc.ranges, set.ranges)
		cc.canonicalize()
	} else {
		cc.addRanges(set.ranges)
	}
}

func ecmaRangesForProperty(property string) []SingleRange {
	if category, ok := strings.CutPrefix(property, "gc="); ok {
		return ecmaRangesFromTable(unicode.Categories[category])
	}
	if script, ok := strings.CutPrefix(property, "sc="); ok {
		return ecmaRangesForScript(script)
	}
	if script, ok := strings.CutPrefix(property, "scx="); ok {
		// Remove all explicit overrides from Script, then add the overrides
		// that include this script. In particular, Common and Inherited
		// must lose characters explicitly assigned to other script sets.
		// https://www.unicode.org/reports/tr24/#Script_Extensions_Def
		base := ecmaRangesForScript(script)
		var result []SingleRange
		j := 0
		for _, r := range base {
			next := r.First
			for j < len(ecmaScriptExtensionOverrides) && ecmaScriptExtensionOverrides[j].Last < next {
				j++
			}
			for k := j; k < len(ecmaScriptExtensionOverrides) && ecmaScriptExtensionOverrides[k].First <= r.Last; k++ {
				override := ecmaScriptExtensionOverrides[k]
				if next < override.First {
					result = append(result, SingleRange{next, override.First - 1})
				}
				if next <= override.Last {
					next = override.Last + 1
				}
			}
			if next <= r.Last {
				result = append(result, SingleRange{next, r.Last})
			}
		}
		set := CharSet{ranges: append(result, ecmaScriptExtensions[script]...)}
		set.canonicalize()
		return set.ranges
	}
	if table := unicode.Properties[property]; table != nil {
		return ecmaRangesFromTable(table)
	}
	if table := unicodeAliasCategories[property]; table != nil {
		return ecmaRangesFromTable(table)
	}
	return ecmaPropertyRanges[property]
}

func ecmaRangesForScript(script string) []SingleRange {
	if script == "Katakana_Or_Hiragana" {
		return nil // A valid Unicode alias with no characters assigned to it.
	}
	if script == "Unknown" {
		// UAX #24 assigns Unknown to unassigned, private-use, and surrogate
		// code points. Derive it from Go so new assignments follow the toolchain.
		// https://www.unicode.org/reports/tr24/#Common_Inherited
		set := CharSet{}
		for _, table := range []*unicode.RangeTable{unicode.Cn, unicode.Co, unicode.Cs} {
			set.ranges = append(set.ranges, ecmaRangesFromTable(table)...)
		}
		set.canonicalize()
		return set.ranges
	}
	return ecmaRangesFromTable(unicode.Scripts[script])
}

// Expand strided Go tables lazily into the shared property cache; their full
// contents aren't embedded again in the package's generated Unicode data.
func ecmaRangesFromTable(table *unicode.RangeTable) []SingleRange {
	ranges := make([]SingleRange, 0, len(table.R16)+len(table.R32))
	add := func(lo, hi, stride rune) {
		if stride == 1 {
			ranges = append(ranges, SingleRange{lo, hi})
			return
		}
		for r := lo; r <= hi; r += stride {
			ranges = append(ranges, SingleRange{r, r})
		}
	}
	for _, r := range table.R16 {
		add(rune(r.Lo), rune(r.Hi), rune(r.Stride))
	}
	for _, r := range table.R32 {
		add(rune(r.Lo), rune(r.Hi), rune(r.Stride))
	}
	return ranges
}
