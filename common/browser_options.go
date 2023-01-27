package common

import (
	"context"
	"time"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/k6ext"
	"github.com/grafana/xk6-browser/log"
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
			"env":               l.Env,
			"headless":          l.Headless,
			"logCategoryFilter": l.LogCategoryFilter,
			"timeout":           l.Timeout,
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
		case "args":
			err = exportOpt(rt, k, v, &l.Args)
		case "debug":
			l.Debug, err = parseBoolOpt(k, v)
		case "devtools":
			l.Devtools, err = parseBoolOpt(k, v)
		case "env":
			err = exportOpt(rt, k, v, &l.Env)
		case "executablePath":
			l.ExecutablePath, err = parseStrOpt(k, v)
		case "headless":
			l.Headless, err = parseBoolOpt(k, v)
		case "ignoreDefaultArgs":
			err = exportOpt(rt, k, v, &l.IgnoreDefaultArgs)
		case "logCategoryFilter":
			l.LogCategoryFilter, err = parseStrOpt(k, v)
		case "proxy":
			err = exportOpt(rt, k, v, &l.Proxy)
		case "slowMo":
			l.SlowMo, err = parseTimeOpt(k, v)
		case "timeout":
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
	return opt == "devtools" || opt == "executablePath" || opt == "headless"
}
