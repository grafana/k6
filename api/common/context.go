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

	"go.k6.io/k6/core"
)

type ContextKey int

const ctxKeyEngine = ContextKey(1)

// WithEngine sets the k6 running Engine in the under the hood context.
//
// Deprecated: Use directly the Engine as dependency.
func WithEngine(ctx context.Context, engine *core.Engine) context.Context {
	return context.WithValue(ctx, ctxKeyEngine, engine)
}

// GetEngine returns the k6 running Engine fetching it from the context.
//
// Deprecated: Use directly the Engine as dependency.
func GetEngine(ctx context.Context) *core.Engine {
	return ctx.Value(ctxKeyEngine).(*core.Engine)
}
