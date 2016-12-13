package json

import (
	"github.com/loadimpact/k6/stats"
	"time"
)

type Envelope struct {
	Type   string      `json:"type"`
	Data   interface{} `json:"data"`
	Metric string      `json:"metric,omitempty"`
}

type JSONSample struct {
	Time  time.Time         `json:"time"`
	Value float64           `json:"value"`
	Tags  map[string]string `json:"tags"`
}

func NewJSONSample(sample *stats.Sample) *JSONSample {
	return &JSONSample{
		Time:  sample.Time,
		Value: sample.Value,
		Tags:  sample.Tags,
	}
}

func Wrap(t interface{}) *Envelope {
	switch data := t.(type) {
	case stats.Sample:
		return &Envelope{
			Type:   "Point",
			Metric: data.Metric.Name,
			Data:   NewJSONSample(&data),
		}
	case *stats.Metric:
		return &Envelope{
			Type:   "Metric",
			Metric: data.Name,
			Data:   data,
		}
	}
	return nil
}
