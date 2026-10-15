/*
Package regexp2 is a regexp package that has an interface similar to Go's framework regexp engine but uses a
more feature full regex engine behind the scenes.

It doesn't have constant time guarantees, but it allows backtracking and is compatible with Perl5 and .NET.
You'll likely be better off with the RE2 engine from the regexp package and should only use this if you
need to write very complex patterns or require compatibility with .NET.
*/
package regexp2

import (
	"container/list"
	"errors"
	"log"
	"math"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dlclark/regexp2/v2/syntax"
)

var (
	// DefaultMatchTimeout used when running regexp matches -- "forever"
	DefaultMatchTimeout = time.Duration(math.MaxInt64)
	// ErrBacktrackingStackLimit is returned when a match exceeds its configured backtracking stack size.
	ErrBacktrackingStackLimit = errors.New("regexp2: maximum backtracking stack size exceeded")
)

// Regexp is the representation of a compiled regular expression.
// A Regexp is safe for concurrent use by multiple goroutines.
type Regexp struct {
	// A match will time out if it takes (approximately) more than
	// MatchTimeout. This is a safety check in case the match
	// encounters catastrophic backtracking.  The default value
	// (DefaultMatchTimeout) causes all time out checking to be
	// suppressed.
	MatchTimeout time.Duration

	// read-only after Compile
	pattern string       // as passed to Compile
	options RegexOptions // options
	debug   bool

	caps     map[int]int    // capnum->index
	capnames map[string]int //capture group name -> index
	capslist []string       //sorted list of capture group names
	capsize  int            // size of the capture array

	code *syntax.Code // compiled program

	optimizations OptimizationOptions

	// cache of machines for running regexp
	runnerPool *sync.Pool

	replaceCache *replacerDataCache

	// hook points to override runner functions
	findFirstChar      func(r *Runner) bool
	execute            func(r *Runner) error
	executeQuick       func(r *Runner) error
	stringPrefixFilter StringPrefixFilter
	quickCode          *syntax.Code // bool-only program with unobservable captures removed
	// leftContextRunes is used when code is nil (registered engines).
	// The interpreter reads the same value from code.LeftContextRunes.
	leftContextRunes int
}

// Compile parses a regular expression and returns, if successful,
// a Regexp object that can be used to match against text.
func Compile(expr string, options ...CompileOption) (*Regexp, error) {
	c := newCompileConfig(options)
	return compile(expr, c)
}

func compile(expr string, c compileConfig) (*Regexp, error) {
	// parse it
	parseOptions := syntax.ParseOptions{
		RegexOptions:         syntax.RegexOptions(c.regexOptions),
		MaintainCaptureOrder: c.maintainCaptureOrder,
		CodeGen:              c.codeGen,
	}
	tree, err := syntax.Parse(expr, parseOptions)
	if err != nil {
		return nil, err
	}

	if c.debug {
		log.Print(tree.Dump())
	}

	// translate it to code
	code, err := syntax.Write(tree)
	if err != nil {
		return nil, err
	}
	if c.debug {
		log.Print(code.Dump())
	}
	if !c.optimizations.DisableCharClassASCIIBitmap {
		code.PrepareCharSetASCIIBitmaps()
	}

	// return it
	re := &Regexp{
		pattern:       expr,
		options:       c.regexOptions,
		debug:         c.debug,
		caps:          code.Caps,
		capnames:      tree.Capnames,
		capslist:      tree.Caplist,
		capsize:       code.Capsize,
		code:          code,
		quickCode:     makeQuickCode(code),
		MatchTimeout:  DefaultMatchTimeout,
		optimizations: c.optimizations,
	}
	re.stringPrefixFilter = newStringPrefixFilter(code)
	re.initCaches()
	return re, nil
}

func makeQuickCode(code *syntax.Code) *syntax.Code {
	if code == nil || len(code.QuickCodes) == 0 {
		return nil
	}
	quick := *code
	quick.Codes = code.QuickCodes
	quick.Dispatches = code.QuickDispatches
	quick.QuickCodes = nil
	quick.QuickDispatches = nil
	return &quick
}

