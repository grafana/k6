package kafka

import (
	"testing"

	"github.com/grafana/sobek"
)

func TestExportArgumentMapNilExport(t *testing.T) {
	t.Parallel()
	rt := sobek.New()
	requireGoErrorMessage(t, func() {
		_ = exportArgumentMap(rt, sobek.Undefined(), "reader config")
	}, "reader config is required")
}

func TestExportArgumentMapWrongType(t *testing.T) {
	t.Parallel()
	rt := sobek.New()
	requireGoErrorMessage(t, func() {
		_ = exportArgumentMap(rt, rt.ToValue("not-object"), "reader config")
	}, "Invalid reader config, OriginalError: expected object, got string")
}

func TestDecodeArgumentMapNilParams(t *testing.T) {
	t.Parallel()
	rt := sobek.New()
	requireGoErrorMessage(t, func() {
		decodeArgumentMap(rt, nil, &struct{}{}, "topic config")
	}, "topic config is required")
}

func TestDecodeArgumentMapMarshalError(t *testing.T) {
	t.Parallel()
	rt := sobek.New()
	ch := make(chan int)
	requireGoErrorMessage(t, func() {
		var target struct{}
		decodeArgumentMap(rt, map[string]any{"c": ch}, &target, "topic config")
	}, "Invalid topic config, OriginalError: json: unsupported type: chan int")
}

func TestDecodeArgumentMapUnmarshalError(t *testing.T) {
	t.Parallel()
	rt := sobek.New()
	type bad struct {
		A string `json:"a"`
	}
	var target bad
	requireGoErrorMessage(t, func() {
		decodeArgumentMap(rt, map[string]any{"a": 12345}, &target, "topic config")
	}, "Invalid topic config, OriginalError: json: cannot unmarshal number into Go struct field bad.a of type string")
}
