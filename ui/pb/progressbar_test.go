/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package pb

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

// See https://github.com/loadimpact/k6/issues/1295 for details.
func TestProgressBarStringShouldNotPanic(t *testing.T) {
	fmtStr := GetFixedLengthIntFormat(int64(0)) + "/%d shared iters among %d VUs"
	progresFn := func() (float64, string) {
		val := atomic.LoadUint64(new(uint64))
		return float64(val) / float64(0), fmt.Sprintf(fmtStr, val, 0, 0)
	}
	pb := &ProgressBar{progress: progresFn, width: 40}
	assert.NotPanics(t, func() { _ = pb.String() })
}
