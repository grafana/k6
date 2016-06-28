package stats

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestApplyIntentDefault(t *testing.T) {
	v := ApplyIntent(10.0, DefaultIntent)
	assert.IsType(t, 10.0, v)
}

func TestApplyIntentTime(t *testing.T) {
	v := ApplyIntent(10.0, TimeIntent)
	assert.IsType(t, time.Duration(10), v)
}
