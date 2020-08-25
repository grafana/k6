package goja

import (
	"fmt"
	"github.com/dlclark/regexp2"
	"github.com/dop251/goja/unistring"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"
)

type regexp2Wrapper regexp2.Regexp
type regexpWrapper regexp.Regexp

type positionMapItem struct {
	src, dst int
}
type positionMap []positionMapItem

func (m positionMap) get(src int) int {
	if src == 0 {
		return 0
	}
	res := sort.Search(len(m), func(n int) bool { return m[n].src >= src })
	if res >= len(m) || m[res].src != src {
		panic("index not found")
	}
	return m[res].dst
}

type arrayRuneReader struct {
	runes []rune
	pos   int
}

func (rd *arrayRuneReader) ReadRune() (r rune, size int, err error) {
	if rd.pos < len(rd.runes) {
		r = rd.runes[rd.pos]
		size = 1
		rd.pos++
	} else {
		err = io.EOF
	}
	return
}

type regexpPattern struct {
	src string

	global, ignoreCase, multiline, sticky, unicode bool

	regexpWrapper  *regexpWrapper
	regexp2Wrapper *regexp2Wrapper
}

func compileRegexp2(src string, multiline, ignoreCase bool) (*regexp2Wrapper, error) {
	var opts regexp2.RegexOptions = regexp2.ECMAScript
	if multiline {
		opts |= regexp2.Multiline
	}
	if ignoreCase {
		opts |= regexp2.IgnoreCase
	}
	regexp2Pattern, err1 := regexp2.Compile(src, opts)
	if err1 != nil {
		return nil, fmt.Errorf("Invalid regular expression (regexp2): %s (%v)", src, err1)
	}

	return (*regexp2Wrapper)(regexp2Pattern), nil
}

func (p *regexpPattern) createRegexp2() {
	if p.regexp2Wrapper != nil {
		return
	}
	rx, err := compileRegexp2(p.src, p.multiline, p.ignoreCase)
	if err != nil {
		// At this point the regexp should have been successfully converted to re2, if it fails now, it's a bug.
		panic(err)
	}
	p.regexp2Wrapper = rx
}

func buildUTF8PosMap(s valueString) (positionMap, string) {
	pm := make(positionMap, 0, s.length())
	rd := s.reader(0)
	sPos, utf8Pos := 0, 0
	var sb strings.Builder
	for {
		r, size, err := rd.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			// the string contains invalid UTF-16, bailing out
			return nil, ""
		}
		utf8Size, _ := sb.WriteRune(r)
		sPos += size
		utf8Pos += utf8Size
		pm = append(pm, positionMapItem{src: utf8Pos, dst: sPos})
	}
	return pm, sb.String()
}

func (p *regexpPattern) findSubmatchIndex(s valueString, start int) []int {
	if p.regexpWrapper == nil {
		return p.regexp2Wrapper.findSubmatchIndex(s, start, p.unicode)
	}
	if start != 0 {
		// Unfortunately Go's regexp library does not allow starting from an arbitrary position.
		// If we just drop the first _start_ characters of the string the assertions (^, $, \b and \B) will not
		// work correctly.
		p.createRegexp2()
		return p.regexp2Wrapper.findSubmatchIndex(s, start, p.unicode)
	}
	return p.regexpWrapper.findSubmatchIndex(s, p.unicode)
}

func (p *regexpPattern) findAllSubmatchIndex(s valueString, start int, limit int, sticky bool) [][]int {
	if p.regexpWrapper == nil {
		return p.regexp2Wrapper.findAllSubmatchIndex(s, start, limit, sticky, p.unicode)
	}
	if start == 0 {
		if s, ok := s.(asciiString); ok {
			return p.regexpWrapper.findAllSubmatchIndex(s.String(), limit, sticky)
		}
		if limit == 1 {
			result := p.regexpWrapper.findSubmatchIndex(s, p.unicode)
			if result == nil {
				return nil
			}
			return [][]int{result}
		}
		// Unfortunately Go's regexp library lacks FindAllReaderSubmatchIndex(), so we have to use a UTF-8 string as an
		// input.
		if p.unicode {
			// Try to convert s to UTF-8. If it does not contain any invalid UTF-16 we can do the matching in UTF-8.
			pm, str := buildUTF8PosMap(s)
			if pm != nil {
				res := p.regexpWrapper.findAllSubmatchIndex(str, limit, sticky)
				for _, result := range res {
					for i, idx := range result {
						result[i] = pm.get(idx)
					}
				}
				return res
			}
		}
	}

	p.createRegexp2()
	return p.regexp2Wrapper.findAllSubmatchIndex(s, start, limit, sticky, p.unicode)
}

