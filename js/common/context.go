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
	ctxKeyRuntime ctxKey = iota
	ctxKeyInitEnv
)

// WithRuntime attaches the given goja runtime to the context.
func WithRuntime(ctx context.Context, rt *goja.Runtime) context.Context {
	return context.WithValue(ctx, ctxKeyRuntime, rt)
}

// GetRuntime retrieves the attached goja runtime from the given context.
func GetRuntime(ctx context.Context) *goja.Runtime {
	v := ctx.Value(ctxKeyRuntime)
	if v == nil {
		return nil
	}
	return v.(*goja.Runtime)
}

// WithInitEnv attaches the given init environment to the context.
func WithInitEnv(ctx context.Context, initEnv *InitEnvironment) context.Context {
	return context.WithValue(ctx, ctxKeyInitEnv, initEnv)
}

// GetInitEnv retrieves the attached init environment struct from the given context.
func GetInitEnv(ctx context.Context) *InitEnvironment {
	v := ctx.Value(ctxKeyInitEnv)
	if v == nil {
		return nil
	}
	return v.(*InitEnvironment)
}
