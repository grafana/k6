package sobek

import (
	"fmt"
	"io"
	"sort"
	"unicode/utf16"

	"github.com/dlclark/regexp2/v2"

	"github.com/grafana/sobek/unistring"
)

type regexp2MatchCache struct {
	target String
	runes  []rune
	posMap []int
}

// Not goroutine-safe. Use regexp2Wrapper.clone()
type regexp2Wrapper struct {
	rx    *regexp2.Regexp
	cache *regexp2MatchCache
}

// Not goroutine-safe. Use regexpPattern.clone()
type regexpPattern struct {
	src string

	global, ignoreCase, multiline, dotAll, sticky, unicode bool

	regexp2Wrapper *regexp2Wrapper
}

type regexpResult struct {
	indexes []int
	groups  []string
}

func (result *regexpResult) appendIndexesAndGroup(index1, index2 int, name string) {
	result.indexes = append(result.indexes, index1, index2)
	result.groups = append(result.groups, name)
}

func compileRegexp2(src string, multiline, dotAll, ignoreCase, unicode bool) (*regexp2Wrapper, error) {
	var opts = regexp2.ECMAScript
	if multiline {
		opts |= regexp2.Multiline
	}
	if dotAll {
		opts |= regexp2.Singleline
	}
	if ignoreCase {
		opts |= regexp2.IgnoreCase
	}
	if unicode {
		opts |= regexp2.Unicode
	}
	regexp2Pattern, err1 := regexp2.Compile(src, opts)
	if err1 != nil {
		return nil, fmt.Errorf("Invalid regular expression (regexp2): %s (%v)", src, err1)
	}

	return &regexp2Wrapper{rx: regexp2Pattern}, nil
}

func (p *regexpPattern) findSubmatchIndex(s String, start int) regexpResult {
	return p.regexp2Wrapper.findSubmatchIndex(s, start, p.unicode, true)
}

func (p *regexpPattern) findAllSubmatchIndex(s String, start int, limit int, sticky bool) []regexpResult {
	return p.regexp2Wrapper.findAllSubmatchIndex(s, start, limit, sticky, p.unicode)
}

// clone creates a copy of the regexpPattern which can be used concurrently.
func (p *regexpPattern) clone() *regexpPattern {
	ret := &regexpPattern{
		src:        p.src,
		global:     p.global,
		ignoreCase: p.ignoreCase,
		multiline:  p.multiline,
		dotAll:     p.dotAll,
		sticky:     p.sticky,
		unicode:    p.unicode,
	}
	if p.regexp2Wrapper != nil {
		ret.regexp2Wrapper = p.regexp2Wrapper.clone()
	}
	return ret
}

type regexpObject struct {
	baseObject
	pattern *regexpPattern
	source  String

	standard bool
}

func (r *regexp2Wrapper) findSubmatchIndex(s String, start int, fullUnicode, doCache bool) regexpResult {
	if fullUnicode {
		return r.findSubmatchIndexUnicode(s, start, doCache)
	}
	return r.findSubmatchIndexUTF16(s, start, doCache)
}

func (r *regexp2Wrapper) findSubmatchIndexUTF16(s String, start int, doCache bool) regexpResult {
	var runes []rune
	if cache := r.cache; cache != nil && cache.posMap == nil && cache.target.SameAs(s) {
		runes = cache.runes
	} else {
		runes = s.utf16Runes()
	}
	var result regexpResult
	matched := false
	// Using FindRunesMatches with a one-shot callback instead of
	// FindRunesMatchStartingAt keeps the match state inside regexp2's runner
	// pool, avoiding the per-call Match and match text allocations.
	err := r.rx.FindRunesMatches(runes, start, func(match regexp2.MatchView) bool {
		matched = true
		groupCount := match.GroupCount()
		result = regexpResult{
			indexes: make([]int, 0, groupCount<<1),
			groups:  make([]string, 0, groupCount),
		}
		for i := range groupCount {
			// GroupRange avoids materialising Group objects; note that on a
			// match owned by the runner (see FindRunesMatches) Groups() would
			// not even report group 0 as captured.
			name := r.rx.GroupNameFromNumber(i)
			if startPos, length, ok := match.GroupRange(i); ok {
				result.appendIndexesAndGroup(startPos, startPos+length, name)
			} else {
				result.appendIndexesAndGroup(-1, 0, name)
			}
		}
		return false
	})
	if err != nil {
		panic(rangeError(err.Error()))
	}
	if doCache && matched {
		if r.cache == nil {
			r.cache = new(regexp2MatchCache)
		}
		*r.cache = regexp2MatchCache{
			target: s,
			runes:  runes,
		}
	} else {
		r.cache = nil
	}
	return result
}

