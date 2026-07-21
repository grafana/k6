package regexp2

import (
	"sync"
)

type RuntimeEngineData struct {
	Caps               map[int]int        // capnum->index
	CapNames           map[string]int     // cap group name -> index
	CapsList           []string           // sorted list of capture group names
	CapSize            int                // size of the capture array
	FindFirstChar      func(*Runner) bool // generated candidate search
	Execute            func(*Runner) error
	StringPrefixFilter StringPrefixFilter // optional pre-decode candidate search for string input
}

type cacheKey struct {
	pattern              string
	opt                  RegexOptions
	maintainCaptureOrder bool
}

func RegisterEngine(pattern string, engine RuntimeEngineData, options ...CompileOption) {
	c := newCompileConfig(options)
	enginesMu.Lock()
	engines[cacheKeyFromConfig(pattern, c)] = engine
	enginesMu.Unlock()
}

func newEngineRegexp(pattern string, c compileConfig, engine RuntimeEngineData) *Regexp {
	re := &Regexp{
		pattern:            pattern,
		options:            c.regexOptions,
		debug:              c.debug,
		caps:               engine.Caps,
		capnames:           engine.CapNames,
		capslist:           engine.CapsList,
		capsize:            engine.CapSize,
		MatchTimeout:       DefaultMatchTimeout,
		optimizations:      c.optimizations,
		findFirstChar:      engine.FindFirstChar,
		execute:            engine.Execute,
		stringPrefixFilter: engine.StringPrefixFilter,
	}
	re.initCaches()
	return re
}

func getEngineRegexp(pattern string, c compileConfig) *Regexp {
	enginesMu.RLock()
	engine, ok := engines[cacheKeyFromConfig(pattern, c)]
	enginesMu.RUnlock()
	if !ok {
		return nil
	}
	return newEngineRegexp(pattern, c, engine)
}

func cacheKeyFromConfig(pattern string, c compileConfig) cacheKey {
	return cacheKey{
		pattern:              pattern,
		opt:                  c.regexOptions,
		maintainCaptureOrder: c.maintainCaptureOrder,
	}
}

var (
	enginesMu sync.RWMutex
	engines   = map[cacheKey]RuntimeEngineData{}
)
