package accumulate

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSumEmpty(t *testing.T) {
	assert.Equal(t, 0.0, Dimension{}.Sum())
}

func TestSum(t *testing.T) {
	assert.Equal(t, 20.0, Dimension{Values: []float64{5, 3, 4, 7, 1}}.Sum())
}

func TestMinEmpty(t *testing.T) {
	assert.Equal(t, 0.0, Dimension{}.Min())
}

func TestMin(t *testing.T) {
	assert.Equal(t, 1.0, Dimension{Values: []float64{5, 3, 4, 7, 1}}.Min())
}

func TestMaxEmpty(t *testing.T) {
	assert.Equal(t, 0.0, Dimension{}.Max())
}

func TestMax(t *testing.T) {
	assert.Equal(t, 7.0, Dimension{Values: []float64{5, 3, 4, 7, 1}}.Max())
}

func TestAvgEmpty(t *testing.T) {
	assert.Equal(t, 0.0, Dimension{}.Avg())
}

func TestAvgOne(t *testing.T) {
	assert.Equal(t, 5.0, Dimension{Values: []float64{5}}.Avg())
}

func TestAvgTwo(t *testing.T) {
	assert.Equal(t, 4.0, Dimension{Values: []float64{5, 3}}.Avg())
}

func TestAvgThree(t *testing.T) {
	assert.Equal(t, 4.0, Dimension{Values: []float64{5, 3, 4}}.Avg())
}

func TestMedEmpty(t *testing.T) {
	assert.Equal(t, 0.0, Dimension{}.Med())
}

func TestMedOne(t *testing.T) {
	assert.Equal(t, 5.0, Dimension{Values: []float64{5}}.Med())
}

func TestMedTwo(t *testing.T) {
	assert.Equal(t, 4.0, Dimension{Values: []float64{5, 3}}.Med())
}

func TestMedThree(t *testing.T) {
	assert.Equal(t, 3.0, Dimension{Values: []float64{5, 3, 4}}.Med())
}

func TestMedFour(t *testing.T) {
	assert.Equal(t, 3.5, Dimension{Values: []float64{5, 3, 4, 7}}.Med())
}

func TestMedFive(t *testing.T) {
	assert.Equal(t, 4.0, Dimension{Values: []float64{5, 3, 4, 7, 1}}.Med())
}

func TestPct90One(t *testing.T) {
	assert.Equal(t, 1.0, Dimension{Values: []float64{1}}.Pct(0.9))
}

func TestPct90Two(t *testing.T) {
	assert.Equal(t, 2.0, Dimension{Values: []float64{1, 2}}.Pct(0.9))
}

func TestPct90Three(t *testing.T) {
	assert.Equal(t, 3.0, Dimension{Values: []float64{1, 2, 3}}.Pct(0.9))
}

func TestPct90Four(t *testing.T) {
	assert.Equal(t, 4.0, Dimension{Values: []float64{1, 2, 3, 4}}.Pct(0.9))
}

func TestPct90Five(t *testing.T) {
	assert.Equal(t, 5.0, Dimension{Values: []float64{1, 2, 3, 4, 5}}.Pct(0.9))
}

func TestPct90Six(t *testing.T) {
	assert.Equal(t, 6.0, Dimension{Values: []float64{1, 2, 3, 4, 5, 6}}.Pct(0.9))
}

func TestPct90Seven(t *testing.T) {
	assert.Equal(t, 7.0, Dimension{Values: []float64{1, 2, 3, 4, 5, 6, 7}}.Pct(0.9))
}

func TestPct90Eight(t *testing.T) {
	assert.Equal(t, 8.0, Dimension{Values: []float64{1, 2, 3, 4, 5, 6, 7, 8}}.Pct(0.9))
}

func TestPct90Nine(t *testing.T) {
	assert.Equal(t, 9.0, Dimension{Values: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}}.Pct(0.9))
}

func TestPct90Ten(t *testing.T) {
	assert.Equal(t, 9.0, Dimension{Values: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}}.Pct(0.9))
}
