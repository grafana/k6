/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package common

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/dop251/goja"
	k6common "go.k6.io/k6/js/common"
)

type ProxyOptions struct {
	Server   string
	Bypass   string
	Username string
	Password string
}

// LaunchOptions stores browser launch options
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

// LaunchPersistentContextOptions stores browser launch options for persistent context
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
	rt := k6common.GetRuntime(ctx)
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
				switch v.ExportType() {
				case reflect.TypeOf(goja.Object{}):
					args := v.Export().([]string)
					l.IgnoreDefaultArgs = append(l.IgnoreDefaultArgs, args...)
				}
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
