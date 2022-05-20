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
)

type ctxKey int

const (
	ctxKeyLaunchOptions ctxKey = iota
	ctxKeyHooks
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
