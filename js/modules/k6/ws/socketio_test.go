package ws

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstance(t *testing.T) {
	instance := NewSocketIO()
	assert.NotNil(t, instance)
}
