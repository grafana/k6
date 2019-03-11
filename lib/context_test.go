package lib

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextState(t *testing.T) {
	st := &State{}
	assert.Equal(t, st, GetState(WithState(context.Background(), st)))
}

func TestContextStateNil(t *testing.T) {
	assert.Nil(t, GetState(context.Background()))
}