func (r *regexp2Wrapper) findSubmatchIndexUnicode(s String, start int, doCache bool) regexpResult {
	var runes []rune
	var posMap []int
	var mappedStart int
	var splitPair bool
	var savedRune rune
	if cache := r.cache; cache != nil && cache.posMap != nil && cache.target.SameAs(s) {
		runes, posMap = cache.runes, cache.posMap
		mappedStart, splitPair = posMapReverseLookup(posMap, start)
	} else {
		posMap, runes, mappedStart, splitPair = buildPosMap(&lenientUtf16Decoder{utf16Reader: s.utf16Reader()}, s.Length(), start)
	}
	if splitPair {
		// temporarily set the rune at mappedStart to the second code point of the pair
		_, second := utf16.EncodeRune(runes[mappedStart])
		savedRune, runes[mappedStart] = runes[mappedStart], second
		defer func() { runes[mappedStart] = savedRune }()
	}
	var result regexpResult
	matched := false
	// Using FindRunesMatches with a one-shot callback instead of
	// FindRunesMatchStartingAt keeps the match state inside regexp2's runner
	// pool, avoiding the per-call Match and match text allocations.
	err := r.rx.FindRunesMatches(runes, mappedStart, func(match regexp2.MatchView) bool {
		matched = true
		groupCount := match.GroupCount()
		result = regexpResult{
			indexes: make([]int, 0, groupCount<<1),
			groups:  make([]string, 0, groupCount),
		}
		for i := range groupCount {
			// GroupRange avoids materialising Group objects; note that on a
			// match owned by the runner (see FindRunesMatches) Groups() would
			// not even report group 0 as captured.
			name := r.rx.GroupNameFromNumber(i)
			if startPos, length, ok := match.GroupRange(i); ok {
				result.appendIndexesAndGroup(posMap[startPos], posMap[startPos+length], name)
			} else {
				result.appendIndexesAndGroup(-1, 0, name)
			}
		}
		return false
	})
	if err != nil {
		panic(rangeError(err.Error()))
	}
	if doCache && matched {
		if r.cache == nil {
			r.cache = new(regexp2MatchCache)
		}
		*r.cache = regexp2MatchCache{
			target: s,
			runes:  runes,
			posMap: posMap,
		}
	} else {
		r.cache = nil
	}
	return result
}

func (r *regexp2Wrapper) findAllSubmatchIndexUTF16(s String, start, limit int, sticky bool) []regexpResult {
	runes := s.utf16Runes()
	if limit < 0 {
		limit = len(runes) + 1
	}
	var results []regexpResult
	var names []string
	var flat []int
	err := r.rx.FindRunesMatches(runes, start, func(match regexp2.MatchView) bool {
		var result regexpResult
		if groupCount := match.GroupCount(); groupCount > 1 {
			if names == nil {
				names = make([]string, groupCount)
				for i := 1; i < groupCount; i++ {
					names[i] = r.rx.GroupNameFromNumber(i)
				}
			}
			base := len(flat)
			for i := range groupCount {
				if startPos, length, ok := match.GroupRange(i); ok {
					flat = append(flat, startPos, startPos+length)
				} else {
					flat = append(flat, -1, 0)
				}
			}
			result.indexes = flat[base:len(flat):len(flat)]
			result.groups = names
		} else {
			// Reading the whole-match interval directly avoids materialising
			// Group objects which the callers would not use anyway.
			base := len(flat)
			startPos, length, _ := match.GroupRange(0)
			flat = append(flat, startPos, startPos+length)
			result.indexes = flat[base:len(flat):len(flat)]
		}

		if sticky && len(result.indexes) > 1 {
			if result.indexes[0] != start {
				return false
			}
			if result.indexes[0] == result.indexes[1] {
				start = advanceStringIndex(s, start, false)
			} else {
				start = result.indexes[1]
			}
		}

		results = append(results, result)
		limit--
		return limit > 0
	})
	if err != nil {
		panic(rangeError(err.Error()))
	}
	return results
}

