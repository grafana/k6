package bridge

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestBridgeType(t *testing.T) {
	tp := BridgeType(reflect.TypeOf(""))
	assert.Equal(t, reflect.String, tp.Kind)
}

func TestBridgeTypeInvalid(t *testing.T) {
	assert.Panics(t, func() { BridgeType(reflect.TypeOf(func() {})) })
}

func TestBridgeTypeStruct(t *testing.T) {
	tp := BridgeType(reflect.TypeOf(struct {
		F1 string `json:"f1"`
		F2 int    `json:"f2"`
	}{}))
	assert.Contains(t, tp.Spec, "F1")
	assert.Contains(t, tp.Spec, "F2")
}

func TestBridgeTypeStructNoTagExcluded(t *testing.T) {
	tp := BridgeType(reflect.TypeOf(struct {
		F1 string `json:"f1"`
		F2 int
	}{}))
	assert.Equal(t, 1, len(tp.Spec))
}
