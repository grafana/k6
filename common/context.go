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

	k6modules "go.k6.io/k6/js/modules"
)

type ctxKey int

const (
	ctxKeyLaunchOptions ctxKey = iota
	ctxKeyPid
	ctxKeyHooks
	ctxKeyVU
)

func WithHooks(ctx context.Context, hooks *Hooks) context.Context {
	return context.WithValue(ctx, ctxKeyHooks, hooks)
}

func GetHooks(ctx context.Context) *Hooks {
	v := ctx.Value(ctxKeyHooks)
	if v == nil {
		return nil
	}
	return v.(*Hooks)
}

func WithLaunchOptions(ctx context.Context, opts *LaunchOptions) context.Context {
	return context.WithValue(ctx, ctxKeyLaunchOptions, opts)
}

func GetLaunchOptions(ctx context.Context) *LaunchOptions {
	v := ctx.Value(ctxKeyLaunchOptions)
	if v == nil {
		return nil
	}
	return v.(*LaunchOptions)
}

// WithProcessID saves the browser process ID to the context.
func WithProcessID(ctx context.Context, pid int) context.Context {
	return context.WithValue(ctx, ctxKeyPid, pid)
}

// GetProcessID returns the browser process ID from the context.
func GetProcessID(ctx context.Context) int {
	v, _ := ctx.Value(ctxKeyPid).(int)
	return v // it will return zero on error
}

// WithVU returns a new context based on ctx with the k6 VU instance attached.
func WithVU(ctx context.Context, vu k6modules.VU) context.Context {
	return context.WithValue(ctx, ctxKeyVU, vu)
}

// GetVU returns the attached k6 VU instance from ctx, which can be used to
// retrieve the goja runtime and other k6 objects relevant to the currently
// executing VU.
// See https://github.com/grafana/k6/blob/v0.38.0/js/initcontext.go#L168-L186
func GetVU(ctx context.Context) k6modules.VU {
	v := ctx.Value(ctxKeyVU)
	if vu, ok := v.(k6modules.VU); ok {
		return vu
	}
	return nil
}

// contextWithDoneChan returns a new context that is canceled either
// when the done channel is closed or ctx is canceled.
func contextWithDoneChan(ctx context.Context, done chan struct{}) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		select {
		case <-done:
		case <-ctx.Done():
		}
	}()
	return ctx
}
