package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/internal/ui/pb"
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
			t.Parallel()
			pbs := createTestProgressBars(3, tc.padding, 1)
			out, longestLine := renderMultipleBars(true, false, false, 6+tc.padding, 80, tc.widthDelta, pbs)
			assert.Equal(t, tc.expOut, out)
			assert.Equal(t, tc.expLongLine, longestLine)
		})
	}
}