// MustCompile is like Compile but panics if the expression cannot be parsed.
// It simplifies safe initialization of global variables holding compiled regular
// expressions.
func MustCompile(str string, options ...CompileOption) *Regexp {
	c := newCompileConfig(options)

	// lookup if we have a pre-built state machine for this pattern and options
	regexp := getEngineRegexp(str, c)
	if regexp != nil {
		return regexp
	}

	regexp, err := compile(str, c)
	if err != nil {
		panic(`regexp2: Compile(` + quote(str) + `): ` + err.Error())
	}
	return regexp
}

// Escape adds backslashes to any special characters in the input string
func Escape(input string) string {
	return syntax.Escape(input)
}

// Unescape removes any backslashes from previously-escaped special characters in the input string
func Unescape(input string) (string, error) {
	return syntax.Unescape(input)
}

// SetTimeoutPeriod is a debug function that sets the frequency of the timeout goroutine's sleep cycle.
// Defaults to 100ms. The only benefit of setting this lower is that the 1 background goroutine that manages
// timeouts may exit slightly sooner after all the timeouts have expired. See Github issue #63
func SetTimeoutCheckPeriod(d time.Duration) {
	clockPeriod = d
}

// StopTimeoutClock should only be used in unit tests to prevent the timeout clock goroutine
// from appearing like a leaking goroutine
func StopTimeoutClock() {
	stopClock()
}

// String returns the source text used to compile the regular expression.
func (re *Regexp) String() string {
	return re.pattern
}

func quote(s string) string {
	if strconv.CanBackquote(s) {
		return "`" + s + "`"
	}
	return strconv.Quote(s)
}

func (re *Regexp) RightToLeft() bool {
	return re.options&RightToLeft != 0
}

func (re *Regexp) Debug() bool {
	return re.debug
}

// Replace searches the input string and replaces each match found with the replacement text.
// Count will limit the number of matches attempted and startAt will allow
// us to skip past possible matches at the start of the input (left or right depending on RightToLeft option).
// Set startAt and count to -1 to go through the whole string
func (re *Regexp) Replace(input, replacement string, startAt, count int) (string, error) {
	data, err := re.getReplacerData(replacement)
	if err != nil {
		return "", err
	}

	return replace(re, data, nil, input, startAt, count)
}

func (re *Regexp) getReplacerData(replacement string) (*syntax.ReplacerData, error) {
	shouldCache := re.replaceCache != nil && re.optimizations.cacheReplacerData(replacement)
	if shouldCache {
		if data, ok := re.replaceCache.get(replacement); ok {
			return data, nil
		}
	}

	data, err := syntax.NewReplacerData(replacement, re.caps, re.capsize, re.capnames, syntax.RegexOptions(re.options))
	if err != nil {
		return nil, err
	}
	if shouldCache {
		re.replaceCache.add(replacement, data)
	}
	return data, nil
}

// ReplaceFunc searches the input string and replaces each match found using the string from the evaluator
// Count will limit the number of matches attempted and startAt will allow
// us to skip past possible matches at the start of the input (left or right depending on RightToLeft option).
// Set startAt and count to -1 to go through the whole string.
func (re *Regexp) ReplaceFunc(input string, evaluator MatchEvaluator, startAt, count int) (string, error) {
	return replace(re, nil, evaluator, input, startAt, count)
}

// FindStringMatch searches the input string for a Regexp match
func (re *Regexp) FindStringMatch(s string) (*Match, error) {
	startAt, ok, err := re.findStringMatchStart(s, -1)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return re.findDecodedStringMatch(s, startAt, -1)
}

// FindRunesMatch searches the input rune slice for a Regexp match
func (re *Regexp) FindRunesMatch(r []rune) (*Match, error) {
	return re.run(false, -1, -1, r, newMatchText(r))
}

