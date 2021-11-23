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
	"github.com/stretchr/testify/require"
)

func TestBarrier(t *testing.T) {
	t.Run("should work", func(t *testing.T) {
		ctx := context.Background()
		timeoutSetings := NewTimeoutSettings(nil)
		frameManager := NewFrameManager(ctx, nil, nil, timeoutSetings, NewLogger(ctx, NullLogger(), false, nil))
		frame := NewFrame(ctx, frameManager, nil, cdp.FrameID("frame_id_0123456789"))

		barrier := NewBarrier()
		barrier.AddFrameNavigation(frame)
		frame.emit(EventFrameNavigation, "some data")

		err := barrier.Wait(ctx)
		require.Nil(t, err)
	})
}
