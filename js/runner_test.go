package js

import (
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"testing"
)

func TestNewVU(t *testing.T) {
	r := New("script", "1+1")
	_, err := r.NewVU()
	assert.NoError(t, err)
}

func TestNewVUInvalidJS(t *testing.T) {
	r := New("script", "aiugbauibeuifa")
	_, err := r.NewVU()
	assert.NoError(t, err)
}

func TestReconfigure(t *testing.T) {
	r := New("script", "1+1")
	vu_, err := r.NewVU()
	assert.NoError(t, err)
	vu := vu_.(*VU)

	vu.ID = 100
	vu.Iteration = 100

	vu.Reconfigure(1)
	assert.Equal(t, int64(1), vu.ID)
	assert.Equal(t, int64(0), vu.Iteration)
}

func TestRunOnceIncreasesIterations(t *testing.T) {
	r := New("script", "1+1")
	vu_, err := r.NewVU()
	assert.NoError(t, err)
	vu := vu_.(*VU)

	assert.Equal(t, int64(0), vu.Iteration)
	vu.RunOnce(context.Background())
	assert.Equal(t, int64(1), vu.Iteration)
}
