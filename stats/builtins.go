package stats

import (
	"encoding/json"
	"io"
)

type JSONBackend struct {
	encoder *json.Encoder
}

func NewJSONBackend(w io.Writer) Backend {
	return &JSONBackend{encoder: json.NewEncoder(w)}
}

func (b *JSONBackend) Submit(batches [][]Sample) error {
	for _, batch := range batches {
		for _, s := range batch {
			if err := b.encoder.Encode(b.format(&s)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (JSONBackend) format(s *Sample) map[string]interface{} {
	data := map[string]interface{}{
		"time":   s.Time,
		"stat":   s.Stat.Name,
		"tags":   s.Tags,
		"values": s.Values,
	}
	if s.Tags == nil {
		data["tags"] = Tags{}
	}
	return data
}
