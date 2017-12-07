package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlotLimiter(t *testing.T) {
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

func TestMultiSlotLimiter(t *testing.T) {
	t.Run("0", func(t *testing.T) {
		l := NewMultiSlotLimiter(0)
		assert.Nil(t, l.Slot("test"))
	})
	t.Run("1", func(t *testing.T) {
		l := NewMultiSlotLimiter(1)
		assert.Equal(t, l.Slot("test"), l.Slot("test"))
		assert.NotNil(t, l.Slot("test"))
	})
}
