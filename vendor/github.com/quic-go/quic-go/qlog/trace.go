package qlog

import (
	"time"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/logging"

	"github.com/francoispqt/gojay"
)

type topLevel struct {
	trace trace
}

func (topLevel) IsNil() bool { return false }
func (l topLevel) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("qlog_format", "NDJSON")
	enc.StringKey("qlog_version", "draft-02")
	enc.StringKeyOmitEmpty("title", "quic-go qlog")
	enc.ObjectKey("configuration", configuration{Version: quicGoVersion})
	enc.ObjectKey("trace", l.trace)
}

type configuration struct {
	Version string
}

func (c configuration) IsNil() bool { return false }
func (c configuration) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("code_version", c.Version)
}

type vantagePoint struct {
	Name string
	Type protocol.Perspective
}

func (p vantagePoint) IsNil() bool { return false }
func (p vantagePoint) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKeyOmitEmpty("name", p.Name)
	switch p.Type {
	case protocol.PerspectiveClient:
		enc.StringKey("type", "client")
	case protocol.PerspectiveServer:
		enc.StringKey("type", "server")
	}
}

type commonFields struct {
	ODCID         logging.ConnectionID
	GroupID       logging.ConnectionID
	ProtocolType  string
	ReferenceTime time.Time
}

func (f commonFields) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("ODCID", f.ODCID.String())
	enc.StringKey("group_id", f.ODCID.String())
	enc.StringKeyOmitEmpty("protocol_type", f.ProtocolType)
	enc.Float64Key("reference_time", float64(f.ReferenceTime.UnixNano())/1e6)
	enc.StringKey("time_format", "relative")
}

func (f commonFields) IsNil() bool { return false }

type trace struct {
	VantagePoint vantagePoint
	CommonFields commonFields
}

func (trace) IsNil() bool { return false }
func (t trace) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("vantage_point", t.VantagePoint)
	enc.ObjectKey("common_fields", t.CommonFields)
}
