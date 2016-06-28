package stats

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestValue(t *testing.T) {
	assert.EqualValues(t, Values{"value": 1.0}, Value(1.0))
}
