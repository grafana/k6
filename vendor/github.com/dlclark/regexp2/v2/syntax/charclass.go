package syntax

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CharSet combines start-end rune ranges and unicode categories representing a set of characters
type CharSet struct {
	ranges     []SingleRange
	categories []Category
	sub        *CharSet //optional subtractor
	negate     bool
	anything   bool

	ascii *asciiBitmap
}

type asciiBitmap struct {
	bits [2]uint64
}

type Category struct {
	Negate bool
	Cat    string
}

type SingleRange struct {
	First rune
	Last  rune
}

const (
	SpaceCategoryText = " "
	WordCategoryText  = "W"
)

var (
	ecmaSpace = []rune{0x0009, 0x000e, 0x0020, 0x0021, 0x00a0, 0x00a1, 0x1680, 0x1681, 0x2000, 0x200b, 0x2028, 0x202a, 0x202f, 0x2030, 0x205f, 0x2060, 0x3000, 0x3001, 0xfeff, 0xff00}
	ecmaWord  = []rune{0x0030, 0x003a, 0x0041, 0x005b, 0x005f, 0x0060, 0x0061, 0x007b}
	ecmaDigit = []rune{0x0030, 0x003a}

	re2Space = []rune{0x0009, 0x000b, 0x000c, 0x000e, 0x0020, 0x0021}
)

var (
	AnyClass          = getCharSetFromOldString([]rune{0}, false)
	ECMAAnyClass      = getCharSetFromOldString([]rune{0, 0x000a, 0x000b, 0x000d, 0x000e}, false)
	NoneClass         = getCharSetFromOldString(nil, false)
	ECMAWordClass     = getCharSetFromOldString(ecmaWord, false)
	NotECMAWordClass  = getCharSetFromOldString(ecmaWord, true)
	ECMASpaceClass    = getCharSetFromOldString(ecmaSpace, false)
	NotECMASpaceClass = getCharSetFromOldString(ecmaSpace, true)
	ECMADigitClass    = getCharSetFromOldString(ecmaDigit, false)
	NotECMADigitClass = getCharSetFromOldString(ecmaDigit, true)

	WordClass     = getCharSetFromCategoryString(false, false, WordCategoryText)
	NotWordClass  = getCharSetFromCategoryString(true, false, WordCategoryText)
	SpaceClass    = getCharSetFromCategoryString(false, false, SpaceCategoryText)
	NotSpaceClass = getCharSetFromCategoryString(true, false, SpaceCategoryText)
	DigitClass    = getCharSetFromCategoryString(false, false, "Nd")
	NotDigitClass = getCharSetFromCategoryString(false, true, "Nd")

	RE2SpaceClass    = getCharSetFromOldString(re2Space, false)
	NotRE2SpaceClass = getCharSetFromOldString(re2Space, true)

	NotNewLineClass = getCharSetFromOldString([]rune{0x0a, 0x0b}, true)
)

var unicodeCategories = func() map[string]*unicode.RangeTable {
	retVal := make(map[string]*unicode.RangeTable)
	for k, v := range unicode.Scripts {
		retVal[k] = v
	}
	for k, v := range unicode.Categories {
		retVal[k] = v
	}
	// aliases are just pointers to the original keys
	for k, v := range unicode.CategoryAliases {
		retVal[k] = unicode.Categories[v]
	}
	for k, v := range unicode.Properties {
		retVal[k] = v
	}
	for k, v := range unicodeAliasCategories {
		retVal[k] = v
	}
	return retVal
}()

func getCharSetFromCategoryString(negateSet bool, negateCat bool, cats ...string) func() *CharSet {
	if negateCat && negateSet {
		panic("BUG!  You should only negate the set OR the category in a constant setup, but not both")
	}

	c := CharSet{negate: negateSet}

	c.categories = make([]Category, len(cats))
	for i, cat := range cats {
		c.categories[i] = Category{Cat: cat, Negate: negateCat}
	}
	return func() *CharSet {
		//make a copy each time
		local := c
		//return that address
		return &local
	}
}

func getCharSetFromOldString(setText []rune, negate bool) func() *CharSet {
	c := CharSet{}
	if len(setText) > 0 {
		fillFirst := false
		l := len(setText)
		if negate {
			if setText[0] == 0 {
				setText = setText[1:]
			} else {
				l++
				fillFirst = true
			}
		}

		if l%2 == 0 {
			c.ranges = make([]SingleRange, l/2)
		} else {
			c.ranges = make([]SingleRange, l/2+1)
		}

		first := true
		if fillFirst {
			c.ranges[0] = SingleRange{First: 0}
			first = false
		}

		i := 0
		for _, r := range setText {
			if first {
				// lower bound in a new range
				c.ranges[i] = SingleRange{First: r}
				first = false
			} else {
				c.ranges[i].Last = r - 1
				i++
				first = true
			}
		}
		if !first {
			c.ranges[i].Last = utf8.MaxRune
		}
		if len(c.ranges) == 1 && c.ranges[0].First == 0 && c.ranges[0].Last >= unicode.MaxRune {
			// this is anything...or nothing
			c.anything = !negate
		}
	}

	return func() *CharSet {
		local := c
		return &local
	}
}

// Copy makes a deep copy to prevent accidental mutation of a set
func (c CharSet) Copy() CharSet {
	ret := CharSet{
		anything: c.anything,
		negate:   c.negate,
	}

	ret.ranges = append(ret.ranges, c.ranges...)
	ret.categories = append(ret.categories, c.categories...)

	if c.sub != nil {
		sub := c.sub.Copy()
		ret.sub = &sub
	}

	return ret
}

// gets a human-readable description for a set string
func (c CharSet) String() string {
	buf := &bytes.Buffer{}
	buf.WriteRune('[')

	if c.IsNegated() {
		buf.WriteRune('^')
	}

	for _, r := range c.ranges {

		buf.WriteString(CharDescription(r.First))
		if r.First != r.Last {
			if r.Last-r.First != 1 {
				//groups that are 1 char apart skip the dash
				buf.WriteRune('-')
			}
			buf.WriteString(CharDescription(r.Last))
		}
	}

	for _, c := range c.categories {
		buf.WriteString(c.String())
	}

	if c.sub != nil {
		buf.WriteRune('-')
		buf.WriteString(c.sub.String())
	}

	buf.WriteRune(']')

	return buf.String()
}