func buildPosMap(rd io.RuneReader, l, start int) (posMap []int, runes []rune, mappedStart int, splitPair bool) {
	posMap = make([]int, 0, l+1)
	curPos := 0
	runes = make([]rune, 0, l)
	startFound := false
	for {
		if !startFound {
			if curPos == start {
				mappedStart = len(runes)
				startFound = true
			}
			if curPos > start {
				// start position splits a surrogate pair
				mappedStart = len(runes) - 1
				splitPair = true
				startFound = true
			}
		}
		rn, size, err := rd.ReadRune()
		if err != nil {
			break
		}
		runes = append(runes, rn)
		posMap = append(posMap, curPos)
		curPos += size
	}
	posMap = append(posMap, curPos)
	return
}

func posMapReverseLookup(posMap []int, pos int) (int, bool) {
	mapped := sort.SearchInts(posMap, pos)
	if mapped < len(posMap) && posMap[mapped] != pos {
		return mapped - 1, true
	}
	return mapped, false
}

func (r *regexp2Wrapper) findAllSubmatchIndexUnicode(s unicodeString, start, limit int, sticky bool) []regexpResult {
	var posMap []int
	var runes []rune
	var mappedStart int
	var splitPair bool
	var savedRune rune
	if cache := r.cache; cache != nil && cache.posMap != nil && cache.target.SameAs(s) {
		runes, posMap = cache.runes, cache.posMap
		mappedStart, splitPair = posMapReverseLookup(posMap, start)
	} else {
		posMap, runes, mappedStart, splitPair = buildPosMap(&lenientUtf16Decoder{utf16Reader: s.utf16Reader()}, s.Length(), start)
	}
	if splitPair {
		// temporarily set the rune at mappedStart to the second code point of the pair
		_, second := utf16.EncodeRune(runes[mappedStart])
		savedRune, runes[mappedStart] = runes[mappedStart], second
		defer func() { runes[mappedStart] = savedRune }()
	}
	if limit < 0 {
		limit = len(s) + 1
	}
	var results []regexpResult
	var names []string
	var flat []int
	err := r.rx.FindRunesMatches(runes, mappedStart, func(match regexp2.MatchView) bool {
		var result regexpResult
		if groupCount := match.GroupCount(); groupCount > 1 {
			if names == nil {
				names = make([]string, groupCount)
				for i := 1; i < groupCount; i++ {
					names[i] = r.rx.GroupNameFromNumber(i)
				}
			}
			base := len(flat)
			for i := range groupCount {
				if startPos, length, ok := match.GroupRange(i); ok {
					flat = append(flat, posMap[startPos], posMap[startPos+length])
				} else {
					flat = append(flat, -1, 0)
				}
			}
			result.indexes = flat[base:len(flat):len(flat)]
			result.groups = names
		} else {
			base := len(flat)
			startPos, length, _ := match.GroupRange(0)
			flat = append(flat, posMap[startPos], posMap[startPos+length])
			result.indexes = flat[base:len(flat):len(flat)]
		}

		if sticky && len(result.indexes) > 1 {
			if result.indexes[0] != start {
				return false
			}
			if result.indexes[0] == result.indexes[1] {
				start = advanceStringIndex(s, start, false)
			} else {
				start = result.indexes[1]
			}
		}

		results = append(results, result)
		limit--
		return limit > 0
	})
	if err != nil {
		panic(rangeError(err.Error()))
	}
	return results
}

func (r *regexp2Wrapper) findAllSubmatchIndex(s String, start, limit int, sticky, fullUnicode bool) []regexpResult {
	a, u := devirtualizeString(s)
	if u != nil {
		if fullUnicode {
			return r.findAllSubmatchIndexUnicode(u, start, limit, sticky)
		}
		return r.findAllSubmatchIndexUTF16(u, start, limit, sticky)
	}
	return r.findAllSubmatchIndexUTF16(a, start, limit, sticky)
}

func (r *regexp2Wrapper) clone() *regexp2Wrapper {
	return &regexp2Wrapper{
		rx: r.rx,
	}
}

func (r *Runtime) createRegexpGroupsObj(valueArray []Value, groupNames []string) *Object {
	var groups *baseObject
	for index := 1; index < len(valueArray); index++ {
		if index < len(groupNames) {
			name := groupNames[index]
			if name == "" {
				continue
			} else if groups == nil {
				groups = r.newBaseObject(nil, classObject)
			}
			groups.setOwnStr(unistring.NewFromString(name), valueArray[index], false)
		}
	}
	if groups != nil {
		return groups.val
	}
	return nil
}

func createRegexpGroupsMap(result regexpResult) (groups map[unistring.String]int) {
	if len(result.groups) > 0 {
		groups = make(map[unistring.String]int)
		for i := 1; i < len(result.groups); i++ {
			if group := result.groups[i]; group != "" {
				idx := i * 2
				if idx < len(result.indexes)-1 {
					groups[unistring.NewFromString(group)] = idx
				}
			}
		}
	}
	return
}

