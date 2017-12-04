package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLimiter(t *testing.T) {
	l := NewSlotLimiter(1)
	l.Begin()
	done := false
	go func() {
		done = true
		l.End()
	}()
	l.Begin()
	assert.True(t, done)
	l.End()
}
