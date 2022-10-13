package common

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/k6ext"
)

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
func (l *LaunchOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := k6ext.Runtime(ctx)
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "args":
				v := opts.Get(k)
				if args, ok := v.Export().([]interface{}); ok {
					for _, argv := range args {
						l.Args = append(l.Args, fmt.Sprintf("%v", argv))
					}
				}
			case "debug":
				l.Debug = opts.Get(k).ToBoolean()
			case "devtools":
				l.Devtools = opts.Get(k).ToBoolean()
			case "env":
				v := opts.Get(k)
				switch v.ExportType() {
				case reflect.TypeOf(goja.Object{}):
					env := v.ToObject(rt)
					for _, k := range env.Keys() {
						l.Env[k] = env.Get(k).String()
					}
				}
			case "executablePath":
				l.ExecutablePath = opts.Get(k).String()
			case "headless":
				l.Headless = opts.Get(k).ToBoolean()
			case "ignoreDefaultArgs":
				v := opts.Get(k)
				var args []string
				err := rt.ExportTo(v, &args)
				if err != nil {
					return fmt.Errorf("ignoreDefaultArgs should be an array of strings: %w", err)
				}
				l.IgnoreDefaultArgs = append(l.IgnoreDefaultArgs, args...)
			case "logCategoryFilter":
				l.LogCategoryFilter = opts.Get(k).String()
			case "proxy":
				v := opts.Get(k)
				switch v.ExportType() {
				case reflect.TypeOf(goja.Object{}):
					env := v.ToObject(rt)
					switch k {
					case "server":
						l.Proxy.Server = env.Get(k).String()
					case "bypass":
						l.Proxy.Bypass = env.Get(k).String()
					case "username":
						l.Proxy.Username = env.Get(k).String()
					case "password":
						l.Proxy.Password = env.Get(k).String()
					}
				}
			case "slowMo":
				l.SlowMo, _ = time.ParseDuration(opts.Get(k).String())
			case "timeout":
				l.Timeout, _ = time.ParseDuration(opts.Get(k).String())
			}
		}
	}
	return nil
}
