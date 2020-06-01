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

	"github.com/loadimpact/k6/core"
)

type contextKey int

const (
	ctxKeyEngine ContextKey = 1 << iota
)

// WithEngine returns a new context, derived from the given one and 
// carrying the given Engine within it.
func WithEngine(ctx context.Context, engine *core.Engine) context.Context {
	return context.WithValue(ctx, ctxKeyEngine, engine)
}

// GetEngine returns one core.Engine instance.
func GetEngine(ctx context.Context) *core.Engine {
	return ctx.Value(ctxKeyEngine).(*core.Engine)
}