// FindStringMatchStartingAt searches the input string for a Regexp match starting at the startAt index
func (re *Regexp) FindStringMatchStartingAt(s string, startAt int) (*Match, error) {
	candidate, ok, err := re.findStringMatchStart(s, startAt)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return re.findDecodedStringMatch(s, candidate, startAt)
}

func (re *Regexp) findDecodedStringMatch(s string, candidate, startAt int) (*Match, error) {
	// Returned matches retain their rune data, so this path must not consume a
	// pooled buffer that can never be returned.
	d := re.decodeStringInput(s, candidate, false)
	runner := re.getRunner()
	defer re.putRunner(runner)
	text := newStringMatchTextAt(s, d.runes, d.runeOffset, d.byteOffset)
	origin := re.stringSearchOrigin(s, startAt, d.runeStart)
	return runner.scan(d.runes, text, origin, d.runeStart, -1, false, re.MatchTimeout)
}

// FindRunesMatchStartingAt searches the input rune slice for a Regexp match starting at the startAt index
func (re *Regexp) FindRunesMatchStartingAt(r []rune, startAt int) (*Match, error) {
	return re.run(false, startAt, -1, r, newMatchText(r))
}

// FindAllStringIndex returns a slice of byte index pairs identifying all
// successive matches in s.
func (re *Regexp) FindAllStringIndex(s string, n int) ([][]int, error) {
	if n == 0 {
		return nil, nil
	}

	startAt, ok, err := re.findStringMatchStart(s, -1)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	// Index results only need byte offsets. Keep the mapper relative to the
	// decoded suffix so neither decoding nor mapping has to count prefix runes.
	d := decodeInput(s, startAt, re.decodeFrom(s, startAt), re.optimizations.MaxCachedRuneBufferLength, false)
	runner := re.getRunner()
	defer func() {
		re.putRunner(runner)
		d.release()
	}()
	byteOffsets := stringByteMapper{input: s[d.byteOffset:]}
	if re.RightToLeft() {
		byteOffsets.runePos = len(d.runes)
		byteOffsets.bytePos = len(byteOffsets.input)
	}
	if re.quickCode != nil {
		runner.code = re.quickCode
	}
	origin := re.stringSearchOrigin(s, -1, d.runeStart)
	return re.findAllRunesIndex(runner, d.runes, origin, d.runeStart, n, func(runeIndex, runeLength int) (int, int) {
		if len(d.runes) == len(byteOffsets.input) {
			return d.byteOffset + runeIndex, d.byteOffset + runeIndex + runeLength
		}
		return d.byteOffset + byteOffsets.byteIndex(runeIndex), d.byteOffset + byteOffsets.byteIndex(runeIndex+runeLength)
	})
}

// FindAllRunesIndex returns a slice of rune index pairs identifying all
// successive matches in r.
func (re *Regexp) FindAllRunesIndex(r []rune, n int) ([][]int, error) {
	if n == 0 {
		return nil, nil
	}

	runner := re.getRunner()
	defer re.putRunner(runner)

	startAt := 0
	if re.RightToLeft() {
		startAt = len(r)
	}
	if re.quickCode != nil {
		runner.code = re.quickCode
	}
	return re.findAllRunesIndex(runner, r, startAt, startAt, n, func(runeIndex, runeLength int) (int, int) {
		return runeIndex, runeIndex + runeLength
	})
}

func (re *Regexp) findAllRunesIndex(runner *Runner, input []rune, origin, startAt, n int, makeIndex func(runeIndex, runeLength int) (int, int)) ([][]int, error) {
	var out [][]int
	var flat []int
	if n > 0 {
		out = make([][]int, 0, n)
		flat = make([]int, 0, n*2)
	}

	prevEnd := -1
	previousMatchLength := -1
	for n != 0 {
		m, err := runner.scan(input, nil, origin, startAt, previousMatchLength, true, re.MatchTimeout)
		if err != nil {
			return nil, err
		}
		if m == nil {
			break
		}

		localIndex := m.runeSliceIndex()
		if m.RuneLength != 0 || localIndex != prevEnd {
			start, end := makeIndex(localIndex, m.RuneLength)
			flat = append(flat, start, end)
			out = append(out, flat[len(flat)-2:len(flat):len(flat)])
			prevEnd = localIndex + m.RuneLength
			if n > 0 {
				n--
			}
		}

		startAt = m.textpos
		origin = startAt
		previousMatchLength = m.RuneLength
	}
	return out, nil
}

