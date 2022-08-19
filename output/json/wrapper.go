package json

import (
	"errors"
	"time"

	jlexer "github.com/mailru/easyjson/jlexer"
	"github.com/mailru/easyjson/jwriter"
	"github.com/mstoykov/atlas"
	"go.k6.io/k6/metrics"
)

//go:generate easyjson -pkg -no_std_marshalers -gen_build_flags -mod=mod .

//easyjson:json
type sampleEnvelope struct {
	Metric string `json:"metric"`
	Type   string `json:"type"`
	Data   struct {
		Time  time.Time           `json:"time"`
		Value float64             `json:"value"`
		Tags  tagsAndMetaEnvelope `json:"tags"`
	} `json:"data"`
}

type tagsAndMetaEnvelope metrics.TagsAndMeta

func (tme tagsAndMetaEnvelope) MarshalEasyJSON(w *jwriter.Writer) {
	w.RawByte('{')
	first := true

	// TODO: figure out some way to not rely on the Atlas API?
	n := (*atlas.Node)(tme.Tags)
	for !n.IsRoot() {
		prev, key, value := n.Data()
		if first {
			first = false
		} else {
			w.RawByte(',')
		}
		w.String(key)
		w.RawByte(':')
		w.String(value)
		n = prev
	}
	for key, value := range tme.Metadata {
		if first {
			first = false
		} else {
			w.RawByte(',')
		}
		w.String(key)
		w.RawByte(':')
		w.String(value)
	}
	w.RawByte('}')
}

func (tme *tagsAndMetaEnvelope) UnmarshalEasyJSON(l *jlexer.Lexer) {
	l.AddError(errors.New("tagsAndMetaEnvelope cannot be unmarshalled from JSON"))
}

// wrapSample is used to package a metric sample in a way that's nice to export
// to JSON and backwards-compatible.
func wrapSample(sample metrics.Sample) sampleEnvelope {
	s := sampleEnvelope{
		Type:   "Point",
		Metric: sample.Metric.Name,
	}
	s.Data.Time = sample.Time
	s.Data.Value = sample.Value
	s.Data.Tags = tagsAndMetaEnvelope{
		Tags:     sample.Tags,
		Metadata: sample.Metadata,
	}
	return s
}

//easyjson:json
type metricEnvelope struct {
	Type string `json:"type"`
	Data struct {
		Name       string               `json:"name"`
		Type       metrics.MetricType   `json:"type"`
		Contains   metrics.ValueType    `json:"contains"`
		Thresholds metrics.Thresholds   `json:"thresholds"`
		Submetrics []*metrics.Submetric `json:"submetrics"`
	} `json:"data"`
	Metric string `json:"metric"`
}
