package registry

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPidRegistry(t *testing.T) {
	p := &PidRegistry{}

	var wg sync.WaitGroup
	expected := []int{}
	iteration := 100
	wg.Add(iteration)
	for i := 0; i < iteration; i++ {
		go func(i int) {
			p.RegisterPid(i)
			wg.Done()
		}(i)
		expected = append(expected, i)
	}

	wg.Wait()

	got := p.Pids()

	assert.ElementsMatch(t, expected, got)
}