type regexpObject struct {
	baseObject
	pattern *regexpPattern
	source  valueString

	standard bool
}

func (r *regexp2Wrapper) findSubmatchIndex(s valueString, start int, fullUnicode bool) (result []int) {
	if fullUnicode {
		return r.findSubmatchIndexUnicode(s, start)
	}
	return r.findSubmatchIndexUTF16(s, start)
}

func (r *regexp2Wrapper) findSubmatchIndexUTF16(s valueString, start int) (result []int) {
	wrapped := (*regexp2.Regexp)(r)
	match, err := wrapped.FindRunesMatchStartingAt(s.utf16Runes(), start)
	if err != nil {
		return
	}

	if match == nil {
		return
	}
	groups := match.Groups()

	result = make([]int, 0, len(groups)<<1)
	for _, group := range groups {
		if len(group.Captures) > 0 {
			result = append(result, group.Index, group.Index+group.Length)
		} else {
			result = append(result, -1, 0)
		}
	}
	return
}

func (r *regexp2Wrapper) findSubmatchIndexUnicode(s valueString, start int) (result []int) {
	wrapped := (*regexp2.Regexp)(r)
	posMap, runes, mappedStart := buildPosMap(&lenientUtf16Decoder{utf16Reader: s.utf16Reader(0)}, s.length(), start)
	match, err := wrapped.FindRunesMatchStartingAt(runes, mappedStart)
	if err != nil {
		return
	}

	if match == nil {
		return
	}
	groups := match.Groups()

	result = make([]int, 0, len(groups)<<1)
	for _, group := range groups {
		if len(group.Captures) > 0 {
			result = append(result, posMap[group.Index], posMap[group.Index+group.Length])
		} else {
			result = append(result, -1, 0)
		}
	}
	return
}

func (r *regexp2Wrapper) findAllSubmatchIndexUTF16(s valueString, start, limit int, sticky bool) [][]int {
	wrapped := (*regexp2.Regexp)(r)
	runes := s.utf16Runes()
	match, err := wrapped.FindRunesMatchStartingAt(runes, start)
	if err != nil {
		return nil
	}
	if limit < 0 {
		limit = len(runes) + 1
	}
	results := make([][]int, 0, limit)
	for match != nil {
		groups := match.Groups()

		result := make([]int, 0, len(groups)<<1)

		for _, group := range groups {
			if len(group.Captures) > 0 {
				startPos := group.Index
				endPos := group.Index + group.Length
				result = append(result, startPos, endPos)
			} else {
				result = append(result, -1, 0)
			}
		}

		if sticky && len(result) > 1 {
			if result[0] != start {
				break
			}
			start = result[1]
		}

		results = append(results, result)
		limit--
		if limit <= 0 {
			break
		}
		match, err = wrapped.FindNextMatch(match)
		if err != nil {
			return nil
		}
	}
	return results
}

