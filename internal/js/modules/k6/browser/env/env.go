// Package env provides types to interact with environment setup.
package env

import (
	"os"
	"strconv"
)

// Execution specific.
const (
	// InstanceScenarios is an environment variable that can be used to
	// define the extra scenarios details to use when running remotely.
	InstanceScenarios = "K6_INSTANCE_SCENARIOS"

	// WebSocketURLs is an environment variable that can be used to
	// define the WS URLs to connect to when running remotely.
	WebSocketURLs = "K6_BROWSER_WS_URL"

	// BrowserArguments is an environment variable that can be used to
	// pass extra arguments to the browser process.
	BrowserArguments = "K6_BROWSER_ARGS"

	// BrowserExecutablePath is an environment variable that can be used
	// to define the path to the browser to execute.
	BrowserExecutablePath = "K6_BROWSER_EXECUTABLE_PATH"

	// BrowserEnableDebugging is an environment variable that can be used to
	// define if the browser should be launched with debugging enabled.
	BrowserEnableDebugging = "K6_BROWSER_DEBUG"

	// BrowserHeadless is an environment variable that can be used to
	// define if the browser should be launched in headless mode.
	BrowserHeadless = "K6_BROWSER_HEADLESS"

	// BrowserIgnoreDefaultArgs is an environment variable that can be
	// used to define if the browser should ignore default arguments.
	BrowserIgnoreDefaultArgs = "K6_BROWSER_IGNORE_DEFAULT_ARGS"

	// BrowserGlobalTimeout is an environment variable that can be used
	// to set the global timeout for the browser.
	BrowserGlobalTimeout = "K6_BROWSER_TIMEOUT"
)

// Logging and debugging.
const (
	// EnableProfiling is an environment variable that can be used to
	// enable profiling for the browser. It will start up a debugging
	// server on ProfilingServerAddr.
	EnableProfiling = "K6_BROWSER_ENABLE_PPROF"

	// ProfilingServerAddr is the address of the profiling server.
	ProfilingServerAddr = "localhost:6060"

	// LogCaller is an environment variable that can be used to enable
	// the caller function information in the browser logs.
	LogCaller = "K6_BROWSER_LOG_CALLER"

	// LogLevel is an environment variable that can be used to set the
	// log level for the browser logs.
	LogLevel = "K6_BROWSER_LOG"

	// LogCategoryFilter is an environment variable that can be used to
	// filter the browser logs based on their category. It supports
	// regular expressions.
	LogCategoryFilter = "K6_BROWSER_LOG_CATEGORY_FILTER"
)

// Tracing.
const (
	// TracesMetadata is an environment variable that can be used to
	// set additional metadata to be included in the generated traces.
	// The format must comply with: key1=value1,key2=value2,...
	TracesMetadata = "K6_BROWSER_TRACES_METADATA"
)

// Screenshots.
const (
	// ScreenshotsOutput can be used to configure the browser module
	// to upload screenshots to a remote location instead of saving
	// to the local disk.
	ScreenshotsOutput = "K6_BROWSER_SCREENSHOTS_OUTPUT"
)

// Infrastructural.
const (
	// K6TestRunID represents the test run id. Note: this was taken from
	// k6.
	K6TestRunID = "K6_CLOUD_PUSH_REF_ID"
)

// LookupFunc defines a function to look up a key from the environment.
type LookupFunc func(key string) (string, bool)

// EmptyLookup is a LookupFunc that always returns "" and false.
func EmptyLookup(_ string) (string, bool) { return "", false }

// Lookup is a LookupFunc that uses os.LookupEnv.
func Lookup(key string) (string, bool) { return os.LookupEnv(key) } //nolint:forbidigo

// ConstLookup is a LookupFunc that always returns the given value and true
// if the key matches the given key. Otherwise it returns EmptyLookup
// behaviour. Useful for testing.
func ConstLookup(k, v string) LookupFunc {
	return func(key string) (string, bool) {
		if key == k {
			return v, true
		}
		return EmptyLookup(key)
	}
}

// LookupBool returns the result of Lookup as a bool.
// If the key does not exist or the value is not a valid bool, it returns false.
// Otherwise it returns the bool value and true.
func LookupBool(key string) (value bool, ok bool) {
	v, ok := Lookup(key)
	if !ok {
		return false, false
	}
	bv, err := strconv.ParseBool(v)
	if err != nil {
		return false, true
	}
	return bv, true
}

// IsBrowserHeadless returns true if the BrowserHeadless environment
// variable is not set or set to true.
// The default behaviour is to run the browser in headless mode.
func IsBrowserHeadless() bool {
	v, ok := LookupBool(BrowserHeadless)
	return !ok || v
}