type stringByteMapper struct {
	input   string
	runePos int
	bytePos int
}

// runeIndex is relative to input; each invalid UTF-8 byte counts as one rune.
func (m *stringByteMapper) byteIndex(runeIndex int) int {
	for m.runePos < runeIndex {
		size := 1
		if m.input[m.bytePos] >= utf8.RuneSelf {
			_, size = utf8.DecodeRuneInString(m.input[m.bytePos:])
		}
		m.bytePos += size
		m.runePos++
	}
	for m.runePos > runeIndex {
		size := 1
		if m.input[m.bytePos-1] >= utf8.RuneSelf {
			_, size = utf8.DecodeLastRuneInString(m.input[:m.bytePos])
		}
		m.bytePos -= size
		m.runePos--
	}
	return m.bytePos
}

// FindNextMatch returns the next match in the same input string as the match parameter.
// Will return nil if there is no next match or if given a nil match.
func (re *Regexp) FindNextMatch(m *Match) (*Match, error) {
	if m == nil {
		return nil, nil
	}

	return re.run(false, m.textpos, m.RuneLength, m.text.runes, m.text)
}

// MatchString return true if the string matches the regex
// error will be set if a timeout occurs
func (re *Regexp) MatchString(s string) (bool, error) {
	if re.stringPrefixFilter != nil && !re.RightToLeft() {
		candidateByteIndex, ok := re.stringPrefixFilter(s, 0)
		if !ok {
			return false, nil
		}

		return re.matchStringAt(s, candidateByteIndex)
	}
	return re.matchStringAt(s, -1)
}

func (re *Regexp) matchStringAt(s string, startAt int) (bool, error) {
	runner := re.getRunner()
	var input []rune
	var pooledInput *[]rune
	runeStart := 0
	if startAt <= 0 {
		// Common path: decode the whole string without start/offset work.
		input, pooledInput = decodeString(s, re.optimizations.MaxCachedRuneBufferLength)
		if re.RightToLeft() {
			runeStart = len(input)
		}
	} else {
		d := decodeInput(s, startAt, re.decodeFrom(s, startAt), re.optimizations.MaxCachedRuneBufferLength, false)
		input = d.runes
		pooledInput = d.pooled
		runeStart = d.runeStart
		if runeStart < 0 {
			runeStart = 0
		}
	}
	defer func() {
		re.putRunner(runner)
		if pooledInput != nil {
			*pooledInput = input
			pooledRuneBuffers.put(pooledInput)
		}
	}()
	if re.quickCode != nil {
		runner.code = re.quickCode
	}

	origin := re.stringSearchOrigin(s, -1, runeStart)
	m, err := runner.scan(input, nil, origin, runeStart, -1, true, re.MatchTimeout)
	if err != nil {
		return false, err
	}
	return m != nil, nil
}

// MatchRunes return true if the runes matches the regex
// error will be set if a timeout occurs
func (re *Regexp) MatchRunes(r []rune) (bool, error) {
	m, err := re.run(true, -1, -1, r, nil)
	if err != nil {
		return false, err
	}
	return m != nil, nil
}

// GetGroupNames Returns the set of strings used to name capturing groups in the expression.
func (re *Regexp) GetGroupNames() []string {
	var result []string

	if re.capslist == nil {
		result = make([]string, re.capsize)

		for i := 0; i < len(result); i++ {
			result[i] = strconv.Itoa(i)
		}
	} else {
		result = make([]string, len(re.capslist))
		copy(result, re.capslist)
	}

	return result
}

