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
	"testing"

	"github.com/chromedp/cdproto/cdp"
	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

func TestFrameDocument_Issue53(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fm := NewFrameManager(ctx, nil, nil, nil, nil)
	frame := NewFrame(ctx, fm, nil, cdp.FrameID("42"))

	// frame should not panic with a nil document
	stub := &executionContextTestStub{
		evaluateFn: func(apiCtx context.Context, forceCallable bool, returnByValue bool, pageFunc goja.Value, args ...goja.Value) (res interface{}, err error) {
			// return nil to test for panic
			return nil, nil
		},
	}

	// document() waits for the main execution context
	go frame.setContext("main", stub)

	require.NotPanics(t, func() {
		_, err := frame.document()
		require.Error(t, err)
	})

	// frame gets the document from the evaluate call
	want := &ElementHandle{}
	stub.evaluateFn = func(apiCtx context.Context, forceCallable bool, returnByValue bool, pageFunc goja.Value, args ...goja.Value) (res interface{}, err error) {
		return want, nil
	}
	got, err := frame.document()
	require.NoError(t, err)
	require.Equal(t, want, got)

	// frame sets documentHandle in the document method
	got = frame.documentHandle
	require.Equal(t, want, got)
}

type executionContextTestStub struct {
	ExecutionContext
	evaluateFn func(apiCtx context.Context, forceCallable bool, returnByValue bool, pageFunc goja.Value, args ...goja.Value) (res interface{}, err error)
}

func (e executionContextTestStub) evaluate(apiCtx context.Context, forceCallable bool, returnByValue bool, pageFunc goja.Value, args ...goja.Value) (res interface{}, err error) {
	return e.evaluateFn(apiCtx, forceCallable, returnByValue, pageFunc, args...)
}
