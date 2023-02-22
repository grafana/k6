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

	onCloud bool // some options will be ignored when running in the cloud
}

// LaunchPersistentContextOptions stores browser launch options for persistent context.
type LaunchPersistentContextOptions struct {
	LaunchOptions
	BrowserContextOptions
}

// NewLaunchOptions returns a new LaunchOptions.
func NewLaunchOptions(onCloud bool) *LaunchOptions {
	return &LaunchOptions{
		Env:               make(map[string]string),
		Headless:          true,
		LogCategoryFilter: ".*",
		Timeout:           DefaultTimeout,
		onCloud:           onCloud,
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
		if l.shouldIgnoreOnCloud(k) {
			logger.Warnf("LaunchOptions", "setting %s option is disallowed on cloud.", k)
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

func (l *LaunchOptions) shouldIgnoreOnCloud(opt string) bool {
	if !l.onCloud {
		return false
	}
	shouldIgnoreOnCloud := map[string]struct{}{
		optDevTools:       {},
		optExecutablePath: {},
		optHeadless:       {},
	}
	_, ignore := shouldIgnoreOnCloud[opt]
	return ignore
}
