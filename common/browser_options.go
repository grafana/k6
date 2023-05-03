package common

import (
	"context"
	"time"

	"github.com/dop251/goja"

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

// LaunchPersistentContextOptions stores browser launch options for persistent context.
type LaunchPersistentContextOptions struct {
	BrowserOptions
	BrowserContextOptions
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
