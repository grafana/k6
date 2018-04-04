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

package netext

import (
	"context"
	"net/http/httptrace"
)

type ctxKey int

const (
	ctxKeyTracer ctxKey = iota
	ctxKeyAuth
)

func WithTracer(ctx context.Context, tracer *Tracer) context.Context {
	ctx = httptrace.WithClientTrace(ctx, tracer.Trace())
	ctx = context.WithValue(ctx, ctxKeyTracer, tracer)
	return ctx
}

func WithAuth(ctx context.Context, auth string) context.Context {
	return context.WithValue(ctx, ctxKeyAuth, auth)
}

func GetAuth(ctx context.Context) string {
	v := ctx.Value(ctxKeyAuth)
	if v == nil {
		return ""
	}
	return v.(string)
}
