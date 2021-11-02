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
	"fmt"

	k6common "go.k6.io/k6/js/common"
	"golang.org/x/net/context"
)

// k6Throw throws a k6 error
func k6Throw(ctx context.Context, format string, a ...interface{}) {
	rt := k6common.GetRuntime(ctx)
	if rt == nil {
		// this should never happen unless a programmer error
		panic("cannot get k6 runtime")
	}
	k6common.Throw(rt, fmt.Errorf(format, a...))
}
