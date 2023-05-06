package common

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/env"
	"github.com/grafana/xk6-browser/log"

	"go.k6.io/k6/lib/types"
)

const (
	// Script variables.

	optType = "type"

	// ENV variables.

	optArgs              = "K6_BROWSER_ARGS"
	optDebug             = "K6_BROWSER_DEBUG"
	optExecutablePath    = "K6_BROWSER_EXECUTABLE_PATH"
	optHeadless          = "K6_BROWSER_HEADLESS"
	optIgnoreDefaultArgs = "K6_BROWSER_IGNORE_DEFAULT_ARGS"
	optLogCategoryFilter = "K6_BROWSER_LOG_CATEGORY_FILTER"
	optSlowMo            = "K6_BROWSER_SLOWMO"
	optTimeout           = "K6_BROWSER_TIMEOUT"
)

// BrowserOptions stores browser options.
type BrowserOptions struct {
	Args              []string
	Debug             bool
	ExecutablePath    string
	Headless          bool
	IgnoreDefaultArgs []string
	LogCategoryFilter string
	SlowMo            time.Duration
	Timeout           time.Duration

	isRemoteBrowser bool // some options will be ignored if browser is in a remote machine
}

// NewLocalBrowserOptions returns a new BrowserOptions
// for a browser launched in the local machine.
func NewLocalBrowserOptions() *BrowserOptions {
	return &BrowserOptions{
		Headless:          true,
		LogCategoryFilter: ".*",
		Timeout:           DefaultTimeout,
	}
}

// NewRemoteBrowserOptions returns a new BrowserOptions
// for a browser running in a remote machine.
func NewRemoteBrowserOptions() *BrowserOptions {
	return &BrowserOptions{
		Headless:          true,
		LogCategoryFilter: ".*",
		Timeout:           DefaultTimeout,
		isRemoteBrowser:   true,
	}
}

// Parse parses browser options from a JS object.
func (bo *BrowserOptions) Parse( //nolint:cyclop
	ctx context.Context, logger *log.Logger, opts map[string]any, envLookup env.LookupFunc,
) error {
	// Parse opts
	bt, ok := opts[optType]
	// Only 'chromium' is supported by now, so return error
	// if type option is not set, or if it's set and its value
	// is different than 'chromium'
	if !ok {
		return errors.New("browser type option must be set")
	}
	if bt != "chromium" {
		return fmt.Errorf("unsupported browser type: %s", bt)
	}

	// Parse env
	envOpts := [...]string{
		optArgs,
		optDebug,
		optExecutablePath,
		optHeadless,
		optIgnoreDefaultArgs,
		optLogCategoryFilter,
		optSlowMo,
		optTimeout,
	}

	for _, e := range envOpts {
		ev, ok := envLookup(e)
		if !ok || ev == "" {
			continue
		}
		if bo.shouldIgnoreIfBrowserIsRemote(e) {
			logger.Warnf("BrowserOptions", "setting %s option is disallowed when browser is remote", e)
			continue
		}
		var err error
		switch e {
		case optArgs:
			bo.Args = parseListOpt(ev)
		case optDebug:
			bo.Debug, err = parseBoolOpt(e, ev)
		case optExecutablePath:
			bo.ExecutablePath = ev
		case optHeadless:
			bo.Headless, err = parseBoolOpt(e, ev)
		case optIgnoreDefaultArgs:
			bo.IgnoreDefaultArgs = parseListOpt(ev)
		case optLogCategoryFilter:
			bo.LogCategoryFilter = ev
		case optSlowMo:
			bo.SlowMo, err = parseTimeOpt(e, ev)
		case optTimeout:
			bo.Timeout, err = parseTimeOpt(e, ev)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (bo *BrowserOptions) shouldIgnoreIfBrowserIsRemote(opt string) bool {
	if !bo.isRemoteBrowser {
		return false
	}

	shouldIgnoreIfBrowserIsRemote := map[string]struct{}{
		optArgs:              {},
		optExecutablePath:    {},
		optHeadless:          {},
		optIgnoreDefaultArgs: {},
	}
	_, ignore := shouldIgnoreIfBrowserIsRemote[opt]

	return ignore
}

func parseBoolOpt(k, v string) (bool, error) {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s should be a boolean", k)
	}

	return b, nil
}

func parseStrOpt(key string, val goja.Value) (s string, err error) {
	if val.ExportType().Kind() != reflect.String {
		return "", fmt.Errorf("%s should be a string", key)
	}
	return val.String(), nil
}

func parseTimeOpt(k, v string) (time.Duration, error) {
	t, err := types.GetDurationValue(v)
	if err != nil {
		return time.Duration(0), fmt.Errorf("%s should be a time duration value: %w", k, err)
	}

	return t, nil
}

func parseListOpt(v string) []string {
	elems := strings.Split(v, ",")
	// If last element is a void string,
	// because value contained an ending comma,
	// remove it
	if elems[len(elems)-1] == "" {
		elems = elems[:len(elems)-1]
	}

	return elems
}

// exportOpt exports src to dst and dynamically returns an error
// depending on the type if an error occurs. Panics if dst is not
// a pointer and not points to a map, struct, or slice.
func exportOpt[T any](rt *goja.Runtime, key string, src goja.Value, dst T) error {
	typ := reflect.TypeOf(dst)
	if typ.Kind() != reflect.Pointer {
		panic("dst should be a pointer")
	}
	kind := typ.Elem().Kind()
	s, ok := map[reflect.Kind]string{
		reflect.Map:    "a map",
		reflect.Struct: "an object",
		reflect.Slice:  "an array of",
	}[kind]
	if !ok {
		panic("dst should be one of: map, struct, slice")
	}
	if err := rt.ExportTo(src, dst); err != nil {
		if kind == reflect.Slice {
			s += fmt.Sprintf(" %ss", typ.Elem().Elem())
		}
		return fmt.Errorf("%s should be %s: %w", key, s, err)
	}

	return nil
}
