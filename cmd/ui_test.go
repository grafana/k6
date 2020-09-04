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

package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/loadimpact/k6/ui/pb"
)

// Return progressbars with different content lengths, to test for
// padding.
func createTestProgressBars(num, padding, colIdx int) []*pb.ProgressBar {
	pbs := make([]*pb.ProgressBar, num)
	for i := 0; i < num; i++ {
		left := fmt.Sprintf("left %d", i)
		rightCol1 := fmt.Sprintf("right %d", i)
		progress := 0.0
		status := pb.Running
		if i == colIdx {
			pad := strings.Repeat("+", padding)
			left += pad
			rightCol1 += pad
			progress = 1.0
			status = pb.Done
		}
		pbs[i] = pb.New(
			pb.WithLeft(func() string { return left }),
			pb.WithStatus(status),
			pb.WithProgress(func() (float64, []string) {
				return progress, []string{rightCol1, "000"}
			}),
		)
	}
	return pbs
}

func TestRenderMultipleBars(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		padding     int
		widthDelta  int
		expOut      string
		expLongLine int
	}{
		{"pad0", 0, 0, `
left 0   [--------------------------------------] right 0  000
left 1 ✓ [======================================] right 1  000
left 2   [--------------------------------------] right 2  000
`, 62},
		{"pad2", 2, 0, `
left 0     [--------------------------------------] right 0    000
left 1++ ✓ [======================================] right 1++  000
left 2     [--------------------------------------] right 2    000
`, 66},
		{"pad0compact", 0, -50, `
left 0   [   0% ] right 0  000
left 1 ✓ [ 100% ] right 1  000
left 2   [   0% ] right 2  000
`, 30},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pbs := createTestProgressBars(3, tc.padding, 1)
			out, longestLine := renderMultipleBars(false, false, 6+tc.padding, 80, tc.widthDelta, pbs)
			assert.Equal(t, tc.expOut, out)
			assert.Equal(t, tc.expLongLine, longestLine)
		})
	}
}
