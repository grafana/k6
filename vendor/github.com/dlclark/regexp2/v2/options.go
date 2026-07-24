package regexp2

var (
	// DefaultUnmarshalOptions used when unmarshaling a regex from text
	DefaultUnmarshalOptions = None
	// DefaultOptimizationOptions controls the default memory/performance trade-offs used by Compile.
	DefaultOptimizationOptions = OptimizationOptions{
		MaxCachedRuneBufferLength:    256 << 10,
		MaxCachedReplaceBufferLength: 256 << 10,
		MaxCachedReplacerDataEntries: 16,
		MaxCachedReplacerDataBytes:   4 << 10,
		DisableCharClassASCIIBitmap:  false,
	}
)

// RegexOptions impact the runtime and parsing behavior
// for each specific regex.  They are setable in code as well
// as in the regex pattern itself.
type RegexOptions int32

func (o RegexOptions) applyCompileOption(c *compileConfig) {
	c.regexOptions |= o
}

const (
	None                    RegexOptions = 0x0
	IgnoreCase              RegexOptions = 0x0001 // "i"
	Multiline               RegexOptions = 0x0002 // "m"
	ExplicitCapture         RegexOptions = 0x0004 // "n"
	Singleline              RegexOptions = 0x0010 // "s"
	IgnorePatternWhitespace RegexOptions = 0x0020 // "x"
	RightToLeft             RegexOptions = 0x0040 // "r"
	// ECMAScript attempts to follow ECMAScript regex behavior rather than C# RegexOptions.ECMAScript compatibility.
	ECMAScript RegexOptions = 0x0100 // "e"
	RE2        RegexOptions = 0x0200 // RE2 (regexp package) compatibility mode
	Unicode    RegexOptions = 0x0400 // "u"
)

// OptimizationOptions controls optional runtime caches and compile-time fast paths.
//
// For replacement data cache size fields, 0 disables persistent retention and
// -1 means unbounded. For pooled buffer cache size fields, 0 disables pooling
// and -1 allows all built-in size classes.
// Defaults are intentionally bounded so Compile is safe for mixed-cardinality inputs.
type OptimizationOptions struct {
	// MaxCachedRuneBufferLength limits retained string-to-rune buffers in the shared size-classed pool.
	MaxCachedRuneBufferLength int
	// MaxCachedReplaceBufferLength limits retained replacement output buffers in the shared size-classed pool.
	MaxCachedReplaceBufferLength int
	// MaxCachedReplacerDataEntries limits the number of parsed replacement patterns cached per Regexp.
	MaxCachedReplacerDataEntries int
	// MaxCachedReplacerDataBytes skips caching replacement patterns longer than this many bytes.
	MaxCachedReplacerDataBytes int
	// DisableCharClassASCIIBitmap disables compile-time ASCII bitmap construction for character classes.
	DisableCharClassASCIIBitmap bool
}

// CompileOption configures Compile and MustCompile.
type CompileOption interface {
	applyCompileOption(*compileConfig)
}

type compileConfig struct {
	regexOptions         RegexOptions
	optimizations        OptimizationOptions
	codeGen              bool
	debug                bool
	maintainCaptureOrder bool
}

type compileOptionFunc func(*compileConfig)

func (f compileOptionFunc) applyCompileOption(c *compileConfig) {
	f(c)
}

func (o OptimizationOptions) cacheReplacerData(replacement string) bool {
	if o.MaxCachedReplacerDataEntries == 0 {
		return false
	}
	return keepCacheBytes(o.MaxCachedReplacerDataBytes, len(replacement))
}

func keepCacheBytes(maxBytes, actualBytes int) bool {
	if maxBytes < 0 {
		return true
	}
	return maxBytes > 0 && actualBytes <= maxBytes
}

func newCompileConfig(options []CompileOption) compileConfig {
	c := compileConfig{
		optimizations: DefaultOptimizationOptions,
	}
	for _, option := range options {
		if option != nil {
			option.applyCompileOption(&c)
		}
	}
	return c
}

// OptionMaxCachedRuneBufferLength limits retained string-to-rune buffers in the shared size-classed pool.
func OptionMaxCachedRuneBufferLength(n int) CompileOption {
	return compileOptionFunc(func(c *compileConfig) {
		c.optimizations.MaxCachedRuneBufferLength = n
	})
}

// OptionMaxCachedReplaceBufferLength limits retained replacement output buffers in the shared size-classed pool.
func OptionMaxCachedReplaceBufferLength(n int) CompileOption {
	return compileOptionFunc(func(c *compileConfig) {
		c.optimizations.MaxCachedReplaceBufferLength = n
	})
}

// OptionMaxCachedReplacerDataEntries limits parsed replacement patterns cached per Regexp.
func OptionMaxCachedReplacerDataEntries(n int) CompileOption {
	return compileOptionFunc(func(c *compileConfig) {
		c.optimizations.MaxCachedReplacerDataEntries = n
	})
}

// OptionMaxCachedReplacerDataBytes skips caching replacement patterns longer than n bytes.
func OptionMaxCachedReplacerDataBytes(n int) CompileOption {
	return compileOptionFunc(func(c *compileConfig) {
		c.optimizations.MaxCachedReplacerDataBytes = n
	})
}

// OptionDisableCharClassASCIIBitmap disables compile-time ASCII bitmaps for character classes.
func OptionDisableCharClassASCIIBitmap() CompileOption {
	return compileOptionFunc(func(c *compileConfig) {
		c.optimizations.DisableCharClassASCIIBitmap = true
	})
}

// OptionIsCodeGen enables more expensive compile-time analysis intended for regexp2cg generated engines.
func OptionIsCodeGen() CompileOption {
	return compileOptionFunc(func(c *compileConfig) {
		c.codeGen = true
	})
}

// OptionDebug enables debug output and runner tracing for the compiled regexp.
func OptionDebug() CompileOption {
	return compileOptionFunc(func(c *compileConfig) {
		c.debug = true
	})
}

// OptionMaintainCaptureOrder assigns named and unnamed capture slots in pattern order.
func OptionMaintainCaptureOrder() CompileOption {
	return compileOptionFunc(func(c *compileConfig) {
		c.maintainCaptureOrder = true
	})
}