func buildPosMap(rd io.RuneReader, l, start int) (posMap []int, runes []rune, mappedStart int) {
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
				_, second := utf16.EncodeRune(runes[mappedStart])
				runes[mappedStart] = second
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

func (r *regexp2Wrapper) findAllSubmatchIndexUnicode(s unicodeString, start, limit int, sticky bool) [][]int {
	wrapped := (*regexp2.Regexp)(r)
	if limit < 0 {
		limit = len(s) + 1
	}
	results := make([][]int, 0, limit)
	posMap, runes, mappedStart := buildPosMap(&lenientUtf16Decoder{utf16Reader: s.utf16Reader(0)}, s.length(), start)

	match, err := wrapped.FindRunesMatchStartingAt(runes, mappedStart)
	if err != nil {
		return nil
	}
	for match != nil {
		groups := match.Groups()

		result := make([]int, 0, len(groups)<<1)

		for _, group := range groups {
			if len(group.Captures) > 0 {
				start := posMap[group.Index]
				end := posMap[group.Index+group.Length]
				result = append(result, start, end)
			} else {
				result = append(result, -1, 0)
			}
		}

		if sticky && len(result) > 1 {
			if result[0] != start {
				break
			}
			start = result[1]
		}

		results = append(results, result)
		match, err = wrapped.FindNextMatch(match)
		if err != nil {
			return nil
		}
	}
	return results
}

func (r *regexp2Wrapper) findAllSubmatchIndex(s valueString, start, limit int, sticky, fullUnicode bool) [][]int {
	switch s := s.(type) {
	case asciiString:
		return r.findAllSubmatchIndexUTF16(s, start, limit, sticky)
	case unicodeString:
		if fullUnicode {
			return r.findAllSubmatchIndexUnicode(s, start, limit, sticky)
		}
		return r.findAllSubmatchIndexUTF16(s, start, limit, sticky)
	default:
		panic("Unsupported string type")
	}
}

func (r *regexpWrapper) findAllSubmatchIndex(s string, limit int, sticky bool) (results [][]int) {
	wrapped := (*regexp.Regexp)(r)
	results = wrapped.FindAllStringSubmatchIndex(s, limit)
	pos := 0
	if sticky {
		for i, result := range results {
			if len(result) > 1 {
				if result[0] != pos {
					return results[:i]
				}
				pos = result[1]
			}
		}
	}
	return
}

func (r *regexpWrapper) findSubmatchIndex(s valueString, fullUnicode bool) (result []int) {
	wrapped := (*regexp.Regexp)(r)
	if fullUnicode {
		posMap, runes, _ := buildPosMap(&lenientUtf16Decoder{utf16Reader: s.utf16Reader(0)}, s.length(), 0)
		res := wrapped.FindReaderSubmatchIndex(&arrayRuneReader{runes: runes})
		for i, item := range res {
			res[i] = posMap[item]
		}
		return res
	}
	return wrapped.FindReaderSubmatchIndex(s.utf16Reader(0))
}

func (r *regexpObject) execResultToArray(target valueString, result []int) Value {
	captureCount := len(result) >> 1
	valueArray := make([]Value, captureCount)
	matchIndex := result[0]
	lowerBound := matchIndex
	for index := 0; index < captureCount; index++ {
		offset := index << 1
		if result[offset] >= lowerBound {
			valueArray[index] = target.substring(result[offset], result[offset+1])
			lowerBound = result[offset]
		} else {
			valueArray[index] = _undefined
		}
	}
	match := r.val.runtime.newArrayValues(valueArray)
	match.self.setOwnStr("input", target, false)
	match.self.setOwnStr("index", intToValue(int64(matchIndex)), false)
	return match
}

func (r *regexpObject) getLastIndex() int64 {
	lastIndex := toLength(r.getStr("lastIndex", nil))
	if !r.pattern.global && !r.pattern.sticky {
		return 0
	}
	return lastIndex
}

func (r *regexpObject) updateLastIndex(index int64, firstResult, lastResult []int) bool {
	if r.pattern.sticky {
		if firstResult == nil || int64(firstResult[0]) != index {
			r.setOwnStr("lastIndex", intToValue(0), true)
			return false
		}
	} else {
		if firstResult == nil {
			if r.pattern.global {
				r.setOwnStr("lastIndex", intToValue(0), true)
			}
			return false
		}
	}

	if r.pattern.global || r.pattern.sticky {
		r.setOwnStr("lastIndex", intToValue(int64(lastResult[1])), true)
	}
	return true
}

func (r *regexpObject) execRegexp(target valueString) (match bool, result []int) {
	index := r.getLastIndex()
	if index >= 0 && index <= int64(target.length()) {
		result = r.pattern.findSubmatchIndex(target, int(index))
	}
	match = r.updateLastIndex(index, result, result)
	return
}

func (r *regexpObject) exec(target valueString) Value {
	match, result := r.execRegexp(target)
	if match {
		return r.execResultToArray(target, result)
	}
	return _null
}

func (r *regexpObject) test(target valueString) bool {
	match, _ := r.execRegexp(target)
	return match
}

func (r *regexpObject) clone() *Object {
	r1 := r.val.runtime.newRegexpObject(r.prototype)
	r1.source = r.source
	r1.pattern = r.pattern

	return r1.val
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

func (r *regexpObject) deleteStr(name unistring.String, throw bool) bool {
	res := r.baseObject.deleteStr(name, throw)
	if res {
		r.standard = false
	}
	return res
}

func (r *regexpObject) setOwnStr(name unistring.String, value Value, throw bool) bool {
	if r.standard {
		if name == "exec" {
			res := r.baseObject.setOwnStr(name, value, throw)
			if res {
				r.standard = false
			}
			return res
		}
	}
	return r.baseObject.setOwnStr(name, value, throw)
}
