package lib

import (
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

func TestEase(t *testing.T) {
	// x[y][t] = x1, tx = 0, ty = 100
	data := map[int64]map[int64]map[int64]int64{
		0: map[int64]map[int64]int64{
			0:   map[int64]int64{0: 0, 10: 0, 50: 0, 100: 0},
			100: map[int64]int64{0: 0, 10: 10, 50: 50, 100: 100},
			500: map[int64]int64{0: 0, 10: 50, 50: 250, 100: 500},
		},
		100: map[int64]map[int64]int64{
			200: map[int64]int64{0: 100, 10: 110, 50: 150, 100: 200},
			0:   map[int64]int64{0: 100, 10: 90, 50: 50, 100: 0},
		},
	}

	for x, data := range data {
		t.Run("x="+strconv.FormatInt(x, 10), func(t *testing.T) {
			for y, data := range data {
				t.Run("y="+strconv.FormatInt(y, 10), func(t *testing.T) {
					for t0, x1 := range data {
						t.Run("t="+strconv.FormatInt(t0, 10), func(t *testing.T) {
							assert.Equal(t, x1, Ease(t0, 0, 100, x, y))
						})
					}
				})
			}
		})
	}
}
