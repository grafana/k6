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

func (b *JSONBackend) Submit(batches [][]Point) error {
	for _, batch := range batches {
		for _, p := range batch {
			data := map[string]interface{}{
				"time":   p.Time,
				"stat":   p.Stat.Name,
				"tags":   p.Tags,
				"values": p.Values,
			}
			if p.Tags == nil {
				data["tags"] = map[string]interface{}{}
			}
			if err := b.encoder.Encode(data); err != nil {
				return err
			}
		}
	}

	return nil
}
