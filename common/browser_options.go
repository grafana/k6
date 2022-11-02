package common

import (
	"context"
	"errors"
	"time"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/k6ext"
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
}

// LaunchPersistentContextOptions stores browser launch options for persistent context.
type LaunchPersistentContextOptions struct {
	LaunchOptions
	BrowserContextOptions
}

// NewLaunchOptions returns a new LaunchOptions.
func NewLaunchOptions() *LaunchOptions {
	launchOpts := LaunchOptions{
		Env:               make(map[string]string),
		Headless:          true,
		LogCategoryFilter: ".*",
		Timeout:           DefaultTimeout,
	}
	return &launchOpts
}

// Parse parses launch options from a JS object.
func (l *LaunchOptions) Parse(ctx context.Context, opts goja.Value) error { //nolint:cyclop
	if !gojaValueExists(opts) {
		return errors.New("LaunchOptions does not exist in the runtime")
	}
	var (
		rt = k6ext.Runtime(ctx)
		o  = opts.ToObject(rt)
	)
	for _, k := range o.Keys() {
		v := o.Get(k)
		if v.Export() == nil {
			continue // don't override the defaults on `null``
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
