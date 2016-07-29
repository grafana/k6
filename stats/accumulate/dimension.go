package accumulate

import (
	"math"
)

type Dimension struct {
	Values []float64
	Last   float64

	dirty bool
}

func (d Dimension) Sum() float64 {
	var sum float64
	for _, v := range d.Values {
		sum += v
	}
	return sum
}

func (d Dimension) Min() float64 {
	if len(d.Values) == 0 {
		return 0
	}

	var min float64 = math.MaxFloat64
	for _, v := range d.Values {
		if v < min {
			min = v
		}
	}
	return min
}

func (d Dimension) Max() float64 {
	var max float64
	for _, v := range d.Values {
		if v > max {
			max = v
		}
	}
	return max
}

func (d Dimension) Avg() float64 {
	l := len(d.Values)
	switch l {
	case 0:
		return 0
	case 1:
		return d.Values[0]
	default:
		return d.Sum() / float64(l)
	}
}

func (d Dimension) Med() float64 {
	l := len(d.Values)
	switch {
	case l == 0:
		// No items: median is 0
		return 0
	case l == 1:
		// One item: median is that one item
		return d.Values[0]
	case (l & 0x01) == 0:
		// Even number of items: median is the mean of the middle values
		return (d.Values[l/2] + d.Values[(l/2)-1]) / 2
	default:
		// Odd number of items: median is the middle value
		return d.Values[l/2]
	}
}

func (d Dimension) Pct(pct float64) float64 {
	l := len(d.Values)
	if l == 0 {
		return 0
	}

	idx := int(math.Ceil(float64(l)*pct)) - 1
	return d.Values[idx]
}