func b2i(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// mapHashFill converts a charset into a buffer for use in maps
func (c CharSet) mapHashFill(buf *bytes.Buffer) {
	buf.WriteByte(b2i(c.negate) + b2i(c.anything)*2)

	_ = binary.Write(buf, binary.LittleEndian, int32(len(c.ranges)))
	_ = binary.Write(buf, binary.LittleEndian, int32(len(c.categories)))
	for _, r := range c.ranges {
		buf.WriteRune(r.First)
		buf.WriteRune(r.Last)
	}
	for _, ct := range c.categories {
		// write the length of the cat and indicate if it's negated
		if ct.Negate {
			_ = binary.Write(buf, binary.LittleEndian, int8(-1*len(ct.Cat)))
		} else {
			_ = binary.Write(buf, binary.LittleEndian, int8(len(ct.Cat)))
		}
		buf.WriteString(ct.Cat)
	}

	if c.sub != nil {
		c.sub.mapHashFill(buf)
	}
}

func NewCharSetRuntime(buf string) CharSet {
	retVal := CharSet{}
	b := bytes.NewBufferString(buf)
	val, _ := b.ReadByte()
	//1s bit == negate, 2s bit == anything
	retVal.negate = (val&0x1 == 0x1)
	retVal.anything = (val&0x2 == 0x2)
	var lenRanges, lenCats int32
	_ = binary.Read(b, binary.LittleEndian, &lenRanges)
	_ = binary.Read(b, binary.LittleEndian, &lenCats)

	retVal.ranges = make([]SingleRange, lenRanges)
	for i := 0; i < int(lenRanges); i++ {
		r := SingleRange{}
		r.First, _, _ = b.ReadRune()
		r.Last, _, _ = b.ReadRune()
		retVal.ranges[i] = r
	}

	retVal.categories = make([]Category, lenCats)
	for i := 0; i < int(lenCats); i++ {
		var lenCat int8
		c := Category{}
		_ = binary.Read(b, binary.LittleEndian, &lenCat)
		if lenCat < 0 {
			c.Negate = true
			lenCat *= -1
		}
		c.Cat = string(b.Next(int(lenCat)))
		retVal.categories[i] = c
	}

	//sub
	if b.Len() > 0 {
		sub := NewCharSetRuntime(b.String())
		retVal.sub = &sub
	}

	return retVal
}

// CharIn returns true if the rune is in our character set (either ranges or categories).
// It handles negations and subtracted sub-charsets.
func (c CharSet) CharIn(ch rune) bool {
	if ch >= 0 && ch < 128 && c.ascii != nil {
		return (c.ascii.bits[ch/64] & (1 << (uint(ch) % 64))) != 0
	}
	return c.charInSlow(ch)
}

func (c CharSet) charInSlow(ch rune) bool {
	val := false
	// in s && !s.subtracted

	//check ranges -- binary search for sets with many ranges, linear for small sets
	n := len(c.ranges)
	if n > 0 {
		if n <= 4 {
			for _, r := range c.ranges {
				if ch < r.First {
					break
				}
				if ch <= r.Last {
					val = true
					break
				}
			}
		} else {
			lo, hi := 0, n
			for lo < hi {
				mid := int(uint(lo+hi) >> 1)
				if c.ranges[mid].First <= ch {
					lo = mid + 1
				} else {
					hi = mid
				}
			}
			if lo > 0 && ch <= c.ranges[lo-1].Last {
				val = true
			}
		}
	}

	//check categories if we haven't already found a range
	if !val && len(c.categories) > 0 {
		val = c.charInCategories(ch)
	}

	// negate the whole char set
	if c.negate {
		val = !val
	}

	// get subtracted recurse
	if val && c.sub != nil {
		val = !c.sub.CharIn(ch)
	}

	//log.Printf("Char '%v' in %v == %v", string(ch), c.String(), val)
	return val
}

func (c *CharSet) prepareASCIIBitmap() {
	if c == nil || c.ascii != nil {
		return
	}
	if c.sub != nil {
		c.sub.prepareASCIIBitmap()
	}
	bm := &asciiBitmap{}
	for i := range rune(128) {
		if c.charInSlow(i) {
			bm.bits[i/64] |= 1 << (uint(i) % 64)
		}
	}
	c.ascii = bm
}

func (c *CharSet) charInCategories(ch rune) bool {
	for _, ct := range c.categories {
		// special categories...then unicode
		if ct.Cat == SpaceCategoryText {
			if unicode.IsSpace(ch) {
				// we found a space so we're done
				// negate means this is a "bad" thing
				return !ct.Negate
			} else if ct.Negate {
				return true
			}
		} else if ct.Cat == WordCategoryText {
			if IsWordChar(ch) {
				return !ct.Negate
			} else if ct.Negate {
				return true
			}
		} else if unicode.Is(unicodeCategories[ct.Cat], ch) {
			// if we're in this unicode category then we're done
			// if negate=true on this category then we "failed" our test
			// otherwise we're good that we found it
			return !ct.Negate
		} else if ct.Negate {
			return true
		}
	}
	return false
}

func (c Category) String() string {
	switch c.Cat {
	case SpaceCategoryText:
		if c.Negate {
			return "\\S"
		}
		return "\\s"
	case WordCategoryText:
		if c.Negate {
			return "\\W"
		}
		return "\\w"
	}
	if _, ok := unicodeCategories[c.Cat]; ok {

		if c.Negate {
			return "\\P{" + c.Cat + "}"
		}
		return "\\p{" + c.Cat + "}"
	}
	return "Unknown category: " + c.Cat
}

// CharDescription Produces a human-readable description for a single character.
func CharDescription(ch rune) string {
	/*if ch == '\\' {
		return "\\\\"
	}

	if ch > ' ' && ch <= '~' {
		return string(ch)
	} else if ch == '\n' {
		return "\\n"
	} else if ch == ' ' {
		return "\\ "
	}*/

	b := &bytes.Buffer{}
	escape(b, ch, false) //fmt.Sprintf("%U", ch)
	return b.String()
}

// According to UTS#18 Unicode Regular Expressions (http://www.unicode.org/reports/tr18/)
// RL 1.4 Simple Word Boundaries  The class of <word_character> includes all Alphabetic
// values from the Unicode character database, from UnicodeData.txt [UData], plus the U+200C
// ZERO WIDTH NON-JOINER and U+200D ZERO WIDTH JOINER.
func IsWordChar(r rune) bool {
	//"L", "Mn", "Nd", "Pc"
	return unicode.In(r,
		unicode.Categories["L"], unicode.Categories["Mn"],
		unicode.Categories["Nd"], unicode.Categories["Pc"]) || r == '\u200D' || r == '\u200C'
	//return 'A' <= r && r <= 'Z' || 'a' <= r && r <= 'z' || '0' <= r && r <= '9' || r == '_'
}

func IsECMAWordChar(r rune) bool {
	return unicode.In(r,
		unicode.Categories["L"], unicode.Categories["Mn"],
		unicode.Categories["Nd"], unicode.Categories["Pc"])

	//return 'A' <= r && r <= 'Z' || 'a' <= r && r <= 'z' || '0' <= r && r <= '9' || r == '_'
}

func IsECMAIdentifierStartChar(r rune) bool {
	return r == '$' || r == '_' || unicode.In(r, unicode.L, unicode.Nl, unicode.Other_ID_Start)
}

func IsECMAIdentifierChar(r rune) bool {
	return IsECMAIdentifierStartChar(r) || r == '\u200C' || r == '\u200D' ||
		unicode.In(r, unicode.Mn, unicode.Mc, unicode.Nd, unicode.Pc, unicode.Other_ID_Continue)
}

// SingletonChar will return the char from the first range without validation.
// It assumes you have checked for IsSingleton or IsSingletonInverse and will panic given bad input
func (c CharSet) SingletonChar() rune {
	return c.ranges[0].First
}

func (c CharSet) IsSingleton() bool {
	return !c.negate && //negated is multiple chars
		len(c.categories) == 0 && len(c.ranges) == 1 && // multiple ranges and unicode classes represent multiple chars
		c.sub == nil && // subtraction means we've got multiple chars
		c.ranges[0].First == c.ranges[0].Last // first and last equal means we're just 1 char
}

func (c CharSet) IsSingletonInverse() bool {
	return c.negate && //same as above, but requires negated
		len(c.categories) == 0 && len(c.ranges) == 1 && // multiple ranges and unicode classes represent multiple chars
		c.sub == nil && // subtraction means we've got multiple chars
		c.ranges[0].First == c.ranges[0].Last // first and last equal means we're just 1 char
}

func (c CharSet) IsMergeable() bool {
	return !c.IsNegated() && !c.HasSubtraction()
}

func (c CharSet) IsNegated() bool {
	return c.negate
}

func (c CharSet) HasSubtraction() bool {
	return c.sub != nil
}

func (c CharSet) IsEmpty() bool {
	return len(c.ranges) == 0 && len(c.categories) == 0 && c.sub == nil
}

func (c CharSet) IsAnything() bool {
	return c.anything
}

func (c *CharSet) addDigit(ecma, negate bool) {
	if ecma {
		if negate {
			c.addRanges(NotECMADigitClass().ranges)
		} else {
			c.addRanges(ECMADigitClass().ranges)
		}
	} else {
		c.addCategories(Category{Cat: "Nd", Negate: negate})
	}
}

func (c *CharSet) addChar(ch rune) {
	c.addRange(ch, ch)
}

func (c *CharSet) addSpace(ecma, re2, negate bool) {
	if ecma {
		if negate {
			c.addRanges(NotECMASpaceClass().ranges)
		} else {
			c.addRanges(ECMASpaceClass().ranges)
		}
	} else if re2 {
		if negate {
			c.addRanges(NotRE2SpaceClass().ranges)
		} else {
			c.addRanges(RE2SpaceClass().ranges)
		}
	} else {
		c.addCategories(Category{Cat: SpaceCategoryText, Negate: negate})
	}
}

func (c *CharSet) addWord(ecma, negate bool) {
	if ecma {
		if negate {
			c.addRanges(NotECMAWordClass().ranges)
		} else {
			c.addRanges(ECMAWordClass().ranges)
		}
	} else {
		c.addCategories(Category{Cat: WordCategoryText, Negate: negate})
	}
}

// Add set ranges and categories into ours -- no deduping or anything
func (c *CharSet) addSet(set CharSet) {
	if c.anything {
		return
	}
	if set.anything {
		c.makeAnything()
		return
	}
	// just append here to prevent double-canon
	c.ranges = append(c.ranges, set.ranges...)
	c.addCategories(set.categories...)
	c.canonicalize()
}

func (c *CharSet) makeAnything() {
	c.anything = true
	c.categories = []Category{}
	c.ranges = []SingleRange{{First: 0, Last: unicode.MaxRune}}
}

func (c *CharSet) addCategories(cats ...Category) {
	// don't add dupes and remove positive+negative
	if c.anything {
		// if we've had a previous positive+negative group then
		// just return, we're as broad as we can get
		return
	}

	for _, ct := range cats {
		found := false
		for _, ct2 := range c.categories {
			if ct.Cat == ct2.Cat {
				if ct.Negate != ct2.Negate {
					// oposite negations...this mean we just
					// take us as anything and move on
					c.makeAnything()
					return
				}
				found = true
				break
			}
		}

		if !found {
			c.categories = append(c.categories, ct)
		}
	}
}

// Merges new ranges to our own
func (c *CharSet) addRanges(ranges []SingleRange) {
	if c.anything {
		return
	}
	c.ranges = append(c.ranges, ranges...)
	c.canonicalize()
}

// Merges everything but the new ranges into our own
func (c *CharSet) addNegativeRanges(ranges []SingleRange) {
	if c.anything {
		return
	}

	var hi rune

	// convert incoming ranges into opposites, assume they are in order
	for _, r := range ranges {
		if hi < r.First {
			c.ranges = append(c.ranges, SingleRange{hi, r.First - 1})
		}
		hi = r.Last + 1
	}

	if hi < utf8.MaxRune {
		c.ranges = append(c.ranges, SingleRange{hi, utf8.MaxRune})
	}

	c.canonicalize()
}

func normalizeUnicodeCategoryAlias(catName string) string {
	var b strings.Builder
	b.Grow(len(catName))
	for _, ch := range catName {
		switch ch {
		case '_', '-', ' ':
			continue
		default:
			b.WriteRune(unicode.ToLower(ch))
		}
	}
	return b.String()
}

func canonicalUnicodeCatName(catName string) (string, bool) {
	if _, ok := unicodeCategories[catName]; ok {
		return catName, true
	}

	normalized := normalizeUnicodeCategoryAlias(catName)
	if canonical, ok := unicodeSupportedPropertyAliases[normalized]; ok {
		return canonical, true
	}
	if canonical, ok := unicodeBarePropertyValueAliases[normalized]; ok {
		return canonical, true
	}

	if eq := strings.IndexRune(catName, '='); eq >= 0 {
		propName := catName[:eq]
		valueName := catName[eq+1:]
		prop, ok := unicodeSupportedPropertyAliases[normalizeUnicodeCategoryAlias(propName)]
		if !ok {
			return "", false
		}
		values := unicodeSupportedPropertyValueAliases[prop]
		if values == nil {
			return "", false
		}
		value, ok := values[normalizeUnicodeCategoryAlias(valueName)]
		if !ok {
			return "", false
		}
		canonical := prop + "=" + value
		if _, ok := unicodeCategories[canonical]; ok {
			return canonical, true
		}
	}

	return "", false
}

func isValidUnicodeCat(catName string) bool {
	_, ok := canonicalUnicodeCatName(catName)
	return ok
}

func (c *CharSet) addCategory(categoryName string, negate, caseInsensitive bool) {
	var ok bool
	categoryName, ok = canonicalUnicodeCatName(categoryName)
	if !ok {
		// unknown unicode category, script, or property "blah"
		panic(fmt.Errorf("unknown unicode category, script, or property '%v'", categoryName))

	}

	if caseInsensitive && (categoryName == "Ll" || categoryName == "Lu" || categoryName == "Lt") {
		// when RegexOptions.IgnoreCase is specified then {Ll} {Lu} and {Lt} cases should all match
		c.addCategories(
			Category{Cat: "Ll", Negate: negate},
			Category{Cat: "Lu", Negate: negate},
			Category{Cat: "Lt", Negate: negate})
	}
	c.addCategories(Category{Cat: categoryName, Negate: negate})
}

// Adds to the class any case-equivalence versions of characters already
// in the class. Used for case-insensitivity.
func (c *CharSet) addCaseEquivalences() {
	// we already have all case equiv
	if c.anything {
		return
	}
	for i := 0; i < len(c.ranges); i++ {
		r := c.ranges[i]
		if r.First == r.Last {
			equiv := tryFindCaseEquivalences(r.First)
			for _, eq := range equiv {
				c.addChar(eq)
			}
		} else {
			c.addCaseEquivalenceRange(r.First, r.Last)
		}
	}
}

// For a single range that's in the set, adds any additional ranges
// necessary to ensure that lowercase equivalents are also included.
func (c *CharSet) addCaseEquivalenceRange(chMin, chMax rune) {
	for i := chMin; i <= chMax; i++ {
		equiv := tryFindCaseEquivalences(i)
		for _, eq := range equiv {
			c.addChar(eq)
		}
	}
}

// Performs a fast lookup which determines if a character is involved in case conversion, as well as
// returns the OTHER characters that should be considered equivalent in case it does participate in case conversion.
func tryFindCaseEquivalences(ch rune) []rune {
	newCh := unicode.SimpleFold(ch)
	if newCh == ch {
		// no case support
		return nil
	}
	equiv := []rune{newCh}
	for {
		newCh = unicode.SimpleFold(newCh)
		if newCh == ch {
			return equiv
		}
		equiv = append(equiv, newCh)
	}
}

func (c *CharSet) addSubtraction(sub *CharSet) {
	c.sub = sub
}

func (c *CharSet) addRange(chMin, chMax rune) {
	c.ranges = append(c.ranges, SingleRange{First: chMin, Last: chMax})
	c.canonicalize()
}

func (c *CharSet) addNamedASCII(name string, negate bool) bool {
	var rs []SingleRange

	switch name {
	case "alnum":
		rs = []SingleRange{{'0', '9'}, {'A', 'Z'}, {'a', 'z'}}
	case "alpha":
		rs = []SingleRange{{'A', 'Z'}, {'a', 'z'}}
	case "ascii":
		rs = []SingleRange{{0, 0x7f}}
	case "blank":
		rs = []SingleRange{{'\t', '\t'}, {' ', ' '}}
	case "cntrl":
		rs = []SingleRange{{0, 0x1f}, {0x7f, 0x7f}}
	case "digit":
		c.addDigit(false, negate)
	case "graph":
		rs = []SingleRange{{'!', '~'}}
	case "lower":
		rs = []SingleRange{{'a', 'z'}}
	case "print":
		rs = []SingleRange{{' ', '~'}}
	case "punct": //[!-/:-@[-`{-~]
		rs = []SingleRange{{'!', '/'}, {':', '@'}, {'[', '`'}, {'{', '~'}}
	case "space":
		c.addSpace(true, false, negate)
	case "upper":
		rs = []SingleRange{{'A', 'Z'}}
	case "word":
		c.addWord(true, negate)
	case "xdigit":
		rs = []SingleRange{{'0', '9'}, {'A', 'F'}, {'a', 'f'}}
	default:
		return false
	}

	if len(rs) > 0 {
		if negate {
			c.addNegativeRanges(rs)
		} else {
			c.addRanges(rs)
		}
	}

	return true
}

type singleRangeSorter []SingleRange

func (p singleRangeSorter) Len() int           { return len(p) }
func (p singleRangeSorter) Less(i, j int) bool { return p[i].First < p[j].First }
func (p singleRangeSorter) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Logic to reduce a character class to a unique, sorted form.
func (c *CharSet) canonicalize() {
	var i, j int
	var last rune

	if len(c.ranges) == 0 {
		return
	}

	//
	// Find and eliminate overlapping or abutting ranges
	//

	if len(c.ranges) > 1 {
		sort.Sort(singleRangeSorter(c.ranges))

		done := false

		for i, j = 1, 0; ; i++ {
			for last = c.ranges[j].Last; ; i++ {
				if i == len(c.ranges) || last >= unicode.MaxRune {
					done = true
					break
				}

				CurrentRange := c.ranges[i]
				if CurrentRange.First > last+1 {
					break
				}

				if last < CurrentRange.Last {
					last = CurrentRange.Last
				}
			}

			c.ranges[j] = SingleRange{First: c.ranges[j].First, Last: last}

			j++

			if done {
				break
			}

			if j < i {
				c.ranges[j] = c.ranges[i]
			}
		}

		c.ranges = append(c.ranges[:j], c.ranges[len(c.ranges):]...)
	}

	// If the class now represents a single negated range, but does so by including every
	// other character, invert it to produce a normalized form with a single range.  This
	// is valuable for subsequent optimizations in most of the engines.
	if !c.negate && c.sub == nil && len(c.categories) == 0 {
		if len(c.ranges) == 2 {
			// There are two ranges in the list.  See if there's one missing range between them.
			// Such a range might be as small as a single character.
			if c.ranges[0].First == 0 &&
				c.ranges[1].Last >= unicode.MaxRune &&
				c.ranges[0].Last < c.ranges[1].First-1 {
				c.ranges = []SingleRange{{c.ranges[0].Last + 1, c.ranges[1].First - 1}}
				c.negate = true
			}
		} else if len(c.ranges) == 1 {
			switch c.ranges[0].First {
			case 0:
				// There's only one range in the list.  Does it include everything but the last char?
				if c.ranges[0].Last == unicode.MaxRune-1 {
					c.ranges[0] = SingleRange{unicode.MaxRune, unicode.MaxRune}
					c.negate = true
				}
			case 1:
				// Or everything but the first char?
				if c.ranges[0].Last >= unicode.MaxRune {
					c.ranges[0] = SingleRange{'\x00', '\x00'}
					c.negate = true
				}
			}
		}
	}

	// If the class now has a range that includes everything, and if it doesn't have subtraction,
	// we can remove all of its categories, as they're duplicative (the set already includes everything).
	if !c.negate &&
		c.sub == nil &&
		len(c.ranges) == 1 && c.ranges[0].First == 0 && c.ranges[0].Last >= unicode.MaxRune {

		c.makeAnything()
	}

	// If there's only a single character omitted from ranges, if there's no subtractor, and if there are categories,
	// see if that character is in the categories.  If it is, then we can replace whole thing with a complete "any" range.
	// If it's not, then we can remove the categories, as they're only duplicating the rest of the range, turning the set
	// into a "not one". This primarily helps in the case of a synthesized set from analysis that ends up combining '.' with
	// categories, as we want to reduce that set down to either [^\n] or [\0-\uFFFF]. (This can be extrapolated to any number
	// of missing characters; in fact, categories in general are superfluous and the entire set can be represented as ranges.
	// But categories serve as a space optimization, and we strike a balance between testing many characters and the time/complexity
	// it takes to do so.  Thus, we limit this to the common case of a single missing character.)
	if !c.negate && c.sub == nil && len(c.categories) > 0 &&
		len(c.ranges) == 2 && c.ranges[0].First == 0 && c.ranges[0].Last+2 == c.ranges[1].First && c.ranges[1].Last == unicode.MaxRune {

		if c.charInCategories(c.ranges[0].Last + 1) {
			//c.ranges = []SingleRange{{'\x00', unicode.MaxRune}}
			c.makeAnything()
		} else {
			c.negate = true
			c.ranges = []SingleRange{{c.ranges[0].Last + 1, c.ranges[0].Last + 1}}
			c.categories = []Category{}
		}
	}
}

// Adds to the class any lowercase versions of characters already
// in the class. Used for case-insensitivity.
func (c *CharSet) addLowercase() {
	if c.anything {
		return
	}
	toAdd := []SingleRange{}
	for i := 0; i < len(c.ranges); i++ {
		r := c.ranges[i]
		if r.First == r.Last {
			lower := unicode.ToLower(r.First)
			c.ranges[i] = SingleRange{First: lower, Last: lower}
		} else {
			toAdd = append(toAdd, r)
		}
	}

	for _, r := range toAdd {
		c.addLowercaseRange(r.First, r.Last)
	}
	c.canonicalize()
}

/**************************************************************************
    Let U be the set of Unicode character values and let L be the lowercase
    function, mapping from U to U. To perform case insensitive matching of
    character sets, we need to be able to map an interval I in U, say

        I = [chMin, chMax] = { ch : chMin <= ch <= chMax }

    to a set A such that A contains L(I) and A is contained in the union of
    I and L(I).

    The table below partitions U into intervals on which L is non-decreasing.
    Thus, for any interval J = [a, b] contained in one of these intervals,
    L(J) is contained in [L(a), L(b)].

    It is also true that for any such J, [L(a), L(b)] is contained in the
    union of J and L(J). This does not follow from L being non-decreasing on
    these intervals. It follows from the nature of the L on each interval.
    On each interval, L has one of the following forms:

        (1) L(ch) = constant            (LowercaseSet)
        (2) L(ch) = ch + offset         (LowercaseAdd)
        (3) L(ch) = ch | 1              (LowercaseBor)
        (4) L(ch) = ch + (ch & 1)       (LowercaseBad)

    It is easy to verify that for any of these forms [L(a), L(b)] is
    contained in the union of [a, b] and L([a, b]).
***************************************************************************/

const (
	LowercaseSet = 0 // Set to arg.
	LowercaseAdd = 1 // Add arg.
	LowercaseBor = 2 // Bitwise or with 1.
	LowercaseBad = 3 // Bitwise and with 1 and add original.
)

type lcMap struct {
	chMin, chMax rune
	op, data     int32
}

var lcTable = []lcMap{
	{'\u0041', '\u005A', LowercaseAdd, 32},
	{'\u00C0', '\u00DE', LowercaseAdd, 32},
	{'\u0100', '\u012E', LowercaseBor, 0},
	{'\u0130', '\u0130', LowercaseSet, 0x0069},
	{'\u0132', '\u0136', LowercaseBor, 0},
	{'\u0139', '\u0147', LowercaseBad, 0},
	{'\u014A', '\u0176', LowercaseBor, 0},
	{'\u0178', '\u0178', LowercaseSet, 0x00FF},
	{'\u0179', '\u017D', LowercaseBad, 0},
	{'\u0181', '\u0181', LowercaseSet, 0x0253},
	{'\u0182', '\u0184', LowercaseBor, 0},
	{'\u0186', '\u0186', LowercaseSet, 0x0254},
	{'\u0187', '\u0187', LowercaseSet, 0x0188},
	{'\u0189', '\u018A', LowercaseAdd, 205},
	{'\u018B', '\u018B', LowercaseSet, 0x018C},
	{'\u018E', '\u018E', LowercaseSet, 0x01DD},
	{'\u018F', '\u018F', LowercaseSet, 0x0259},
	{'\u0190', '\u0190', LowercaseSet, 0x025B},
	{'\u0191', '\u0191', LowercaseSet, 0x0192},
	{'\u0193', '\u0193', LowercaseSet, 0x0260},
	{'\u0194', '\u0194', LowercaseSet, 0x0263},
	{'\u0196', '\u0196', LowercaseSet, 0x0269},
	{'\u0197', '\u0197', LowercaseSet, 0x0268},
	{'\u0198', '\u0198', LowercaseSet, 0x0199},
	{'\u019C', '\u019C', LowercaseSet, 0x026F},
	{'\u019D', '\u019D', LowercaseSet, 0x0272},
	{'\u019F', '\u019F', LowercaseSet, 0x0275},
	{'\u01A0', '\u01A4', LowercaseBor, 0},
	{'\u01A7', '\u01A7', LowercaseSet, 0x01A8},
	{'\u01A9', '\u01A9', LowercaseSet, 0x0283},
	{'\u01AC', '\u01AC', LowercaseSet, 0x01AD},
	{'\u01AE', '\u01AE', LowercaseSet, 0x0288},
	{'\u01AF', '\u01AF', LowercaseSet, 0x01B0},
	{'\u01B1', '\u01B2', LowercaseAdd, 217},
	{'\u01B3', '\u01B5', LowercaseBad, 0},
	{'\u01B7', '\u01B7', LowercaseSet, 0x0292},
	{'\u01B8', '\u01B8', LowercaseSet, 0x01B9},
	{'\u01BC', '\u01BC', LowercaseSet, 0x01BD},
	{'\u01C4', '\u01C5', LowercaseSet, 0x01C6},
	{'\u01C7', '\u01C8', LowercaseSet, 0x01C9},
	{'\u01CA', '\u01CB', LowercaseSet, 0x01CC},
	{'\u01CD', '\u01DB', LowercaseBad, 0},
	{'\u01DE', '\u01EE', LowercaseBor, 0},
	{'\u01F1', '\u01F2', LowercaseSet, 0x01F3},
	{'\u01F4', '\u01F4', LowercaseSet, 0x01F5},
	{'\u01FA', '\u0216', LowercaseBor, 0},
	{'\u0386', '\u0386', LowercaseSet, 0x03AC},
	{'\u0388', '\u038A', LowercaseAdd, 37},
	{'\u038C', '\u038C', LowercaseSet, 0x03CC},
	{'\u038E', '\u038F', LowercaseAdd, 63},
	{'\u0391', '\u03AB', LowercaseAdd, 32},
	{'\u03E2', '\u03EE', LowercaseBor, 0},
	{'\u0401', '\u040F', LowercaseAdd, 80},
	{'\u0410', '\u042F', LowercaseAdd, 32},
	{'\u0460', '\u0480', LowercaseBor, 0},
	{'\u0490', '\u04BE', LowercaseBor, 0},
	{'\u04C1', '\u04C3', LowercaseBad, 0},
	{'\u04C7', '\u04C7', LowercaseSet, 0x04C8},
	{'\u04CB', '\u04CB', LowercaseSet, 0x04CC},
	{'\u04D0', '\u04EA', LowercaseBor, 0},
	{'\u04EE', '\u04F4', LowercaseBor, 0},
	{'\u04F8', '\u04F8', LowercaseSet, 0x04F9},
	{'\u0531', '\u0556', LowercaseAdd, 48},
	{'\u10A0', '\u10C5', LowercaseAdd, 48},
	{'\u1E00', '\u1EF8', LowercaseBor, 0},
	{'\u1F08', '\u1F0F', LowercaseAdd, -8},
	{'\u1F18', '\u1F1F', LowercaseAdd, -8},
	{'\u1F28', '\u1F2F', LowercaseAdd, -8},
	{'\u1F38', '\u1F3F', LowercaseAdd, -8},
	{'\u1F48', '\u1F4D', LowercaseAdd, -8},
	{'\u1F59', '\u1F59', LowercaseSet, 0x1F51},
	{'\u1F5B', '\u1F5B', LowercaseSet, 0x1F53},
	{'\u1F5D', '\u1F5D', LowercaseSet, 0x1F55},
	{'\u1F5F', '\u1F5F', LowercaseSet, 0x1F57},
	{'\u1F68', '\u1F6F', LowercaseAdd, -8},
	{'\u1F88', '\u1F8F', LowercaseAdd, -8},
	{'\u1F98', '\u1F9F', LowercaseAdd, -8},
	{'\u1FA8', '\u1FAF', LowercaseAdd, -8},
	{'\u1FB8', '\u1FB9', LowercaseAdd, -8},
	{'\u1FBA', '\u1FBB', LowercaseAdd, -74},
	{'\u1FBC', '\u1FBC', LowercaseSet, 0x1FB3},
	{'\u1FC8', '\u1FCB', LowercaseAdd, -86},
	{'\u1FCC', '\u1FCC', LowercaseSet, 0x1FC3},
	{'\u1FD8', '\u1FD9', LowercaseAdd, -8},
	{'\u1FDA', '\u1FDB', LowercaseAdd, -100},
	{'\u1FE8', '\u1FE9', LowercaseAdd, -8},
	{'\u1FEA', '\u1FEB', LowercaseAdd, -112},
	{'\u1FEC', '\u1FEC', LowercaseSet, 0x1FE5},
	{'\u1FF8', '\u1FF9', LowercaseAdd, -128},
	{'\u1FFA', '\u1FFB', LowercaseAdd, -126},
	{'\u1FFC', '\u1FFC', LowercaseSet, 0x1FF3},
	{'\u2160', '\u216F', LowercaseAdd, 16},
	{'\u24B6', '\u24D0', LowercaseAdd, 26},
	{'\uFF21', '\uFF3A', LowercaseAdd, 32},
}

func (c *CharSet) addLowercaseRange(chMin, chMax rune) {
	var i, iMax, iMid int
	var chMinT, chMaxT rune
	var lc lcMap

	for i, iMax = 0, len(lcTable); i < iMax; {
		iMid = (i + iMax) / 2
		if lcTable[iMid].chMax < chMin {
			i = iMid + 1
		} else {
			iMax = iMid
		}
	}

	for ; i < len(lcTable); i++ {
		lc = lcTable[i]
		if lc.chMin > chMax {
			return
		}
		chMinT = lc.chMin
		if chMinT < chMin {
			chMinT = chMin
		}

		chMaxT = lc.chMax
		if chMaxT > chMax {
			chMaxT = chMax
		}

		switch lc.op {
		case LowercaseSet:
			chMinT = rune(lc.data)
			chMaxT = rune(lc.data)
		case LowercaseAdd:
			chMinT += lc.data
			chMaxT += lc.data
		case LowercaseBor:
			chMinT |= 1
			chMaxT |= 1
		case LowercaseBad:
			chMinT += (chMinT & 1)
			chMaxT += (chMaxT & 1)
		}

		if chMinT < chMin || chMaxT > chMax {
			c.addRange(chMinT, chMaxT)
		}
	}
}

// Determines whether two sets could overlap.
func (set1 *CharSet) MayOverlap(set2 *CharSet) bool {
	// If the sets are identical, there's obviously overlap.
	if set1.Equals(set2) {
		return true
	}

	// If either set is all-inclusive, there's overlap by definition (unless
	// the other set is empty, but that's so rare it's not worth checking.)
	if set1.IsAnything() || set2.IsAnything() {
		return true
	}

	// If one set is negated and the other one isn't, we're in one of two situations:
	// - The remainder of the sets are identical, in which case these are inverses of
	//   each other, and they don't overlap.
	// - The remainder of the sets aren't identical, in which case there's very likely
	//   overlap, and it's not worth spending more time investigating.
	set1Negated := set1.IsNegated()
	set2Negated := set2.IsNegated()
	if set1Negated != set2Negated {

		return !set1.equals(set2, true)
	}

	// If the sets are negated, since they're not equal, there's almost certainly overlap.
	if set1Negated {
		return true
	}

	// Special-case some known, common classes that don't overlap.
	if knownDistinctSets(set1, set2) || knownDistinctSets(set2, set1) {
		return false
	}

	// If set2 can be easily enumerated (e.g. no unicode categories), then enumerate it and
	// check if any of its members are in set1.  Otherwise, the same for set1.
	if !set2.HasSubtraction() && len(set2.categories) == 0 {
		return mayOverlapByEnumeration(set1, set2)
	} else if !set1.HasSubtraction() && len(set1.categories) == 0 {
		return mayOverlapByEnumeration(set2, set1)
	}

	// Assume that everything else might overlap.  In the future if it proved impactful, we could be more accurate here,
	// at the exense of more computation time.
	return true
}

func knownDistinctSets(set1, set2 *CharSet) bool {

	return (set1.Equals(SpaceClass()) || set1.Equals(ECMASpaceClass())) &&
		(set2.Equals(DigitClass()) || set2.Equals(WordClass()) ||
			set2.Equals(ECMADigitClass()) || set2.Equals(ECMAWordClass()))

}

func mayOverlapByEnumeration(set1, set2 *CharSet) bool {
	for i := 0; i < len(set2.ranges); i++ {
		for c := set2.ranges[i].First; c <= set2.ranges[i].Last; c++ {
			if set1.CharIn(c) {
				return true
			}
		}
	}

	return false
}

// Gets all of the characters in the specified set, storing them into the provided span.
//
// Only considers character classes that only contain sets (no categories),
// just simple sets containing starting/ending pairs (subtraction from those pairs
// is factored in, however).The returned characters may be negated: if IsNegated(set)
// is false, then the returned characters are the only ones that match; if it returns
// true, then the returned characters are the only ones that don't match.
func (c *CharSet) GetSetChars(maxChars int) []rune {
	// don't support categories, just ranges
	if len(c.categories) > 0 || len(c.ranges) > maxChars {
		return nil
	}

	// Negation with subtraction is too cumbersome to reason about efficiently.
	if c.IsNegated() && c.HasSubtraction() {
		return nil
	}

	chars := make([]rune, 0, maxChars)
	curWork := 0
	// Iterate through the pairs of ranges, storing each value in each range
	// into the supplied span.  If they all won't fit, we give up and return 0.
	// Otherwise we return the number found.  Note that we don't bother to handle
	// the corner case where the last range's upper bound is LastChar (\uFFFF),
	// based on it a) complicating things, and b) it being really unlikely to
	// be part of a small set.
	for _, r := range c.ranges {
		// loop through each char in the range
		for ch := r.First; ch <= r.Last; ch++ {

			// Keep track of how many characters we've checked. This could work
			// just comparing count rather than evaluated, but we also want to
			// limit how much work is done here, which we can do by constraining
			// the number of checks to the size of the storage provided.
			curWork++
			if curWork > maxChars {
				return nil
			}

			// If the set is all ranges but has a subtracted class,
			// validate the char is actually in the set prior to storing it:
			// it might be in the subtracted range.
			if c.HasSubtraction() && !c.CharIn(ch) {
				continue
			}
			chars = append(chars, ch)
		}
	}
	return chars
}

func (c *CharSet) Hash() []byte {
	b := &bytes.Buffer{}
	c.mapHashFill(b)
	return b.Bytes()
}

func (c *CharSet) Equals(c2 *CharSet) bool {
	return c.equals(c2, false)
}

func (c *CharSet) equals(c2 *CharSet, ignoreNegate bool) bool {
	if c == nil && c2 == nil {
		return true
	}
	if c == nil && c2 != nil || c2 == nil && c != nil {
		return false
	}

	if !ignoreNegate {
		if c.negate != c2.negate {
			return false
		}
	}
	if c.anything != c2.anything {
		return false
	}

	if !slices.Equal(c.ranges, c2.ranges) {
		return false
	}
	if !slices.Equal(c.categories, c2.categories) {
		return false
	}

	return c.sub.equals(c2.sub, false)
}

var whitespaceChars = []rune{'\u0009', '\u000A', '\u000B', '\u000C', '\u000D',
	'\u0020', '\u0085', '\u00A0', '\u1680', '\u2000',
	'\u2001', '\u2002', '\u2003', '\u2004', '\u2005',
	'\u2006', '\u2007', '\u2008', '\u2009', '\u200A',
	'\u2028', '\u2029', '\u202F', '\u205F', '\u3000'}

// Gets whether the specified set is a named set with a reasonably small count
// of Unicode characters. Designed to help the regexp code generator choose a better
// search algo for finding chars
// Description is a short name that can be used as part of a var name in code gen
func (c *CharSet) IsUnicodeCategoryOfSmallCharCount() (isSmall bool, chars []rune, negated bool, desc string) {
	// figure out if we're SpaceClass, RE2SpaceClass, ECMASpaceClass or inverse
	// "hash" ourselves -- this is actually fully serialized, not just hashed
	if c.IsSingleton() {
		return true, []rune{c.SingletonChar()}, false, ""
	}
	if c.IsSingletonInverse() {
		return true, []rune{c.SingletonChar()}, true, ""
	}

	if c.Equals(SpaceClass()) {
		// we're SpaceClass
		return true, whitespaceChars, false, "whitespace"
	}
	if c.Equals(NotSpaceClass()) {
		// we're NotSpaceClass
		return true, whitespaceChars, true, "whitespace"
	}

	return false, nil, false, ""
}

// Gets whether the set description string is for two ASCII letters that case
// to each other under IgnoreCase rules.
func (c *CharSet) containsAsciiIgnoreCaseCharacter() (bool, []rune) {
	if c.IsNegated() {
		return false, nil
	}
	// get up to 3 chars, just to be able to error on both "too many" and "too few"
	twoChars := c.GetSetChars(3)
	return len(twoChars) == 2 && twoChars[0] < unicode.MaxASCII && twoChars[1] < unicode.MaxASCII &&
		(twoChars[0]|0x20) == (twoChars[1]|0x20) &&
		unicode.IsLetter(twoChars[0]) && unicode.IsLetter(twoChars[1]), twoChars
}

func anyParticipateInCaseConversion(chars []rune) bool {
	for _, c := range chars {
		if participatesInCaseConversion(c) {
			return true
		}
	}
	return false
}

func participatesInCaseConversion(ch rune) bool {
	/*
		case UnicodeCategory.ClosePunctuation:
		case UnicodeCategory.ConnectorPunctuation:
		case UnicodeCategory.Control:
		case UnicodeCategory.DashPunctuation:
		case UnicodeCategory.DecimalDigitNumber:
		case UnicodeCategory.FinalQuotePunctuation:
		case UnicodeCategory.InitialQuotePunctuation:
		case UnicodeCategory.LineSeparator:
		case UnicodeCategory.OpenPunctuation:
		case UnicodeCategory.OtherNumber:
		case UnicodeCategory.OtherPunctuation:
		case UnicodeCategory.ParagraphSeparator:
		case UnicodeCategory.SpaceSeparator:
	*/
	return !unicode.In(ch, unicode.Pe, unicode.Pc, unicode.Cc, unicode.Pd, unicode.Nd, unicode.Pf,
		unicode.Pi, unicode.Zl, unicode.Ps, unicode.No, unicode.Po, unicode.Zp, unicode.Zs)
}

func anyParticipatesInCaseConversion(str string) bool {
	for _, c := range str {
		if participatesInCaseConversion(c) {
			return true
		}
	}
	return false
}

func isAsciiRunes(in []rune) bool {
	for i := 0; i < len(in); i++ {
		if in[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func (c CharSet) GetIfNRanges(n int) []SingleRange {
	if len(c.categories) > 0 {
		return nil
	}
	if c.sub != nil {
		return nil
	}
	if len(c.ranges) == n {
		return c.ranges[:n]
	}
	return nil
}

func (c *CharSet) GetIfOnlyUnicodeCategories() (cats []Category, negate bool) {
	if c.sub != nil {
		return nil, false
	}
	if len(c.ranges) > 0 {
		return nil, false
	}
	// all cats need to be rationalized to the same negation or not
	if len(c.categories) == 0 {
		return nil, false
	}

	neg := c.categories[0].Negate
	for _, cat := range c.categories {
		if neg != cat.Negate || cat.Cat == SpaceCategoryText || cat.Cat == WordCategoryText {
			// negate some and not others...a problem
			// or one of our non-unicode categories
			return nil, false
		}
	}

	// tell the caller to negate if either all the categories
	// are negated or the set as a whole is negated, but not
	// both
	return c.categories, neg != c.negate
}

type CharClassAnalysisResults struct {
	// true if the set contains only ranges; false if it contains Unicode categories and/or subtraction.
	OnlyRanges bool
	// true if we know for sure that the set contains only ASCII values; otherwise, false.
	// This can only be true if OnlyRanges is true.
	ContainsOnlyAscii bool
	// true if we know for sure that the set doesn't contain any ASCII values; otherwise, false.
	// This can only be true if OnlyRanges is true.
	ContainsNoAscii bool
	// true if we know for sure that all ASCII values are in the set; otherwise, false.
	// This can only be true if OnlyRanges is true.
	AllAsciiContained bool
	// true if we know for sure that all non-ASCII values are in the set; otherwise, false.
	// This can only be true if OnlyRanges is true.
	AllNonAsciiContained bool
	// The inclusive lower bound.
	// This is only valid if OnlyRanges is true.
	LowerBoundInclusiveIfOnlyRanges rune
	// The exclusive upper bound.
	// This is only valid if OnlyRanges is true.
	UpperBoundExclusiveIfOnlyRanges rune
}

// <summary>Analyzes the set to determine some basic properties that can be used to optimize usage.
func (set *CharSet) Analyze() CharClassAnalysisResults {
	// The analysis is performed based entirely on ranges contained within the set.
	// Thus, we require that it can be "easily enumerated", meaning it contains only
	// ranges (and more specifically those with both the lower inclusive and upper
	// exclusive bounds specified). We also permit the set to contain a subtracted
	// character class, as for non-negated sets, that can only narrow what's permitted,
	// and the analysis can be performed on the overestimate of the set prior to subtraction.
	// However, negation is performed before subtraction, which means we can't trust
	// the ranges to inform AllNonAsciiContained and AllAsciiContained, as the subtraction
	// could create holes in those.  As such, while we can permit subtraction for non-negated
	// sets, for negated sets, we need to bail.
	if (set.IsNegated() && set.HasSubtraction()) || len(set.categories) > 0 || len(set.ranges) == 0 {
		// We can't make any strong claims about the set.
		return CharClassAnalysisResults{}
	}

	firstValueInclusive := set.ranges[0].First
	lastValueExclusive := set.ranges[len(set.ranges)-1].Last + 1

	if set.IsNegated() {
		// We're negated: if the upper bound of the range is ASCII, that means everything
		// above it is actually included, meaning all non-ASCII are in the class.
		// Similarly if the lower bound is non-ASCII, that means in a negated world
		// everything ASCII is included.
		return CharClassAnalysisResults{
			OnlyRanges:                      true,
			AllNonAsciiContained:            lastValueExclusive <= unicode.MaxASCII,
			AllAsciiContained:               firstValueInclusive >= unicode.MaxASCII,
			ContainsNoAscii:                 firstValueInclusive == 0 && set.ranges[0].Last >= unicode.MaxASCII,
			ContainsOnlyAscii:               false,
			LowerBoundInclusiveIfOnlyRanges: firstValueInclusive,
			UpperBoundExclusiveIfOnlyRanges: lastValueExclusive,
		}
	}

	// If the upper bound is ASCII, that means everything included in the class is ASCII.
	// Similarly if the lower bound is non-ASCII, that means no ASCII is in the class.
	return CharClassAnalysisResults{
		OnlyRanges:                      true,
		AllNonAsciiContained:            false,
		AllAsciiContained:               firstValueInclusive == 0 && set.ranges[0].Last >= unicode.MaxASCII && !set.HasSubtraction(),
		ContainsOnlyAscii:               lastValueExclusive <= unicode.MaxASCII,
		ContainsNoAscii:                 firstValueInclusive >= unicode.MaxASCII,
		LowerBoundInclusiveIfOnlyRanges: firstValueInclusive,
		UpperBoundExclusiveIfOnlyRanges: lastValueExclusive,
	}
}
