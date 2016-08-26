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
	Filter    stats.Filter
	Writer    io.Writer
	Formatter Formatter

	mutex sync.Mutex
}

func (b Backend) Submit(batches [][]stats.Sample) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	for _, batch := range batches {
		for _, s := range batch {
			if !b.Filter.Check(s) {
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
