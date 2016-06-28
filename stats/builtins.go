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
			if err := b.encoder.Encode(b.format(&p)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (JSONBackend) format(p *Point) map[string]interface{} {
	data := map[string]interface{}{
		"time":   p.Time,
		"stat":   p.Stat.Name,
		"tags":   p.Tags,
		"values": p.Values,
	}
	if p.Tags == nil {
		data["tags"] = Tags{}
	}
	return data
}
