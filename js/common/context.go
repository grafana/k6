/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

	"github.com/dop251/goja"
)

type ctxKey int

const (
	ctxKeyState ctxKey = iota
	ctxKeyRuntime
)

func WithState(ctx context.Context, state *State) context.Context {
	return context.WithValue(ctx, ctxKeyState, state)
}

func GetState(ctx context.Context) *State {
	v := ctx.Value(ctxKeyState)
	if v == nil {
		return nil
	}
	return v.(*State)
}

func WithRuntime(ctx context.Context, rt *goja.Runtime) context.Context {
	return context.WithValue(ctx, ctxKeyRuntime, rt)
}

func GetRuntime(ctx context.Context) *goja.Runtime {
	v := ctx.Value(ctxKeyRuntime)
	if v == nil {
		return nil
	}
	return v.(*goja.Runtime)
}
