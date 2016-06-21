package lib

import (
	"testing"
	"time"
)

func TestVUsAtSingleStage(t *testing.T) {
	test := Test{
		Stages: []TestStage{
			TestStage{Duration: 10 * time.Second, StartVUs: 0, EndVUs: 10},
		},
	}
	if n := test.VUsAt(0 * time.Second); n != 0 {
		t.Errorf("Wrong number at 0s: %d", n)
	}
	if n := test.VUsAt(5 * time.Second); n != 5 {
		t.Errorf("Wrong number at 5s: %d", n)
	}
	if n := test.VUsAt(10 * time.Second); n != 10 {
		t.Errorf("Wrong number at 10s: %d", n)
	}
}

func TestVUsAtMultiStage(t *testing.T) {
	test := Test{
		Stages: []TestStage{
			TestStage{Duration: 5 * time.Second, StartVUs: 0, EndVUs: 10},
			TestStage{Duration: 10 * time.Second, StartVUs: 10, EndVUs: 20},
		},
	}
	if n := test.VUsAt(5 * time.Second); n != 10 {
		t.Errorf("Wrong number at 5s: %d", n)
	}
	if n := test.VUsAt(10 * time.Second); n != 15 {
		t.Errorf("Wrong number at 10s: %d", n)
	}
	if n := test.VUsAt(15 * time.Second); n != 20 {
		t.Errorf("Wrong number at 15s: %d", n)
	}
}
