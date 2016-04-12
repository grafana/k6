package runner

import (
	"testing"
)

func TestScaleNoChange(t *testing.T) {
	i := 10
	start := func() { i++ }
	stop := func() { i-- }
	scale(i, 10, start, stop)
	if i != 10 {
		t.Fail()
	}
}

func TestScaleAdd(t *testing.T) {
	i := 10
	start := func() { i++ }
	stop := func() { i-- }
	scale(i, 15, start, stop)
	if i != 15 {
		t.Fail()
	}
}

func TestScaleRemove(t *testing.T) {
	i := 10
	start := func() { i++ }
	stop := func() { i-- }
	scale(i, 5, start, stop)
	if i != 5 {
		t.Fail()
	}
}
