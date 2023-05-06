package common

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/dop251/goja"
	"go.k6.io/k6/lib/types"

	"github.com/grafana/xk6-browser/k6ext"
	"github.com/grafana/xk6-browser/log"
)

const (
	optArgs              = "args"
	optDebug             = "debug"
	optExecutablePath    = "executablePath"
	optHeadless          = "headless"
	optIgnoreDefaultArgs = "ignoreDefaultArgs"
	optLogCategoryFilter = "logCategoryFilter"
	optSlowMo            = "slowMo"
	optTimeout           = "timeout"
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
func (bo *BrowserOptions) Parse(ctx context.Context, logger *log.Logger, opts goja.Value) error { //nolint:cyclop
	// when opts is nil, we just return the default options without error.
	if !gojaValueExists(opts) {
		return nil
	}
	var (
		rt       = k6ext.Runtime(ctx)
		o        = opts.ToObject(rt)
		defaults = map[string]any{
			optHeadless:          bo.Headless,
			optLogCategoryFilter: bo.LogCategoryFilter,
			optTimeout:           bo.Timeout,
		}
	)
	for _, k := range o.Keys() {
		if bo.shouldIgnoreIfBrowserIsRemote(k) {
			logger.Warnf("BrowserOptions", "setting %s option is disallowed when browser is remote", k)
			continue
		}
		v := o.Get(k)
		if v.Export() == nil {
			if dv, ok := defaults[k]; ok {
				logger.Warnf("BrowserOptions", "%s was null and set to its default: %v", k, dv)
			}
			continue
		}
		var err error
		switch k {
		case optArgs:
			err = exportOpt(rt, k, v, &bo.Args)
		case optDebug:
			bo.Debug, err = parseBoolOpt(k, v)
		case optExecutablePath:
			bo.ExecutablePath, err = parseStrOpt(k, v)
		case optHeadless:
			bo.Headless, err = parseBoolOpt(k, v)
		case optIgnoreDefaultArgs:
			err = exportOpt(rt, k, v, &bo.IgnoreDefaultArgs)
		case optLogCategoryFilter:
			bo.LogCategoryFilter, err = parseStrOpt(k, v)
		case optSlowMo:
			bo.SlowMo, err = parseTimeOpt(k, v)
		case optTimeout:
			bo.Timeout, err = parseTimeOpt(k, v)
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

func parseBoolOpt(key string, val goja.Value) (b bool, err error) {
	if val.ExportType().Kind() != reflect.Bool {
		return false, fmt.Errorf("%s should be a boolean", key)
	}
	b, _ = val.Export().(bool)
	return b, nil
}

func parseStrOpt(key string, val goja.Value) (s string, err error) {
	if val.ExportType().Kind() != reflect.String {
		return "", fmt.Errorf("%s should be a string", key)
	}
	return val.String(), nil
}

func parseTimeOpt(key string, val goja.Value) (t time.Duration, err error) {
	if t, err = types.GetDurationValue(val.String()); err != nil {
		return time.Duration(0), fmt.Errorf("%s should be a time duration value: %w", key, err)
	}
	return
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