// GetGroupNumbers returns the integer group numbers corresponding to a group name.
func (re *Regexp) GetGroupNumbers() []int {
	var result []int

	if re.caps == nil {
		result = make([]int, re.capsize)

		for i := 0; i < len(result); i++ {
			result[i] = i
		}
	} else {
		result = make([]int, len(re.caps))

		for k, v := range re.caps {
			result[v] = k
		}
	}

	return result
}

// GroupNameFromNumber retrieves a group name that corresponds to a group number.
// It will return "" for an unknown group number. Unnamed groups automatically
// receive a name that is the decimal string equivalent of its number, except in
// ECMAScript mode where unnamed groups have no name.
func (re *Regexp) GroupNameFromNumber(i int) string {
	if re.capslist == nil {
		if i >= 0 && i < re.capsize {
			return strconv.Itoa(i)
		}

		return ""
	}

	if re.caps != nil {
		var ok bool
		if i, ok = re.caps[i]; !ok {
			return ""
		}
	}

	if i >= 0 && i < len(re.capslist) {
		return re.capslist[i]
	}

	return ""
}

// GroupNumberFromName returns a group number that corresponds to a group name.
// Returns -1 if the name is not a recognized group name. Numbered groups
// automatically get a group name that is the decimal string equivalent of its
// number, except in ECMAScript mode where unnamed groups have no name.
func (re *Regexp) GroupNumberFromName(name string) int {
	// look up name if we have a hashtable of names
	if re.capnames != nil {
		if k, ok := re.capnames[name]; ok {
			return k
		}

		return -1
	}

	// convert to an int if it looks like a number
	result := 0
	for i := 0; i < len(name); i++ {
		ch := name[i]

		if ch > '9' || ch < '0' {
			return -1
		}

		result *= 10
		result += int(ch - '0')
	}

	// return int if it's in range
	if result >= 0 && result < re.capsize {
		return result
	}

	return -1
}

// MarshalText implements [encoding.TextMarshaler]. The output
// matches that of calling the [Regexp.String] method.
func (re *Regexp) MarshalText() ([]byte, error) {
	return []byte(re.String()), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler] by calling
// [Compile] on the encoded value.
func (re *Regexp) UnmarshalText(text []byte) error {
	newRE, err := Compile(string(text), DefaultUnmarshalOptions)
	if err != nil {
		return err
	}
	*re = *newRE
	return nil
}

func (re *Regexp) initCaches() {
	re.runnerPool = &sync.Pool{
		New: func() any {
			return &Runner{
				re:   re,
				code: re.code,
			}
		},
	}
	if re.optimizations.MaxCachedReplacerDataEntries > 0 {
		re.replaceCache = newReplacerDataCache(re.optimizations.MaxCachedReplacerDataEntries)
	}
}

type replacerDataCache struct {
	mu      sync.Mutex
	maxSize int
	ll      *list.List
	cache   map[string]*list.Element
}

type replacerDataCacheEntry struct {
	key  string
	data *syntax.ReplacerData
}

func newReplacerDataCache(maxSize int) *replacerDataCache {
	return &replacerDataCache{
		maxSize: maxSize,
		ll:      list.New(),
		cache:   make(map[string]*list.Element),
	}
}

func (c *replacerDataCache) get(key string) (*syntax.ReplacerData, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		return ele.Value.(*replacerDataCacheEntry).data, true
	}
	return nil, false
}

func (c *replacerDataCache) add(key string, data *syntax.ReplacerData) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ele, ok := c.cache[key]; ok {
		ele.Value.(*replacerDataCacheEntry).data = data
		c.ll.MoveToFront(ele)
		return
	}

	ele := c.ll.PushFront(&replacerDataCacheEntry{key: key, data: data})
	c.cache[key] = ele
	if c.maxSize > 0 && c.ll.Len() > c.maxSize {
		oldest := c.ll.Back()
		if oldest != nil {
			c.ll.Remove(oldest)
			delete(c.cache, oldest.Value.(*replacerDataCacheEntry).key)
		}
	}
}
