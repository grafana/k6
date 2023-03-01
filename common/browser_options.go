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
	optDevTools          = "devtools"
	optEnv               = "env"
	optExecutablePath    = "executablePath"
	optHeadless          = "headless"
	optIgnoreDefaultArgs = "ignoreDefaultArgs"
	optLogCategoryFilter = "logCategoryFilter"
	optProxy             = "proxy"
	optSlowMo            = "slowMo"
	optTimeout           = "timeout"
)

// ProxyOptions allows configuring a proxy server.
type ProxyOptions struct {
	Server   string
	Bypass   string
	Username string
	Password string
}

// LaunchOptions stores browser launch options.
type LaunchOptions struct {
	Args              []string
	Debug             bool
	Devtools          bool
	Env               map[string]string
	ExecutablePath    string
	Headless          bool
	IgnoreDefaultArgs []string
	LogCategoryFilter string
	Proxy             ProxyOptions
	SlowMo            time.Duration
	Timeout           time.Duration

	isRemoteBrowser bool // some options will be ignored if browser is in a remote machine
}

// LaunchPersistentContextOptions stores browser launch options for persistent context.
type LaunchPersistentContextOptions struct {
	LaunchOptions
	BrowserContextOptions
}

// NewLaunchOptions returns a new LaunchOptions.
func NewLaunchOptions() *LaunchOptions {
	return &LaunchOptions{
		Env:               make(map[string]string),
		Headless:          true,
		LogCategoryFilter: ".*",
		Timeout:           DefaultTimeout,
	}
}

// NewRemoteBrowserLaunchOptions returns a new LaunchOptions
// for a browser running in a remote machine.
func NewRemoteBrowserLaunchOptions() *LaunchOptions {
	return &LaunchOptions{
		Env:               make(map[string]string),
		Headless:          true,
		LogCategoryFilter: ".*",
		Timeout:           DefaultTimeout,
		isRemoteBrowser:   true,
	}
}

// Parse parses launch options from a JS object.
func (l *LaunchOptions) Parse(ctx context.Context, logger *log.Logger, opts goja.Value) error { //nolint:cyclop
	// when opts is nil, we just return the default options without error.
	if !gojaValueExists(opts) {
		return nil
	}
	var (
		rt       = k6ext.Runtime(ctx)
		o        = opts.ToObject(rt)
		defaults = map[string]any{
			optEnv:               l.Env,
			optHeadless:          l.Headless,
			optLogCategoryFilter: l.LogCategoryFilter,
			optTimeout:           l.Timeout,
		}
	)
	for _, k := range o.Keys() {
		if l.shouldIgnoreIfBrowserIsRemote(k) {
			logger.Warnf("LaunchOptions", "setting %s option is disallowed when browser is remote", k)
			continue
		}
		v := o.Get(k)
		if v.Export() == nil {
			if dv, ok := defaults[k]; ok {
				logger.Warnf("LaunchOptions", "%s was null and set to its default: %v", k, dv)
			}
			continue
		}
		var err error
		switch k {
		case optArgs:
			err = exportOpt(rt, k, v, &l.Args)
		case optDebug:
			l.Debug, err = parseBoolOpt(k, v)
		case optDevTools:
			l.Devtools, err = parseBoolOpt(k, v)
		case optEnv:
			err = exportOpt(rt, k, v, &l.Env)
		case optExecutablePath:
			l.ExecutablePath, err = parseStrOpt(k, v)
		case optHeadless:
			l.Headless, err = parseBoolOpt(k, v)
		case optIgnoreDefaultArgs:
			err = exportOpt(rt, k, v, &l.IgnoreDefaultArgs)
		case optLogCategoryFilter:
			l.LogCategoryFilter, err = parseStrOpt(k, v)
		case optProxy:
			err = exportOpt(rt, k, v, &l.Proxy)
		case optSlowMo:
			l.SlowMo, err = parseTimeOpt(k, v)
		case optTimeout:
			l.Timeout, err = parseTimeOpt(k, v)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *LaunchOptions) shouldIgnoreIfBrowserIsRemote(opt string) bool {
	if !l.isRemoteBrowser {
		return false
	}

	shouldIgnoreIfBrowserIsRemote := map[string]struct{}{
		optArgs:              {},
		optDevTools:          {},
		optEnv:               {},
		optExecutablePath:    {},
		optHeadless:          {},
		optIgnoreDefaultArgs: {},
		optProxy:             {},
	}
	_, ignore := shouldIgnoreIfBrowserIsRemote[opt]

	return ignore
}
