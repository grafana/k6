package writer

import (
	"github.com/loadimpact/speedboat/stats"
	"io"
	"sync"
)

type Formatter interface {
	Format(data interface{}) ([]byte, error)
}

type Backend struct {
	Only    map[string]bool
	Exclude map[string]bool

	Writer    io.Writer
	Formatter Formatter

	mutex sync.Mutex
}

func (b Backend) Submit(batches [][]stats.Sample) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	hasOnly := len(b.Only) > 0

	for _, batch := range batches {
		for _, s := range batch {
			if hasOnly && !b.Only[s.Stat.Name] {
				continue
			}
			if b.Exclude[s.Stat.Name] {
				continue
			}

			data := b.Format(&s)
			bytes, err := b.Formatter.Format(data)
			if err != nil {
				return err
			}

			if _, err := b.Writer.Write(bytes); err != nil {
				return err
			}
			if _, err := b.Writer.Write([]byte{'\n'}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (b Backend) Format(s *stats.Sample) map[string]interface{} {
	data := map[string]interface{}{
		"time":   s.Time,
		"stat":   s.Stat.Name,
		"tags":   s.Tags,
		"values": s.Values,
	}
	if s.Tags == nil {
		data["tags"] = stats.Tags{}
	}
	return data
}