func (r *regexpObject) execResultToArray(target String, result regexpResult) Value {
	captureCount := len(result.indexes) >> 1
	valueArray := make([]Value, captureCount)
	matchIndex := result.indexes[0]
	valueArray[0] = target.Substring(result.indexes[0], result.indexes[1])
	lowerBound := 0
	for index := 1; index < captureCount; index++ {
		offset := index << 1
		if result.indexes[offset] >= 0 && result.indexes[offset+1] >= lowerBound {
			valueArray[index] = target.Substring(result.indexes[offset], result.indexes[offset+1])
			lowerBound = result.indexes[offset]
		} else {
			valueArray[index] = _undefined
		}
	}
	match := r.val.runtime.newArrayValues(valueArray)
	match.self.setOwnStr("input", target, false)
	match.self.setOwnStr("index", intToValue(int64(matchIndex)), false)

	groups := r.val.runtime.createRegexpGroupsObj(valueArray, result.groups)

	var groupsVal Value
	if groups != nil {
		groupsVal = groups
	} else {
		groupsVal = _undefined
	}
	match.self.defineOwnPropertyStr("groups", PropertyDescriptor{
		Value:        groupsVal,
		Writable:     FLAG_TRUE,
		Configurable: FLAG_TRUE,
		Enumerable:   FLAG_TRUE,
	}, false)

	return match
}

func (r *regexpObject) getLastIndex() int64 {
	lastIndex := toLength(r.getStr("lastIndex", nil))
	if !r.pattern.global && !r.pattern.sticky {
		return 0
	}
	return lastIndex
}

func (r *regexpObject) execRegexp(target String) (match bool, result regexpResult) {
	index := r.getLastIndex()
	if index >= 0 && index <= int64(target.Length()) {
		result = r.pattern.findSubmatchIndex(target, int(index))
	}
	match = len(result.indexes) > 0 && (!r.pattern.sticky || int64(result.indexes[0]) == index)

	if r.pattern.global || r.pattern.sticky {
		var newLastIndex int64
		if match {
			newLastIndex = int64(result.indexes[1])
		}
		r.setOwnStr("lastIndex", intToValue(newLastIndex), true)
	}

	return
}

func (r *regexpObject) exec(target String) Value {
	match, result := r.execRegexp(target)
	if match {
		return r.execResultToArray(target, result)
	}
	return _null
}

func (r *regexpObject) test(target String) bool {
	match, _ := r.execRegexp(target)
	return match
}

func (r *regexpObject) clone() *regexpObject {
	r1 := r.val.runtime.newRegexpObject(r.prototype)
	r1.source = r.source
	r1.pattern = r.pattern

	return r1
}

func (r *regexpObject) init() {
	r.baseObject.init()
	r.standard = true
	r._putProp("lastIndex", intToValue(0), true, false, false)
}

func (r *regexpObject) setProto(proto *Object, throw bool) bool {
	res := r.baseObject.setProto(proto, throw)
	if res {
		r.standard = false
	}
	return res
}

func (r *regexpObject) defineOwnPropertyStr(name unistring.String, desc PropertyDescriptor, throw bool) bool {
	res := r.baseObject.defineOwnPropertyStr(name, desc, throw)
	if res {
		r.standard = false
	}
	return res
}

func (r *regexpObject) defineOwnPropertySym(name *Symbol, desc PropertyDescriptor, throw bool) bool {
	res := r.baseObject.defineOwnPropertySym(name, desc, throw)
	if res && r.standard {
		switch name {
		case SymMatch, SymMatchAll, SymSearch, SymSplit, SymReplace:
			r.standard = false
		}
	}
	return res
}

func (r *regexpObject) deleteStr(name unistring.String, throw bool) bool {
	res := r.baseObject.deleteStr(name, throw)
	if res {
		r.standard = false
	}
	return res
}

func (r *regexpObject) setOwnStr(name unistring.String, value Value, throw bool) bool {
	res := r.baseObject.setOwnStr(name, value, throw)
	if res && r.standard && name == "exec" {
		r.standard = false
	}
	return res
}

func (r *regexpObject) setOwnSym(name *Symbol, value Value, throw bool) bool {
	res := r.baseObject.setOwnSym(name, value, throw)
	if res && r.standard {
		switch name {
		case SymMatch, SymMatchAll, SymSearch, SymSplit, SymReplace:
			r.standard = false
		}
	}
	return res
}
