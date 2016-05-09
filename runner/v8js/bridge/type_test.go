package bridge

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestBridgeType(t *testing.T) {
	tp := BridgeType(reflect.TypeOf(""))
	assert.Equal(t, reflect.String, tp.Kind)
	assert.Equal(t, nil, tp.Spec)
	assert.Equal(t, "", tp.JSONKey)
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
	assert.Equal(t, reflect.String, tp.Spec["F1"].Kind)
	assert.Equal(t, "f1", tp.Spec["F1"].JSONKey)
	assert.Contains(t, tp.Spec, "F2")
	assert.Equal(t, reflect.Int, tp.Spec["F2"].Kind)
	assert.Equal(t, "f2", tp.Spec["F2"].JSONKey)
}

func TestBridgeTypeStructNoTagExcluded(t *testing.T) {
	tp := BridgeType(reflect.TypeOf(struct {
		F1 string `json:"f1"`
		F2 int
	}{}))
	assert.Equal(t, 1, len(tp.Spec))
}
