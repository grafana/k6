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

	"github.com/chromedp/cdproto"
	"github.com/stretchr/testify/require"
)

func TestEventEmitterSpecificEvent(t *testing.T) {
	t.Parallel()

	t.Run("add event handler", func(t *testing.T) {
		ctx := context.Background()
		emitter := NewBaseEventEmitter(ctx)
		ch := make(chan Event)

		emitter.on(ctx, []string{cdproto.EventTargetTargetCreated}, ch)
		emitter.sync(func() {
			require.Len(t, emitter.handlers, 1)
			require.Contains(t, emitter.handlers, cdproto.EventTargetTargetCreated)
			require.Len(t, emitter.handlers[cdproto.EventTargetTargetCreated], 1)
			require.Equal(t, ch, emitter.handlers[cdproto.EventTargetTargetCreated][0].ch)
			require.Empty(t, emitter.handlersAll)
		})
	})

	t.Run("remove event handler", func(t *testing.T) {
		ctx := context.Background()
		cancelCtx, cancelFn := context.WithCancel(ctx)
		emitter := NewBaseEventEmitter(cancelCtx)
		ch := make(chan Event)

		emitter.on(cancelCtx, []string{cdproto.EventTargetTargetCreated}, ch)
		cancelFn()
		emitter.emit(cdproto.EventTargetTargetCreated, nil) // Event handlers are removed as part of event emission

		emitter.sync(func() {
			require.Contains(t, emitter.handlers, cdproto.EventTargetTargetCreated)
			require.Len(t, emitter.handlers[cdproto.EventTargetTargetCreated], 0)
		})
	})

	t.Run("emit event", func(t *testing.T) {
		ctx := context.Background()
		emitter := NewBaseEventEmitter(ctx)
		ch := make(chan Event, 1)

		emitter.on(ctx, []string{cdproto.EventTargetTargetCreated}, ch)
		emitter.emit(cdproto.EventTargetTargetCreated, "hello world")
		msg := <-ch

		emitter.sync(func() {
			require.Equal(t, cdproto.EventTargetTargetCreated, msg.typ)
			require.Equal(t, "hello world", msg.data)
		})
	})
}

func TestEventEmitterAllEvents(t *testing.T) {
	t.Parallel()

	t.Run("add catch-all event handler", func(t *testing.T) {
		ctx := context.Background()
		emitter := NewBaseEventEmitter(ctx)
		ch := make(chan Event)

		emitter.onAll(ctx, ch)

		emitter.sync(func() {
			require.Len(t, emitter.handlersAll, 1)
			require.Equal(t, ch, emitter.handlersAll[0].ch)
			require.Empty(t, emitter.handlers)
		})
	})

	t.Run("remove catch-all event handler", func(t *testing.T) {
		ctx := context.Background()
		emitter := NewBaseEventEmitter(ctx)
		cancelCtx, cancelFn := context.WithCancel(ctx)
		ch := make(chan Event)

		emitter.onAll(cancelCtx, ch)
		cancelFn()
		emitter.emit(cdproto.EventTargetTargetCreated, nil) // Event handlers are removed as part of event emission

		emitter.sync(func() {
			require.Len(t, emitter.handlersAll, 0)
		})
	})

	t.Run("emit event", func(t *testing.T) {
		ctx := context.Background()
		emitter := NewBaseEventEmitter(ctx)
		ch := make(chan Event, 1)

		emitter.onAll(ctx, ch)
		emitter.emit(cdproto.EventTargetTargetCreated, "hello world")
		msg := <-ch

		emitter.sync(func() {
			require.Equal(t, cdproto.EventTargetTargetCreated, msg.typ)
			require.Equal(t, "hello world", msg.data)
		})
	})
}
