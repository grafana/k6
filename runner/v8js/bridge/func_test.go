package bridge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBridgeFuncEmpty(t *testing.T) {
	assert.NotPanics(t, func() { BridgeFunc(func() {}) })
}

func TestBridgeFuncInvalid(t *testing.T) {
	assert.Panics(t, func() { BridgeFunc(struct{}{}) })
}

func TestBridgeFuncArgs(t *testing.T) {
	fn := BridgeFunc(func(i int, a string) {})
	assert.Equal(t, 2, len(fn.In))
}

func TestBridgeFuncReturns(t *testing.T) {
	fn := BridgeFunc(func() (int, string) { return 0, "" })
	assert.Equal(t, 2, len(fn.Out))
}
